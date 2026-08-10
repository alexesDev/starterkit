package signinbyoidc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"starterkit/internal/db"
	"starterkit/internal/dbtime"
)

//go:generate go tool github.com/matryer/moq -out mocks_test.go -pkg signinbyoidc_test . Env

type Env interface {
	DBUpsertUserByOIDC(ctx context.Context, arg db.DBUpsertUserByOIDCParams) (db.User, error)
	DBGetAdminByUserID(ctx context.Context, userID int64) (db.Admin, error)
	DBCountAdmins(ctx context.Context) (int64, error)
	DBInsertAdmin(ctx context.Context, arg db.DBInsertAdminParams) error
	BootstrapAdminEmail() string
	Now() time.Time
}

type Input struct {
	Issuer   string
	Subject  string
	Email    string
	Name     string
	IssuedAt time.Time
}

type Result struct {
	User    db.User
	IsAdmin bool
}

func Resolve(ctx context.Context, env Env, input Input) (*Result, error) {
	if input.Issuer == "" || input.Subject == "" {
		return nil, errors.New("identity has no issuer or subject")
	}

	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email == "" {
		return nil, errors.New("identity has no email")
	}

	userParams := db.DBUpsertUserByOIDCParams{
		OidcIssuer:   input.Issuer,
		OidcSubject:  input.Subject,
		Email:        email,
		Name:         input.Name,
		LastSigninAt: seenAt(input.IssuedAt),
		CreatedAt:    dbtime.At(env.Now()),
	}

	user, err := env.DBUpsertUserByOIDC(ctx, userParams)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert user: %w", err)
	}

	isAdmin, err := ensureAdmin(ctx, env, user, email)
	if err != nil {
		return nil, err
	}

	return &Result{User: user, IsAdmin: isAdmin}, nil
}

func seenAt(issued time.Time) *dbtime.Stamp {
	if issued.IsZero() {
		return nil
	}

	at := dbtime.At(issued)

	return &at
}

func ensureAdmin(ctx context.Context, env Env, user db.User, email string) (bool, error) {
	_, err := env.DBGetAdminByUserID(ctx, user.ID)
	if err == nil {
		return true, nil
	}

	if !db.IsNotFound(err) {
		return false, fmt.Errorf("failed to look up admin: %w", err)
	}

	bootstrap := strings.ToLower(strings.TrimSpace(env.BootstrapAdminEmail()))
	if bootstrap == "" || bootstrap != email {
		return false, nil
	}

	count, err := env.DBCountAdmins(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to count admins: %w", err)
	}

	if count > 0 {
		return false, nil
	}

	adminParams := db.DBInsertAdminParams{UserID: user.ID, CreatedAt: dbtime.At(env.Now())}

	err = env.DBInsertAdmin(ctx, adminParams)
	if err != nil {
		return false, fmt.Errorf("failed to bootstrap admin: %w", err)
	}

	return true, nil
}
