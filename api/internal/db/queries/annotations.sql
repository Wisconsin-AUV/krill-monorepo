-- name: GetFrame :one
SELECT f.* FROM frames f
JOIN clips c ON c.id = f.clip_id
JOIN videos v ON v.id = c.video_id
WHERE f.id = $1 AND c.deleted_at IS NULL AND v.deleted_at IS NULL;

-- name: CreateTrack :one
INSERT INTO tracks (clip_id, label_type_id, attributes)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetTrack :one
SELECT t.* FROM tracks t
JOIN clips c ON c.id = t.clip_id
JOIN videos v ON v.id = c.video_id
WHERE t.id = $1 AND c.deleted_at IS NULL AND v.deleted_at IS NULL;

-- name: UpdateTrack :one
UPDATE tracks SET label_type_id = @label_type_id, attributes = @attributes, class_edited = class_edited OR @class_edited
WHERE id = @id
RETURNING *;

-- name: DeleteTrack :one
DELETE FROM tracks t
USING clips c, videos v
WHERE t.id = $1 AND c.id = t.clip_id AND v.id = c.video_id AND c.deleted_at IS NULL AND v.deleted_at IS NULL
RETURNING t.clip_id;

-- name: ListClipTracks :many
SELECT * FROM tracks WHERE clip_id = $1 ORDER BY id;

-- name: ListClipAnnotations :many
SELECT a.*
FROM annotations a
JOIN tracks t ON t.id = a.track_id
WHERE t.clip_id = $1
ORDER BY a.frame_id, a.track_id;

-- name: UpsertAnnotation :one
INSERT INTO annotations (track_id, frame_id, x, y, width, height, source, status, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, 'human', 'verified', sqlc.arg('user_id')::uuid, sqlc.arg('user_id')::uuid)
ON CONFLICT (track_id, frame_id) DO UPDATE SET
    x = excluded.x,
    y = excluded.y,
    width = excluded.width,
    height = excluded.height,
    source = 'human',
    status = 'verified',
    model_version = '',
    confidence = NULL,
    drift = CASE WHEN annotations.drift = 'flagged' THEN 'reviewed' ELSE annotations.drift END,
    updated_by = excluded.updated_by,
    updated_at = now()
RETURNING *;

-- name: DeleteAnnotation :one
DELETE FROM annotations a
WHERE a.track_id = $1 AND a.frame_id = $2
    AND EXISTS (
        SELECT 1 FROM tracks t JOIN live_clips c ON c.id = t.clip_id WHERE t.id = a.track_id
    )
RETURNING (SELECT t.clip_id FROM tracks t WHERE t.id = a.track_id)::bigint AS clip_id;

-- name: CountTrackAnnotations :one
SELECT count(*)::int FROM annotations WHERE track_id = $1;

-- name: CopyAnnotations :many
INSERT INTO annotations (track_id, frame_id, x, y, width, height, source, status, created_by, updated_by)
SELECT a.track_id, sqlc.arg('to_frame_id'), a.x, a.y, a.width, a.height, 'human', 'verified', sqlc.arg('user_id')::uuid, sqlc.arg('user_id')::uuid
FROM annotations a
WHERE a.frame_id = sqlc.arg('from_frame_id')
    AND a.status <> 'rejected'
    AND (cardinality(sqlc.arg('track_ids')::bigint[]) = 0 OR a.track_id = ANY(sqlc.arg('track_ids')::bigint[]))
ON CONFLICT (track_id, frame_id) DO NOTHING
RETURNING *;

-- name: SetFrameStatus :one
-- Credit stays with whoever first set the current status.
UPDATE frames SET
    status = sqlc.arg('status'),
    status_by = CASE
        WHEN sqlc.arg('status') = 'unlabeled' THEN NULL
        WHEN status = sqlc.arg('status') THEN status_by
        ELSE sqlc.arg('user_id')::uuid
    END,
    status_at = CASE
        WHEN sqlc.arg('status') = 'unlabeled' THEN NULL
        WHEN status = sqlc.arg('status') THEN status_at
        ELSE now()
    END
WHERE frames.id = sqlc.arg('id')
    AND frames.clip_id IN (SELECT lc.id FROM live_clips lc)
RETURNING status, clip_id;

-- name: CountFrameAnnotations :one
SELECT count(*)::int FROM annotations WHERE frame_id = $1 AND status <> 'rejected';

-- name: ClearEmptyFrame :exec
UPDATE frames SET status = 'unlabeled', status_by = NULL, status_at = NULL
WHERE id = $1 AND status = 'empty';

-- name: ListTypeTracks :many
-- Each track comes with its first verified box, or its middle box when none
-- is verified yet.
SELECT t.id, t.clip_id, t.label_type_id, t.attributes, c.video_id,
    b.frame_id, b.idx AS frame_idx, b.x, b.y, b.width, b.height
FROM tracks t
JOIN clips c ON c.id = t.clip_id
JOIN videos v ON v.id = c.video_id
JOIN label_types lt ON lt.id = t.label_type_id
JOIN LATERAL (
    SELECT r.frame_id, r.idx, r.x, r.y, r.width, r.height
    FROM (
        SELECT a.frame_id, f.idx, a.x, a.y, a.width, a.height, a.status,
            row_number() OVER (ORDER BY f.idx) AS n,
            count(*) OVER () AS total
        FROM annotations a
        JOIN frames f ON f.id = a.frame_id
        WHERE a.track_id = t.id AND a.status <> 'rejected'
    ) r
    ORDER BY (r.status = 'verified') DESC,
        CASE WHEN r.status = 'verified' THEN r.n ELSE abs(2 * r.n - r.total - 1) END,
        r.n
    LIMIT 1
) b ON true
WHERE t.label_type_id = sqlc.arg('label_type_id')
    AND c.deleted_at IS NULL
    AND v.deleted_at IS NULL
    AND (sqlc.arg('video_id')::bigint = 0 OR c.video_id = sqlc.arg('video_id')::bigint)
    AND t.id > sqlc.arg('after_id')::bigint
    AND (NOT sqlc.arg('missing_only')::bool OR EXISTS (
        SELECT 1 FROM jsonb_array_elements(lt.attributes) d
        WHERE NOT ((d->'options') @> jsonb_build_array(t.attributes -> (d->>'name')))
    ))
ORDER BY t.id
LIMIT sqlc.arg('lim');
