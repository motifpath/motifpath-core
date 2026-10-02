//go:build integration

package repo

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/domain"
	"github.com/stretchr/testify/require"
)

// TestBasicCatalog validates the frozen production payload through the same
// constructors used by authoring, then proves the SQL installs atomically.
func TestBasicCatalog(t *testing.T) {
	ctx := context.Background()
	root, err := filepath.Abs("../../../../..")
	require.NoError(t, err)
	catalogDir := filepath.Join(root, "catalog/basic-guitar-v1")
	raw, err := os.ReadFile(filepath.Join(catalogDir, "catalog.json"))
	require.NoError(t, err, "generate the catalog first")
	var entries []struct {
		ID       string              `json:"diagram_id"`
		Key      string              `json:"key"`
		Names    map[string]string   `json:"names"`
		Root     string              `json:"root_note"`
		Mode     *domain.DiagramMode `json:"mode"`
		Tempo    *int                `json:"tempo_bpm"`
		Sequence []struct {
			IDs   []string         `json:"position_ids"`
			Value domain.NoteValue `json:"value"`
			Strum domain.Strum     `json:"strum"`
		} `json:"sequence"`
		Positions []struct {
			ID       string               `json:"position_id"`
			Interval string               `json:"interval"`
			Name     string               `json:"note_name"`
			Shape    domain.PositionShape `json:"shape"`
			Color    *string              `json:"color"`
			String   int                  `json:"string"`
			Fret     int                  `json:"fret"`
		} `json:"positions"`
	}
	require.NoError(t, json.Unmarshal(raw, &entries))
	six := 6
	instrument, err := domain.NewInstrument(uuid.NewString(), map[string]string{"en": "Acoustic guitar", "pt_BR": "Violão"}, []string{"en", "pt_BR"}, domain.InstrumentFamilyFretted, &six, []string{"E2", "A2", "D3", "G3", "B3", "E4"}, nil, domain.Voice{ID: "acoustic-guitar", Family: domain.InstrumentFamilyFretted})
	require.NoError(t, err)
	owner := "77d0239a-8d28-5c95-bc6e-53d59f7f84a8"
	positionsCount := 0
	for _, e := range entries {
		ps := make([]domain.Position, len(e.Positions))
		for i, p := range e.Positions {
			ps[i] = domain.Position{ID: p.ID, Interval: p.Interval, NoteName: p.Name, Shape: p.Shape, Color: p.Color, String: &p.String, Fret: &p.Fret}
		}
		steps := make([]domain.SequenceStep, len(e.Sequence))
		for i, s := range e.Sequence {
			steps[i] = domain.SequenceStep{PositionIDs: s.IDs, Value: s.Value, Strum: s.Strum}
		}
		_, err := domain.NewDiagram(e.ID, owner, instrument, e.Names, []string{"en", "pt_BR"}, ps, []string{uuid.NewString()}, []string{uuid.NewString()}, domain.DiagramOptions{Kind: domain.DiagramKindBasic, RootNote: &e.Root, Mode: e.Mode, TempoBPM: e.Tempo, Sequence: steps}, time.Now())
		require.NoError(t, err, e.Key)
		positionsCount += len(ps)
	}
	db := startMigrationPostgres(t, ctx)
	for _, file := range migrationFiles(t) {
		if filepath.Base(file) == "20261002123500_basic_guitar_catalog.up.sql" {
			continue
		}
		contents, err := os.ReadFile(file)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, string(contents))
		require.NoError(t, err, file)
	}
	sqlBytes, err := os.ReadFile(filepath.Join(root, "services/core-domain/internal/adapters/repo/ent/migrate/migrations/20261002123500_basic_guitar_catalog.up.sql"))
	require.NoError(t, err)
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(sqlBytes))
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	var count int
	var clerkUserID, role, displayName, locale string
	require.NoError(t, db.QueryRowContext(ctx, "SELECT u.clerk_user_id,u.role,u.display_name,l.code FROM users u JOIN languages l ON l.id=u.locale_id WHERE u.id=$1", owner).Scan(&clerkUserID, &role, &displayName, &locale))
	require.Equal(t, "system:catalog", clerkUserID)
	require.Equal(t, "admin", role)
	require.Equal(t, "MotifPath Catalog", displayName)
	require.Equal(t, "en", locale)
	var layouts []string
	rows, err := db.QueryContext(ctx, "SELECT DISTINCT instrument_id::text FROM diagrams")
	require.NoError(t, err)
	for rows.Next() {
		var id string
		require.NoError(t, rows.Scan(&id))
		layouts = append(layouts, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []string{acousticGuitarID}, layouts)
	var linkedInstruments int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM diagram_instruments WHERE instrument_id IN ($1, $2)", acousticGuitarID, electricGuitarID).Scan(&linkedInstruments))
	require.Equal(t, len(entries)*2, linkedInstruments)
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM diagrams").Scan(&count))
	require.Equal(t, len(entries), count)
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM positions").Scan(&count))
	require.Equal(t, positionsCount, count)
	// Reusing a released payload outside Atlas must fail rather than overwrite content.
	tx, err = db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(sqlBytes))
	require.Error(t, err)
	require.NoError(t, tx.Rollback())
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM diagrams").Scan(&count))
	require.Equal(t, len(entries), count)
	t.Logf("Validated %d diagrams, %d positions and SQL rollback/owner constraints", len(entries), positionsCount)
}

// TestCatalogAtlasChecksum ensures frozen SQL agrees with the migration ledger manifest.
func TestCatalogAtlasChecksum(t *testing.T) {
	if _, err := exec.LookPath("atlas"); err != nil {
		t.Skip("Atlas CLI not on PATH")
	}
	root, err := filepath.Abs("../../../../..")
	require.NoError(t, err)
	cmd := exec.Command("atlas", "migrate", "validate", "--dir", "file://"+filepath.Join(root, "services/core-domain/internal/adapters/repo/ent/migrate/migrations"))
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, strings.TrimSpace(string(output)))
}

// Keep the generated ORM package in this test's dependency set so changes to
// its schema are compiled alongside the data migration assertions.
var _ = ent.Client{}
