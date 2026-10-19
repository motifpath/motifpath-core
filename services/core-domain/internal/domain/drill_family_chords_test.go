package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

var (
	aMinor = domain.ChordDrill{ID: "drill-am", DiagramID: "diagram-am", Chord: "Am", Ordinal: 6}
	cMajor = domain.ChordDrill{ID: "drill-c", DiagramID: "diagram-c", Chord: "C", Ordinal: 1}
	// standardBass is a four-string bass in standard tuning, lowest string first.
	standardBass = domain.Instrument{ID: "bass", Tuning: []string{"E1", "A1", "D2", "G2"}}
)

func plannedItem(t *testing.T, item domain.DrillItem, planning domain.DrillPlanning) domain.PracticeSessionItem {
	t.Helper()
	var planned domain.PracticeSessionItem
	item.Plan(item.Ways()[0], planning).AttachTo(&planned)
	return planned
}

func TestAChordVoicingIsASingleChordPlayedInHand(t *testing.T) {
	item := domain.ChordVoicingItem{Drill: aMinor}

	assert.Equal(t, "chord_voicing:diagram-am", item.ItemKey())
	assert.Equal(t, domain.PracticeItemKindChordVoicing, item.Kind())
	assert.Equal(t, 20, item.Seconds())
	assert.True(t, item.SuitsSession(true))
	assert.False(t, item.SuitsSession(false), "a chord is played, never recalled in the head")

	planned := plannedItem(t, item, domain.DrillPlanning{})
	assert.Equal(t, domain.PracticeItemKindChordVoicing, planned.Kind)
	assert.Equal(t, &domain.PlannedChordVoicing{DiagramID: "diagram-am", ChordSymbol: "Am"}, planned.ChordVoicing)
	assert.Equal(t, domain.PracticeRungPlayClean, domain.ItemRung(planned, nil))
	assert.Empty(t, planned.DrillTemplateKey(), "a rated chord practises no timed template")
}

func TestAChordChangeIsAMinuteBetweenTwoChords(t *testing.T) {
	item := domain.ChordChangeItem{From: aMinor, To: cMajor, TargetChangesPerMinute: 60}

	assert.Equal(t, "chord_change:diagram-am:diagram-c", item.ItemKey())
	assert.Equal(t, 90, item.Seconds())
	assert.False(t, item.SuitsSession(false))

	planned := plannedItem(t, item, domain.DrillPlanning{})
	require.NotNil(t, planned.ChordChange)
	assert.Equal(t, domain.PlannedChordChange{
		FromDiagramID: "diagram-am", ToDiagramID: "diagram-c", FromChordSymbol: "Am", ToChordSymbol: "C",
		Minute: domain.PlannedChangeMinute{StartTempoBPM: 40, TargetChangesPerMinute: 60},
	}, *planned.ChordChange)
	assert.Equal(t, domain.PracticeRungPlayInTime, domain.ItemRung(planned, nil))
	assert.Empty(t, planned.DrillTemplateKey())
}

func TestAChangeMinuteStartsOnTheLadder(t *testing.T) {
	best := 38
	for _, tc := range []struct {
		name string
		last *domain.ChangeMinuteState
		want int
	}{
		{"never played: 40", nil, 40},
		{"from the last minute's tempo", &domain.ChangeMinuteState{TempoBPM: 55, ChangesPerMinute: 40}, 55},
		{"one clean minute keeps the tempo", &domain.ChangeMinuteState{TempoBPM: 50, ChangesPerMinute: 48, CleanInARow: 1}, 50},
		{"two clean minutes in a row raise it by 5", &domain.ChangeMinuteState{TempoBPM: 50, ChangesPerMinute: 48, CleanInARow: 2}, 55},
		{"a struggled minute lowers it by 5", &domain.ChangeMinuteState{TempoBPM: 50, ChangesPerMinute: 20, Struggled: true}, 45},
		{"never below 20", &domain.ChangeMinuteState{TempoBPM: 20, ChangesPerMinute: 5, Struggled: true}, 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := &domain.PracticeItemState{BestChangesPerMinute: &best, LastChangeMinute: tc.last}
			planned := plannedItem(t, domain.ChordChangeItem{From: aMinor, To: cMajor, TargetChangesPerMinute: 60}, domain.DrillPlanning{State: state})

			assert.Equal(t, tc.want, planned.ChordChange.Minute.StartTempoBPM)
			assert.Equal(t, &best, planned.ChordChange.Minute.BestCleanChangesPerMinute)
			if tc.last != nil {
				assert.Equal(t, tc.last.ChangesPerMinute, *planned.ChordChange.Minute.LastChangesPerMinute)
			} else {
				assert.Nil(t, planned.ChordChange.Minute.LastChangesPerMinute)
			}
		})
	}
}

func TestARootAndFifthIsMarkedAtTheLowestFretThenTheLowestString(t *testing.T) {
	for _, tc := range []struct {
		chord       string
		root, fifth domain.Place
	}{
		// A on the open A string, E on the open E string.
		{"Am", domain.Place{String: 3, Fret: 0}, domain.Place{String: 4, Fret: 0}},
		// C at the A string's third fret; G on the open G string.
		{"C", domain.Place{String: 3, Fret: 3}, domain.Place{String: 1, Fret: 0}},
	} {
		t.Run(tc.chord, func(t *testing.T) {
			item, err := domain.NewRootFifthItem(standardBass, tc.chord)
			require.NoError(t, err)

			assert.Equal(t, "root_fifth:bass:"+tc.chord, item.ItemKey())
			assert.False(t, item.SuitsSession(false))
			planned := plannedItem(t, item, domain.DrillPlanning{})
			assert.Equal(t, &domain.PlannedRootFifth{LayoutInstrumentID: "bass", ChordSymbol: tc.chord, Root: tc.root, Fifth: tc.fifth}, planned.RootFifth)
			assert.Equal(t, domain.PracticeRungPlayClean, domain.ItemRung(planned, nil))
		})
	}
}

func TestARootChangeMarksBothRoots(t *testing.T) {
	item, err := domain.NewRootChangeItem(standardBass, "Am", "C", 60)
	require.NoError(t, err)

	assert.Equal(t, "root_change:bass:Am:C", item.ItemKey())
	planned := plannedItem(t, item, domain.DrillPlanning{})
	require.NotNil(t, planned.RootChange)
	assert.Equal(t, domain.Place{String: 3, Fret: 0}, planned.RootChange.FromRoot)
	assert.Equal(t, domain.Place{String: 3, Fret: 3}, planned.RootChange.ToRoot)
	assert.Equal(t, 40, planned.RootChange.Minute.StartTempoBPM)
	assert.Equal(t, domain.PracticeRungPlayInTime, domain.ItemRung(planned, nil))
}

func TestARootAndFifthOfAChordThatIsntOneIsRefused(t *testing.T) {
	_, err := domain.NewRootFifthItem(standardBass, "H")
	assert.Error(t, err)
}

func TestChordItemsMarkNoAnswer(t *testing.T) {
	rootFifth, err := domain.NewRootFifthItem(standardBass, "Am")
	require.NoError(t, err)
	rootChange, err := domain.NewRootChangeItem(standardBass, "Am", "C", 60)
	require.NoError(t, err)
	for _, item := range []domain.DrillItem{
		domain.ChordVoicingItem{Drill: aMinor},
		domain.ChordChangeItem{From: aMinor, To: cMajor, TargetChangesPerMinute: 60},
		rootFifth, rootChange,
	} {
		assert.NoError(t, domain.CheckChallenge(item.Plan(item.Ways()[0], domain.DrillPlanning{}).Stimulus()), item.ItemKey())
	}
}
