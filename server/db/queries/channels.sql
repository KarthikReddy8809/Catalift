-- name: InsertChannelRuleEdit :one
INSERT INTO channel_rule_edits (channel, title_max_length, required_attributes, banned_words, edited_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, created_at;

-- name: LatestChannelRuleEdits :many
SELECT DISTINCT ON (e.channel) e.channel, e.title_max_length, e.required_attributes, e.banned_words,
       u.email AS edited_by, e.created_at
FROM channel_rule_edits e
JOIN users u ON u.id = e.edited_by
ORDER BY e.channel, e.id DESC;
