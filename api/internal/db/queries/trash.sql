-- name: ListTrashedVideos :many
-- Counts cover clips trashed on their own too, since they go with the video.
SELECT
    v.id, v.name, v.status, v.frame_count, v.width, v.height, v.deleted_at,
    u.id AS deleted_by_id, u.name AS deleted_by_name, u.username AS deleted_by_username,
    u.role AS deleted_by_role, u.created_at AS deleted_by_created_at,
    (SELECT count(*) FROM clips c WHERE c.video_id = v.id)::int AS clip_count,
    (SELECT count(*) FROM frames f WHERE f.video_id = v.id AND f.status <> 'unlabeled')::int AS labeled_frame_count,
    (SELECT count(*) FROM tracks t JOIN clips c ON c.id = t.clip_id WHERE c.video_id = v.id)::int AS track_count,
    (SELECT count(*) FROM annotations a JOIN frames f ON f.id = a.frame_id WHERE f.video_id = v.id)::int AS box_count,
    coalesce((
        SELECT c.start_frame + c.frame_count / 2 FROM clips c
        WHERE c.video_id = v.id
        ORDER BY c.start_frame
        OFFSET (SELECT count(*) / 2 FROM clips m WHERE m.video_id = v.id)
        LIMIT 1
    ), -1)::int AS thumbnail_frame
FROM videos v
LEFT JOIN users u ON u.id = v.deleted_by
WHERE v.deleted_at IS NOT NULL
ORDER BY v.deleted_at DESC, v.id DESC;

-- name: ListTrashedClips :many
-- Clips of a trashed video are listed with the video instead.
SELECT
    c.id, c.video_id, v.name AS video_name, c.idx, c.start_frame, c.frame_count, c.deleted_at,
    u.id AS deleted_by_id, u.name AS deleted_by_name, u.username AS deleted_by_username,
    u.role AS deleted_by_role, u.created_at AS deleted_by_created_at,
    (SELECT count(*) FROM frames f WHERE f.clip_id = c.id AND f.status <> 'unlabeled')::int AS labeled_frame_count,
    (SELECT count(*) FROM tracks t WHERE t.clip_id = c.id)::int AS track_count,
    (SELECT count(*) FROM annotations a JOIN tracks t ON t.id = a.track_id WHERE t.clip_id = c.id)::int AS box_count
FROM clips c
JOIN videos v ON v.id = c.video_id
LEFT JOIN users u ON u.id = c.deleted_by
WHERE c.deleted_at IS NOT NULL AND v.deleted_at IS NULL
ORDER BY c.deleted_at DESC, c.id DESC;

-- name: ListTrashIDs :many
-- Everything trashed before @before. A clip whose video is also trashed goes
-- with the video.
SELECT 'video'::text AS kind, tv.id FROM videos tv WHERE tv.deleted_at < @before
UNION ALL
SELECT 'clip'::text, c.id
FROM clips c
JOIN videos v ON v.id = c.video_id
WHERE c.deleted_at < @before AND v.deleted_at IS NULL;

-- name: LockTrashedVideo :one
SELECT id, name, status FROM videos WHERE id = $1 AND deleted_at IS NOT NULL FOR UPDATE;

-- name: ListLiveClipIDs :many
SELECT id FROM clips WHERE video_id = $1 AND deleted_at IS NULL;
