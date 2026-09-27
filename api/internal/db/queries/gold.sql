-- name: ListFrameVerifiedBoxes :many
SELECT t.label_type_id, t.attributes, a.x, a.y, a.width, a.height
FROM annotations a
JOIN tracks t ON t.id = a.track_id
WHERE a.frame_id = $1 AND a.status = 'verified'
ORDER BY a.id;

-- name: InsertGoldFrame :exec
INSERT INTO gold_frames (frame_id, boxes, created_by) VALUES (@frame_id, @boxes, @created_by::uuid)
ON CONFLICT (frame_id) DO NOTHING;

-- name: DeleteGoldFrame :exec
DELETE FROM gold_frames WHERE frame_id = $1;

-- name: GetGoldFrame :one
SELECT g.frame_id, g.boxes, f.idx, f.video_id, v.width, v.height
FROM gold_frames g
JOIN frames f ON f.id = g.frame_id
JOIN videos v ON v.id = f.video_id
WHERE g.frame_id = $1;

-- name: ListClipGoldFrames :many
SELECT g.frame_id
FROM gold_frames g
JOIN frames f ON f.id = g.frame_id
WHERE f.clip_id = $1;

-- name: HasGoldAttempt :one
SELECT EXISTS (SELECT 1 FROM gold_attempts WHERE frame_id = @frame_id AND user_id = @user_id);

-- name: InsertGoldAttempt :execrows
INSERT INTO gold_attempts (
    frame_id, user_id, boxes, reference_count, answer_count, matched, correct, iou_sum, score
) VALUES (
    @frame_id, @user_id, @boxes, @reference_count, @answer_count, @matched, @correct, @iou_sum, @score
)
ON CONFLICT (frame_id, user_id) DO NOTHING;

-- name: FramesSinceGoldCheck :one
SELECT count(*)::int
FROM frames
WHERE status_by = @user_id::uuid
    AND status_at > coalesce(
        (SELECT max(created_at) FROM gold_attempts WHERE user_id = @user_id::uuid),
        '-infinity'::timestamptz
    );

-- name: NextGoldFrame :one
-- Skips gold frames from clips the labeler worked on, since they may remember
-- the answer. The order is shuffled per labeler but stable, so skipping a
-- check does not hand out an easier one.
SELECT g.frame_id
FROM gold_frames g
JOIN frames f ON f.id = g.frame_id
JOIN videos v ON v.id = f.video_id
WHERE v.status = 'ready'
    AND g.created_by IS DISTINCT FROM @user_id::uuid
    AND NOT EXISTS (SELECT 1 FROM gold_attempts ga WHERE ga.frame_id = g.frame_id AND ga.user_id = @user_id::uuid)
    AND NOT EXISTS (SELECT 1 FROM clip_claims cc WHERE cc.clip_id = f.clip_id AND cc.user_id = @user_id::uuid)
    AND NOT EXISTS (SELECT 1 FROM frames wf WHERE wf.clip_id = f.clip_id AND wf.status_by = @user_id::uuid)
    AND NOT EXISTS (
        SELECT 1
        FROM annotations a
        JOIN tracks t ON t.id = a.track_id
        WHERE t.clip_id = f.clip_id AND (a.created_by = @user_id::uuid OR a.updated_by = @user_id::uuid)
    )
ORDER BY md5(g.frame_id::text || @user_id::text)
LIMIT 1;

-- name: ListGoldLabelers :many
SELECT
    u.id, u.name, u.username, u.role, u.created_at,
    count(*)::int AS attempts,
    sum(ga.correct)::bigint AS correct,
    sum(ga.reference_count + ga.answer_count - ga.matched)::bigint AS scored,
    sum(ga.matched)::bigint AS matched,
    sum(ga.iou_sum)::float8 AS iou_sum
FROM gold_attempts ga
JOIN users u ON u.id = ga.user_id
GROUP BY u.id;

-- name: ListGoldFrames :many
SELECT
    g.frame_id, f.clip_id, f.idx, f.video_id, v.name AS video_name,
    jsonb_array_length(g.boxes)::int AS box_count,
    count(ga.user_id)::int AS attempts,
    coalesce(avg(ga.score), 0)::float8 AS mean_score
FROM gold_frames g
JOIN frames f ON f.id = g.frame_id
JOIN videos v ON v.id = f.video_id
LEFT JOIN gold_attempts ga ON ga.frame_id = g.frame_id
GROUP BY g.frame_id, f.id, v.id
ORDER BY g.created_at DESC, g.frame_id DESC;
