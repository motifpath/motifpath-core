package application_test

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

// chordCatalog is eight open chords, every pair of neighbours a change, and a
// change target of 60 clean changes a minute in force since before the
// fixture's now.
func chordCatalog() domain.ChordDrillCatalog {
	chords := []string{"C", "A", "G", "E", "D", "Am", "Em", "Dm"}
	var catalog domain.ChordDrillCatalog
	for i, chord := range chords {
		catalog.Drills = append(catalog.Drills, domain.ChordDrill{ID: "drill-" + chord, DiagramID: "voicing-" + chord, Chord: chord, Ordinal: i + 1})
	}
	for i := range len(chords) - 1 {
		for j := i + 1; j < len(chords); j++ {
			catalog.Changes = append(catalog.Changes, domain.ChordChangePair{
				ID: fmt.Sprintf("pair-%d-%d", i, j), FromChordDrillID: "drill-" + chords[i], ToChordDrillID: "drill-" + chords[j], Ordinal: len(catalog.Changes) + 1,
			})
		}
	}
	catalog.Targets = []domain.ChangeTarget{{ID: "target-1", Version: 1, EffectiveFrom: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), ChangesPerMinute: 60}}
	return catalog
}

func voicingKey(chord string) string { return "chord_voicing:voicing-" + chord }
func changeKey(from, to string) string {
	return "chord_change:voicing-" + from + ":voicing-" + to
}

// chordFixture is a practice fixture with the chord drills installed: the
// single chords under play-open-chords and the changes under change-chords,
// both on the student's guitar path, and on bass the same chords' roots and
// fifths under play-root-fifth-lines and their root changes under
// play-root-notes. pairs limits the changes classified, all when zero.
func chordFixture(t *testing.T, chords []string, pairs [][2]string) *practiceFixture {
	f := newPracticeFixture(t)
	f.instruments.put(domain.Instrument{ID: practiceBass, Tuning: []string{"E1", "A1", "D2", "G2"}})
	for _, skill := range []string{"play-open-chords", "change-chords", "play-root-fifth-lines", "play-root-notes"} {
		f.skill(skill)
	}
	f.onPathFor([]string{practiceGuitar}, "play-open-chords", "change-chords")
	f.onPathFor([]string{practiceBass}, "play-root-fifth-lines", "play-root-notes")
	for _, chord := range chords {
		f.cells[practiceGuitar] = append(f.cells[practiceGuitar], domain.ClassifiedItem{ItemKey: voicingKey(chord), NodeIDs: []string{"play-open-chords"}})
		f.cells[practiceBass] = append(f.cells[practiceBass], domain.ClassifiedItem{ItemKey: "root_fifth:" + practiceBass + ":" + chord, NodeIDs: []string{"play-root-fifth-lines"}})
	}
	for _, p := range pairs {
		f.cells[practiceGuitar] = append(f.cells[practiceGuitar], domain.ClassifiedItem{ItemKey: changeKey(p[0], p[1]), NodeIDs: []string{"change-chords"}})
		f.cells[practiceBass] = append(f.cells[practiceBass], domain.ClassifiedItem{ItemKey: "root_change:" + practiceBass + ":" + p[0] + ":" + p[1], NodeIDs: []string{"play-root-notes"}})
	}
	f.svc.WithChordDrills(fakeChordDrills(chordCatalog()), f.instruments)
	return f
}

func TestPracticeSessionService_ComposePlanChordDrills(t *testing.T) {
	t.Run("an in-hand guitar session plays new single chords, named, on the play_clean rung", func(t *testing.T) {
		f := chordFixture(t, []string{"Am", "C"}, nil)

		plan := f.compose(t, practiceGuitar, 10)

		chords := itemsOfKind(plan, domain.PracticeItemKindChordVoicing)
		require.Len(t, chords, 2)
		assert.Equal(t, &domain.PlannedChordVoicing{DiagramID: "voicing-Am", ChordSymbol: "Am"}, chords[0].ChordVoicing)
		for _, item := range chords {
			assert.Equal(t, domain.PracticePickNew, item.Reason)
			assert.Equal(t, domain.PracticeRungPlayClean, item.Rung)
			assert.Equal(t, 20, item.EstimatedSeconds)
		}
	})

	t.Run("a chord change is planned with both chords and its minute, on the play_in_time rung", func(t *testing.T) {
		f := chordFixture(t, nil, [][2]string{{"Am", "C"}})

		plan := f.compose(t, practiceGuitar, 10)

		changes := itemsOfKind(plan, domain.PracticeItemKindChordChange)
		require.Len(t, changes, 1)
		assert.Equal(t, &domain.PlannedChordChange{
			FromDiagramID: "voicing-Am", ToDiagramID: "voicing-C", FromChordSymbol: "Am", ToChordSymbol: "C",
			Minute: domain.PlannedChangeMinute{StartTempoBPM: 40, TargetChangesPerMinute: 60},
		}, changes[0].ChordChange)
		assert.Equal(t, domain.PracticeRungPlayInTime, changes[0].Rung)
		assert.Equal(t, 90, changes[0].EstimatedSeconds)
	})

	t.Run("a change starts from its last minute, up 5 after two clean minutes in a row", func(t *testing.T) {
		f := chordFixture(t, nil, [][2]string{{"Am", "C"}})
		best := 38
		f.stateOf(changeKey("Am", "C"), domain.PracticeItemState{
			Level: domain.KnowledgeLevelLearning, Counted: 2, Box: 1, DueAt: f.daysAgo(1), BestChangesPerMinute: &best,
			LastChangeMinute: &domain.ChangeMinuteState{TempoBPM: 50, ChangesPerMinute: 48, CleanInARow: 2},
		})

		plan := f.compose(t, practiceGuitar, 10)

		changes := itemsOfKind(plan, domain.PracticeItemKindChordChange)
		require.Len(t, changes, 1)
		minute := changes[0].ChordChange.Minute
		assert.Equal(t, 55, minute.StartTempoBPM)
		assert.Equal(t, &best, minute.BestCleanChangesPerMinute)
		require.NotNil(t, minute.LastChangesPerMinute)
		assert.Equal(t, 48, *minute.LastChangesPerMinute)
	})

	t.Run("a session plays at most 4 single chords and 2 changes, and may end early", func(t *testing.T) {
		chords := []string{"C", "A", "G", "E", "D", "Am", "Em", "Dm"}
		var pairs [][2]string
		for i := range len(chords) - 1 {
			pairs = append(pairs, [2]string{chords[i], chords[i+1]})
		}
		f := chordFixture(t, chords, pairs)

		plan := f.compose(t, practiceGuitar, 30)

		assert.Len(t, itemsOfKind(plan, domain.PracticeItemKindChordVoicing), 4)
		assert.Len(t, itemsOfKind(plan, domain.PracticeItemKindChordChange), 2)
	})

	t.Run("on guitar, a single chord comes before a change that uses it", func(t *testing.T) {
		f := chordFixture(t, []string{"Am"}, [][2]string{{"Am", "C"}})
		// Due, the change would otherwise come before the new chord.
		f.stateOf(changeKey("Am", "C"), domain.PracticeItemState{Level: domain.KnowledgeLevelLearning, Counted: 2, Box: 1, DueAt: f.daysAgo(1)})

		plan := f.compose(t, practiceGuitar, 10)

		keys := make([]string, len(plan.Items))
		for i, item := range plan.Items {
			keys[i] = item.ItemKey
		}
		chord, change := slices.Index(keys, voicingKey("Am")), slices.Index(keys, changeKey("Am", "C"))
		require.GreaterOrEqual(t, chord, 0)
		require.GreaterOrEqual(t, change, 0)
		assert.Less(t, chord, change)
	})

	t.Run("a session in the head never plays chords", func(t *testing.T) {
		f := chordFixture(t, []string{"Am", "C"}, [][2]string{{"Am", "C"}})
		f.exercise("ex-1", "play-open-chords", 30)

		plan := f.composeInTheHead(t, 10)

		for _, item := range plan.Items {
			assert.NotContains(t, []domain.PracticeItemKind{
				domain.PracticeItemKindChordVoicing, domain.PracticeItemKindChordChange,
				domain.PracticeItemKindRootFifth, domain.PracticeItemKindRootChange,
			}, item.Kind, item.ItemKey)
		}
	})

	t.Run("an in-hand bass session plays a chord's root and fifth, both marked", func(t *testing.T) {
		f := chordFixture(t, []string{"Am"}, nil)

		plan := f.compose(t, practiceBass, 10)

		items := itemsOfKind(plan, domain.PracticeItemKindRootFifth)
		require.Len(t, items, 1)
		assert.Equal(t, &domain.PlannedRootFifth{
			LayoutInstrumentID: practiceBass, ChordSymbol: "Am",
			Root: domain.Place{String: 3, Fret: 0}, Fifth: domain.Place{String: 4, Fret: 0},
		}, items[0].RootFifth)
	})

	t.Run("an in-hand bass session plays root changes on their minute", func(t *testing.T) {
		f := chordFixture(t, nil, [][2]string{{"Am", "C"}})

		plan := f.compose(t, practiceBass, 10)

		items := itemsOfKind(plan, domain.PracticeItemKindRootChange)
		require.Len(t, items, 1)
		assert.Equal(t, domain.Place{String: 3, Fret: 3}, items[0].RootChange.ToRoot)
		assert.Equal(t, 60, items[0].RootChange.Minute.TargetChangesPerMinute)
	})

	t.Run("a chord item is never the warm-up or the application ending", func(t *testing.T) {
		f := chordFixture(t, []string{"Am", "C"}, [][2]string{{"Am", "C"}})
		for _, key := range []string{voicingKey("Am"), voicingKey("C")} {
			f.stateOf(key, domain.PracticeItemState{Level: domain.KnowledgeLevelFluent, Counted: 6, Box: 4, DueAt: f.inDays(10)})
		}

		plan := f.compose(t, practiceGuitar, 20)

		for _, item := range plan.Items {
			if item.Reason == domain.PracticePickWarmUp || item.Reason == domain.PracticePickApplication {
				assert.Equal(t, domain.PracticeItemKindPlayAlong, item.Kind)
			}
		}
	})

	t.Run("an in-hand session with only chords to play is composed", func(t *testing.T) {
		f := chordFixture(t, []string{"Am"}, nil)

		plan := f.compose(t, practiceGuitar, 10)

		require.NotEmpty(t, itemsOfKind(plan, domain.PracticeItemKindChordVoicing))
	})
}
