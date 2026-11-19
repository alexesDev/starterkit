package textsearch_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"starterkit/internal/textsearch"
)

func compile(t *testing.T, query string, op textsearch.Op) textsearch.Matcher {
	t.Helper()

	matcher, err := textsearch.Compile(query, op)
	require.NoError(t, err)

	return matcher
}

func TestTheThreeOps(t *testing.T) {
	line := "Run a GraphQL Mutation as the owner"

	require.True(t, compile(t, "mutation owner", textsearch.OpAnd).Match(line))
	require.False(t, compile(t, "mutation banana", textsearch.OpAnd).Match(line))
	require.True(t, compile(t, "mutation banana", textsearch.OpOr).Match(line))
	require.False(t, compile(t, "query banana", textsearch.OpOr).Match(line))
	require.True(t, compile(t, "graphql mutation", textsearch.OpExact).Match(line))
	require.False(t, compile(t, "mutation graphql", textsearch.OpExact).Match(line))
}

func TestAndIsTheDefault(t *testing.T) {
	require.False(t, compile(t, "mutation banana", "").Match("a mutation"))
}

func TestCompileRefusesAnEmptyQueryAndAnUnknownOp(t *testing.T) {
	_, err := textsearch.Compile("  ", textsearch.OpAnd)
	require.ErrorIs(t, err, textsearch.ErrEmptyQuery)

	_, err = textsearch.Compile("x", "xor")
	require.ErrorIs(t, err, textsearch.ErrUnknownOp)
}

func TestCutNumbersLinesMarksHitsAndMergesContext(t *testing.T) {
	lines := []string{"a", "hit one", "b", "hit two", "c", "d", "e", "hit three"}

	excerpt := textsearch.Cut(lines, compile(t, "hit", textsearch.OpAnd), textsearch.Window{Before: 1, After: 1, Hits: 2, FirstLine: 1})

	require.Equal(t, "1- a\n2: hit one\n3- b\n4: hit two\n5- c\n", excerpt.Text)
	require.Equal(t, 3, excerpt.Hits)
	require.Equal(t, 1, excerpt.Omitted, "a capped excerpt must say how much it left out")
}
