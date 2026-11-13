-- +goose Up
-- Uploaded runs hold weights trained outside Krill, so they skip straight to
-- succeeded with their classes and none of the training columns.
ALTER TABLE training_runs DROP CONSTRAINT training_runs_trigger_check;
ALTER TABLE training_runs ADD CONSTRAINT training_runs_trigger_check
    CHECK (trigger IN ('manual', 'schedule', 'upload'));
ALTER TABLE training_runs ADD COLUMN filename text NOT NULL DEFAULT '';

-- +goose Down
DELETE FROM training_runs WHERE trigger = 'upload';
ALTER TABLE training_runs DROP COLUMN filename;
ALTER TABLE training_runs DROP CONSTRAINT training_runs_trigger_check;
ALTER TABLE training_runs ADD CONSTRAINT training_runs_trigger_check
    CHECK (trigger IN ('manual', 'schedule'));
