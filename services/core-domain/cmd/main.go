// Command core-domain runs the Core Domain Service HTTP server.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	mathrand "math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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

	mediaS3Bucket      string
	mediaS3Region      string
	mediaS3Endpoint    string // empty = real AWS S3; set = MinIO or another S3-compatible endpoint
	mediaS3AccessKeyID string // only used when mediaS3Endpoint is set
	mediaS3SecretKey   string // only used when mediaS3Endpoint is set
	mediaPublicBaseURL string
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
	mediaS3Bucket, err := mustGetenv("MEDIA_S3_BUCKET")
	if err != nil {
		return config{}, err
	}
	mediaPublicBaseURL, err := mustGetenv("MEDIA_PUBLIC_BASE_URL")
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

		mediaS3Bucket:      mediaS3Bucket,
		mediaS3Region:      getenvDefault("MEDIA_S3_REGION", "us-east-1"),
		mediaS3Endpoint:    os.Getenv("MEDIA_S3_ENDPOINT"),
		mediaS3AccessKeyID: os.Getenv("MEDIA_S3_ACCESS_KEY_ID"),
		mediaS3SecretKey:   os.Getenv("MEDIA_S3_SECRET_ACCESS_KEY"),
		mediaPublicBaseURL: mediaPublicBaseURL,
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

	sqlDB, entClient, mongoClient, closeStores, err := connectStores(cfg, logger)
	if err != nil {
		return err
	}
	defer closeStores()

	// JWKS fetching, in-memory caching, and refresh are handled internally
	// by the SDK from this point on — see ADR-007/ADR-009.
	clerk.SetKey(cfg.clerkSecretKey)

	handler, err := buildHandler(ctx, cfg, entClient, sqlDB, mongoClient)
	if err != nil {
		return err
	}
	strictHandler := generated.NewStrictHandler(handler, nil)

	router := generated.HandlerWithOptions(strictHandler, generated.ChiServerOptions{
		Middlewares: []generated.MiddlewareFunc{appHTTP.ClerkAuthMiddleware, appHTTP.AcceptLanguageMiddleware},
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

// connectStores opens Postgres (via *sql.DB, so the readiness probe can
// ping the same pool ent queries through — see PostgresPinger) and MongoDB,
// returning a single cleanup func that closes both, logging any close
// failure rather than returning it (there's nothing left to do differently
// at shutdown time).
func connectStores(cfg config, logger *slog.Logger) (*sql.DB, *ent.Client, *mongo.Client, func(), error) {
	sqlDB, err := sql.Open("postgres", cfg.databaseURL)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("connect to postgres: %w", err)
	}
	entClient := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, sqlDB)))

	mongoClient, err := mongo.Connect(options.Client().ApplyURI(cfg.mongoURI))
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("connect to mongodb: %w", err)
	}

	cleanup := func() {
		if err := entClient.Close(); err != nil {
			logger.Error("failed to close postgres connection", "error", err)
		}
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			logger.Error("failed to disconnect mongodb client", "error", err)
		}
	}
	return sqlDB, entClient, mongoClient, cleanup, nil
}

// buildHandler wires every repository and application service into the
// HTTP handler. Split out of run so the resource-lifecycle concerns there
// (connect, defer-close, apply migrations, start the server) stay separate
// from dependency wiring.
func buildHandler(ctx context.Context, cfg config, entClient *ent.Client, sqlDB *sql.DB, mongoClient *mongo.Client) (*appHTTP.Handler, error) {
	s3Client, err := newS3Client(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("configure media storage client: %w", err)
	}
	mediaStorage := repo.NewS3MediaStorage(s3Client, cfg.mediaS3Bucket, cfg.mediaPublicBaseURL)

	userRepo := repo.NewEntUserRepository(entClient)
	languageRepo := repo.NewEntLanguageRepository(entClient)
	nodeRepo := repo.NewEntContentNodeRepository(entClient)
	challengeRepo := repo.NewEntChallengeRepository(entClient)
	exerciseRepo := repo.NewEntExerciseRepository(entClient)
	expandedRepo := repo.NewEntExpandedContentRepository(entClient)
	pathRepo := repo.NewEntLearningPathRepository(entClient)
	assignmentRepo := repo.NewEntPathAssignmentRepository(entClient)
	completionReader := repo.NewMongoCompletionStateReader(mongoClient.Database(cfg.mongoDatabase))
	learningGraphPinger := repo.NewPostgresPinger(sqlDB)

	newID := uuid.NewString
	now := func() time.Time { return time.Now().UTC() }

	identityService := application.NewIdentityService(userRepo, languageRepo, newID, now)
	contentService := application.NewContentService(nodeRepo, expandedRepo, newID, now)
	challengeService := application.NewChallengeService(nodeRepo, challengeRepo, newID, now)
	exerciseService := application.NewExerciseService(challengeRepo, exerciseRepo, nodeRepo, newID, now, mathrand.Shuffle)
	mediaService := application.NewMediaService(exerciseRepo, mediaStorage, newID)
	pathService := application.NewLearningPathService(nodeRepo, pathRepo, newID, now)
	assignmentService := application.NewPathAssignmentService(userRepo, pathRepo, assignmentRepo, nodeRepo, exerciseRepo, completionReader, newID, now)

	return appHTTP.NewHandler(identityService, contentService, challengeService, exerciseService, mediaService, pathService, assignmentService,
		learningGraphPinger, completionReader), nil
}

// newS3Client builds the client MediaService's presigned uploads go
// through. With cfg.mediaS3Endpoint unset, it uses the AWS SDK's default
// credential chain and endpoint resolution — real S3 in production. With it
// set (local dev), it points at MinIO instead: a fixed endpoint, static
// credentials, and path-style addressing (MinIO doesn't support the
// virtual-hosted-style bucket subdomains real S3 uses). The upload/presign
// code path itself never branches on environment — only this construction
// does.
func newS3Client(ctx context.Context, cfg config) (*s3.Client, error) {
	if cfg.mediaS3Endpoint == "" {
		awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.mediaS3Region))
		if err != nil {
			return nil, err
		}
		return s3.NewFromConfig(awsCfg), nil
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.mediaS3Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.mediaS3AccessKeyID, cfg.mediaS3SecretKey, "")),
	)
	if err != nil {
		return nil, err
	}
	return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.mediaS3Endpoint)
		o.UsePathStyle = true
	}), nil
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
