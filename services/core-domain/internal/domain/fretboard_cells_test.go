package domain_test

import (
	"testing"

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
