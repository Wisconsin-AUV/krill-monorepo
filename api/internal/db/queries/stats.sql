-- name: Leaderboard :many
WITH contributions AS (
    SELECT a.created_by AS user_id, 1 AS boxes, 0 AS frames
    FROM annotations a
    JOIN tracks t ON t.id = a.track_id
    JOIN live_clips c ON c.id = t.clip_id
    WHERE a.created_by IS NOT NULL
        AND (sqlc.narg('since')::timestamptz IS NULL OR a.created_at >= sqlc.narg('since'))
    UNION ALL
    SELECT f.status_by, 0, 1
    FROM frames f
    JOIN live_clips c ON c.id = f.clip_id
    WHERE f.status_by IS NOT NULL
        AND (sqlc.narg('since')::timestamptz IS NULL OR f.status_at >= sqlc.narg('since'))
)
SELECT
    u.id, u.name, u.username, u.role, u.created_at,
    sum(c.boxes)::bigint AS boxes,
    sum(c.frames)::bigint AS frames
FROM contributions c
JOIN users u ON u.id = c.user_id
GROUP BY u.id
ORDER BY sum(c.boxes) + sum(c.frames) DESC, sum(c.frames) DESC, u.name;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1;

-- name: UserContributionDays :many
SELECT day::text AS day, count(*)::bigint AS count
FROM (
    SELECT (a.created_at AT TIME ZONE sqlc.arg('time_zone')::text)::date AS day
    FROM annotations a
    JOIN tracks t ON t.id = a.track_id
    JOIN live_clips c ON c.id = t.clip_id
    WHERE a.created_by = sqlc.arg('user_id')::uuid
    UNION ALL
    SELECT (f.status_at AT TIME ZONE sqlc.arg('time_zone')::text)::date
    FROM frames f
    JOIN live_clips c ON c.id = f.clip_id
    WHERE f.status_by = sqlc.arg('user_id')::uuid
) c
GROUP BY day
ORDER BY day;

-- name: UserLabelTypeCounts :many
SELECT lt.id, lt.name, lt.color, count(*)::bigint AS boxes
FROM annotations a
JOIN tracks t ON t.id = a.track_id
JOIN live_clips c ON c.id = t.clip_id
JOIN label_types lt ON lt.id = t.label_type_id
WHERE a.created_by = sqlc.arg('user_id')::uuid
GROUP BY lt.id
ORDER BY boxes DESC, lt.position;

-- name: UserClipCount :one
SELECT count(DISTINCT clip_id)::bigint
FROM (
    SELECT t.clip_id
    FROM annotations a
    JOIN tracks t ON t.id = a.track_id
    WHERE a.created_by = sqlc.arg('user_id')::uuid
    UNION ALL
    SELECT clip_id FROM frames WHERE status_by = sqlc.arg('user_id')::uuid
) c
WHERE c.clip_id IN (SELECT id FROM live_clips);

-- name: DriftByLabelType :many
SELECT
    t.label_type_id,
    count(*)::bigint AS proposals,
    count(*) FILTER (WHERE a.drift <> 'ok')::bigint AS flagged
FROM annotations a
JOIN tracks t ON t.id = a.track_id
JOIN live_clips c ON c.id = t.clip_id
WHERE a.drift IS NOT NULL
GROUP BY t.label_type_id;

-- name: DatabaseSize :one
SELECT pg_database_size(current_database())::bigint;
