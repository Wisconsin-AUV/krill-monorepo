-- +goose Up
-- A label type's keypoints are [{"name": "top-left", "mirror": 1}], its quads
-- [{"corners": [0, 1, 2, 3], "reference": true, "center": 4}], and its edges
-- [{"from": 0, "to": 1}], with indexes into its keypoints.
ALTER TABLE label_types
    ADD COLUMN keypoints jsonb NOT NULL DEFAULT '[]',
    ADD COLUMN quads jsonb NOT NULL DEFAULT '[]',
    ADD COLUMN keypoint_edges jsonb NOT NULL DEFAULT '[]';

-- A box's keypoints are x, y, visibility triples in the order of its type's
-- keypoints, with YOLO-pose's visibility values. NULL is a box with none
-- labeled, and a shorter array than the type has keypoints is padded with
-- unlabeled ones. Gold frames and attempts copy them into each box's
-- "keypoints".
ALTER TABLE annotations
    ADD COLUMN keypoints double precision[] CHECK (cardinality(keypoints) % 3 = 0);

ALTER TABLE model_trial_boxes
    ADD COLUMN keypoints double precision[] CHECK (cardinality(keypoints) % 3 = 0);

ALTER TABLE training_runs ADD COLUMN keypoints int NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE training_runs DROP COLUMN keypoints;
ALTER TABLE model_trial_boxes DROP COLUMN keypoints;
ALTER TABLE annotations DROP COLUMN keypoints;
ALTER TABLE label_types DROP COLUMN keypoint_edges, DROP COLUMN quads, DROP COLUMN keypoints;
