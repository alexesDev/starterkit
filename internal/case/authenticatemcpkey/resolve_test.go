package authenticatemcpkey_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"starterkit/internal/audit"
	"starterkit/internal/case/authenticatemcpkey"
	"starterkit/internal/db"
	"starterkit/internal/dbtime"
	"starterkit/internal/mcpkey"
)

const presented = mcpkey.Secret("mcp_presented")

func envFinding(row db.DBGetMCPKeyIdentityByHashRow, lookupErr error) *EnvMock {
	return &EnvMock{
		DBGetMCPKeyIdentityByHashFunc: func(_ context.Context, keyHash mcpkey.Hash) (db.DBGetMCPKeyIdentityByHashRow, error) {
			if string(keyHash) != string(presented.Hash()) {
				return db.DBGetMCPKeyIdentityByHashRow{}, sql.ErrNoRows
			}

			return row, lookupErr
		},
		DBMarkMCPKeyUsedFunc: func(context.Context, db.DBMarkMCPKeyUsedParams) error { return nil },
		WriteAuditFunc:       func(context.Context, audit.Entry) {},
		NowFunc:              func() time.Time { return time.Unix(1786000000, 0).UTC() },
	}
}

func activeKey() db.DBGetMCPKeyIdentityByHashRow {
	return db.DBGetMCPKeyIdentityByHashRow{KeyID: 3, ID: 7, Email: "owner@example.com", IsAdmin: 1}
}

func TestResolveActsAsTheOwnerAndMarksTheKeyUsed(t *testing.T) {
	env := envFinding(activeKey(), nil)

	result, err := authenticatemcpkey.Resolve(t.Context(), env, authenticatemcpkey.Input{Secret: presented, IP: "10.0.0.1"})
	require.NoError(t, err)

	require.Equal(t, int64(3), result.KeyID)
	require.Equal(t, int64(7), result.User.ID)
	require.True(t, result.IsAdmin)

	used := env.DBMarkMCPKeyUsedCalls()
	require.Len(t, used, 1)
	require.Equal(t, int64(3), used[0].Arg.ID)
	require.Equal(t, int64(1786000000), used[0].Arg.UsedAt.Unix())
	require.Equal(t, int64(1786000000-60), used[0].Arg.StaleBefore.Unix(), "last_used_at moves at most once a minute")
	require.Empty(t, env.WriteAuditCalls(), "an accepted key is audited per tool call, not here")
}

func TestResolveRefusesAnUnknownKeyAndAuditsIt(t *testing.T) {
	env := envFinding(activeKey(), nil)

	_, err := authenticatemcpkey.Resolve(t.Context(), env, authenticatemcpkey.Input{Secret: "mcp_other", IP: "10.0.0.1"})
	require.ErrorIs(t, err, authenticatemcpkey.ErrRefused)

	audits := env.WriteAuditCalls()
	require.Len(t, audits, 1)
	require.Equal(t, authenticatemcpkey.ActionDenied, audits[0].Entry.Action)
	require.Nil(t, audits[0].Entry.UserID)
	require.Equal(t, "10.0.0.1", audits[0].Entry.IP)
	require.Empty(t, env.DBMarkMCPKeyUsedCalls())
}

func TestResolveRefusesARevokedKeyUnderItsOwnersName(t *testing.T) {
	row := activeKey()
	revokedAt := dbtime.At(time.Unix(1785000000, 0))
	row.RevokedAt = &revokedAt

	env := envFinding(row, nil)

	_, err := authenticatemcpkey.Resolve(t.Context(), env, authenticatemcpkey.Input{Secret: presented})
	require.ErrorIs(t, err, authenticatemcpkey.ErrRefused)

	audits := env.WriteAuditCalls()
	require.Len(t, audits, 1)
	require.Equal(t, int64(7), *audits[0].Entry.UserID)
	require.Contains(t, audits[0].Entry.Detail, "revoked key 3")
	require.Empty(t, env.DBMarkMCPKeyUsedCalls(), "a revoked key must not look used")
}

func TestResolveRefusesTheKeyOfABannedOwner(t *testing.T) {
	row := activeKey()
	row.BanReason = "spam"

	env := envFinding(row, nil)

	_, err := authenticatemcpkey.Resolve(t.Context(), env, authenticatemcpkey.Input{Secret: presented})
	require.ErrorIs(t, err, authenticatemcpkey.ErrRefused)
	require.Empty(t, env.DBMarkMCPKeyUsedCalls())
}

func TestResolvePropagatesALookupFailureAsAnError(t *testing.T) {
	env := envFinding(activeKey(), errors.New("database is locked"))

	_, err := authenticatemcpkey.Resolve(t.Context(), env, authenticatemcpkey.Input{Secret: presented})
	require.Error(t, err)
	require.NotErrorIs(t, err, authenticatemcpkey.ErrRefused, "an outage must not read as a bad key")
}
