-- name: ClaimClip :execrows
-- Takes the clip unless someone else claimed it and is still active.
INSERT INTO clip_claims (clip_id, user_id) VALUES (@clip_id, @user_id)
ON CONFLICT (clip_id) DO UPDATE SET user_id = excluded.user_id, active_at = now()
WHERE clip_claims.user_id = excluded.user_id OR clip_claims.active_at < @stale_before;

-- name: ReleaseClipClaim :exec
DELETE FROM clip_claims WHERE clip_id = @clip_id AND user_id = @user_id;

-- name: ListClipClaims :many
SELECT cc.clip_id, cc.active_at, sqlc.embed(u)
FROM clip_claims cc
JOIN users u ON u.id = cc.user_id
WHERE cc.clip_id = ANY(@clip_ids::bigint[]);

-- name: ListOpenClips :many
SELECT
    sqlc.embed(c),
    sqlc.embed(v),
    (SELECT count(*) FROM frames f WHERE f.clip_id = c.id AND f.status <> 'unlabeled')::int AS labeled_frame_count,
    (SELECT count(*) FROM annotations a JOIN tracks t ON t.id = a.track_id WHERE t.clip_id = c.id)::int AS box_count
FROM clip_claims cc
JOIN clips c ON c.id = cc.clip_id
JOIN videos v ON v.id = c.video_id
WHERE cc.user_id = @user_id
    AND v.status = 'ready'
    AND EXISTS (SELECT 1 FROM frames f WHERE f.clip_id = c.id AND f.status = 'unlabeled')
ORDER BY cc.active_at DESC;

-- name: ListClipProgress :many
SELECT
    c.id, c.video_id, v.name AS video_name, c.idx, c.frame_count,
    (SELECT count(*) FROM frames f WHERE f.clip_id = c.id AND f.status <> 'unlabeled')::int AS labeled_frame_count
FROM clips c
JOIN videos v ON v.id = c.video_id
WHERE v.status = 'ready'
ORDER BY v.created_at DESC, v.id DESC, c.idx;

-- name: NextClip :one
-- Started clips that were abandoned come first so they get finished. After
-- that, clips come from the video with the fewest clips worked on, so labels
-- spread across as much footage as possible.
WITH worked AS (
    SELECT c.video_id, count(*) AS clips
    FROM clips c
    WHERE EXISTS (SELECT 1 FROM clip_claims cc WHERE cc.clip_id = c.id)
        OR EXISTS (SELECT 1 FROM frames f WHERE f.clip_id = c.id AND f.status <> 'unlabeled')
    GROUP BY c.video_id
)
SELECT c.id
FROM clips c
JOIN videos v ON v.id = c.video_id
LEFT JOIN clip_claims cc ON cc.clip_id = c.id
LEFT JOIN worked w ON w.video_id = c.video_id
WHERE v.status = 'ready'
    AND c.id <> @skip_clip_id
    AND (cc.clip_id IS NULL OR cc.active_at < @stale_before)
    AND EXISTS (SELECT 1 FROM frames f WHERE f.clip_id = c.id AND f.status = 'unlabeled')
ORDER BY
    EXISTS (SELECT 1 FROM frames f WHERE f.clip_id = c.id AND f.status <> 'unlabeled') DESC,
    coalesce(w.clips, 0),
    random()
LIMIT 1
FOR UPDATE OF c SKIP LOCKED;
