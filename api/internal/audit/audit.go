// Package audit records who changed what. Services record an entry with the
// same queries as the change, inside its transaction, so a change that rolls
// back leaves no entry.
package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
)

const (
	ClipDelete      = "clip.delete"
	ClipRelease     = "clip.release"
	ClipTake        = "clip.take"
	ExportDelete    = "export.delete"
	GoldSet         = "gold.set"
	GoldUnset       = "gold.unset"
	LabelTypeCreate = "label_type.create"
	LabelTypeUpdate = "label_type.update"
	LabelTypeDelete = "label_type.delete"
	ReviewResolve   = "review.resolve"
	SettingUpdate   = "setting.update"
	UserDelete      = "user.delete"
	UserDisable     = "user.disable"
	UserEnable      = "user.enable"
	UserPassword    = "user.password"
	UserRole        = "user.role"
	UserSignOut     = "user.sign_out"
	UserVerifyEmail = "user.verify_email"
	VideoDelete     = "video.delete"

	ApprovedEmailAdd    = "approved_email.add"
	ApprovedEmailRemove = "approved_email.remove"
	InviteCreate        = "invite.create"
	InviteRevoke        = "invite.revoke"
	SessionRevoke       = "session.revoke"
	UserMerge           = "user.merge"
	UserPasswordReset   = "user.password_reset"
	JobRetry            = "job.retry"
	JobCancel           = "job.cancel"
	JobPriority         = "job.priority"
	WorkerWarmUp        = "worker.warm_up"
	WorkerUnload        = "worker.unload"
	WorkerTokenCreate   = "worker_token.create" //nolint:gosec // an action name, not a credential
	WorkerTokenRevoke   = "worker_token.revoke"
	StorageScan         = "storage.scan"
	StorageCleanup      = "storage.cleanup"
	BackupCreate        = "backup.create"
	BackupDownload      = "backup.download"
	BackupDelete        = "backup.delete"
)

const (
	TargetClip      = "clip"
	TargetExport    = "export"
	TargetFrame     = "frame"
	TargetLabelType = "label_type"
	TargetReview    = "review"
	TargetSetting   = "setting"
	TargetUser      = "user"
	TargetVideo     = "video"

	TargetApprovedEmail = "approved_email"
	TargetInvite        = "invite"
	TargetJob           = "job"
	TargetWorker        = "worker"
	TargetWorkerToken   = "worker_token"
	TargetScan          = "storage_scan"
	TargetBackup        = "backup"
)

type Entry struct {
	Action     string
	TargetType string
	TargetID   string
	// TargetName is how the target reads to a person, kept so the entry
	// still makes sense after the target is renamed or deleted.
	TargetName string
	// Details is stored as JSON. Never put a secret in it.
	Details any
}

// Change is the details of an entry that changed one value.
func Change(from, to any) map[string]any {
	return map[string]any{"from": from, "to": to}
}

// Record saves e as done by the signed-in caller from their address.
func Record(ctx context.Context, q *db.Queries, e Entry) error {
	details := []byte("{}")
	if e.Details != nil {
		var err error
		if details, err = json.Marshal(e.Details); err != nil {
			return fmt.Errorf("encode %s details: %w", e.Action, err)
		}
	}
	s, signedIn := auth.SessionFrom(ctx)
	err := q.InsertAuditEntry(ctx, db.InsertAuditEntryParams{
		Actor:      pgtype.UUID{Bytes: s.User.ID, Valid: signedIn},
		ActorName:  s.User.Name,
		Action:     e.Action,
		TargetType: e.TargetType,
		TargetID:   e.TargetID,
		TargetName: e.TargetName,
		Details:    details,
		Ip:         auth.ClientIP(ctx),
	})
	if err != nil {
		return fmt.Errorf("record %s: %w", e.Action, err)
	}
	return nil
}

// ClipName reads like "harbour.mp4 · Clip 3".
func ClipName(ctx context.Context, q *db.Queries, id int64) (string, error) {
	r, err := q.GetClipAuditName(ctx, id)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s · Clip %d", r.VideoName, r.Idx+1), nil
}

// FrameName reads like "harbour.mp4 · Frame 120".
func FrameName(ctx context.Context, q *db.Queries, id int64) (string, error) {
	r, err := q.GetFrameAuditName(ctx, id)
	if err != nil {
		return "", err
	}
	return frameName(r.VideoName, r.Idx), nil
}

func frameName(video string, idx int32) string {
	return fmt.Sprintf("%s · Frame %d", video, idx+1)
}

// ReviewName names the review's frame.
func ReviewName(ctx context.Context, q *db.Queries, id int64) (string, error) {
	r, err := q.GetReviewAuditName(ctx, id)
	if err != nil {
		return "", err
	}
	return frameName(r.VideoName, r.Idx), nil
}
