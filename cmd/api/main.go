package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	infrastructurepostgres "example.com/taskservice/internal/infrastructure/postgres"
	postgresrepo "example.com/taskservice/internal/repository/postgres"
	transporthttp "example.com/taskservice/internal/transport/http"
	swaggerdocs "example.com/taskservice/internal/transport/http/docs"
	httphandlers "example.com/taskservice/internal/transport/http/handlers"
	"example.com/taskservice/internal/usecase/task"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg := loadConfig()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := infrastructurepostgres.Open(ctx, cfg.DatabaseDSN)
	if err != nil {
		logger.Error("open postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	taskRepo := postgresrepo.New(pool)
	taskUsecase := task.NewService(taskRepo)
	taskHandler := httphandlers.NewTaskHandler(taskUsecase, taskRepo)
	docsHandler := swaggerdocs.NewHandler()
	router := transporthttp.NewRouter(taskHandler, docsHandler)

	// Start scheduler for generating recurring task instances
	schedulerCtx, schedulerCancel := context.WithCancel(context.Background())
	defer schedulerCancel()
	go runScheduler(schedulerCtx, taskUsecase, logger, cfg.SchedulerInterval)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown http server", "error", err)
		}
	}()

	logger.Info("http server started", "addr", cfg.HTTPAddr)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("listen and serve", "error", err)
		os.Exit(1)
	}
}

func runScheduler(ctx context.Context, uc task.Usecase, logger *slog.Logger, interval time.Duration) {
	if interval <= 0 {
		interval = 1 * time.Hour
	}

	logger.Info("scheduler started", "interval", interval)

	// Run once immediately on startup
	if err := uc.GenerateInstances(ctx, 30); err != nil {
		logger.Error("scheduler initial run", "error", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("scheduler stopped")
			return
		case <-ticker.C:
			if err := uc.GenerateInstances(ctx, 30); err != nil {
				logger.Error("scheduler tick", "error", err)
			}
		}
	}
}

type config struct {
	HTTPAddr          string
	DatabaseDSN       string
	SchedulerInterval time.Duration
}

func loadConfig() config {
	cfg := config{
		HTTPAddr:          envOrDefault("HTTP_ADDR", ":8080"),
		DatabaseDSN:       envOrDefault("DATABASE_DSN", "postgres://postgres:postgres@localhost:5432/taskservice?sslmode=disable"),
		SchedulerInterval: parseDurationOrDefault("SCHEDULER_INTERVAL", "1h"),
	}

	if cfg.DatabaseDSN == "" {
		panic(fmt.Errorf("DATABASE_DSN is required"))
	}

	return cfg
}

func parseDurationOrDefault(key, fallback string) time.Duration {
	val := fallback
	if v := os.Getenv(key); v != "" {
		val = v
	}

	d, err := time.ParseDuration(val)
	if err != nil {
		return 1 * time.Hour
	}

	return d
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
