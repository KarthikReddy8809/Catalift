-- name: ApprovalCounts :many
SELECT l.channel,
       count(*)::int AS total,
       count(*) FILTER (WHERE EXISTS (
         SELECT 1 FROM approvals a WHERE a.listing_id = l.id AND a.listing_version = l.version
       ))::int AS approved
FROM listings l
GROUP BY l.channel
ORDER BY l.channel;

-- name: ApprovedForChannel :many
SELECT p.sku, l.title, l.bullet_1, l.bullet_2, l.bullet_3, l.bullet_4, l.bullet_5, l.description,
       pa.colour, pa.pattern, pa.sleeve, pa.neckline, pa.fit
FROM listings l
JOIN products p ON p.id = l.product_id
LEFT JOIN product_attributes pa ON pa.product_id = l.product_id
WHERE l.channel = $1 AND l.status = 'generated'
  AND EXISTS (SELECT 1 FROM approvals a WHERE a.listing_id = l.id AND a.listing_version = l.version)
ORDER BY lower(p.sku);

-- name: CreateExport :one
INSERT INTO exports (created_by) VALUES ($1) RETURNING id, created_at;

-- name: InsertExportFile :exec
INSERT INTO export_files (export_id, channel, file_path, row_count) VALUES ($1, $2, $3, $4);

-- name: GetExport :one
SELECT id, created_at FROM exports WHERE id = $1;

-- name: ListExportFiles :many
SELECT channel, file_path, row_count FROM export_files WHERE export_id = $1 ORDER BY channel;

-- name: GetExportFile :one
SELECT file_path FROM export_files WHERE export_id = $1 AND channel = $2;
