-- +goose Up
-- Where labelers rejected YOLO and CV proposals. Rejecting deletes the box,
-- so without this the next pre-label run proposes it again. Kept apart from
-- annotations so the rejection outlives the track it was in.
CREATE TABLE rejected_prelabels (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    frame_id bigint NOT NULL REFERENCES frames (id) ON DELETE CASCADE,
    label_type_id bigint NOT NULL REFERENCES label_types (id) ON DELETE CASCADE,
    x double precision NOT NULL,
    y double precision NOT NULL,
    width double precision NOT NULL,
    height double precision NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX rejected_prelabels_frame_id_idx ON rejected_prelabels (frame_id);
CREATE INDEX rejected_prelabels_label_type_id_idx ON rejected_prelabels (label_type_id);

-- +goose Down
DROP TABLE rejected_prelabels;
