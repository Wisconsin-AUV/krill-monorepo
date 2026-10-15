-- name: ListContentVideos :many
SELECT
    v.id, v.name, v.status, v.error, v.ingest_progress, v.extract_fps, v.fps, v.frame_count, v.created_at,
    count(c.id)::int AS clip_count,
    count(c.id) FILTER (WHERE c.embedded_at IS NOT NULL)::int AS embedded_clip_count,
    count(c.id) FILTER (WHERE c.embedded_at IS NOT NULL AND c.embed_model = @model::text)::int AS current_clip_count,
    (
        SELECT count(*) FROM frames f JOIN clips lc ON lc.id = f.clip_id
        WHERE f.video_id = v.id AND lc.deleted_at IS NULL AND f.status <> 'unlabeled'
    )::int AS labeled_frame_count,
    (
        SELECT count(*) FROM annotations a JOIN tracks t ON t.id = a.track_id JOIN clips lc ON lc.id = t.clip_id
        WHERE lc.video_id = v.id AND lc.deleted_at IS NULL
    )::int AS box_count
FROM videos v
LEFT JOIN clips c ON c.video_id = v.id AND c.deleted_at IS NULL
WHERE v.deleted_at IS NULL
GROUP BY v.id
ORDER BY v.created_at DESC, v.id DESC;

-- name: ListReextractStats :many
-- What re-extracting each video throws away. Ingest replaces every clip,
-- including the ones in the trash.
SELECT
    v.id, v.name, v.status, v.extract_fps,
    (SELECT count(*) FROM frames f WHERE f.video_id = v.id AND f.status <> 'unlabeled')::int AS labeled_frame_count,
    (SELECT count(*) FROM tracks t JOIN clips c ON c.id = t.clip_id WHERE c.video_id = v.id)::int AS track_count,
    (SELECT count(*) FROM annotations a JOIN frames f ON f.id = a.frame_id WHERE f.video_id = v.id)::int AS box_count,
    (SELECT count(*) FROM gold_frames g JOIN frames f ON f.id = g.frame_id WHERE f.video_id = v.id)::int AS gold_frame_count,
    (
        SELECT count(*) FROM reviews r JOIN frames f ON f.id = r.frame_id
        WHERE f.video_id = v.id AND r.resolved_at IS NULL
    )::int AS open_review_count,
    (SELECT count(*) FROM clip_claims cc JOIN clips c ON c.id = cc.clip_id WHERE c.video_id = v.id)::int AS claim_count
FROM videos v
WHERE v.id = ANY(@ids::bigint[]) AND v.deleted_at IS NULL
ORDER BY v.id;

-- name: ListEmbedClips :many
-- Clips to embed, from the given videos and clips. Stale clips have no
-- embeddings or ones from another model than @model, when it is known.
SELECT c.id
FROM clips c
JOIN videos v ON v.id = c.video_id
WHERE v.status = 'ready'
    AND c.deleted_at IS NULL
    AND v.deleted_at IS NULL
    AND (c.video_id = ANY(@video_ids::bigint[]) OR c.id = ANY(@clip_ids::bigint[]))
    AND (NOT @only_stale::bool OR c.embedded_at IS NULL OR (@model::text <> '' AND c.embed_model <> @model::text))
ORDER BY v.id, c.idx;

-- name: CreateContentJob :one
INSERT INTO content_jobs (kind, summary, river_job_ids, created_by)
VALUES (@kind, @summary, @river_job_ids::bigint[], sqlc.narg(created_by)::uuid)
RETURNING *;

-- name: ListContentJobs :many
SELECT
    j.id, j.kind, j.summary, j.river_job_ids, j.created_at,
    u.id AS creator_id, u.name AS creator_name, u.username AS creator_username,
    u.role AS creator_role, u.created_at AS creator_created_at
FROM content_jobs j
LEFT JOIN users u ON u.id = j.created_by
WHERE j.created_at > @since
ORDER BY j.created_at DESC, j.id DESC;
