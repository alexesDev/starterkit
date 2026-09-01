package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"starterkit/internal/appreq"
	"starterkit/internal/db"
)

type txEnvKeyType struct{}

var ErrNestedTransaction = errors.New("a write transaction is already open")

func (a *app) txEnvFromCtx(ctx context.Context) *app {
	txEnv, ok := ctx.Value(txEnvKeyType{}).(*app)
	if ok && txEnv.currentTx != nil {
		return txEnv
	}

	req, err := appreq.FromCtx(ctx)
	if err == nil {
		env, isApp := req.Env.(*app)
		if isApp && env.currentTx != nil {
			return env
		}
	}

	if a.currentTx != nil {
		return a
	}

	return nil
}

func (a *app) withTxQueries(tx *sql.Tx) *app {
	writeQueries := db.NewWriteQueries(tx)

	newEnv := *a
	newEnv.Queries = writeQueries.Queries
	newEnv.WriteQueries = writeQueries
	newEnv.currentTx = tx

	return &newEnv
}

// WithTransaction is how code with no HTTP request in scope opens one: jobs,
// cron, boot. The write pool holds exactly one connection, so a nested BeginTx
// would block forever waiting for the connection the outer scope already holds.
func (a *app) WithTransaction(ctx context.Context, fn func(context.Context, *app) error) error {
	if txEnv := a.txEnvFromCtx(ctx); txEnv != nil {
		return ErrNestedTransaction
	}

	tx, err := a.conns.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			a.log.Error("failed to roll back transaction", "error", rollbackErr)
		}
	}()

	txEnv := a.withTxQueries(tx)
	txCtx := context.WithValue(ctx, txEnvKeyType{}, txEnv)

	err = fn(txCtx, txEnv)
	if err != nil {
		return err
	}

	commitErr := tx.Commit()
	if commitErr != nil {
		return fmt.Errorf("failed to commit transaction: %w", commitErr)
	}

	return nil
}
