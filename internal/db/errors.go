package db

import (
	"database/sql"
	"errors"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func IsUniqueViolation(err error) bool {
	code, ok := resultCode(err)

	return ok && (code == sqlite3.SQLITE_CONSTRAINT_UNIQUE || code == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY)
}

func IsBusy(err error) bool {
	code, ok := resultCode(err)

	return ok && (code&0xff == sqlite3.SQLITE_BUSY || code&0xff == sqlite3.SQLITE_LOCKED)
}

func resultCode(err error) (int, bool) {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return 0, false
	}

	return sqliteErr.Code(), true
}
