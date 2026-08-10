package model

import "starterkit/internal/db"

type AuditLogConnection struct {
	TotalCount int64
	Nodes      []db.AuditLog
	PageInfo   *PageInfo
}

type PageInfo struct {
	HasNextPage bool
	EndCursor   *int64
}
