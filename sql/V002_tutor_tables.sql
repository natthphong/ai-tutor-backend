-- ============================================
-- AI Tutor Loop - Database Schema
-- V002: Tutor System Tables
-- ============================================

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================
-- 1. tutor_users - User profiles with LINE integration
-- ============================================
CREATE TABLE IF NOT EXISTS tutor_users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    line_user_id VARCHAR(100) UNIQUE,
    display_name TEXT NOT NULL DEFAULT 'Learner',
    preferred_language TEXT DEFAULT 'th',
    current_unit_id INT DEFAULT 1,
    current_level TEXT DEFAULT 'A1',
    streak_count INT DEFAULT 0,
    last_active_at TIMESTAMPTZ,
    settings JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_tutor_users_line_id ON tutor_users(line_user_id);

-- ============================================
-- 2. lesson_units - 145 grammar units from lecture/
-- ============================================
CREATE TABLE IF NOT EXISTS lesson_units (
    id SERIAL PRIMARY KEY,
    unit_no INT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    source_path TEXT NOT NULL,
    summary TEXT,
    grammar_focus TEXT,
    grammar_pattern TEXT,
    level TEXT DEFAULT 'A1',
    status TEXT DEFAULT 'active',
    raw_content TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_lesson_units_unit_no ON lesson_units(unit_no);
CREATE INDEX idx_lesson_units_level ON lesson_units(level);

-- ============================================
-- 3. lesson_items - Items within each unit
-- ============================================
CREATE TABLE IF NOT EXISTS lesson_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    unit_id INT NOT NULL REFERENCES lesson_units(id) ON DELETE CASCADE,
    item_type TEXT NOT NULL,
    title TEXT,
    content TEXT NOT NULL,
    content_th TEXT,
    pattern TEXT,
    difficulty TEXT DEFAULT 'A1',
    sort_order INT DEFAULT 0,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- item_type values:
-- grammar_explanation, example_sentence, listening_sentence,
-- speaking_pattern, speaking_situation, reading_passage, quiz

CREATE INDEX idx_lesson_items_unit_id ON lesson_items(unit_id);
CREATE INDEX idx_lesson_items_type ON lesson_items(item_type);

-- ============================================
-- 4. user_unit_progress - Per-user learning state
-- ============================================
CREATE TABLE IF NOT EXISTS user_unit_progress (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES tutor_users(id) ON DELETE CASCADE,
    unit_id INT NOT NULL REFERENCES lesson_units(id) ON DELETE CASCADE,
    status TEXT DEFAULT 'not_started',
    current_step TEXT DEFAULT 'intro',
    mastery_score NUMERIC(5,2) DEFAULT 0,
    listening_score NUMERIC(5,2) DEFAULT 0,
    speaking_score NUMERIC(5,2) DEFAULT 0,
    reading_score NUMERIC(5,2) DEFAULT 0,
    attempt_count INT DEFAULT 0,
    completed_at TIMESTAMPTZ,
    next_due_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(user_id, unit_id)
);

-- status values: not_started, in_progress, review_due, completed, mastered
-- current_step values: intro, grammar_explanation, listening_practice,
--   speaking_practice, reading_practice, mini_quiz, review_summary, schedule_review

CREATE INDEX idx_user_unit_progress_user ON user_unit_progress(user_id);
CREATE INDEX idx_user_unit_progress_status ON user_unit_progress(status);
CREATE INDEX idx_user_unit_progress_due ON user_unit_progress(next_due_at);

-- ============================================
-- 5. tutor_sessions - Active learning sessions
-- ============================================
CREATE TABLE IF NOT EXISTS tutor_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES tutor_users(id) ON DELETE CASCADE,
    unit_id INT REFERENCES lesson_units(id),
    mode TEXT NOT NULL DEFAULT 'mixed',
    status TEXT DEFAULT 'active',
    current_action TEXT,
    current_item_id UUID,
    started_at TIMESTAMPTZ DEFAULT now(),
    ended_at TIMESTAMPTZ,
    metadata JSONB DEFAULT '{}'
);

-- mode values: mixed, listening, speaking, reading, review, vocabulary_review, weakness_review
-- status values: active, paused, completed

CREATE INDEX idx_tutor_sessions_user ON tutor_sessions(user_id);
CREATE INDEX idx_tutor_sessions_status ON tutor_sessions(status);

-- ============================================
-- 6. tutor_messages - Chat history
-- ============================================
CREATE TABLE IF NOT EXISTS tutor_messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID NOT NULL REFERENCES tutor_sessions(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES tutor_users(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    content TEXT,
    content_th TEXT,
    audio_object_key TEXT,
    message_type TEXT DEFAULT 'text',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT now()
);

-- role values: system, assistant, user, tool
-- message_type values: text, audio, correction, hint, quiz, summary

CREATE INDEX idx_tutor_messages_session ON tutor_messages(session_id);
CREATE INDEX idx_tutor_messages_created ON tutor_messages(created_at);

-- ============================================
-- 7. listening_attempts - Listening exercise results
-- ============================================
CREATE TABLE IF NOT EXISTS listening_attempts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES tutor_users(id) ON DELETE CASCADE,
    session_id UUID NOT NULL REFERENCES tutor_sessions(id) ON DELETE CASCADE,
    unit_id INT NOT NULL REFERENCES lesson_units(id),
    lesson_item_id UUID REFERENCES lesson_items(id),
    target_text TEXT NOT NULL,
    user_text TEXT NOT NULL,
    score NUMERIC(5,2),
    is_correct BOOLEAN DEFAULT false,
    hint_level INT DEFAULT 0,
    mistakes JSONB DEFAULT '[]',
    attempt_number INT DEFAULT 1,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_listening_attempts_user ON listening_attempts(user_id);
CREATE INDEX idx_listening_attempts_session ON listening_attempts(session_id);

-- ============================================
-- 8. speaking_attempts - Speaking exercise results
-- ============================================
CREATE TABLE IF NOT EXISTS speaking_attempts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES tutor_users(id) ON DELETE CASCADE,
    session_id UUID NOT NULL REFERENCES tutor_sessions(id) ON DELETE CASCADE,
    unit_id INT NOT NULL REFERENCES lesson_units(id),
    lesson_item_id UUID REFERENCES lesson_items(id),
    audio_object_key TEXT,
    transcript TEXT,
    target_pattern TEXT,
    expected_answer TEXT,
    score NUMERIC(5,2),
    feedback_th TEXT,
    correction_text TEXT,
    mistakes JSONB DEFAULT '[]',
    stt_provider TEXT,
    stt_model TEXT,
    attempt_number INT DEFAULT 1,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_speaking_attempts_user ON speaking_attempts(user_id);
CREATE INDEX idx_speaking_attempts_session ON speaking_attempts(session_id);

-- ============================================
-- 9. reading_attempts - Reading exercise results
-- ============================================
CREATE TABLE IF NOT EXISTS reading_attempts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES tutor_users(id) ON DELETE CASCADE,
    session_id UUID NOT NULL REFERENCES tutor_sessions(id) ON DELETE CASCADE,
    unit_id INT NOT NULL REFERENCES lesson_units(id),
    lesson_item_id UUID REFERENCES lesson_items(id),
    passage TEXT NOT NULL,
    user_translation TEXT,
    ai_translation TEXT,
    score NUMERIC(5,2),
    feedback_th TEXT,
    vocabulary JSONB DEFAULT '[]',
    mistakes JSONB DEFAULT '[]',
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_reading_attempts_user ON reading_attempts(user_id);
CREATE INDEX idx_reading_attempts_session ON reading_attempts(session_id);

-- ============================================
-- 10. weaknesses - User weakness tracking
-- ============================================
CREATE TABLE IF NOT EXISTS weaknesses (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES tutor_users(id) ON DELETE CASCADE,
    unit_id INT REFERENCES lesson_units(id),
    source_type TEXT NOT NULL,
    source_id UUID,
    weakness_type TEXT NOT NULL,
    weakness_code TEXT,
    detail TEXT,
    example_wrong TEXT,
    example_correct TEXT,
    severity TEXT DEFAULT 'medium',
    mastery_score NUMERIC(5,2) DEFAULT 0,
    next_due_at TIMESTAMPTZ,
    review_count INT DEFAULT 0,
    last_reviewed_at TIMESTAMPTZ,
    resolved BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- weakness_type values: grammar, vocabulary, word_order, pronunciation,
--   listening, translation, article, preposition, tense
-- severity values: low, medium, high, critical

CREATE INDEX idx_weaknesses_user ON weaknesses(user_id);
CREATE INDEX idx_weaknesses_due ON weaknesses(next_due_at);
CREATE INDEX idx_weaknesses_type ON weaknesses(weakness_type);

-- ============================================
-- 11. flashcards - SRS vocabulary/grammar cards
-- ============================================
CREATE TABLE IF NOT EXISTS tutor_flashcards (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES tutor_users(id) ON DELETE CASCADE,
    unit_id INT REFERENCES lesson_units(id),
    card_type TEXT NOT NULL DEFAULT 'vocabulary',
    front TEXT NOT NULL,
    back TEXT NOT NULL,
    example TEXT,
    example_th TEXT,
    source_type TEXT,
    source_id UUID,
    mastery_score NUMERIC(5,2) DEFAULT 0,
    ease_factor NUMERIC(5,2) DEFAULT 2.5,
    interval_days INT DEFAULT 0,
    next_due_at TIMESTAMPTZ DEFAULT now(),
    review_count INT DEFAULT 0,
    consecutive_correct INT DEFAULT 0,
    last_reviewed_at TIMESTAMPTZ,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- card_type values: vocabulary, grammar_pattern, weakness, sentence, phrase

CREATE INDEX idx_tutor_flashcards_user ON tutor_flashcards(user_id);
CREATE INDEX idx_tutor_flashcards_due ON tutor_flashcards(next_due_at);
CREATE INDEX idx_tutor_flashcards_unit ON tutor_flashcards(unit_id);

-- ============================================
-- 12. reviews - Scheduled review items
-- ============================================
CREATE TABLE IF NOT EXISTS reviews (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES tutor_users(id) ON DELETE CASCADE,
    review_type TEXT NOT NULL,
    target_id UUID NOT NULL,
    target_table TEXT NOT NULL,
    due_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    score NUMERIC(5,2),
    result TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT now()
);

-- review_type values: flashcard, weakness, unit
-- result values: pass, fail, skip

CREATE INDEX idx_reviews_user ON reviews(user_id);
CREATE INDEX idx_reviews_due ON reviews(due_at);
CREATE INDEX idx_reviews_type ON reviews(review_type);

-- ============================================
-- 13. embeddings - Content embeddings for search
-- ============================================
CREATE TABLE IF NOT EXISTS embeddings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_type TEXT NOT NULL,
    source_id TEXT NOT NULL,
    content TEXT NOT NULL,
    content_hash TEXT NOT NULL UNIQUE,
    embedding_blob BYTEA NOT NULL,
    dimensions INT NOT NULL,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_embeddings_source ON embeddings(source_type, source_id);
CREATE INDEX idx_embeddings_hash ON embeddings(content_hash);

-- ============================================
-- 14. ai_call_logs - AI API usage tracking
-- ============================================
CREATE TABLE IF NOT EXISTS ai_call_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID,
    session_id UUID,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    use_case TEXT NOT NULL,
    prompt_hash TEXT,
    input_tokens INT DEFAULT 0,
    output_tokens INT DEFAULT 0,
    total_tokens INT DEFAULT 0,
    latency_ms INT DEFAULT 0,
    cost_usd NUMERIC(10,6) DEFAULT 0,
    status TEXT DEFAULT 'success',
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_ai_call_logs_user ON ai_call_logs(user_id);
CREATE INDEX idx_ai_call_logs_provider ON ai_call_logs(provider);
CREATE INDEX idx_ai_call_logs_created ON ai_call_logs(created_at);
CREATE INDEX idx_ai_call_logs_use_case ON ai_call_logs(use_case);
