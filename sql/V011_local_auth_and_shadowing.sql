-- ============================================
-- AI Tutor Loop - Local Auth + Shadowing Mode
-- V011: local users, chat-history convenience indexes,
--       shadowing clips/segments/progress/recordings/notes
-- ============================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================
-- Local password-based login (for dev/QA/agents)
-- ============================================

-- Tutor users gain optional username/password fields. A user can have either a
-- line_user_id (LINE login) or a username (local login), or both. line_user_id
-- is already UNIQUE-but-nullable from V002.
ALTER TABLE tutor_users
    ADD COLUMN IF NOT EXISTS username TEXT UNIQUE,
    ADD COLUMN IF NOT EXISTS password_hash TEXT,
    ADD COLUMN IF NOT EXISTS auth_kind TEXT DEFAULT 'line';

CREATE INDEX IF NOT EXISTS idx_tutor_users_username ON tutor_users(username);

-- Dev seed user (test / test1234). Password hash is bcrypt cost 10. The hash
-- below corresponds to the plaintext "test1234".
INSERT INTO tutor_users (id, username, display_name, auth_kind, password_hash)
VALUES (
    uuid_generate_v4(),
    'test',
    'Test User',
    'local',
    '$2a$10$LWdGuw5pj8RrIDP9aafpAOAJyzgEZWihJtsVfddyXFB.PjLMmL2HC'
)
ON CONFLICT (username) DO NOTHING;

-- ============================================
-- Shadowing Mode (parroto.app-style YouTube practice)
-- ============================================

CREATE TABLE IF NOT EXISTS shadowing_clips (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES tutor_users(id) ON DELETE CASCADE,
    youtube_url TEXT NOT NULL,
    youtube_id TEXT,
    title TEXT,
    thumbnail_url TEXT,
    minio_object_key TEXT,
    stream_url TEXT,
    duration_seconds INT DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',
    error_message TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- status values: pending, processing, ready, failed
CREATE INDEX IF NOT EXISTS idx_shadowing_clips_user ON shadowing_clips(user_id);
CREATE INDEX IF NOT EXISTS idx_shadowing_clips_status ON shadowing_clips(status);

CREATE TABLE IF NOT EXISTS shadowing_segments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    clip_id UUID NOT NULL REFERENCES shadowing_clips(id) ON DELETE CASCADE,
    idx INT NOT NULL,
    start_time NUMERIC(10,3) NOT NULL DEFAULT 0,
    end_time NUMERIC(10,3) NOT NULL DEFAULT 0,
    text TEXT NOT NULL,
    thai_translation TEXT,
    ipa TEXT,
    created_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(clip_id, idx)
);
CREATE INDEX IF NOT EXISTS idx_shadowing_segments_clip ON shadowing_segments(clip_id, idx);

CREATE TABLE IF NOT EXISTS shadowing_progress (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES tutor_users(id) ON DELETE CASCADE,
    clip_id UUID NOT NULL REFERENCES shadowing_clips(id) ON DELETE CASCADE,
    current_segment_index INT DEFAULT 0,
    last_watched_time NUMERIC(10,3) DEFAULT 0,
    completed_segments JSONB DEFAULT '[]',
    updated_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(user_id, clip_id)
);
CREATE INDEX IF NOT EXISTS idx_shadowing_progress_user ON shadowing_progress(user_id);

CREATE TABLE IF NOT EXISTS shadowing_recordings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES tutor_users(id) ON DELETE CASCADE,
    clip_id UUID NOT NULL REFERENCES shadowing_clips(id) ON DELETE CASCADE,
    segment_id UUID REFERENCES shadowing_segments(id) ON DELETE SET NULL,
    audio_object_key TEXT NOT NULL,
    audio_url TEXT,
    duration_seconds NUMERIC(10,3) DEFAULT 0,
    score NUMERIC(5,2),
    ai_feedback TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_shadowing_recordings_user ON shadowing_recordings(user_id);
CREATE INDEX IF NOT EXISTS idx_shadowing_recordings_clip_seg ON shadowing_recordings(clip_id, segment_id);

CREATE TABLE IF NOT EXISTS shadowing_notes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES tutor_users(id) ON DELETE CASCADE,
    clip_id UUID NOT NULL REFERENCES shadowing_clips(id) ON DELETE CASCADE,
    segment_id UUID REFERENCES shadowing_segments(id) ON DELETE SET NULL,
    note_text TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_shadowing_notes_user_clip ON shadowing_notes(user_id, clip_id);
