-- name: GetClip :one
SELECT * FROM clips WHERE id = $1;

-- name: GetClipLabelStats :one
SELECT
    (SELECT count(*) FROM frames f WHERE f.clip_id = $1 AND f.status <> 'unlabeled')::int AS labeled_frame_count,
    (SELECT count(*) FROM annotations a JOIN tracks t ON t.id = a.track_id WHERE t.clip_id = $1)::int AS box_count;

-- name: ListClipFrames :many
SELECT * FROM frames WHERE clip_id = $1 ORDER BY idx;

-- name: GetNeighbourClips :one
SELECT
    coalesce((SELECT p.id FROM clips p WHERE p.video_id = c.video_id AND p.idx = c.idx - 1), 0)::bigint AS previous_id,
    coalesce((SELECT n.id FROM clips n WHERE n.video_id = c.video_id AND n.idx = c.idx + 1), 0)::bigint AS next_id
FROM clips c
WHERE c.id = $1;

-- name: SetClipEmbedded :exec
UPDATE clips SET embedded_at = now(), embed_model = $2 WHERE id = $1;

-- name: SubtractClipFrames :one
UPDATE videos v SET frame_count = v.frame_count - c.frame_count
FROM clips c
WHERE c.id = $1 AND v.id = c.video_id
RETURNING v.id AS video_id;

-- name: DeleteClip :one
DELETE FROM clips WHERE id = $1 RETURNING idx;

-- name: ParkClipsAfter :exec
-- The unique (video_id, idx) constraint is checked row by row, so shifting
-- every later clip down by one in a single UPDATE can collide with a row that
-- hasn't moved yet. Parking them at negative indexes first avoids that.
UPDATE clips SET idx = -idx WHERE video_id = $1 AND idx > $2;

-- name: UnparkClips :exec
UPDATE clips SET idx = -idx - 1 WHERE video_id = $1 AND idx < 0;
