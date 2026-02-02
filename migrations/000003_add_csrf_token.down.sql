-- Rollback: Remove CSRF token support
ALTER TABLE sessions DROP CONSTRAINT unique_csrf_token;
DROP INDEX idx_sessions_csrf_token;
ALTER TABLE sessions DROP COLUMN csrf_token;
