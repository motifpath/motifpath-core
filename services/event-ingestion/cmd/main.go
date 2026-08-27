// Command event-ingestion runs the Event Ingestion Service HTTP server.
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

	"github.com/clerk/clerk-sdk-go/v2"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/motifpath/event-ingestion/internal/adapters/coredomain"
	appHTTP "github.com/motifpath/event-ingestion/internal/adapters/http"
	"github.com/motifpath/event-ingestion/internal/adapters/http/generated"
	"github.com/motifpath/event-ingestion/internal/adapters/kafka"
	"github.com/motifpath/event-ingestion/internal/adapters/repo"
	"github.com/motifpath/event-ingestion/internal/application"
)

const shutdownTimeout = 15 * time.Second

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("service exited with error", "error", err)
		os.Exit(1)
	}
}

type config struct {
	port              string
	mongoURI          string
	mongoDatabase     string
	kafkaBrokers      []string
	clerkSecretKey    string
	coreDomainBaseURL string
}

func loadConfig() (config, error) {
	mongoURI, err := mustGetenv("MONGO_URI")
	if err != nil {
		return config{}, err
	}
	kafkaBrokersRaw, err := mustGetenv("KAFKA_BROKERS")
	if err != nil {
		return config{}, err
	}
	clerkSecretKey, err := mustGetenv("CLERK_SECRET_KEY")
	if err != nil {
		return config{}, err
	}
	// Required: the outbox admin endpoints resolve the caller's role by calling
	// the Core Domain Service (ADR-013). Absent this, the service fails to start
	// rather than silently falling back to trusting a JWT claim.
	coreDomainBaseURL, err := mustGetenv("CORE_DOMAIN_BASE_URL")
	if err != nil {
		return config{}, err
	}
	return config{
		port:              getenvDefault("PORT", "8081"),
		mongoURI:          mongoURI,
		mongoDatabase:     getenvDefault("MONGO_DATABASE", "motifpath_events"),
		kafkaBrokers:      strings.Split(kafkaBrokersRaw, ","),
		clerkSecretKey:    clerkSecretKey,
		coreDomainBaseURL: coreDomainBaseURL,
	}, nil
}

func run(logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	mongoClient, err := mongo.Connect(options.Client().ApplyURI(cfg.mongoURI))
	if err != nil {
		return fmt.Errorf("connect to mongodb: %w", err)
	}
	defer func() {
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			logger.Error("failed to disconnect mongodb client", "error", err)
		}
	}()

	eventRepo := repo.NewMongoEventRepository(mongoClient.Database(cfg.mongoDatabase))
	// Fatal on failure: the unique event_id index is what makes Save idempotent.
	// Starting without it risks silently accepting duplicate event documents.
	if err := eventRepo.EnsureIndexes(ctx); err != nil {
		return fmt.Errorf("ensure mongodb indexes: %w", err)
	}

	outboxRepo := repo.NewMongoPublishOutboxRepository(mongoClient.Database(cfg.mongoDatabase))
	if err := outboxRepo.EnsureIndexes(ctx); err != nil {
		return fmt.Errorf("ensure mongodb indexes: %w", err)
	}

	publisher := kafka.NewKafkaEventPublisher(cfg.kafkaBrokers)
	defer func() {
		if err := publisher.Close(); err != nil {
			logger.Error("failed to close kafka writer", "error", err)
		}
	}()

	// JWKS fetching, in-memory caching, and refresh are handled internally by the
	// SDK from this point on — see ADR-007/ADR-009. No CLERK_JWKS_URL is needed:
	// despite ADR-007 naming that variable, the current SDK only needs the secret
	// key and resolves JWKS via Clerk's backend API itself.
	clerk.SetKey(cfg.clerkSecretKey)

	roleResolver := coredomain.NewRoleResolver(cfg.coreDomainBaseURL, nil)
	adminAuthorizer := application.NewAdminAuthorizer(roleResolver)

	service := application.NewIngestEventService(eventRepo, outboxRepo, publisher, logger)
	adminOutbox := application.NewAdminOutboxService(outboxRepo, eventRepo, publisher, adminAuthorizer)
	handler := appHTTP.NewHandler(service, adminOutbox, eventRepo, publisher)
	strictHandler := generated.NewStrictHandler(handler, nil)

	router := generated.HandlerWithOptions(strictHandler, generated.ChiServerOptions{
		Middlewares: []generated.MiddlewareFunc{appHTTP.ClerkAuthMiddleware},
	})

	// Retries publish_outbox entries a prior attempt failed to deliver, per
	// ADR-012. Runs for the lifetime of the process; Run returns on its own
	// once ctx is cancelled by the shutdown signal below.
	retrySweep := application.NewRetrySweepService(outboxRepo, eventRepo, publisher, logger)
	go retrySweep.Run(ctx)

	srv := &http.Server{
		Addr:              ":" + cfg.port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("event ingestion service listening", "port", cfg.port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-serverErr:
		if err != nil {
			return fmt.Errorf("server failed: %w", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	logger.Info("event ingestion service stopped cleanly")
	return nil
}

func getenvDefault(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func mustGetenv(name string) (string, error) {
	v := os.Getenv(name)
	if v == "" {
		return "", fmt.Errorf("required environment variable %s is not set", name)
	}
	return v, nil
}
