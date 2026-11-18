package graph

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestTruncateLeavesShortDetailAlone(t *testing.T) {
	detail := strings.Repeat("a", maxDetail)

	require.Equal(t, detail, truncate(detail))
}

func TestTruncateCutsOnARuneBoundary(t *testing.T) {
	oversized := strings.Repeat("é", maxDetail)

	got := truncate(oversized)

	require.True(t, utf8.ValidString(got), "the audit column must never receive invalid UTF-8")
	require.True(t, strings.HasSuffix(got, "..."), "a truncated detail must say so")
	require.Less(t, len(got), len(oversized))
}
