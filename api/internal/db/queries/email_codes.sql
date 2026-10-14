-- name: IssueEmailCode :one
-- Replaces the user's code for this purpose once the cooldown has passed. With
-- keep_live, a code that has not expired and went to the same address is kept.
-- No row means nothing was issued.
INSERT INTO email_codes (user_id, purpose, email, code_hash, expires_at)
VALUES ($1, $2, $3, $4, now() + make_interval(secs => sqlc.arg(ttl_seconds)::int))
ON CONFLICT (user_id, purpose) DO UPDATE
SET email = EXCLUDED.email,
    code_hash = EXCLUDED.code_hash,
    attempts = 0,
    created_at = now(),
    expires_at = EXCLUDED.expires_at
WHERE email_codes.created_at <= now() - make_interval(secs => sqlc.arg(cooldown_seconds)::int)
    AND (NOT sqlc.arg(keep_live)::bool
        OR email_codes.expires_at <= now()
        OR email_codes.email <> EXCLUDED.email)
RETURNING created_at;

-- name: LockEmailCode :one
SELECT *, expires_at <= now() AS expired FROM email_codes
WHERE user_id = $1 AND purpose = $2
FOR UPDATE;

-- name: CountEmailCodeAttempt :exec
UPDATE email_codes SET attempts = attempts + 1 WHERE user_id = $1 AND purpose = $2;

-- name: DeleteEmailCode :exec
DELETE FROM email_codes WHERE user_id = $1 AND purpose = $2;
