package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"

	"starterkit/internal/graph"
)

type requestRunner struct {
	env           Env
	tool          string
	accepts       ast.Operation
	responseChars int
}

func (r requestRunner) handle(ctx context.Context, _ *mcp.CallToolRequest, input RequestInput) (*mcp.CallToolResult, any, error) {
	r.env.RecordMCPToolCall(ctx, ToolCall{Tool: r.tool, Query: input.Query})

	refusal := r.refuseOtherKinds(input.Query)
	if refusal != "" {
		return text(refusal, true), nil, nil
	}

	request := graph.Request{
		Query:         input.Query,
		OperationName: input.OperationName,
		Variables:     input.Variables,
	}

	response := r.env.ExecuteGraphQL(ctx, request)

	encoded, err := json.Marshal(response)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode the GraphQL response: %w", err)
	}

	body := r.cut(string(encoded))

	if len(response.Errors) == 0 {
		return text(body, false), nil, nil
	}

	messages := make([]string, 0, len(response.Errors))
	for _, gqlErr := range response.Errors {
		messages = append(messages, gqlErr.Message)
	}

	explanation := r.env.GraphQLSurface(ctx).Explain(messages)
	body += "\n\n" + explanation + "Read: instructions(topics=[\"" + r.tool + "\"])."

	return text(body, true), nil, nil
}

func (r requestRunner) refuseOtherKinds(document string) string {
	parsed, err := parser.ParseQuery(&ast.Source{Input: document})
	if err != nil {
		return ""
	}

	for _, operation := range parsed.Operations {
		if operation.Operation == r.accepts {
			continue
		}

		other := ToolQuery
		if operation.Operation == ast.Mutation {
			other = ToolMutation
		}

		return "This tool runs only " + string(r.accepts) + " operations; this document holds a " +
			string(operation.Operation) + ". Use " + other + "."
	}

	return ""
}

func (r requestRunner) cut(body string) string {
	if len(body) <= r.responseChars {
		return body
	}

	end := r.responseChars
	for end > 0 && !utf8.RuneStart(body[end]) {
		end--
	}

	kept := body[:end]

	return kept + "\n\nThe response was cut at " + strconv.Itoa(r.responseChars) + " of " + strconv.Itoa(len(body)) +
		" characters. Narrow it: select fewer fields, or fewer nodes (first, where a list takes it)."
}
