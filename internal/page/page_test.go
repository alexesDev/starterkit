package page_test

import (
	"bytes"
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"starterkit/internal/page"
)

func limits() page.Limits {
	return page.Limits{Default: 50, Max: 500}
}

func first(n int64) *int64 {
	return &n
}

func marshal(t *testing.T, cursor page.Cursor) string {
	t.Helper()

	var buf bytes.Buffer

	cursor.MarshalGQL(&buf)

	text, err := strconv.Unquote(buf.String())
	require.NoError(t, err)

	return text
}

func ids(n int) []int64 {
	out := make([]int64, 0, n)
	for i := range n {
		out = append(out, int64(100-i))
	}

	return out
}

func self(id int64) int64 {
	return id
}

func TestWindowStartsAboveEveryIDWithoutACursor(t *testing.T) {
	window := limits().Window(page.Args{First: first(10)})

	require.Equal(t, int64(math.MaxInt64), window.Before)
}

func TestWindowAsksForOneRowMoreThanTheLimit(t *testing.T) {
	window := limits().Window(page.Args{First: first(10)})

	require.Equal(t, int64(10), window.Limit)
	require.Equal(t, int64(11), window.Probe())
}

func TestWindowClampsAnOversizedFirstToTheMax(t *testing.T) {
	window := limits().Window(page.Args{First: first(100000)})

	require.Equal(t, int64(500), window.Limit)
}

func TestWindowFallsBackToTheDefault(t *testing.T) {
	for name, args := range map[string]page.Args{
		"absent":   {},
		"zero":     {First: first(0)},
		"negative": {First: first(-3)},
	} {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, int64(50), limits().Window(args).Limit)
		})
	}
}

func TestWindowStartsBelowTheCursorRow(t *testing.T) {
	after := page.At(42)

	window := limits().Window(page.Args{First: first(10), After: &after})

	require.Equal(t, int64(42), window.Before)
}

func TestCutTrimsTheProbeRowAndReportsAnotherPage(t *testing.T) {
	window := limits().Window(page.Args{First: first(3)})

	nodes, info := page.Cut(window, ids(4), self)

	require.Equal(t, []int64{100, 99, 98}, nodes, "the probe row leaked into the page")
	require.True(t, info.HasNextPage)
	require.NotNil(t, info.EndCursor)
	require.Equal(t, marshal(t, page.At(98)), marshal(t, *info.EndCursor))
}

func TestCutReportsNoNextPageWhenTheProbeFoundNothing(t *testing.T) {
	window := limits().Window(page.Args{First: first(3)})

	nodes, info := page.Cut(window, ids(2), self)

	require.Len(t, nodes, 2)
	require.False(t, info.HasNextPage)
	require.NotNil(t, info.EndCursor)
}

func TestCutLeavesTheCursorUnsetOnAnEmptyPage(t *testing.T) {
	window := limits().Window(page.Args{First: first(3)})

	nodes, info := page.Cut(window, nil, self)

	require.Empty(t, nodes)
	require.False(t, info.HasNextPage)
	require.Nil(t, info.EndCursor)
}

func TestCursorRoundTripsThroughItsWireForm(t *testing.T) {
	var decoded page.Cursor

	err := decoded.UnmarshalGQL(marshal(t, page.At(1234567890123)))
	require.NoError(t, err)

	window := limits().Window(page.Args{After: &decoded})
	require.Equal(t, int64(1234567890123), window.Before)
}

func TestCursorWireFormIsNotTheBareID(t *testing.T) {
	require.NotEqual(t, "42", marshal(t, page.At(42)))
}

func TestCursorRefusesWhatItDidNotIssue(t *testing.T) {
	for name, value := range map[string]any{
		"not base64":      "%%%",
		"not a number":    "bm90LWFuLWlk",
		"zero":            "MA",
		"negative":        "LTU",
		"not a string":    42,
		"an empty string": "",
	} {
		t.Run(name, func(t *testing.T) {
			var cursor page.Cursor

			err := cursor.UnmarshalGQL(value)
			require.ErrorIs(t, err, page.ErrInvalidCursor)
		})
	}
}
