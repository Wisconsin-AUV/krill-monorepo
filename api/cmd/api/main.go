package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/admin"
	"github.com/wauv/krill/api/internal/annotation"
	"github.com/wauv/krill/api/internal/audit"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/clip"
	"github.com/wauv/krill/api/internal/compute"
	"github.com/wauv/krill/api/internal/config"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/email"
	"github.com/wauv/krill/api/internal/export"
	"github.com/wauv/krill/api/internal/gold"
	"github.com/wauv/krill/api/internal/health"
	"github.com/wauv/krill/api/internal/ingest"
	"github.com/wauv/krill/api/internal/insight"
	"github.com/wauv/krill/api/internal/queue"
	"github.com/wauv/krill/api/internal/review"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/settings"
	"github.com/wauv/krill/api/internal/stats"
	"github.com/wauv/krill/api/internal/storage"
	"github.com/wauv/krill/api/internal/taxonomy"
	"github.com/wauv/krill/api/internal/tracker"
	"github.com/wauv/krill/api/internal/user"
	"github.com/wauv/krill/api/internal/video"
	"github.com/wauv/krill/api/internal/web"
)

// version is set at build time with -ldflags "-X main.version=$(git
// describe --tags --always --dirty)".
var version = "dev"

// commitVersion names a plain go build by its commit, which Go records when
// building inside the repository.
func commitVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	var rev string
	dirty := false
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if len(rev) < 7 {
		return version
	}
	if dirty {
		return "dev-" + rev[:7] + "-dirty"
	}
	return "dev-" + rev[:7]
}

func main() {
	if version == "dev" {
		version = commitVersion()
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(); err != nil {
		slog.Error("api exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	// Everything else can be saved in the database, so it is parsed once
	// the database is reachable.
	databaseURL, ok := config.Env("KRILL_DATABASE_URL")
	if !ok {
		return errors.New("KRILL_DATABASE_URL is required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		return err
	}
	settingsStore, err := settings.Open(ctx, pool, config.Env)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	// Only env-only fields may be read from cfg. The rest come from live so
	// changes saved in the admin panel apply without a restart.
	cfg := settingsStore.Config()
	cookies := auth.Cookies{Secure: strings.HasPrefix(cfg.PublicURL, "https://")}
	var live atomic.Pointer[runtime]
	settingsStore.OnChange(func(c config.Config) { live.Store(newRuntime(c, cookies)) })
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("create river migrator: %w", err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("migrate river: %w", err)
	}
	// Only one API process runs ingest, so anything still processing at boot
	// was cut off by a restart and will never finish.
	if n, err := db.New(pool).FailInterruptedIngests(ctx); err != nil {
		return fmt.Errorf("fail interrupted ingests: %w", err)
	} else if n > 0 {
		slog.Warn("marked interrupted ingests as failed", "count", n)
	}
	if n, err := db.New(pool).FailInterruptedDatasets(ctx); err != nil {
		return fmt.Errorf("fail interrupted exports: %w", err)
	} else if n > 0 {
		slog.Warn("marked interrupted exports as failed", "count", n)
	}

	store, err := storage.New(ctx, storage.Config{
		Endpoint:       cfg.S3Endpoint,
		PublicEndpoint: cfg.S3PublicEndpoint,
		Bucket:         cfg.S3Bucket,
		AccessKey:      cfg.S3AccessKey,
		SecretKey:      cfg.S3SecretKey,
	})
	if err != nil {
		return err
	}

	dispatcher := tracker.NewDispatcher()
	nodes := compute.NewRegistry()
	go nodes.SampleAPI(ctx, version)
	workers := river.NewWorkers()
	river.AddWorker(workers, ingest.NewWorker(pool, store, func() bool { return live.Load().cfg.EmbedAtIngest }))
	river.AddWorker(workers, export.NewWorker(pool, store, version))
	river.AddWorker(workers, tracker.NewWorker(pool, store, dispatcher))
	river.AddWorker(workers, tracker.NewExemplarWorker(pool, store, dispatcher))
	river.AddWorker(workers, tracker.NewEmbedWorker(pool, store, dispatcher))
	jobs, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Logger: slog.Default(),
		Queues: map[string]river.QueueConfig{
			river.QueueDefault:    {MaxWorkers: 2},
			ingest.Queue:          {MaxWorkers: 1},
			annotation.TrackQueue: {MaxWorkers: 1},
			ingest.EmbedQueue:     {MaxWorkers: 1},
		},
		Workers: workers,
	})
	if err != nil {
		return fmt.Errorf("create job client: %w", err)
	}
	if err := jobs.Start(ctx); err != nil {
		return fmt.Errorf("start jobs: %w", err)
	}

	goldRouting := func() review.GoldRouting { return live.Load().gold }
	rpcOpts := connect.WithInterceptors(
		rpc.ErrorInterceptor{},
		auth.NewInterceptor(db.New(pool).GetSessionUser, func() string { return live.Load().cfg.WorkerToken }).
			RequireVerifiedEmail(func() bool { return live.Load().auth.RequireVerifiedEmail }).
			TrustClientIPHeader(func() string { return live.Load().cfg.ClientIPHeader }),
	)
	mux := http.NewServeMux()
	mux.Handle(krillv1connect.NewHealthServiceHandler(health.NewService(version), rpcOpts))
	mux.Handle(krillv1connect.NewAuthServiceHandler(auth.NewService(pool, func() auth.Options { return live.Load().auth }), rpcOpts))
	slack := auth.NewSlack(pool, func() auth.SlackConfig { return live.Load().slack }, cookies)
	mux.HandleFunc("GET /auth/slack/login", slack.Login)
	mux.HandleFunc("GET /auth/slack/callback", slack.Callback)
	mux.Handle(krillv1connect.NewVideoServiceHandler(video.NewService(pool, store, jobs, func() bool { return live.Load().cfg.EmbedAtIngest }), rpcOpts))
	mux.Handle(krillv1connect.NewClipServiceHandler(clip.NewService(pool, store, jobs), rpcOpts))
	mux.Handle(krillv1connect.NewQueueServiceHandler(queue.NewService(pool, store), rpcOpts))
	mux.Handle(krillv1connect.NewLabelServiceHandler(taxonomy.NewService(pool, store), rpcOpts))
	mux.Handle(krillv1connect.NewAnnotationServiceHandler(annotation.NewService(pool, store, jobs, goldRouting), rpcOpts))
	mux.Handle(krillv1connect.NewWorkerServiceHandler(tracker.NewService(pool, dispatcher, nodes), rpcOpts))
	mux.Handle(krillv1connect.NewExportServiceHandler(export.NewService(pool, store, jobs), rpcOpts))
	mux.Handle(krillv1connect.NewUserServiceHandler(user.NewService(pool), rpcOpts))
	mux.Handle(krillv1connect.NewStatsServiceHandler(stats.NewService(pool), rpcOpts))
	mux.Handle(krillv1connect.NewGoldServiceHandler(gold.NewService(pool, store, goldRouting), rpcOpts))
	mux.Handle(krillv1connect.NewReviewServiceHandler(review.NewService(pool, store), rpcOpts))
	mux.Handle(krillv1connect.NewAdminServiceHandler(admin.NewService(admin.Deps{
		Pool:       pool,
		Store:      store,
		Settings:   settingsStore,
		Nodes:      nodes,
		Dispatcher: dispatcher,
		Mailer:     func() email.Sender { return live.Load().auth.Mailer },
		Version:    version,
	}), rpcOpts))
	mux.Handle(krillv1connect.NewAuditServiceHandler(audit.NewService(pool), rpcOpts))
	mux.Handle(krillv1connect.NewSettingsCheckServiceHandler(admin.NewSettingsCheck(slack, settingsStore,
		func() email.Sender { return live.Load().auth.Mailer }), rpcOpts))
	mux.Handle(krillv1connect.NewInsightServiceHandler(insight.NewService(pool, settingsStore), rpcOpts))
	if cfg.WebDir != "" {
		// No method in the pattern: "GET /" would conflict with the RPC routes.
		mux.Handle("/", web.Handler(cfg.WebDir))
	}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "database unreachable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(version))
	})

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("api listening", "addr", cfg.Addr, "version", version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("serve", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := jobs.Stop(shutdownCtx); err != nil {
		return fmt.Errorf("stop jobs: %w", err)
	}
	return nil
}

// runtime is what the services need from one version of the config.
type runtime struct {
	cfg   config.Config
	auth  auth.Options
	slack auth.SlackConfig
	gold  review.GoldRouting
}

func newRuntime(cfg config.Config, cookies auth.Cookies) *runtime {
	var mailer email.Sender
	switch {
	case cfg.ResendAPIKey != "":
		mailer = email.NewResend(cfg.ResendAPIKey, cfg.EmailFrom)
	case cfg.LogEmails:
		mailer = email.Log{}
	}
	signupDomain := cfg.SignupEmailDomain
	if cfg.EmailVerification == config.VerifyOff {
		signupDomain = ""
	}
	return &runtime{
		cfg: cfg,
		auth: auth.Options{
			Cookies:              cookies,
			AllowSignup:          cfg.AllowSignup,
			SignupEmailDomain:    signupDomain,
			RequireVerifiedEmail: cfg.EmailVerification == config.VerifyCode,
			Mailer:               mailer,
			SlackEnabled:         cfg.SlackClientID != "",
			TeamName:             cfg.TeamName,
			ClientIPHeader:       cfg.ClientIPHeader,
		},
		slack: auth.SlackConfig{
			ClientID:     cfg.SlackClientID,
			ClientSecret: cfg.SlackClientSecret,
			TeamID:       cfg.SlackTeamID,
			PublicURL:    cfg.PublicURL,
		},
		gold: review.GoldRouting{Threshold: cfg.GoldReviewThreshold, MinAttempts: cfg.GoldReviewMinAttempts},
	}
}
