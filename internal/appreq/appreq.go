// Package appreq carries the per-request Env through context, because gqlgen
// resolvers are generated with fixed signatures and cannot take it as an
// argument.
package appreq

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"starterkit/internal/audit"
	"starterkit/internal/db"
)

type ctxKeyType struct{}

var ErrNotFound = errors.New("request not found in context")

type Request struct {
	Env any
	R   *http.Request
	W   http.ResponseWriter

	user    *db.User
	isAdmin bool
	ip      string

	mu     sync.Mutex
	audits []audit.Entry
}

func New(env any, w http.ResponseWriter, r *http.Request, ip string) *Request {
	return &Request{Env: env, R: r, W: w, ip: ip}
}

func (c *Request) AddAudit(entry audit.Entry) {
	c.mu.Lock()
	c.audits = append(c.audits, entry)
	c.mu.Unlock()
}

func (c *Request) TakeAudit() []audit.Entry {
	c.mu.Lock()
	entries := c.audits
	c.audits = nil
	c.mu.Unlock()

	return entries
}

func (c *Request) NewContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKeyType{}, c)
}

func FromCtx(ctx context.Context) (*Request, error) {
	c, ok := ctx.Value(ctxKeyType{}).(*Request)
	if !ok {
		return nil, ErrNotFound
	}

	return c, nil
}

func (c *Request) SetIdentity(user *db.User, isAdmin bool) {
	c.user = user
	c.isAdmin = isAdmin
}

func (c *Request) User() *db.User {
	return c.user
}

func (c *Request) IsAdmin() bool {
	return c.isAdmin
}

func (c *Request) IP() string {
	return c.ip
}

func (c *Request) UserID() *int64 {
	if c.user == nil {
		return nil
	}

	return &c.user.ID
}
