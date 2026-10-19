//go:build integration

package repo

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

// chordCatalogPayload is catalog/chord-voicings-v1/catalog.json, the frozen
// payload the chord catalog's reference data migration installs.
type chordCatalogPayload struct {
	Chords []struct {
		ID             string   `json:"chord_definition_id"`
		Symbol         string   `json:"canonical_symbol"`
		Root           string   `json:"root"`
		RootPitchClass int      `json:"root_pitch_class"`
		Quality        string   `json:"quality"`
		Formula        []string `json:"formula"`
		Omittable      []string `json:"omittable"`
		Bass           *string  `json:"bass"`
		BassPitchClass *int     `json:"bass_pitch_class"`
	} `json:"chords"`
	Voicings []struct {
		Key       string   `json:"key"`
		ID        string   `json:"chord_voicing_id"`
		ChordID   string   `json:"chord_definition_id"`
		DiagramID string   `json:"diagram_id"`
		Tuning    string   `json:"tuning_fingerprint"`
		Omitted   []string `json:"omitted_intervals"`
		IsMovable bool     `json:"is_movable"`
		Rank      int      `json:"recommended_rank"`
	} `json:"voicings"`
	Diagrams []struct {
		ID        string            `json:"diagram_id"`
		Key       string            `json:"key"`
		Names     map[string]string `json:"names"`
		Root      string            `json:"root_note"`
		Playbacks []struct {
			ID    string            `json:"playback_id"`
			Names map[string]string `json:"names"`
			Tempo int               `json:"tempo_bpm"`
			Steps []struct {
				IDs   []string         `json:"position_ids"`
				Value domain.NoteValue `json:"value"`
				Strum domain.Strum     `json:"strum"`
			} `json:"steps"`
		} `json:"playbacks"`
		DefaultPlaybackID *string `json:"default_playback_id"`
		Positions         []struct {
			ID       string               `json:"position_id"`
			Interval string               `json:"interval"`
			Name     string               `json:"note_name"`
			Shape    domain.PositionShape `json:"shape"`
			Color    *string              `json:"color"`
			String   int                  `json:"string"`
			Fret     int                  `json:"fret"`
		} `json:"positions"`
	} `json:"diagrams"`
}

// TestChordCatalog checks the frozen chord catalog with the same domain
// rules authoring and the chord catalog use, so the Python generator and the
// Go validator can't disagree, then installs it through every migration.
func TestChordCatalog(t *testing.T) {
	ctx := context.Background()
	root, err := filepath.Abs("../../../../..")
	require.NoError(t, err)
	raw, err := os.ReadFile(filepath.Join(root, "catalog/chord-voicings-v1/catalog.json"))
	require.NoError(t, err, "generate the chord catalog first")
	var payload chordCatalogPayload
	require.NoError(t, json.Unmarshal(raw, &payload))

	six := 6
	instrument, err := domain.NewInstrument(acousticGuitarID, map[string]string{"en": "Acoustic guitar", "pt_BR": "Violão"}, []string{"en", "pt_BR"}, domain.InstrumentFamilyFretted, &six, []string{"E2", "A2", "D3", "G3", "B3", "E4"}, nil, domain.Voice{ID: "acoustic-guitar", Family: domain.InstrumentFamilyFretted})
	require.NoError(t, err)
	chords := map[string]domain.ChordDefinition{}
	for _, c := range payload.Chords {
		chords[c.ID] = domain.ChordDefinition{ID: c.ID, CanonicalSymbol: c.Symbol, Root: c.Root, RootPitchClass: c.RootPitchClass,
			Quality: domain.ChordQuality(c.Quality), Formula: c.Formula, Omittable: c.Omittable, Bass: c.Bass, BassPitchClass: c.BassPitchClass}
		require.True(t, domain.ChordQuality(c.Quality).Valid(), c.Symbol)
		parsed := domain.ParseChordSymbol(c.Symbol)
		require.NotNil(t, parsed.Parsed, "the parser reads every canonical symbol: %s", c.Symbol)
		require.Equal(t, c.Symbol, parsed.Parsed.CanonicalSymbol)
	}
	diagrams := map[string]domain.Diagram{}
	for _, e := range payload.Diagrams {
		positions := make([]domain.Position, len(e.Positions))
		for i, p := range e.Positions {
			positions[i] = domain.Position{ID: p.ID, Interval: p.Interval, NoteName: p.Name, Shape: p.Shape, Color: p.Color, String: &p.String, Fret: &p.Fret}
		}
		playbacks := make([]domain.DiagramPlayback, len(e.Playbacks))
		for i, p := range e.Playbacks {
			steps := make([]domain.SequenceStep, len(p.Steps))
			for j, s := range p.Steps {
				steps[j] = domain.SequenceStep{PositionIDs: s.IDs, Value: s.Value, Strum: s.Strum}
			}
			playbacks[i] = domain.DiagramPlayback{ID: p.ID, Names: p.Names, TempoBPM: p.Tempo, Steps: steps}
		}
		d, err := domain.NewDiagram(e.ID, uuid.NewString(), instrument, e.Names, []string{"en", "pt_BR"}, positions, []string{uuid.NewString()}, []string{uuid.NewString()}, domain.DiagramOptions{Kind: domain.DiagramKindBasic, RootNote: &e.Root, Playbacks: playbacks, DefaultPlaybackID: e.DefaultPlaybackID}, time.Now())
		require.NoError(t, err, e.Key)
		d.Purpose = domain.DiagramPurposeChordVoicing
		diagrams[d.ID] = d
	}
	for _, v := range payload.Voicings {
		voicing := domain.ChordVoicing{ID: v.ID, ChordDefinitionID: v.ChordID, DiagramID: v.DiagramID, InstrumentID: acousticGuitarID,
			TuningFingerprint: v.Tuning, OmittedIntervals: v.Omitted, IsMovable: v.IsMovable, RecommendedRank: v.Rank}
		require.NoError(t, domain.ValidateChordVoicing(chords[v.ChordID], diagrams[v.DiagramID], instrument, voicing), v.Key)
	}

	db := startMigrationPostgres(t, ctx)
	for _, file := range migrationFiles(t) {
		contents, err := os.ReadFile(file)
		require.NoError(t, err)
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, string(contents))
		require.NoError(t, err, file)
		require.NoError(t, tx.Commit())
	}
	count := func(query string, args ...any) int {
		var n int
		require.NoError(t, db.QueryRowContext(ctx, query, args...).Scan(&n))
		return n
	}
	require.Equal(t, len(payload.Chords), count("SELECT count(*) FROM chord_definitions"))
	require.Equal(t, len(payload.Voicings), count("SELECT count(*) FROM chord_voicings WHERE status='active'"))
	require.Equal(t, len(payload.Diagrams), count("SELECT count(*) FROM diagrams WHERE purpose='chord_voicing'"))
	require.Equal(t, 0, count("SELECT count(*) FROM chord_voicings v JOIN diagrams d ON d.id=v.diagram_id WHERE d.purpose<>'chord_voicing'"))
	require.Equal(t, 0, count("SELECT count(*) FROM diagrams WHERE purpose='chord_voicing' AND created_by<>$1", "77d0239a-8d28-5c95-bc6e-53d59f7f84a8"))
	t.Logf("Validated %d chords and %d voicings in Go and installed them through every migration", len(payload.Chords), len(payload.Voicings))
}
