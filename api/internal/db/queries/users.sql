-- name: CreateUser :one
INSERT INTO users (name, username, email, password_hash, role, slack_user_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: LockUsers :exec
LOCK TABLE users IN SHARE ROW EXCLUSIVE MODE;

-- name: CountUsers :one
SELECT count(*)::int FROM users;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByLogin :one
SELECT * FROM users WHERE username = sqlc.arg('login')::text OR email = sqlc.arg('login')::text;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserBySlackID :one
SELECT * FROM users WHERE slack_user_id = $1;

-- name: UsernameExists :one
SELECT EXISTS (SELECT 1 FROM users WHERE username = $1);

-- name: ListUsers :many
SELECT * FROM users ORDER BY created_at, id;

-- name: UpdateUserProfile :one
UPDATE users
SET name = $2, username = $3, email = $4,
    email_verified_at = CASE WHEN email = $4 THEN email_verified_at END
WHERE id = $1
RETURNING *;

-- name: SetUserRole :one
UPDATE users SET role = $2 WHERE id = $1 RETURNING *;

-- name: SetUserDisabled :one
UPDATE users SET disabled = $2 WHERE id = $1 RETURNING *;

-- name: SetUserPassword :execrows
UPDATE users SET password_hash = $2 WHERE id = $1;

-- name: LinkSlack :one
UPDATE users SET slack_user_id = $2 WHERE id = $1 RETURNING *;

-- name: TouchLogin :exec
UPDATE users SET last_login_at = now() WHERE id = $1;

-- name: DeleteUser :execrows
DELETE FROM users WHERE id = $1;

-- name: CreateSession :exec
INSERT INTO sessions (token_hash, user_id, expires_at, ip, user_agent) VALUES ($1, $2, $3, $4, $5);

-- name: LookupSession :one
-- Returns the session's active user. A session not seen for five minutes is
-- marked seen in the same statement, so most requests don't write.
WITH seen AS (
    UPDATE sessions
    SET last_seen_at = now(), ip = CASE WHEN @ip::text = '' THEN sessions.ip ELSE @ip::text END
    WHERE sessions.token_hash = @token_hash
        AND sessions.expires_at > now()
        AND sessions.last_seen_at < now() - interval '5 minutes'
)
SELECT u.* FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = @token_hash AND s.expires_at > now() AND NOT u.disabled;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash = $1;

-- name: DeleteUserSessions :exec
DELETE FROM sessions WHERE user_id = $1;

-- name: DeleteOtherSessions :exec
DELETE FROM sessions WHERE user_id = $1 AND token_hash <> $2;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at <= now();

-- name: VerifyUserEmail :one
UPDATE users SET email = $2, email_verified_at = now(), pending_email = NULL WHERE id = $1 RETURNING *;

-- name: SetPendingEmail :one
UPDATE users SET pending_email = $2 WHERE id = $1 RETURNING *;

-- name: EmailInUse :one
SELECT EXISTS (SELECT 1 FROM users WHERE email = $1 AND id <> $2);

-- name: MarkEmailVerified :one
UPDATE users SET email_verified_at = coalesce(email_verified_at, now()) WHERE id = $1 RETURNING *;

-- name: ClearPassword :exec
UPDATE users SET password_hash = NULL WHERE id = $1;

-- name: CountActiveSessions :many
SELECT user_id, count(*)::int AS sessions FROM sessions WHERE expires_at > now() GROUP BY user_id;

-- name: EndUserSessions :execrows
DELETE FROM sessions WHERE user_id = $1;

-- name: DeleteUserEmailCodes :exec
DELETE FROM email_codes WHERE user_id = $1;

-- name: ReleaseUserClaims :execrows
DELETE FROM clip_claims WHERE user_id = $1;

-- name: LockUserPair :many
SELECT * FROM users WHERE id IN (@source_id::uuid, @target_id::uuid) ORDER BY id FOR UPDATE;

-- name: SetMergedIdentity :one
UPDATE users
SET role = @role,
    slack_user_id = @slack_user_id,
    password_hash = @password_hash,
    email = @email,
    email_verified_at = @email_verified_at,
    pending_email = CASE WHEN pending_email = @email THEN NULL ELSE pending_email END,
    created_at = @created_at,
    last_login_at = @last_login_at
WHERE id = @id
RETURNING *;
