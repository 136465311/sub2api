-- Phase 1 user AI bridge support.
-- Internal keys are real user-owned keys hidden from normal key management,
-- so user AI requests can reuse the existing /v1 gateway billing path.

ALTER TABLE api_keys
ADD COLUMN IF NOT EXISTS source VARCHAR(32) NOT NULL DEFAULT 'user';

CREATE INDEX IF NOT EXISTS idx_api_keys_user_group_source_active
ON api_keys (user_id, group_id, source)
WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_user_ai_internal_unique
ON api_keys (user_id, COALESCE(group_id, 0), source)
WHERE deleted_at IS NULL AND source = 'user_ai';

CREATE TABLE IF NOT EXISTS chat_conversations (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL,
    title VARCHAR(200) NOT NULL DEFAULT '',
    model VARCHAR(100) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_chat_conversations_user_active
ON chat_conversations (user_id, updated_at DESC)
WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS chat_messages (
    id BIGSERIAL PRIMARY KEY,
    conversation_id BIGINT NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    model VARCHAR(100) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_chat_messages_conversation_created
ON chat_messages (conversation_id, created_at ASC, id ASC);

CREATE INDEX IF NOT EXISTS idx_chat_messages_user_created
ON chat_messages (user_id, created_at DESC);
