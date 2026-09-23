-- name: CreateDataset :one
INSERT INTO datasets (name, options, stats) VALUES ($1, $2, $3) RETURNING *;

-- name: GetDataset :one
SELECT * FROM datasets WHERE id = $1;

-- name: ListDatasets :many
SELECT * FROM datasets ORDER BY created_at DESC, id DESC;

-- name: DeleteDataset :execrows
DELETE FROM datasets WHERE id = $1;

-- name: StartDataset :execrows
UPDATE datasets SET status = 'running', error = '', progress = 0 WHERE id = $1;

-- name: SetDatasetProgress :exec
UPDATE datasets SET progress = $2 WHERE id = $1;

-- name: SetDatasetStats :exec
UPDATE datasets SET stats = $2 WHERE id = $1;

-- name: FinishDataset :exec
UPDATE datasets SET status = 'ready', progress = 1, size_bytes = $2, finished_at = now() WHERE id = $1;

-- name: FailDataset :exec
UPDATE datasets SET status = 'failed', error = $2, finished_at = now() WHERE id = $1;

-- name: FailInterruptedDatasets :execrows
UPDATE datasets SET status = 'failed', error = 'Export was interrupted by an API restart.', finished_at = now()
WHERE status = 'running';

-- name: ListExportVideos :many
SELECT id, name, split FROM videos
WHERE status = 'ready'
    AND (cardinality(sqlc.arg('video_ids')::bigint[]) = 0 OR id = ANY(sqlc.arg('video_ids')::bigint[]))
ORDER BY id;

-- name: ListExportFrames :many
SELECT f.id, f.video_id, f.clip_id, f.idx, f.phash, f.status
FROM frames f
JOIN videos v ON v.id = f.video_id
WHERE v.status = 'ready'
    AND (cardinality(sqlc.arg('video_ids')::bigint[]) = 0 OR v.id = ANY(sqlc.arg('video_ids')::bigint[]))
ORDER BY f.video_id, f.idx;

-- name: ListExportAnnotations :many
SELECT a.frame_id, a.x, a.y, a.width, a.height, a.status, t.label_type_id, t.attributes
FROM annotations a
JOIN tracks t ON t.id = a.track_id
JOIN frames f ON f.id = a.frame_id
JOIN videos v ON v.id = f.video_id
WHERE v.status = 'ready'
    AND f.status <> 'unlabeled'
    AND a.status <> 'rejected'
    AND (cardinality(sqlc.arg('video_ids')::bigint[]) = 0 OR v.id = ANY(sqlc.arg('video_ids')::bigint[]))
ORDER BY a.frame_id, a.track_id;

-- name: ListAllLabelTypes :many
SELECT * FROM label_types ORDER BY position, id;
