package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

const (
	guitarLayout = "6ea2d087-ab9c-59dc-9657-8546025414d2"
	bassLayout   = "14fe11ad-efdb-589a-b713-2e81ec041cbe"
	rootStrings  = "11111111-1111-4111-8111-111111111111"
)

func sixString() domain.Instrument {
	six := 6
	return domain.Instrument{ID: guitarLayout, Family: domain.InstrumentFamilyFretted, StringCount: &six,
		Tuning: []string{"E2", "A2", "D3", "G3", "B3", "E4"}}
}

func fourString() domain.Instrument {
	four := 4
	return domain.Instrument{ID: bassLayout, Family: domain.InstrumentFamilyFretted, StringCount: &four,
		Tuning: []string{"E1", "A1", "D2", "G2"}}
}

func TestFretboardCellItemKeyNamesTheLayoutStringAndFret(t *testing.T) {
	assert.Equal(t, "fretboard_cell:"+guitarLayout+":5:3", domain.FretboardCellItemKey(guitarLayout, 5, 3))
}

func TestARangeYieldsOneItemPerStringAndFretUnderItsSkill(t *testing.T) {
	r := domain.FretboardCellRange{SkillID: rootStrings, LayoutInstrumentID: guitarLayout, Strings: []int{6, 5}, FromFret: 0, ToFret: 11}

	items := r.Items()

	require.Len(t, items, 24)
	assert.Equal(t, domain.FretboardCellItemKey(guitarLayout, 6, 0), items[0].ItemKey)
	assert.Equal(t, domain.FretboardCellItemKey(guitarLayout, 5, 11), items[23].ItemKey)
	for _, item := range items {
		assert.Equal(t, []string{rootStrings}, item.NodeIDs)
	}
}

func TestARangeFitsALayoutWithItsStringsAndASkillForIt(t *testing.T) {
	r := domain.FretboardCellRange{SkillID: rootStrings, LayoutInstrumentID: bassLayout, Strings: []int{4, 3}, FromFret: 0, ToFret: 11}
	skill := domain.KnowledgeNode{ID: rootStrings, InstrumentIDs: []string{guitarLayout, bassLayout}}

	assert.NoError(t, r.CheckFits(fourString(), skill))
}

func TestARangeOnAStringTheLayoutDoesntHaveDoesntFit(t *testing.T) {
	r := domain.FretboardCellRange{SkillID: rootStrings, LayoutInstrumentID: bassLayout, Strings: []int{2, 5}, FromFret: 0, ToFret: 11}
	skill := domain.KnowledgeNode{ID: rootStrings}

	err := r.CheckFits(fourString(), skill)

	require.ErrorIs(t, err, domain.ErrInvalidDrillCatalog)
	assert.ErrorContains(t, err, bassLayout)
	assert.ErrorContains(t, err, "string 5")
}

func TestARangeForASkillThatDoesntSuitTheLayoutDoesntFit(t *testing.T) {
	r := domain.FretboardCellRange{SkillID: rootStrings, LayoutInstrumentID: bassLayout, Strings: []int{4}, FromFret: 0, ToFret: 11}
	guitarsOnly := domain.KnowledgeNode{ID: rootStrings, InstrumentIDs: []string{guitarLayout}}

	err := r.CheckFits(fourString(), guitarsOnly)

	require.ErrorIs(t, err, domain.ErrInvalidDrillCatalog)
	assert.ErrorContains(t, err, rootStrings)
	assert.ErrorContains(t, err, bassLayout)
}

func TestARangeForAnotherLayoutDoesntFit(t *testing.T) {
	r := domain.FretboardCellRange{SkillID: rootStrings, LayoutInstrumentID: bassLayout, Strings: []int{4}, FromFret: 0, ToFret: 11}

	assert.ErrorIs(t, r.CheckFits(sixString(), domain.KnowledgeNode{ID: rootStrings}), domain.ErrInvalidDrillCatalog)
}

func TestARangeWithFretsOutOfOrderDoesntFit(t *testing.T) {
	r := domain.FretboardCellRange{SkillID: rootStrings, LayoutInstrumentID: guitarLayout, Strings: []int{6}, FromFret: 5, ToFret: 2}

	assert.ErrorIs(t, r.CheckFits(sixString(), domain.KnowledgeNode{ID: rootStrings}), domain.ErrInvalidDrillCatalog)
}

func TestInstrumentsWithTheSameStringsAndTuningShareGeometry(t *testing.T) {
	electric := sixString()
	electric.ID = "e6fac4f3-7d52-5f46-8f44-4de1b239ebdd"

	assert.True(t, electric.SameGeometry(sixString()))
	assert.False(t, fourString().SameGeometry(sixString()))
}

func TestAFretboardCellItemKeyParsesBackToItsLayoutStringAndFret(t *testing.T) {
	cell, ok := domain.ParseFretboardCellItemKey(domain.FretboardCellItemKey(guitarLayout, 5, 3))

	require.True(t, ok)
	assert.Equal(t, domain.FretboardCell{LayoutInstrumentID: guitarLayout, String: 5, Fret: 3}, cell)
}

func TestOnlyAFretboardCellItemKeyParsesAsACell(t *testing.T) {
	for _, key := range []string{
		domain.ExerciseItemKey("e1"),
		domain.PlayAlongItemKey("d1"),
		"fretboard_cell:" + guitarLayout + ":5",
		"fretboard_cell:" + guitarLayout + ":five:3",
		"fretboard_cell:" + guitarLayout + ":0:3",
		"fretboard_cell:" + guitarLayout + ":5:-1",
		"fretboard_cell::5:3",
	} {
		_, ok := domain.ParseFretboardCellItemKey(key)

		assert.False(t, ok, key)
	}
}

func TestAFretboardMapHasEveryCellOnceByStringThenFretWithTheStudentsLevel(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	key := func(str, fret int) string { return domain.FretboardCellItemKey(guitarLayout, str, fret) }
	items := []domain.ClassifiedItem{
		{ItemKey: key(5, 1), NodeIDs: []string{rootStrings}},
		{ItemKey: domain.ExerciseItemKey("e1"), NodeIDs: []string{rootStrings}},
		{ItemKey: key(1, 0), NodeIDs: []string{"top"}},
		{ItemKey: key(5, 0), NodeIDs: []string{rootStrings}},
		{ItemKey: key(5, 0), NodeIDs: []string{"wide"}},
	}
	notDue, yesterday, longAgo := now.AddDate(0, 0, 2), now.AddDate(0, 0, -1), now.AddDate(0, 0, -30)
	states := map[string]domain.PracticeItemState{
		key(5, 0): {ItemKey: key(5, 0), RulesVersion: domain.PracticeRulesVersion, Level: domain.KnowledgeLevelAccurate, Counted: 3, Box: 2, DueAt: &notDue},
		key(5, 1): {ItemKey: key(5, 1), RulesVersion: domain.PracticeRulesVersion, Level: domain.KnowledgeLevelFluent, Counted: 6, Box: 4, DueAt: &yesterday},
		key(1, 0): {ItemKey: key(1, 0), RulesVersion: domain.PracticeRulesVersion, Level: domain.KnowledgeLevelFluent, Counted: 6, Box: 4, DueAt: &longAgo},
	}

	got := domain.NewFretboardMap("electric-guitar", items, states, now)

	assert.Equal(t, "electric-guitar", got.InstrumentID)
	require.NotNil(t, got.LayoutInstrumentID)
	assert.Equal(t, guitarLayout, *got.LayoutInstrumentID)
	assert.Equal(t, []domain.FretboardMapCell{
		{FretboardCell: domain.FretboardCell{LayoutInstrumentID: guitarLayout, String: 1, Fret: 0}, ItemKey: key(1, 0), Level: domain.KnowledgeLevelAccurate, Fading: true},
		{FretboardCell: domain.FretboardCell{LayoutInstrumentID: guitarLayout, String: 5, Fret: 0}, ItemKey: key(5, 0), Level: domain.KnowledgeLevelAccurate, Fading: false},
		{FretboardCell: domain.FretboardCell{LayoutInstrumentID: guitarLayout, String: 5, Fret: 1}, ItemKey: key(5, 1), Level: domain.KnowledgeLevelFluent, Fading: true},
	}, got.Cells)
}

func TestAFretboardCellNeverPractisedIsNewAndNotFading(t *testing.T) {
	key := domain.FretboardCellItemKey(guitarLayout, 6, 3)

	got := domain.NewFretboardMap(guitarLayout, []domain.ClassifiedItem{{ItemKey: key}}, nil, time.Now())

	require.Len(t, got.Cells, 1)
	assert.Equal(t, domain.KnowledgeLevelNew, got.Cells[0].Level)
	assert.False(t, got.Cells[0].Fading)
}

func TestAFretboardMapWithoutCellsHasNoLayout(t *testing.T) {
	got := domain.NewFretboardMap("piano", []domain.ClassifiedItem{{ItemKey: domain.ExerciseItemKey("e1")}}, nil, time.Now())

	assert.Nil(t, got.LayoutInstrumentID)
	assert.Empty(t, got.Cells)
	assert.NotNil(t, got.Cells)
}
