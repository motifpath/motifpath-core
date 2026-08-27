// Command entmigrate generates a new versioned Atlas migration file from the
// current diff between the ent schema and the migrations directory, per
// ADR-005/ADR-010. Invoked via `make migrate:diff name=<description>`.
//
// It spins up a scratch Postgres container via testcontainers to act as
// Atlas's "dev database" — the same role Atlas CLI's own `--dev-url
// docker://postgres/16/dev` flag would play, had this Atlas build not
// dropped the `ent://` schema loader it relied on. The container never
// holds real data; it exists only so Atlas can compute the diff.
package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql/schema"
	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"ariga.io/atlas/sql/migrate"
	"ariga.io/atlas/sql/sqltool"

	entmigrate "github.com/motifpath/core-domain/internal/adapters/repo/ent/migrate"
)

const migrationsDir = "internal/adapters/repo/ent/migrate/migrations"

func main() {
	if len(os.Args) != 2 {
		log.Fatalln("usage: entmigrate <migration-name>")
	}
	name := os.Args[1]

	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("dev"),
		tcpostgres.WithUsername("dev"),
		tcpostgres.WithPassword("dev"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp")),
	)
	if err != nil {
		log.Fatalf("failed starting scratch postgres container: %v", err)
	}
	defer func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			log.Printf("failed terminating scratch postgres container: %v", err)
		}
	}()

	devURL, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("failed resolving scratch postgres connection string: %v", err)
	}

	dir, err := migrate.NewLocalDir(migrationsDir)
	if err != nil {
		log.Fatalf("failed opening atlas migration directory: %v", err)
	}

	opts := []schema.MigrateOption{
		schema.WithDir(dir),
		schema.WithMigrationMode(schema.ModeReplay),
		schema.WithDialect(dialect.Postgres),
		schema.WithFormatter(sqltool.GolangMigrateFormatter),
	}

	if err := entmigrate.NamedDiff(ctx, devURL, name, opts...); err != nil {
		log.Fatalf("failed generating migration file: %v", err)
	}

	// ADR-005: down migrations are not generated, not maintained, not used —
	// rollback is redeploying the previous image, never a schema rollback.
	// sqltool.GolangMigrateFormatter emits a paired .down.sql file regardless;
	// discard it so nothing tempts a future incident responder to run it.
	downFiles, err := filepath.Glob(filepath.Join(migrationsDir, "*.down.sql"))
	if err != nil {
		log.Fatalf("failed listing down-migration files: %v", err)
	}
	for _, f := range downFiles {
		if err := os.Remove(f); err != nil {
			log.Fatalf("failed removing down-migration file %s: %v", f, err)
		}
	}

	// Removing the down files invalidates the atlas.sum checksum NamedDiff
	// just wrote (it hashed the directory including those files) — recompute
	// and rewrite it so `atlas migrate lint`/`apply` don't see a mismatch.
	sum, err := dir.Checksum()
	if err != nil {
		log.Fatalf("failed recomputing migration directory checksum: %v", err)
	}
	if err := migrate.WriteSumFile(dir, sum); err != nil {
		log.Fatalf("failed writing atlas.sum: %v", err)
	}

	log.Println("migration file generated for", name)
}
