-- name: ListSessions :many
SELECT
    s.id, s.ip, s.user_agent, s.created_at, s.last_seen_at, s.expires_at,
    s.token_hash = @current::bytea AS current,
    u.id AS user_id, u.name AS user_name, u.username, u.role, u.created_at AS user_created_at
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.expires_at > now()
    AND (sqlc.narg('user_id')::uuid IS NULL OR s.user_id = sqlc.narg('user_id')::uuid)
ORDER BY s.last_seen_at DESC, s.created_at DESC;

-- name: DeleteSessionByID :one
DELETE FROM sessions
WHERE id = @id
    AND (sqlc.narg('user_id')::uuid IS NULL OR user_id = sqlc.narg('user_id')::uuid)
RETURNING user_id, ip, user_agent, token_hash;
