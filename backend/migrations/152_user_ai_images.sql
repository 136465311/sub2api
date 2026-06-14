-- Phase 3 user AI image generation history.
-- Generation requests still run through hidden user-owned keys and the existing
-- OpenAI images gateway billing path; this table only stores the in-site view.

CREATE TABLE IF NOT EXISTS image_generation_history (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL,
    prompt TEXT NOT NULL DEFAULT '',
    model VARCHAR(100) NOT NULL DEFAULT '',
    size VARCHAR(32) NOT NULL DEFAULT '',
    n INTEGER NOT NULL DEFAULT 1,
    images JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_image_generation_history_user_created
ON image_generation_history (user_id, created_at DESC, id DESC);
