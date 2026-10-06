// Command sync-practice-reference rebuilds the practice reference snapshot
// in MongoDB from PostgreSQL (ADR-047) — the same sync the service runs on
// start, for when the snapshot needs repairing without a restart.
// Mirrors cmd/link-exercise's connection setup.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/lib/pq"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/motifpath/core-domain/internal/adapters/repo"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/application"
)

func getenvDefault(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()
	databaseURL := getenvDefault("DATABASE_URL", "postgres://motifpath:motifpath@localhost:5432/core_domain?sslmode=disable")
	mongoURI := getenvDefault("MONGO_URI", "mongodb://motifpath:motifpath@localhost:27017/?authSource=admin")
	mongoDatabase := getenvDefault("MONGO_DATABASE", "motifpath_events")

	sqlDB, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer func() {
		if closeErr := sqlDB.Close(); closeErr != nil {
			log.Printf("failed to close postgres connection: %v", closeErr)
		}
	}()
	entClient := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, sqlDB)))

	mongoClient, err := mongo.Connect(options.Client().ApplyURI(mongoURI))
	if err != nil {
		return fmt.Errorf("connect to mongodb: %w", err)
	}
	defer func() {
		if closeErr := mongoClient.Disconnect(ctx); closeErr != nil {
			log.Printf("failed to disconnect mongodb client: %v", closeErr)
		}
	}()

	writer := repo.NewMongoPracticeReferenceWriter(mongoClient.Database(mongoDatabase), func() time.Time { return time.Now().UTC() })
	if err := writer.EnsureIndexes(ctx); err != nil {
		return fmt.Errorf("ensure the practice reference indexes: %w", err)
	}
	sync := application.NewPracticeReferenceService(repo.NewEntDiagramRepository(entClient), repo.NewEntExerciseRepository(entClient), repo.NewEntInstrumentRepository(entClient), repo.NewEntDrillThresholdRepository(entClient), writer)
	synced, err := sync.Sync(ctx)
	if err != nil {
		return fmt.Errorf("sync the snapshot (%d diagrams, %d exercises, %d instruments, %d drill thresholds written): %w",
			synced.Diagrams, synced.Exercises, synced.Instruments, synced.DrillThresholds, err)
	}
	log.Printf("practice reference snapshot synced: %d diagrams, %d exercises, %d instruments, %d drill thresholds",
		synced.Diagrams, synced.Exercises, synced.Instruments, synced.DrillThresholds)
	return nil
}
