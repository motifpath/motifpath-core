//go:build integration

package repo

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// TestMigrationsApplyCleanly runs every versioned migration file in
// internal/adapters/repo/ent/migrate/migrations/ against a fresh Postgres
// container, in filename (timestamp) order — the same mechanism
// `atlas migrate apply` uses at service startup (ADR-005), minus the
// advisory lock, which only matters under concurrent replica startup, not
// a single-connection test.
func TestMigrationsApplyCleanly(t *testing.T) {
	ctx := context.Background()
	db := startMigrationPostgres(t, ctx)

	files := migrationFiles(t)
	applied := 0
	for _, file := range files {
		n, err := execMigrationFile(ctx, db, file)
		require.NoError(t, err)
		applied += n
	}

	// Atlas writes a `-- description` line above every statement, and those
	// comments share a `;`-delimited chunk with the statement they describe.
	// Guard against a splitter regression that silently skips every
	// statement and leaves this test passing against an empty database.
	require.NotZero(t, applied, "expected at least one migration statement to be executed")
	assertColumnExists(t, ctx, db, "learning_path_items", "position")
	assertColumnExists(t, ctx, db, "learning_path_items", "section_label")
	assertColumnExists(t, ctx, db, "diagrams", "kind")
	assertColumnExists(t, ctx, db, "diagrams", "created_by")
	assertColumnExists(t, ctx, db, "users", "display_name")
}

// TestUserDisplayNameMigration covers the backfill of users that predate
// display names: they get a placeholder, which their real name replaces on
// their next authenticated request, and the column ends up NOT NULL with
// no default, so a new user can only be created with a name.
func TestUserDisplayNameMigration(t *testing.T) {
	ctx := context.Background()
	files := migrationFiles(t)
	displayName := -1
	for i, f := range files {
		if strings.HasSuffix(f, "_pb66_user_display_name.up.sql") {
			displayName = i
		}
	}
	require.NotEqual(t, -1, displayName, "expected a *_pb66_user_display_name.up.sql migration")

	db := startMigrationPostgres(t, ctx)
	for _, file := range files[:displayName] {
		_, err := execMigrationFile(ctx, db, file)
		require.NoError(t, err)
	}
	mustExec(t, ctx, db, `INSERT INTO users (id, clerk_user_id, role, registered_at, locale_id)
		SELECT 'bbbbbbbb-0000-0000-0000-000000000001', 'clerk-existing', 'teacher', now(), id FROM languages WHERE code = 'en'`)

	_, err := execMigrationFile(ctx, db, files[displayName])
	require.NoError(t, err)

	var name string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT display_name FROM users WHERE clerk_user_id = 'clerk-existing'`).Scan(&name))
	assert.Equal(t, "MotifPath user", name)

	_, err = db.ExecContext(ctx, `INSERT INTO users (id, clerk_user_id, role, registered_at, locale_id)
		SELECT 'bbbbbbbb-0000-0000-0000-000000000002', 'clerk-new', 'student', now(), id FROM languages WHERE code = 'en'`)
	require.Error(t, err, "a user without a display name must be refused once the placeholder default is dropped")
}

// TestDiagramOwnershipMigration covers the one migration that backfills
// data rather than only reshaping it: diagrams that predate ownership become
// basic templates owned by the zero user — the earliest-registered admin —
// and the migration refuses to invent an owner when no admin exists.
func TestDiagramOwnershipMigration(t *testing.T) {
	ctx := context.Background()
	files := migrationFiles(t)
	ownership := -1
	for i, f := range files {
		if strings.HasSuffix(f, "_pb57_diagram_ownership.up.sql") {
			ownership = i
		}
	}
	require.NotEqual(t, -1, ownership, "expected a *_pb57_diagram_ownership.up.sql migration")

	// prepare applies every migration before the ownership one, then seeds
	// one instrument and one diagram that predate it.
	prepare := func(t *testing.T) *sql.DB {
		t.Helper()
		db := startMigrationPostgres(t, ctx)
		for _, file := range files[:ownership] {
			_, err := execMigrationFile(ctx, db, file)
			require.NoError(t, err)
		}
		mustExec(t, ctx, db, `INSERT INTO instruments (id, name, family, string_count) VALUES ('11111111-1111-1111-1111-111111111111', 'Guitar', 'fretted', 6)`)
		mustExec(t, ctx, db, `INSERT INTO diagrams (id, name, created_at, instrument_id) VALUES ('22222222-2222-2222-2222-222222222222', 'Legacy', now(), '11111111-1111-1111-1111-111111111111')`)
		return db
	}
	insertUser := func(t *testing.T, db *sql.DB, id, role, registeredAt string) {
		t.Helper()
		mustExec(t, ctx, db, `INSERT INTO users (id, clerk_user_id, role, registered_at, locale_id)
			SELECT '`+id+`', 'clerk-`+id+`', '`+role+`', '`+registeredAt+`', id FROM languages WHERE code = 'en'`)
	}

	t.Run("existing diagrams become basic, owned by the earliest-registered admin", func(t *testing.T) {
		db := prepare(t)
		insertUser(t, db, "aaaaaaaa-0000-0000-0000-000000000001", "teacher", "2026-01-01T00:00:00Z")
		insertUser(t, db, "aaaaaaaa-0000-0000-0000-000000000002", "admin", "2026-02-01T00:00:00Z")
		insertUser(t, db, "aaaaaaaa-0000-0000-0000-000000000003", "admin", "2026-03-01T00:00:00Z")

		_, err := execMigrationFile(ctx, db, files[ownership])
		require.NoError(t, err)

		var kind, createdBy string
		require.NoError(t, db.QueryRowContext(ctx, `SELECT kind, created_by FROM diagrams`).Scan(&kind, &createdBy))
		assert.Equal(t, "basic", kind)
		assert.Equal(t, "aaaaaaaa-0000-0000-0000-000000000002", createdBy)
	})

	t.Run("diagrams without any admin to own them fail the migration", func(t *testing.T) {
		db := prepare(t)
		insertUser(t, db, "aaaaaaaa-0000-0000-0000-000000000001", "teacher", "2026-01-01T00:00:00Z")

		_, err := execMigrationFile(ctx, db, files[ownership])

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no admin user exists")
	})
}

// TestInstrumentNamesMigration covers moving each instrument's single name
// into the per-language names map: the existing name becomes the English
// one, and no other language is invented for it.
func TestInstrumentNamesMigration(t *testing.T) {
	ctx := context.Background()
	files := migrationFiles(t)
	target := -1
	for i, f := range files {
		if strings.HasSuffix(f, "_instrument_names_per_language.up.sql") {
			target = i
		}
	}
	require.NotEqual(t, -1, target, "expected a *_instrument_names_per_language.up.sql migration")

	db := startMigrationPostgres(t, ctx)
	for _, file := range files[:target] {
		_, err := execMigrationFile(ctx, db, file)
		require.NoError(t, err)
	}
	mustExec(t, ctx, db, `INSERT INTO instruments (id, name, family, string_count) VALUES ('11111111-1111-1111-1111-111111111111', 'Guitar', 'fretted', 6)`)

	_, err := execMigrationFile(ctx, db, files[target])
	require.NoError(t, err)

	var names string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT names::text FROM instruments`).Scan(&names))
	assert.JSONEq(t, `{"en": "Guitar"}`, names)
	var nameColumns int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT count(*) FROM information_schema.columns WHERE table_name = 'instruments' AND column_name = 'name'`).Scan(&nameColumns))
	assert.Zero(t, nameColumns, "the single name column should be gone")
}

// TestDiagramNamesMigration covers moving each diagram's single name into
// the per-language names map, as its English name.
func TestDiagramNamesMigration(t *testing.T) {
	ctx := context.Background()
	files := migrationFiles(t)
	target := -1
	for i, f := range files {
		if strings.HasSuffix(f, "_diagram_names_per_language.up.sql") {
			target = i
		}
	}
	require.NotEqual(t, -1, target, "expected a *_diagram_names_per_language.up.sql migration")

	db := startMigrationPostgres(t, ctx)
	for _, file := range files[:target] {
		_, err := execMigrationFile(ctx, db, file)
		require.NoError(t, err)
	}
	mustExec(t, ctx, db, `INSERT INTO users (id, clerk_user_id, role, registered_at, locale_id, display_name)
		SELECT 'aaaaaaaa-0000-0000-0000-000000000001', 'clerk-admin', 'admin', now(), id, 'Admin' FROM languages WHERE code = 'en'`)
	mustExec(t, ctx, db, `INSERT INTO instruments (id, names, family, string_count) VALUES ('11111111-1111-1111-1111-111111111111', '{"en": "Guitar"}', 'fretted', 6)`)
	mustExec(t, ctx, db, `INSERT INTO diagrams (id, name, created_at, instrument_id, kind, created_by, label_display)
		VALUES ('22222222-2222-2222-2222-222222222222', 'A Minor Pentatonic', now(), '11111111-1111-1111-1111-111111111111', 'basic', 'aaaaaaaa-0000-0000-0000-000000000001', 'interval')`)

	_, err := execMigrationFile(ctx, db, files[target])
	require.NoError(t, err)

	var names string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT names::text FROM diagrams`).Scan(&names))
	assert.JSONEq(t, `{"en": "A Minor Pentatonic"}`, names)
	var nameColumns int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT count(*) FROM information_schema.columns WHERE table_name = 'diagrams' AND column_name = 'name'`).Scan(&nameColumns))
	assert.Zero(t, nameColumns, "the single name column should be gone")
}

// startMigrationPostgres starts an empty Postgres container and returns a
// connection to it that is known to accept statements.
func startMigrationPostgres(t *testing.T, ctx context.Context) *sql.DB {
	t.Helper()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("core_domain_migrate_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, testcontainers.TerminateContainer(container))
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, db.Close())
	})

	// Same race setupPostgres documents: the postgres module reports ready
	// on a log line that can precede Postgres actually accepting
	// connections, and sql.Open is lazy, so the first Exec eats the reset.
	// Ping with backoff before executing any migration.
	for attempt := 0; ; attempt++ {
		err = db.PingContext(ctx)
		if err == nil || attempt >= 4 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.NoError(t, err)
	return db
}

// migrationFiles returns every versioned migration file, in the filename
// (timestamp) order they are applied in.
func migrationFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("ent/migrate/migrations/*.up.sql")
	require.NoError(t, err)
	require.NotEmpty(t, files, "expected at least one migration file")
	sort.Strings(files)
	return files
}

// execMigrationFile executes file's statements one at a time and returns
// how many ran, stopping at the first that fails.
func execMigrationFile(ctx context.Context, db *sql.DB, file string) (int, error) {
	sqlBytes, err := os.ReadFile(file) //nolint:gosec // fixed test-local migration directory, not user input
	if err != nil {
		return 0, err
	}
	applied := 0
	for _, stmt := range strings.Split(string(sqlBytes), ";") {
		stmt = stripSQLComments(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return applied, fmt.Errorf("migration %s failed on statement %q: %w", file, stmt, err)
		}
		applied++
	}
	return applied, nil
}

func mustExec(t *testing.T, ctx context.Context, db *sql.DB, stmt string) {
	t.Helper()
	_, err := db.ExecContext(ctx, stmt)
	require.NoError(t, err)
}

// stripSQLComments removes whole-line `--` comments from one `;`-delimited
// chunk of a migration file and trims the remainder. Returns "" when the
// chunk carries no executable SQL.
func stripSQLComments(chunk string) string {
	var lines []string
	for _, line := range strings.Split(chunk, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" && !strings.HasPrefix(trimmed, "--") {
			lines = append(lines, line)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func assertColumnExists(t *testing.T, ctx context.Context, db *sql.DB, table, column string) {
	t.Helper()
	var exists bool
	err := db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = $1 AND column_name = $2)`,
		table, column).Scan(&exists)
	require.NoError(t, err)
	assert.Truef(t, exists, "expected migrations to create %s.%s", table, column)
}

// migrateUpTo applies every migration before the one whose file name ends in
// suffix and returns the database with that one's index, so a test can seed
// rows in the old shape before applying it.
func migrateUpTo(t *testing.T, ctx context.Context, suffix string) (*sql.DB, []string, int) {
	t.Helper()
	files := migrationFiles(t)
	target := -1
	for i, f := range files {
		if strings.HasSuffix(f, suffix) {
			target = i
		}
	}
	require.NotEqual(t, -1, target, "expected a *%s migration", suffix)
	db := startMigrationPostgres(t, ctx)
	for _, file := range files[:target] {
		_, err := execMigrationFile(ctx, db, file)
		require.NoError(t, err)
	}
	return db, files, target
}

// TestCourseLanguageMigration covers existing courses, and the versions
// already published from them, all being English.
func TestCourseLanguageMigration(t *testing.T) {
	ctx := context.Background()
	db, files, target := migrateUpTo(t, ctx, "_course_language.up.sql")
	mustExec(t, ctx, db, `INSERT INTO courses (id, title, summary, level, status, created_by, created_at)
		VALUES ('33333333-3333-3333-3333-333333333333', 'Fingerstyle', 'S', 'beginner', 'published', '44444444-4444-4444-4444-444444444444', now())`)
	mustExec(t, ctx, db, `INSERT INTO course_versions (id, course_id, version_number, title_snapshot, summary_snapshot, level_snapshot, available_for_new_enrollments, published_at)
		VALUES ('55555555-5555-5555-5555-555555555555', '33333333-3333-3333-3333-333333333333', 1, 'Fingerstyle', 'S', 'beginner', true, now())`)

	_, err := execMigrationFile(ctx, db, files[target])
	require.NoError(t, err)

	var language, snapshot string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT language FROM courses`).Scan(&language))
	require.NoError(t, db.QueryRowContext(ctx, `SELECT language_snapshot FROM course_versions`).Scan(&snapshot))
	assert.Equal(t, "en", language)
	assert.Equal(t, "en", snapshot)
}

// TestLearningPathLevelMigration covers existing paths getting no level, and
// a last update equal to their creation time.
func TestLearningPathLevelMigration(t *testing.T) {
	ctx := context.Background()
	db, files, target := migrateUpTo(t, ctx, "_learning_path_level_and_updated_at.up.sql")
	mustExec(t, ctx, db, `INSERT INTO learning_paths (id, teacher_id, title, created_at)
		VALUES ('66666666-6666-6666-6666-666666666666', '44444444-4444-4444-4444-444444444444', 'Open Chords', '2026-09-01T10:00:00Z')`)

	_, err := execMigrationFile(ctx, db, files[target])
	require.NoError(t, err)

	var level sql.NullString
	var createdAt, updatedAt time.Time
	require.NoError(t, db.QueryRowContext(ctx, `SELECT level, created_at, updated_at FROM learning_paths`).Scan(&level, &createdAt, &updatedAt))
	assert.False(t, level.Valid, "an existing path has no level recorded")
	assert.True(t, updatedAt.Equal(createdAt), "an existing path was last updated when it was created")
}

// TestDiagramAudioPlaybackMigration covers the platform's voices being
// provided, every instrument getting its family's voice and octaves on its
// tuning, and every diagram starting with no playback: its sequence indices
// were the order the editor's positions were clicked in, not a sequence an
// author chose, so none of them becomes playback steps.
func TestDiagramAudioPlaybackMigration(t *testing.T) {
	ctx := context.Background()
	const (
		admin  = "aaaaaaaa-0000-0000-0000-000000000001"
		guitar = "11111111-1111-1111-1111-111111111111"
		bass   = "11111111-1111-1111-1111-111111111112"
		piano  = "11111111-1111-1111-1111-111111111113"
		lick   = "22222222-2222-2222-2222-222222222221"
		shape  = "22222222-2222-2222-2222-222222222222"
	)
	seed := func(t *testing.T, db *sql.DB) {
		t.Helper()
		mustExec(t, ctx, db, `INSERT INTO users (id, clerk_user_id, role, registered_at, locale_id, display_name)
			SELECT '`+admin+`', 'clerk-admin', 'admin', now(), id, 'Admin' FROM languages WHERE code = 'en'`)
		mustExec(t, ctx, db, `INSERT INTO instruments (id, names, family, string_count, tuning) VALUES
			('`+guitar+`', '{"en": "Guitar"}', 'fretted', 6, '["E", "A", "D", "G", "B", "E"]'),
			('`+bass+`', '{"en": "Bass"}', 'fretted', 4, '["E", "A", "D", "G"]')`)
		mustExec(t, ctx, db, `INSERT INTO instruments (id, names, family, key_range_lowest, key_range_highest) VALUES
			('`+piano+`', '{"en": "Piano"}', 'keyboard', 'A0', 'C8')`)
		mustExec(t, ctx, db, `INSERT INTO diagrams (id, names, created_at, instrument_id, kind, created_by, label_display) VALUES
			('`+lick+`', '{"en": "Lick"}', now(), '`+guitar+`', 'basic', '`+admin+`', 'interval'),
			('`+shape+`', '{"en": "Shape"}', now(), '`+guitar+`', 'basic', '`+admin+`', 'interval')`)
		// Every position of the lick carries a sequence index, as the diagram
		// editor gives each one; the shape carries none.
		mustExec(t, ctx, db, `INSERT INTO positions (id, ordinal, interval, note_name, sequence_index, string_number, fret, diagram_id) VALUES
			('33333333-0000-0000-0000-000000000001', 0, 'R', 'A', 0, 6, 5, '`+lick+`'),
			('33333333-0000-0000-0000-000000000002', 1, 'b3', 'C', 1, 6, 8, '`+lick+`'),
			('33333333-0000-0000-0000-000000000003', 2, '4', 'D', 2, 5, 5, '`+lick+`'),
			('33333333-0000-0000-0000-000000000006', 0, 'R', 'A', NULL, 6, 5, '`+shape+`')`)
	}

	t.Run("voices, default voices and octave tunings are migrated, and no diagram plays", func(t *testing.T) {
		db, files, target := migrateUpTo(t, ctx, "_diagram_audio_playback.up.sql")
		seed(t, db)

		_, err := execMigrationFile(ctx, db, files[target])
		require.NoError(t, err)

		var voices string
		require.NoError(t, db.QueryRowContext(ctx, `SELECT string_agg(id || ':' || family, ',' ORDER BY id) FROM voices`).Scan(&voices))
		assert.Equal(t, "acoustic-guitar:fretted,piano:keyboard", voices)
		var guitarPitches, pianoPitches string
		require.NoError(t, db.QueryRowContext(ctx, `SELECT pitches::text FROM voices WHERE id = 'acoustic-guitar'`).Scan(&guitarPitches))
		require.NoError(t, db.QueryRowContext(ctx, `SELECT pitches::text FROM voices WHERE id = 'piano'`).Scan(&pianoPitches))
		assert.JSONEq(t, `[40, 43, 46, 49, 52, 55, 58, 61, 64, 67, 70, 73, 76, 79, 82, 85, 88]`, guitarPitches, "guitar is sampled every 3 semitones from MIDI 40 to 88")
		assert.JSONEq(t, `[21, 24, 27, 30, 33, 36, 39, 42, 45, 48, 51, 54, 57, 60, 63, 66, 69, 72, 75, 78, 81, 84, 87, 90, 93, 96, 99, 102, 105, 108]`, pianoPitches, "piano is sampled every 3 semitones from MIDI 21 to 108")

		instruments := map[string]struct{ voice, tuning string }{}
		rows, err := db.QueryContext(ctx, `SELECT id, default_voice_id, coalesce(tuning::text, '') FROM instruments`)
		require.NoError(t, err)
		for rows.Next() {
			var id, voice, tuning string
			require.NoError(t, rows.Scan(&id, &voice, &tuning))
			instruments[id] = struct{ voice, tuning string }{voice, tuning}
		}
		require.NoError(t, rows.Err())
		assert.Equal(t, "acoustic-guitar", instruments[guitar].voice)
		assert.JSONEq(t, `["E2", "A2", "D3", "G3", "B3", "E4"]`, instruments[guitar].tuning)
		assert.Equal(t, "acoustic-guitar", instruments[bass].voice)
		assert.JSONEq(t, `["E1", "A1", "D2", "G2"]`, instruments[bass].tuning)
		assert.Equal(t, "piano", instruments[piano].voice)
		assert.Empty(t, instruments[piano].tuning)

		var sequence string
		var tempo sql.NullInt64
		var beats, beatValue int
		require.NoError(t, db.QueryRowContext(ctx, `SELECT sequence::text, tempo_bpm, time_signature_beats, time_signature_beat_value FROM diagrams WHERE id = '`+lick+`'`).Scan(&sequence, &tempo, &beats, &beatValue))
		assert.JSONEq(t, `[]`, sequence, "sequence indices don't become playback steps")
		assert.False(t, tempo.Valid, "a diagram without steps has no tempo")
		assert.Equal(t, [2]int{4, 4}, [2]int{beats, beatValue})

		require.NoError(t, db.QueryRowContext(ctx, `SELECT sequence::text, tempo_bpm FROM diagrams WHERE id = '`+shape+`'`).Scan(&sequence, &tempo))
		assert.JSONEq(t, `[]`, sequence)
		assert.False(t, tempo.Valid)

		var sequenceIndexColumns int
		require.NoError(t, db.QueryRowContext(ctx,
			`SELECT count(*) FROM information_schema.columns WHERE table_name = 'positions' AND column_name = 'sequence_index'`).Scan(&sequenceIndexColumns))
		assert.Zero(t, sequenceIndexColumns, "the sequence_index column should be gone")
	})

	t.Run("a tuning the migration can't place in octaves fails it", func(t *testing.T) {
		db, files, target := migrateUpTo(t, ctx, "_diagram_audio_playback.up.sql")
		mustExec(t, ctx, db, `INSERT INTO instruments (id, names, family, string_count, tuning) VALUES
			('`+guitar+`', '{"en": "Open G"}', 'fretted', 6, '["D", "G", "D", "G", "B", "D"]')`)

		_, err := execMigrationFile(ctx, db, files[target])

		require.Error(t, err)
	})
}
