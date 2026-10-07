//go:build integration

package repo

import (
	"context"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/domain"
)

// TestDiagramShapes reads the shapes the migrations install, so it migrates
// an empty database rather than creating the bare schema.
func TestDiagramShapes(t *testing.T) {
	ctx := context.Background()
	db := startMigrationPostgres(t, ctx)
	for _, file := range migrationFiles(t) {
		_, err := execMigrationFile(ctx, db, file)
		require.NoError(t, err)
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	source := NewEntNodeItemSource(client)
	diagrams := NewEntDiagramRepository(client)
	cAGrip := catalogID("caged/C/A/3")

	// shapes keeps the diagram shape items of items, by item key.
	shapes := func(items []domain.ClassifiedItem) map[string][]string {
		byKey := map[string][]string{}
		for _, item := range items {
			if strings.HasPrefix(item.ItemKey, string(domain.PracticeItemKindDiagramShape)+":") {
				byKey[item.ItemKey] = item.NodeIDs
			}
		}
		return byKey
	}

	t.Run("the guitar has every catalog shape, under its diagram's skill and concept", func(t *testing.T) {
		got, err := source.ClassifiedItems(ctx, acousticGuitarID)

		require.NoError(t, err)
		byKey := shapes(got)
		assert.Len(t, byKey, 298)
		assert.ElementsMatch(t,
			[]string{catalogID("knowledge-node/map-fretboard-caged"), catalogID("knowledge-node/caged-system")},
			byKey[domain.DiagramShapeItemKey(cAGrip)])
		assert.NotContains(t, byKey, domain.DiagramShapeItemKey(catalogID("chromatic/C")), "a map of no family is no shape")
	})

	t.Run("the electric guitar has the same shapes, with the same item keys", func(t *testing.T) {
		guitar, err := source.ClassifiedItems(ctx, acousticGuitarID)
		require.NoError(t, err)
		electric, err := source.ClassifiedItems(ctx, electricGuitarID)
		require.NoError(t, err)

		assert.Equal(t, shapes(guitar), shapes(electric))
	})

	t.Run("an instrument no shape diagram is linked to has none", func(t *testing.T) {
		got, err := source.ClassifiedItems(ctx, electricBassID)

		require.NoError(t, err)
		assert.Empty(t, shapes(got))
	})

	t.Run("no instrument has no shapes", func(t *testing.T) {
		got, err := source.ClassifiedItems(ctx, "")

		require.NoError(t, err)
		assert.Empty(t, shapes(got))
	})

	t.Run("a shape diagram reads back with its family and member", func(t *testing.T) {
		got, err := diagrams.GetByID(ctx, cAGrip)

		require.NoError(t, err)
		require.NotNil(t, got.Shape)
		assert.Equal(t, "A", got.Shape.Shape)
		assert.Equal(t, "caged-grip", got.Shape.Family.Key)
		assert.Equal(t, catalogID("diagram-shape-family/caged-grip"), got.Shape.Family.ID)
		assert.Equal(t, domain.LocalizedText{"en": "CAGED grips", "pt_BR": "Formas do CAGED"}, got.Shape.Family.Names)
		members := make([]string, len(got.Shape.Family.Members))
		for i, m := range got.Shape.Family.Members {
			members[i] = m.Shape
		}
		assert.Equal(t, []string{"C", "A", "G", "E", "D"}, members)
		assert.Equal(t, domain.LocalizedText{"en": "A shape", "pt_BR": "Forma de A"}, got.Shape.Family.Members[1].Names)
	})

	t.Run("a listed shape diagram carries its shape too", func(t *testing.T) {
		page, err := diagrams.List(ctx, domain.DiagramListFilter{}, domain.PageRequest{Limit: domain.MaxPageLimit})

		require.NoError(t, err)
		withShape := 0
		for _, d := range page.Items {
			if d.Shape != nil {
				withShape++
			}
		}
		assert.Positive(t, withShape)
	})

	t.Run("a diagram of no family has no shape", func(t *testing.T) {
		got, err := diagrams.GetByID(ctx, catalogID("chromatic/C"))

		require.NoError(t, err)
		assert.Nil(t, got.Shape)
	})
}
