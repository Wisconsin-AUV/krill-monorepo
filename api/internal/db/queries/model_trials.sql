-- name: QueueModelTrial :one
-- Returns no row when the trial exists and is not failed or cancelled.
INSERT INTO model_trials (training_run_id, clip_id, created_by)
VALUES (@training_run_id, @clip_id, sqlc.narg('created_by')::uuid)
ON CONFLICT (training_run_id, clip_id) DO UPDATE SET
    status = 'queued',
    frames_total = 0,
    frames_done = 0,
    error = '',
    created_by = excluded.created_by,
    created_at = now(),
    started_at = NULL,
    finished_at = NULL
WHERE model_trials.status IN ('failed', 'cancelled')
RETURNING id;

-- name: GetModelTrialID :one
SELECT id FROM model_trials WHERE training_run_id = @training_run_id AND clip_id = @clip_id;

-- name: GetModelTrial :one
SELECT
    sqlc.embed(t),
    c.video_id,
    v.name AS video_name,
    c.idx AS clip_idx,
    coalesce(u.name, '')::text AS created_by_name
FROM model_trials t
JOIN clips c ON c.id = t.clip_id
JOIN videos v ON v.id = c.video_id
LEFT JOIN users u ON u.id = t.created_by
WHERE t.id = $1;

-- name: ListModelTrials :many
SELECT
    sqlc.embed(t),
    c.video_id,
    v.name AS video_name,
    c.idx AS clip_idx,
    coalesce(u.name, '')::text AS created_by_name
FROM model_trials t
JOIN clips c ON c.id = t.clip_id
JOIN videos v ON v.id = c.video_id
LEFT JOIN users u ON u.id = t.created_by
WHERE (sqlc.narg('training_run_id')::bigint IS NULL OR t.training_run_id = sqlc.narg('training_run_id'))
    AND (sqlc.narg('clip_id')::bigint IS NULL OR t.clip_id = sqlc.narg('clip_id'))
    AND c.deleted_at IS NULL AND v.deleted_at IS NULL
ORDER BY t.id DESC;

-- name: RunModelTrial :execrows
UPDATE model_trials SET
    status = 'running',
    frames_total = @frames_total,
    confidence_floor = @confidence_floor,
    started_at = now()
WHERE id = @id AND status = 'queued';

-- name: AdvanceModelTrial :execrows
UPDATE model_trials SET frames_done = greatest(frames_done, @frames_done)
WHERE id = @id AND status = 'running';

-- name: FinishModelTrial :execrows
UPDATE model_trials SET status = 'succeeded', frames_done = frames_total, finished_at = now()
WHERE id = $1 AND status = 'running';

-- name: FailModelTrial :exec
UPDATE model_trials SET status = 'failed', error = @error, finished_at = now()
WHERE id = @id AND status IN ('queued', 'running');

-- name: CancelModelTrial :execrows
UPDATE model_trials SET status = 'cancelled', finished_at = now()
WHERE id = $1 AND status IN ('queued', 'running');

-- name: FailInterruptedModelTrials :execrows
UPDATE model_trials SET status = 'failed', error = 'The trial was interrupted by an API restart.', finished_at = now()
WHERE status = 'running';

-- name: InsertModelTrialBoxes :execrows
-- Boxes a worker reports after the trial stopped are dropped.
INSERT INTO model_trial_boxes (trial_id, frame_id, class, x, y, width, height, confidence)
SELECT
    @trial_id,
    unnest(@frame_ids::bigint[]),
    unnest(@classes::int[]),
    unnest(@xs::float8[]),
    unnest(@ys::float8[]),
    unnest(@widths::float8[]),
    unnest(@heights::float8[]),
    unnest(@confidences::real[])
WHERE EXISTS (SELECT 1 FROM model_trials t WHERE t.id = @trial_id AND t.status = 'running');

-- name: DeleteModelTrialBoxes :exec
DELETE FROM model_trial_boxes WHERE trial_id = $1;

-- name: ListModelTrialBoxes :many
SELECT b.id, b.frame_id, f.idx AS frame_idx, b.class, b.x, b.y, b.width, b.height, b.confidence, b.track
FROM model_trial_boxes b
JOIN frames f ON f.id = b.frame_id
WHERE b.trial_id = $1
ORDER BY f.idx, b.confidence DESC, b.id;

-- name: SetModelTrialTracks :exec
UPDATE model_trial_boxes b SET track = u.track
FROM (SELECT unnest(@ids::bigint[]) AS id, unnest(@tracks::int[]) AS track) u
WHERE b.id = u.id AND b.trial_id = @trial_id;

-- name: ListDoneTrialBoxes :many
-- The boxes a summary scores: those at or above the threshold on frames
-- someone marked done.
SELECT b.frame_id, b.class, b.x, b.y, b.width, b.height
FROM model_trial_boxes b
JOIN frames f ON f.id = b.frame_id
WHERE b.trial_id = @trial_id AND f.status <> 'unlabeled' AND b.confidence >= @min_confidence;

-- name: ListDoneClipFrames :many
SELECT id FROM frames WHERE clip_id = $1 AND status <> 'unlabeled' ORDER BY idx;

-- name: ListDoneVerifiedBoxes :many
SELECT a.frame_id, t.label_type_id, t.attributes, a.x, a.y, a.width, a.height
FROM annotations a
JOIN tracks t ON t.id = a.track_id
JOIN frames f ON f.id = a.frame_id
WHERE f.clip_id = $1 AND f.status <> 'unlabeled' AND a.status = 'verified';
