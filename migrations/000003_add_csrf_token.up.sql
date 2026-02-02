-- Add CSRF token column to sessions table
-- Allows per-session CSRF token protection
ALTER TABLE sessions ADD COLUMN csrf_token VARCHAR(64);

-- Create index for faster lookups during validation
CREATE INDEX idx_sessions_csrf_token ON sessions(csrf_token);

-- Add unique constraint to prevent token reuse across sessions
ALTER TABLE sessions ADD CONSTRAINT unique_csrf_token UNIQUE (csrf_token);
