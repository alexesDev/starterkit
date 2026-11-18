package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"starterkit/internal/appreq"
	"starterkit/internal/audit"
	"starterkit/internal/case/signinbyoidc"
	"starterkit/internal/db"
	"starterkit/internal/dbtime"
	"starterkit/internal/gateauth"
	"starterkit/internal/model"
)

func (a *app) CurrentAdmin(ctx context.Context) (int64, error) {
	req, err := appreq.FromCtx(ctx)
	if err != nil || !req.IsAdmin() {
		return 0, model.ErrNotAdmin
	}

	userID := req.UserID()
	if userID == nil {
		return 0, model.ErrNotAdmin
	}

	return *userID, nil
}

func (a *app) CurrentUser(ctx context.Context) *db.User {
	req, err := appreq.FromCtx(ctx)
	if err != nil {
		return nil
	}

	return req.User()
}

func (a *app) CurrentUserID(ctx context.Context) *int64 {
	req, err := appreq.FromCtx(ctx)
	if err != nil {
		return nil
	}

	return req.UserID()
}

func (a *app) WriteAudit(ctx context.Context, entry audit.Entry) {
	req, err := appreq.FromCtx(ctx)
	if err == nil {
		req.AddAudit(entry)
		return
	}

	a.flushAudit(ctx, []audit.Entry{entry})
}

func (a *app) flushAudit(ctx context.Context, entries []audit.Entry) {
	if len(entries) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	for _, entry := range entries {
		auditParams := db.DBInsertAuditLogParams{
			UserID:    entry.UserID,
			Email:     entry.Email,
			Action:    entry.Action,
			Detail:    entry.Detail,
			Ip:        entry.IP,
			CreatedAt: dbtime.At(a.Now()),
		}

		err := a.baseWrite.DBInsertAuditLog(ctx, auditParams)
		if err != nil {
			a.log.Error("failed to write audit entry", "action", entry.Action, "error", err)
		}
	}
}

func (a *app) SignInByOIDC(ctx context.Context, input signinbyoidc.Input) (*signinbyoidc.Result, error) {
	var result *signinbyoidc.Result

	err := a.WithTransaction(ctx, func(txCtx context.Context, env *app) error {
		var txErr error

		result, txErr = signinbyoidc.Resolve(txCtx, env, input)

		return txErr
	})

	return result, err
}

func (a *app) resolveIdentity(req *appreq.Request, r *http.Request) (*model.AuthIdentity, error) {
	ctx := r.Context()

	claimed, err := a.gate.Identity(ctx, r)
	if errors.Is(err, gateauth.ErrNoToken) {
		return nil, nil
	}

	if errors.Is(err, gateauth.ErrInvalidToken) {
		a.log.Warn("rejected an identity from the gate", "error", err)

		return &model.AuthIdentity{StaleToken: true}, nil
	}

	if err != nil {
		a.log.Warn("rejected an identity from the gate", "error", err)

		return nil, nil
	}

	identityParams := db.DBGetIdentityByOIDCParams{
		OidcIssuer:  claimed.Issuer,
		OidcSubject: claimed.Subject,
	}

	identity, err := a.Queries.DBGetIdentityByOIDC(ctx, identityParams)
	if db.IsNotFound(err) {
		return a.firstSignIn(ctx, req, claimed)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load identity: %w", err)
	}

	if identity.BanReason != "" {
		return &model.AuthIdentity{BanReason: identity.BanReason}, nil
	}

	user := &db.User{
		ID:        identity.ID,
		Email:     identity.Email,
		Name:      identity.Name,
		CreatedAt: identity.CreatedAt,
	}

	req.SetIdentity(user, identity.IsAdmin != 0)
	a.recordSignIn(ctx, req, identity, claimed.IssuedAt)

	return &model.AuthIdentity{User: user, IsAdmin: identity.IsAdmin != 0}, nil
}

func (a *app) recordSignIn(ctx context.Context, req *appreq.Request, identity db.DBGetIdentityByOIDCRow, issued time.Time) {
	if issued.IsZero() {
		return
	}

	if identity.LastSigninAt != nil && !issued.After(identity.LastSigninAt.Time) {
		return
	}

	at := dbtime.At(issued)
	markParams := db.DBMarkSignInParams{ID: identity.ID, IssuedAt: &at}

	marked, err := a.baseWrite.DBMarkSignIn(ctx, markParams)
	if err != nil {
		a.log.Error("failed to record a sign-in", "user", identity.ID, "error", err)
		return
	}

	if marked == 0 {
		return
	}

	a.MetricsIncreaseSignIn(ctx)

	signIn := audit.Entry{
		UserID: &identity.ID,
		Email:  identity.Email,
		Action: audit.ActionSignIn,
		IP:     req.IP(),
	}

	a.WriteAudit(ctx, signIn)
}

func (a *app) RecordSignOut(ctx context.Context) {
	req, err := appreq.FromCtx(ctx)
	if err != nil {
		return
	}

	user := req.User()
	if user == nil {
		return
	}

	signOut := audit.Entry{
		UserID: &user.ID,
		Email:  user.Email,
		Action: audit.ActionSignOut,
		IP:     req.IP(),
	}

	a.WriteAudit(ctx, signOut)
}

func (a *app) firstSignIn(ctx context.Context, req *appreq.Request, claimed *gateauth.Identity) (*model.AuthIdentity, error) {
	signIn := signinbyoidc.Input{
		Issuer:   claimed.Issuer,
		Subject:  claimed.Subject,
		Email:    claimed.Email,
		Name:     claimed.Name,
		IssuedAt: claimed.IssuedAt,
	}

	result, err := a.SignInByOIDC(ctx, signIn)
	if err != nil {
		return nil, fmt.Errorf("failed to sign in: %w", err)
	}

	req.SetIdentity(&result.User, result.IsAdmin)
	a.MetricsIncreaseSignIn(ctx)

	userID := result.User.ID
	signInEntry := audit.Entry{
		UserID: &userID,
		Email:  result.User.Email,
		Action: audit.ActionSignIn,
		IP:     req.IP(),
	}

	a.WriteAudit(ctx, signInEntry)

	return &model.AuthIdentity{User: &result.User, IsAdmin: result.IsAdmin}, nil
}
