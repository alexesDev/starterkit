// Package model holds app-level types the GraphQL layer binds to directly.
package model

import (
	"errors"

	"starterkit/internal/db"
)

var ErrNotAdmin = errors.New("unauthorized")

type AdminQuery struct{}

type AdminMutation struct{}

type Viewer struct{}

type AuthIdentity struct {
	User       *db.User
	IsAdmin    bool
	BanReason  string
	StaleToken bool
}

func (i *AuthIdentity) Banned() bool {
	return i != nil && i.BanReason != ""
}

func (i *AuthIdentity) Stale() bool {
	return i != nil && i.StaleToken
}
