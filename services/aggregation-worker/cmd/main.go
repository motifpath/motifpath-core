// Command aggregation-worker runs the minimal Aggregation Worker described in
// ADR-011: a Kafka consumer that derives per-student, per-content-node
// completion status from lesson-family tracking events, and grades practice
// answers into evidence and per-item knowledge state.
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

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/motifpath/aggregation-worker/internal/adapters/health"
	"github.com/motifpath/aggregation-worker/internal/adapters/kafka"
	"github.com/motifpath/aggregation-worker/internal/adapters/repo"
	"github.com/motifpath/aggregation-worker/internal/application"
)

const healthShutdownTimeout = 5 * time.Second

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("service exited with error", "error", err)
		os.Exit(1)
	}
}

type config struct {
	port          string
	mongoURI      string
	mongoDatabase string
	kafkaBrokers  []string
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
	return config{
		// PORT with an 8082 default, matching core-domain (8080) and
		// event-ingestion (8081); here it serves only the health probes.
		port:          getenvDefault("PORT", "8082"),
		mongoURI:      mongoURI,
		mongoDatabase: getenvDefault("MONGO_DATABASE", "motifpath_events"),
		kafkaBrokers:  strings.Split(kafkaBrokersRaw, ","),
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

	completionRepo, service, err := newEventService(ctx, mongoClient.Database(cfg.mongoDatabase), logger)
	if err != nil {
		return err
	}
	consumer := kafka.NewKafkaEventConsumer(cfg.kafkaBrokers, service, logger)
	defer func() {
		if err := consumer.Close(); err != nil {
			logger.Error("failed to close kafka reader", "error", err)
		}
	}()

	// The health server runs alongside the consumer loop for the whole
	// lifetime of run: consumer.Run blocks until shutdown or an unrecoverable
	// error, and only then does this Shutdown fire — so /healthz stays
	// answerable throughout, including after a consumer-loop failure.
	healthSrv := health.NewServer(":"+cfg.port, []health.Check{
		{Name: "mongodb", Pinger: completionRepo},
		{Name: "kafka_broker", Pinger: consumer},
	}, logger)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), healthShutdownTimeout)
		defer cancel()
		if err := healthSrv.Shutdown(shutdownCtx); err != nil {
			logger.Error("failed to shut down health server", "error", err)
		}
	}()
	go func() {
		logger.Info("health server listening", "port", cfg.port)
		if err := healthSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("health server failed", "error", err)
		}
	}()

	logger.Info("aggregation worker consuming motifpath.events")
	// Run returns nil on a clean shutdown (ctx cancelled by the signal above)
	// and each message is committed synchronously right after a successful
	// Handle call, so there is no in-flight batch left to flush here.
	if err := consumer.Run(ctx); err != nil {
		return fmt.Errorf("consumer stopped with error: %w", err)
	}

	logger.Info("aggregation worker stopped cleanly")
	return nil
}

// newEventService builds the event handler over its MongoDB repositories,
// creating their indexes first.
func newEventService(ctx context.Context, db *mongo.Database, logger *slog.Logger) (*repo.MongoCompletionStateRepository, *application.ProcessEventService, error) {
	completionRepo := repo.NewMongoCompletionStateRepository(db)
	// Fatal on failure: the unique (student_id, content_node_id) index is what
	// keeps this collection to exactly one document per pair.
	if err := completionRepo.EnsureIndexes(ctx); err != nil {
		return nil, nil, fmt.Errorf("ensure mongodb indexes: %w", err)
	}

	evidenceRepo := repo.NewMongoPracticeEvidenceRepository(db)
	// Fatal on failure: the unique evidence_id index is what makes a redelivered
	// practice answer count once.
	if err := evidenceRepo.EnsureIndexes(ctx); err != nil {
		return nil, nil, fmt.Errorf("ensure practice evidence indexes: %w", err)
	}
	itemStateRepo := repo.NewMongoPracticeItemStateRepository(db)
	if err := itemStateRepo.EnsureIndexes(ctx); err != nil {
		return nil, nil, fmt.Errorf("ensure practice item state indexes: %w", err)
	}
	historyRepo := repo.NewMongoPracticeItemHistoryRepository(db)
	// Fatal on failure: the unique (student_id, item_key, day) index is what keeps
	// one snapshot per item and day.
	if err := historyRepo.EnsureIndexes(ctx); err != nil {
		return nil, nil, fmt.Errorf("ensure practice item history indexes: %w", err)
	}
	practice := application.NewPracticeEvidenceService(
		repo.NewMongoPracticeReferenceReader(db), evidenceRepo, itemStateRepo, historyRepo, logger,
	)

	sessionRepo := repo.NewMongoPracticeSessionRepository(db)
	if err := sessionRepo.EnsureIndexes(ctx); err != nil {
		return nil, nil, fmt.Errorf("ensure practice session indexes: %w", err)
	}
	learningRepo := repo.NewMongoLearningActivityRepository(db)
	// Fatal on failure: the unique event_id index is what keeps a redelivered
	// completion once.
	if err := learningRepo.EnsureIndexes(ctx); err != nil {
		return nil, nil, fmt.Errorf("ensure learning activity indexes: %w", err)
	}
	tapCheckRepo := repo.NewMongoTapCheckRepository(db)
	// Fatal on failure: the unique event_id index is what keeps a redelivered
	// tap check once.
	if err := tapCheckRepo.EnsureIndexes(ctx); err != nil {
		return nil, nil, fmt.Errorf("ensure tap check indexes: %w", err)
	}

	service := application.NewProcessEventService(completionRepo, practice,
		application.NewActivityService(sessionRepo, learningRepo, tapCheckRepo))
	return completionRepo, service, nil
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
