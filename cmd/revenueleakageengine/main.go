// Package main starts the Revenue Leakage Engine application binary.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	caseapp "github.com/deplagene/revenueleakageengine/internal/app/case"
	"github.com/deplagene/revenueleakageengine/internal/app/config"
	contractapp "github.com/deplagene/revenueleakageengine/internal/app/contract"
	appevent "github.com/deplagene/revenueleakageengine/internal/app/event"
	gatewaygrpc "github.com/deplagene/revenueleakageengine/internal/app/gateway/grpc"
	gatewayhttp "github.com/deplagene/revenueleakageengine/internal/app/gateway/http"
	httpmiddleware "github.com/deplagene/revenueleakageengine/internal/app/gateway/middleware"
	ingestionapp "github.com/deplagene/revenueleakageengine/internal/app/ingestion"
	"github.com/deplagene/revenueleakageengine/internal/app/outbox"
	appreconciliation "github.com/deplagene/revenueleakageengine/internal/app/reconciliation"
	"github.com/deplagene/revenueleakageengine/internal/migrator"
	platformgrpc "github.com/deplagene/revenueleakageengine/internal/platform/grpc"
	"github.com/deplagene/revenueleakageengine/internal/platform/sqlite"
	casework "github.com/deplagene/revenueleakageengine/internal/service/case"
	contractwork "github.com/deplagene/revenueleakageengine/internal/service/contract"
	reconciliationservice "github.com/deplagene/revenueleakageengine/internal/service/reconciliation"
	revenueservice "github.com/deplagene/revenueleakageengine/internal/service/revenue"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/theartofdevel/logging"
	googlegrpc "google.golang.org/grpc"
)

type eventSink interface {
	Append(ctx context.Context, envelope appevent.Envelope) error
	AppendBatch(ctx context.Context, envelopes []appevent.Envelope) error
}

// main is the process entrypoint.
func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "revenueleakageengine: %v\n", err)
		os.Exit(1)
	}
}

func run() (err error) {
	ctx := context.Background()
	cfg := config.Load()
	logger := newLogger(cfg.Logging)
	ctx = logging.ContextWithLogger(ctx, logger)

	db, err := sqlite.Open(ctx, sqlite.LocalDSN(cfg.SQLite.Path))
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close sqlite: %w", closeErr)
		}
	}()

	if err := runMigrations(ctx, db, cfg.Migrations.Path); err != nil {
		return err
	}

	contractQueries, contractCommands, err := buildContractUseCases(db)
	if err != nil {
		return err
	}

	outboxStore, err := outbox.NewSQLiteStore(db)
	if err != nil {
		return fmt.Errorf("build outbox store: %w", err)
	}

	ingestionCommands, ingestionQueries, err := buildIngestionUseCases(db, outboxStore)
	if err != nil {
		return err
	}

	reconciliationWorkflow, err := buildReconciliationWorkflow(db, contractQueries, ingestionQueries, outboxStore)
	if err != nil {
		return err
	}

	caseQueries, caseCommands, err := buildCaseUseCases(db)
	if err != nil {
		return err
	}

	httpHandler, err := gatewayhttp.NewHandler(
		reconciliationWorkflow,
		caseQueries,
		caseCommands,
		contractQueries,
		contractCommands,
		ingestionCommands,
	)
	if err != nil {
		return fmt.Errorf("build http handler: %w", err)
	}

	grpcHandler, err := gatewaygrpc.NewHandler(reconciliationWorkflow, caseQueries)
	if err != nil {
		return fmt.Errorf("build grpc handler: %w", err)
	}

	grpcServer := platformgrpc.NewServer()
	grpcHandler.Register(grpcServer)
	platformgrpc.RegisterHealthCheck(grpcServer)

	httpServer := newHTTPServer(cfg.HTTP, newRouter(logger, cfg.HTTP, httpHandler))
	httpErrCh := startHTTPServer(httpServer, logger)
	grpcErrCh, err := startGRPCServer(grpcServer, cfg.GRPC, logger)
	if err != nil {
		return err
	}

	return waitForShutdown(
		ctx,
		httpServer,
		grpcServer,
		cfg.HTTP.ShutdownTimeout,
		cfg.GRPC.ShutdownTimeout,
		httpErrCh,
		grpcErrCh,
		logger,
	)
}

func newLogger(cfg config.LoggingConfig) *logging.Logger {
	return logging.NewLogger(
		logging.WithLevel(cfg.Level),
		logging.WithIsJSON(cfg.IsJSON),
	)
}

func buildReconciliationWorkflow(
	db *sql.DB,
	contractQueries appreconciliation.ContractQueries,
	ingestionQueries appreconciliation.IngestionQueries,
	events eventSink,
) (*appreconciliation.RevenueLeakageWorkflow, error) {
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
		contractQueries,
		ingestionQueries,
		appreconciliation.WithEventSink(events),
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

func startGRPCServer(
	server *googlegrpc.Server,
	cfg config.GRPCConfig,
	logger *logging.Logger,
) (<-chan error, error) {
	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return nil, fmt.Errorf("listen grpc: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("grpc server starting", "addr", cfg.Addr)
		if err := server.Serve(listener); err != nil {
			errCh <- fmt.Errorf("serve grpc: %w", err)
			return
		}

		errCh <- nil
	}()

	return errCh, nil
}

func waitForShutdown(
	ctx context.Context,
	httpServer *http.Server,
	grpcServer *googlegrpc.Server,
	httpShutdownTimeout time.Duration,
	grpcShutdownTimeout time.Duration,
	httpErrCh <-chan error,
	grpcErrCh <-chan error,
	logger *logging.Logger,
) error {
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signalCh)

	select {
	case err := <-httpErrCh:
		return err
	case err := <-grpcErrCh:
		return err
	case signalValue := <-signalCh:
		logger.Info("shutdown signal received", "signal", signalValue.String())
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, httpShutdownTimeout)
	defer cancel()

	var shutdownErr error
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		shutdownErr = errors.Join(shutdownErr, fmt.Errorf("shutdown http server: %w", err))
	}

	if err := stopGRPCServer(grpcServer, grpcShutdownTimeout); err != nil {
		shutdownErr = errors.Join(shutdownErr, err)
	}

	if err := <-httpErrCh; err != nil {
		shutdownErr = errors.Join(shutdownErr, err)
	}

	if err := <-grpcErrCh; err != nil {
		shutdownErr = errors.Join(shutdownErr, err)
	}

	if shutdownErr != nil {
		return shutdownErr
	}

	logger.Info("servers stopped")
	return nil
}

func stopGRPCServer(server *googlegrpc.Server, timeout time.Duration) error {
	stopped := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(stopped)
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-stopped:
		return nil
	case <-timer.C:
		server.Stop()
		return errors.New("grpc graceful stop timed out")
	}
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

func buildIngestionUseCases(db *sql.DB, events eventSink) (*ingestionapp.Commands, *ingestionapp.Queries, error) {
	store := ingestionapp.NewSQLiteStore(db)
	commands, err := ingestionapp.NewCommands(store, ingestionapp.WithEventSink(events))
	if err != nil {
		return nil, nil, fmt.Errorf("build ingestion commands: %w", err)
	}

	queries, err := ingestionapp.NewQueries(store)
	if err != nil {
		return nil, nil, fmt.Errorf("build ingestion queries: %w", err)
	}

	return commands, queries, nil
}
