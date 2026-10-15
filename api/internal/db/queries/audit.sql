-- name: InsertAuditEntry :exec
INSERT INTO audit_log (actor, actor_name, action, target_type, target_id, target_name, details, ip)
VALUES (@actor, @actor_name, @action, @target_type, @target_id, @target_name, @details, @ip);

-- name: ListAuditLog :many
SELECT
    a.id, a.at, a.actor_name, a.action, a.target_type, a.target_id, a.target_name, a.details, a.ip,
    u.id AS actor_id, u.name AS actor_user_name, u.username AS actor_username, u.role AS actor_role,
    u.created_at AS actor_created_at
FROM audit_log a
LEFT JOIN users u ON u.id = a.actor
WHERE (sqlc.narg(actor)::uuid IS NULL OR a.actor = sqlc.narg(actor)::uuid)
    AND (sqlc.narg(action_pattern)::text IS NULL OR a.action LIKE sqlc.narg(action_pattern)::text)
    AND (sqlc.narg(target_type)::text IS NULL OR (a.target_type = sqlc.narg(target_type)::text AND a.target_id = sqlc.narg(target_id)::text))
    AND (sqlc.narg(before_id)::bigint IS NULL OR a.id < sqlc.narg(before_id)::bigint)
ORDER BY a.id DESC
LIMIT @lim::int;

-- name: ListAuditActions :many
SELECT DISTINCT action FROM audit_log ORDER BY action;

-- name: ListAuditActors :many
SELECT u.* FROM users u
WHERE EXISTS (SELECT 1 FROM audit_log a WHERE a.actor = u.id)
ORDER BY u.name, u.username;

-- name: GetClipAuditName :one
SELECT v.name AS video_name, c.idx FROM clips c JOIN videos v ON v.id = c.video_id WHERE c.id = $1;

-- name: GetFrameAuditName :one
SELECT v.name AS video_name, f.idx FROM frames f JOIN videos v ON v.id = f.video_id WHERE f.id = $1;

-- name: GetReviewAuditName :one
SELECT v.name AS video_name, f.idx
FROM reviews r
JOIN frames f ON f.id = r.frame_id
JOIN videos v ON v.id = f.video_id
WHERE r.id = $1;
