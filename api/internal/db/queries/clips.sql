-- name: GetClip :one
SELECT c.* FROM clips c
JOIN videos v ON v.id = c.video_id
WHERE c.id = $1 AND c.deleted_at IS NULL AND v.deleted_at IS NULL;

-- name: GetClipLabelStats :one
SELECT
    (SELECT count(*) FROM frames f WHERE f.clip_id = $1 AND f.status <> 'unlabeled')::int AS labeled_frame_count,
    (SELECT count(*) FROM annotations a JOIN tracks t ON t.id = a.track_id WHERE t.clip_id = $1)::int AS box_count;

-- name: ListClipFrames :many
SELECT * FROM frames WHERE clip_id = $1 ORDER BY idx;

-- name: GetNeighbourClips :one
SELECT
    coalesce((
        SELECT p.id FROM clips p WHERE p.video_id = c.video_id AND p.idx = c.idx - 1 AND p.deleted_at IS NULL
    ), 0)::bigint AS previous_id,
    coalesce((
        SELECT n.id FROM clips n WHERE n.video_id = c.video_id AND n.idx = c.idx + 1 AND n.deleted_at IS NULL
    ), 0)::bigint AS next_id
FROM clips c
WHERE c.id = $1;

-- name: SetClipEmbedded :exec
UPDATE clips SET embedded_at = now(), embed_model = $2 WHERE id = $1;

-- name: SubtractClipFrames :one
-- Locks the video row before touching its clips, so two clips trashed or
-- restored in one video renumber one after the other instead of
-- deadlocking.
UPDATE videos v SET frame_count = v.frame_count - c.frame_count
FROM clips c
WHERE c.id = $1 AND v.id = c.video_id AND c.deleted_at IS NULL AND v.deleted_at IS NULL
RETURNING v.id AS video_id;

-- name: AddClipFrames :one
UPDATE videos v SET frame_count = v.frame_count + c.frame_count
FROM clips c
WHERE c.id = $1 AND v.id = c.video_id AND c.deleted_at IS NOT NULL AND v.deleted_at IS NULL
RETURNING v.id AS video_id;

-- name: TrashClip :exec
UPDATE clips SET deleted_at = now(), deleted_by = sqlc.narg(user_id)::uuid WHERE id = @id;

-- name: RestoreClip :exec
-- Run after ParkClips. A live clip may hold the index the clip had, so it
-- comes back parked below the others until NumberClips.
UPDATE clips c SET deleted_at = NULL, deleted_by = NULL, idx = (
    SELECT least(min(l.idx), 0) - 1 FROM clips l WHERE l.video_id = c.video_id AND l.deleted_at IS NULL
)
WHERE c.id = $1;

-- name: ParkClips :exec
-- The unique index on (video_id, idx) is checked row by row, so renumbering
-- in one UPDATE can collide with a row that hasn't moved yet. Parking them
-- at negative indexes first avoids that.
UPDATE clips SET idx = -1 - idx WHERE video_id = $1 AND deleted_at IS NULL;

-- name: NumberClips :exec
-- Clips never overlap, so start_frame keeps a restored clip where it was.
UPDATE clips c SET idx = n.idx
FROM (
    SELECT l.id, (row_number() OVER (ORDER BY l.start_frame, l.id) - 1)::int AS idx
    FROM clips l
    WHERE l.video_id = $1 AND l.deleted_at IS NULL
) n
WHERE c.id = n.id;

-- name: ReleaseClipClaims :execrows
DELETE FROM clip_claims WHERE clip_id = ANY(@clip_ids::bigint[]);

-- name: DeleteClip :one
DELETE FROM clips WHERE id = $1 AND deleted_at IS NOT NULL
RETURNING video_id, start_frame, frame_count;
