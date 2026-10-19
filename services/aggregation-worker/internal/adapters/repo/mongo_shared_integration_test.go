//go:build integration

package repo

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// sharedMongoClient talks to a single mongo container started once for this
// package by TestMain.
//
// Every test here used to start and terminate its own container — five of
// them — which cost about a second each and gave the container-readiness race
// five chances to bite instead of one. What that bought was isolation, and
// isolation is cheaper as a database per test (see mongoDatabase) than as a
// container per test.
var sharedMongoClient *mongo.Client

// mongoDatabaseSeq disambiguates database names, so two tests whose names
// collide after sanitising and truncating still get separate databases.
var mongoDatabaseSeq atomic.Int64

func TestMain(m *testing.M) {
	os.Exit(runWithSharedMongo(m))
}

// runWithSharedMongo exists so the deferred cleanup actually runs — os.Exit
// inside TestMain would skip it and leak the container.
func runWithSharedMongo(m *testing.M) int {
	ctx := context.Background()

	container, err := mongodb.Run(ctx, "mongo:7")
	if err != nil {
		log.Printf("failed starting shared mongo container: %v", err)
		return 1
	}
	defer func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			log.Printf("failed terminating shared mongo container: %v", err)
		}
	}()

	connStr, err := container.ConnectionString(ctx)
	if err != nil {
		log.Printf("failed resolving shared mongo connection string: %v", err)
		return 1
	}

	client, err := mongo.Connect(options.Client().ApplyURI(connStr))
	if err != nil {
		log.Printf("failed connecting to shared mongo container: %v", err)
		return 1
	}
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			log.Printf("failed disconnecting from shared mongo container: %v", err)
		}
	}()

	// The module reports ready on a log line that can precede mongo actually
	// accepting connections, and Connect is lazy. Ping with backoff once here
	// rather than letting the first test eat the reset.
	for attempt := 0; ; attempt++ {
		err = client.Ping(ctx, nil)
		if err == nil || attempt >= 4 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		log.Printf("shared mongo container never became reachable: %v", err)
		return 1
	}

	sharedMongoClient = client
	return m.Run()
}

// mongoDatabase returns a database unique to the calling test, so tests
// sharing the package's one container still cannot observe each other's
// documents.
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
	return sharedMongoClient.Database(fmt.Sprintf("%s_%d", name, mongoDatabaseSeq.Add(1)))
}
