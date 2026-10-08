-- +goose Up
-- Reviewer edits to a channel's rules (title length, required attributes,
-- banned words), one row per save so who changed what, and when, is kept.
-- The latest row per channel overrides the channel file's rules; the file
-- stays the starting point. A new table, so no lock on an existing one.
-- Index decision: idx_channel_rule_edits_channel_id serves "latest edit per
-- channel" (DISTINCT ON channel ORDER BY channel, id DESC), read at API
-- start-up, after each save and before each worker job.
CREATE TABLE channel_rule_edits (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    channel text NOT NULL,
    title_max_length integer NOT NULL,
    required_attributes text[] NOT NULL,
    banned_words text[] NOT NULL,
    edited_by bigint NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_channel_rule_edits_channel_shape CHECK (channel ~ '^[a-z][a-z0-9_]{1,39}$'),
    CONSTRAINT chk_channel_rule_edits_title_range CHECK (title_max_length BETWEEN 10 AND 500),
    CONSTRAINT chk_channel_rule_edits_attributes CHECK (required_attributes <@ ARRAY['colour', 'pattern', 'sleeve', 'neckline', 'fit']::text[]),
    CONSTRAINT chk_channel_rule_edits_banned_count CHECK (cardinality(banned_words) <= 200)
);
CREATE INDEX idx_channel_rule_edits_channel_id ON channel_rule_edits (channel, id DESC);
CREATE INDEX idx_channel_rule_edits_edited_by ON channel_rule_edits (edited_by);
COMMENT ON TABLE channel_rule_edits IS 'Reviewer edits to channel rules; the latest per channel is in force.';
COMMENT ON COLUMN channel_rule_edits.channel IS 'Channel id from the channel file.';
COMMENT ON COLUMN channel_rule_edits.title_max_length IS 'Longest title allowed, in characters.';
COMMENT ON COLUMN channel_rule_edits.required_attributes IS 'Attributes a listing must have a known value for.';
COMMENT ON COLUMN channel_rule_edits.banned_words IS 'Words and phrases the rules engine refuses, lowercase.';
COMMENT ON COLUMN channel_rule_edits.edited_by IS 'The reviewer who saved the edit.';
COMMENT ON COLUMN channel_rule_edits.created_at IS 'When the edit was saved.';

-- +goose Down
DROP TABLE IF EXISTS channel_rule_edits;
