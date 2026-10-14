-- +goose Up
-- Settings changed in the admin panel. The environment still wins, so a row
-- only applies while its variable is unset.
CREATE TABLE settings (
    key text PRIMARY KEY,
    value text NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    updated_by uuid REFERENCES users (id) ON DELETE SET NULL
);

-- +goose Down
DROP TABLE settings;
