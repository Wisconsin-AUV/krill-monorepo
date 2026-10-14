-- name: ClaimClip :execrows
-- Takes the clip unless someone else claimed it and is still active.
INSERT INTO clip_claims (clip_id, user_id) VALUES (@clip_id, @user_id)
ON CONFLICT (clip_id) DO UPDATE SET user_id = excluded.user_id, active_at = now()
WHERE clip_claims.user_id = excluded.user_id OR clip_claims.active_at < @stale_before;

-- name: TakeClip :exec
INSERT INTO clip_claims (clip_id, user_id) VALUES (@clip_id, @user_id)
ON CONFLICT (clip_id) DO UPDATE SET user_id = excluded.user_id, active_at = now();

-- name: RefreshClipClaim :exec
UPDATE clip_claims SET active_at = now() WHERE clip_id = @clip_id AND user_id = @user_id;

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
    AND c.deleted_at IS NULL
    AND v.deleted_at IS NULL
    AND EXISTS (SELECT 1 FROM frames f WHERE f.clip_id = c.id AND f.status = 'unlabeled')
ORDER BY cc.active_at DESC;

-- name: ListClipProgress :many
SELECT
    c.id, c.video_id, v.name AS video_name, c.idx, c.frame_count,
    (SELECT count(*) FROM frames f WHERE f.clip_id = c.id AND f.status <> 'unlabeled')::int AS labeled_frame_count
FROM clips c
JOIN videos v ON v.id = c.video_id
WHERE v.status = 'ready' AND c.deleted_at IS NULL AND v.deleted_at IS NULL
ORDER BY v.created_at DESC, v.id DESC, c.idx;

-- name: NextClip :one
-- Started clips that were abandoned come first so they get finished. After
-- that, clips come from the video with the fewest clips worked on, so labels
-- spread across as much footage as possible.
WITH worked AS (
    SELECT c.video_id, count(*) AS clips
    FROM clips c
    WHERE c.deleted_at IS NULL AND (
        EXISTS (SELECT 1 FROM clip_claims cc WHERE cc.clip_id = c.id)
        OR EXISTS (SELECT 1 FROM frames f WHERE f.clip_id = c.id AND f.status <> 'unlabeled')
    )
    GROUP BY c.video_id
)
SELECT c.id
FROM clips c
JOIN videos v ON v.id = c.video_id
LEFT JOIN clip_claims cc ON cc.clip_id = c.id
LEFT JOIN worked w ON w.video_id = c.video_id
WHERE v.status = 'ready'
    AND c.deleted_at IS NULL
    AND v.deleted_at IS NULL
    AND c.id <> @skip_clip_id
    AND (cc.clip_id IS NULL OR cc.active_at < @stale_before)
    AND EXISTS (SELECT 1 FROM frames f WHERE f.clip_id = c.id AND f.status = 'unlabeled')
ORDER BY
    EXISTS (SELECT 1 FROM frames f WHERE f.clip_id = c.id AND f.status <> 'unlabeled') DESC,
    coalesce(w.clips, 0),
    random()
LIMIT 1
FOR UPDATE OF c SKIP LOCKED;

-- name: LockClipClaim :one
SELECT cc.user_id, cc.active_at, u.name
FROM clip_claims cc
JOIN users u ON u.id = cc.user_id
WHERE cc.clip_id = @clip_id
FOR UPDATE OF cc;

-- name: ListContentClips :many
-- Clips for the admin panel, by video or by who holds them. With neither
-- filter it lists every claimed clip.
SELECT
    c.id, c.video_id, v.name AS video_name, c.idx, c.frame_count, c.embedded_at, c.embed_model,
    (SELECT count(*) FROM frames f WHERE f.clip_id = c.id AND f.status <> 'unlabeled')::int AS labeled_frame_count,
    (SELECT count(*) FROM annotations a JOIN tracks t ON t.id = a.track_id WHERE t.clip_id = c.id)::int AS box_count,
    cc.active_at AS claim_active_at,
    u.id AS claimant_id, u.name AS claimant_name, u.username AS claimant_username,
    u.role AS claimant_role, u.created_at AS claimant_created_at
FROM clips c
JOIN videos v ON v.id = c.video_id
LEFT JOIN clip_claims cc ON cc.clip_id = c.id
LEFT JOIN users u ON u.id = cc.user_id
WHERE c.deleted_at IS NULL
    AND v.deleted_at IS NULL
    AND (sqlc.narg(video_id)::bigint IS NULL OR c.video_id = sqlc.narg(video_id)::bigint)
    AND (sqlc.narg(claimant)::uuid IS NULL OR cc.user_id = sqlc.narg(claimant)::uuid)
    AND (sqlc.narg(video_id)::bigint IS NOT NULL OR sqlc.narg(claimant)::uuid IS NOT NULL OR cc.clip_id IS NOT NULL)
ORDER BY v.created_at DESC, v.id DESC, c.idx
LIMIT @lim::int;

-- name: LockLiveClips :many
SELECT c.id, c.idx, v.name AS video_name, cc.user_id AS claimant_id, u.name AS claimant_name
FROM clips c
JOIN videos v ON v.id = c.video_id
LEFT JOIN clip_claims cc ON cc.clip_id = c.id
LEFT JOIN users u ON u.id = cc.user_id
WHERE c.id = ANY(@ids::bigint[]) AND c.deleted_at IS NULL AND v.deleted_at IS NULL
ORDER BY c.id
FOR UPDATE OF c;

-- name: AssignClips :execrows
INSERT INTO clip_claims (clip_id, user_id)
SELECT unnest(@clip_ids::bigint[]), @user_id::uuid
ON CONFLICT (clip_id) DO UPDATE SET user_id = excluded.user_id, active_at = now();
