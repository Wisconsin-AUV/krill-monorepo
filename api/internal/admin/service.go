// Package admin backs the admin panel: server status, settings, and compute.
package admin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/dustin/go-humanize"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/compute"
	"github.com/wauv/krill/api/internal/config"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/email"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/settings"
	"github.com/wauv/krill/api/internal/storage"
	"github.com/wauv/krill/api/internal/tracker"
)

type Service struct {
	krillv1connect.UnimplementedAdminServiceHandler
	pool       *pgxpool.Pool
	q          *db.Queries
	store      *storage.Store
	settings   *settings.Store
	releases   *Releases
	nodes      *compute.Registry
	dispatcher *tracker.Dispatcher
	// mailer is the one the auth service uses, so a test email proves codes
	// will arrive.
	mailer    func() email.Sender
	version   string
	startedAt time.Time
}

type Deps struct {
	Pool       *pgxpool.Pool
	Store      *storage.Store
	Settings   *settings.Store
	Nodes      *compute.Registry
	Dispatcher *tracker.Dispatcher
	Mailer     func() email.Sender
	Version    string
}

func NewService(d Deps) *Service {
	return &Service{
		pool:       d.Pool,
		q:          db.New(d.Pool),
		store:      d.Store,
		settings:   d.Settings,
		releases:   NewReleases(d.Version),
		nodes:      d.Nodes,
		dispatcher: d.Dispatcher,
		mailer:     d.Mailer,
		version:    d.Version,
		startedAt:  time.Now(),
	}
}

func ts(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

func (s *Service) GetStatus(ctx context.Context, req *krillv1.GetStatusRequest) (*krillv1.GetStatusResponse, error) {
	cfg := s.settings.Config()
	// A slow GitHub shouldn't hold up the checks, and a cancelled request
	// shouldn't be cached as a failed lookup.
	version := make(chan *krillv1.VersionStatus, 1)
	go func() {
		version <- s.releases.Status(context.WithoutCancel(ctx), s.version, cfg.UpdateCheck, req.GetRefresh())
	}()
	checks := []*krillv1.ServiceCheck{
		s.checkDatabase(ctx),
		s.checkStorage(ctx),
		s.checkWorker(ctx, cfg),
		checkEmail(cfg),
		checkSlack(cfg),
	}
	return &krillv1.GetStatusResponse{
		Version:   <-version,
		Checks:    checks,
		StartedAt: ts(s.startedAt),
	}, nil
}

func check(name string, status krillv1.CheckStatus, format string, args ...any) *krillv1.ServiceCheck {
	return &krillv1.ServiceCheck{Name: name, Status: status, Detail: fmt.Sprintf(format, args...)}
}

const (
	ok      = krillv1.CheckStatus_CHECK_STATUS_OK
	warning = krillv1.CheckStatus_CHECK_STATUS_WARNING
	failed  = krillv1.CheckStatus_CHECK_STATUS_ERROR
	off     = krillv1.CheckStatus_CHECK_STATUS_OFF
)

func (s *Service) checkDatabase(ctx context.Context) *krillv1.ServiceCheck {
	start := time.Now()
	size, err := s.q.DatabaseSize(ctx)
	if err != nil {
		return check("Database", failed, "%s", err)
	}
	return check("Database", ok, "%s, answered in %s", humanize.IBytes(uint64(size)), round(time.Since(start))) //nolint:gosec // sizes are positive
}

func (s *Service) checkStorage(ctx context.Context) *krillv1.ServiceCheck {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	start := time.Now()
	if err := s.store.Ping(ctx); err != nil {
		return check("Storage", failed, "%s", err)
	}
	latency := round(time.Since(start))
	u, err := s.store.Usage(ctx)
	if err != nil {
		return check("Storage", ok, "Bucket %s answered in %s", s.store.Bucket(), latency)
	}
	status := ok
	if float64(u.UsedBytes) >= storageWarnAt*float64(u.TotalBytes) {
		status = warning
	}
	return check("Storage", status, "%s of %s used, answered in %s",
		humanize.IBytes(u.UsedBytes), humanize.IBytes(u.TotalBytes), latency)
}

// storageWarnAt leaves room for a few more videos before uploads start
// failing.
const storageWarnAt = 0.9

func round(d time.Duration) time.Duration {
	if d < time.Millisecond {
		return d.Round(time.Microsecond)
	}
	return d.Round(time.Millisecond)
}

func (s *Service) checkWorker(ctx context.Context, cfg config.Config) *krillv1.ServiceCheck {
	const name = "GPU worker"
	if cfg.WorkerToken == "" {
		saved, err := s.q.CountActiveWorkerTokens(ctx)
		if err != nil {
			return check(name, failed, "%s", err)
		}
		if saved == 0 {
			return check(name, off, "No worker token, so tracking and embeddings are off")
		}
	}
	var online []compute.Worker
	for _, w := range s.nodes.Workers() {
		if w.Online {
			online = append(online, w)
		}
	}
	if len(online) == 0 {
		return check(name, failed, "No worker connected, so tracking won't start")
	}
	if api := ParseBuild(s.version).Release; api != "" {
		for _, w := range online {
			if v := canonical(w.Version); v != "" && v != api {
				return check(name, warning, "Worker runs %s but the API runs %s", v, api)
			}
		}
	}
	if len(online) == 1 {
		return check(name, ok, "Running %s", online[0].Model)
	}
	return check(name, ok, "%d workers connected", len(online))
}

func checkEmail(cfg config.Config) *krillv1.ServiceCheck {
	switch {
	case cfg.ResendAPIKey != "":
		return check("Email", ok, "Sending through Resend from %s", cfg.EmailFrom)
	case cfg.LogEmails:
		return check("Email", warning, "No Resend key, so emails go to the API log")
	}
	return check("Email", off, "No Resend key, so password resets and sign-in codes are off")
}

func checkSlack(cfg config.Config) *krillv1.ServiceCheck {
	if cfg.SlackClientID == "" {
		return check("Slack sign-in", off, "No client ID")
	}
	return check("Slack sign-in", ok, "Workspace %s", cfg.SlackTeamID)
}

func (s *Service) ListSettings(context.Context, *krillv1.ListSettingsRequest) (*krillv1.ListSettingsResponse, error) {
	return &krillv1.ListSettingsResponse{Settings: s.listSettings()}, nil
}

func (s *Service) listSettings() []*krillv1.Setting {
	entries := s.settings.List()
	out := make([]*krillv1.Setting, len(entries))
	for i, e := range entries {
		choices := make([]*krillv1.SettingChoice, len(e.Choices))
		for j, c := range e.Choices {
			choices[j] = &krillv1.SettingChoice{Value: c.Value, Label: c.Label}
		}
		out[i] = &krillv1.Setting{
			Key:          e.Key,
			Group:        e.Group,
			Label:        e.Label,
			Description:  e.Description,
			Kind:         kinds[e.Kind],
			Choices:      choices,
			DefaultValue: e.Default,
			EnvOnly:      e.EnvOnly,
			Source:       sources[e.Source],
			Value:        e.Value,
			IsSet:        e.IsSet,
			UpdatedAt:    ts(e.UpdatedAt),
			UpdatedBy:    e.UpdatedBy,
		}
	}
	return out
}

var kinds = map[config.Kind]krillv1.SettingKind{
	config.KindString: krillv1.SettingKind_SETTING_KIND_STRING,
	config.KindSecret: krillv1.SettingKind_SETTING_KIND_SECRET,
	config.KindBool:   krillv1.SettingKind_SETTING_KIND_BOOL,
	config.KindInt:    krillv1.SettingKind_SETTING_KIND_INT,
	config.KindFloat:  krillv1.SettingKind_SETTING_KIND_FLOAT,
	config.KindChoice: krillv1.SettingKind_SETTING_KIND_CHOICE,
}

var sources = map[settings.Source]krillv1.SettingSource{
	settings.SourceDefault: krillv1.SettingSource_SETTING_SOURCE_DEFAULT,
	settings.SourceEnv:     krillv1.SettingSource_SETTING_SOURCE_ENVIRONMENT,
	settings.SourceSaved:   krillv1.SettingSource_SETTING_SOURCE_SAVED,
}

func (s *Service) UpdateSettings(ctx context.Context, req *krillv1.UpdateSettingsRequest) (*krillv1.UpdateSettingsResponse, error) {
	if len(req.GetChanges()) == 0 {
		return nil, rpc.Invalid("no changes")
	}
	changes := make([]settings.Change, len(req.GetChanges()))
	for i, c := range req.GetChanges() {
		changes[i] = settings.Change{Key: c.GetKey(), Value: c.GetValue(), Reset: c.GetUseDefault()}
	}
	if err := s.settings.Update(ctx, auth.CallerID(ctx), changes); err != nil {
		var fe *config.FieldError
		if !errors.As(err, &fe) {
			return nil, rpc.Internal(err, "save settings")
		}
		cerr := connect.NewError(connect.CodeInvalidArgument, err)
		if d, derr := connect.NewErrorDetail(&krillv1.SettingError{Key: fe.Key, Message: fe.Message}); derr == nil {
			cerr.AddDetail(d)
		}
		return nil, cerr
	}
	return &krillv1.UpdateSettingsResponse{Settings: s.listSettings()}, nil
}

func (s *Service) SendTestEmail(ctx context.Context, _ *krillv1.SendTestEmailRequest) (*krillv1.SendTestEmailResponse, error) {
	sess, _ := auth.SessionFrom(ctx)
	mailer := s.mailer()
	if mailer == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("set a Resend API key first"))
	}
	msg, err := email.TestEmail(sess.User.Email, email.Test{
		Username: sess.User.Username,
		Team:     s.settings.Config().TeamName,
		Year:     time.Now().Year(),
	})
	if err != nil {
		return nil, rpc.Internal(err, "render email")
	}
	if err := mailer.Send(ctx, msg); err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("email was not sent: %w", err))
	}
	_, logged := mailer.(email.Log)
	return &krillv1.SendTestEmailResponse{To: sess.User.Email, Logged: logged}, nil
}

func (s *Service) GetCompute(ctx context.Context, _ *krillv1.GetComputeRequest) (*krillv1.GetComputeResponse, error) {
	nodes := s.nodes.Nodes(func(id int64) *krillv1.RunningTask {
		t, ok := s.dispatcher.Running(id)
		if !ok {
			return nil
		}
		return &krillv1.RunningTask{
			Id:        t.ID,
			Kind:      t.Kind,
			Frames:    int32(t.Frames), //nolint:gosec // frame counts fit in int32
			StartedAt: ts(t.StartedAt),
		}
	})
	interactive, background, err := gpuBacklog(ctx, s.pool, s.dispatcher)
	if err != nil {
		return nil, rpc.Internal(err, "count GPU jobs")
	}
	storage := &krillv1.ObjectStorage{}
	if u, err := s.store.Usage(ctx); err != nil {
		storage.Error = err.Error()
	} else {
		storage.UsedBytes, storage.TotalBytes = u.UsedBytes, u.TotalBytes
	}
	return &krillv1.GetComputeResponse{
		Storage: storage,
		Nodes:   nodes,
		Queue: &krillv1.GpuQueue{
			Interactive: int32(interactive), //nolint:gosec // queue lengths fit in int32
			Background:  int32(background),  //nolint:gosec // queue lengths fit in int32
		},
		SampleIntervalSeconds: int32(compute.Interval / time.Second),
		SamModel:              s.settings.Config().SamModel,
	}, nil
}
