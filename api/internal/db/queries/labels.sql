-- name: ListLabelTypes :many
SELECT
    sqlc.embed(lt),
    (
        SELECT count(*) FROM tracks t JOIN live_clips c ON c.id = t.clip_id
        WHERE t.label_type_id = lt.id
    )::int AS track_count,
    (
        SELECT count(*) FROM annotations a JOIN tracks t ON t.id = a.track_id JOIN live_clips c ON c.id = t.clip_id
        WHERE t.label_type_id = lt.id
    )::int AS box_count
FROM label_types lt
ORDER BY lt.position, lt.id;

-- name: GetLabelType :one
SELECT * FROM label_types WHERE id = $1;

-- name: CreateLabelType :one
INSERT INTO label_types (name, title, color, description, guideline, attributes, position)
VALUES ($1, $2, $3, $4, $5, $6, (SELECT coalesce(max(position), -1) + 1 FROM label_types))
RETURNING *;

-- name: UpdateLabelType :one
UPDATE label_types
SET name = $2, title = $3, color = $4, description = $5, guideline = $6, attributes = $7
WHERE id = $1
RETURNING *;

-- name: DeleteLabelType :execrows
DELETE FROM label_types WHERE id = $1;

-- name: CountTracksForLabelType :one
SELECT
    count(*) FILTER (WHERE clip_id IN (SELECT id FROM live_clips))::int AS live,
    count(*) FILTER (WHERE clip_id NOT IN (SELECT id FROM live_clips))::int AS trashed
FROM tracks WHERE label_type_id = $1;

-- name: SetLabelTypePosition :exec
UPDATE label_types SET position = $2 WHERE id = $1;

-- name: ListLabelExamples :many
SELECT * FROM label_examples ORDER BY label_type_id, id;

-- name: CreateLabelExample :one
INSERT INTO label_examples (label_type_id, object_key, caption)
VALUES ($1, $2, $3)
RETURNING *;

-- name: DeleteLabelExample :one
DELETE FROM label_examples WHERE id = $1
RETURNING *;

-- name: LockLabelTypes :many
SELECT * FROM label_types WHERE id = ANY(@ids::bigint[]) ORDER BY id FOR UPDATE;

-- name: GetMergeImpact :one
-- Covers tracks in the trash too, since they move with the rest.
SELECT
    (SELECT count(*) FROM tracks t WHERE t.label_type_id = @id)::int AS track_count,
    (SELECT count(*) FROM annotations a JOIN tracks t ON t.id = a.track_id WHERE t.label_type_id = @id)::int AS box_count,
    (SELECT count(DISTINCT t.clip_id) FROM tracks t WHERE t.label_type_id = @id)::int AS clip_count,
    (
        SELECT count(DISTINCT c.video_id) FROM tracks t JOIN clips c ON c.id = t.clip_id
        WHERE t.label_type_id = @id
    )::int AS video_count,
    (
        SELECT count(*) FROM tracks t
        WHERE t.label_type_id = @id AND NOT EXISTS (SELECT 1 FROM live_clips c WHERE c.id = t.clip_id)
    )::int AS trashed_track_count,
    (
        SELECT count(*) FROM gold_frames g
        WHERE g.boxes @> jsonb_build_array(jsonb_build_object('label_type_id', @id::bigint))
    )::int AS gold_frame_count,
    (
        SELECT count(*) FROM gold_attempts g
        WHERE g.boxes @> jsonb_build_array(jsonb_build_object('label_type_id', @id::bigint))
    )::int AS gold_attempt_count,
    (
        SELECT count(DISTINCT r.id) FROM reviews r
        JOIN annotations a ON a.frame_id = r.frame_id
        JOIN tracks t ON t.id = a.track_id
        WHERE r.resolved_at IS NULL AND t.label_type_id = @id
    )::int AS open_review_count,
    (SELECT count(*) FROM label_examples e WHERE e.label_type_id = @id)::int AS example_count;

-- name: CountTrackValues :many
SELECT e.key::text AS attribute, e.value::text AS option, count(*)::int AS tracks
FROM tracks t
CROSS JOIN LATERAL jsonb_each_text(
    CASE WHEN jsonb_typeof(t.attributes) = 'object' THEN t.attributes ELSE '{}' END
) AS e (key, value)
WHERE t.label_type_id = $1
GROUP BY e.key, e.value
ORDER BY e.key, e.value;

-- name: MergeTracks :execrows
-- @mapping is {"source attribute": {"source option": ["target attribute",
-- "target option"]}}. Values without a mapping are dropped.
UPDATE tracks t SET
    label_type_id = @target_id,
    attributes = coalesce((
        SELECT jsonb_object_agg(@mapping::jsonb -> e.key -> e.value ->> 0, @mapping::jsonb -> e.key -> e.value ->> 1)
        FROM jsonb_each_text(CASE WHEN jsonb_typeof(t.attributes) = 'object' THEN t.attributes ELSE '{}' END) AS e (key, value)
        WHERE @mapping::jsonb -> e.key -> e.value IS NOT NULL
    ), '{}')
WHERE t.label_type_id = @source_id;

-- name: MergeRejectedPrelabels :exec
UPDATE rejected_prelabels SET label_type_id = @target_id WHERE label_type_id = @source_id;

-- name: MergeGoldFrames :execrows
-- Rewrites the copied boxes the same way as MergeTracks, so the key keeps
-- matching what labelers will draw.
UPDATE gold_frames g SET boxes = (
    SELECT jsonb_agg(
        CASE WHEN (b.box ->> 'label_type_id')::bigint = @source_id::bigint THEN
            b.box || jsonb_build_object(
                'label_type_id', @target_id::bigint,
                'attributes', coalesce((
                    SELECT jsonb_object_agg(@mapping::jsonb -> e.key -> e.value ->> 0, @mapping::jsonb -> e.key -> e.value ->> 1)
                    FROM jsonb_each_text(
                        CASE WHEN jsonb_typeof(b.box -> 'attributes') = 'object' THEN b.box -> 'attributes' ELSE '{}' END
                    ) AS e (key, value)
                    WHERE @mapping::jsonb -> e.key -> e.value IS NOT NULL
                ), '{}'::jsonb)
            )
        ELSE b.box END
        ORDER BY b.n
    )
    FROM jsonb_array_elements(g.boxes) WITH ORDINALITY AS b (box, n)
)
WHERE g.boxes @> jsonb_build_array(jsonb_build_object('label_type_id', @source_id::bigint));

-- name: MergeGoldAttempts :execrows
UPDATE gold_attempts g SET boxes = (
    SELECT jsonb_agg(
        CASE WHEN (b.box ->> 'label_type_id')::bigint = @source_id::bigint THEN
            b.box || jsonb_build_object(
                'label_type_id', @target_id::bigint,
                'attributes', coalesce((
                    SELECT jsonb_object_agg(@mapping::jsonb -> e.key -> e.value ->> 0, @mapping::jsonb -> e.key -> e.value ->> 1)
                    FROM jsonb_each_text(
                        CASE WHEN jsonb_typeof(b.box -> 'attributes') = 'object' THEN b.box -> 'attributes' ELSE '{}' END
                    ) AS e (key, value)
                    WHERE @mapping::jsonb -> e.key -> e.value IS NOT NULL
                ), '{}'::jsonb)
            )
        ELSE b.box END
        ORDER BY b.n
    )
    FROM jsonb_array_elements(g.boxes) WITH ORDINALITY AS b (box, n)
)
WHERE g.boxes @> jsonb_build_array(jsonb_build_object('label_type_id', @source_id::bigint));
