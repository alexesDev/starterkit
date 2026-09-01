package main

import (
	"context"

	"starterkit/internal/case/banuser"
	"starterkit/internal/case/listauditlog"
	"starterkit/internal/case/makeadmin"
	"starterkit/internal/case/removeadmin"
	"starterkit/internal/case/unbanuser"
)

func runInTx[O any](ctx context.Context, a *app, fn func(context.Context, *app) (O, error)) (O, error) {
	var out O

	err := a.WithTransaction(ctx, func(txCtx context.Context, env *app) error {
		var txErr error

		out, txErr = fn(txCtx, env)

		return txErr
	})

	return out, err
}

func (a *app) BanUser(ctx context.Context, input banuser.Input) (banuser.Payload, error) {
	return runInTx(ctx, a, func(txCtx context.Context, env *app) (banuser.Payload, error) {
		return banuser.Resolve(txCtx, env, input)
	})
}

func (a *app) UnbanUser(ctx context.Context, input unbanuser.Input) (unbanuser.Payload, error) {
	return runInTx(ctx, a, func(txCtx context.Context, env *app) (unbanuser.Payload, error) {
		return unbanuser.Resolve(txCtx, env, input)
	})
}

func (a *app) MakeAdmin(ctx context.Context, input makeadmin.Input) (makeadmin.Payload, error) {
	return runInTx(ctx, a, func(txCtx context.Context, env *app) (makeadmin.Payload, error) {
		return makeadmin.Resolve(txCtx, env, input)
	})
}

func (a *app) RemoveAdmin(ctx context.Context, input removeadmin.Input) (removeadmin.Payload, error) {
	return runInTx(ctx, a, func(txCtx context.Context, env *app) (removeadmin.Payload, error) {
		return removeadmin.Resolve(txCtx, env, input)
	})
}

func (a *app) ListAuditLog(ctx context.Context, limit int64, before *int64) (*listauditlog.Result, error) {
	return listauditlog.Resolve(ctx, a, listauditlog.Input{Limit: limit, Before: before})
}
