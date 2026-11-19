package main

import (
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"starterkit/internal/appconfig"
	"starterkit/internal/db"
	"starterkit/internal/dbtime"
	"starterkit/internal/graph"
	"starterkit/internal/mcpkey"
	"starterkit/internal/mcpserver"
)

const testBasePath = "/_system/starterkit"

type bearer struct {
	key mcpkey.Secret
}

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	signed := r.Clone(r.Context())
	signed.Header.Set("Authorization", "Bearer "+string(b.key))

	return http.DefaultTransport.RoundTrip(signed)
}

type mcpStand struct {
	app      *app
	endpoint string
	ownerID  int64
}

func newMCPStand(t *testing.T) *mcpStand {
	t.Helper()

	a := newTestApp(t)
	a.config = &appconfig.Config{BasePath: testBasePath, MCP: mcpserver.DefaultConfig()}
	a.Executor = graph.NewExecutor(a, false)

	owner, err := a.DBUpsertUserByOIDC(t.Context(), testUser("owner"))
	require.NoError(t, err)

	err = a.DBInsertAdmin(t.Context(), db.DBInsertAdminParams{UserID: owner.ID, CreatedAt: dbtime.At(a.Now())})
	require.NoError(t, err)

	srv := httptest.NewServer(a.routes())
	t.Cleanup(srv.Close)

	return &mcpStand{app: a, endpoint: srv.URL + testBasePath + "/mcp", ownerID: owner.ID}
}

func (s *mcpStand) issueKey(t *testing.T) (mcpkey.Secret, int64) {
	t.Helper()

	secret, err := mcpkey.Generate()
	require.NoError(t, err)

	insertParams := db.DBInsertMCPKeyParams{
		UserID:    s.ownerID,
		Name:      "test",
		KeyHash:   secret.Hash(),
		CreatedAt: dbtime.At(s.app.Now()),
	}

	stored, err := s.app.DBInsertMCPKey(t.Context(), insertParams)
	require.NoError(t, err)

	return secret, stored.ID
}

func (s *mcpStand) connect(t *testing.T, key mcpkey.Secret) (*mcp.ClientSession, error) {
	t.Helper()

	transport := &mcp.StreamableClientTransport{
		Endpoint:             s.endpoint,
		HTTPClient:           &http.Client{Transport: bearer{key: key}},
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "test"}, nil)

	session, err := client.Connect(t.Context(), transport, nil)
	if err == nil {
		t.Cleanup(func() {
			_ = session.Close()
		})
	}

	return session, err
}

func (s *mcpStand) requireAudited(t *testing.T, action string) {
	t.Helper()

	listParams := db.DBListAuditLogParams{Before: math.MaxInt64, PageLimit: 100}

	require.Eventually(t, func() bool {
		rows, err := s.app.DBListAuditLog(t.Context(), listParams)
		require.NoError(t, err)

		for _, row := range rows {
			if row.Action == action {
				return true
			}
		}

		return false
	}, 2*time.Second, 10*time.Millisecond, "no %s entry in the audit log", action)
}

func callText(t *testing.T, session *mcp.ClientSession, tool string, arguments any) (string, bool) {
	t.Helper()

	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: tool, Arguments: arguments})
	require.NoError(t, err)
	require.Len(t, result.Content, 1)

	content, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok, "expected text content, got %#v", result.Content[0])

	return content.Text, result.IsError
}

func TestMCPRefusesARequestWithoutAKey(t *testing.T) {
	stand := newMCPStand(t)

	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, stand.endpoint, strings.NewReader(`{}`))
	require.NoError(t, err)

	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)

	defer response.Body.Close()

	require.Equal(t, http.StatusUnauthorized, response.StatusCode)
}

func TestMCPRefusesAnUnknownKeyAndAuditsIt(t *testing.T) {
	stand := newMCPStand(t)

	_, err := stand.connect(t, "mcp_unknown")
	require.Error(t, err)

	stand.requireAudited(t, "denied:mcp")
}

func TestMCPServesTheToolsAsTheKeysOwner(t *testing.T) {
	stand := newMCPStand(t)
	key, keyID := stand.issueKey(t)

	session, err := stand.connect(t, key)
	require.NoError(t, err)

	listed, err := session.ListTools(t.Context(), nil)
	require.NoError(t, err)
	require.Len(t, listed.Tools, 4)

	guide, _ := callText(t, session, mcpserver.ToolInstructions, map[string]any{})
	require.Contains(t, guide, "graphql_query")

	sdl, _ := callText(t, session, mcpserver.ToolIntrospection, map[string]any{})
	require.Contains(t, sdl, "Query.admin -> AdminQuery (fields: ", "an admin's key must see the admin namespace")

	query := map[string]any{"query": "{ viewer { role user { email } } admin { mcpKeys { totalCount } } }"}

	answer, failed := callText(t, session, mcpserver.ToolQuery, query)
	require.False(t, failed, answer)
	require.JSONEq(t, `{"data":{"viewer":{"role":"ADMIN","user":{"email":"owner@example.com"}},"admin":{"mcpKeys":{"totalCount":1}}}}`, answer)

	stored, err := stand.app.DBGetMCPKeyByID(t.Context(), keyID)
	require.NoError(t, err)
	require.NotNil(t, stored.LastUsedAt, "a used key must say when")

	stand.requireAudited(t, "mcp:instructions")
	stand.requireAudited(t, "mcp:graphql_introspection")
	stand.requireAudited(t, "mcp:graphql_query")
}

func TestMCPRunsMutationsThroughTheAdminBoundary(t *testing.T) {
	stand := newMCPStand(t)
	key, _ := stand.issueKey(t)

	session, err := stand.connect(t, key)
	require.NoError(t, err)

	mutation := map[string]any{
		"query": `mutation ($input: CreateMCPKeyInput!) { admin { createMCPKey(input: $input) {
			__typename ... on CreateMCPKeyPayload { mcpKeyId } } } }`,
		"variables": map[string]any{"input": map[string]any{"name": "made over mcp"}},
	}

	answer, failed := callText(t, session, mcpserver.ToolMutation, mutation)
	require.False(t, failed, answer)
	require.Contains(t, answer, "CreateMCPKeyPayload")

	count, err := stand.app.DBCountMCPKeys(t.Context())
	require.NoError(t, err)
	require.Equal(t, int64(2), count, "the mutation's transaction did not commit")

	stand.requireAudited(t, "mutation:createMCPKey")
}

func TestMCPRefusesARevokedKey(t *testing.T) {
	stand := newMCPStand(t)
	key, keyID := stand.issueKey(t)

	revokedAt := dbtime.At(stand.app.Now())

	_, err := stand.app.DBRevokeMCPKey(t.Context(), db.DBRevokeMCPKeyParams{ID: keyID, RevokedAt: &revokedAt})
	require.NoError(t, err)

	_, err = stand.connect(t, key)
	require.Error(t, err)
}

func TestMCPNarrowsTheSurfaceWhenTheOwnerIsNoLongerAdmin(t *testing.T) {
	stand := newMCPStand(t)
	key, _ := stand.issueKey(t)

	other, err := stand.app.DBUpsertUserByOIDC(t.Context(), testUser("other"))
	require.NoError(t, err)

	err = stand.app.DBInsertAdmin(t.Context(), db.DBInsertAdminParams{UserID: other.ID, CreatedAt: dbtime.At(stand.app.Now())})
	require.NoError(t, err)

	removed, err := stand.app.DBDeleteAdmin(t.Context(), stand.ownerID)
	require.NoError(t, err)
	require.Equal(t, int64(1), removed)

	session, err := stand.connect(t, key)
	require.NoError(t, err)

	sdl, _ := callText(t, session, mcpserver.ToolIntrospection, map[string]any{})
	require.NotContains(t, sdl, "AdminQuery")

	answer, failed := callText(t, session, mcpserver.ToolQuery, map[string]any{"query": "{ admin { mcpKeys { totalCount } } }"})
	require.True(t, failed, answer)
	require.Contains(t, answer, "unauthorized")
}
