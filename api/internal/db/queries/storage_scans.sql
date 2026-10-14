-- name: CreateStorageScan :one
INSERT INTO storage_scans (created_by) VALUES ($1) RETURNING id;

-- name: StartStorageScan :execrows
UPDATE storage_scans SET status = 'running' WHERE id = $1 AND status = 'queued';

-- name: FinishStorageScan :exec
UPDATE storage_scans SET status = 'done', categories = $2, finished_at = now() WHERE id = $1;

-- name: FailStorageScan :exec
UPDATE storage_scans SET status = 'failed', error = $2, finished_at = now() WHERE id = $1;

-- name: FailInterruptedStorageScans :execrows
UPDATE storage_scans SET
    status = CASE WHEN status = 'running' THEN 'failed' ELSE status END,
    error = CASE WHEN status = 'running' THEN 'Scan was interrupted by an API restart.' ELSE error END,
    finished_at = CASE WHEN status = 'running' THEN now() ELSE finished_at END,
    cleanup_status = CASE WHEN cleanup_status = 'running' THEN 'failed' ELSE cleanup_status END,
    cleanup_error = CASE WHEN cleanup_status = 'running' THEN 'Cleanup was interrupted by an API restart.' ELSE cleanup_error END,
    cleanup_finished_at = CASE WHEN cleanup_status = 'running' THEN now() ELSE cleanup_finished_at END
WHERE status = 'running' OR cleanup_status = 'running';

-- name: DeleteOldStorageScans :exec
-- Keeps the last few so the page can show the previous scan while one runs.
DELETE FROM storage_scans
WHERE id NOT IN (SELECT id FROM storage_scans ORDER BY id DESC LIMIT 3)
    AND status IN ('done', 'failed')
    AND cleanup_status NOT IN ('queued', 'running');

-- name: InsertStorageScanVideos :copyfrom
INSERT INTO storage_scan_videos (scan_id, video_id, frame_bytes, frames, embedding_bytes, embeddings, source_bytes, orphan_bytes)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: InsertStorageOrphans :copyfrom
INSERT INTO storage_orphans (scan_id, key, upload_id, kind, size_bytes, modified_at) VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetLatestStorageScanID :one
SELECT id FROM storage_scans ORDER BY id DESC LIMIT 1;

-- name: GetLatestFinishedStorageScanID :one
SELECT id FROM storage_scans WHERE status = 'done' ORDER BY id DESC LIMIT 1;

-- name: GetStorageScan :one
SELECT s.*, cu.name AS created_by_name, ku.name AS cleanup_by_name
FROM storage_scans s
LEFT JOIN users cu ON cu.id = s.created_by
LEFT JOIN users ku ON ku.id = s.cleanup_by
WHERE s.id = $1;

-- name: ListStorageScanVideos :many
SELECT sv.*, v.name AS video_name
FROM storage_scan_videos sv
LEFT JOIN videos v ON v.id = sv.video_id
WHERE sv.scan_id = $1
ORDER BY sv.frame_bytes + sv.embedding_bytes + sv.source_bytes + sv.orphan_bytes DESC, sv.video_id;

-- name: ListLargestStorageOrphans :many
SELECT key, kind, size_bytes, modified_at FROM storage_orphans
WHERE scan_id = $1
ORDER BY size_bytes DESC, key
LIMIT $2;

-- name: ListStorageOrphans :many
SELECT key, upload_id, kind, size_bytes, modified_at FROM storage_orphans
WHERE scan_id = $1 AND (key, upload_id) > (sqlc.arg('after_key')::text, sqlc.arg('after_upload_id')::text)
ORDER BY key, upload_id
LIMIT $2;

-- name: RequestStorageCleanup :execrows
-- Only the latest scan, since an older one's orphans were checked against an
-- older state of the bucket.
UPDATE storage_scans SET cleanup_status = 'queued', cleanup_by = $2, cleanup_error = '',
    cleanup_finished_at = NULL, deleted_objects = 0, deleted_bytes = 0, skipped_objects = 0
WHERE storage_scans.id = $1
    AND status = 'done'
    AND cleanup_status IN ('', 'failed')
    AND storage_scans.id = (SELECT max(latest.id) FROM storage_scans latest);

-- name: StartStorageCleanup :execrows
UPDATE storage_scans SET cleanup_status = 'running' WHERE id = $1 AND cleanup_status = 'queued';

-- name: FinishStorageCleanup :exec
UPDATE storage_scans SET cleanup_status = 'done', cleanup_finished_at = now(),
    deleted_objects = $2, deleted_bytes = $3, skipped_objects = $4
WHERE id = $1;

-- name: FailStorageCleanup :exec
UPDATE storage_scans SET cleanup_status = 'failed', cleanup_finished_at = now(), cleanup_error = $2,
    deleted_objects = $3, deleted_bytes = $4, skipped_objects = $5
WHERE id = $1;

-- name: ScanVideos :many
SELECT id, status, frame_count FROM videos;

-- name: ScanVideoFrames :many
SELECT f.idx, (c.embedded_at IS NOT NULL)::bool AS embedded
FROM frames f
JOIN clips c ON c.id = f.clip_id
WHERE f.video_id = $1;

-- name: ScanDatasets :many
SELECT id, status FROM datasets;

-- name: ScanLabelExampleKeys :many
SELECT object_key FROM label_examples;

-- name: ScanBackups :many
SELECT object_key, status FROM backups;
