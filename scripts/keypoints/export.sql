-- Everything overlay.py needs about one label type, as one JSON document.
-- Run with psql -v type=<name or id>.
SELECT json_build_object(
    'label_type', (
        SELECT row_to_json(l) FROM (
            SELECT id, name, keypoints, quads, reference_key
            FROM label_types
            WHERE name = :'type' OR id::text = :'type'
        ) l
    ),
    'videos', (
        SELECT coalesce(json_agg(json_build_object('id', id, 'name', name)), '[]')
        FROM videos
        WHERE deleted_at IS NULL
    ),
    'clips', (
        SELECT coalesce(json_agg(json_build_object(
            'id', id, 'video_id', video_id, 'idx', idx, 'start_frame', start_frame, 'frame_count', frame_count
        )), '[]')
        FROM clips
        WHERE deleted_at IS NULL
    ),
    'boxes', (
        SELECT coalesce(json_agg(json_build_object(
            'id', a.id,
            'track_id', a.track_id,
            'video_id', f.video_id,
            'idx', f.idx,
            'box', json_build_array(a.x, a.y, a.width, a.height),
            'keypoints', a.keypoints,
            'source', a.source,
            'status', a.status
        ) ORDER BY f.video_id, f.idx, a.track_id), '[]')
        FROM annotations a
        JOIN tracks t ON t.id = a.track_id
        JOIN frames f ON f.id = a.frame_id
        JOIN label_types l ON l.id = t.label_type_id
        WHERE (l.name = :'type' OR l.id::text = :'type') AND a.status <> 'rejected'
    )
);
