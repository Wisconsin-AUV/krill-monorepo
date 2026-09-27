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
