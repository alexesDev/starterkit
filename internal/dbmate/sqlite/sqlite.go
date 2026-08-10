// Package sqlite is dbmate's sqlite driver rebound to modernc.org/sqlite.
//
// Adapted from github.com/amacneil/dbmate/pkg/driver/sqlite, whose upstream
// file sits behind a `cgo` build tag because it imports mattn/go-sqlite3.
// Importing this package is what makes `sqlite:` URLs work in a CGO_ENABLED=0
// build, so the server migrates itself on boot without a C toolchain. The
// dbmate CLI used at authoring time still needs cgo, and never runs in
// production. See docs/data-layer.md.
package sqlite

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/amacneil/dbmate/v2/pkg/dbmate"
	"github.com/amacneil/dbmate/v2/pkg/dbutil"

	_ "modernc.org/sqlite"
)

func init() { //nolint:gochecknoinits // driver registration
	dbmate.RegisterDriver(NewDriver, "sqlite")
	dbmate.RegisterDriver(NewDriver, "sqlite3")
}

type Driver struct {
	migrationsTableName string
	databaseURL         *url.URL
	log                 io.Writer
}

func NewDriver(config dbmate.DriverConfig) dbmate.Driver {
	return &Driver{
		migrationsTableName: config.MigrationsTableName,
		databaseURL:         config.DatabaseURL,
		log:                 config.Log,
	}
}

var leadingSlashes = regexp.MustCompile("^//+")

func ConnectionString(u *url.URL) string {
	newURL := *u
	newURL.Scheme = ""

	if newURL.Opaque == "" && newURL.Path != "" {
		newURL.Opaque = "//" + newURL.Host + dbutil.MustUnescapePath(newURL.Path)
		newURL.Path = ""
	}

	return leadingSlashes.ReplaceAllString(newURL.String(), "/")
}

func (drv *Driver) Open() (*sql.DB, error) {
	return sql.Open("sqlite", ConnectionString(drv.databaseURL))
}

func (drv *Driver) CreateDatabase() error {
	fmt.Fprintf(drv.log, "Creating: %s\n", ConnectionString(drv.databaseURL))

	db, err := drv.Open()
	if err != nil {
		return err
	}
	defer dbutil.MustClose(db)

	return db.Ping()
}

func (drv *Driver) DropDatabase() error {
	path := ConnectionString(drv.databaseURL)
	fmt.Fprintf(drv.log, "Dropping: %s\n", path)

	exists, err := drv.DatabaseExists()
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	return os.Remove(path)
}

func (drv *Driver) schemaMigrationsDump(db *sql.DB) ([]byte, error) {
	migrationsTable := drv.quotedMigrationsTableName()

	migrations, err := dbutil.QueryColumn(db,
		"select quote(version) from "+migrationsTable+" order by version asc")
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer

	buf.WriteString("-- Dbmate schema migrations\n")

	if len(migrations) > 0 {
		buf.WriteString("INSERT INTO " + migrationsTable + " (version) VALUES\n  (" +
			strings.Join(migrations, "),\n  (") +
			");\n")
	}

	return buf.Bytes(), nil
}

func (drv *Driver) DumpSchema(db *sql.DB, _ ...string) ([]byte, error) {
	path := ConnectionString(drv.databaseURL)

	schema, err := dbutil.RunCommand("sqlite3", path, ".schema --nosys")
	if err != nil {
		return nil, fmt.Errorf("failed to dump schema (is the sqlite3 CLI installed?): %w", err)
	}

	migrations, err := drv.schemaMigrationsDump(db)
	if err != nil {
		return nil, err
	}

	schema = append(schema, migrations...)

	return dbutil.TrimLeadingSQLComments(schema)
}

func (drv *Driver) DatabaseExists() (bool, error) {
	_, err := os.Stat(ConnectionString(drv.databaseURL))
	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return true, nil
}

func (drv *Driver) MigrationsTableExists(db *sql.DB) (bool, error) {
	exists := false

	err := db.QueryRow("select 1 from sqlite_master where type='table' and name=?",
		drv.migrationsTableName).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}

	return exists, err
}

func (drv *Driver) CreateMigrationsTable(db *sql.DB) error {
	_, err := db.Exec("create table if not exists " + drv.quotedMigrationsTableName() +
		" (version varchar(128) primary key)")

	return err
}

func (drv *Driver) SelectMigrations(db *sql.DB, limit int) (map[string]bool, error) {
	query := "select version from " + drv.quotedMigrationsTableName() + " order by version desc"
	if limit >= 0 {
		query = fmt.Sprintf("%s limit %d", query, limit)
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer dbutil.MustClose(rows)

	migrations := map[string]bool{}

	for rows.Next() {
		var version string

		scanErr := rows.Scan(&version)
		if scanErr != nil {
			return nil, scanErr
		}

		migrations[version] = true
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return migrations, nil
}

func (drv *Driver) InsertMigration(db dbutil.Transaction, version string) error {
	_, err := db.Exec("insert into "+drv.quotedMigrationsTableName()+" (version) values (?)", version)

	return err
}

func (drv *Driver) DeleteMigration(db dbutil.Transaction, version string) error {
	_, err := db.Exec("delete from "+drv.quotedMigrationsTableName()+" where version = ?", version)

	return err
}

func (drv *Driver) Ping() error {
	db, err := drv.Open()
	if err != nil {
		return err
	}
	defer dbutil.MustClose(db)

	return db.Ping()
}

func (drv *Driver) QueryError(query string, err error) error {
	return &dbmate.QueryError{Err: err, Query: query}
}

func (drv *Driver) quotedMigrationsTableName() string {
	return quoteIdentifier(drv.migrationsTableName)
}

func quoteIdentifier(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
