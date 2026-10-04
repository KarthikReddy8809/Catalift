-- name: LockBudget :one
SELECT limit_micro_usd, blocked_at FROM budget WHERE id = 1 FOR UPDATE;

-- name: GetBudget :one
SELECT b.limit_micro_usd, b.blocked_at,
       coalesce((SELECT sum(spend_micro_usd) FROM ai_call_spend), 0)::bigint AS spent_micro_usd
FROM budget b WHERE b.id = 1;

-- name: TotalSpend :one
SELECT coalesce(sum(spend_micro_usd), 0)::bigint FROM ai_call_spend;

-- name: SetBudgetBlocked :exec
UPDATE budget SET blocked_at = now(), updated_at = now() WHERE id = 1 AND blocked_at IS NULL;

-- name: ClearBudgetBlock :exec
UPDATE budget SET blocked_at = NULL, limit_micro_usd = sqlc.arg(limit_micro_usd), updated_at = now() WHERE id = 1;

-- name: ReserveCall :one
INSERT INTO ai_calls (job_id, product_id, purpose, model, prompt_template_id, prompt, reserved_micro_usd)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id;

-- name: FinishCall :exec
UPDATE ai_calls
SET status = $2, cost_micro_usd = sqlc.narg(cost_micro_usd), input_tokens = sqlc.narg(input_tokens),
    output_tokens = sqlc.narg(output_tokens), provider_generation_id = sqlc.narg(provider_generation_id),
    error = sqlc.narg(error), updated_at = now()
WHERE id = $1;

-- name: SweepReservations :execrows
UPDATE ai_calls SET status = 'possibly_charged', updated_at = now()
WHERE status = 'reserved' AND created_at < now() - make_interval(secs => sqlc.arg(older_than_seconds)::int);
