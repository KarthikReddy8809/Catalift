-- name: ApprovalCounts :many
SELECT l.channel,
       count(*)::int AS total,
       count(*) FILTER (WHERE EXISTS (
         SELECT 1 FROM approvals a WHERE a.listing_id = l.id AND a.listing_version = l.version
       ))::int AS approved
FROM listings l
JOIN products p ON p.id = l.product_id
WHERE (sqlc.narg(upload_id)::bigint IS NULL OR p.upload_id = sqlc.narg(upload_id))
GROUP BY l.channel
ORDER BY l.channel;

-- name: ApprovedForChannel :many
SELECT p.sku, l.title, l.bullet_1, l.bullet_2, l.bullet_3, l.bullet_4, l.bullet_5, l.description,
       pa.colour, pa.pattern, pa.sleeve, pa.neckline, pa.fit
FROM listings l
JOIN products p ON p.id = l.product_id
LEFT JOIN product_attributes pa ON pa.product_id = l.product_id
WHERE l.channel = sqlc.arg(channel) AND l.status = 'generated'
  AND (sqlc.narg(upload_id)::bigint IS NULL OR p.upload_id = sqlc.narg(upload_id))
  AND EXISTS (SELECT 1 FROM approvals a WHERE a.listing_id = l.id AND a.listing_version = l.version)
ORDER BY lower(p.sku);

-- name: CreateExport :one
INSERT INTO exports (created_by, upload_id) VALUES (sqlc.arg(created_by), sqlc.narg(upload_id)) RETURNING id, created_at;

-- name: InsertExportFile :exec
INSERT INTO export_files (export_id, channel, file_path, row_count) VALUES ($1, $2, $3, $4);

-- name: GetExport :one
SELECT e.id, e.created_at, e.sent_at, u.email AS sent_by_email
FROM exports e LEFT JOIN users u ON u.id = e.sent_by
WHERE e.id = $1;

-- name: SendExport :execrows
-- Sends once: a second send changes nothing, so the first sender and time stay.
UPDATE exports SET sent_at = now(), sent_by = sqlc.arg(sent_by)
WHERE id = sqlc.arg(id) AND sent_at IS NULL;

-- name: ListExports :many
-- Newest export first; sent_only limits it to what reviewers sent the seller.
SELECT e.id, e.created_at, e.sent_at, u.email AS sent_by_email
FROM exports e LEFT JOIN users u ON u.id = e.sent_by
WHERE (NOT sqlc.arg(sent_only)::boolean OR e.sent_at IS NOT NULL)
  AND (sqlc.narg(upload_id)::bigint IS NULL OR e.upload_id = sqlc.narg(upload_id))
  AND (sqlc.arg(before_id)::bigint = 0 OR e.id < sqlc.arg(before_id))
ORDER BY e.id DESC
LIMIT sqlc.arg(page_size);

-- name: ListExportFiles :many
SELECT channel, file_path, row_count FROM export_files WHERE export_id = $1 ORDER BY channel;

-- name: GetExportFile :one
SELECT file_path FROM export_files WHERE export_id = $1 AND channel = $2;
