-- Track whether an AI conversation title was generated from the first user turn.
-- This keeps generated titles stable and prevents later messages from renaming
-- the conversation automatically.

ALTER TABLE chat_conversations
ADD COLUMN IF NOT EXISTS title_generated BOOLEAN NOT NULL DEFAULT FALSE;

