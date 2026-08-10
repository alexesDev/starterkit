package graph

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func TestTruncateLeavesShortDetailAlone(t *testing.T) {
	detail := strings.Repeat("a", maxDetail)

	assert.Equal(t, detail, truncate(detail))
}

func TestTruncateCutsOnARuneBoundary(t *testing.T) {
	oversized := strings.Repeat("é", maxDetail)

	got := truncate(oversized)

	assert.True(t, utf8.ValidString(got), "the audit column must never receive invalid UTF-8")
	assert.True(t, strings.HasSuffix(got, "..."), "a truncated detail must say so")
	assert.Less(t, len(got), len(oversized))
}
