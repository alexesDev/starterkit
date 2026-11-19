package graph

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vektah/gqlparser/v2/ast"

	"starterkit/internal/appreq"
	"starterkit/internal/audit"
	"starterkit/internal/buildinfo"
	"starterkit/internal/case/banuser"
	"starterkit/internal/case/createmcpkey"
	"starterkit/internal/case/deletemcpkey"
	"starterkit/internal/case/listadmins"
	"starterkit/internal/case/listauditlog"
	"starterkit/internal/case/listmcpkeys"
	"starterkit/internal/case/listusers"
	"starterkit/internal/case/makeadmin"
	"starterkit/internal/case/removeadmin"
	"starterkit/internal/case/revokemcpkey"
	"starterkit/internal/case/unbanuser"
	"starterkit/internal/db"
	"starterkit/internal/logger"
	"starterkit/internal/model"
)

//go:generate go tool github.com/matryer/moq -out mocks_test.go -pkg graph_test . Env

type Resolver struct {
	Env Env
}

type Env interface {
	audit.Writer

	BanUser(ctx context.Context, input banuser.Input) (banuser.Payload, error)
	UnbanUser(ctx context.Context, input unbanuser.Input) (unbanuser.Payload, error)
	MakeAdmin(ctx context.Context, input makeadmin.Input) (makeadmin.Payload, error)
	RemoveAdmin(ctx context.Context, input removeadmin.Input) (removeadmin.Payload, error)
	ListAuditLog(ctx context.Context, input listauditlog.Input) (*listauditlog.Result, error)
	ListUsers(ctx context.Context, input listusers.Input) (*listusers.Result, error)
	ListAdmins(ctx context.Context, input listadmins.Input) (*listadmins.Result, error)
	ListMCPKeys(ctx context.Context, input listmcpkeys.Input) (*listmcpkeys.Result, error)
	CreateMCPKey(ctx context.Context, input createmcpkey.Input) (createmcpkey.Payload, error)
	RevokeMCPKey(ctx context.Context, input revokemcpkey.Input) (revokemcpkey.Payload, error)
	DeleteMCPKey(ctx context.Context, input deletemcpkey.Input) (deletemcpkey.Payload, error)

	DBCountUsers(ctx context.Context) (int64, error)
	DBGetAdminUserByID(ctx context.Context, id int64) (db.DBGetAdminUserByIDRow, error)
	DBCountAdmins(ctx context.Context) (int64, error)
	DBCountMCPKeys(ctx context.Context) (int64, error)
	DBGetMCPKeyByID(ctx context.Context, id int64) (db.DBGetMCPKeyByIDRow, error)
	DBCountAuditLog(ctx context.Context) (int64, error)

	RecordSignOut(ctx context.Context)
	SignOutURL() string
	Build() buildinfo.Info
	Logger() logger.Logger
	Now() time.Time
	MetricsIncreaseAdminDenied(ctx context.Context, operation string)
	CurrentAdmin(ctx context.Context) (int64, error)
	CurrentUser(ctx context.Context) *db.User
	CurrentUserID(ctx context.Context) *int64
}

func (r *Resolver) checkAdmin(ctx context.Context, operation ast.Operation) error {
	env := r.Env
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
