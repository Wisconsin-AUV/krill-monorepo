-- name: CreateWorkerToken :one
INSERT INTO worker_tokens (name, token_hash, created_by)
VALUES (@name, @token_hash, @created_by)
RETURNING *;

-- name: ListWorkerTokens :many
SELECT
    t.id, t.name, t.created_at, t.last_used_at, t.revoked_at,
    u.id AS creator_id, u.name AS creator_name, u.username AS creator_username, u.role AS creator_role,
    u.created_at AS creator_created_at
FROM worker_tokens t
LEFT JOIN users u ON u.id = t.created_by
ORDER BY t.revoked_at IS NOT NULL, t.id DESC;

-- name: RevokeWorkerToken :one
UPDATE worker_tokens SET revoked_at = now()
WHERE id = $1 AND revoked_at IS NULL
RETURNING *;

-- name: GetWorkerToken :one
SELECT * FROM worker_tokens WHERE id = $1;

-- UseWorkerToken returns the active token with the hash. last_used_at is
-- only written once a minute, since workers call several times a minute.
-- name: UseWorkerToken :one
WITH token AS (
    SELECT id, name, last_used_at FROM worker_tokens
    WHERE worker_tokens.token_hash = @token_hash::bytea AND revoked_at IS NULL
), touched AS (
    UPDATE worker_tokens w SET last_used_at = now()
    FROM token
    WHERE w.id = token.id AND (token.last_used_at IS NULL OR token.last_used_at < now() - interval '1 minute')
)
SELECT id, name FROM token;

-- name: CountActiveWorkerTokens :one
SELECT count(*) FROM worker_tokens WHERE revoked_at IS NULL;
