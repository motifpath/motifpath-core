//go:build integration

package repo

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// catalogNamespace is the UUID v5 namespace every pre-loaded row's fixed ID
// derives from, as <type>/<key>.
var catalogNamespace = uuid.MustParse("4ac75155-7804-5527-a6ba-01b73c0e3e1a")

func catalogID(name string) string { return uuid.NewSHA1(catalogNamespace, []byte(name)).String() }

var (
	acousticGuitarID = catalogID("instrument/guitar")
	electricGuitarID = catalogID("instrument/electric-guitar")
	electricBassID   = catalogID("instrument/electric-bass")
)

// TestFreshInstall migrates an empty database and checks the reference data
// every environment starts with: fixed IDs, the knowledge map's counts and
// invariants, and a classified basic catalog.
func TestFreshInstall(t *testing.T) {
	ctx := context.Background()
	db := startMigrationPostgres(t, ctx)
	for _, file := range migrationFiles(t) {
		_, err := execMigrationFile(ctx, db, file)
		require.NoError(t, err)
	}
	count := func(t *testing.T, query string) int {
		t.Helper()
		var n int
		require.NoError(t, db.QueryRowContext(ctx, query).Scan(&n), query)
		return n
	}

	t.Run("no reference-data migration generates a random id", func(t *testing.T) {
		for _, file := range migrationFiles(t) {
			contents, err := os.ReadFile(file) //nolint:gosec // fixed test-local migration directory, not user input
			require.NoError(t, err)
			assert.NotContains(t, string(contents), "gen_random_uuid", file)
		}
	})

	t.Run("languages have their fixed ids", func(t *testing.T) {
		assert.Equal(t, map[string]string{
			"en": catalogID("language/en"), "pt_BR": catalogID("language/pt_BR"), "any": catalogID("language/any"),
		}, stringPairs(t, ctx, db, "SELECT code, id::text FROM languages"))
	})

	t.Run("voices include the electric bass down to E1", func(t *testing.T) {
		assert.Equal(t, map[string]string{"acoustic-guitar": "fretted", "electric-bass": "fretted", "piano": "keyboard"},
			stringPairs(t, ctx, db, "SELECT id, family FROM voices"))
		assert.LessOrEqual(t, count(t, "SELECT min(p::int) FROM voices, jsonb_array_elements_text(pitches) AS p WHERE id = 'electric-bass'"), 28)
	})

	t.Run("the catalog instruments have their fixed ids, names, voices and icons", func(t *testing.T) {
		assert.Equal(t, map[string]string{
			acousticGuitarID: "Acoustic guitar|Violão|6|acoustic-guitar|acoustic_guitar",
			electricGuitarID: "Electric guitar|Guitarra elétrica|6|acoustic-guitar|electric_guitar",
			electricBassID:   "Electric bass|Contrabaixo elétrico|4|electric-bass|electric_bass",
		}, stringPairs(t, ctx, db, "SELECT id::text, concat_ws('|', names->>'en', names->>'pt_BR', string_count, default_voice_id, icon) FROM instruments"))
	})

	t.Run("the knowledge map has its nodes and edges, with fixed ids", func(t *testing.T) {
		assert.Equal(t, map[string]string{"skill": "216", "concept": "124"}, stringPairs(t, ctx, db, "SELECT kind, count(*)::text FROM knowledge_nodes GROUP BY kind"))
		assert.Equal(t, map[string]string{"applies": "335", "requires": "136"}, stringPairs(t, ctx, db, "SELECT type, count(*)::text FROM knowledge_edges GROUP BY type"))
		assert.Equal(t, 1, count(t, "SELECT count(*) FROM knowledge_nodes WHERE key = 'scales' AND id = '"+catalogID("knowledge-node/scales")+"'"))
		assert.Equal(t, 1, count(t, "SELECT count(*) FROM knowledge_edges WHERE id = '"+catalogID("knowledge-edge/requires/change-chords/play-open-chords")+"'"))
	})

	invariants := []struct {
		name       string
		violations string
	}{
		{"every name is non-blank in en and pt_BR", `SELECT count(*) FROM knowledge_nodes
			WHERE coalesce(trim(names->>'en'), '') = '' OR coalesce(trim(names->>'pt_BR'), '') = ''`},
		{"every parent has its child's kind", `SELECT count(*) FROM knowledge_nodes c JOIN knowledge_nodes p ON p.id = c.parent_id WHERE p.kind <> c.kind`},
		{"no child is for an instrument its parent is not for", `SELECT count(*) FROM knowledge_nodes c JOIN knowledge_nodes p ON p.id = c.parent_id
			WHERE EXISTS (SELECT 1 FROM knowledge_node_instruments WHERE knowledge_node_id = p.id)
			  AND (NOT EXISTS (SELECT 1 FROM knowledge_node_instruments WHERE knowledge_node_id = c.id)
			       OR EXISTS (SELECT 1 FROM knowledge_node_instruments ci WHERE ci.knowledge_node_id = c.id
			                  AND NOT EXISTS (SELECT 1 FROM knowledge_node_instruments pi WHERE pi.knowledge_node_id = p.id AND pi.instrument_id = ci.instrument_id)))`},
		{"applies runs only skill to concept, without a level", `SELECT count(*) FROM knowledge_edges e
			JOIN knowledge_nodes f ON f.id = e.from_id JOIN knowledge_nodes t ON t.id = e.to_id
			WHERE e.type = 'applies' AND (f.kind <> 'skill' OR t.kind <> 'concept' OR e.level IS NOT NULL)`},
		{"every requires edge has a level", `SELECT count(*) FROM knowledge_edges WHERE type = 'requires' AND level IS NULL`},
		{"requires edges form no cycle", `WITH RECURSIVE reach(start, node) AS (
				SELECT from_id, to_id FROM knowledge_edges WHERE type = 'requires'
				UNION SELECT r.start, e.to_id FROM reach r JOIN knowledge_edges e ON e.from_id = r.node AND e.type = 'requires')
			SELECT count(*) FROM reach WHERE start = node`},
		{"the two ends of every requires edge share an instrument", `SELECT count(*) FROM knowledge_edges e
			WHERE e.type = 'requires'
			  AND EXISTS (SELECT 1 FROM knowledge_node_instruments WHERE knowledge_node_id = e.from_id)
			  AND EXISTS (SELECT 1 FROM knowledge_node_instruments WHERE knowledge_node_id = e.to_id)
			  AND NOT EXISTS (SELECT 1 FROM knowledge_node_instruments a JOIN knowledge_node_instruments b ON a.instrument_id = b.instrument_id
			                  WHERE a.knowledge_node_id = e.from_id AND b.knowledge_node_id = e.to_id)`},
		{"every basic diagram has a skill and a concept", `SELECT count(*) FROM diagrams d
			WHERE NOT EXISTS (SELECT 1 FROM diagram_skills WHERE diagram_id = d.id) OR NOT EXISTS (SELECT 1 FROM diagram_concepts WHERE diagram_id = d.id)`},
		{"every diagram classification suits the diagram's instruments", `SELECT count(*) FROM (
				SELECT diagram_id, skill_id AS node_id FROM diagram_skills UNION ALL SELECT diagram_id, concept_id FROM diagram_concepts) c
			WHERE EXISTS (SELECT 1 FROM knowledge_node_instruments WHERE knowledge_node_id = c.node_id)
			  AND NOT EXISTS (SELECT 1 FROM knowledge_node_instruments ni JOIN diagram_instruments di ON di.instrument_id = ni.instrument_id
			                  WHERE ni.knowledge_node_id = c.node_id AND di.diagram_id = c.diagram_id)`},
	}
	for _, inv := range invariants {
		t.Run(inv.name, func(t *testing.T) {
			assert.Zero(t, count(t, inv.violations))
		})
	}

	t.Run("the basic catalog is installed", func(t *testing.T) {
		assert.Positive(t, count(t, "SELECT count(*) FROM diagrams WHERE kind = 'basic'"))
	})

	t.Run("the drill catalog has its templates and every exercise family's default fluent time, with fixed ids", func(t *testing.T) {
		assert.Equal(t, 7, count(t, "SELECT count(*) FROM drill_templates"))
		assert.Equal(t, 1, count(t, "SELECT count(*) FROM drill_templates WHERE key = 'fretboard_cell:name_the_note' AND id = '"+catalogID("drill-template/fretboard_cell:name_the_note")+"'"))
		assert.Equal(t, map[string]string{
			catalogID("drill-threshold/exercise:text_response/v1"):     "exercise:text_response|1|2026-10-01|6000|default",
			catalogID("drill-threshold/exercise:audio_recognition/v1"): "exercise:audio_recognition|1|2026-10-01|4000|default",
			catalogID("drill-threshold/exercise:image_recognition/v1"): "exercise:image_recognition|1|2026-10-01|5000|default",
			catalogID("drill-threshold/exercise:image_choice/v1"):      "exercise:image_choice|1|2026-10-01|5000|default",
			catalogID("drill-threshold/exercise:audio_selection/v1"):   "exercise:audio_selection|1|2026-10-01|4000|default",
		}, stringPairs(t, ctx, db, `SELECT th.id::text, concat_ws('|', te.key, th.version, to_char(th.effective_from AT TIME ZONE 'UTC', 'YYYY-MM-DD'), th.fluent_net_ms, th.source)
			FROM drill_thresholds th JOIN drill_templates te ON te.id = th.template_id`))
	})
}

// stringPairs runs a two-column query and returns its rows as a map.
func stringPairs(t *testing.T, ctx context.Context, db *sql.DB, query string) map[string]string {
	t.Helper()
	rows, err := db.QueryContext(ctx, query)
	require.NoError(t, err, query)
	defer func() { require.NoError(t, rows.Close()) }()
	result := map[string]string{}
	for rows.Next() {
		var k, v string
		require.NoError(t, rows.Scan(&k, &v))
		result[strings.TrimSpace(k)] = v
	}
	require.NoError(t, rows.Err())
	return result
}
