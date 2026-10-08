-- name: EnsureAttributes :exec
INSERT INTO product_attributes (product_id) VALUES ($1) ON CONFLICT (product_id) DO NOTHING;

-- name: GetAttributesForUpdate :one
SELECT product_id, detection_status, revision, colour, pattern, sleeve, neckline, fit, detection_error
FROM product_attributes WHERE product_id = $1 FOR UPDATE;

-- name: GetAttributes :one
SELECT product_id, detection_status, revision, colour, pattern, sleeve, neckline, fit, detection_error
FROM product_attributes WHERE product_id = $1;

-- name: SetDetectionDone :one
UPDATE product_attributes
SET detection_status = 'done', colour = $2, pattern = $3, sleeve = $4, neckline = $5, fit = $6,
    detection_confidence = sqlc.narg(confidence)::real,
    detection_error = NULL, revision = revision + 1, updated_at = now()
WHERE product_id = $1
RETURNING revision;

-- name: SetDetectionEnded :exec
UPDATE product_attributes
SET detection_status = $2, detection_error = sqlc.narg(detection_error), updated_at = now()
WHERE product_id = $1;

-- name: ResetDetectionPending :exec
UPDATE product_attributes
SET detection_status = 'pending', detection_error = NULL, updated_at = now()
WHERE product_id = $1 AND detection_status IN ('failed', 'stopped_budget');

-- name: CorrectAttributes :one
UPDATE product_attributes
SET colour = coalesce(sqlc.narg(colour), colour),
    pattern = coalesce(sqlc.narg(pattern), pattern),
    sleeve = coalesce(sqlc.narg(sleeve), sleeve),
    neckline = coalesce(sqlc.narg(neckline), neckline),
    fit = coalesce(sqlc.narg(fit), fit),
    corrected_by = sqlc.arg(corrected_by),
    -- A reviewer's correction is certain; it leaves the low-confidence list.
    detection_confidence = NULL,
    revision = revision + 1,
    updated_at = now()
WHERE product_id = sqlc.arg(product_id) AND revision = sqlc.arg(expected_revision)
  AND detection_status = 'done'
RETURNING product_id, detection_status, revision, colour, pattern, sleeve, neckline, fit, detection_error;

-- name: QueueListing :one
INSERT INTO listings (product_id, channel) VALUES ($1, $2)
ON CONFLICT (product_id, channel) DO UPDATE
SET status = 'queued', failure_reason = NULL, updated_at = now()
WHERE listings.status IN ('failed', 'stopped_budget')
RETURNING id;

-- name: QueuedListingsForProduct :many
SELECT id, channel FROM listings WHERE product_id = $1 AND status = 'queued' ORDER BY channel;

-- name: EndQueuedListingsForProduct :exec
UPDATE listings SET status = $2, failure_reason = sqlc.narg(failure_reason), updated_at = now()
WHERE product_id = $1 AND status = 'queued';

-- name: GetListing :one
SELECT l.id, l.product_id, l.channel, l.status, l.failure_reason, l.version,
       l.title, l.bullet_1, l.bullet_2, l.bullet_3, l.bullet_4, l.bullet_5, l.description,
       l.rule_status, l.rule_config_hash, l.first_pass_passed, p.sku
FROM listings l JOIN products p ON p.id = l.product_id
WHERE l.id = $1;

-- name: GetListingForUpdate :one
SELECT id, product_id, channel, status, version, title, bullet_1, bullet_2, bullet_3, bullet_4, bullet_5, description, rule_status
FROM listings WHERE id = $1 FOR UPDATE;

-- A text or attribute change sets rule_status to failing as a placeholder:
-- a generated listing is never unchecked (chk_listings_generated_checked), and
-- Revalidate sets the real status in the same transaction. A listing of a
-- switched-off channel stays failing, so it cannot be approved (D8).

-- name: WriteGenerated :one
UPDATE listings
SET status = 'generated', failure_reason = NULL, version = version + 1,
    title = $2, bullet_1 = $3, bullet_2 = $4, bullet_3 = $5, bullet_4 = $6, bullet_5 = $7,
    description = $8, rule_status = 'failing', updated_at = now()
WHERE id = $1
RETURNING version;

-- name: SetListingEnded :exec
UPDATE listings SET status = $2, failure_reason = sqlc.narg(failure_reason), updated_at = now()
WHERE id = $1;

-- name: UpdateListingText :one
UPDATE listings
SET title = coalesce(sqlc.narg(title), title),
    bullet_1 = coalesce(sqlc.narg(bullet_1), bullet_1),
    bullet_2 = coalesce(sqlc.narg(bullet_2), bullet_2),
    bullet_3 = coalesce(sqlc.narg(bullet_3), bullet_3),
    bullet_4 = coalesce(sqlc.narg(bullet_4), bullet_4),
    bullet_5 = coalesce(sqlc.narg(bullet_5), bullet_5),
    description = coalesce(sqlc.narg(description), description),
    version = version + 1, rule_status = 'failing', updated_at = now()
WHERE id = sqlc.arg(id) AND version = sqlc.arg(expected_version) AND status = 'generated'
RETURNING version;

-- name: SetFieldIfUnchanged :one
UPDATE listings
SET title = CASE WHEN sqlc.arg(field)::text = 'title' THEN sqlc.arg(value)::text ELSE title END,
    bullet_1 = CASE WHEN sqlc.arg(field)::text = 'bullet_1' THEN sqlc.arg(value)::text ELSE bullet_1 END,
    bullet_2 = CASE WHEN sqlc.arg(field)::text = 'bullet_2' THEN sqlc.arg(value)::text ELSE bullet_2 END,
    bullet_3 = CASE WHEN sqlc.arg(field)::text = 'bullet_3' THEN sqlc.arg(value)::text ELSE bullet_3 END,
    bullet_4 = CASE WHEN sqlc.arg(field)::text = 'bullet_4' THEN sqlc.arg(value)::text ELSE bullet_4 END,
    bullet_5 = CASE WHEN sqlc.arg(field)::text = 'bullet_5' THEN sqlc.arg(value)::text ELSE bullet_5 END,
    description = CASE WHEN sqlc.arg(field)::text = 'description' THEN sqlc.arg(value)::text ELSE description END,
    version = version + 1, rule_status = 'failing', updated_at = now()
WHERE id = sqlc.arg(id) AND status = 'generated'
  AND CASE sqlc.arg(field)::text
        WHEN 'title' THEN title WHEN 'bullet_1' THEN bullet_1 WHEN 'bullet_2' THEN bullet_2
        WHEN 'bullet_3' THEN bullet_3 WHEN 'bullet_4' THEN bullet_4 WHEN 'bullet_5' THEN bullet_5
        WHEN 'description' THEN description END = sqlc.arg(base_value)::text
RETURNING version;

-- name: BumpProductListings :many
UPDATE listings SET version = version + 1, rule_status = 'failing', updated_at = now()
WHERE product_id = $1 AND status = 'generated'
RETURNING id;

-- name: BumpListing :exec
UPDATE listings SET version = version + 1, updated_at = now() WHERE id = $1;

-- name: SetRuleStatus :exec
UPDATE listings
SET rule_status = $2, rule_config_hash = $3,
    first_pass_passed = coalesce(first_pass_passed, sqlc.arg(first_pass_passed)::boolean),
    first_pass_failed_rules = coalesce(first_pass_failed_rules, sqlc.arg(first_pass_failed_rules)::text[]),
    updated_at = now()
WHERE id = $1;

-- name: DeleteRuleResults :exec
DELETE FROM rule_results WHERE listing_id = $1;

-- name: InsertRuleResult :exec
INSERT INTO rule_results (listing_id, listing_version, rule, field, message, config_hash)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListRuleResults :many
SELECT listing_id, rule, field, message FROM rule_results
WHERE listing_id = ANY(sqlc.arg(listing_ids)::bigint[])
ORDER BY listing_id, id;

-- name: ListingsForRecheck :many
SELECT l.id FROM listings l
WHERE l.channel = $1 AND l.status = 'generated'
  AND (l.rule_config_hash IS NULL OR l.rule_config_hash <> sqlc.arg(config_hash)::text)
ORDER BY l.id;

-- name: IsApprovedNow :one
SELECT EXISTS (
  SELECT 1 FROM approvals a JOIN listings l ON l.id = a.listing_id
  WHERE a.listing_id = $1 AND a.listing_version = l.version
)::boolean;

-- name: ListGrid :many
SELECT l.id, l.product_id, p.sku, l.channel, l.status, l.failure_reason, l.version,
       l.title, l.bullet_1, l.bullet_2, l.bullet_3, l.bullet_4, l.bullet_5, l.description,
       l.rule_status,
       a.approved_at AS approved_at, u.email AS approved_by,
       pa.colour, pa.pattern, pa.sleeve, pa.neckline, pa.fit, coalesce(pa.revision, 0)::int AS attributes_revision
FROM listings l
JOIN products p ON p.id = l.product_id
LEFT JOIN product_attributes pa ON pa.product_id = l.product_id
LEFT JOIN LATERAL (
  SELECT ap.created_at AS approved_at, ap.approved_by FROM approvals ap
  WHERE ap.listing_id = l.id AND ap.listing_version = l.version LIMIT 1
) a ON true
LEFT JOIN users u ON u.id = a.approved_by
WHERE (sqlc.narg(channel)::text IS NULL OR l.channel = sqlc.narg(channel))
  AND (sqlc.narg(rule_status)::rule_status IS NULL OR l.rule_status = sqlc.narg(rule_status))
  AND (sqlc.narg(approved)::boolean IS NULL OR (a.approved_at IS NOT NULL) = sqlc.narg(approved))
  AND (sqlc.narg(listing_id)::bigint IS NULL OR l.id = sqlc.narg(listing_id))
  AND (sqlc.narg(upload_id)::bigint IS NULL OR p.upload_id = sqlc.narg(upload_id))
  AND (lower(p.sku), l.channel) > (lower(sqlc.arg(after_sku)::text), sqlc.arg(after_channel)::text)
ORDER BY lower(p.sku), l.channel
LIMIT sqlc.arg(page_size);

-- name: ApproveIfCurrent :one
INSERT INTO approvals (listing_id, listing_version, approved_by)
SELECT l.id, l.version, sqlc.arg(approved_by) FROM listings l
WHERE l.id = sqlc.arg(listing_id) AND l.version = sqlc.arg(listing_version)
  AND l.status = 'generated' AND l.rule_status = 'passing'
ON CONFLICT (listing_id, listing_version) DO UPDATE SET listing_id = approvals.listing_id
RETURNING created_at;

-- name: CreateRegeneration :one
INSERT INTO regeneration_requests (listing_id, field, instruction, base_value, requested_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, listing_id, field, instruction, status, created_at;

-- name: GetRegeneration :one
SELECT id, listing_id, field, instruction, base_value, status FROM regeneration_requests WHERE id = $1;

-- name: SetRegenerationStatus :exec
UPDATE regeneration_requests SET status = $2, updated_at = now() WHERE id = $1;

-- name: ListRegenerations :many
SELECT id, listing_id, field, instruction, status, created_at FROM regeneration_requests
WHERE listing_id = sqlc.arg(listing_id) AND id < sqlc.arg(before_id)
ORDER BY id DESC LIMIT sqlc.arg(page_size);

-- name: LatestRegenerations :many
SELECT DISTINCT ON (listing_id) listing_id, field, status FROM regeneration_requests
WHERE listing_id = ANY(sqlc.arg(listing_ids)::bigint[])
ORDER BY listing_id, id DESC;

-- name: InsertConfigRecheck :exec
INSERT INTO config_rechecks (channel, config_hash, listings_rechecked, approvals_cleared)
VALUES ($1, $2, $3, $4);

-- name: LatestRechecks :many
SELECT DISTINCT ON (channel) channel, config_hash, listings_rechecked, approvals_cleared, created_at
FROM config_rechecks ORDER BY channel, created_at DESC;

-- name: ForceDetectionPending :exec
UPDATE product_attributes
SET detection_status = 'pending', detection_error = NULL, updated_at = now()
WHERE product_id = $1;

-- name: RequeueProductListings :exec
-- A re-run rewrites every channel's listing; the version rises so no older
-- approval can match the new text.
UPDATE listings SET status = 'queued', failure_reason = NULL, version = version + 1, updated_at = now()
WHERE product_id = $1;
