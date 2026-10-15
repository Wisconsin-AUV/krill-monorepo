-- +goose Up
-- class_edited is set once a labeler changes a track's label type or
-- attributes. Pre-label runs leave such a track's proposals alone, and
-- deciding them counts as an edit rather than an acceptance of the model's
-- class. proposed_label_type_id is the class a pre-labeler gave the track, so
-- only relabeling away from it rejects the model's class.
ALTER TABLE tracks
    ADD COLUMN class_edited bool NOT NULL DEFAULT false,
    ADD COLUMN proposed_label_type_id bigint REFERENCES label_types (id) ON DELETE SET NULL;

CREATE INDEX tracks_proposed_label_type_id_idx ON tracks (proposed_label_type_id)
    WHERE proposed_label_type_id IS NOT NULL;

UPDATE tracks t SET proposed_label_type_id = t.label_type_id
WHERE EXISTS (SELECT 1 FROM annotations a WHERE a.track_id = t.id AND a.source IN ('yolo', 'cv'));

-- +goose Down
ALTER TABLE tracks DROP COLUMN proposed_label_type_id, DROP COLUMN class_edited;
