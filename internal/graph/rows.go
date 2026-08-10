package graph

import "starterkit/internal/db"

// adminUserRow bridges the two sqlc reads that produce an AdminUser. sqlc names
// a row type per query, so the single-row read and the list read are different
// Go types with identical columns, and one GraphQL object can bind to only one
// of them. The conversion stops compiling the day the two shapes diverge.
func adminUserRow(row db.DBGetAdminUserByIDRow) db.DBListUsersRow {
	return db.DBListUsersRow(row)
}
