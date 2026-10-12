-- name: EnqueueReview :exec
INSERT INTO reviews (frame_id, reason, created_by) VALUES (@frame_id, @reason, sqlc.narg(created_by)::uuid)
ON CONFLICT (frame_id) WHERE resolved_at IS NULL DO NOTHING;

-- name: GetOpenReview :one
SELECT
    r.id, r.frame_id, r.reason, r.created_at,
    u.id AS creator_id, u.name AS creator_name, u.username AS creator_username,
    u.role AS creator_role, u.created_at AS creator_created_at
FROM reviews r
LEFT JOIN users u ON u.id = r.created_by
WHERE r.frame_id = $1 AND r.resolved_at IS NULL;

-- name: ListClipOpenReviews :many
SELECT
    r.id, r.frame_id, r.reason, r.created_at,
    u.id AS creator_id, u.name AS creator_name, u.username AS creator_username,
    u.role AS creator_role, u.created_at AS creator_created_at
FROM reviews r
JOIN frames f ON f.id = r.frame_id
LEFT JOIN users u ON u.id = r.created_by
WHERE f.clip_id = $1 AND r.resolved_at IS NULL;

-- name: DeleteOwnFlag :exec
DELETE FROM reviews
WHERE frame_id = @frame_id AND resolved_at IS NULL AND reason = 'flagged' AND created_by = @user_id::uuid;

-- name: ListOpenReviews :many
SELECT
    r.id, r.frame_id, r.reason, r.created_at,
    u.id AS creator_id, u.name AS creator_name, u.username AS creator_username,
    u.role AS creator_role, u.created_at AS creator_created_at,
    f.clip_id, c.idx AS clip_idx, f.idx AS frame_idx, f.video_id, v.name AS video_name,
    l.id AS labeler_id, l.name AS labeler_name, l.username AS labeler_username,
    l.role AS labeler_role, l.created_at AS labeler_created_at
FROM reviews r
JOIN frames f ON f.id = r.frame_id
JOIN clips c ON c.id = f.clip_id
JOIN videos v ON v.id = f.video_id
LEFT JOIN users u ON u.id = r.created_by
LEFT JOIN users l ON l.id = f.status_by
WHERE r.resolved_at IS NULL AND (sqlc.narg(reason)::text IS NULL OR r.reason = sqlc.narg(reason)::text)
ORDER BY r.created_at DESC, r.id DESC;

-- name: CountOpenReviews :one
SELECT count(*)::int FROM reviews WHERE resolved_at IS NULL;

-- name: ResolveReview :execrows
UPDATE reviews
SET resolved_by = @resolved_by::uuid, resolved_at = now(), resolution = @resolution::text
WHERE id = @id AND resolved_at IS NULL;

-- name: ReviewExists :one
SELECT EXISTS (SELECT 1 FROM reviews WHERE id = $1);
