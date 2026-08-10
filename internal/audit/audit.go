// Package audit records crossings of the admin boundary.
//
// A Writer must write outside any request transaction: a denied admin
// operation rolls its transaction back, and the record of the attempt has to
// survive that. See docs/auth-dex.md.
package audit

import "context"

const (
	ActionSignIn  = "sign_in"
	ActionSignOut = "sign_out"

	PrefixDenied = "denied:"

	Redacted = "[SENSITIVE]"
)

type Entry struct {
	UserID *int64
	Email  string
	Action string
	Detail string
	IP     string
}

type Writer interface {
	WriteAudit(ctx context.Context, entry Entry)
}
