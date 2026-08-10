package signinbyoidc_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"starterkit/internal/case/signinbyoidc"
	"starterkit/internal/db"
)

func input() signinbyoidc.Input {
	return signinbyoidc.Input{
		Issuer:   "https://issuer.test",
		Subject:  "subject-1",
		Email:    "  Admin@Example.com ",
		Name:     "Admin",
		IssuedAt: time.Date(2026, time.August, 10, 9, 0, 0, 0, time.UTC),
	}
}

func envWithNoAdmins(stored *db.DBUpsertUserByOIDCParams) *EnvMock {
	return &EnvMock{
		DBUpsertUserByOIDCFunc: func(_ context.Context, arg db.DBUpsertUserByOIDCParams) (db.User, error) {
			*stored = arg
			return db.User{ID: 1, Email: arg.Email}, nil
		},
		DBGetAdminByUserIDFunc: func(context.Context, int64) (db.Admin, error) {
			return db.Admin{}, sql.ErrNoRows
		},
		DBCountAdminsFunc:       func(context.Context) (int64, error) { return 0, nil },
		DBInsertAdminFunc:       func(context.Context, db.DBInsertAdminParams) error { return nil },
		BootstrapAdminEmailFunc: func() string { return "admin@example.com" },
		NowFunc:                 func() time.Time { return time.Unix(1786000000, 0).UTC() },
	}
}

func TestResolveNormalisesTheAddressAndSeedsTheSignInTime(t *testing.T) {
	var stored db.DBUpsertUserByOIDCParams

	env := envWithNoAdmins(&stored)

	_, err := signinbyoidc.Resolve(t.Context(), env, input())
	require.NoError(t, err)

	assert.Equal(t, "admin@example.com", stored.Email)

	if assert.NotNil(t, stored.LastSigninAt, "the token's iat must seed last_signin_at") {
		assert.True(t, input().IssuedAt.Equal(stored.LastSigninAt.Time))
	}
}

func TestResolvePromotesTheBootstrapAdminWhileTheTableIsEmpty(t *testing.T) {
	var stored db.DBUpsertUserByOIDCParams

	env := envWithNoAdmins(&stored)

	result, err := signinbyoidc.Resolve(t.Context(), env, input())
	require.NoError(t, err)

	assert.True(t, result.IsAdmin)
	assert.Len(t, env.DBInsertAdminCalls(), 1)
}

func TestResolveNeverPromotesOnceAnAdminExists(t *testing.T) {
	var stored db.DBUpsertUserByOIDCParams

	env := envWithNoAdmins(&stored)
	env.DBCountAdminsFunc = func(context.Context) (int64, error) { return 1, nil }

	result, err := signinbyoidc.Resolve(t.Context(), env, input())
	require.NoError(t, err)

	assert.False(t, result.IsAdmin)
	assert.Empty(t, env.DBInsertAdminCalls())
}

func TestResolveOnlyPromotesTheConfiguredAddress(t *testing.T) {
	var stored db.DBUpsertUserByOIDCParams

	env := envWithNoAdmins(&stored)
	env.BootstrapAdminEmailFunc = func() string { return "someone.else@example.com" }

	result, err := signinbyoidc.Resolve(t.Context(), env, input())
	require.NoError(t, err)

	assert.False(t, result.IsAdmin)
	assert.Empty(t, env.DBCountAdminsCalls(), "a non-matching address must not even count the admins")
}

func TestResolveRefusesAnIdentityWithNoEmail(t *testing.T) {
	var stored db.DBUpsertUserByOIDCParams

	env := envWithNoAdmins(&stored)

	in := input()
	in.Email = "   "

	_, err := signinbyoidc.Resolve(t.Context(), env, in)
	require.Error(t, err)
}

func TestResolveRefusesAnIdentityWithNoSubject(t *testing.T) {
	var stored db.DBUpsertUserByOIDCParams

	env := envWithNoAdmins(&stored)

	in := input()
	in.Subject = ""

	_, err := signinbyoidc.Resolve(t.Context(), env, in)
	require.Error(t, err)
}

func TestResolveLeavesTheSignInTimeUnsetWithoutAnIssuedAt(t *testing.T) {
	var stored db.DBUpsertUserByOIDCParams

	env := envWithNoAdmins(&stored)

	in := input()
	in.IssuedAt = time.Time{}

	_, err := signinbyoidc.Resolve(t.Context(), env, in)
	require.NoError(t, err)

	assert.Nil(t, stored.LastSigninAt)
}
