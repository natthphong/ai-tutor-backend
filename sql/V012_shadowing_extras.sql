-- ============================================
-- AI Tutor Loop - Shadowing extras
-- V012: folders, mark-as-watched, completed_at on progress,
--       resume "continue watching" flow
-- ============================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS shadowing_folders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES tutor_users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    color TEXT,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_shadowing_folders_user ON shadowing_folders(user_id);

ALTER TABLE shadowing_clips
    ADD COLUMN IF NOT EXISTS folder_id UUID REFERENCES shadowing_folders(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS watched_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS is_completed BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_shadowing_clips_folder ON shadowing_clips(folder_id);

ALTER TABLE shadowing_progress
    ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ;
