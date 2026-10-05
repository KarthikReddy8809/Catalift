-- +goose Up
-- The one spend rule (data model section 4): succeeded calls at their cost,
-- failed calls at their cost or 0, reserved and possibly charged calls at
-- their reservation. The budget check, product cost and the banner all read
-- this view, so they cannot disagree.
CREATE VIEW ai_call_spend AS
SELECT
    id,
    product_id,
    purpose,
    status,
    CASE status
        WHEN 'succeeded' THEN cost_micro_usd
        WHEN 'failed' THEN coalesce(cost_micro_usd, 0)
        ELSE reserved_micro_usd
    END::bigint AS spend_micro_usd
FROM ai_calls;

COMMENT ON VIEW ai_call_spend IS 'Spend per AI call under the single spend rule (data model section 4).';

-- +goose Down
DROP VIEW IF EXISTS ai_call_spend;
