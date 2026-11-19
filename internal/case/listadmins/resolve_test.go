package listadmins_test

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"starterkit/internal/case/listadmins"
	"starterkit/internal/db"
	"starterkit/internal/page"
)

func TestResolvePagesAdminsByUserID(t *testing.T) {
	var asked []db.DBListAdminsParams

	env := &EnvMock{
		PageLimitsFunc: func() page.Limits {
			return page.Limits{Default: 2, Max: 500}
		},
		DBListAdminsFunc: func(_ context.Context, arg db.DBListAdminsParams) ([]db.DBListAdminsRow, error) {
			asked = append(asked, arg)
			return []db.DBListAdminsRow{{UserID: 9}, {UserID: 4}, {UserID: 1}}, nil
		},
	}

	result, err := listadmins.Resolve(t.Context(), env, listadmins.Input{})
	require.NoError(t, err)

	require.Len(t, result.Nodes, 2)
	require.True(t, result.PageInfo.HasNextPage)

	_, err = listadmins.Resolve(t.Context(), env, listadmins.Input{After: result.PageInfo.EndCursor})
	require.NoError(t, err)

	expected := []db.DBListAdminsParams{
		{Before: math.MaxInt64, PageLimit: 3},
		{Before: 4, PageLimit: 3},
	}
	require.Equal(t, expected, asked)
}
