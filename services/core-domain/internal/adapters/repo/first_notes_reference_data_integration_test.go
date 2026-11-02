//go:build integration

package repo

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertFirstNotesSkills(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	assert.Equal(t, map[string]string{
		"interpret-note-names": catalogID("knowledge-node/interpret-note-names") + "|skill|Interpret note names|Interpretar os nomes das notas|" + catalogID("knowledge-node/reading"),
		"count-semitone-steps": catalogID("knowledge-node/count-semitone-steps") + "|skill|Count semitone steps from a visual reference|Contar semitons com uma referência visual|" + catalogID("knowledge-node/reading"),
	}, stringPairs(t, ctx, db, `SELECT key, concat_ws('|', id::text, kind, names->>'en', names->>'pt_BR', parent_id::text) FROM knowledge_nodes WHERE key IN ('interpret-note-names','count-semitone-steps')`))
	expected := map[string]string{}
	for skill, concepts := range map[string][]string{"interpret-note-names": {"note-names", "octaves"}, "count-semitone-steps": {"tones-and-semitones", "sharps-and-flats"}} {
		for _, concept := range concepts {
			expected[skill+"/"+concept] = catalogID("knowledge-edge/applies/"+skill+"/"+concept) + "|applies|"
		}
	}
	assert.Equal(t, expected, stringPairs(t, ctx, db, `SELECT f.key || '/' || target.key, concat_ws('|', e.id::text,e.type,coalesce(e.level,'')) FROM knowledge_edges e JOIN knowledge_nodes f ON f.id=e.from_id JOIN knowledge_nodes target ON target.id=e.to_id WHERE f.key IN ('interpret-note-names','count-semitone-steps')`))
	var scoped int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM knowledge_node_instruments i JOIN knowledge_nodes n ON n.id=i.knowledge_node_id WHERE n.key IN ('interpret-note-names','count-semitone-steps')`).Scan(&scoped))
	assert.Zero(t, scoped, "both skills are universal")
}

func TestFirstNotesSkillsUpgrade(t *testing.T) {
	ctx := context.Background()
	files := migrationFiles(t)
	var addition string
	for _, file := range files {
		if strings.HasSuffix(file, "_first_notes_skills_reference_data.up.sql") {
			addition = file
		}
	}
	require.NotEmpty(t, addition, "an additive migration must install the two skills")
	db := startMigrationPostgres(t, ctx)
	for _, file := range files {
		if file == addition {
			break
		}
		_, err := execMigrationFile(ctx, db, file)
		require.NoError(t, err)
	}
	beforeNodes := stringPairs(t, ctx, db, `SELECT key,to_jsonb(n)::text FROM knowledge_nodes n`)
	beforeEdges := stringPairs(t, ctx, db, `SELECT id::text,to_jsonb(e)::text FROM knowledge_edges e`)
	_, err := execMigrationFile(ctx, db, addition)
	require.NoError(t, err)
	assertFirstNotesSkills(t, ctx, db)
	afterNodes := stringPairs(t, ctx, db, `SELECT key,to_jsonb(n)::text FROM knowledge_nodes n`)
	afterEdges := stringPairs(t, ctx, db, `SELECT id::text,to_jsonb(e)::text FROM knowledge_edges e`)
	assert.Len(t, afterNodes, len(beforeNodes)+2)
	assert.Len(t, afterEdges, len(beforeEdges)+4)
	for key, row := range beforeNodes {
		assert.Equal(t, row, afterNodes[key], key)
	}
	for key, row := range beforeEdges {
		assert.Equal(t, row, afterEdges[key], key)
	}
	t.Run("an existing definition is not silently skipped", func(t *testing.T) {
		_, err := execMigrationFile(ctx, db, addition)
		require.Error(t, err)
		assertFirstNotesSkills(t, ctx, db)
	})
}
