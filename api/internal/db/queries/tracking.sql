-- name: ListFramesFrom :many
SELECT * FROM frames WHERE clip_id = $1 AND idx >= $2 ORDER BY idx;

-- name: DeleteTrackProposalsFrom :exec
-- Clears what an earlier run proposed on frames the new run covers.
DELETE FROM annotations a
USING frames f
WHERE f.id = a.frame_id
    AND a.track_id = sqlc.arg('track_id')
    AND a.status = 'proposed'
    AND f.idx >= sqlc.arg('from_idx');

-- name: UpsertProposals :execrows
-- Only proposals are replaced. Boxes a labeler drew, accepted, or rejected stay.
INSERT INTO annotations (track_id, frame_id, x, y, width, height, source, status, model_version)
SELECT
    sqlc.arg('track_id'),
    unnest(sqlc.arg('frame_ids')::bigint[]),
    unnest(sqlc.arg('xs')::float8[]),
    unnest(sqlc.arg('ys')::float8[]),
    unnest(sqlc.arg('widths')::float8[]),
    unnest(sqlc.arg('heights')::float8[]),
    sqlc.arg('source'),
    'proposed',
    sqlc.arg('model_version')
ON CONFLICT (track_id, frame_id) DO UPDATE SET
    x = excluded.x,
    y = excluded.y,
    width = excluded.width,
    height = excluded.height,
    source = excluded.source,
    model_version = excluded.model_version,
    updated_at = now()
WHERE annotations.status = 'proposed';

-- name: ListTrackBoxes :many
SELECT a.id, a.x, a.y, a.width, a.height, a.status, a.drift
FROM annotations a
JOIN frames f ON f.id = a.frame_id
WHERE a.track_id = $1
ORDER BY f.idx;

-- name: SetDrift :exec
UPDATE annotations SET
    drift = CASE WHEN id = ANY(sqlc.arg('flagged_ids')::bigint[]) THEN 'flagged' ELSE 'ok' END
WHERE id = ANY(sqlc.arg('ids')::bigint[]) AND status = 'proposed';

-- name: DeleteTrackIfEmpty :execrows
DELETE FROM tracks t
WHERE t.id = $1 AND NOT EXISTS (SELECT 1 FROM annotations a WHERE a.track_id = t.id);

-- name: AcceptFrameProposals :exec
UPDATE annotations SET
    status = 'verified',
    drift = CASE WHEN drift = 'flagged' THEN 'reviewed' ELSE drift END,
    updated_by = sqlc.arg('user_id')::uuid,
    updated_at = now()
WHERE frame_id = sqlc.arg('frame_id') AND status = 'proposed';

-- name: GetTrackBox :one
SELECT x, y, width, height FROM annotations
WHERE track_id = $1 AND frame_id = $2 AND status <> 'rejected';

-- name: ListFrameBoxes :many
SELECT x, y, width, height FROM annotations
WHERE frame_id = $1 AND status <> 'rejected';
