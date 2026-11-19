package graph

import (
	"testing"

	"github.com/bradleyjkemp/cupaloy/v2"
	"github.com/stretchr/testify/require"

	"starterkit/internal/graph/generated"
	"starterkit/internal/textsearch"
)

func testSurface() surface {
	schema := generated.NewExecutableSchema(generated.Config{Resolvers: &Resolver{}})

	return newSurface(schema.Schema())
}

func grep(t *testing.T, view *SchemaView, query string) string {
	t.Helper()

	matcher, err := textsearch.Compile(query, textsearch.OpExact)
	require.NoError(t, err)

	return view.Grep(matcher, 0, 0)
}

func TestTheUserSurfaceHidesTheAdminNamespace(t *testing.T) {
	user := testSurface().user

	_, err := user.Types([]string{"AdminQuery"})
	require.ErrorIs(t, err, ErrUnknownType)

	_, err = user.Types([]string{"BanUserInput"})
	require.ErrorIs(t, err, ErrUnknownType, "a type only the admin namespace reaches leaked")

	cupaloy.SnapshotT(t, user.EntryPoints())
}

func TestTheAdminSurfaceListsTheNamespacesWithoutGoDirectives(t *testing.T) {
	admin := testSurface().admin

	cupaloy.SnapshotT(t, admin.EntryPoints())

	mutation, err := admin.Types([]string{"AdminMutation"})
	require.NoError(t, err)
	require.Contains(t, mutation, "createMCPKey(input: CreateMCPKeyInput!): CreateMCPKeyOrErrorPayload!")

	require.Contains(t, grep(t, admin, "@go"), "No line matches", "the SDL carries Go binding directives")
	require.Contains(t, grep(t, admin, "starterkit/internal"), "No line matches", "the SDL names Go packages")
}
