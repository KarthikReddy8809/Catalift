-- +goose Up
-- Brand voice settings (brief: "tone, words to avoid"): voice_note stays the
-- tone; words_to_avoid lists what the brand never says. Generation is told to
-- avoid them and the rules engine flags any that appear (brand_avoid_word).
-- Exports a reviewer sends to the seller: sent_at and sent_by are set together
-- once, and only a sent export's files can be downloaded by a seller.
-- Lock: ADD COLUMN with a constant default is a catalogue-only change on
-- PostgreSQL 11 and newer; brands and exports hold tens of rows (10^1).
ALTER TABLE brands
    ADD COLUMN words_to_avoid text[] NOT NULL DEFAULT '{}',
    ADD CONSTRAINT chk_brands_words_to_avoid_count CHECK (cardinality(words_to_avoid) <= 50);
COMMENT ON COLUMN brands.words_to_avoid IS 'Words or phrases the brand never uses; checked on every listing of its products.';

ALTER TABLE exports
    ADD COLUMN sent_at timestamptz,
    ADD COLUMN sent_by bigint REFERENCES users (id) ON DELETE RESTRICT,
    ADD CONSTRAINT chk_exports_sent_together CHECK ((sent_at IS NULL) = (sent_by IS NULL));
COMMENT ON COLUMN exports.sent_at IS 'When a reviewer sent the export to the seller; null until sent.';
COMMENT ON COLUMN exports.sent_by IS 'The reviewer who sent it; null until sent.';
-- FK index: no scan when a user row is checked on delete.
CREATE INDEX idx_exports_sent_by ON exports (sent_by);
-- The seller's list of received exports, newest first.
CREATE INDEX idx_exports_sent_at ON exports (sent_at DESC) WHERE sent_at IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_exports_sent_at;
DROP INDEX IF EXISTS idx_exports_sent_by;
ALTER TABLE exports
    DROP CONSTRAINT IF EXISTS chk_exports_sent_together,
    DROP COLUMN IF EXISTS sent_by,
    DROP COLUMN IF EXISTS sent_at;
ALTER TABLE brands
    DROP CONSTRAINT IF EXISTS chk_brands_words_to_avoid_count,
    DROP COLUMN IF EXISTS words_to_avoid;
