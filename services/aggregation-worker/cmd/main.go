// Command aggregation-worker runs the minimal Aggregation Worker described in
// ADR-011: a Kafka consumer that derives per-student, per-content-node
// completion status from lesson-family tracking events.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/motifpath/aggregation-worker/internal/adapters/kafka"
	"github.com/motifpath/aggregation-worker/internal/adapters/repo"
	"github.com/motifpath/aggregation-worker/internal/application"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("service exited with error", "error", err)
		os.Exit(1)
	}
}

type config struct {
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

	completionRepo := repo.NewMongoCompletionStateRepository(mongoClient.Database(cfg.mongoDatabase))
	// Fatal on failure: the unique (student_id, content_node_id) index is what
	// keeps this collection to exactly one document per pair.
	if err := completionRepo.EnsureIndexes(ctx); err != nil {
		return fmt.Errorf("ensure mongodb indexes: %w", err)
	}

	service := application.NewProcessEventService(completionRepo)
	consumer := kafka.NewKafkaEventConsumer(cfg.kafkaBrokers, service, logger)
	defer func() {
		if err := consumer.Close(); err != nil {
			logger.Error("failed to close kafka reader", "error", err)
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
