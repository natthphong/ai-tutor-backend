-- ============================================
-- AI Tutor Loop - Tutor Intelligence Upgrade
-- V010: intent history, resume state, richer speaking feedback
-- ============================================

ALTER TABLE tutor_messages
    ADD COLUMN IF NOT EXISTS unit_id INT REFERENCES lesson_units(id),
    ADD COLUMN IF NOT EXISTS intent TEXT,
    ADD COLUMN IF NOT EXISTS score NUMERIC(5,2);

CREATE INDEX IF NOT EXISTS idx_tutor_messages_user_unit_created
    ON tutor_messages(user_id, unit_id, created_at DESC);

ALTER TABLE tutor_sessions
    ADD COLUMN IF NOT EXISTS resume_state JSONB DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT now();

CREATE INDEX IF NOT EXISTS idx_tutor_sessions_user_unit_updated
    ON tutor_sessions(user_id, unit_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS tutor_turn_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID NOT NULL REFERENCES tutor_sessions(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES tutor_users(id) ON DELETE CASCADE,
    unit_id INT REFERENCES lesson_units(id),
    raw_input TEXT,
    input_kind TEXT DEFAULT 'text',
    client_action TEXT,
    classified_intent TEXT NOT NULL,
    confidence NUMERIC(5,2) DEFAULT 0,
    action_taken TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_tutor_turn_events_session_created
    ON tutor_turn_events(session_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_tutor_turn_events_user_unit_created
    ON tutor_turn_events(user_id, unit_id, created_at DESC);

ALTER TABLE speaking_attempts
    ADD COLUMN IF NOT EXISTS pronunciation_score NUMERIC(5,2),
    ADD COLUMN IF NOT EXISTS fluency_score NUMERIC(5,2),
    ADD COLUMN IF NOT EXISTS grammar_score NUMERIC(5,2),
    ADD COLUMN IF NOT EXISTS native_suggestion TEXT;
