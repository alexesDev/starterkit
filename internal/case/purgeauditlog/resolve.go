package purgeauditlog

import (
	"context"
	"fmt"
	"time"

	"starterkit/internal/dbtime"
)

//go:generate go tool github.com/matryer/moq -out mocks_test.go -pkg purgeauditlog_test . Env

type Env interface {
	DBDeleteAuditLogBefore(ctx context.Context, before dbtime.Stamp) error
	AuditRetention() time.Duration
	Now() time.Time
}

func Resolve(ctx context.Context, env Env) error {
	retention := env.AuditRetention()
	if retention <= 0 {
		return nil
	}

	cutoff := dbtime.At(env.Now().Add(-retention))

	err := env.DBDeleteAuditLogBefore(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("failed to purge audit log before %s: %w", cutoff.Format(time.RFC3339), err)
	}

	return nil
}
