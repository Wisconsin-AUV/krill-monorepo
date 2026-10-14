-- name: LabelerFrameCounts :many
SELECT
    status_by::uuid AS user_id,
    count(*) FILTER (WHERE status = 'labeled')::bigint AS labeled,
    count(*) FILTER (WHERE status = 'empty')::bigint AS empty
FROM frames
WHERE status_by IS NOT NULL
    AND (sqlc.narg('since')::timestamptz IS NULL OR status_at >= sqlc.narg('since'))
GROUP BY status_by;

-- name: LabelerBoxesDrawn :many
SELECT created_by::uuid AS user_id, count(*)::bigint AS boxes
FROM annotations
WHERE created_by IS NOT NULL
    AND (sqlc.narg('since')::timestamptz IS NULL OR created_at >= sqlc.narg('since'))
GROUP BY created_by;

-- name: LabelerProposalCounts :many
SELECT
    user_id::uuid AS user_id,
    source,
    count(*) FILTER (WHERE kind = 'accepted')::bigint AS accepted,
    count(*) FILTER (WHERE kind = 'edited')::bigint AS edited,
    count(*) FILTER (WHERE kind = 'rejected')::bigint AS rejected
FROM box_events
WHERE kind IN ('accepted', 'edited', 'rejected')
    AND user_id IS NOT NULL
    AND (sqlc.narg('since')::timestamptz IS NULL OR created_at >= sqlc.narg('since'))
GROUP BY user_id, source
ORDER BY source;

-- name: LabelerCorrections :many
-- Corrections count against the period the frame was marked in, not when the
-- fix happened. A frame a correction reopened is no longer credited to its
-- labeler, so it is added back to the frames they marked.
WITH fixed AS (
    SELECT labeler_id, frame_id, count(DISTINCT track_id) AS boxes
    FROM box_events
    WHERE labeler_id IS NOT NULL
        AND (sqlc.narg('since')::timestamptz IS NULL OR marked_at >= sqlc.narg('since'))
    GROUP BY labeler_id, frame_id
)
SELECT
    x.labeler_id::uuid AS user_id,
    count(*)::bigint AS frames,
    sum(x.boxes)::bigint AS boxes,
    count(*) FILTER (WHERE f.status_by IS DISTINCT FROM x.labeler_id)::bigint AS reopened
FROM fixed x
JOIN frames f ON f.id = x.frame_id
GROUP BY x.labeler_id;

-- name: LabelerActiveSeconds :many
-- See activeGapCap in internal/insight for how this counts.
WITH acts AS (
    SELECT created_by AS user_id, created_at AS at FROM annotations
    WHERE created_by IS NOT NULL
        AND (sqlc.narg('since')::timestamptz IS NULL OR created_at >= sqlc.narg('since'))
    UNION ALL
    SELECT updated_by, updated_at FROM annotations
    WHERE updated_by IS NOT NULL AND updated_at > created_at
        AND (sqlc.narg('since')::timestamptz IS NULL OR updated_at >= sqlc.narg('since'))
    UNION ALL
    SELECT status_by, status_at FROM frames
    WHERE status_by IS NOT NULL
        AND (sqlc.narg('since')::timestamptz IS NULL OR status_at >= sqlc.narg('since'))
    UNION ALL
    SELECT user_id, created_at FROM box_events
    WHERE user_id IS NOT NULL
        AND (sqlc.narg('since')::timestamptz IS NULL OR created_at >= sqlc.narg('since'))
), gaps AS (
    SELECT user_id, extract(epoch FROM at - lag(at) OVER (PARTITION BY user_id ORDER BY at)) AS gap
    FROM acts
)
SELECT user_id::uuid AS user_id, coalesce(sum(least(gap, @gap_cap::float8)), 0)::float8 AS seconds
FROM gaps
GROUP BY user_id;

-- name: ListUsersByID :many
SELECT * FROM users WHERE id = ANY(@ids::uuid[]);

-- name: LabelerTrend :many
WITH acts AS (
    SELECT created_at AS at, 'box' AS kind FROM annotations
    WHERE created_by = @user_id::uuid
        AND (sqlc.narg('since')::timestamptz IS NULL OR created_at >= sqlc.narg('since'))
    UNION ALL
    SELECT updated_at, 'edit' FROM annotations
    WHERE updated_by = @user_id::uuid AND updated_at > created_at
        AND (sqlc.narg('since')::timestamptz IS NULL OR updated_at >= sqlc.narg('since'))
    UNION ALL
    SELECT status_at, 'frame' FROM frames
    WHERE status_by = @user_id::uuid
        AND (sqlc.narg('since')::timestamptz IS NULL OR status_at >= sqlc.narg('since'))
    UNION ALL
    SELECT created_at, CASE WHEN kind IN ('accepted', 'edited') THEN 'kept' ELSE 'event' END FROM box_events
    WHERE user_id = @user_id::uuid
        AND (sqlc.narg('since')::timestamptz IS NULL OR created_at >= sqlc.narg('since'))
), gaps AS (
    SELECT at, kind, extract(epoch FROM at - lag(at) OVER (ORDER BY at)) AS gap
    FROM acts
), activity AS (
    SELECT
        date_trunc(@unit::text, at, @time_zone::text) AS bucket,
        count(*) FILTER (WHERE kind = 'frame') AS frames,
        count(*) FILTER (WHERE kind = 'box') AS boxes,
        count(*) FILTER (WHERE kind = 'kept') AS kept,
        coalesce(sum(least(gap, @gap_cap::float8)), 0) AS seconds
    FROM gaps
    GROUP BY 1
), fixed AS (
    SELECT date_trunc(@unit::text, marked_at, @time_zone::text) AS bucket, count(DISTINCT frame_id) AS frames
    FROM box_events
    WHERE labeler_id = @user_id::uuid
        AND (sqlc.narg('since')::timestamptz IS NULL OR marked_at >= sqlc.narg('since'))
    GROUP BY 1
)
SELECT
    coalesce(a.bucket, x.bucket)::timestamptz AS bucket,
    coalesce(a.frames, 0)::bigint AS frames,
    coalesce(a.boxes, 0)::bigint AS boxes,
    coalesce(a.kept, 0)::bigint AS kept,
    coalesce(x.frames, 0)::bigint AS frames_corrected,
    coalesce(a.seconds, 0)::float8 AS seconds
FROM activity a
FULL JOIN fixed x ON x.bucket = a.bucket
ORDER BY 1;

-- name: LabelerCorrectionKinds :many
SELECT kind, count(DISTINCT (frame_id, track_id))::bigint AS boxes
FROM box_events
WHERE labeler_id = @user_id::uuid
    AND (sqlc.narg('since')::timestamptz IS NULL OR marked_at >= sqlc.narg('since'))
GROUP BY kind
ORDER BY boxes DESC;

-- name: CoverageVideos :many
SELECT
    v.id, v.name, v.split, v.created_at,
    count(f.id)::int AS frames,
    count(f.id) FILTER (WHERE f.status = 'labeled')::int AS labeled,
    count(f.id) FILTER (WHERE f.status = 'empty')::int AS empty
FROM videos v
LEFT JOIN frames f ON f.video_id = v.id
WHERE v.status = 'ready'
GROUP BY v.id
ORDER BY v.created_at DESC, v.id DESC;

-- name: CoverageCells :many
SELECT
    f.video_id,
    t.label_type_id,
    count(*) FILTER (WHERE a.status = 'verified')::bigint AS verified,
    count(*) FILTER (WHERE a.status = 'proposed')::bigint AS proposed,
    count(DISTINCT a.frame_id) FILTER (WHERE a.status = 'verified')::bigint AS frames
FROM annotations a
JOIN tracks t ON t.id = a.track_id
JOIN frames f ON f.id = a.frame_id
JOIN videos v ON v.id = f.video_id
WHERE v.status = 'ready' AND a.status <> 'rejected'
GROUP BY f.video_id, t.label_type_id;

-- name: ListStuckClips :many
SELECT
    c.id AS clip_id, c.idx AS clip_idx, c.frame_count, v.id AS video_id, v.name AS video_name,
    cc.active_at, sqlc.embed(u),
    (SELECT count(*) FROM frames f WHERE f.clip_id = c.id AND f.status <> 'unlabeled')::int AS labeled_frames,
    (SELECT count(*) FROM annotations a JOIN tracks t ON t.id = a.track_id WHERE t.clip_id = c.id AND a.status <> 'rejected')::int AS boxes
FROM clip_claims cc
JOIN clips c ON c.id = cc.clip_id
JOIN videos v ON v.id = c.video_id
JOIN users u ON u.id = cc.user_id
WHERE cc.active_at < @idle_before
    AND v.status = 'ready'
    AND EXISTS (SELECT 1 FROM frames f WHERE f.clip_id = c.id AND f.status = 'unlabeled')
ORDER BY cc.active_at, c.id;

-- name: ReleaseStuckClaims :many
-- Only claims still idle since idle_before go, so a claimant who came back
-- after the list loaded keeps their clip.
WITH released AS (
    DELETE FROM clip_claims
    WHERE clip_id = ANY(@clip_ids::bigint[]) AND active_at < @idle_before
    RETURNING clip_id, user_id, active_at
)
SELECT r.clip_id, r.active_at, u.id AS user_id, u.name AS user_name
FROM released r
JOIN users u ON u.id = r.user_id
ORDER BY r.clip_id;
