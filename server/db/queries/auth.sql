-- name: CreateUser :one
INSERT INTO users (email, role, password_hash)
VALUES ($1, $2, $3)
RETURNING id;

-- name: SetUserPassword :execrows
UPDATE users SET password_hash = $2, updated_at = now() WHERE lower(email) = lower($1);

-- name: GetUserByEmail :one
SELECT id, email, role, password_hash FROM users WHERE lower(email) = lower($1);

-- name: CreateSession :exec
INSERT INTO sessions (token_hash, user_id, csrf_token_hash, expires_at)
VALUES ($1, $2, $3, $4);

-- name: GetSession :one
SELECT s.user_id, s.csrf_token_hash, s.expires_at, u.email, u.role
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = $1 AND s.expires_at > now();

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash = $1;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at < now();
