-- name: CreateInvite :one
INSERT INTO invites (token_hash, role, max_uses, note, email, expires_at, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListInvites :many
SELECT sqlc.embed(i), c.name AS creator_name, c.username AS creator_username,
    c.role AS creator_role, c.created_at AS creator_created_at
FROM invites i
LEFT JOIN users c ON c.id = i.created_by
ORDER BY i.created_at DESC, i.id DESC;

-- name: GetInvite :one
SELECT sqlc.embed(i), c.name AS creator_name, c.username AS creator_username,
    c.role AS creator_role, c.created_at AS creator_created_at
FROM invites i
LEFT JOIN users c ON c.id = i.created_by
WHERE i.id = $1;

-- name: ListInviteUses :many
SELECT iu.invite_id, iu.used_at, u.id, u.name, u.username, u.role, u.created_at
FROM invite_uses iu
JOIN users u ON u.id = iu.user_id
WHERE iu.invite_id = ANY(@invite_ids::bigint[])
ORDER BY iu.used_at;

-- name: RevokeInvite :one
UPDATE invites SET revoked_at = coalesce(revoked_at, now()) WHERE id = $1 RETURNING *;

-- name: GetInviteByToken :one
SELECT * FROM invites WHERE token_hash = $1;

-- name: UseInvite :one
-- Counts a use only while the invite is live. The row stays locked until the
-- sign-up commits, so two sign-ups can't share the last use.
UPDATE invites SET uses = uses + 1
WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now() AND uses < max_uses
RETURNING *;

-- name: AddInviteUse :exec
INSERT INTO invite_uses (invite_id, user_id) VALUES ($1, $2);

-- name: ListApprovedEmails :many
SELECT sqlc.embed(a),
    c.name AS creator_name, c.username AS creator_username,
    c.role AS creator_role, c.created_at AS creator_created_at,
    u.name AS user_name, u.username AS user_username,
    u.role AS user_role, u.created_at AS user_created_at
FROM approved_emails a
LEFT JOIN users c ON c.id = a.created_by
LEFT JOIN users u ON u.id = a.used_by
ORDER BY a.used_at IS NOT NULL, a.created_at DESC, a.email;

-- name: AddApprovedEmail :one
INSERT INTO approved_emails (email, role, created_by) VALUES ($1, $2, $3)
ON CONFLICT (email) DO UPDATE
SET role = excluded.role, created_by = excluded.created_by, created_at = now(), used_by = NULL, used_at = NULL
RETURNING *;

-- name: RemoveApprovedEmail :one
DELETE FROM approved_emails WHERE email = $1 RETURNING *;

-- name: LockApprovedEmail :one
SELECT * FROM approved_emails WHERE email = $1 AND used_at IS NULL FOR UPDATE;

-- name: UseApprovedEmail :exec
UPDATE approved_emails SET used_by = $2, used_at = now() WHERE email = $1;

-- name: ApprovedSignupOpen :one
SELECT EXISTS (SELECT 1 FROM approved_emails WHERE used_at IS NULL);
