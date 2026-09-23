-- name: CreateUser :one
INSERT INTO users (name, username, email, password_hash, role, slack_user_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: LockUsers :exec
LOCK TABLE users IN SHARE ROW EXCLUSIVE MODE;

-- name: CountUsers :one
SELECT count(*)::int FROM users;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByLogin :one
SELECT * FROM users WHERE username = sqlc.arg('login')::text OR email = sqlc.arg('login')::text;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserBySlackID :one
SELECT * FROM users WHERE slack_user_id = $1;

-- name: UsernameExists :one
SELECT EXISTS (SELECT 1 FROM users WHERE username = $1);

-- name: ListUsers :many
SELECT * FROM users ORDER BY created_at, id;

-- name: UpdateUserProfile :one
UPDATE users SET name = $2, username = $3, email = $4 WHERE id = $1 RETURNING *;

-- name: SetUserRole :one
UPDATE users SET role = $2 WHERE id = $1 RETURNING *;

-- name: SetUserDisabled :one
UPDATE users SET disabled = $2 WHERE id = $1 RETURNING *;

-- name: SetUserPassword :execrows
UPDATE users SET password_hash = $2 WHERE id = $1;

-- name: LinkSlack :one
UPDATE users SET slack_user_id = $2 WHERE id = $1 RETURNING *;

-- name: TouchLogin :exec
UPDATE users SET last_login_at = now() WHERE id = $1;

-- name: DeleteUser :execrows
DELETE FROM users WHERE id = $1;

-- name: CreateSession :exec
INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3);

-- name: GetSessionUser :one
SELECT u.* FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = $1 AND s.expires_at > now() AND NOT u.disabled;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash = $1;

-- name: DeleteUserSessions :exec
DELETE FROM sessions WHERE user_id = $1;

-- name: DeleteOtherSessions :exec
DELETE FROM sessions WHERE user_id = $1 AND token_hash <> $2;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at <= now();
