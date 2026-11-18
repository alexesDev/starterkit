package graph

import "starterkit/internal/db"

func adminUserRow(row db.DBGetAdminUserByIDRow) db.DBListUsersRow {
	return db.DBListUsersRow(row)
}
