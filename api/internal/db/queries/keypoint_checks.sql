-- name: ListHandPosedBoxes :many
-- Boxes a labeler placed keypoints on.
SELECT
    a.id, a.frame_id, f.idx AS frame_idx, f.video_id, c.id AS clip_id, a.x, a.y, a.width, a.height, a.updated_at,
    a.keypoints, v.width AS video_width, v.height AS video_height
FROM annotations a
JOIN tracks t ON t.id = a.track_id
JOIN live_clips c ON c.id = t.clip_id
JOIN videos v ON v.id = c.video_id
JOIN frames f ON f.id = a.frame_id
WHERE t.label_type_id = $1 AND a.source = 'human' AND a.keypoints IS NOT NULL
    AND a.status <> 'rejected' AND v.status = 'ready'
ORDER BY f.video_id, f.idx, a.id;

-- name: ListKeypointCheckTypes :many
-- Types that could be checked, with how many boxes ListHandPosedBoxes
-- lists for each.
SELECT
    sqlc.embed(lt),
    (
        SELECT count(*)
        FROM annotations a
        JOIN tracks t ON t.id = a.track_id
        JOIN live_clips c ON c.id = t.clip_id
        JOIN videos v ON v.id = c.video_id
        WHERE t.label_type_id = lt.id AND a.source = 'human' AND a.keypoints IS NOT NULL
            AND a.status <> 'rejected' AND v.status = 'ready'
    )::int AS posed_box_count
FROM label_types lt
WHERE lt.keypoints <> '[]' AND lt.reference_key IS NOT NULL
ORDER BY lt.position, lt.id;

-- name: CountHandPosedBoxes :one
SELECT count(*)::int
FROM annotations a
JOIN tracks t ON t.id = a.track_id
JOIN live_clips c ON c.id = t.clip_id
JOIN videos v ON v.id = c.video_id
WHERE t.label_type_id = $1 AND a.source = 'human' AND a.keypoints IS NOT NULL
    AND a.status <> 'rejected' AND v.status = 'ready';

-- name: LockRunningKeypointCheck :one
SELECT * FROM keypoint_checks WHERE label_type_id = $1 AND status = 'running' FOR UPDATE;

-- name: CreateKeypointCheck :one
INSERT INTO keypoint_checks (label_type_id, job_id, box_count, created_by)
VALUES (@label_type_id, @job_id, @box_count, sqlc.narg(created_by)::uuid)
RETURNING *;

-- name: GetRunningKeypointCheckByJob :one
SELECT * FROM keypoint_checks WHERE job_id = $1 AND status = 'running';

-- name: FinishKeypointCheck :exec
UPDATE keypoint_checks SET
    status = 'succeeded',
    finished_at = now(),
    box_count = @box_count,
    found_count = @found_count,
    turned_count = @turned_count,
    result = @result
WHERE id = @id AND status = 'running';

-- name: FailKeypointCheck :exec
UPDATE keypoint_checks SET status = 'failed', finished_at = now(), error = @error
WHERE id = @id AND status = 'running';

-- name: ListLatestKeypointChecks :many
-- Each type's running check and its latest finished one.
SELECT DISTINCT ON (k.label_type_id, k.status = 'running')
    sqlc.embed(k),
    u.id AS creator_id, u.name AS creator_name, u.username AS creator_username,
    u.role AS creator_role, u.created_at AS creator_created_at
FROM keypoint_checks k
LEFT JOIN users u ON u.id = k.created_by
ORDER BY k.label_type_id, k.status = 'running', k.created_at DESC, k.id DESC;
