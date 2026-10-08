-- +goose Up
-- Every screen shows the latest upload only; older records stay in the
-- database (request 2026-10-08). An export now covers one upload, so the
-- seller's file holds exactly the launch they uploaded. Null for exports
-- written before this migration, which covered every upload.
-- Lock: ADD COLUMN with no default is a catalogue-only change; exports holds
-- tens of rows (10^1).
ALTER TABLE exports ADD COLUMN upload_id bigint REFERENCES uploads (id) ON DELETE RESTRICT;
COMMENT ON COLUMN exports.upload_id IS 'The upload whose approved listings the export holds; null for older exports that held every upload.';
-- FK index; also serves the export list filtered by upload.
CREATE INDEX idx_exports_upload_id ON exports (upload_id);

-- +goose Down
DROP INDEX IF EXISTS idx_exports_upload_id;
ALTER TABLE exports DROP COLUMN IF EXISTS upload_id;
