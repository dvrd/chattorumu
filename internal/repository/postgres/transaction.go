package postgres

import (
	"context"
	"database/sql"
	"fmt"
)

// TxManager manages database transactions
type TxManager struct {
	db *sql.DB
}

// NewTxManager creates a new transaction manager
func NewTxManager(db *sql.DB) *TxManager {
	return &TxManager{db: db}
}

// WithTx executes a function within a database transaction
// If the function returns an error, the transaction is rolled back
// Otherwise, the transaction is committed
func (tm *TxManager) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := tm.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx err: %v, rb err: %v", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
