package db

import (
	"context"
	"database/sql"
	"fmt"
)

func ReclaimFreePages(ctx context.Context, conn *sql.DB) error {
	_, err := conn.ExecContext(ctx, `pragma incremental_vacuum`)
	if err != nil {
		return fmt.Errorf("failed to reclaim free pages: %w", err)
	}

	return nil
}
