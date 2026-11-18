// Package backup copies the live database into a standalone file with VACUUM
// INTO and keeps the newest few. It runs as a queued job on a cron spec, so a
// copy that fails is retried like any other job. See docs/data-layer.md.
package backup

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"starterkit/internal/confread"
)

const (
	filePrefix = "starterkit-"
	fileSuffix = ".sqlite3"
	stampForm  = "20060102T150405Z"
)

type Config struct {
	Dir  string
	Spec string
	Keep int
}

func DefaultConfig() Config {
	return Config{
		Spec: "30 3 * * *",
		Keep: 7,
	}
}

func (c *Config) Register(r *confread.Reader) {
	*c = DefaultConfig()

	r.String(&c.Dir, "backup-dir", "directory the database is copied into; empty means a backups directory beside the database")
	r.String(&c.Spec, "backup-spec", "cron spec for the database copy")
	r.Int(&c.Keep, "backup-keep", 0, "how many copies are kept; 0 disables the copy")
}

func (c *Config) Check(r *confread.Reader) {
	r.Require("backup-spec", c.Spec != "", "is empty")
}

func (c *Config) Enabled() bool {
	return c.Keep > 0
}

type Path string

type Backup struct {
	conn   *sql.DB
	config Config
}

func New(conn *sql.DB, config Config) *Backup {
	return &Backup{conn: conn, config: config}
}

func (b *Backup) BackupDatabase(ctx context.Context, now time.Time) (Path, error) {
	err := os.MkdirAll(b.config.Dir, 0o700)
	if err != nil {
		return "", fmt.Errorf("failed to create the backup directory: %w", err)
	}

	path := filepath.Join(b.config.Dir, filePrefix+now.UTC().Format(stampForm)+fileSuffix)

	_, err = b.conn.ExecContext(ctx, `vacuum into ?`, path)
	if err != nil {
		return "", fmt.Errorf("failed to copy the database: %w", err)
	}

	err = b.prune()
	if err != nil {
		return Path(path), err
	}

	return Path(path), nil
}

func (b *Backup) prune() error {
	entries, err := os.ReadDir(b.config.Dir)
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	var copies []string

	for _, entry := range entries {
		name := entry.Name()
		if entry.Type().IsRegular() && strings.HasPrefix(name, filePrefix) && strings.HasSuffix(name, fileSuffix) {
			copies = append(copies, name)
		}
	}

	slices.Sort(copies)

	for len(copies) > b.config.Keep {
		err = os.Remove(filepath.Join(b.config.Dir, copies[0]))
		if err != nil {
			return fmt.Errorf("failed to remove an old backup: %w", err)
		}

		copies = copies[1:]
	}

	return nil
}
