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

-- name: DeleteTrackIfEmpty :execrows
DELETE FROM tracks t
WHERE t.id = $1 AND NOT EXISTS (SELECT 1 FROM annotations a WHERE a.track_id = t.id);
