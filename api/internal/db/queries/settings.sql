-- name: ListSettings :many
SELECT s.key, s.value, s.updated_at, u.name AS updated_by_name, u.username AS updated_by_username
FROM settings s
LEFT JOIN users u ON u.id = s.updated_by;

-- name: UpsertSetting :exec
INSERT INTO settings (key, value, updated_by) VALUES ($1, $2, $3)
ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = now(), updated_by = excluded.updated_by;

-- name: DeleteSetting :exec
DELETE FROM settings WHERE key = $1;
