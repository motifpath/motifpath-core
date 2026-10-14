package application_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

const fakeDrillKind domain.PracticeItemKind = "fake_drill"

// fakeDrillItem is an item of a drill family the composer has never heard
// of, asked "first" or "second", recalled in the head.
type fakeDrillItem struct{ id string }

func (i fakeDrillItem) ItemKey() string             { return string(fakeDrillKind) + ":" + i.id }
func (fakeDrillItem) Kind() domain.PracticeItemKind { return fakeDrillKind }
func (fakeDrillItem) Ways() []string                { return []string{"first", "second"} }
func (fakeDrillItem) Seconds() int                  { return 12 }
func (fakeDrillItem) SuitsSession(inHand bool) bool { return !inHand }
func (i fakeDrillItem) Plan(way string, _ domain.DrillPlanning) domain.PlannedDrill {
	return fakePlannedDrill{way: way}
}

type fakePlannedDrill struct{ way string }

func (p fakePlannedDrill) Way() string                               { return p.way }
func (fakePlannedDrill) Stimulus() domain.DrillStimulus              { return domain.DrillStimulus{} }
func (p fakePlannedDrill) AttachTo(item *domain.PracticeSessionItem) { item.Kind = fakeDrillKind }

// fakeDrillSource offers count fake items under skillID.
type fakeDrillSource struct {
	skillID string
	count   int
}

func (s fakeDrillSource) SkillItems(_ context.Context, skillID, _ string, _ bool, _ application.KnowledgeMap) ([]domain.DrillItem, error) {
	if skillID != s.skillID {
		return nil, nil
	}
	items := make([]domain.DrillItem, s.count)
	for i := range items {
		items[i] = fakeDrillItem{id: fmt.Sprintf("%02d", i)}
	}
	return items, nil
}

func (s fakeDrillSource) Item(_ context.Context, itemKey string) (domain.DrillItem, bool, error) {
	id, ok := strings.CutPrefix(itemKey, string(fakeDrillKind)+":")
	return fakeDrillItem{id: id}, ok, nil
}

func TestPracticeSessionService_ANewDrillFamilyNeedsNoComposerChange(t *testing.T) {
	f := newPracticeFixture(t)
	f.skill("skill-fake")
	f.onPathFor([]string{practiceGuitar}, "skill-fake")
	f.svc.WithDrillSources(fakeDrillSource{skillID: "skill-fake", count: 4})
	// The knowledge map classifies the family's items, as it does cells and
	// shapes, so the student's states on them are read.
	for i := range 4 {
		key := fakeDrillItem{id: fmt.Sprintf("%02d", i)}.ItemKey()
		f.cells[practiceGuitar] = append(f.cells[practiceGuitar], domain.ClassifiedItem{ItemKey: key, NodeIDs: []string{"skill-fake"}})
	}
	answeredFirst := fakeDrillItem{id: "00"}.ItemKey()
	f.stateOf(answeredFirst, domain.PracticeItemState{
		Level: domain.KnowledgeLevelLearning, Counted: 2, Box: 1, DueAt: f.daysAgo(1),
		RightByResponse: map[string]int{"first": 2},
	})

	plan := f.composeInTheHead(t, 5)

	items := itemsOfKind(plan, fakeDrillKind)
	require.NotEmpty(t, items, "the composer offers the new family's items")
	assert.Equal(t, answeredFirst, items[0].ItemKey, "a due item comes first")
	assert.Equal(t, domain.PracticePickDue, items[0].Reason)
	for _, item := range items {
		assert.Equal(t, 12, item.EstimatedSeconds)
		require.NotNil(t, item.NodeID)
		assert.Equal(t, "skill-fake", *item.NodeID)
	}
}
