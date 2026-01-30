-- Insert test users (password for all: "strange!")
-- Bcrypt hash generated with cost 10
INSERT INTO users (id, username, email, password_hash, created_at) VALUES
    ('11111111-1111-1111-1111-111111111111', 'daniel', 'daniel@test.com', '$2a$10$JKgx8VGMuMXHwLaHbbI4G.6vlU/9TSR3R7deCAAWbu8CpHT/.1p/e', NOW()),
    ('22222222-2222-2222-2222-222222222222', 'bob', 'bob@test.com', '$2a$10$JKgx8VGMuMXHwLaHbbI4G.6vlU/9TSR3R7deCAAWbu8CpHT/.1p/e', NOW()),
    ('33333333-3333-3333-3333-333333333333', 'charlie', 'charlie@test.com', '$2a$10$JKgx8VGMuMXHwLaHbbI4G.6vlU/9TSR3R7deCAAWbu8CpHT/.1p/e', NOW()),
    ('44444444-4444-4444-4444-444444444444', 'diana', 'diana@test.com', '$2a$10$JKgx8VGMuMXHwLaHbbI4G.6vlU/9TSR3R7deCAAWbu8CpHT/.1p/e', NOW()),
    ('55555555-5555-5555-5555-555555555555', 'eve', 'eve@test.com', '$2a$10$JKgx8VGMuMXHwLaHbbI4G.6vlU/9TSR3R7deCAAWbu8CpHT/.1p/e', NOW())
ON CONFLICT (username) DO NOTHING;

-- Insert default chatrooms
INSERT INTO chatrooms (id, name, created_by, created_at) VALUES
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'General', '11111111-1111-1111-1111-111111111111', NOW()),
    ('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'Random', '22222222-2222-2222-2222-222222222222', NOW()),
    ('cccccccc-cccc-cccc-cccc-cccccccccccc', 'Tech Talk', '33333333-3333-3333-3333-333333333333', NOW())
ON CONFLICT (id) DO NOTHING;

-- Add all users to General chatroom
INSERT INTO chatroom_members (user_id, chatroom_id, joined_at) VALUES
    ('11111111-1111-1111-1111-111111111111', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', NOW()),
    ('22222222-2222-2222-2222-222222222222', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', NOW()),
    ('33333333-3333-3333-3333-333333333333', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', NOW()),
    ('44444444-4444-4444-4444-444444444444', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', NOW()),
    ('55555555-5555-5555-5555-555555555555', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', NOW())
ON CONFLICT (user_id, chatroom_id) DO NOTHING;

-- Add some users to Random chatroom
INSERT INTO chatroom_members (user_id, chatroom_id, joined_at) VALUES
    ('11111111-1111-1111-1111-111111111111', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', NOW()),
    ('22222222-2222-2222-2222-222222222222', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', NOW()),
    ('33333333-3333-3333-3333-333333333333', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', NOW())
ON CONFLICT (user_id, chatroom_id) DO NOTHING;

-- Add some users to Tech Talk chatroom
INSERT INTO chatroom_members (user_id, chatroom_id, joined_at) VALUES
    ('11111111-1111-1111-1111-111111111111', 'cccccccc-cccc-cccc-cccc-cccccccccccc', NOW()),
    ('33333333-3333-3333-3333-333333333333', 'cccccccc-cccc-cccc-cccc-cccccccccccc', NOW()),
    ('44444444-4444-4444-4444-444444444444', 'cccccccc-cccc-cccc-cccc-cccccccccccc', NOW())
ON CONFLICT (user_id, chatroom_id) DO NOTHING;

-- Insert some sample messages
INSERT INTO messages (chatroom_id, user_id, content, is_bot, created_at) VALUES
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '11111111-1111-1111-1111-111111111111', 'Hey everyone! 👋', false, NOW() - INTERVAL '5 minutes'),
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '22222222-2222-2222-2222-222222222222', 'Hi Daniel! How are you doing?', false, NOW() - INTERVAL '4 minutes'),
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '33333333-3333-3333-3333-333333333333', 'Welcome! Try the /stock command to get stock quotes', false, NOW() - INTERVAL '3 minutes'),
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '11111111-1111-1111-1111-111111111111', 'Thanks! Let me try: /stock=AAPL.US', false, NOW() - INTERVAL '2 minutes'),
    ('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', '22222222-2222-2222-2222-222222222222', 'Anyone here?', false, NOW() - INTERVAL '10 minutes'),
    ('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', '33333333-3333-3333-3333-333333333333', 'Yep! Just chilling', false, NOW() - INTERVAL '8 minutes'),
    ('cccccccc-cccc-cccc-cccc-cccccccccccc', '33333333-3333-3333-3333-333333333333', 'What are you all working on?', false, NOW() - INTERVAL '15 minutes'),
    ('cccccccc-cccc-cccc-cccc-cccccccccccc', '44444444-4444-4444-4444-444444444444', 'Building a chat app with Go!', false, NOW() - INTERVAL '12 minutes')
ON CONFLICT (id) DO NOTHING;
