-- name: CreateDistractor :one
INSERT INTO distractors (object_key, filename, width, height, size_bytes, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListDistractors :many
SELECT d.*, u.name AS created_by_name
FROM distractors d
LEFT JOIN users u ON u.id = d.created_by
ORDER BY d.created_at DESC, d.id DESC;

-- name: ListDistractorKeys :many
SELECT object_key FROM distractors ORDER BY random() LIMIT $1;

-- name: DeleteDistractor :one
DELETE FROM distractors WHERE id = $1 RETURNING *;
