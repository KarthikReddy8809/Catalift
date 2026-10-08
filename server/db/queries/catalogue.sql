-- name: UpsertBrand :one
INSERT INTO brands (name) VALUES (btrim($1))
ON CONFLICT ((lower(btrim(name)))) DO UPDATE SET updated_at = brands.updated_at
RETURNING id;

-- name: ListBrands :many
SELECT id, name, voice_note, created_at, updated_at, words_to_avoid
FROM brands
WHERE lower(name) > lower(sqlc.arg(after_name)::text)
  AND (sqlc.narg(upload_id)::bigint IS NULL
       OR EXISTS (SELECT 1 FROM products p WHERE p.brand_id = brands.id AND p.upload_id = sqlc.narg(upload_id)))
ORDER BY lower(name)
LIMIT sqlc.arg(page_size);

-- name: UpdateBrandVoice :one
-- A null words_to_avoid keeps the stored list.
UPDATE brands SET voice_note = sqlc.narg(voice_note),
    words_to_avoid = coalesce(sqlc.narg(words_to_avoid)::text[], words_to_avoid),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, name, voice_note, created_at, updated_at, words_to_avoid;

-- name: AvoidWordsForProduct :one
SELECT b.words_to_avoid FROM products p JOIN brands b ON b.id = p.brand_id WHERE p.id = $1;

-- name: GeneratedListingsOfBrand :many
SELECT l.id FROM listings l JOIN products p ON p.id = l.product_id
WHERE p.brand_id = $1 AND l.status = 'generated'
ORDER BY l.id;

-- name: BrandsWithoutVoice :many
SELECT DISTINCT b.name
FROM brands b
JOIN products p ON p.brand_id = b.id
WHERE b.voice_note IS NULL
  AND (sqlc.narg(upload_id)::bigint IS NULL OR p.upload_id = sqlc.narg(upload_id))
ORDER BY b.name;

-- name: CreateUpload :one
INSERT INTO uploads (uploaded_by, file_name, rows_total, rows_accepted, rows_rejected)
VALUES ($1, $2, 0, 0, 0)
RETURNING id;

-- name: FinishUpload :exec
UPDATE uploads
SET rows_total = $2, rows_accepted = $3, rows_rejected = $4, updated_at = now()
WHERE id = $1;

-- name: InsertRowError :exec
INSERT INTO upload_row_errors (upload_id, row_number, sku, reason, raw_category, raw_brand, raw_price)
VALUES ($1, $2, $3, $4, sqlc.narg(raw_category), sqlc.narg(raw_brand), sqlc.narg(raw_price));

-- name: ListOpenRowErrors :many
SELECT e.upload_id, u.file_name, e.row_number, e.sku, e.reason, e.raw_category, e.raw_brand, e.raw_price
FROM upload_row_errors e
JOIN uploads u ON u.id = e.upload_id
WHERE (sqlc.narg(upload_id)::bigint IS NULL OR e.upload_id = sqlc.narg(upload_id))
ORDER BY e.upload_id DESC, e.row_number;

-- name: GetRowErrorForUpdate :one
SELECT id FROM upload_row_errors WHERE upload_id = $1 AND row_number = $2 FOR UPDATE;

-- name: DeleteRowError :exec
DELETE FROM upload_row_errors WHERE id = $1;

-- name: UpdateRowError :exec
UPDATE upload_row_errors
SET sku = sqlc.narg(sku), reason = sqlc.arg(reason), raw_category = sqlc.narg(raw_category),
    raw_brand = sqlc.narg(raw_brand), raw_price = sqlc.narg(raw_price)
WHERE id = sqlc.arg(id);

-- name: CountRowFixed :exec
UPDATE uploads SET rows_accepted = rows_accepted + 1, rows_rejected = rows_rejected - 1, updated_at = now()
WHERE id = $1 AND rows_rejected > 0;

-- name: InsertProduct :one
INSERT INTO products (sku, brand_id, upload_id, category, price_minor)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT ((lower(sku))) DO NOTHING
RETURNING id;

-- name: GetUpload :one
SELECT u.id, u.file_name, u.rows_total, u.rows_accepted, u.rows_rejected, u.created_at,
       coalesce((SELECT sum(s.spend_micro_usd) FROM ai_call_spend s
                 JOIN products p ON p.id = s.product_id WHERE p.upload_id = u.id), 0)::bigint AS ai_cost_micro_usd
FROM uploads u WHERE u.id = $1;

-- name: LatestUpload :one
SELECT id FROM uploads ORDER BY id DESC LIMIT 1;

-- name: ListRowErrors :many
SELECT row_number, sku, reason FROM upload_row_errors WHERE upload_id = $1 ORDER BY row_number;

-- name: ListUploadSkus :many
SELECT id, sku FROM products WHERE upload_id = $1;

-- name: ProductsMissingImage :many
SELECT p.sku FROM products p
WHERE p.upload_id = $1 AND NOT EXISTS (SELECT 1 FROM product_images i WHERE i.product_id = p.id)
ORDER BY p.sku;

-- name: NextImagePosition :one
SELECT (coalesce(max(position), 0) + 1)::smallint FROM product_images WHERE product_id = $1;

-- name: InsertImage :exec
INSERT INTO product_images (product_id, position, original_file_name, original_path, detection_path, content_type, byte_size)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: FirstImage :one
SELECT detection_path, content_type FROM product_images WHERE product_id = $1 ORDER BY position LIMIT 1;

-- name: ListProducts :many
SELECT p.id, p.sku, p.category, p.price_minor, p.currency, p.created_at, p.upload_id,
       b.id AS brand_id, b.name AS brand_name,
       (SELECT count(*) FROM product_images i WHERE i.product_id = p.id)::int AS image_count,
       coalesce(a.detection_status, 'pending')::detection_status AS detection_status,
       coalesce(a.revision, 0)::int AS revision,
       a.colour, a.pattern, a.sleeve, a.neckline, a.fit, a.detection_error, a.detection_confidence,
       (a.product_id IS NOT NULL)::boolean AS enrichment_started,
       coalesce((SELECT sum(s.spend_micro_usd) FROM ai_call_spend s WHERE s.product_id = p.id), 0)::bigint AS ai_cost_micro_usd,
       -- Listing counts the product status is derived from (seller flow step 5).
       (SELECT count(*) FROM listings l WHERE l.product_id = p.id)::int AS listings_total,
       (SELECT count(*) FROM listings l WHERE l.product_id = p.id AND l.status = 'queued')::int AS listings_queued,
       (SELECT count(*) FROM listings l WHERE l.product_id = p.id AND l.status = 'failed')::int AS listings_failed,
       (SELECT count(*) FROM listings l WHERE l.product_id = p.id AND l.status = 'stopped_budget')::int AS listings_stopped,
       (SELECT count(*) FROM listings l WHERE l.product_id = p.id AND l.rule_status = 'failing')::int AS listings_failing,
       (SELECT count(*) FROM listings l WHERE l.product_id = p.id AND EXISTS (
          SELECT 1 FROM approvals ap WHERE ap.listing_id = l.id AND ap.listing_version = l.version))::int AS listings_approved
FROM products p
JOIN brands b ON b.id = p.brand_id
LEFT JOIN product_attributes a ON a.product_id = p.id
WHERE (sqlc.narg(upload_id)::bigint IS NULL OR p.upload_id = sqlc.narg(upload_id))
  AND (sqlc.narg(detection_status)::detection_status IS NULL
       OR coalesce(a.detection_status, 'pending') = sqlc.narg(detection_status))
  AND (sqlc.narg(product_id)::bigint IS NULL OR p.id = sqlc.narg(product_id))
  AND lower(p.sku) > lower(sqlc.arg(after_sku)::text)
ORDER BY lower(p.sku)
LIMIT sqlc.arg(page_size);

-- name: ProductIDsForRun :many
SELECT p.id FROM products p
WHERE (sqlc.narg(upload_id)::bigint IS NULL OR p.upload_id = sqlc.narg(upload_id))
  AND EXISTS (SELECT 1 FROM product_images i WHERE i.product_id = p.id)
ORDER BY p.id;

-- name: GetProductForPrompt :one
SELECT p.id, p.sku, p.category, p.price_minor, p.currency, b.name AS brand_name, b.voice_note, b.words_to_avoid
FROM products p JOIN brands b ON b.id = p.brand_id WHERE p.id = $1;

-- name: GetProductForUpdate :one
SELECT p.id, p.sku, p.category, p.price_minor, b.name AS brand_name
FROM products p JOIN brands b ON b.id = p.brand_id
WHERE p.id = $1 FOR UPDATE OF p;

-- name: SkuTakenByOther :one
SELECT EXISTS (SELECT 1 FROM products WHERE lower(sku) = lower(sqlc.arg(sku)::text) AND id <> sqlc.arg(id))::boolean;

-- name: UpdateProductFields :exec
UPDATE products SET sku = $2, brand_id = $3, category = $4, price_minor = $5, updated_at = now()
WHERE id = $1;

-- name: ProductHasApprovals :one
SELECT EXISTS (
  SELECT 1 FROM listings l JOIN approvals a ON a.listing_id = l.id AND a.listing_version = l.version
  WHERE l.product_id = $1
)::boolean;

-- name: DiscardRowError :execrows
DELETE FROM upload_row_errors WHERE upload_id = $1 AND row_number = $2;

-- name: DiscardRowErrors :execrows
DELETE FROM upload_row_errors
WHERE (sqlc.narg(upload_id)::bigint IS NULL OR upload_id = sqlc.narg(upload_id));
