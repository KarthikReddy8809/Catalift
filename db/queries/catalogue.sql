-- name: UpsertBrand :one
INSERT INTO brands (name) VALUES (btrim($1))
ON CONFLICT ((lower(btrim(name)))) DO UPDATE SET updated_at = brands.updated_at
RETURNING id;

-- name: ListBrands :many
SELECT id, name, voice_note, created_at, updated_at
FROM brands
WHERE lower(name) > lower(sqlc.arg(after_name)::text)
ORDER BY lower(name)
LIMIT sqlc.arg(page_size);

-- name: UpdateBrandVoice :one
UPDATE brands SET voice_note = sqlc.narg(voice_note), updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, name, voice_note, created_at, updated_at;

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
INSERT INTO upload_row_errors (upload_id, row_number, sku, reason) VALUES ($1, $2, $3, $4);

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
SELECT p.id, p.sku, p.category, p.price_minor, p.currency, p.created_at,
       b.id AS brand_id, b.name AS brand_name,
       (SELECT count(*) FROM product_images i WHERE i.product_id = p.id)::int AS image_count,
       coalesce(a.detection_status, 'pending')::detection_status AS detection_status,
       coalesce(a.revision, 0)::int AS revision,
       a.colour, a.pattern, a.sleeve, a.neckline, a.fit, a.detection_error,
       coalesce((SELECT sum(s.spend_micro_usd) FROM ai_call_spend s WHERE s.product_id = p.id), 0)::bigint AS ai_cost_micro_usd
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
SELECT p.id, p.sku, p.category, p.price_minor, p.currency, b.name AS brand_name, b.voice_note
FROM products p JOIN brands b ON b.id = p.brand_id WHERE p.id = $1;
