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
INSERT INTO label_types (name, color, description, attributes, position)
VALUES ($1, $2, $3, $4, (SELECT coalesce(max(position), -1) + 1 FROM label_types))
RETURNING *;

-- name: UpdateLabelType :one
UPDATE label_types SET name = $2, color = $3, description = $4, attributes = $5
WHERE id = $1
RETURNING *;

-- name: DeleteLabelType :execrows
DELETE FROM label_types WHERE id = $1;

-- name: CountTracksForLabelType :one
SELECT count(*)::int FROM tracks WHERE label_type_id = $1;

-- name: SetLabelTypePosition :exec
UPDATE label_types SET position = $2 WHERE id = $1;
