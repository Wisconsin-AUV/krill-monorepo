-- +goose Up
-- What labelers did with model proposals, and changes to boxes on frames
-- someone else had already marked done. Annotations only keep their latest
-- state and a deleted box leaves nothing behind, so rejections and
-- corrections are only known from here.
CREATE TABLE box_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- accepted, edited, and rejected are what happened to a proposal. The
    -- rest are corrections to a frame labeler_id marked.
    kind text NOT NULL CHECK (kind IN ('accepted', 'edited', 'rejected', 'added', 'moved', 'deleted', 'relabeled')),
    frame_id bigint NOT NULL REFERENCES frames (id) ON DELETE CASCADE,
    -- No foreign key, so the event outlives a deleted track.
    track_id bigint NOT NULL,
    label_type_id bigint NOT NULL REFERENCES label_types (id) ON DELETE CASCADE,
    -- Who made the change.
    user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    -- The model behind a proposal, or human.
    source text NOT NULL,
    labeler_id uuid REFERENCES users (id) ON DELETE CASCADE,
    -- When labeler_id marked the frame.
    marked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((kind IN ('accepted', 'edited', 'rejected')) = (labeler_id IS NULL))
);

CREATE INDEX box_events_user_idx ON box_events (user_id, created_at);
CREATE INDEX box_events_labeler_idx ON box_events (labeler_id, marked_at) WHERE labeler_id IS NOT NULL;

-- Accepting proposals leaves them verified with their model as the source,
-- so earlier acceptances can be recovered. Edits turned them into human boxes
-- and rejections deleted them.
INSERT INTO box_events (kind, frame_id, track_id, label_type_id, user_id, source, created_at)
SELECT 'accepted', a.frame_id, a.track_id, t.label_type_id, a.updated_by, a.source, a.updated_at
FROM annotations a
JOIN tracks t ON t.id = a.track_id
WHERE a.status = 'verified' AND a.source <> 'human' AND a.updated_by IS NOT NULL;

-- +goose Down
DROP TABLE box_events;
