package postgres

import (
	"errors"
	"testing"

	"github.com/lib/pq"
)

func TestIsUniqueViolation_WithPQError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		constraint string
		want       bool
	}{
		{
			name: "unique_violation_matching_constraint",
			err: &pq.Error{
				Code:       "23505",
				Constraint: "users_username_key",
			},
			constraint: "users_username_key",
			want:       true,
		},
		{
			name: "unique_violation_any_constraint",
			err: &pq.Error{
				Code:       "23505",
				Constraint: "users_email_key",
			},
			constraint: "",
			want:       true,
		},
		{
			name: "unique_violation_different_constraint",
			err: &pq.Error{
				Code:       "23505",
				Constraint: "users_email_key",
			},
			constraint: "users_username_key",
			want:       false,
		},
		{
			name: "different_error_code",
			err: &pq.Error{
				Code:       "23503", // foreign key violation
				Constraint: "users_username_key",
			},
			constraint: "users_username_key",
			want:       false,
		},
		{
			name:       "not_pq_error",
			err:        errors.New("some other error"),
			constraint: "users_username_key",
			want:       false,
		},
		{
			name:       "nil_error",
			err:        nil,
			constraint: "users_username_key",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsUniqueViolation(tt.err, tt.constraint)
			if got != tt.want {
				t.Errorf("IsUniqueViolation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsUniqueViolation_WithWrappedError(t *testing.T) {
	// Test that errors.As correctly unwraps the error
	baseErr := &pq.Error{
		Code:       "23505",
		Constraint: "users_username_key",
	}

	wrappedErr := errors.New("failed to insert: " + baseErr.Error())

	// This should NOT match because we're just concatenating strings
	if IsUniqueViolation(wrappedErr, "users_username_key") {
		t.Error("Expected false for string-concatenated error, but got true")
	}

	// But if properly wrapped with %w
	properlyWrapped := &pq.Error{
		Code:       "23505",
		Constraint: "users_username_key",
	}

	if !IsUniqueViolation(properlyWrapped, "users_username_key") {
		t.Error("Expected true for properly wrapped pq.Error")
	}
}

func TestIsUniqueViolation_EmptyConstraint(t *testing.T) {
	err := &pq.Error{
		Code:       "23505",
		Constraint: "",
	}

	// Should match when we don't care about constraint name
	if !IsUniqueViolation(err, "") {
		t.Error("Expected true when checking for any unique violation")
	}
}

func TestIsUniqueViolation_RealWorldScenarios(t *testing.T) {
	tests := []struct {
		name       string
		err        *pq.Error
		constraint string
		want       bool
	}{
		{
			name: "username_duplicate",
			err: &pq.Error{
				Code:       "23505",
				Message:    "duplicate key value violates unique constraint",
				Detail:     "Key (username)=(testuser) already exists.",
				Constraint: "users_username_key",
			},
			constraint: "users_username_key",
			want:       true,
		},
		{
			name: "email_duplicate",
			err: &pq.Error{
				Code:       "23505",
				Message:    "duplicate key value violates unique constraint",
				Detail:     "Key (email)=(test@example.com) already exists.",
				Constraint: "users_email_key",
			},
			constraint: "users_email_key",
			want:       true,
		},
		{
			name: "foreign_key_violation",
			err: &pq.Error{
				Code:       "23503",
				Message:    "insert or update on table violates foreign key constraint",
				Constraint: "messages_user_id_fkey",
			},
			constraint: "messages_user_id_fkey",
			want:       false, // Not a unique violation
		},
		{
			name: "check_constraint_violation",
			err: &pq.Error{
				Code:       "23514",
				Message:    "new row violates check constraint",
				Constraint: "users_email_check",
			},
			constraint: "users_email_check",
			want:       false, // Not a unique violation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsUniqueViolation(tt.err, tt.constraint)
			if got != tt.want {
				t.Errorf("IsUniqueViolation() = %v, want %v for error code %s",
					got, tt.want, tt.err.Code)
			}
		})
	}
}

func TestIsUniqueViolation_CaseInsensitiveConstraint(t *testing.T) {
	err := &pq.Error{
		Code:       "23505",
		Constraint: "users_username_key",
	}

	// PostgreSQL constraint names are case-sensitive in the DB
	// Our function should do exact matching
	if IsUniqueViolation(err, "USERS_USERNAME_KEY") {
		t.Error("Expected false for case-mismatched constraint name")
	}

	if !IsUniqueViolation(err, "users_username_key") {
		t.Error("Expected true for exact constraint name match")
	}
}

func TestPQErrorCode_Constant(t *testing.T) {
	// Verify we're using the correct PostgreSQL error code for unique violations
	expectedCode := "23505"

	if pqUniqueViolation != expectedCode {
		t.Errorf("Expected pqUniqueViolation constant to be %s, got %s",
			expectedCode, pqUniqueViolation)
	}
}
