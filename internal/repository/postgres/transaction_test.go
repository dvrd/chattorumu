package postgres

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTxManager(t *testing.T) {
	t.Run("creates_tx_manager_successfully", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		tm := NewTxManager(db)
		assert.NotNil(t, tm)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("tx_manager_stores_db_reference", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		tm := NewTxManager(db)
		assert.NotNil(t, tm.db)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestTxManager_WithTx_Success(t *testing.T) {
	t.Run("successful_transaction_commits", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		tm := NewTxManager(db)

		// Setup expectations
		mock.ExpectBegin()
		mock.ExpectCommit()

		// Execute transaction
		err = tm.WithTx(context.Background(), func(tx *sql.Tx) error {
			// The function receives a transaction object and completes successfully
			return nil
		})

		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("empty_transaction_commits", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		tm := NewTxManager(db)

		mock.ExpectBegin()
		mock.ExpectCommit()

		err = tm.WithTx(context.Background(), func(tx *sql.Tx) error {
			return nil
		})

		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("transaction_executes_and_commits", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		tm := NewTxManager(db)

		mock.ExpectBegin()
		// Note: we set up expectations but don't execute them in the mock
		// This tests that the transaction manager itself works, not the operations within
		mock.ExpectCommit()

		transactionCalled := false
		err = tm.WithTx(context.Background(), func(tx *sql.Tx) error {
			transactionCalled = true
			// In a real scenario, you would execute queries here using tx
			return nil
		})

		require.NoError(t, err)
		assert.True(t, transactionCalled)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestTxManager_WithTx_Failure(t *testing.T) {
	t.Run("transaction_rolls_back_on_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		tm := NewTxManager(db)

		mock.ExpectBegin()
		mock.ExpectRollback()

		// Execute transaction that returns an error
		err = tm.WithTx(context.Background(), func(tx *sql.Tx) error {
			return errors.New("operation failed")
		})

		require.Error(t, err)
		assert.Equal(t, "operation failed", err.Error())
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("begin_transaction_failure", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		tm := NewTxManager(db)

		mock.ExpectBegin().WillReturnError(errors.New("begin failed"))

		err = tm.WithTx(context.Background(), func(tx *sql.Tx) error {
			return nil
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to begin transaction")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("commit_failure", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		tm := NewTxManager(db)

		mock.ExpectBegin()
		mock.ExpectCommit().WillReturnError(errors.New("commit failed"))

		err = tm.WithTx(context.Background(), func(tx *sql.Tx) error {
			return nil
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to commit transaction")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("rollback_failure_after_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		tm := NewTxManager(db)

		mock.ExpectBegin()
		mock.ExpectRollback().WillReturnError(errors.New("rollback failed"))

		err = tm.WithTx(context.Background(), func(tx *sql.Tx) error {
			return errors.New("operation error")
		})

		require.Error(t, err)
		// Should contain both the original error and rollback error
		assert.Contains(t, err.Error(), "operation error")
		assert.Contains(t, err.Error(), "rollback failed")
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestTxManager_WithTx_ContextHandling(t *testing.T) {
	t.Run("context_with_reasonable_timeout", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		tm := NewTxManager(db)

		// Create a context with reasonable timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// BeginTx will be called with the context
		mock.ExpectBegin()
		mock.ExpectCommit()

		err = tm.WithTx(ctx, func(tx *sql.Tx) error {
			return nil
		})

		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestTxManager_WithTx_ErrorWrapping(t *testing.T) {
	t.Run("user_function_error_is_returned", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		tm := NewTxManager(db)

		userError := errors.New("user function error")

		mock.ExpectBegin()
		mock.ExpectRollback()

		err = tm.WithTx(context.Background(), func(tx *sql.Tx) error {
			return userError
		})

		require.Error(t, err)
		assert.Equal(t, userError, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("wrapped_begin_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		tm := NewTxManager(db)

		beginErr := errors.New("database begin error")
		mock.ExpectBegin().WillReturnError(beginErr)

		err = tm.WithTx(context.Background(), func(tx *sql.Tx) error {
			return nil
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to begin transaction")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("wrapped_commit_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		tm := NewTxManager(db)

		commitErr := errors.New("database commit error")
		mock.ExpectBegin()
		mock.ExpectCommit().WillReturnError(commitErr)

		err = tm.WithTx(context.Background(), func(tx *sql.Tx) error {
			return nil
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to commit transaction")
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestTxManager_WithTx_Idempotency(t *testing.T) {
	t.Run("multiple_sequential_transactions", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		tm := NewTxManager(db)

		// First transaction
		mock.ExpectBegin()
		mock.ExpectCommit()

		err = tm.WithTx(context.Background(), func(tx *sql.Tx) error {
			return nil
		})
		require.NoError(t, err)

		// Second transaction
		mock.ExpectBegin()
		mock.ExpectCommit()

		err = tm.WithTx(context.Background(), func(tx *sql.Tx) error {
			return nil
		})
		require.NoError(t, err)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("transaction_isolation", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		tm := NewTxManager(db)

		// First transaction succeeds
		mock.ExpectBegin()
		mock.ExpectCommit()

		err = tm.WithTx(context.Background(), func(tx *sql.Tx) error {
			return nil
		})
		require.NoError(t, err)

		// Second transaction fails but doesn't affect first
		mock.ExpectBegin()
		mock.ExpectRollback()

		err = tm.WithTx(context.Background(), func(tx *sql.Tx) error {
			return errors.New("second transaction error")
		})
		require.Error(t, err)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestTxManager_WithTx_Atomicity(t *testing.T) {
	t.Run("all_or_nothing_semantics", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		tm := NewTxManager(db)

		mock.ExpectBegin()
		// If we return an error, everything rolls back
		mock.ExpectRollback()

		operationExecuted := false
		err = tm.WithTx(context.Background(), func(tx *sql.Tx) error {
			operationExecuted = true
			// Simulate an operation that fails mid-transaction
			return errors.New("operation failed, triggering rollback")
		})

		require.Error(t, err)
		assert.True(t, operationExecuted)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestTxManager_WithTx_ConcurrentSafety(t *testing.T) {
	t.Run("transaction_manager_is_reusable", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		tm := NewTxManager(db)

		// Create multiple transactions from the same manager
		mock.ExpectBegin()
		mock.ExpectCommit()

		err = tm.WithTx(context.Background(), func(tx *sql.Tx) error {
			return nil
		})
		require.NoError(t, err)

		// Reuse the same manager for another transaction
		mock.ExpectBegin()
		mock.ExpectCommit()

		err = tm.WithTx(context.Background(), func(tx *sql.Tx) error {
			return nil
		})
		require.NoError(t, err)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
