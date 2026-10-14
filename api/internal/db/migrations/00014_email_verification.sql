-- +goose Up
ALTER TABLE users ADD COLUMN email_verified_at timestamptz;
-- A changed email waits here until it is verified, so the old one keeps
-- working in the meantime.
ALTER TABLE users ADD COLUMN pending_email text;
-- Accounts made before verification existed are trusted.
UPDATE users SET email_verified_at = created_at;

CREATE TABLE email_codes (
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    purpose text NOT NULL CHECK (purpose IN ('verify_email', 'reset_password')),
    -- The address the code was sent to. Verifying moves the account to it.
    email text NOT NULL,
    -- SHA-256, so reading the table does not reveal live codes.
    code_hash bytea NOT NULL,
    attempts int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (user_id, purpose)
);

-- +goose Down
DROP TABLE email_codes;
ALTER TABLE users DROP COLUMN pending_email;
ALTER TABLE users DROP COLUMN email_verified_at;
