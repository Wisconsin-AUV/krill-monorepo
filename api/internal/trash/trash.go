// Package trash moves videos and clips to the trash and back, and deletes
// them for good. Trashed content keeps its rows, so every read path leaves
// it out and restoring brings its labels back.
package trash

import (
	"context"
	"errors"
	"strconv"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wauv/krill/api/internal/audit"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
)

// TrashVideo runs in the caller's transaction, like the rest of this file.
// The video's claims are released so nobody's queue holds its clips.
func TrashVideo(ctx context.Context, q *db.Queries, id int64) (db.Video, error) {
	v, err := q.TrashVideo(ctx, db.TrashVideoParams{ID: id, UserID: caller(ctx)})
	if err != nil {
		return db.Video{}, rpc.DBError(err, "video")
	}
	clips, err := q.ListLiveClipIDs(ctx, id)
	if err != nil {
		return db.Video{}, rpc.Internal(err, "list clips")
	}
	if _, err := q.ReleaseClipClaims(ctx, clips); err != nil {
		return db.Video{}, rpc.Internal(err, "release claims")
	}
	if err := record(ctx, q, audit.VideoTrash, audit.TargetVideo, id, v.Name, nil); err != nil {
		return db.Video{}, err
	}
	return v, nil
}

func RestoreVideo(ctx context.Context, q *db.Queries, id int64) (db.Video, error) {
	v, err := q.RestoreVideo(ctx, id)
	if err != nil {
		return db.Video{}, rpc.DBError(err, "trashed video")
	}
	if err := record(ctx, q, audit.VideoRestore, audit.TargetVideo, id, v.Name, nil); err != nil {
		return db.Video{}, err
	}
	return v, nil
}

// TrashClip renumbers the clips after it so the video has no gap.
func TrashClip(ctx context.Context, q *db.Queries, id int64) error {
	videoID, err := q.SubtractClipFrames(ctx, id)
	if err != nil {
		return rpc.DBError(err, "clip")
	}
	name, err := audit.ClipName(ctx, q, id)
	if err != nil {
		return rpc.Internal(err, "name clip")
	}
	if err := q.TrashClip(ctx, db.TrashClipParams{ID: id, UserID: caller(ctx)}); err != nil {
		return rpc.Internal(err, "trash clip")
	}
	if _, err := q.ReleaseClipClaims(ctx, []int64{id}); err != nil {
		return rpc.Internal(err, "release claim")
	}
	if err := renumber(ctx, q, videoID); err != nil {
		return err
	}
	return record(ctx, q, audit.ClipTrash, audit.TargetClip, id, name, nil)
}

// RestoreClip puts the clip back in its place among the video's clips. It
// fails while the video itself is in the trash.
func RestoreClip(ctx context.Context, q *db.Queries, id int64) error {
	videoID, err := q.AddClipFrames(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return connect.NewError(connect.CodeNotFound, errors.New("trashed clip not found, or its video is in the trash"))
	}
	if err != nil {
		return rpc.Internal(err, "restore clip")
	}
	if err := q.ParkClips(ctx, videoID); err != nil {
		return rpc.Internal(err, "renumber clips")
	}
	if err := q.RestoreClip(ctx, id); err != nil {
		return rpc.Internal(err, "restore clip")
	}
	if err := q.NumberClips(ctx, videoID); err != nil {
		return rpc.Internal(err, "renumber clips")
	}
	name, err := audit.ClipName(ctx, q, id)
	if err != nil {
		return rpc.Internal(err, "name clip")
	}
	return record(ctx, q, audit.ClipRestore, audit.TargetClip, id, name, nil)
}

func caller(ctx context.Context) pgtype.UUID {
	s, ok := auth.SessionFrom(ctx)
	return pgtype.UUID{Bytes: s.User.ID, Valid: ok}
}

func renumber(ctx context.Context, q *db.Queries, videoID int64) error {
	if err := q.ParkClips(ctx, videoID); err != nil {
		return rpc.Internal(err, "renumber clips")
	}
	if err := q.NumberClips(ctx, videoID); err != nil {
		return rpc.Internal(err, "renumber clips")
	}
	return nil
}

func record(ctx context.Context, q *db.Queries, action, targetType string, id int64, name string, details any) error {
	err := audit.Record(ctx, q, audit.Entry{
		Action:     action,
		TargetType: targetType,
		TargetID:   strconv.FormatInt(id, 10),
		TargetName: name,
		Details:    details,
	})
	if err != nil {
		return rpc.Internal(err, "record audit entry")
	}
	return nil
}
