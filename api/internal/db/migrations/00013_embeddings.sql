-- +goose Up
ALTER TABLE clips ADD COLUMN embedded_at timestamptz;
-- Embeddings only help a worker running the model that computed them.
ALTER TABLE clips ADD COLUMN embed_model text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE clips DROP COLUMN embed_model;
ALTER TABLE clips DROP COLUMN embedded_at;
