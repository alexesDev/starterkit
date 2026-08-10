package listauditlog

import (
	"context"
	"fmt"

	"starterkit/internal/db"
	"starterkit/internal/graph/model"
)

//go:generate go tool github.com/matryer/moq -out mocks_test.go -pkg listauditlog_test . Env

const (
	defaultLimit = 50
	maxLimit     = 500
)

type Env interface {
	DBListAuditLog(ctx context.Context, arg db.DBListAuditLogParams) ([]db.AuditLog, error)
	DBCountAuditLog(ctx context.Context) (int64, error)
}

type Input struct {
	Limit  int64
	Before *int64
}

type Result = model.AuditLogConnection

func Resolve(ctx context.Context, env Env, input Input) (*Result, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	if limit > maxLimit {
		limit = maxLimit
	}

	var before int64
	if input.Before != nil {
		before = *input.Before
	}

	pageParams := db.DBListAuditLogParams{Before: before, PageLimit: limit + 1}

	rows, err := env.DBListAuditLog(ctx, pageParams)
	if err != nil {
		return nil, fmt.Errorf("failed to list audit log: %w", err)
	}

	result := &Result{Nodes: rows, PageInfo: &model.PageInfo{}}

	if int64(len(rows)) > limit {
		result.Nodes = rows[:limit]
		result.PageInfo.HasNextPage = true
	}

	if len(result.Nodes) > 0 {
		cursor := result.Nodes[len(result.Nodes)-1].ID
		result.PageInfo.EndCursor = &cursor
	}

	result.TotalCount, err = env.DBCountAuditLog(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count audit log: %w", err)
	}

	return result, nil
}
