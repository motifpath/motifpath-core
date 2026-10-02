//go:build integration

package repo

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

func knowledgeNode(kind domain.KnowledgeNodeKind, key string, parent *domain.KnowledgeNode, instrumentIDs ...string) domain.KnowledgeNode {
	node := domain.KnowledgeNode{
		ID: uuid.NewString(), Kind: kind, Key: key,
		Names:         domain.LocalizedText{"en": key, "pt_BR": key},
		InstrumentIDs: instrumentIDs,
	}
	if parent != nil {
		node.ParentID = &parent.ID
	}
	return node
}

func nodeKeys(nodes []domain.KnowledgeNode) []string {
	keys := make([]string, len(nodes))
	for i, n := range nodes {
		keys[i] = n.Key
	}
	return keys
}

func TestEntKnowledgeNodeRepository(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	repo := NewEntKnowledgeNodeRepository(client)
	instruments := NewEntInstrumentRepository(client)
	guitar, piano := frettedInstrument(), keyboardInstrument()
	require.NoError(t, instruments.Create(ctx, guitar))
	require.NoError(t, instruments.Create(ctx, piano))

	fretting := knowledgeNode(domain.KnowledgeNodeKindSkill, "fretting", nil, guitar.ID)
	barre := knowledgeNode(domain.KnowledgeNodeKindSkill, "barre-chords", &fretting, guitar.ID)
	eShape := knowledgeNode(domain.KnowledgeNodeKindSkill, "e-shape-barre", &barre, guitar.ID)
	pedal := knowledgeNode(domain.KnowledgeNodeKindSkill, "pedal-sustain", nil, piano.ID)
	scale := knowledgeNode(domain.KnowledgeNodeKindConcept, "major-scale", nil)
	scale.Descriptions = domain.LocalizedText{"en": "Seven notes", "pt_BR": "Sete notas"}
	for _, n := range []domain.KnowledgeNode{fretting, barre, eShape, pedal, scale} {
		require.NoError(t, repo.Create(ctx, n))
	}

	t.Run("a node reads back as created", func(t *testing.T) {
		got, err := repo.GetByID(ctx, barre.ID)

		require.NoError(t, err)
		assert.Equal(t, barre, got)
		got, err = repo.GetByID(ctx, scale.ID)
		require.NoError(t, err)
		assert.Equal(t, scale, got)
	})

	t.Run("an unknown or malformed id is not found", func(t *testing.T) {
		_, err := repo.GetByID(ctx, uuid.NewString())
		assert.ErrorIs(t, err, domain.ErrNotFound)
		_, err = repo.GetByID(ctx, "not-a-uuid")
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a duplicate key already exists", func(t *testing.T) {
		err := repo.Create(ctx, knowledgeNode(domain.KnowledgeNodeKindConcept, "fretting", nil))

		assert.ErrorIs(t, err, domain.ErrAlreadyExists)
	})

	t.Run("GetByIDs and GetByKeys skip what does not exist", func(t *testing.T) {
		byID, err := repo.GetByIDs(ctx, []string{fretting.ID, scale.ID, uuid.NewString(), "bad"})
		require.NoError(t, err)
		assert.Len(t, byID, 2)
		assert.Equal(t, "major-scale", byID[scale.ID].Key)

		byKey, err := repo.GetByKeys(ctx, []string{"fretting", "nope"})
		require.NoError(t, err)
		assert.Len(t, byKey, 1)
		assert.Equal(t, fretting.ID, byKey["fretting"].ID)
	})

	t.Run("List sorts by key and filters by kind and instruments", func(t *testing.T) {
		all, err := repo.List(ctx, ports.KnowledgeNodeFilter{})
		require.NoError(t, err)
		assert.Equal(t, []string{"barre-chords", "e-shape-barre", "fretting", "major-scale", "pedal-sustain"}, nodeKeys(all))

		concept := domain.KnowledgeNodeKindConcept
		concepts, err := repo.List(ctx, ports.KnowledgeNodeFilter{Kind: &concept})
		require.NoError(t, err)
		assert.Equal(t, []string{"major-scale"}, nodeKeys(concepts))

		forPiano, err := repo.List(ctx, ports.KnowledgeNodeFilter{InstrumentIDs: []string{piano.ID}})
		require.NoError(t, err)
		assert.Equal(t, []string{"major-scale", "pedal-sustain"}, nodeKeys(forPiano))

		forBoth, err := repo.List(ctx, ports.KnowledgeNodeFilter{InstrumentIDs: []string{piano.ID, guitar.ID}})
		require.NoError(t, err)
		assert.Len(t, forBoth, 5)
	})

	t.Run("Children lists only direct children", func(t *testing.T) {
		got, err := repo.Children(ctx, fretting.ID)
		require.NoError(t, err)
		assert.Equal(t, []string{"barre-chords"}, nodeKeys(got))
		assert.Equal(t, []string{guitar.ID}, got[0].InstrumentIDs)

		got, err = repo.Children(ctx, pedal.ID)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("InSubtree follows parent links to any depth", func(t *testing.T) {
		for _, tt := range []struct {
			root, candidate string
			want            bool
		}{
			{fretting.ID, fretting.ID, true},
			{fretting.ID, eShape.ID, true},
			{barre.ID, eShape.ID, true},
			{eShape.ID, fretting.ID, false},
			{fretting.ID, pedal.ID, false},
		} {
			got, err := repo.InSubtree(ctx, tt.root, tt.candidate)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got, "%s ⊇ %s", tt.root, tt.candidate)
		}
	})

	t.Run("Update replaces names, descriptions, parent and instruments", func(t *testing.T) {
		moved := eShape
		moved.Names = domain.LocalizedText{"en": "E-shape barre", "pt_BR": "Pestana forma de Mi"}
		moved.Descriptions = domain.LocalizedText{"en": "d", "pt_BR": "d"}
		moved.ParentID = &fretting.ID
		moved.InstrumentIDs = nil

		require.NoError(t, repo.Update(ctx, moved))

		got, err := repo.GetByID(ctx, eShape.ID)
		require.NoError(t, err)
		assert.Equal(t, moved, got)

		moved.Descriptions = nil
		moved.ParentID = nil
		require.NoError(t, repo.Update(ctx, moved))
		got, err = repo.GetByID(ctx, eShape.ID)
		require.NoError(t, err)
		assert.Nil(t, got.Descriptions)
		assert.Nil(t, got.ParentID)
	})

	t.Run("Update and Delete of an unknown node are not found", func(t *testing.T) {
		assert.ErrorIs(t, repo.Update(ctx, knowledgeNode(domain.KnowledgeNodeKindSkill, "ghost", nil)), domain.ErrNotFound)
		assert.ErrorIs(t, repo.Delete(ctx, uuid.NewString()), domain.ErrNotFound)
	})

	t.Run("Delete removes a node and its instrument links", func(t *testing.T) {
		require.NoError(t, repo.Delete(ctx, pedal.ID))

		_, err := repo.GetByID(ctx, pedal.ID)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestEntKnowledgeNodeRepository_UsageAndClassifiedInstruments(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	repo := NewEntKnowledgeNodeRepository(client)
	edges := NewEntKnowledgeEdgeRepository(client)
	nodes := NewEntContentNodeRepository(client)
	instruments := NewEntInstrumentRepository(client)
	guitar := frettedInstrument()
	require.NoError(t, instruments.Create(ctx, guitar))

	skill := knowledgeNode(domain.KnowledgeNodeKindSkill, "fretting", nil)
	child := knowledgeNode(domain.KnowledgeNodeKindSkill, "barre-chords", &skill)
	concept := knowledgeNode(domain.KnowledgeNodeKindConcept, "chords", nil)
	unused := knowledgeNode(domain.KnowledgeNodeKindConcept, "unused", nil)
	for _, n := range []domain.KnowledgeNode{skill, child, concept, unused} {
		require.NoError(t, repo.Create(ctx, n))
	}
	require.NoError(t, edges.Create(ctx, domain.KnowledgeEdge{ID: uuid.NewString(), FromID: skill.ID, ToID: concept.ID, Type: domain.KnowledgeEdgeTypeApplies}))
	content := domain.ContentNode{
		ID: uuid.NewString(), TeacherID: uuid.NewString(), Title: "Fretting basics", ContentType: domain.ContentTypeVideo, InstrumentIDs: []string{guitar.ID},
		Classification: domain.Classification{Skills: []domain.KnowledgeNode{skill}, Concepts: []domain.KnowledgeNode{concept}, DifficultyLevel: domain.DifficultyLevelBeginner, ReviewState: domain.ReviewStatePending},
		CreatedAt:      fixedAt,
	}
	require.NoError(t, nodes.Create(ctx, content))

	t.Run("Usage counts children, edges and content", func(t *testing.T) {
		got, err := repo.Usage(ctx, skill.ID)
		require.NoError(t, err)
		assert.Equal(t, ports.KnowledgeNodeUsage{Children: 1, Edges: 1, ContentNodes: 1}, got)

		got, err = repo.Usage(ctx, concept.ID)
		require.NoError(t, err)
		assert.Equal(t, ports.KnowledgeNodeUsage{Edges: 1, ContentNodes: 1}, got)

		got, err = repo.Usage(ctx, unused.ID)
		require.NoError(t, err)
		assert.Equal(t, ports.KnowledgeNodeUsage{}, got)
	})

	t.Run("ClassifiedInstrumentSets lists each classified item's instruments", func(t *testing.T) {
		got, err := repo.ClassifiedInstrumentSets(ctx, skill.ID)
		require.NoError(t, err)
		assert.Equal(t, [][]string{{guitar.ID}}, got)

		got, err = repo.ClassifiedInstrumentSets(ctx, unused.ID)
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestEntKnowledgeEdgeRepository(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	nodes := NewEntKnowledgeNodeRepository(client)
	repo := NewEntKnowledgeEdgeRepository(client)

	improvise := knowledgeNode(domain.KnowledgeNodeKindSkill, "improvise-over-a-blues", nil)
	pentatonic := knowledgeNode(domain.KnowledgeNodeKindSkill, "play-pentatonic-positions", nil)
	scale := knowledgeNode(domain.KnowledgeNodeKindConcept, "minor-pentatonic-scale", nil)
	blues := knowledgeNode(domain.KnowledgeNodeKindConcept, "blues-form", nil)
	for _, n := range []domain.KnowledgeNode{improvise, pentatonic, scale, blues} {
		require.NoError(t, nodes.Create(ctx, n))
	}
	fluent, accurate := domain.MasteryLevelFluent, domain.MasteryLevelAccurate
	needsPentatonic := domain.KnowledgeEdge{ID: uuid.NewString(), FromID: improvise.ID, ToID: pentatonic.ID, Type: domain.KnowledgeEdgeTypeRequires, Level: &fluent}
	needsScale := domain.KnowledgeEdge{ID: uuid.NewString(), FromID: pentatonic.ID, ToID: scale.ID, Type: domain.KnowledgeEdgeTypeRequires, Level: &accurate}
	appliesBlues := domain.KnowledgeEdge{ID: uuid.NewString(), FromID: improvise.ID, ToID: blues.ID, Type: domain.KnowledgeEdgeTypeApplies}
	for _, e := range []domain.KnowledgeEdge{needsPentatonic, needsScale, appliesBlues} {
		require.NoError(t, repo.Create(ctx, e))
	}

	t.Run("an edge reads back as created", func(t *testing.T) {
		got, err := repo.GetByID(ctx, needsPentatonic.ID)
		require.NoError(t, err)
		assert.Equal(t, needsPentatonic, got)

		got, err = repo.GetByID(ctx, appliesBlues.ID)
		require.NoError(t, err)
		assert.Equal(t, appliesBlues, got)
	})

	t.Run("the same type between the same nodes already exists, another type does not", func(t *testing.T) {
		dup := appliesBlues
		dup.ID = uuid.NewString()
		assert.ErrorIs(t, repo.Create(ctx, dup), domain.ErrAlreadyExists)

		alsoRequires := domain.KnowledgeEdge{ID: uuid.NewString(), FromID: improvise.ID, ToID: blues.ID, Type: domain.KnowledgeEdgeTypeRequires, Level: &fluent}
		assert.NoError(t, repo.Create(ctx, alsoRequires))
		require.NoError(t, repo.Delete(ctx, alsoRequires.ID))
	})

	t.Run("List filters by type, from and to", func(t *testing.T) {
		requires := domain.KnowledgeEdgeTypeRequires
		got, err := repo.List(ctx, ports.KnowledgeEdgeFilter{Type: &requires, FromID: &improvise.ID})
		require.NoError(t, err)
		assert.Equal(t, []domain.KnowledgeEdge{needsPentatonic}, got)

		got, err = repo.List(ctx, ports.KnowledgeEdgeFilter{ToID: &blues.ID})
		require.NoError(t, err)
		assert.Equal(t, []domain.KnowledgeEdge{appliesBlues}, got)

		got, err = repo.List(ctx, ports.KnowledgeEdgeFilter{})
		require.NoError(t, err)
		assert.Len(t, got, 3)
	})

	t.Run("RequiresPathExists follows requires edges across kinds, not applies", func(t *testing.T) {
		for _, tt := range []struct {
			from, to string
			want     bool
		}{
			{improvise.ID, pentatonic.ID, true},
			{improvise.ID, scale.ID, true},
			{scale.ID, improvise.ID, false},
			{improvise.ID, blues.ID, false},
		} {
			got, err := repo.RequiresPathExists(ctx, tt.from, tt.to)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		}
	})

	t.Run("UpdateLevel changes the level", func(t *testing.T) {
		retained := domain.MasteryLevelRetained
		changed := needsPentatonic
		changed.Level = &retained

		require.NoError(t, repo.UpdateLevel(ctx, changed))

		got, err := repo.GetByID(ctx, needsPentatonic.ID)
		require.NoError(t, err)
		assert.Equal(t, changed, got)
	})

	t.Run("an unknown edge is not found", func(t *testing.T) {
		_, err := repo.GetByID(ctx, uuid.NewString())
		assert.ErrorIs(t, err, domain.ErrNotFound)
		assert.ErrorIs(t, repo.UpdateLevel(ctx, domain.KnowledgeEdge{ID: uuid.NewString(), Level: &fluent}), domain.ErrNotFound)
		assert.ErrorIs(t, repo.Delete(ctx, uuid.NewString()), domain.ErrNotFound)
	})

	t.Run("Delete removes the edge", func(t *testing.T) {
		require.NoError(t, repo.Delete(ctx, appliesBlues.ID))

		_, err := repo.GetByID(ctx, appliesBlues.ID)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}
