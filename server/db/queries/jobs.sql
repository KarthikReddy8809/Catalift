-- name: CreateRun :one
INSERT INTO generation_runs (started_by, neutral_voice_confirmed) VALUES ($1, $2)
RETURNING id, created_at;

-- name: GetRun :one
SELECT id, created_at FROM generation_runs WHERE id = $1;

-- name: EnqueueJob :one
INSERT INTO jobs (type, dedupe_key, run_id, product_id, listing_id, regeneration_request_id, attributes_revision)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (dedupe_key) WHERE status IN ('queued', 'running') DO NOTHING
RETURNING id;

-- name: ClaimJob :one
UPDATE jobs
SET status = 'running', claimed_by = sqlc.arg(worker), claim_token = sqlc.arg(claim_token),
    lease_until = now() + make_interval(secs => sqlc.arg(lease_seconds)::int),
    first_attempt_at = coalesce(first_attempt_at, now()), updated_at = now()
WHERE id = (
  SELECT id FROM jobs WHERE status = 'queued' AND run_after <= now()
  ORDER BY run_after, id FOR UPDATE SKIP LOCKED LIMIT 1
)
RETURNING id, type, run_id, product_id, listing_id, regeneration_request_id, attributes_revision, attempts, first_attempt_at;

-- name: FinishJob :execrows
UPDATE jobs
SET status = sqlc.arg(status), last_error = sqlc.narg(last_error), finished_at = now(),
    claimed_by = NULL, claim_token = NULL, lease_until = NULL, updated_at = now()
WHERE id = sqlc.arg(id) AND status = 'running' AND claim_token = sqlc.arg(claim_token);

-- name: RetryJob :execrows
UPDATE jobs
SET status = 'queued', attempts = attempts + sqlc.arg(add_attempt)::smallint,
    run_after = now() + make_interval(secs => sqlc.arg(delay_seconds)::int),
    last_error = sqlc.narg(last_error),
    claimed_by = NULL, claim_token = NULL, lease_until = NULL, updated_at = now()
WHERE id = sqlc.arg(id) AND status = 'running' AND claim_token = sqlc.arg(claim_token);

-- name: SweepStaleJobs :execrows
UPDATE jobs
SET status = CASE WHEN attempts + 1 >= 3 THEN 'failed'::job_status ELSE 'queued'::job_status END,
    attempts = attempts + 1,
    finished_at = CASE WHEN attempts + 1 >= 3 THEN now() END,
    last_error = 'worker stopped before finishing',
    claimed_by = NULL, claim_token = NULL, lease_until = NULL, updated_at = now()
WHERE status = 'running' AND lease_until < now();

-- name: CancelQueuedRunJobs :execrows
UPDATE jobs SET status = 'cancelled', finished_at = now(), updated_at = now()
WHERE run_id = $1 AND status = 'queued';

-- name: RunProgress :one
SELECT
  count(*) FILTER (WHERE pa.detection_status = 'pending')::int AS detection_pending,
  count(*) FILTER (WHERE pa.detection_status = 'done')::int AS detection_done,
  count(*) FILTER (WHERE pa.detection_status = 'failed')::int AS detection_failed,
  count(*) FILTER (WHERE pa.detection_status = 'stopped_budget')::int AS detection_stopped
FROM product_attributes pa
WHERE pa.product_id IN (SELECT DISTINCT j.product_id FROM jobs j WHERE j.run_id = $1);

-- name: RunListingProgress :one
SELECT
  count(*) FILTER (WHERE l.status = 'queued')::int AS queued,
  count(*) FILTER (WHERE l.status = 'generated')::int AS generated,
  count(*) FILTER (WHERE l.status = 'failed')::int AS failed,
  count(*) FILTER (WHERE l.status = 'stopped_budget')::int AS stopped
FROM listings l
WHERE l.product_id IN (SELECT DISTINCT j.product_id FROM jobs j WHERE j.run_id = $1);

-- name: RunProductIDs :many
SELECT DISTINCT product_id FROM jobs WHERE run_id = $1 ORDER BY product_id;
