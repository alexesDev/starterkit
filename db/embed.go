// Package db carries the migration files into the binary, so a deploy migrates
// itself on boot from the same directory the dbmate CLI operates on.
package db

import "embed"

//go:embed migrations/*.sql
var FS embed.FS
