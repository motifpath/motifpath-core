package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// fakeNodeItemSource is an in-memory ports.NodeItemSource holding the
// classified items for each instrument.
type fakeNodeItemSource struct {
	items map[string][]domain.ClassifiedItem
	err   error
}

func (f *fakeNodeItemSource) ClassifiedItems(_ context.Context, instrumentID string) ([]domain.ClassifiedItem, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.items[instrumentID], nil
}

func TestKnowledgeRollupService_Standings(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	const alice = "alice"

	fretboard := domain.KnowledgeNode{ID: "fretboard", Kind: domain.KnowledgeNodeKindSkill, Key: "fretboard-knowledge", InstrumentIDs: []string{practiceGuitar, practiceBass}}
	low := domain.KnowledgeNode{ID: "low", Kind: domain.KnowledgeNodeKindSkill, Key: "notes-on-low-strings", ParentID: &fretboard.ID, InstrumentIDs: []string{practiceGuitar, practiceBass}}
	all := domain.KnowledgeNode{ID: "all", Kind: domain.KnowledgeNodeKindSkill, Key: "notes-on-all-strings", InstrumentIDs: []string{practiceGuitar}}
	intervals := domain.KnowledgeNode{ID: "intervals", Kind: domain.KnowledgeNodeKindConcept, Key: "intervals"}
	reading := domain.KnowledgeNode{ID: "reading", Kind: domain.KnowledgeNodeKindSkill, Key: "music-reading"}
	slap := domain.KnowledgeNode{ID: "slap", Kind: domain.KnowledgeNodeKindSkill, Key: "slap", InstrumentIDs: []string{practiceBass}}

	accurate := func(key string) domain.PracticeItemState {
		due := now.AddDate(0, 0, 3)
		return domain.PracticeItemState{ItemKey: key, RulesVersion: domain.PracticeRulesVersion, Level: domain.KnowledgeLevelAccurate, Counted: 3, Box: 2, DueAt: &due}
	}

	type fixture struct {
		svc    *application.KnowledgeRollupService
		states *fakePracticeItemStateReader
		items  *fakeNodeItemSource
		edges  *fakeKnowledgeEdgeRepository
	}
	setup := func(t *testing.T) fixture {
		t.Helper()
		nodes := newFakeKnowledgeNodeRepository()
		for _, n := range []domain.KnowledgeNode{fretboard, low, all, intervals, reading, slap} {
			nodes.put(n)
		}
		f := fixture{
			states: newFakePracticeItemStateReader(),
			items:  &fakeNodeItemSource{items: map[string][]domain.ClassifiedItem{}},
			edges:  newFakeKnowledgeEdgeRepository(),
		}
		f.svc = application.NewKnowledgeRollupService(nodes, f.edges, f.items, f.states, func() time.Time { return now })
		return f
	}
	exercisesForEveryInstrument := func(f fixture, nodeID string, keys ...string) {
		for _, instrument := range []string{practiceGuitar, practiceBass} {
			for _, k := range keys {
				f.items.items[instrument] = append(f.items.items[instrument], domain.ClassifiedItem{ItemKey: k, NodeIDs: []string{nodeID}})
			}
		}
	}

	t.Run("items for every instrument count toward every instrument's level", func(t *testing.T) {
		f := setup(t)
		keys := []string{"exercise:1", "exercise:2", "exercise:3", "exercise:4", "exercise:5"}
		exercisesForEveryInstrument(f, intervals.ID, keys...)
		for _, k := range keys {
			f.states.put(alice, accurate(k))
		}

		for _, instrument := range []string{practiceGuitar, practiceBass} {
			got, err := f.svc.Standings(ctx, alice, instrument)

			require.NoError(t, err)
			require.NotNil(t, got[intervals.ID].Level, instrument)
			assert.Equal(t, domain.KnowledgeLevelAccurate, *got[intervals.ID].Level, instrument)
		}
	})

	t.Run("a parent rolls up the items of its children, each counted once", func(t *testing.T) {
		f := setup(t)
		f.items.items[practiceGuitar] = []domain.ClassifiedItem{
			{ItemKey: "play_along:1", NodeIDs: []string{low.ID, fretboard.ID}},
			{ItemKey: "play_along:2", NodeIDs: []string{low.ID}},
		}
		f.states.put(alice, accurate("play_along:1"))

		got, err := f.svc.Standings(ctx, alice, practiceGuitar)

		require.NoError(t, err)
		assert.Equal(t, 2, got[fretboard.ID].Total)
		assert.Equal(t, 1, got[fretboard.ID].Covered)
	})

	t.Run("only nodes for the instrument have a standing", func(t *testing.T) {
		f := setup(t)

		got, err := f.svc.Standings(ctx, alice, practiceGuitar)

		require.NoError(t, err)
		assert.Contains(t, got, all.ID)
		assert.Contains(t, got, intervals.ID)
		assert.NotContains(t, got, slap.ID)
	})

	t.Run("a node with nothing to practise has no level", func(t *testing.T) {
		f := setup(t)

		got, err := f.svc.Standings(ctx, alice, practiceGuitar)

		require.NoError(t, err)
		assert.Nil(t, got[reading.ID].Level)
	})

	t.Run("readiness and requires depth come from the requires edges for the instrument", func(t *testing.T) {
		f := setup(t)
		level := domain.MasteryLevelAccurate
		require.NoError(t, f.edges.Create(ctx, domain.KnowledgeEdge{ID: "e1", FromID: all.ID, ToID: low.ID, Type: domain.KnowledgeEdgeTypeRequires, Level: &level}))
		require.NoError(t, f.edges.Create(ctx, domain.KnowledgeEdge{ID: "e2", FromID: all.ID, ToID: reading.ID, Type: domain.KnowledgeEdgeTypeRequires, Level: &level}))
		require.NoError(t, f.edges.Create(ctx, domain.KnowledgeEdge{ID: "e3", FromID: low.ID, ToID: intervals.ID, Type: domain.KnowledgeEdgeTypeRequires, Level: &level}))
		f.items.items[practiceGuitar] = []domain.ClassifiedItem{{ItemKey: "play_along:1", NodeIDs: []string{low.ID}}}
		f.states.put(alice, accurate("play_along:1"))

		got, err := f.svc.Standings(ctx, alice, practiceGuitar)

		require.NoError(t, err)
		assert.Equal(t, domain.Readiness{Met: 1, Total: 2}, got[all.ID].Readiness)
		assert.Equal(t, 2, got[all.ID].RequiresDepth)
		assert.Equal(t, 1, got[low.ID].RequiresDepth)
	})

	t.Run("the map lists the nodes for the instrument in catalog order, with their subtree items and the student's states", func(t *testing.T) {
		f := setup(t)
		f.items.items[practiceGuitar] = []domain.ClassifiedItem{
			{ItemKey: "play_along:1", NodeIDs: []string{low.ID}},
			{ItemKey: "exercise:1", NodeIDs: []string{intervals.ID}},
		}
		f.states.put(alice, accurate("play_along:1"))

		got, err := f.svc.Map(ctx, alice, practiceGuitar)

		require.NoError(t, err)
		ids := make([]string, len(got.Nodes))
		for i, n := range got.Nodes {
			ids[i] = n.ID
		}
		assert.Equal(t, []string{fretboard.ID, intervals.ID, reading.ID, all.ID, low.ID}, ids, "sorted by key, without the bass-only slap")
		assert.Equal(t, []string{"play_along:1"}, got.Subtrees[fretboard.ID])
		assert.Equal(t, []string{"exercise:1"}, got.Subtrees[intervals.ID])
		assert.Equal(t, map[string]domain.PracticeItemState{"play_along:1": accurate("play_along:1")}, got.States)
		require.NotNil(t, got.Standings[low.ID].Level)
		assert.Equal(t, domain.KnowledgeLevelAccurate, *got.Standings[low.ID].Level)
	})

	t.Run("the map carries the applies edges, apart from the requires edges", func(t *testing.T) {
		f := setup(t)
		level := domain.MasteryLevelAccurate
		applies := domain.KnowledgeEdge{ID: "e1", FromID: all.ID, ToID: intervals.ID, Type: domain.KnowledgeEdgeTypeApplies}
		require.NoError(t, f.edges.Create(ctx, applies))
		require.NoError(t, f.edges.Create(ctx, domain.KnowledgeEdge{ID: "e2", FromID: all.ID, ToID: low.ID, Type: domain.KnowledgeEdgeTypeRequires, Level: &level}))

		got, err := f.svc.Map(ctx, alice, practiceGuitar)

		require.NoError(t, err)
		assert.Equal(t, []domain.KnowledgeEdge{applies}, got.Applies)
		assert.Equal(t, 1, got.Standings[all.ID].Readiness.Total, "an applies edge is never a requirement")
	})

	t.Run("a failure listing items or reading states fails the request", func(t *testing.T) {
		f := setup(t)
		f.items.err = errors.New("db down")

		_, err := f.svc.Standings(ctx, alice, practiceGuitar)
		require.Error(t, err)

		f = setup(t)
		f.items.items[practiceGuitar] = []domain.ClassifiedItem{{ItemKey: "play_along:1", NodeIDs: []string{low.ID}}}
		f.states.err = errors.New("mongo down")

		_, err = f.svc.Standings(ctx, alice, practiceGuitar)
		require.Error(t, err)
	})
}
