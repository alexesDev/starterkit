// Package audit records crossings of the admin boundary.
//
// A Writer must write outside any request transaction: the command an entry
// describes may fail and roll its transaction back, and the record of the
// attempt has to survive that. See docs/auth-dex.md.
package audit

import (
	"context"
	"sync"
	"time"

	"starterkit/internal/confread"
)

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

type Buffer struct {
	mu      sync.Mutex
	entries []Entry
}

func (b *Buffer) Add(entry Entry) {
	b.mu.Lock()
	b.entries = append(b.entries, entry)
	b.mu.Unlock()
}

func (b *Buffer) Take() []Entry {
	b.mu.Lock()
	entries := b.entries
	b.entries = nil
	b.mu.Unlock()

	return entries
}

type Config struct {
	Retention time.Duration
	PurgeSpec string
}

func DefaultConfig() Config {
	return Config{
		Retention: 30 * 24 * time.Hour,
		PurgeSpec: "0 4 * * *",
	}
}

func (c *Config) Register(r *confread.Reader) {
	*c = DefaultConfig()

	r.Duration(&c.Retention, "audit-retention", 0, "how long audit_log rows are kept; 0 disables the purge")
	r.String(&c.PurgeSpec, "audit-purge-spec", "cron spec for the audit retention sweep")
}

func (c *Config) Check(r *confread.Reader) {
	r.Require("audit-purge-spec", c.PurgeSpec != "", "is empty")
}
