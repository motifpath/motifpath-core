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

func TestAFretboardCellIsAskedTheWayItHasFewerRightAnswersNamingTheNoteOnATie(t *testing.T) {
	tests := []struct {
		name  string
		state *domain.PracticeItemState
		want  domain.FretboardDrill
	}{
		{"never practised", nil, domain.FretboardDrillNameTheNote},
		{"practised, never right", &domain.PracticeItemState{Counted: 2}, domain.FretboardDrillNameTheNote},
		{"named 4 times, found once", &domain.PracticeItemState{RightByResponse: map[string]int{"name_the_note": 4, "find_the_note": 1}}, domain.FretboardDrillFindTheNote},
		{"found more often than named", &domain.PracticeItemState{RightByResponse: map[string]int{"name_the_note": 1, "find_the_note": 3}}, domain.FretboardDrillNameTheNote},
		{"named and found as often", &domain.PracticeItemState{RightByResponse: map[string]int{"name_the_note": 2, "find_the_note": 2}}, domain.FretboardDrillNameTheNote},
		{"only named so far", &domain.PracticeItemState{RightByResponse: map[string]int{"name_the_note": 1}}, domain.FretboardDrillFindTheNote},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, domain.NextFretboardDrill(tt.state))
		})
	}
}

func TestAFretboardCellTakesAbout8Seconds(t *testing.T) {
	assert.Equal(t, 8, domain.FretboardCellSeconds)
}

func TestAFretWindowAtTheNutHoldsTheOpenStringAndFrets1To5(t *testing.T) {
	nut := domain.FretWindow{FromFret: 0}
	for fret := 0; fret <= 5; fret++ {
		assert.True(t, nut.Contains(fret), "fret %d", fret)
	}
	assert.False(t, nut.Contains(6))
	assert.Equal(t, 5, nut.ToFret())
}

func TestAFretWindowAboveTheNutHoldsFiveFrets(t *testing.T) {
	w := domain.FretWindow{FromFret: 3}

	assert.False(t, w.Contains(2))
	assert.True(t, w.Contains(3))
	assert.True(t, w.Contains(7))
	assert.False(t, w.Contains(8))
	assert.Equal(t, 7, w.ToFret())
}

func TestTheFretWindowsRunFromTheNutTo7To11(t *testing.T) {
	var froms []int
	for _, w := range domain.FretWindows() {
		froms = append(froms, w.FromFret)
	}
	assert.Equal(t, []int{0, 2, 3, 4, 5, 6, 7}, froms)
}

func TestADrillsFretWindowIsPickedByItsDueThenWeakThenNewCellsNearestTheNutOnATie(t *testing.T) {
	tests := []struct {
		name            string
		due, weak, news []int
		newTurn         bool
		want            int
	}{
		{name: "nothing at all stays at the nut", want: 0},
		{name: "only new cells start at the nut", news: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}, want: 0},
		{name: "the most due cells, nearest the nut on a tie", due: []int{7, 7, 8, 8, 9, 9, 1, 2}, want: 5},
		{name: "with nothing due, the most weak cells", weak: []int{8, 9, 10, 8, 9}, want: 6},
		{name: "due cells outweigh more weak cells elsewhere", due: []int{1}, weak: []int{9, 9, 9}, want: 0},
		{name: "weak cells break a tie on due cells", due: []int{5}, weak: []int{8, 9}, want: 5},
		{name: "new cells break a tie on due and weak cells", due: []int{6}, news: []int{9, 10}, want: 6},
		{name: "a new-cell turn takes the most new cells", due: []int{0, 1, 2, 3, 4, 5}, news: []int{6, 7, 8, 9, 10, 11}, newTurn: true, want: 6},
		{name: "a new-cell turn without new cells keeps the due cells", due: []int{1, 2, 3, 9}, newTurn: true, want: 0},
		{name: "off a new-cell turn the due cells win again", due: []int{0, 1, 2, 3, 4, 5}, news: []int{6, 7, 8, 9, 10, 11}, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.PickFretWindow(tt.due, tt.weak, tt.news, tt.newTurn)

			assert.Equal(t, domain.FretWindow{FromFret: tt.want}, got)
		})
	}
}
