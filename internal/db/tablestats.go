package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func TableRowCounts(ctx context.Context, conn *sql.DB) (map[string]int64, error) {
	names, err := tableNames(ctx, conn)
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int64, len(names))

	for _, name := range names {
		var count int64

		err = conn.QueryRowContext(ctx, `select count(*) from `+quoteIdentifier(name)).Scan(&count)
		if err != nil {
			return nil, fmt.Errorf("failed to count %s: %w", name, err)
		}

		counts[name] = count
	}

	return counts, nil
}

func tableNames(ctx context.Context, conn *sql.DB) ([]string, error) {
	rows, err := conn.QueryContext(ctx, `
		select name from pragma_table_list
		 where schema = 'main' and type = 'table' and name not like 'sqlite_%'
		 order by name`)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	defer rows.Close()

	var names []string

	for rows.Next() {
		var name string

		scanErr := rows.Scan(&name)
		if scanErr != nil {
			return nil, scanErr
		}

		names = append(names, name)
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return names, nil
}

func quoteIdentifier(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
