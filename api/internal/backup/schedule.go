package backup

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/jackc/pgx/v5"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/config"
	"github.com/wauv/krill/api/internal/db"
)

// lastDue is the latest scheduled time at or before now.
func lastDue(now time.Time, hour int) time.Time {
	d := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	if d.After(now) {
		d = time.Date(now.Year(), now.Month(), now.Day()-1, hour, 0, 0, 0, now.Location())
	}
	return d
}

func nextDue(now time.Time, hour int) time.Time {
	d := lastDue(now, hour)
	return time.Date(d.Year(), d.Month(), d.Day()+1, hour, 0, 0, 0, d.Location())
}

// due reports whether a scheduled backup should start, given when the last
// one started. Missing the hour, such as while the API was down, makes one
// due until the next.
func due(now time.Time, hour int, last time.Time) bool {
	return hour >= 0 && last.Before(lastDue(now, hour))
}

// staleAfter allows for a backup that starts late and takes a while before
// warning that the schedule isn't producing them.
const staleAfter = 26 * time.Hour

// Check describes how backups are doing for the admin overview.
func Check(ctx context.Context, q *db.Queries, cfg config.Config, now time.Time) *krillv1.ServiceCheck {
	const name = "Backups"
	result := func(status krillv1.CheckStatus, format string, args ...any) *krillv1.ServiceCheck {
		return &krillv1.ServiceCheck{Name: name, Status: status, Detail: fmt.Sprintf(format, args...)}
	}
	success, err := q.GetLastSuccessfulBackup(ctx)
	hasSuccess := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return result(krillv1.CheckStatus_CHECK_STATUS_ERROR, "%s", err)
	}
	if cfg.BackupHour < 0 {
		if hasSuccess {
			return result(krillv1.CheckStatus_CHECK_STATUS_OFF, "No schedule, last backup %s", humanize.RelTime(success.CreatedAt.Time, now, "ago", "from now"))
		}
		return result(krillv1.CheckStatus_CHECK_STATUS_OFF, "No schedule")
	}
	if err := Available(); err != nil {
		return result(krillv1.CheckStatus_CHECK_STATUS_ERROR, "pg_dump is not installed, so scheduled backups can't run")
	}
	last, err := q.GetLastBackup(ctx)
	if err == nil && last.Status == "failed" {
		return result(krillv1.CheckStatus_CHECK_STATUS_ERROR, "Last backup failed: %s", last.Error)
	}
	if !hasSuccess {
		return result(krillv1.CheckStatus_CHECK_STATUS_WARNING, "No backup yet")
	}
	ago := humanize.RelTime(success.CreatedAt.Time, now, "ago", "from now")
	if now.Sub(success.CreatedAt.Time) > staleAfter {
		return result(krillv1.CheckStatus_CHECK_STATUS_WARNING, "Last backup %s", ago)
	}
	return result(krillv1.CheckStatus_CHECK_STATUS_OK, "Last backup %s, %s", ago, humanize.IBytes(uint64(max(success.SizeBytes, 0))))
}
