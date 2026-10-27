// Command engagement-mailer sends the platform's engagement emails: the
// weekly progress report and the daily and inactivity reminders to
// students, and the weekly student summary to the concierge. It runs as its
// own container beside core-domain, from the same image, so it can be
// stopped without touching the API. It never applies migrations;
// core-domain does.
//
// Flags let an operator check the emails without waiting for their moment:
// --once runs a single pass and exits, --at runs that pass as if it were
// the given moment (an RFC 3339 time, such as 2026-11-02T09:00:00-03:00),
// and --to limits it to one address's emails, with no concierge summary.
// Each pass follows the usual rules and records what it sends, so an email
// already sent for its period isn't sent again.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/lib/pq"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/motifpath/core-domain/internal/adapters/email"
	"github.com/motifpath/core-domain/internal/adapters/repo"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/ports"
)

// runTimeout bounds one pass over every student, so a stuck store can't
// keep a pass running into the next one.
const runTimeout = 10 * time.Minute

type config struct {
	port           string
	interval       time.Duration
	databaseURL    string
	mongoURI       string
	mongoDatabase  string
	unsubscribeKey string
	smtp           email.SMTPConfig
	mailer         application.EngagementMailerConfig
}

// runFlags are the operator's flags; none of them is set in a deployment.
type runFlags struct {
	once bool
	at   time.Time
	to   string
}

func parseRunFlags(args []string) (runFlags, error) {
	fs := flag.NewFlagSet("engagement-mailer", flag.ContinueOnError)
	var flags runFlags
	var at string
	fs.BoolVar(&flags.once, "once", false, "run one pass and exit")
	fs.StringVar(&at, "at", "", "with --once, run the pass as if it were this RFC 3339 time, such as 2026-11-02T09:00:00-03:00")
	fs.StringVar(&flags.to, "to", "", "send only the emails due to this address, and no concierge summary")
	if err := fs.Parse(args); err != nil {
		return runFlags{}, err
	}
	if at == "" {
		return flags, nil
	}
	if !flags.once {
		return runFlags{}, errors.New("--at needs --once: a pass that keeps running can't stay at one moment")
	}
	parsed, err := time.Parse(time.RFC3339, at)
	if err != nil {
		return runFlags{}, fmt.Errorf("--at must be an RFC 3339 time with its offset, such as 2026-11-02T09:00:00-03:00: %w", err)
	}
	flags.at = parsed
	return flags, nil
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	flags, err := parseRunFlags(os.Args[1:])
	if err != nil {
		logger.Error("engagement mailer flags", "error", err)
		os.Exit(2)
	}
	if err := run(logger, flags); err != nil {
		logger.Error("engagement mailer stopped", "error", err)
		os.Exit(1)
	}
}

func loadConfig() (config, error) {
	required := map[string]string{}
	for _, name := range []string{"DATABASE_URL", "MONGO_URI", "EMAIL_UNSUBSCRIBE_SECRET", "SMTP_HOST", "MAIL_FROM_ADDRESS", "APP_BASE_URL", "API_BASE_URL"} {
		value := os.Getenv(name)
		if value == "" {
			return config{}, fmt.Errorf("required environment variable %s is not set", name)
		}
		required[name] = value
	}
	interval, err := time.ParseDuration(getenvDefault("MAILER_INTERVAL", "15m"))
	if err != nil || interval <= 0 {
		return config{}, fmt.Errorf("MAILER_INTERVAL must be a positive duration such as 15m")
	}
	return config{
		// Not PORT: locally the mailer shares core-domain's .env, whose PORT
		// is the API's.
		port:           getenvDefault("MAILER_PORT", "8083"),
		interval:       interval,
		databaseURL:    required["DATABASE_URL"],
		mongoURI:       required["MONGO_URI"],
		mongoDatabase:  getenvDefault("MONGO_DATABASE", "motifpath_events"),
		unsubscribeKey: required["EMAIL_UNSUBSCRIBE_SECRET"],
		smtp: email.SMTPConfig{
			Host:     required["SMTP_HOST"],
			Port:     os.Getenv("SMTP_PORT"),
			Username: os.Getenv("SMTP_USERNAME"),
			Password: os.Getenv("SMTP_PASSWORD"),
			From:     required["MAIL_FROM_ADDRESS"],
		},
		mailer: application.EngagementMailerConfig{
			AppBaseURL:       required["APP_BASE_URL"],
			APIBaseURL:       required["API_BASE_URL"],
			ConciergeAddress: os.Getenv("CONCIERGE_EMAIL"),
		},
	}, nil
}

func run(logger *slog.Logger, flags runFlags) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.mailer.OnlyRecipient = flags.to
	sqlDB, err := sql.Open("postgres", cfg.databaseURL)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	entClient := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, sqlDB)))
	defer closeOrLog(logger, "postgres", entClient.Close)
	mongoClient, err := mongo.Connect(options.Client().ApplyURI(cfg.mongoURI))
	if err != nil {
		return fmt.Errorf("connect to mongodb: %w", err)
	}
	defer closeOrLog(logger, "mongodb", func() error { return mongoClient.Disconnect(context.Background()) })

	mailer, err := buildMailer(cfg, entClient, mongoClient.Database(cfg.mongoDatabase), clockFor(flags))
	if err != nil {
		return err
	}
	if flags.once {
		logger.Info("engagement mailer single pass", "at", clockFor(flags)().Format(time.RFC3339), "to", flags.to)
		passCtx, cancel := context.WithTimeout(ctx, runTimeout)
		defer cancel()
		return mailer.Run(passCtx)
	}
	server := healthServer(cfg.port, repo.NewPostgresPinger(sqlDB), repo.NewMongoCompletionStateReader(mongoClient.Database(cfg.mongoDatabase)))
	serverErr := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	logger.Info("engagement mailer started", "interval", cfg.interval.String(), "concierge", cfg.mailer.ConciergeAddress != "")
	runOnce(ctx, logger, mailer)
	ticker := time.NewTicker(cfg.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			return server.Shutdown(shutdownCtx)
		case err := <-serverErr:
			return fmt.Errorf("health server failed: %w", err)
		case <-ticker.C:
			// Each pass decides from the clock alone what is due, so a pass
			// that comes late or a tick that was dropped never replays.
			runOnce(ctx, logger, mailer)
		}
	}
}

func runOnce(ctx context.Context, logger *slog.Logger, mailer *application.EngagementMailerService) {
	ctx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()
	if err := mailer.Run(ctx); err != nil {
		logger.Error("engagement mailer pass failed", "error", err)
	}
}

// clockFor is the time a pass runs at: the --at moment when one is given,
// the real time otherwise.
func clockFor(flags runFlags) func() time.Time {
	if !flags.at.IsZero() {
		at := flags.at.UTC()
		return func() time.Time { return at }
	}
	return func() time.Time { return time.Now().UTC() }
}

func buildMailer(cfg config, entClient *ent.Client, db *mongo.Database, now func() time.Time) (*application.EngagementMailerService, error) {
	composer, err := email.NewComposer()
	if err != nil {
		return nil, fmt.Errorf("parse email templates: %w", err)
	}
	userRepo := repo.NewEntUserRepository(entClient)
	studentPathRepo := repo.NewEntStudentPathRepository(entClient)
	enrollmentRepo := repo.NewEntCourseEnrollmentRepository(entClient)
	instrumentRepo := repo.NewEntInstrumentRepository(entClient)
	rollup := application.NewKnowledgeRollupService(repo.NewEntKnowledgeNodeRepository(entClient), repo.NewEntKnowledgeEdgeRepository(entClient),
		repo.NewEntNodeItemSource(entClient), repo.NewMongoPracticeItemStateReader(db), now)
	summary := application.NewPracticeSummaryService(instrumentRepo, studentPathRepo, enrollmentRepo, repo.NewEntLearningPathRepository(entClient),
		repo.NewEntCourseVersionRepository(entClient), repo.NewEntContentNodeRepository(entClient), rollup,
		repo.NewMongoPracticeActivityReader(db), repo.NewEntSongChartRepository(entClient), now)

	return application.NewEngagementMailerService(application.EngagementMailerDeps{
		Users:       userRepo,
		Enrollments: enrollmentRepo,
		Paths:       studentPathRepo,
		Activity:    repo.NewMongoAlphaActivityReader(db),
		Figures:     summary,
		Sends:       repo.NewEntEmailSendLog(entClient),
		Composer:    composer,
		Sender:      email.NewSMTPSender(cfg.smtp, now),
		Tokens:      application.NewEmailPreferencesService(userRepo, []byte(cfg.unsubscribeKey)),
	}, cfg.mailer, now), nil
}

// healthServer answers /healthz while the process runs and /readyz while
// both stores answer, the probes every service's deploy waits on.
func healthServer(port string, stores ...ports.Pinger) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeStatus(w, http.StatusOK, "ok")
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		for _, store := range stores {
			if err := store.Ping(ctx); err != nil {
				writeStatus(w, http.StatusServiceUnavailable, "unavailable")
				return
			}
		}
		writeStatus(w, http.StatusOK, "ok")
	})
	return &http.Server{Addr: ":" + port, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
}

func writeStatus(w http.ResponseWriter, code int, status string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": status}); err != nil {
		slog.Default().Error("write health response", "error", err)
	}
}

func closeOrLog(logger *slog.Logger, name string, closeFn func() error) {
	if err := closeFn(); err != nil {
		logger.Error("close failed", "store", name, "error", err)
	}
}

func getenvDefault(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}
