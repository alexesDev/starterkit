package graph_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"starterkit/internal/appreq"
	"starterkit/internal/case/listauditlog"
	"starterkit/internal/db"
	"starterkit/internal/graph"
	"starterkit/internal/graph/model"
	"starterkit/internal/logger"
	"starterkit/internal/page"
)

type response struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func adminEnv() *EnvMock {
	return &EnvMock{
		CurrentAdminFunc: func(context.Context) (int64, error) {
			return 1, nil
		},
		LoggerFunc: logger.Discard,
		ListAuditLogFunc: func(_ context.Context, input listauditlog.Input) (*listauditlog.Result, error) {
			window := page.Limits{Default: 50, Max: 500}.Window(input)
			nodes, info := page.Cut(window, []db.AuditLog{{ID: 3}, {ID: 2}}, func(entry db.AuditLog) int64 { return entry.ID })

			return &model.AuditLogConnection{Nodes: nodes, PageInfo: info}, nil
		},
		DBCountAuditLogFunc: func(context.Context) (int64, error) {
			return 2, nil
		},
	}
}

func query(t *testing.T, env *EnvMock, document string) response {
	t.Helper()

	body, err := json.Marshal(map[string]string{"query": document})
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/graphql", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	scoped := appreq.New("")

	graph.NewHandler(env, false).ServeHTTP(rec, req.WithContext(scoped.NewContext(req.Context())))

	var out response

	err = json.Unmarshal(rec.Body.Bytes(), &out)
	require.NoError(t, err, rec.Body.String())

	return out
}

func TestAuditLogCountsOnlyWhenTotalCountIsSelected(t *testing.T) {
	env := adminEnv()

	scrolled := query(t, env, `{ admin { auditLog(first: 1) { nodes { id } pageInfo { hasNextPage endCursor } } } }`)
	require.Empty(t, scrolled.Errors)
	require.Empty(t, env.DBCountAuditLogCalls(), "a page that only scrolls paid for count(*)")

	counted := query(t, env, `{ admin { auditLog(first: 1) { totalCount } } }`)
	require.Empty(t, counted.Errors)
	require.Len(t, env.DBCountAuditLogCalls(), 1)
	require.JSONEq(t, `{"admin":{"auditLog":{"totalCount":2}}}`, string(counted.Data))
}

func TestAuditLogHandsBackACursorItAccepts(t *testing.T) {
	env := adminEnv()

	first := query(t, env, `{ admin { auditLog(first: 1) { pageInfo { endCursor } } } }`)
	require.Empty(t, first.Errors)

	var parsed struct {
		Admin struct {
			AuditLog struct {
				PageInfo struct {
					EndCursor string `json:"endCursor"`
				} `json:"pageInfo"`
			} `json:"auditLog"`
		} `json:"admin"`
	}

	require.NoError(t, json.Unmarshal(first.Data, &parsed))

	cursor := parsed.Admin.AuditLog.PageInfo.EndCursor
	require.NotEmpty(t, cursor)

	next := query(t, env, `{ admin { auditLog(first: 1, after: "`+cursor+`") { nodes { id } } } }`)
	require.Empty(t, next.Errors)

	calls := env.ListAuditLogCalls()
	require.Len(t, calls, 2)
	require.Equal(t, int64(3), page.Limits{Default: 50, Max: 500}.Window(calls[1].Input).Before)
}

func TestAuditLogRefusesACursorItDidNotIssue(t *testing.T) {
	env := adminEnv()

	got := query(t, env, `{ admin { auditLog(after: "garbage!") { nodes { id } } } }`)

	require.Len(t, got.Errors, 1)
	require.Equal(t, page.ErrInvalidCursor.Error(), got.Errors[0].Message)
	require.Empty(t, env.ListAuditLogCalls(), "a list ran on a cursor that does not decode")
}

func TestAuditLogLeavesAnAbsentFirstToTheConfiguredDefault(t *testing.T) {
	env := adminEnv()

	got := query(t, env, `{ admin { auditLog { nodes { id } } } }`)
	require.Empty(t, got.Errors)

	calls := env.ListAuditLogCalls()
	require.Len(t, calls, 1)
	require.Nil(t, calls[0].Input.First, "the schema supplied a default of its own")
}

func introspect(t *testing.T, introspection graph.Introspection) string {
	t.Helper()

	srv := graph.NewHandler(&EnvMock{LoggerFunc: logger.Discard}, introspection)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/graphql", strings.NewReader(`{"query":"{ __schema { queryType { name } } }"}`))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	return rec.Body.String()
}

func TestTheSchemaIsReadableOnlyWhenIntrospectionIsOn(t *testing.T) {
	require.Contains(t, introspect(t, true), `"queryType":{"name":"Query"}`)
	require.Contains(t, introspect(t, false), "introspection disabled")
}
