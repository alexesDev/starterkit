package listmcpkeys_test

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"starterkit/internal/case/listmcpkeys"
	"starterkit/internal/db"
	"starterkit/internal/page"
)

func TestResolvePagesKeysNewestFirst(t *testing.T) {
	var asked []db.DBListMCPKeysParams

	env := &EnvMock{
		PageLimitsFunc: func() page.Limits {
			return page.Limits{Default: 2, Max: 500}
		},
		DBListMCPKeysFunc: func(_ context.Context, arg db.DBListMCPKeysParams) ([]db.DBListMCPKeysRow, error) {
			asked = append(asked, arg)
			return []db.DBListMCPKeysRow{{ID: 9}, {ID: 4}, {ID: 1}}, nil
		},
	}

	result, err := listmcpkeys.Resolve(t.Context(), env, listmcpkeys.Input{})
	require.NoError(t, err)

	require.Len(t, result.Nodes, 2)
	require.True(t, result.PageInfo.HasNextPage)

	_, err = listmcpkeys.Resolve(t.Context(), env, listmcpkeys.Input{After: result.PageInfo.EndCursor})
	require.NoError(t, err)

	expected := []db.DBListMCPKeysParams{
		{Before: math.MaxInt64, PageLimit: 3},
		{Before: 4, PageLimit: 3},
	}
	require.Equal(t, expected, asked)
}
