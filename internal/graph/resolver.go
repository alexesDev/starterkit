package graph

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vektah/gqlparser/v2/ast"

	"starterkit/internal/appreq"
	"starterkit/internal/audit"
	"starterkit/internal/case/banuser"
	"starterkit/internal/case/listauditlog"
	"starterkit/internal/case/makeadmin"
	"starterkit/internal/case/removeadmin"
	"starterkit/internal/case/unbanuser"
	"starterkit/internal/db"
	"starterkit/internal/logger"
	"starterkit/internal/model"
)

type Resolver struct {
	DefaultEnv Env
}

// Env is the aggregate at the edge: one line per case, written in that case's
// own Input and Payload types, plus the plain reads a case package would only
// wrap. See docs/env-pattern.md.
type Env interface {
	audit.Writer

	BanUser(ctx context.Context, input banuser.Input) (banuser.Payload, error)
	UnbanUser(ctx context.Context, input unbanuser.Input) (unbanuser.Payload, error)
	MakeAdmin(ctx context.Context, input makeadmin.Input) (makeadmin.Payload, error)
	RemoveAdmin(ctx context.Context, input removeadmin.Input) (removeadmin.Payload, error)
	ListAuditLog(ctx context.Context, limit int64, before *int64) (*listauditlog.Result, error)

	DBListUsers(ctx context.Context) ([]db.DBListUsersRow, error)
	DBCountUsers(ctx context.Context) (int64, error)
	DBGetAdminUserByID(ctx context.Context, id int64) (db.DBGetAdminUserByIDRow, error)
	DBListAdmins(ctx context.Context) ([]db.DBListAdminsRow, error)
	DBCountAdmins(ctx context.Context) (int64, error)

	RecordSignOut(ctx context.Context)
	SignOutURL() string
	BuildGitCommit() string
	Logger() logger.Logger
	Now() time.Time
	MetricsIncreaseAdminDenied(ctx context.Context, operation string)
	CurrentAdmin(ctx context.Context) (int64, error)
	CurrentUser(ctx context.Context) *db.User
	CurrentUserID(ctx context.Context) *int64
}

func (r *Resolver) env(ctx context.Context) Env {
	req, err := appreq.FromCtx(ctx)
	if err != nil {
		return r.DefaultEnv
	}

	reqEnv, ok := req.Env.(Env)
	if !ok {
		return r.DefaultEnv
	}

	return reqEnv
}

// checkAdmin runs once, when the `admin` namespace object is resolved, so every
// field underneath it is behind this check by construction.
func (r *Resolver) checkAdmin(ctx context.Context, operation ast.Operation) error {
	env := r.env(ctx)
	entry := audit.Entry{Action: adminAction(ctx, operation)}

	req, err := appreq.FromCtx(ctx)
	if err != nil {
		return r.denyAdmin(ctx, env, entry)
	}

	entry.IP = req.IP()

	if user := req.User(); user != nil {
		entry.UserID = &user.ID
		entry.Email = user.Email
	}

	_, err = env.CurrentAdmin(ctx)
	if errors.Is(err, model.ErrNotAdmin) {
		return r.denyAdmin(ctx, env, entry)
	}

	if err != nil {
		return fmt.Errorf("failed to check admin: %w", err)
	}

	if operation == ast.Mutation {
		entry.Detail = auditVariables(ctx)
		env.WriteAudit(ctx, entry)
	}

	return nil
}

func (r *Resolver) denyAdmin(ctx context.Context, env Env, entry audit.Entry) error {
	entry.Action = audit.PrefixDenied + entry.Action
	entry.Detail = auditVariables(ctx)

	env.WriteAudit(ctx, entry)
	env.MetricsIncreaseAdminDenied(ctx, entry.Action)

	return model.ErrNotAdmin
}
