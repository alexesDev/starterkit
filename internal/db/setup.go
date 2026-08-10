package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"time"

	"github.com/amacneil/dbmate/v2/pkg/dbmate"

	migrations "starterkit/db"
	_ "starterkit/internal/dbmate/sqlite"
)

type Set struct {
	Read  *sql.DB
	Write *sql.DB
	Queue *sql.DB
}

func (s *Set) Close() {
	for _, conn := range []*sql.DB{s.Read, s.Write, s.Queue} {
		if conn != nil {
			_ = conn.Close()
		}
	}
}

func Setup(ctx context.Context, file string) (*Set, error) {
	err := Migrate(file)
	if err != nil {
		return nil, err
	}

	read, err := open(ctx, file, true)
	if err != nil {
		return nil, err
	}

	write, err := open(ctx, file, false)
	if err != nil {
		_ = read.Close()
		return nil, err
	}

	queue, err := open(ctx, file, false)
	if err != nil {
		_ = read.Close()
		_ = write.Close()

		return nil, err
	}

	return &Set{Read: read, Write: write, Queue: queue}, nil
}

func Migrate(file string) error {
	u, err := url.Parse("sqlite:" + file)
	if err != nil {
		return fmt.Errorf("failed to parse database url: %w", err)
	}

	dbm := dbmate.New(u)
	dbm.FS = migrations.FS
	dbm.MigrationsDir = []string{"migrations"}
	dbm.AutoDumpSchema = false
	dbm.Log = nopWriter{}

	err = dbm.CreateAndMigrate()
	if err != nil {
		return fmt.Errorf("failed to migrate: %w", err)
	}

	return nil
}

type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func open(ctx context.Context, file string, readOnly bool) (*sql.DB, error) {
	u := &url.URL{Path: file}
	q := u.Query()

	q.Add("_pragma", "busy_timeout(20000)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "synchronous(NORMAL)")
	q.Add("_pragma", "temp_store(2)")
	q.Add("_pragma", "cache_size(-64000)")
	q.Add("_dqs", "false")

	if readOnly {
		q.Add("_pragma", "query_only(1)")
	}

	if !readOnly {
		q.Set("_txlock", "immediate")
	}

	u.RawQuery = q.Encode()

	conn, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if readOnly {
		conn.SetMaxOpenConns(25)
		conn.SetMaxIdleConns(25)
		conn.SetConnMaxIdleTime(30 * time.Second)
	} else {
		conn.SetMaxOpenConns(1)
		conn.SetMaxIdleConns(1)
	}

	err = conn.PingContext(ctx)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return conn, nil
}
