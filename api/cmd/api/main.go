package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/annotation"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/clip"
	"github.com/wauv/krill/api/internal/config"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/export"
	"github.com/wauv/krill/api/internal/health"
	"github.com/wauv/krill/api/internal/ingest"
	"github.com/wauv/krill/api/internal/storage"
	"github.com/wauv/krill/api/internal/taxonomy"
	"github.com/wauv/krill/api/internal/user"
	"github.com/wauv/krill/api/internal/video"
	"github.com/wauv/krill/api/internal/web"
)

// version is set at build time with -ldflags "-X main.version=vX.Y.Z".
var version = "dev"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(); err != nil {
		slog.Error("api exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		return err
	}
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

	workers := river.NewWorkers()
	river.AddWorker(workers, ingest.NewWorker(pool, store))
	river.AddWorker(workers, export.NewWorker(pool, store, version))
	jobs, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Logger: slog.Default(),
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 2},
			ingest.Queue:       {MaxWorkers: 1},
		},
		Workers: workers,
	})
	if err != nil {
		return fmt.Errorf("create job client: %w", err)
	}
	if err := jobs.Start(ctx); err != nil {
		return fmt.Errorf("start jobs: %w", err)
	}

	cookies := auth.Cookies{Secure: strings.HasPrefix(cfg.PublicURL, "https://")}
	rpcOpts := connect.WithInterceptors(auth.NewInterceptor(db.New(pool).GetSessionUser))
	mux := http.NewServeMux()
	mux.Handle(krillv1connect.NewHealthServiceHandler(health.NewService(version), rpcOpts))
	mux.Handle(krillv1connect.NewAuthServiceHandler(auth.NewService(pool, auth.Options{
		Cookies:     cookies,
		AllowSignup: cfg.AllowSignup,
	}), rpcOpts))
	mux.Handle(krillv1connect.NewVideoServiceHandler(video.NewService(pool, store, jobs), rpcOpts))
	mux.Handle(krillv1connect.NewClipServiceHandler(clip.NewService(pool, store), rpcOpts))
	mux.Handle(krillv1connect.NewLabelServiceHandler(taxonomy.NewService(pool), rpcOpts))
	mux.Handle(krillv1connect.NewAnnotationServiceHandler(annotation.NewService(pool), rpcOpts))
	mux.Handle(krillv1connect.NewExportServiceHandler(export.NewService(pool, store, jobs), rpcOpts))
	mux.Handle(krillv1connect.NewUserServiceHandler(user.NewService(pool), rpcOpts))
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
