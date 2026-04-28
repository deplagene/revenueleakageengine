// Package main starts the Revenue Leakage Engine application binary.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	caseapp "github.com/deplagene/revenueleakageengine/internal/app/case"
	"github.com/deplagene/revenueleakageengine/internal/app/config"
	contractapp "github.com/deplagene/revenueleakageengine/internal/app/contract"
	gatewayhttp "github.com/deplagene/revenueleakageengine/internal/app/gateway/http"
	httpmiddleware "github.com/deplagene/revenueleakageengine/internal/app/gateway/middleware"
	appreconciliation "github.com/deplagene/revenueleakageengine/internal/app/reconciliation"
	"github.com/deplagene/revenueleakageengine/internal/migrator"
	"github.com/deplagene/revenueleakageengine/internal/platform/sqlite"
	casework "github.com/deplagene/revenueleakageengine/internal/service/case"
	contractwork "github.com/deplagene/revenueleakageengine/internal/service/contract"
	reconciliationservice "github.com/deplagene/revenueleakageengine/internal/service/reconciliation"
	revenueservice "github.com/deplagene/revenueleakageengine/internal/service/revenue"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/theartofdevel/logging"
)

// main is the process entrypoint.
func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "revenueleakageengine: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	cfg := config.Load()
	logger := newLogger(cfg.Logging)
	ctx = logging.ContextWithLogger(ctx, logger)

	db, err := sqlite.Open(ctx, sqlite.LocalDSN(cfg.SQLite.Path))
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}
	defer db.Close()

	if err := runMigrations(ctx, db, cfg.Migrations.Path); err != nil {
		return err
	}

	reconciliationWorkflow, err := buildReconciliationWorkflow(db)
	if err != nil {
		return err
	}

	caseQueries, caseCommands, err := buildCaseUseCases(db)
	if err != nil {
		return err
	}

	contractQueries, contractCommands, err := buildContractUseCases(db)
	if err != nil {
		return err
	}

	httpHandler, err := gatewayhttp.NewHandler(
		reconciliationWorkflow,
		caseQueries,
		caseCommands,
		contractQueries,
		contractCommands,
	)

	server := newHTTPServer(cfg.HTTP, newRouter(logger, cfg.HTTP, httpHandler))
	errCh := startHTTPServer(server, logger)

	return waitForShutdown(ctx, server, cfg.HTTP.ShutdownTimeout, errCh, logger)
}

func newLogger(cfg config.LoggingConfig) *logging.Logger {
	return logging.NewLogger(
		logging.WithLevel(cfg.Level),
		logging.WithIsJSON(cfg.IsJSON),
	)
}

func buildReconciliationWorkflow(db *sql.DB) (*appreconciliation.RevenueLeakageWorkflow, error) {
	revenueStore := revenueservice.NewSQLiteStore(db)
	revenueService := revenueservice.NewService(
		revenueservice.WithStore(revenueStore),
	)

	reconciliationStore := reconciliationservice.NewSQLiteStore(db)
	reconciliationService, err := reconciliationservice.NewService(reconciliationStore)
	if err != nil {
		return nil, fmt.Errorf("build reconciliation service: %w", err)
	}

	workflow, err := appreconciliation.NewRevenueLeakageWorkflow(
		revenueService,
		reconciliationService,
	)
	if err != nil {
		return nil, fmt.Errorf("build reconciliation workflow: %w", err)
	}

	return workflow, nil
}

func buildCaseUseCases(db *sql.DB) (*caseapp.Queries, *caseapp.Commands, error) {
	caseStore := casework.NewSQLiteStore(db)
	caseService, err := casework.NewService(caseStore)
	if err != nil {
		return nil, nil, fmt.Errorf("build case service: %w", err)
	}

	queries, err := caseapp.NewQueries(caseService)
	if err != nil {
		return nil, nil, fmt.Errorf("build case queries: %w", err)
	}

	commands, err := caseapp.NewCommands(caseService)
	if err != nil {
		return nil, nil, fmt.Errorf("build case commands: %w", err)
	}

	return queries, commands, nil
}

func newRouter(
	logger *logging.Logger,
	cfg config.HTTPConfig,
	handler *gatewayhttp.Handler,
) http.Handler {
	router := chi.NewRouter()

	router.Use(httpmiddleware.LoggerContext(logger))
	router.Use(chimiddleware.RequestID)
	router.Use(chimiddleware.RealIP)
	router.Use(chimiddleware.Timeout(cfg.ReadTimeout))
	router.Use(chimiddleware.Recoverer)
	router.Use(httpmiddleware.RequestLogger())
	router.Use(httprate.LimitByIP(cfg.RateLimit, cfg.RateLimitWindow))

	router.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	handler.RegisterRoutes(router)

	return router
}

func newHTTPServer(cfg config.HTTPConfig, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
	}
}

func runMigrations(ctx context.Context, db *sql.DB, migrationPath string) error {
	migrator := migrator.NewMigrator(db, migrationPath)
	if err := migrator.Up(ctx); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}

func startHTTPServer(server *http.Server, logger *logging.Logger) <-chan error {
	errCh := make(chan error, 1)

	go func() {
		logger.Info("http server starting", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("listen and serve: %w", err)
			return
		}

		errCh <- nil
	}()

	return errCh
}

func waitForShutdown(
	ctx context.Context,
	server *http.Server,
	shutdownTimeout time.Duration,
	errCh <-chan error,
	logger *logging.Logger,
) error {
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signalCh)

	select {
	case err := <-errCh:
		return err
	case signalValue := <-signalCh:
		logger.Info("shutdown signal received", "signal", signalValue.String())
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}

	if err := <-errCh; err != nil {
		return err
	}

	logger.Info("http server stopped")
	return nil
}

func buildContractUseCases(db *sql.DB) (*contractapp.Queries, *contractapp.Commands, error) {
	contractStore := contractwork.NewSQLiteStore(db)
	contractService, err := contractwork.NewService(contractStore)
	if err != nil {
		return nil, nil, fmt.Errorf("build contract service: %w", err)
	}

	queries := contractapp.NewQueries(contractService)
	commands := contractapp.NewCommands(contractService)

	return queries, commands, nil
}
