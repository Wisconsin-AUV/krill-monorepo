-- Names for the targets of background jobs in the admin panel.

-- name: ListJobVideos :many
SELECT id, name FROM videos WHERE id = ANY(@ids::bigint[]);

-- name: ListJobClips :many
SELECT c.id, v.name AS video_name, c.idx
FROM clips c JOIN videos v ON v.id = c.video_id
WHERE c.id = ANY(@ids::bigint[]);

-- name: ListJobFrames :many
SELECT f.id, v.name AS video_name, f.idx
FROM frames f JOIN videos v ON v.id = f.video_id
WHERE f.id = ANY(@ids::bigint[]);

-- name: ListJobDatasets :many
SELECT id, name FROM datasets WHERE id = ANY(@ids::bigint[]);

-- name: ListJobStarters :many
SELECT * FROM users WHERE id = ANY(@ids::uuid[]);

-- RequeueIngest readies a failed ingest for its job to run again.
-- name: RequeueIngest :execrows
UPDATE videos SET status = 'queued', error = '', ingest_progress = 0
WHERE id = $1 AND status = 'failed';

-- RequeueDataset readies a failed export for its job to run again.
-- name: RequeueDataset :execrows
UPDATE datasets SET status = 'queued', error = '', progress = 0, finished_at = NULL
WHERE id = $1 AND status = 'failed';
