// Command core-domain runs the Core Domain Service HTTP server.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	appHTTP "github.com/motifpath/core-domain/internal/adapters/http"
	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/adapters/repo"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/application"
)

const (
	shutdownTimeout = 15 * time.Second
	migrationsDir   = "file://internal/adapters/repo/ent/migrate/migrations"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("service exited with error", "error", err)
		os.Exit(1)
	}
}

type config struct {
	port               string
	databaseURL        string
	mongoURI           string
	mongoDatabase      string
	clerkSecretKey     string
	corsAllowedOrigins []string
}

// defaultCORSOrigin is the local Vite dev server. Deployed environments override
// this via CORS_ALLOWED_ORIGINS.
const defaultCORSOrigin = "http://localhost:5173"

func loadConfig() (config, error) {
	databaseURL, err := mustGetenv("DATABASE_URL")
	if err != nil {
		return config{}, err
	}
	mongoURI, err := mustGetenv("MONGO_URI")
	if err != nil {
		return config{}, err
	}
	clerkSecretKey, err := mustGetenv("CLERK_SECRET_KEY")
	if err != nil {
		return config{}, err
	}
	return config{
		port:               getenvDefault("PORT", "8080"),
		databaseURL:        databaseURL,
		mongoURI:           mongoURI,
		mongoDatabase:      getenvDefault("MONGO_DATABASE", "motifpath_events"),
		clerkSecretKey:     clerkSecretKey,
		corsAllowedOrigins: appHTTP.ParseAllowedOrigins(getenvDefault("CORS_ALLOWED_ORIGINS", defaultCORSOrigin)),
	}, nil
}

func run(logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Applies pending migrations before anything else touches Postgres, per
	// ADR-005: `atlas migrate apply` acquires a Postgres advisory lock
	// first, so N pods starting simultaneously during a blue/green cutover
	// safely serialize on the same schema change rather than racing it.
	if err := applyMigrations(ctx, cfg.databaseURL); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	entClient, err := ent.Open("postgres", cfg.databaseURL)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer func() {
		if err := entClient.Close(); err != nil {
			logger.Error("failed to close postgres connection", "error", err)
		}
	}()

	mongoClient, err := mongo.Connect(options.Client().ApplyURI(cfg.mongoURI))
	if err != nil {
		return fmt.Errorf("connect to mongodb: %w", err)
	}
	defer func() {
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			logger.Error("failed to disconnect mongodb client", "error", err)
		}
	}()

	// JWKS fetching, in-memory caching, and refresh are handled internally
	// by the SDK from this point on — see ADR-007/ADR-009.
	clerk.SetKey(cfg.clerkSecretKey)

	userRepo := repo.NewEntUserRepository(entClient)
	nodeRepo := repo.NewEntContentNodeRepository(entClient)
	challengeRepo := repo.NewEntChallengeRepository(entClient)
	exerciseRepo := repo.NewEntExerciseRepository(entClient)
	expandedRepo := repo.NewEntExpandedContentRepository(entClient)
	pathRepo := repo.NewEntLearningPathRepository(entClient)
	assignmentRepo := repo.NewEntPathAssignmentRepository(entClient)
	completionReader := repo.NewMongoCompletionStateReader(mongoClient.Database(cfg.mongoDatabase))

	newID := uuid.NewString
	now := func() time.Time { return time.Now().UTC() }

	identityService := application.NewIdentityService(userRepo, newID, now)
	contentService := application.NewContentService(nodeRepo, expandedRepo, newID, now)
	challengeService := application.NewChallengeService(nodeRepo, challengeRepo, exerciseRepo, newID, now)
	pathService := application.NewLearningPathService(nodeRepo, pathRepo, newID, now)
	assignmentService := application.NewPathAssignmentService(userRepo, pathRepo, assignmentRepo, completionReader, newID, now)

	handler := appHTTP.NewHandler(identityService, contentService, challengeService, pathService, assignmentService)
	strictHandler := generated.NewStrictHandler(handler, nil)

	router := generated.HandlerWithOptions(strictHandler, generated.ChiServerOptions{
		Middlewares: []generated.MiddlewareFunc{appHTTP.ClerkAuthMiddleware},
	})

	srv := &http.Server{
		Addr:              ":" + cfg.port,
		Handler:           appHTTP.NewCORSMiddleware(cfg.corsAllowedOrigins)(router),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("core domain service listening", "port", cfg.port)
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

	logger.Info("core domain service stopped cleanly")
	return nil
}

// applyMigrations shells out to the Atlas CLI (bundled into the service
// image — see Dockerfile) rather than reimplementing its advisory-lock
// protocol in Go. This is the literal mechanism ADR-005 specifies: "the
// binary runs atlas migrate apply against the configured database before
// opening the HTTP listener."
func applyMigrations(ctx context.Context, databaseURL string) error {
	cmd := exec.CommandContext(ctx, "atlas", "migrate", "apply", //nolint:gosec // fixed args, databaseURL comes from our own env config, not user input
		"--dir", migrationsDir,
		"--url", databaseURL,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, output)
	}
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
