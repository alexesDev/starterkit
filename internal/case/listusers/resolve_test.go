package listusers_test

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"starterkit/internal/case/listusers"
	"starterkit/internal/db"
	"starterkit/internal/page"
)

func TestResolvePagesUsersNewestFirstByID(t *testing.T) {
	var asked []db.DBListUsersParams

	env := &EnvMock{
		PageLimitsFunc: func() page.Limits {
			return page.Limits{Default: 2, Max: 500}
		},
		DBListUsersFunc: func(_ context.Context, arg db.DBListUsersParams) ([]db.DBListUsersRow, error) {
			asked = append(asked, arg)
			return []db.DBListUsersRow{{ID: 7}, {ID: 5}, {ID: 2}}, nil
		},
	}

	result, err := listusers.Resolve(t.Context(), env, listusers.Input{})
	require.NoError(t, err)

	require.Len(t, result.Nodes, 2)
	require.True(t, result.PageInfo.HasNextPage)

	_, err = listusers.Resolve(t.Context(), env, listusers.Input{After: result.PageInfo.EndCursor})
	require.NoError(t, err)

	expected := []db.DBListUsersParams{
		{Before: math.MaxInt64, PageLimit: 3},
		{Before: 5, PageLimit: 3},
	}
	require.Equal(t, expected, asked)
}
