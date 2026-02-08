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

	"github.com/ryuju-aono/pavlok-llm-manager/internal/config"
	"github.com/ryuju-aono/pavlok-llm-manager/internal/handler"
	"github.com/ryuju-aono/pavlok-llm-manager/internal/repository"
	"github.com/ryuju-aono/pavlok-llm-manager/internal/safety"
	"github.com/ryuju-aono/pavlok-llm-manager/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("application error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize repository based on DB type
	var repo repository.Repository
	if cfg.IsFirestore() {
		logger.Info("using Firestore repository", slog.String("project_id", cfg.GCPProjectID))
		firestoreRepo, err := repository.NewFirestoreRepository(ctx, cfg.GCPProjectID)
		if err != nil {
			return fmt.Errorf("failed to create firestore repository: %w", err)
		}
		repo = firestoreRepo
	} else {
		logger.Info("using SQLite repository", slog.String("path", cfg.SQLitePath))
		sqliteRepo, err := repository.NewSQLiteRepository(cfg.SQLitePath)
		if err != nil {
			return fmt.Errorf("failed to create sqlite repository: %w", err)
		}
		repo = sqliteRepo
	}
	defer repo.Close()

	geminiService, err := service.NewGeminiService(ctx, cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create gemini service: %w", err)
	}
	defer geminiService.Close()

	lineService, err := service.NewLineService(cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create line service: %w", err)
	}

	pavlokService := service.NewPavlokService(cfg, logger)

	safetyChecker := safety.New(cfg, repo)

	schedulerService := service.NewSchedulerService(
		cfg,
		repo,
		geminiService,
		pavlokService,
		lineService,
		safetyChecker,
		logger,
	)

	cronService := service.NewCronService(cfg, schedulerService, logger)
	cronService.Start()
	defer cronService.Stop()

	webhookHandler := handler.NewWebhookHandler(lineService, schedulerService, logger)
	apiHandler := handler.NewAPIHandler(repo, logger)

	mux := http.NewServeMux()

	mux.HandleFunc("/webhook/line", webhookHandler.HandleLineWebhook)
	mux.HandleFunc("/health", apiHandler.HandleHealth)
	mux.HandleFunc("/api/schedules/today", apiHandler.HandleGetTodaySchedules)
	mux.HandleFunc("/api/stats", apiHandler.HandleGetStats)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      loggingMiddleware(logger, mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting server", slog.String("port", cfg.Port))
		serverErr <- server.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return fmt.Errorf("server error: %w", err)
	case sig := <-quit:
		logger.Info("received signal, shutting down", slog.String("signal", sig.String()))
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}

	logger.Info("server stopped")
	return nil
}

func loggingMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		logger.Info("request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", wrapped.statusCode),
			slog.Duration("duration", time.Since(start)),
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
