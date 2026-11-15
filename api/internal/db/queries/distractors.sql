-- name: CreateDistractor :one
INSERT INTO distractors (filename, created_by)
VALUES ($1, $2)
RETURNING *;

-- name: ListDistractors :many
SELECT d.*, u.name AS created_by_name
FROM distractors d
LEFT JOIN users u ON u.id = d.created_by
ORDER BY d.created_at DESC, d.id DESC;

-- name: ListReadyDistractorIDs :many
SELECT id FROM distractors WHERE status = 'ready' ORDER BY random() LIMIT $1;

-- name: GetDistractor :one
SELECT * FROM distractors WHERE id = $1;

-- name: QueueDistractor :one
UPDATE distractors SET status = 'queued', error = ''
WHERE id = $1 AND status IN ('uploading', 'failed')
RETURNING *;

-- name: StartDistractor :execrows
UPDATE distractors SET status = 'processing' WHERE id = $1 AND status = 'queued';

-- name: FinishDistractor :execrows
UPDATE distractors SET status = 'ready', width = $2, height = $3, size_bytes = $4
WHERE id = $1 AND status = 'processing';

-- name: FailDistractor :exec
UPDATE distractors SET status = 'failed', error = $2 WHERE id = $1 AND status = 'processing';

-- name: DeleteDistractors :many
DELETE FROM distractors WHERE id = ANY(@ids::bigint[]) RETURNING *;
