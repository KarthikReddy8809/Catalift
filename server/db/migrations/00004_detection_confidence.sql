-- +goose Up
-- The model's own 0 to 1 confidence in a product's detected attributes, so
-- the reviewer can triage low-confidence detections first (reviewer flow
-- step 2). Nullable: products detected before this migration, and products
-- whose attributes a reviewer corrected, have none.
-- Lock: ADD COLUMN with no default and NOT VALID-free CHECK on a nullable
-- column is a catalogue-only change; product_attributes holds at most a few
-- thousand rows at demo scale (10^3), so the brief ACCESS EXCLUSIVE lock is fine.
-- Index decision: none; the low-confidence filter runs over one upload's
-- products, already narrowed by the products index on upload_id.
ALTER TABLE product_attributes
    ADD COLUMN detection_confidence real,
    ADD CONSTRAINT chk_product_attributes_confidence_range
        CHECK (detection_confidence IS NULL OR detection_confidence BETWEEN 0 AND 1);
COMMENT ON COLUMN product_attributes.detection_confidence IS
    'The model''s confidence in the detected attributes, 0 to 1; null when not detected by enrich-v1 or corrected by a reviewer.';

-- +goose Down
ALTER TABLE product_attributes
    DROP CONSTRAINT IF EXISTS chk_product_attributes_confidence_range,
    DROP COLUMN IF EXISTS detection_confidence;
