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

func TestAChordChangeIsATakeOfTenCyclesBetweenTwoChords(t *testing.T) {
	item := domain.ChordChangeItem{From: aMinor, To: cMajor, TargetTempoBPM: 60}

	assert.Equal(t, "chord_change:diagram-am:diagram-c", item.ItemKey())
	assert.Equal(t, 45, item.Seconds())
	assert.False(t, item.SuitsSession(false))

	planned := plannedItem(t, item, domain.DrillPlanning{})
	require.NotNil(t, planned.ChordChange)
	assert.Equal(t, domain.PlannedChordChange{
		FromDiagramID: "diagram-am", ToDiagramID: "diagram-c", FromChordSymbol: "Am", ToChordSymbol: "C",
		Take: domain.PlannedChangeTake{Cycles: 10, StartTempoBPM: 40, TargetTempoBPM: 60},
	}, *planned.ChordChange)
	assert.Equal(t, domain.PracticeRungPlayInTime, domain.ItemRung(planned, nil))
	assert.Empty(t, planned.DrillTemplateKey())
}

func TestAChangeTakeStartsOnTheLadder(t *testing.T) {
	best := 55
	for _, tc := range []struct {
		name string
		last *domain.ChangeTakeState
		want int
	}{
		{"never played: 40", nil, 40},
		{"from the last take's tempo", &domain.ChangeTakeState{TempoBPM: 55}, 55},
		{"one clean take keeps the tempo", &domain.ChangeTakeState{TempoBPM: 50, CleanInARow: 1}, 50},
		{"two clean takes in a row raise it by 5", &domain.ChangeTakeState{TempoBPM: 50, CleanInARow: 2}, 55},
		{"a struggled take lowers it by 5", &domain.ChangeTakeState{TempoBPM: 50, Struggled: true}, 45},
		{"never below 20", &domain.ChangeTakeState{TempoBPM: 20, Struggled: true}, 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := &domain.PracticeItemState{BestCleanBPM: &best, LastChangeTake: tc.last}
			planned := plannedItem(t, domain.ChordChangeItem{From: aMinor, To: cMajor, TargetTempoBPM: 60}, domain.DrillPlanning{State: state})

			assert.Equal(t, tc.want, planned.ChordChange.Take.StartTempoBPM)
			assert.Equal(t, &best, planned.ChordChange.Take.BestCleanTempoBPM)
		})
	}
}

// windowAt is a session's pick that starts a bass window at fret start,
// failing the test unless it is picked among the frets 0 to 9.
func windowAt(t *testing.T, start int) func(n int) int {
	t.Helper()
	return func(n int) int {
		assert.Equal(t, 10, n, "a bass window starts on one of the frets 0 to 9")
		return start
	}
}

func place(str, fret int) *domain.Place {
	return &domain.Place{String: str, Fret: fret}
}

func TestARootAndFifthIsMarkedInTheWindowTheSessionPicks(t *testing.T) {
	for _, tc := range []struct {
		name        string
		chord       string
		window      int
		root, fifth domain.Place
		fifthBelow  *domain.Place
	}{
		// A on the open A string, E at the D string's second fret, and the
		// open E string below.
		{"Am at the nut", "Am", 0, domain.Place{String: 3, Fret: 0}, domain.Place{String: 2, Fret: 2}, place(4, 0)},
		// A at the E string's fifth fret, E at the A string's seventh; the
		// open E string is outside frets 5 to 9.
		{"Am from fret 5", "Am", 5, domain.Place{String: 4, Fret: 5}, domain.Place{String: 3, Fret: 7}, nil},
		// C at the A string's third fret, G on the open G string, and the
		// G an octave lower at the E string's third fret.
		{"C at the nut", "C", 0, domain.Place{String: 3, Fret: 3}, domain.Place{String: 1, Fret: 0}, place(4, 3)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item, err := domain.NewRootFifthItem(standardBass, tc.chord)
			require.NoError(t, err)

			assert.Equal(t, "root_fifth:bass:"+tc.chord, item.ItemKey())
			assert.False(t, item.SuitsSession(false))
			planned := plannedItem(t, item, domain.DrillPlanning{Pick: windowAt(t, tc.window)})
			assert.Equal(t, &domain.PlannedRootFifth{
				LayoutInstrumentID: "bass", ChordSymbol: tc.chord, WindowStartFret: tc.window,
				Root: tc.root, Fifth: tc.fifth, FifthBelow: tc.fifthBelow, StartTempoBPM: 60,
			}, planned.RootFifth)
			assert.Equal(t, domain.PracticeRungPlayClean, domain.ItemRung(planned, nil))
		})
	}
}

func TestARootChangeMarksBothRootsInTheWindowTheSessionPicks(t *testing.T) {
	item, err := domain.NewRootChangeItem(standardBass, "Am", "C", 60)
	require.NoError(t, err)
	assert.Equal(t, "root_change:bass:Am:C", item.ItemKey())
	assert.Equal(t, 45, item.Seconds())

	for _, tc := range []struct {
		name     string
		window   int
		from, to domain.Place
	}{
		{"at the nut", 0, domain.Place{String: 3, Fret: 0}, domain.Place{String: 3, Fret: 3}},
		// Frets 7 to 11: A on the D string, C on the E string.
		{"from fret 7", 7, domain.Place{String: 2, Fret: 7}, domain.Place{String: 4, Fret: 8}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			planned := plannedItem(t, item, domain.DrillPlanning{Pick: windowAt(t, tc.window)})
			require.NotNil(t, planned.RootChange)
			assert.Equal(t, tc.window, planned.RootChange.WindowStartFret)
			assert.Equal(t, tc.from, planned.RootChange.FromRoot)
			assert.Equal(t, tc.to, planned.RootChange.ToRoot)
			assert.Equal(t, domain.PlannedChangeTake{Cycles: 10, StartTempoBPM: 40, TargetTempoBPM: 60}, planned.RootChange.Take)
			assert.Equal(t, domain.PracticeRungPlayInTime, domain.ItemRung(planned, nil))
		})
	}
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
		domain.ChordChangeItem{From: aMinor, To: cMajor, TargetTempoBPM: 60},
		rootFifth, rootChange,
	} {
		planning := domain.DrillPlanning{Pick: func(int) int { return 0 }}
		assert.NoError(t, domain.CheckChallenge(item.Plan(item.Ways()[0], planning).Stimulus()), item.ItemKey())
	}
}

func TestAMinuteCountedBeforeTakesReadsAsATake(t *testing.T) {
	for _, tc := range []struct {
		name                string
		tempo, count, clean int
		want                domain.ChangeTakeState
	}{
		{"60 percent of the beats reads as 50, a struggle", 50, 30, 0, domain.ChangeTakeState{TempoBPM: 50, Struggled: true}},
		{"96 percent reads as 75, almost", 50, 48, 2, domain.ChangeTakeState{TempoBPM: 50, CleanInARow: 2}},
		{"every beat reads as clean", 40, 40, 1, domain.ChangeTakeState{TempoBPM: 40, CleanInARow: 1}},
		{"a tempo of 0 reads as a struggle", 0, 10, 0, domain.ChangeTakeState{Struggled: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, domain.LegacyChangeTake(tc.tempo, tc.count, tc.clean))
		})
	}
}
