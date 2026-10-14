-- name: CreateTrainingRun :one
INSERT INTO training_runs (trigger, created_by, config, verified_boxes)
VALUES (@trigger, sqlc.narg('created_by')::uuid, @config, @verified_boxes)
RETURNING *;

-- name: GetTrainingRun :one
SELECT sqlc.embed(r), coalesce(u.name, '')::text AS created_by_name
FROM training_runs r
LEFT JOIN users u ON u.id = r.created_by
WHERE r.id = $1;

-- name: ListTrainingRuns :many
SELECT sqlc.embed(r), coalesce(u.name, '')::text AS created_by_name
FROM training_runs r
LEFT JOIN users u ON u.id = r.created_by
ORDER BY r.id DESC
LIMIT $1;

-- name: GetPinnedTrainingRun :one
SELECT * FROM training_runs WHERE pinned;

-- name: GetLatestTrainingRun :one
SELECT * FROM training_runs ORDER BY id DESC LIMIT 1;

-- name: CountActiveTrainingRuns :one
SELECT count(*)::int FROM training_runs
WHERE status IN ('queued', 'preparing', 'training', 'evaluating');

-- name: StartTrainingRun :execrows
UPDATE training_runs SET status = 'preparing', started_at = now()
WHERE id = $1 AND status = 'queued';

-- name: SetTrainingDataset :exec
UPDATE training_runs SET dataset = @dataset, classes = @classes, epochs = @epochs
WHERE id = @id;

-- name: SetTrainingProgress :exec
-- metrics is appended to the history when a report finishes an epoch.
UPDATE training_runs SET
    status = @status,
    epoch = CASE WHEN sqlc.narg('metrics')::jsonb IS NULL THEN epoch ELSE @epoch END,
    history = CASE
        WHEN sqlc.narg('metrics')::jsonb IS NULL THEN history
        ELSE history || jsonb_build_array(sqlc.narg('metrics')::jsonb)
    END,
    log = CASE WHEN @log::text = '' THEN log ELSE @log END,
    device = CASE WHEN @device::text = '' THEN device ELSE @device END
WHERE id = @id AND status IN ('preparing', 'training', 'evaluating');

-- name: FinishTrainingRun :execrows
UPDATE training_runs SET
    status = 'succeeded',
    val_metrics = @val_metrics,
    test_metrics = sqlc.narg('test_metrics'),
    baseline_run_id = sqlc.narg('baseline_run_id'),
    baseline_val_metrics = sqlc.narg('baseline_val_metrics'),
    baseline_test_metrics = sqlc.narg('baseline_test_metrics'),
    weights_bytes = @weights_bytes,
    log = CASE WHEN @log::text = '' THEN log ELSE @log END,
    finished_at = now()
WHERE id = @id AND status IN ('preparing', 'training', 'evaluating');

-- name: FailTrainingRun :exec
UPDATE training_runs SET status = 'failed', error = @error, finished_at = now()
WHERE id = @id AND status IN ('queued', 'preparing', 'training', 'evaluating');

-- name: CancelTrainingRun :execrows
UPDATE training_runs SET status = 'cancelled', finished_at = now()
WHERE id = $1 AND status IN ('queued', 'preparing', 'training', 'evaluating');

-- name: FailInterruptedTrainingRuns :execrows
UPDATE training_runs SET status = 'failed', error = 'Training was interrupted by an API restart.', finished_at = now()
WHERE status IN ('preparing', 'training', 'evaluating');

-- name: UnpinTrainingRuns :exec
UPDATE training_runs SET pinned = false WHERE pinned;

-- name: PinTrainingRun :execrows
UPDATE training_runs SET pinned = true WHERE id = $1 AND status = 'succeeded';

-- name: CountVerifiedBoxes :one
-- Held-out boxes are left out because no run trains on them.
SELECT count(*)::int
FROM annotations a
JOIN frames f ON f.id = a.frame_id
JOIN live_clips c ON c.id = f.clip_id
JOIN videos v ON v.id = f.video_id
WHERE a.status = 'verified' AND v.split <> 'test';

-- name: CountVerifiedBoxesSince :one
SELECT count(*)::int
FROM annotations a
JOIN frames f ON f.id = a.frame_id
JOIN live_clips c ON c.id = f.clip_id
JOIN videos v ON v.id = f.video_id
WHERE a.status = 'verified' AND v.split <> 'test' AND a.updated_at > @since;

-- name: CreatePrelabelRun :one
INSERT INTO prelabel_runs (source, training_run_id, created_by)
VALUES (@source, sqlc.narg('training_run_id'), sqlc.narg('created_by')::uuid)
RETURNING *;

-- name: GetPrelabelRun :one
SELECT * FROM prelabel_runs WHERE id = $1;

-- name: ListPrelabelRuns :many
SELECT sqlc.embed(r), coalesce(u.name, '')::text AS created_by_name
FROM prelabel_runs r
LEFT JOIN users u ON u.id = r.created_by
ORDER BY r.id DESC
LIMIT $1;

-- name: LockPrelabelRun :one
SELECT status FROM prelabel_runs WHERE id = $1 FOR SHARE;

-- name: StartPrelabelRun :execrows
UPDATE prelabel_runs SET status = 'running', clips_total = @clips_total
WHERE id = @id AND status = 'queued';

-- name: AdvancePrelabelRun :exec
UPDATE prelabel_runs SET
    clips_done = clips_done + 1,
    frames = frames + @frames,
    proposals = proposals + @proposals
WHERE id = @id AND status = 'running';

-- name: FinishPrelabelRun :exec
UPDATE prelabel_runs SET status = 'succeeded', finished_at = now()
WHERE id = $1 AND status = 'running';

-- name: FailPrelabelRun :exec
UPDATE prelabel_runs SET status = 'failed', error = @error, finished_at = now()
WHERE id = @id AND status IN ('queued', 'running');

-- name: CancelPrelabelRun :execrows
UPDATE prelabel_runs SET status = 'cancelled', finished_at = now()
WHERE id = $1 AND status IN ('queued', 'running');

-- name: CancelActivePrelabelRuns :many
UPDATE prelabel_runs SET status = 'cancelled', finished_at = now()
WHERE source = $1 AND status IN ('queued', 'running')
RETURNING id;

-- name: FailInterruptedPrelabelRuns :execrows
UPDATE prelabel_runs SET status = 'failed', error = 'Pre-labeling was interrupted by an API restart.', finished_at = now()
WHERE status = 'running';

-- name: CountOpenPrelabels :one
SELECT
    count(*) FILTER (WHERE source = 'yolo')::int AS yolo,
    count(*) FILTER (WHERE source = 'cv')::int AS cv
FROM annotations
WHERE status = 'proposed' AND source IN ('yolo', 'cv');

-- name: ListPrelabelClips :many
-- Held-out videos are never pre-labeled, so their test metrics stay honest.
SELECT c.id, c.video_id
FROM clips c
JOIN videos v ON v.id = c.video_id
WHERE v.status = 'ready'
    AND v.split <> 'test'
    AND c.deleted_at IS NULL AND v.deleted_at IS NULL
    AND EXISTS (SELECT 1 FROM frames f WHERE f.clip_id = c.id AND f.status = 'unlabeled')
ORDER BY v.id, c.idx;

-- name: ListUnlabeledClipFrames :many
SELECT id, idx FROM frames WHERE clip_id = $1 AND status = 'unlabeled' ORDER BY idx;

-- name: LockUnlabeledFrames :many
-- Locking keeps a frame from being marked done while its proposals are
-- replaced.
SELECT id FROM frames
WHERE id = ANY(@ids::bigint[]) AND status = 'unlabeled'
ORDER BY id
FOR UPDATE;

-- name: DeleteOpenPrelabels :many
DELETE FROM annotations
WHERE frame_id = ANY(@frame_ids::bigint[]) AND source = @source AND status = 'proposed'
RETURNING track_id;

-- name: DeleteEmptyTracks :exec
DELETE FROM tracks t
WHERE t.id = ANY(@ids::bigint[])
    AND NOT EXISTS (SELECT 1 FROM annotations a WHERE a.track_id = t.id);

-- name: ListBoxesOnFrames :many
SELECT frame_id, x, y, width, height FROM annotations
WHERE frame_id = ANY(@frame_ids::bigint[]) AND status <> 'rejected';

-- name: InsertPrelabels :execrows
INSERT INTO annotations (track_id, frame_id, x, y, width, height, source, status, model_version, confidence)
SELECT
    unnest(@track_ids::bigint[]),
    unnest(@frame_ids::bigint[]),
    unnest(@xs::float8[]),
    unnest(@ys::float8[]),
    unnest(@widths::float8[]),
    unnest(@heights::float8[]),
    @source,
    'proposed',
    unnest(@model_versions::text[]),
    unnest(@confidences::real[])
ON CONFLICT (track_id, frame_id) DO NOTHING;

-- name: ListPrelabelers :many
SELECT sqlc.embed(p), coalesce(u.name, '')::text AS updated_by_name
FROM prelabelers p
LEFT JOIN users u ON u.id = p.updated_by
ORDER BY p.name;

-- name: GetPrelabeler :one
SELECT * FROM prelabelers WHERE name = $1;

-- name: UpsertPrelabelerSpec :exec
INSERT INTO prelabelers (name, spec) VALUES (@name, @spec)
ON CONFLICT (name) DO UPDATE SET spec = excluded.spec
WHERE prelabelers.spec IS DISTINCT FROM excluded.spec;

-- name: UpdatePrelabeler :exec
UPDATE prelabelers SET
    enabled = @enabled,
    params = @params,
    outputs = @outputs,
    updated_at = now(),
    updated_by = sqlc.narg('updated_by')::uuid
WHERE name = @name;

-- name: ListEnabledPrelabelers :many
SELECT * FROM prelabelers WHERE enabled ORDER BY name;
