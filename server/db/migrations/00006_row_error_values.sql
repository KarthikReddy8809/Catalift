-- +goose Up
-- The values a rejected CSV row had, so the seller can correct them in the
-- app and send the row again (seller flow step 2). Nullable: rows rejected
-- before this migration, and rows that could not be read as CSV, have none.
-- Lock: ADD COLUMN with no default is a catalogue-only change; the table
-- holds at most a few hundred rows per upload at demo scale (10^3).
-- Index decision: idx_upload_row_errors_upload_id_row_number serves the
-- correction's lookup and delete by (upload_id, row_number); the open-errors
-- list reads the whole small table in upload order.
ALTER TABLE upload_row_errors
    ADD COLUMN raw_category text,
    ADD COLUMN raw_brand text,
    ADD COLUMN raw_price text,
    ADD CONSTRAINT chk_upload_row_errors_raw_length CHECK (
        coalesce(length(raw_category), 0) <= 100 AND coalesce(length(raw_brand), 0) <= 100
        AND coalesce(length(raw_price), 0) <= 100);
CREATE INDEX idx_upload_row_errors_upload_id_row_number ON upload_row_errors (upload_id, row_number);
COMMENT ON COLUMN upload_row_errors.raw_category IS 'The category as typed in the CSV.';
COMMENT ON COLUMN upload_row_errors.raw_brand IS 'The brand as typed in the CSV.';
COMMENT ON COLUMN upload_row_errors.raw_price IS 'The price as typed in the CSV.';

-- +goose Down
DROP INDEX IF EXISTS idx_upload_row_errors_upload_id_row_number;
ALTER TABLE upload_row_errors
    DROP CONSTRAINT IF EXISTS chk_upload_row_errors_raw_length,
    DROP COLUMN IF EXISTS raw_price,
    DROP COLUMN IF EXISTS raw_brand,
    DROP COLUMN IF EXISTS raw_category;
