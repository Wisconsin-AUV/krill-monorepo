-- name: ListLabelTypes :many
SELECT
    sqlc.embed(lt),
    (SELECT count(*) FROM tracks t WHERE t.label_type_id = lt.id)::int AS track_count,
    (SELECT count(*) FROM annotations a JOIN tracks t ON t.id = a.track_id WHERE t.label_type_id = lt.id)::int AS box_count
FROM label_types lt
ORDER BY lt.position, lt.id;

-- name: GetLabelType :one
SELECT * FROM label_types WHERE id = $1;

-- name: CreateLabelType :one
INSERT INTO label_types (name, title, color, description, guideline, attributes, position)
VALUES ($1, $2, $3, $4, $5, $6, (SELECT coalesce(max(position), -1) + 1 FROM label_types))
RETURNING *;

-- name: UpdateLabelType :one
UPDATE label_types
SET name = $2, title = $3, color = $4, description = $5, guideline = $6, attributes = $7
WHERE id = $1
RETURNING *;

-- name: DeleteLabelType :execrows
DELETE FROM label_types WHERE id = $1;

-- name: CountTracksForLabelType :one
SELECT count(*)::int FROM tracks WHERE label_type_id = $1;

-- name: SetLabelTypePosition :exec
UPDATE label_types SET position = $2 WHERE id = $1;

-- name: ListLabelExamples :many
SELECT * FROM label_examples ORDER BY label_type_id, id;

-- name: CreateLabelExample :one
INSERT INTO label_examples (label_type_id, object_key, caption)
VALUES ($1, $2, $3)
RETURNING *;

-- name: DeleteLabelExample :one
DELETE FROM label_examples WHERE id = $1
RETURNING *;
