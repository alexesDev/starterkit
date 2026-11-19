package mcpserver_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/99designs/gqlgen/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"starterkit/internal/graph"
	"starterkit/internal/mcpserver"
)

const testSchema = `
scalar Int64
type Query { viewer: Viewer! admin: AdminQuery! }
type Mutation { admin: AdminMutation! }
type Viewer { role: String! }
type AdminQuery { users: [User!]! user(id: Int64!): User }
type AdminMutation { banUser(input: BanUserInput!): User! }
type User { id: Int64! email: String! }
input BanUserInput { userId: Int64! reason: String! }
`

func connect(t *testing.T, env mcpserver.Env) *mcp.ClientSession {
	t.Helper()

	return connectCapped(t, env, 20000)
}

func connectCapped(t *testing.T, env mcpserver.Env, responseChars int) *mcp.ClientSession {
	t.Helper()

	config := mcpserver.Config{ResponseChars: responseChars, Version: "test"}
	srv := httptest.NewServer(mcpserver.NewHandler(env, config))
	t.Cleanup(srv.Close)

	transport := &mcp.StreamableClientTransport{Endpoint: srv.URL, DisableStandaloneSSE: true}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "test"}, nil)

	session, err := client.Connect(t.Context(), transport, nil)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = session.Close()
	})

	return session
}

func envSeeing(view *graph.SchemaView) *EnvMock {
	return &EnvMock{
		RecordMCPToolCallFunc: func(context.Context, mcpserver.ToolCall) {},
		GraphQLSurfaceFunc:    func(context.Context) *graph.SchemaView { return view },
		ExecuteGraphQLFunc: func(context.Context, graph.Request) *graphql.Response {
			return &graphql.Response{Data: json.RawMessage(`{"viewer":{"role":"ADMIN"}}`)}
		},
	}
}

func schema(t *testing.T) *ast.Schema {
	t.Helper()

	return gqlparser.MustLoadSchema(&ast.Source{Input: testSchema})
}

func adminEnv(t *testing.T) *EnvMock {
	t.Helper()

	return envSeeing(graph.NewSchemaView(schema(t)))
}

func call(t *testing.T, session *mcp.ClientSession, tool string, arguments map[string]any) (string, bool) {
	t.Helper()

	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: tool, Arguments: arguments})
	require.NoError(t, err)
	require.Len(t, result.Content, 1)

	content, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok, "expected text content, got %#v", result.Content[0])

	return content.Text, result.IsError
}

func instructions(t *testing.T, session *mcp.ClientSession, arguments map[string]any) (string, bool) {
	t.Helper()

	return call(t, session, mcpserver.ToolInstructions, arguments)
}

func introspect(t *testing.T, session *mcp.ClientSession, arguments map[string]any) (string, bool) {
	t.Helper()

	return call(t, session, mcpserver.ToolIntrospection, arguments)
}

var topicCall = regexp.MustCompile(`Read first: instructions\(topics=\["([a-z_]+)"\]\)\.$`)

func listTools(t *testing.T, session *mcp.ClientSession) []*mcp.Tool {
	t.Helper()

	listed, err := session.ListTools(t.Context(), nil)
	require.NoError(t, err)

	return listed.Tools
}

func TestEveryToolDescriptionEndsByNamingAnExistingTopic(t *testing.T) {
	session := connect(t, adminEnv(t))
	tools := listTools(t, session)
	require.Len(t, tools, 4)

	index, _ := instructions(t, session, map[string]any{})

	for _, tool := range tools {
		if tool.Name == mcpserver.ToolInstructions {
			require.Contains(t, tool.Description, "instructions()")

			for _, topic := range []string{"errors", "graphql_query", "graphql_mutation", "overview"} {
				require.Contains(t, tool.Description, topic, "the manual's description must list its topics")
			}

			continue
		}

		found := topicCall.FindStringSubmatch(tool.Description)
		require.NotNil(t, found, "%s: the description must end with Read first: instructions(topics=[...])", tool.Name)

		body, failed := instructions(t, session, map[string]any{"topics": []string{found[1]}})
		require.False(t, failed, "%s points to a topic that does not exist: %s", tool.Name, body)
		require.Contains(t, index, "| "+found[1]+" |", "%s's topic is missing from the index", tool.Name)
	}
}

func TestToolsComeInAStableOrderWithStableText(t *testing.T) {
	first := listTools(t, connect(t, adminEnv(t)))
	second := listTools(t, connect(t, adminEnv(t)))

	names := make([]string, 0, len(first))
	for i, tool := range first {
		names = append(names, tool.Name)
		require.Equal(t, tool.Description, second[i].Description)
	}

	require.Equal(t, []string{"graphql_introspection", "graphql_mutation", "graphql_query", "instructions"}, names)
}

func TestLookupsAreReadOnlyAndMutationsAreNot(t *testing.T) {
	for _, tool := range listTools(t, connect(t, adminEnv(t))) {
		if tool.Name == mcpserver.ToolMutation {
			require.False(t, tool.Annotations.ReadOnlyHint)
			require.True(t, *tool.Annotations.DestructiveHint, "the kit's mutations delete and revoke")

			continue
		}

		require.True(t, tool.Annotations.ReadOnlyHint, "%s must be marked read-only", tool.Name)
		require.True(t, tool.Annotations.IdempotentHint, "%s must be marked idempotent", tool.Name)
	}
}

func TestTheServerInstructionsPointToTheManualWithoutBeingIt(t *testing.T) {
	session := connect(t, adminEnv(t))

	said := session.InitializeResult().Instructions
	lines := strings.Split(said, "\n")

	require.GreaterOrEqual(t, len(lines), 2)
	require.LessOrEqual(t, len(lines), 4)
	require.Contains(t, said, "instructions()")
}

func TestInstructionsWithoutArgumentsIsTheIndex(t *testing.T) {
	session := connect(t, adminEnv(t))

	body, failed := instructions(t, session, map[string]any{})

	require.False(t, failed)
	require.True(t, strings.HasPrefix(body, "# Index"), body)
	require.Contains(t, body, "| graphql_query |")
}

func TestInstructionsReadsTopicsInTheOrderAskedWithOrWithoutExtension(t *testing.T) {
	session := connect(t, adminEnv(t))

	body, failed := instructions(t, session, map[string]any{"topics": []string{"graphql_mutation", "errors.md"}})

	require.False(t, failed, body)
	require.Less(t, strings.Index(body, "=== graphql_mutation ==="), strings.Index(body, "=== errors ==="))
	require.Contains(t, body, "# Errors and limits")
}

func TestInstructionsRefusesAnUnknownTopicWholeAndListsTheValidOnes(t *testing.T) {
	session := connect(t, adminEnv(t))

	body, failed := instructions(t, session, map[string]any{"topics": []string{"overview", "nope"}})

	require.True(t, failed)
	require.Contains(t, body, `"nope"`)
	require.Contains(t, body, "graphql_introspection, graphql_mutation, graphql_query, index")
	require.NotContains(t, body, "# Overview", "an unknown topic must not return a partial answer")
}

func TestInstructionsCapsTopicsPerCall(t *testing.T) {
	session := connect(t, adminEnv(t))

	body, failed := instructions(t, session, map[string]any{"topics": []string{"a", "b", "c", "d", "e", "f"}})

	require.True(t, failed)
	require.Contains(t, body, "at most 5")
}

func TestInstructionsSearchesWithEachOp(t *testing.T) {
	session := connect(t, adminEnv(t))

	and, _ := instructions(t, session, map[string]any{"query": "byFields input"})
	require.Contains(t, and, "## graphql_mutation")
	require.NotContains(t, and, "## overview")

	or, _ := instructions(t, session, map[string]any{"query": "byFields Int64", "query_op": "or"})
	require.Contains(t, or, "## graphql_mutation")
	require.Contains(t, or, "## overview")

	exact, _ := instructions(t, session, map[string]any{"query": "at most 60 fields", "query_op": "exact"})
	require.Contains(t, exact, "## errors")
	require.NotContains(t, exact, "## graphql_mutation")

	none, _ := instructions(t, session, map[string]any{"query": "60 at most", "query_op": "exact"})
	require.Contains(t, none, "No topic matches")
}

func TestInstructionsRefusesTopicsAndQueryTogether(t *testing.T) {
	session := connect(t, adminEnv(t))

	_, failed := instructions(t, session, map[string]any{"topics": []string{"errors"}, "query": "limit"})
	require.True(t, failed)
}

func TestIntrospectionWithoutArgumentsIsHelpAndEntryPointsWithoutFields(t *testing.T) {
	session := connect(t, adminEnv(t))

	body, failed := introspect(t, session, map[string]any{})

	require.False(t, failed)
	require.Contains(t, body, "Search first")
	require.Contains(t, body, `query="user ban"`)
	require.Contains(t, body, `types=["Query"]`)
	require.Contains(t, body, "Query (fields: 2)\n  Query.viewer -> Viewer (fields: 1)\n  Query.admin -> AdminQuery (fields: 2)\n")
	require.Contains(t, body, "Mutation (fields: 1)\n  Mutation.admin -> AdminMutation (fields: 1)\n")
	require.True(t, strings.HasSuffix(body, `Read first: instructions(topics=["graphql_introspection"]).`+"\n"), body)
	require.NotContains(t, body, "users", "the default must not list fields")
}

func TestIntrospectionReturnsARootsFieldsAsAType(t *testing.T) {
	session := connect(t, adminEnv(t))

	body, failed := introspect(t, session, map[string]any{"types": []string{"Query"}})

	require.False(t, failed, body)
	require.Contains(t, body, "admin: AdminQuery!")
}

func TestIntrospectionReturnsSeveralTypesAndRefusesUnknownOnes(t *testing.T) {
	session := connect(t, adminEnv(t))

	body, failed := introspect(t, session, map[string]any{"types": []string{"BanUserInput", "User"}})
	require.False(t, failed, body)
	require.Less(t, strings.Index(body, "input BanUserInput"), strings.Index(body, "type User"))

	body, failed = introspect(t, session, map[string]any{"types": []string{"User", "Ghost"}})
	require.True(t, failed)
	require.Contains(t, body, `"Ghost"`)

	_, failed = introspect(t, session, map[string]any{"types": []string{"A", "B", "C", "D", "E", "F"}})
	require.True(t, failed)
}

func TestIntrospectionGrepsWithEachOpAndContext(t *testing.T) {
	session := connect(t, adminEnv(t))

	and, _ := introspect(t, session, map[string]any{"query": "userId int64", "before": 0, "after": 0})
	require.Contains(t, and, "## BanUserInput (reached by Mutation.admin.banUser(input))\n")
	require.Regexp(t, `\d+: +userId: Int64!\n`, and)
	require.NotContains(t, and, "reason", "zero context lines must show the hit alone")

	withContext, _ := introspect(t, session, map[string]any{"query": "userId", "after": 1, "before": 0})
	require.Regexp(t, `\d+- +reason: String!`, withContext)

	or, _ := introspect(t, session, map[string]any{"query": "email role", "query_op": "or"})
	require.Contains(t, or, "## User (reached by Query.admin.users)\n")
	require.Contains(t, or, "## Viewer (reached by Query.viewer)\n")

	exact, _ := introspect(t, session, map[string]any{"query": "id: Int64!", "query_op": "exact"})
	require.Contains(t, exact, "## User (reached by")
	require.Contains(t, exact, "## AdminQuery (reached by Query.admin)\n")

	_, failed := introspect(t, session, map[string]any{"query": "user", "before": 11})
	require.True(t, failed, "context beyond the cap must be refused")
}

func TestIntrospectionHidesTheAdminNamespaceFromANonAdminInEveryMode(t *testing.T) {
	session := connect(t, envSeeing(graph.NewSchemaView(schema(t), "admin")))

	entry, _ := introspect(t, session, map[string]any{})
	require.NotContains(t, entry, "Admin")

	grep, _ := introspect(t, session, map[string]any{"query": "ban admin", "query_op": "or"})
	require.Contains(t, grep, "No line matches")

	_, failed := introspect(t, session, map[string]any{"types": []string{"AdminQuery"}})
	require.True(t, failed)
}

func TestQueryRunsTheDocumentAndAuditsIt(t *testing.T) {
	env := adminEnv(t)
	session := connect(t, env)

	arguments := map[string]any{
		"query":     "query ($id: Int64!) { admin { user(id: $id) { email } } }",
		"variables": map[string]any{"id": 3},
	}

	body, failed := call(t, session, mcpserver.ToolQuery, arguments)

	require.False(t, failed)
	require.JSONEq(t, `{"data":{"viewer":{"role":"ADMIN"}}}`, body)

	executed := env.ExecuteGraphQLCalls()
	require.Len(t, executed, 1)
	require.Equal(t, arguments["query"], executed[0].Request.Query)
	require.InDelta(t, 3, executed[0].Request.Variables["id"], 0)

	recorded := env.RecordMCPToolCallCalls()
	require.Len(t, recorded, 1)
	require.Equal(t, mcpserver.ToolQuery, recorded[0].Call.Tool)
}

func TestQueryRefusesAMutationAndMutationRefusesAQuery(t *testing.T) {
	env := adminEnv(t)
	session := connect(t, env)

	body, failed := call(t, session, mcpserver.ToolQuery, map[string]any{"query": "mutation { admin { banUser(input: {userId: 1, reason: \"x\"}) { id } } }"})
	require.True(t, failed)
	require.Contains(t, body, "Use graphql_mutation")

	body, failed = call(t, session, mcpserver.ToolMutation, map[string]any{"query": "{ viewer { role } }"})
	require.True(t, failed)
	require.Contains(t, body, "Use graphql_query")

	require.Empty(t, env.ExecuteGraphQLCalls(), "a refused document must not run")
}

func TestARefusedDocumentTeachesTheSchemaAroundIt(t *testing.T) {
	env := adminEnv(t)
	env.ExecuteGraphQLFunc = func(context.Context, graph.Request) *graphql.Response {
		return &graphql.Response{Errors: gqlerror.List{gqlerror.Errorf(`Cannot query field "emial" on type "User".`)}}
	}

	session := connect(t, env)

	body, failed := call(t, session, mcpserver.ToolQuery, map[string]any{"query": "{ admin { users { emial } } }"})

	require.True(t, failed)
	require.Contains(t, body, `"emial" is not in User. Nearest by spelling: email.`)
	require.Contains(t, body, "type User {")
	require.Contains(t, body, `Read: instructions(topics=["graphql_query"]).`)
}

func TestALongAnswerIsCutAndSaysHowToNarrow(t *testing.T) {
	env := adminEnv(t)
	env.ExecuteGraphQLFunc = func(context.Context, graph.Request) *graphql.Response {
		return &graphql.Response{Data: json.RawMessage(`{"padding":"` + strings.Repeat("é", 500) + `"}`)}
	}

	session := connectCapped(t, env, 300)

	body, failed := call(t, session, mcpserver.ToolQuery, map[string]any{"query": "{ viewer { role } }"})

	require.False(t, failed)
	require.Contains(t, body, "The response was cut at 300 of ")
	require.Contains(t, body, "select fewer fields")
	require.True(t, utf8.ValidString(body), "the cut must land on a character boundary")
}
