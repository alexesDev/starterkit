#!/usr/bin/env bash
# Rebinds the write block's generated methods from *Queries to *WriteQueries.
# This is the entire read/write split: after this runs, a holder of *db.Queries
# cannot reach an insert/update/delete, and the compiler says so.
set -euo pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
TARGET=$SCRIPT_DIR/queries.write.sql.go

sed -i 's/func (q \*Queries)/func (q \*WriteQueries)/g' "$TARGET"

cat >> "$TARGET" << 'EOF'

type WriteQueries struct {
	*Queries
}

func NewWriteQueries(db DBTX) *WriteQueries {
	return &WriteQueries{
		Queries: New(db),
	}
}
EOF
