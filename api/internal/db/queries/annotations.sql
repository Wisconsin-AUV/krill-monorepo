-- name: GetFrame :one
SELECT * FROM frames WHERE id = $1;

-- name: CreateTrack :one
INSERT INTO tracks (clip_id, label_type_id, attributes)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetTrack :one
SELECT * FROM tracks WHERE id = $1;

-- name: UpdateTrack :one
UPDATE tracks SET label_type_id = $2, attributes = $3
WHERE id = $1
RETURNING *;

-- name: DeleteTrack :execrows
DELETE FROM tracks WHERE id = $1;

-- name: ListClipTracks :many
SELECT * FROM tracks WHERE clip_id = $1 ORDER BY id;

-- name: ListClipAnnotations :many
SELECT a.*
FROM annotations a
JOIN tracks t ON t.id = a.track_id
WHERE t.clip_id = $1
ORDER BY a.frame_id, a.track_id;

-- name: UpsertAnnotation :one
INSERT INTO annotations (track_id, frame_id, x, y, width, height, source, status)
VALUES ($1, $2, $3, $4, $5, $6, 'human', 'verified')
ON CONFLICT (track_id, frame_id) DO UPDATE SET
    x = excluded.x,
    y = excluded.y,
    width = excluded.width,
    height = excluded.height,
    source = 'human',
    status = 'verified',
    model_version = '',
    updated_at = now()
RETURNING *;

-- name: DeleteAnnotation :execrows
DELETE FROM annotations WHERE track_id = $1 AND frame_id = $2;

-- name: CountTrackAnnotations :one
SELECT count(*)::int FROM annotations WHERE track_id = $1;

-- name: CopyAnnotations :many
INSERT INTO annotations (track_id, frame_id, x, y, width, height, source, status)
SELECT a.track_id, sqlc.arg('to_frame_id'), a.x, a.y, a.width, a.height, 'human', 'verified'
FROM annotations a
WHERE a.frame_id = sqlc.arg('from_frame_id')
    AND a.status <> 'rejected'
    AND (cardinality(sqlc.arg('track_ids')::bigint[]) = 0 OR a.track_id = ANY(sqlc.arg('track_ids')::bigint[]))
ON CONFLICT (track_id, frame_id) DO NOTHING
RETURNING *;

-- name: SetFrameStatus :one
UPDATE frames SET status = $2 WHERE id = $1 RETURNING status;

-- name: CountFrameAnnotations :one
SELECT count(*)::int FROM annotations WHERE frame_id = $1 AND status <> 'rejected';

-- name: ClearEmptyFrame :exec
UPDATE frames SET status = 'unlabeled' WHERE id = $1 AND status = 'empty';
