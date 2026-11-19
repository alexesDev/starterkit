// Package mcpserver serves the GraphQL schema to MCP callers as four tools:
// instructions, graphql_introspection, graphql_query and graphql_mutation. Each
// answers a small piece at a time, so a model spends its context on what it
// needs. It knows nothing about who the caller is; the route in front of it has
// already decided that.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vektah/gqlparser/v2/ast"

	"starterkit/internal/confread"
	"starterkit/internal/graph"
	"starterkit/internal/textsearch"
)

//go:generate go tool github.com/matryer/moq -out mocks_test.go -pkg mcpserver_test . Env

const (
	ToolInstructions  = "instructions"
	ToolIntrospection = "graphql_introspection"
	ToolQuery         = "graphql_query"
	ToolMutation      = "graphql_mutation"

	defaultContext = 2
	maxContext     = 10
)

var (
	ErrTypesAndQuery     = errors.New("pass types or query, not both")
	ErrContextOutOfRange = fmt.Errorf("before and after must be between 0 and %d", maxContext)
)

type Env interface {
	ExecuteGraphQL(ctx context.Context, request graph.Request) *graphql.Response
	GraphQLSurface(ctx context.Context) *graph.SchemaView
	RecordMCPToolCall(ctx context.Context, call ToolCall)
}

type ToolCall struct {
	Tool  string
	Query string
}

type Version string

type InstructionsInput struct {
	Topics  []string `json:"topics,omitempty"   jsonschema:"topic names to read, at most 5; none reads the index"`
	Query   string   `json:"query,omitempty"    jsonschema:"words to search every topic for"`
	QueryOp string   `json:"query_op,omitempty" jsonschema:"and (every word, the default), or (any word), exact (the phrase)"`
}

type IntrospectionInput struct {
	Types   []string `json:"types,omitempty"    jsonschema:"type names whose full definitions to return, at most 5"`
	Query   string   `json:"query,omitempty"    jsonschema:"words to search the schema lines for"`
	QueryOp string   `json:"query_op,omitempty" jsonschema:"and (every word, the default), or (any word), exact (the phrase)"`
	Before  *int     `json:"before,omitempty"   jsonschema:"context lines before each hit, 2 by default, at most 10"`
	After   *int     `json:"after,omitempty"    jsonschema:"context lines after each hit, 2 by default, at most 10"`
}

type RequestInput struct {
	Query         string         `json:"query"                   jsonschema:"the GraphQL document to run"`
	Variables     map[string]any `json:"variables,omitempty"     jsonschema:"values for the document's variables"`
	OperationName string         `json:"operationName,omitempty" jsonschema:"the operation to run when the document holds several"`
}

type Config struct {
	ResponseChars int
	Version       Version
}

func DefaultConfig() Config {
	return Config{ResponseChars: 20000}
}

func (c *Config) Register(r *confread.Reader) {
	*c = DefaultConfig()

	set := func(raw string) error {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return errors.New("must be a whole number of characters")
		}

		c.ResponseChars = value

		return nil
	}

	r.Func("mcp-response-chars", strconv.Itoa(c.ResponseChars), "longest GraphQL answer an MCP tool returns before cutting it", set)
}

func (c *Config) Check(r *confread.Reader) {
	r.Require("mcp-response-chars", c.ResponseChars > 0, "must be positive")
}

const serverInstructions = "The GraphQL API of this admin panel, as the person who created your key.\n" +
	"Start with instructions(): the index of a short manual, one topic per tool.\n" +
	"Read with graphql_query, change with graphql_mutation; find fields with graphql_introspection first."

const introspectionHelp = "Search first, then fetch only the types you need; never pull the whole schema.\n" +
	"  graphql_introspection(query=\"user ban\", before=1, after=3)   grep the SDL; each hit names its type and the path to it\n" +
	"  graphql_introspection(types=[\"Query\"])                      full definitions, up to five types; this is how a root's fields come\n" +
	"\nEntry points:\n"

func readFirst(topic string) string {
	return ` Read first: instructions(topics=["` + topic + `"]).`
}

func NewHandler(env Env, config Config) http.Handler {
	server := newServer(env, config, loadManual())

	options := &mcp.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
	}

	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, options)
}

func lookup() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: new(false)}
}

func newServer(env Env, config Config, guide manual) *mcp.Server {
	implementation := &mcp.Implementation{Name: "starterkit", Version: string(config.Version)}
	server := mcp.NewServer(implementation, &mcp.ServerOptions{Instructions: serverInstructions})

	instructionsTool := &mcp.Tool{
		Name: ToolInstructions,
		Description: "The manual for this server: how the schema is shaped and how to use each tool. " +
			"Call instructions() for the index, topics=[...] to read, query=... to search. " +
			"Topics: " + strings.Join(guide.names, ", ") + ".",
		Annotations: lookup(),
	}

	introspectionTool := &mcp.Tool{
		Name: ToolIntrospection,
		Description: "Schema lookup: find GraphQL types, fields and arguments before writing a document. " +
			"Search the SDL lines first, then fetch up to five named types; with no arguments it explains itself and lists the entry points." +
			readFirst(ToolIntrospection),
		Annotations: lookup(),
	}

	queryTool := &mcp.Tool{
		Name: ToolQuery,
		Description: "Read data: run a GraphQL query as this key's owner. Refuses mutations. " +
			"A long answer is cut at " + strconv.Itoa(config.ResponseChars) + " characters." + readFirst(ToolQuery),
		Annotations: lookup(),
	}

	mutationTool := &mcp.Tool{
		Name: ToolMutation,
		Description: "Change data: run a GraphQL mutation as this key's owner, written to the audit log. " +
			"Refuses queries; some mutations delete or revoke, so check the input first." + readFirst(ToolMutation),
		Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true), OpenWorldHint: new(false)},
	}

	mcp.AddTool(server, instructionsTool, func(ctx context.Context, _ *mcp.CallToolRequest, input InstructionsInput) (*mcp.CallToolResult, any, error) {
		env.RecordMCPToolCall(ctx, ToolCall{Tool: ToolInstructions, Query: input.Query})

		body, err := answerInstructions(guide, input)
		if err != nil {
			return nil, nil, err
		}

		return text(body, false), nil, nil
	})

	mcp.AddTool(server, introspectionTool, func(ctx context.Context, _ *mcp.CallToolRequest, input IntrospectionInput) (*mcp.CallToolResult, any, error) {
		env.RecordMCPToolCall(ctx, ToolCall{Tool: ToolIntrospection, Query: input.Query})

		body, err := answerIntrospection(env.GraphQLSurface(ctx), input)
		if err != nil {
			return nil, nil, err
		}

		return text(body, false), nil, nil
	})

	queries := requestRunner{env: env, tool: ToolQuery, accepts: ast.Query, responseChars: config.ResponseChars}
	mcp.AddTool(server, queryTool, queries.handle)

	mutations := requestRunner{env: env, tool: ToolMutation, accepts: ast.Mutation, responseChars: config.ResponseChars}
	mcp.AddTool(server, mutationTool, mutations.handle)

	return server
}

func answerInstructions(guide manual, input InstructionsInput) (string, error) {
	if len(input.Topics) > 0 && input.Query != "" {
		return "", ErrTopicsAndQuery
	}

	if input.Query == "" {
		return guide.read(input.Topics)
	}

	matcher, err := textsearch.Compile(input.Query, textsearch.Op(input.QueryOp))
	if err != nil {
		return "", err
	}

	return guide.search(matcher), nil
}

func answerIntrospection(view *graph.SchemaView, input IntrospectionInput) (string, error) {
	if len(input.Types) > 0 && input.Query != "" {
		return "", ErrTypesAndQuery
	}

	if len(input.Types) > 0 {
		return view.Types(input.Types)
	}

	if input.Query == "" {
		return introspectionHelp + view.EntryPoints() + "\n" + strings.TrimSpace(readFirst(ToolIntrospection)) + "\n", nil
	}

	matcher, err := textsearch.Compile(input.Query, textsearch.Op(input.QueryOp))
	if err != nil {
		return "", err
	}

	before, err := contextLines(input.Before)
	if err != nil {
		return "", err
	}

	after, err := contextLines(input.After)
	if err != nil {
		return "", err
	}

	return view.Grep(matcher, before, after), nil
}

func contextLines(requested *int) (int, error) {
	if requested == nil {
		return defaultContext, nil
	}

	if *requested < 0 || *requested > maxContext {
		return 0, ErrContextOutOfRange
	}

	return *requested, nil
}
func text(body string, isError bool) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: body}},
		IsError: isError,
	}
}
