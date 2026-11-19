package graph

import "starterkit/internal/db"

func adminUserRow(row db.DBGetAdminUserByIDRow) db.DBListUsersRow {
	return db.DBListUsersRow(row)
}

func mcpKeyRow(row db.DBGetMCPKeyByIDRow) db.DBListMCPKeysRow {
	return db.DBListMCPKeysRow(row)
}
