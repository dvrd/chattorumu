-- Drop tables in reverse order (respecting foreign keys)
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS chatroom_members;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS chatrooms;
DROP TABLE IF EXISTS users;

-- Drop extension
DROP EXTENSION IF EXISTS "pgcrypto";
