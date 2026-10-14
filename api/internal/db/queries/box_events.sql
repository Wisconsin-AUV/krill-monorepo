-- name: ListBoxStates :many
-- Every box of the track, or only the one on frame_id when it is set, with
-- its frame's status. A frame with no box of the track comes back with the
-- box columns null when frame_id is set.
SELECT
    f.id AS frame_id, f.status AS frame_status, f.status_by, f.status_at,
    t.id AS track_id, t.label_type_id,
    a.status AS box_status, a.source, a.x, a.y, a.width, a.height
FROM tracks t
JOIN frames f ON f.clip_id = t.clip_id
LEFT JOIN annotations a ON a.track_id = t.id AND a.frame_id = f.id
WHERE t.id = @track_id
    AND (sqlc.narg('frame_id')::bigint IS NULL OR f.id = sqlc.narg('frame_id')::bigint)
    AND (sqlc.narg('frame_id')::bigint IS NOT NULL OR a.id IS NOT NULL);

-- name: InsertBoxEvents :exec
INSERT INTO box_events (kind, frame_id, track_id, label_type_id, user_id, source, labeler_id, marked_at)
SELECT
    unnest(@kinds::text[]),
    unnest(@frame_ids::bigint[]),
    unnest(@track_ids::bigint[]),
    unnest(@label_type_ids::bigint[]),
    @user_id::uuid,
    unnest(@sources::text[]),
    nullif(unnest(@labeler_ids::uuid[]), '00000000-0000-0000-0000-000000000000'),
    unnest(@marked_ats::timestamptz[]);

-- name: RecordAcceptedProposals :exec
-- Run before AcceptFrameProposals.
INSERT INTO box_events (kind, frame_id, track_id, label_type_id, user_id, source)
SELECT 'accepted', a.frame_id, a.track_id, t.label_type_id, @user_id::uuid, a.source
FROM annotations a
JOIN tracks t ON t.id = a.track_id
WHERE a.frame_id = @frame_id AND a.status = 'proposed';
