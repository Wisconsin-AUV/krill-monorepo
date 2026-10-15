-- name: CreateVideo :one
INSERT INTO videos (name, filename, extract_fps)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetVideo :one
SELECT * FROM videos WHERE id = $1 AND deleted_at IS NULL;

-- name: GetVideoInTrash :one
-- Also finds trashed videos, for ingest and the trash itself.
SELECT * FROM videos WHERE id = $1;

-- name: ListVideos :many
SELECT
    sqlc.embed(v),
    (SELECT count(*) FROM clips c WHERE c.video_id = v.id AND c.deleted_at IS NULL)::int AS clip_count,
    (
        SELECT count(*) FROM frames f JOIN clips c ON c.id = f.clip_id
        WHERE f.video_id = v.id AND c.deleted_at IS NULL AND f.status <> 'unlabeled'
    )::int AS labeled_frame_count,
    (
        SELECT count(*) FROM annotations a JOIN tracks t ON t.id = a.track_id JOIN clips c ON c.id = t.clip_id
        WHERE c.video_id = v.id AND c.deleted_at IS NULL
    )::int AS box_count,
    coalesce((
        SELECT c.start_frame + c.frame_count / 2 FROM clips c
        WHERE c.video_id = v.id AND c.deleted_at IS NULL
        ORDER BY c.idx
        OFFSET (SELECT count(*) / 2 FROM clips m WHERE m.video_id = v.id AND m.deleted_at IS NULL)
        LIMIT 1
    ), -1)::int AS thumbnail_frame
FROM videos v
WHERE v.deleted_at IS NULL
ORDER BY v.created_at DESC, v.id DESC;

-- name: CountClips :one
SELECT count(*)::int FROM clips WHERE video_id = $1 AND deleted_at IS NULL;

-- name: UpdateVideo :one
UPDATE videos SET
    name = coalesce(sqlc.narg('name'), name),
    notes = coalesce(sqlc.narg('notes'), notes),
    split = coalesce(sqlc.narg('split'), split)
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING *;

-- name: TrashVideo :one
UPDATE videos SET deleted_at = now(), deleted_by = sqlc.narg(user_id)::uuid
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: RestoreVideo :one
UPDATE videos SET deleted_at = NULL, deleted_by = NULL
WHERE id = $1 AND deleted_at IS NOT NULL
RETURNING *;

-- name: DeleteVideo :execrows
DELETE FROM videos WHERE id = $1;

-- name: QueueIngest :one
UPDATE videos SET status = 'queued', error = '', ingest_progress = 0
WHERE id = $1 AND status IN ('uploading', 'failed') AND deleted_at IS NULL
RETURNING *;

-- name: QueueReextract :one
UPDATE videos SET status = 'queued', error = '', ingest_progress = 0, extract_fps = @extract_fps
WHERE id = @id AND status IN ('ready', 'failed') AND deleted_at IS NULL
RETURNING *;

-- name: StartProcessing :execrows
UPDATE videos SET status = 'processing', error = '', ingest_progress = 0
WHERE id = $1;

-- name: SetVideoProbe :exec
UPDATE videos SET width = $2, height = $3, fps = $4, duration_ms = $5
WHERE id = $1;

-- name: SetIngestProgress :exec
UPDATE videos SET ingest_progress = $2 WHERE id = $1;

-- name: FinishIngest :exec
UPDATE videos SET status = 'ready', ingest_progress = 1, frame_count = $2
WHERE id = $1;

-- name: FailInterruptedIngests :execrows
UPDATE videos SET status = 'failed', error = 'Ingest was interrupted by an API restart.'
WHERE status = 'processing';

-- name: FailIngest :exec
UPDATE videos SET status = 'failed', error = $2 WHERE id = $1;

-- name: DeleteClips :exec
DELETE FROM clips WHERE video_id = $1;

-- name: CreateClip :one
INSERT INTO clips (video_id, idx, start_frame, frame_count)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: ListClips :many
SELECT
    sqlc.embed(c),
    (SELECT count(*) FROM frames f WHERE f.clip_id = c.id AND f.status <> 'unlabeled')::int AS labeled_frame_count,
    (SELECT count(*) FROM annotations a JOIN tracks t ON t.id = a.track_id WHERE t.clip_id = c.id)::int AS box_count
FROM clips c
WHERE c.video_id = $1 AND c.deleted_at IS NULL
ORDER BY c.idx;

-- name: GetVideoLabelStats :one
SELECT
    (
        SELECT count(*) FROM frames f JOIN clips c ON c.id = f.clip_id
        WHERE f.video_id = $1 AND c.deleted_at IS NULL AND f.status <> 'unlabeled'
    )::int AS labeled_frame_count,
    (
        SELECT count(*) FROM annotations a JOIN tracks t ON t.id = a.track_id JOIN clips c ON c.id = t.clip_id
        WHERE c.video_id = $1 AND c.deleted_at IS NULL
    )::int AS box_count,
    coalesce((
        SELECT c.start_frame + c.frame_count / 2 FROM clips c
        WHERE c.video_id = $1 AND c.deleted_at IS NULL
        ORDER BY c.idx
        OFFSET (SELECT count(*) / 2 FROM clips m WHERE m.video_id = $1 AND m.deleted_at IS NULL)
        LIMIT 1
    ), -1)::int AS thumbnail_frame;

-- name: InsertFrames :copyfrom
INSERT INTO frames (video_id, clip_id, idx, phash) VALUES ($1, $2, $3, $4);
