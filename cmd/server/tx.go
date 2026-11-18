package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"starterkit/internal/audit"
	"starterkit/internal/db"
)

type txEnvKeyType struct{}

var ErrNestedTransaction = errors.New("a write transaction is already open")

func (a *app) txEnvFromCtx(ctx context.Context) *app {
	txEnv, ok := ctx.Value(txEnvKeyType{}).(*app)
	if ok && txEnv.currentTx != nil {
		return txEnv
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

func (a *app) WithTransaction(ctx context.Context, fn func(context.Context, *app) error) error {
	if txEnv := a.txEnvFromCtx(ctx); txEnv != nil {
		return ErrNestedTransaction
	}

	pending := &audit.Buffer{}

	err := a.runTransaction(ctx, pending, fn)
	a.flushAudit(ctx, pending.Take())

	return err
}

func (a *app) runTransaction(ctx context.Context, pending *audit.Buffer, fn func(context.Context, *app) error) error {
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
	txEnv.pendingAudit = pending
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
