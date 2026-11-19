package listauditlog

import (
	"context"
	"fmt"

	"starterkit/internal/db"
	"starterkit/internal/graph/model"
	"starterkit/internal/page"
)

//go:generate go tool github.com/matryer/moq -out mocks_test.go -pkg listauditlog_test . Env

type Env interface {
	PageLimits() page.Limits
	DBListAuditLog(ctx context.Context, arg db.DBListAuditLogParams) ([]db.AuditLog, error)
}

type (
	Input  = page.Args
	Result = model.AuditLogConnection
)

func Resolve(ctx context.Context, env Env, input Input) (*Result, error) {
	window := env.PageLimits().Window(input)

	rows, err := env.DBListAuditLog(ctx, db.DBListAuditLogParams{Before: window.Before, PageLimit: window.Probe()})
	if err != nil {
		return nil, fmt.Errorf("failed to list audit log: %w", err)
	}

	nodes, info := page.Cut(window, rows, entryID)

	return &Result{Nodes: nodes, PageInfo: info}, nil
}

func entryID(entry db.AuditLog) int64 {
	return entry.ID
}
