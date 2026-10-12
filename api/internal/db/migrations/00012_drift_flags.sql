-- +goose Up
-- NULL on boxes the tracker never proposed. Accepting or editing a flagged
-- proposal moves it to reviewed rather than ok, so the per-type flag rate
-- still counts it.
ALTER TABLE annotations
    ADD COLUMN drift text CHECK (drift IN ('ok', 'flagged', 'reviewed'));

CREATE INDEX annotations_drift_idx ON annotations (track_id, drift) WHERE drift IS NOT NULL;

-- +goose Down
DROP INDEX annotations_drift_idx;
ALTER TABLE annotations DROP COLUMN drift;
