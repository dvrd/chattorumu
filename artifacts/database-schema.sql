-- Jobsity Chat Database Schema
-- PostgreSQL 15+

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(50) UNIQUE NOT NULL CHECK (length(username) >= 3),
    email VARCHAR(255) UNIQUE NOT NULL CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$'),
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- Indexes for users
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_email ON users(email);

-- Chatrooms table
CREATE TABLE chatrooms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL CHECK (length(name) >= 1),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
);

-- Indexes for chatrooms
CREATE INDEX idx_chatrooms_name ON chatrooms(name);
CREATE INDEX idx_chatrooms_created_by ON chatrooms(created_by);

-- Messages table
CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chatroom_id UUID NOT NULL REFERENCES chatrooms(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL CHECK (length(content) > 0 AND length(content) <= 1000),
    is_bot BOOLEAN DEFAULT FALSE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- Indexes for messages (optimized for "last 50 messages" query)
CREATE INDEX idx_messages_chatroom_created ON messages(chatroom_id, created_at DESC);
CREATE INDEX idx_messages_created_at ON messages(created_at DESC);
CREATE INDEX idx_messages_user ON messages(user_id);

-- Chatroom members (many-to-many relationship)
CREATE TABLE chatroom_members (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    chatroom_id UUID NOT NULL REFERENCES chatrooms(id) ON DELETE CASCADE,
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (user_id, chatroom_id)
);

-- Indexes for chatroom members
CREATE INDEX idx_members_chatroom ON chatroom_members(chatroom_id);
CREATE INDEX idx_members_user ON chatroom_members(user_id);

-- Sessions table (for session-based authentication)
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- Indexes for sessions
CREATE INDEX idx_sessions_token ON sessions(token);
CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

-- Function to clean up expired sessions
CREATE OR REPLACE FUNCTION cleanup_expired_sessions()
RETURNS void AS $$
BEGIN
    DELETE FROM sessions WHERE expires_at < CURRENT_TIMESTAMP;
END;
$$ LANGUAGE plpgsql;

-- Common queries for reference

-- Get last 50 messages for a chatroom (ordered by timestamp)
-- SELECT m.id, m.content, m.created_at, m.is_bot, u.username, u.id as user_id
-- FROM messages m
-- JOIN users u ON m.user_id = u.id
-- WHERE m.chatroom_id = $1
-- ORDER BY m.created_at DESC
-- LIMIT 50;

-- Get chatrooms for a user
-- SELECT c.id, c.name, c.created_at
-- FROM chatrooms c
-- JOIN chatroom_members cm ON c.id = cm.chatroom_id
-- WHERE cm.user_id = $1
-- ORDER BY c.created_at DESC;

-- Check if user is member of chatroom
-- SELECT EXISTS(
--     SELECT 1 FROM chatroom_members
--     WHERE user_id = $1 AND chatroom_id = $2
-- );

-- Insert message
-- INSERT INTO messages (chatroom_id, user_id, content, is_bot)
-- VALUES ($1, $2, $3, $4)
-- RETURNING id, created_at;

-- Validate session
-- SELECT s.id, s.user_id, u.username
-- FROM sessions s
-- JOIN users u ON s.user_id = u.id
-- WHERE s.token = $1 AND s.expires_at > CURRENT_TIMESTAMP;
