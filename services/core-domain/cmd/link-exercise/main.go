// Command link-exercise links an existing exercise to an existing content
// node as a path exercise, calling the same application service the HTTP
// handler uses (not raw SQL). One-off dev tool, not wired into any build —
// mirrors cmd/seed-dev-data's connection setup.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/lib/pq"

	"github.com/motifpath/core-domain/internal/adapters/repo"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
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
	if len(os.Args) != 4 {
		return fmt.Errorf("usage: link-exercise <node|challenge> <target-id> <exercise-id>")
	}
	kind, targetID, exerciseID := os.Args[1], os.Args[2], os.Args[3]
	if kind != "node" && kind != "challenge" {
		return fmt.Errorf("first argument must be \"node\" or \"challenge\", got %q", kind)
	}

	ctx := context.Background()
	databaseURL := getenvDefault("DATABASE_URL", "postgres://motifpath:motifpath@localhost:5432/core_domain?sslmode=disable")

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
	defer func() {
		if closeErr := entClient.Close(); closeErr != nil {
			log.Printf("failed to close ent client: %v", closeErr)
		}
	}()

	nodeRepo := repo.NewEntContentNodeRepository(entClient)
	challengeRepo := repo.NewEntChallengeRepository(entClient)
	exerciseRepo := repo.NewEntExerciseRepository(entClient)
	exerciseService := application.NewExerciseService(challengeRepo, exerciseRepo, nodeRepo, nil, nil, nil, nil, rand.Shuffle)

	teacher := domain.User{ID: "dev-tool", Role: domain.RoleTeacher}

	if kind == "node" {
		exercise, err := exerciseService.LinkExerciseToContentNode(ctx, teacher, targetID, exerciseID)
		if err != nil {
			return fmt.Errorf("link exercise to content node: %w", err)
		}
		log.Printf("linked exercise %s to content node %s (now linked to nodes: %v)", exercise.ID, targetID, exercise.ContentNodeIDs)
		return nil
	}

	exercise, err := exerciseService.LinkExerciseToChallenge(ctx, teacher, targetID, exerciseID)
	if err != nil {
		return fmt.Errorf("link exercise to challenge: %w", err)
	}
	log.Printf("linked exercise %s to challenge %s (now linked to challenges: %v)", exercise.ID, targetID, exercise.ChallengeIDs)
	return nil
}
