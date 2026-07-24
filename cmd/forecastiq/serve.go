package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/forecastiq/forecastiq/internal/platform/db"
	"github.com/forecastiq/forecastiq/migrations"
)

func defaultMode() string {
	if m := os.Getenv("FIQ_MODE"); m != "" {
		return m
	}
	return "all"
}

// cmdServe runs the API and/or worker per --mode (api|worker|all).
func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	mode := fs.String("mode", defaultMode(), "run mode: api|worker|all")
	if err := fs.Parse(args); err != nil {
		return err
	}
	runAPI := *mode == "api" || *mode == "all"
	runWorker := *mode == "worker" || *mode == "all"
	if !runAPI && !runWorker {
		return errors.New("invalid --mode (want api|worker|all)")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, err := buildApp(ctx)
	if err != nil {
		return err
	}
	defer app.close()

	if app.cfg.AutoMigrate {
		if err := db.Migrate(migrations.FS, app.cfg.DatabaseURL, app.cfg.MigrationsTable, 0); err != nil {
			return err
		}
		app.logger.InfoContext(ctx, "migrations.applied")
	}
	if app.cfg.AutoSeed {
		if err := seed(ctx, app.pool); err != nil {
			return err
		}
		if err := seedBootstrapAdmin(ctx, app.pool, app.cfg.AuthBootstrapAdminSubject, app.cfg.AuthBootstrapAdminEmail); err != nil {
			return err
		}
		app.logger.InfoContext(ctx, "seed.completed")
	}

	errCh := make(chan error, 2)

	// Metrics server (localhost-bound by default; agent scrapes same host).
	metricsSrv := &http.Server{Addr: app.cfg.MetricsAddr, Handler: app.metrics.Handler()}
	go func() { errCh <- metricsSrv.ListenAndServe() }()
	app.logger.InfoContext(ctx, "metrics.listening", slog.String("addr", app.cfg.MetricsAddr))

	var apiSrv *http.Server
	if runAPI {
		apiSrv = &http.Server{
			Addr:         app.cfg.HTTPAddr,
			Handler:      app.router,
			ReadTimeout:  app.cfg.ReadTimeout,
			WriteTimeout: app.cfg.WriteTimeout,
		}
		go func() { errCh <- apiSrv.ListenAndServe() }()
		app.logger.InfoContext(ctx, "api.listening", slog.String("addr", app.cfg.HTTPAddr))
	}
	// The worker runs under its own cancelable context so shutdown triggered by
	// a server error (not just a signal) still stops and drains it.
	workerCtx, cancelWorker := context.WithCancel(ctx)
	defer cancelWorker()
	var workerDone chan struct{}
	if runWorker {
		workerDone = make(chan struct{})
		go func() {
			defer close(workerDone)
			app.scheduler.Run(workerCtx)
		}()
		app.logger.InfoContext(ctx, "worker.started")
	}

	select {
	case <-ctx.Done():
		app.logger.InfoContext(ctx, "shutdown.signal_received")
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), app.cfg.ShutdownTimeout)
	defer cancel()
	if apiSrv != nil {
		_ = apiSrv.Shutdown(shutdownCtx)
	}
	// Stop the scheduler and wait for it to drain in-flight jobs (bounded by
	// the scheduler's own DrainTimeout) before the pool closes.
	if workerDone != nil {
		cancelWorker()
		<-workerDone
	}
	_ = metricsSrv.Shutdown(shutdownCtx)
	app.logger.InfoContext(ctx, "shutdown.complete")
	return nil
}
