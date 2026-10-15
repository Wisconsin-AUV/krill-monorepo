-- name: CreateBackup :one
INSERT INTO backups (scheduled, object_key, created_by) VALUES ($1, $2, $3) RETURNING *;

-- name: StartBackup :execrows
UPDATE backups SET status = 'running' WHERE id = $1 AND status = 'queued';

-- name: FinishBackup :exec
UPDATE backups SET status = 'done', size_bytes = $2, finished_at = now() WHERE id = $1;

-- name: FailBackup :exec
UPDATE backups SET status = 'failed', error = $2, finished_at = now() WHERE id = $1;

-- name: FailInterruptedBackups :execrows
UPDATE backups SET status = 'failed', error = 'Backup was interrupted by an API restart.', finished_at = now()
WHERE status = 'running';

-- name: GetBackup :one
SELECT * FROM backups WHERE id = $1;

-- name: ListBackups :many
SELECT b.*, u.name AS created_by_name
FROM backups b
LEFT JOIN users u ON u.id = b.created_by
ORDER BY b.created_at DESC, b.id DESC;

-- name: DeleteBackup :one
DELETE FROM backups WHERE id = $1 AND status IN ('done', 'failed') RETURNING *;

-- name: ListExpiredScheduledBackups :many
SELECT * FROM backups
WHERE scheduled AND status = 'done'
ORDER BY created_at DESC, id DESC
OFFSET $1;

-- name: LastScheduledBackupAt :one
SELECT max(created_at)::timestamptz FROM backups WHERE scheduled;

-- name: GetLastBackup :one
SELECT * FROM backups WHERE status IN ('done', 'failed') ORDER BY created_at DESC, id DESC LIMIT 1;

-- name: GetLastSuccessfulBackup :one
SELECT * FROM backups WHERE status = 'done' ORDER BY created_at DESC, id DESC LIMIT 1;

-- name: ServerVersion :one
SELECT current_setting('server_version')::text;
