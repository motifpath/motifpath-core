//go:build integration

package repo

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// One Postgres and one MongoDB container serve this whole package, started
// once by TestMain.
//
// Every test here used to start and terminate its own container — eight
// Postgres and one Mongo — which cost seconds each and gave the
// container-readiness race one chance to bite per test rather than one per
// package. What that bought was a pristine database per test, and that is
// cheaper as an actual database per test (see newPostgresDSN and
// mongoDatabase) than as a container per test.
var (
	sharedPostgresDSN  string
	sharedMongoClient  *mongo.Client
	postgresDatabaseNo atomic.Int64
	mongoDatabaseNo    atomic.Int64
)

func TestMain(m *testing.M) {
	os.Exit(runWithSharedContainers(m))
}

// runWithSharedContainers is separate from TestMain so the deferred cleanups
// actually run — os.Exit inside TestMain would skip them and leak containers.
func runWithSharedContainers(m *testing.M) int {
	ctx := context.Background()

	pgContainer, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("core_domain_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
	)
	if err != nil {
		log.Printf("failed starting shared postgres container: %v", err)
		return 1
	}
	defer func() {
		if err := testcontainers.TerminateContainer(pgContainer); err != nil {
			log.Printf("failed terminating shared postgres container: %v", err)
		}
	}()

	sharedPostgresDSN, err = pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Printf("failed resolving shared postgres connection string: %v", err)
		return 1
	}
	if err := waitForPostgres(ctx, sharedPostgresDSN); err != nil {
		log.Printf("shared postgres container never became reachable: %v", err)
		return 1
	}

	mongoContainer, err := mongodb.Run(ctx, "mongo:7")
	if err != nil {
		log.Printf("failed starting shared mongo container: %v", err)
		return 1
	}
	defer func() {
		if err := testcontainers.TerminateContainer(mongoContainer); err != nil {
			log.Printf("failed terminating shared mongo container: %v", err)
		}
	}()

	mongoConnStr, err := mongoContainer.ConnectionString(ctx)
	if err != nil {
		log.Printf("failed resolving shared mongo connection string: %v", err)
		return 1
	}
	client, err := mongo.Connect(options.Client().ApplyURI(mongoConnStr))
	if err != nil {
		log.Printf("failed connecting to shared mongo container: %v", err)
		return 1
	}
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			log.Printf("failed disconnecting from shared mongo container: %v", err)
		}
	}()
	if err := pingMongo(ctx, client); err != nil {
		log.Printf("shared mongo container never became reachable: %v", err)
		return 1
	}
	sharedMongoClient = client

	return m.Run()
}

// waitForPostgres absorbs the race the module's log-based wait strategy leaves
// open: it reports ready on a log line that can precede Postgres actually
// accepting connections, and sql.Open is lazy, so without this the first query
// eats a connection reset.
func waitForPostgres(ctx context.Context, dsn string) error {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	for attempt := 0; ; attempt++ {
		err = db.PingContext(ctx)
		if err == nil || attempt >= 9 {
			return err
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func pingMongo(ctx context.Context, client *mongo.Client) error {
	var err error
	for attempt := 0; ; attempt++ {
		err = client.Ping(ctx, nil)
		if err == nil || attempt >= 9 {
			return err
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// newPostgresDSN creates a fresh, empty database on the shared container and
// returns a DSN pointing at it, giving the caller the same clean slate a
// dedicated container used to.
func newPostgresDSN(t *testing.T) string {
	t.Helper()
	require.NotEmpty(t, sharedPostgresDSN, "TestMain did not start the shared postgres container")

	// Generated, not caller-supplied — safe to interpolate, and CREATE
	// DATABASE cannot be parameterised anyway.
	name := fmt.Sprintf("test_%d", postgresDatabaseNo.Add(1))

	admin, err := sql.Open("postgres", sharedPostgresDSN)
	require.NoError(t, err)
	defer func() { require.NoError(t, admin.Close()) }()

	_, err = admin.ExecContext(context.Background(), "CREATE DATABASE "+name)
	require.NoError(t, err)

	parsed, err := url.Parse(sharedPostgresDSN)
	require.NoError(t, err)
	parsed.Path = "/" + name
	return parsed.String()
}

// mongoDatabase returns a database unique to the calling test, so tests
// sharing the package's one container cannot observe each other's documents.
func mongoDatabase(t *testing.T) *mongo.Database {
	t.Helper()
	require.NotNil(t, sharedMongoClient, "TestMain did not start the shared mongo container")

	// Mongo database names reject / \ . " $ * < > : | ? and cap at 63 bytes.
	name := strings.NewReplacer(
		"/", "_", "\\", "_", ".", "_", "\"", "_", "$", "_",
		"*", "_", "<", "_", ">", "_", ":", "_", "|", "_", "?", "_", " ", "_",
	).Replace(t.Name())
	if len(name) > 40 {
		name = name[:40]
	}
	return sharedMongoClient.Database(fmt.Sprintf("%s_%d", name, mongoDatabaseNo.Add(1)))
}
