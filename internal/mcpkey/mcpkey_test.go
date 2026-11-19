package mcpkey_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"starterkit/internal/mcpkey"
)

func TestGenerateMintsDistinctWellFormedKeys(t *testing.T) {
	first, err := mcpkey.Generate()
	require.NoError(t, err)

	second, err := mcpkey.Generate()
	require.NoError(t, err)

	require.NotEqual(t, first, second)
	require.True(t, strings.HasPrefix(string(first), "mcp_"), "a key must say what it is: %q", first)
}

func TestHashIsStableAndDiffersFromTheKey(t *testing.T) {
	key := mcpkey.Secret("mcp_example")

	require.Equal(t, mcpkey.Secret("mcp_example").Hash(), key.Hash())
	require.NotEqual(t, []byte(key), []byte(key.Hash()))
	require.Len(t, key.Hash(), 32)
}
