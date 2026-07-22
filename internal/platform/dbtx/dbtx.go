// Package dbtx provides the transaction seam (ADR-027: short bounded
// transactions, no outbox). Repositories accept a DBTX so the same query
// code runs against the pool (single statement) or inside a transaction
// (multi-statement unit of work). Cross-module ports participate in the
// caller's transaction by receiving the DBTX — this is how the collection
// transaction also updates the catalog-owned circuit state atomically.
package dbtx

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX is the minimal query surface shared by *pgxpool.Pool and pgx.Tx.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// TxBeginner begins a transaction (satisfied by *pgxpool.Pool).
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Runner executes units of work inside a transaction with READ COMMITTED
// isolation (the default; sufficient for the append-only pipeline).
type Runner struct {
	pool TxBeginner
}

// NewRunner returns a Runner backed by the given pool.
func NewRunner(pool TxBeginner) *Runner { return &Runner{pool: pool} }

// Run executes fn within a transaction, committing on success and rolling
// back on error or panic. The tx is passed as a DBTX so repositories and
// cross-module ports can share it.
func (r *Runner) Run(ctx context.Context, fn func(ctx context.Context, tx DBTX) error) (err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	if err := fn(ctx, tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil && rbErr != pgx.ErrTxClosed {
			return fmt.Errorf("%w (rollback failed: %v)", err, rbErr)
		}
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
