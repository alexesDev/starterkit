package authenticatemcpkey

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"starterkit/internal/audit"
	"starterkit/internal/db"
	"starterkit/internal/dbtime"
	"starterkit/internal/mcpkey"
)

//go:generate go tool github.com/matryer/moq -out mocks_test.go -pkg authenticatemcpkey_test . Env

type Env interface {
	DBGetMCPKeyIdentityByHash(ctx context.Context, keyHash mcpkey.Hash) (db.DBGetMCPKeyIdentityByHashRow, error)
	DBMarkMCPKeyUsed(ctx context.Context, arg db.DBMarkMCPKeyUsedParams) error
	WriteAudit(ctx context.Context, entry audit.Entry)
	Now() time.Time
}

const (
	ActionDenied        = audit.PrefixDenied + "mcp"
	LastUsedGranularity = time.Minute
)

var ErrRefused = errors.New("the key is unknown, revoked, or its owner is banned")

type Input struct {
	Secret mcpkey.Secret
	IP     string
}

type Result struct {
	KeyID   int64
	User    db.User
	IsAdmin bool
}

func Resolve(ctx context.Context, env Env, input Input) (*Result, error) {
	refusal := audit.Entry{Action: ActionDenied, IP: input.IP}

	row, err := env.DBGetMCPKeyIdentityByHash(ctx, input.Secret.Hash())
	if db.IsNotFound(err) {
		refusal.Detail = "unknown key"
		env.WriteAudit(ctx, refusal)

		return nil, ErrRefused
	}

	if err != nil {
		return nil, fmt.Errorf("failed to look the key up: %w", err)
	}

	refusal.UserID = &row.ID
	refusal.Email = row.Email

	if row.RevokedAt != nil {
		refusal.Detail = "revoked key " + strconv.FormatInt(row.KeyID, 10)
		env.WriteAudit(ctx, refusal)

		return nil, ErrRefused
	}

	if row.BanReason != "" {
		refusal.Detail = "banned owner of key " + strconv.FormatInt(row.KeyID, 10)
		env.WriteAudit(ctx, refusal)

		return nil, ErrRefused
	}

	now := env.Now()
	usedAt := dbtime.At(now)
	staleBefore := dbtime.At(now.Add(-LastUsedGranularity))

	usedParams := db.DBMarkMCPKeyUsedParams{
		ID:          row.KeyID,
		UsedAt:      &usedAt,
		StaleBefore: &staleBefore,
	}

	err = env.DBMarkMCPKeyUsed(ctx, usedParams)
	if err != nil {
		return nil, fmt.Errorf("failed to mark the key used: %w", err)
	}

	result := &Result{
		KeyID: row.KeyID,
		User: db.User{
			ID:        row.ID,
			Email:     row.Email,
			Name:      row.Name,
			CreatedAt: row.CreatedAt,
		},
		IsAdmin: row.IsAdmin != 0,
	}

	return result, nil
}
