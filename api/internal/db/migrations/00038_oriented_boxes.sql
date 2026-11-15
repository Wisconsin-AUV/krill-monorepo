-- +goose Up
-- Boxes of oriented label types are rotated rectangles. corners holds their
-- four corners clockwise as x1, y1, ..., x4, y4, and x, y, width, and height
-- stay the corners' bounds clipped to the frame, so everything that reads
-- axis-aligned boxes keeps working. NULL is an unrotated box.
ALTER TABLE label_types
    ADD COLUMN oriented boolean NOT NULL DEFAULT false,
    ADD COLUMN reference_key text;

ALTER TABLE annotations
    ADD COLUMN corners double precision[] CHECK (cardinality(corners) = 8);

ALTER TABLE model_trial_boxes
    ADD COLUMN corners double precision[] CHECK (cardinality(corners) = 8);

ALTER TABLE training_runs ADD COLUMN oriented boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE training_runs DROP COLUMN oriented;
ALTER TABLE model_trial_boxes DROP COLUMN corners;
ALTER TABLE annotations DROP COLUMN corners;
ALTER TABLE label_types DROP COLUMN reference_key, DROP COLUMN oriented;
