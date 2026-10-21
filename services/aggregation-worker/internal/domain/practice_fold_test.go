package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var day0 = time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)

func onDay(n int) time.Time { return day0.AddDate(0, 0, n) }

func ptr[T any](v T) *T { return &v }

func take(at time.Time, rating SelfRating, bpm int) PracticeEvidence {
	return PracticeEvidence{
		ItemKey:    "play_along:" + diagramA,
		Source:     EvidenceSourceSelfAssessed,
		OccurredAt: at,
		GraderID:   "self_rating.v1",
		Rating:     rating,
		TempoBPM:   ptr(bpm),
	}
}

// takeIn is a take played at a note value.
func takeIn(at time.Time, rating SelfRating, bpm int, v NoteValue) PracticeEvidence {
	e := take(at, rating, bpm)
	e.NoteValue = &v
	return e
}

var target100 = ItemGoal{TargetTempoBPM: ptr(100)}

func foldAll(t *testing.T, goal ItemGoal, evidence ...PracticeEvidence) ItemFold {
	t.Helper()
	var f ItemFold
	for _, e := range evidence {
		var err error
		f, err = FoldEvidence(f, e, goal)
		require.NoError(t, err)
	}
	return f
}

func TestItemFold_NeverPractisedIsNew(t *testing.T) {
	assert.Equal(t, KnowledgeLevelNew, ItemFold{}.Level())
}

func TestFoldEvidence_FirstTakePutsTheItemInBoxOne(t *testing.T) {
	f := foldAll(t, target100, take(onDay(0), SelfRatingClean, 80))

	assert.Equal(t, 1, f.Attempts)
	assert.Equal(t, 1, f.Counted)
	assert.Equal(t, 1, f.Box)
	assert.Equal(t, onDay(1), *f.DueAt)
	assert.Equal(t, onDay(0), *f.LastAt)
	assert.InDelta(t, 1.0, f.Accuracy, 1e-9)
	assert.InDelta(t, 0.8, f.Fluency, 1e-9)
	assert.Equal(t, 80, *f.BestCleanBPM)
	assert.Equal(t, KnowledgeLevelLearning, f.Level())
}

func TestFoldEvidence_LeitnerBoxes(t *testing.T) {
	t.Run("a clean take on a due item moves it up one box", func(t *testing.T) {
		f := foldAll(t, target100, take(onDay(0), SelfRatingClean, 80), take(onDay(1), SelfRatingClean, 80))
		assert.Equal(t, 2, f.Box)
		assert.Equal(t, onDay(3), *f.DueAt)
	})
	t.Run("a clean take before the item is due doesn't move it", func(t *testing.T) {
		f := foldAll(t, target100, take(onDay(0), SelfRatingClean, 80), take(onDay(0).Add(time.Hour), SelfRatingClean, 80))
		assert.Equal(t, 1, f.Box)
		assert.Equal(t, onDay(1), *f.DueAt)
	})
	t.Run("a struggle sends the item back to box 1", func(t *testing.T) {
		f := foldAll(t, target100,
			take(onDay(0), SelfRatingClean, 80), take(onDay(1), SelfRatingClean, 80), take(onDay(3), SelfRatingClean, 80),
			take(onDay(4), SelfRatingStruggled, 70))
		assert.Equal(t, 1, f.Box)
		assert.Equal(t, onDay(5), *f.DueAt)
	})
	t.Run("almost changes no box", func(t *testing.T) {
		f := foldAll(t, target100, take(onDay(0), SelfRatingClean, 80), take(onDay(1), SelfRatingClean, 80),
			take(onDay(3), SelfRatingAlmost, 80))
		assert.Equal(t, 2, f.Box)
		assert.Equal(t, onDay(3), *f.DueAt)
	})
	t.Run("the box stops at 6", func(t *testing.T) {
		evidence := []PracticeEvidence{take(onDay(0), SelfRatingClean, 100)}
		at := onDay(0)
		for _, wait := range []int{1, 2, 4, 8, 16, 32, 32} {
			at = at.AddDate(0, 0, wait)
			evidence = append(evidence, take(at, SelfRatingClean, 100))
		}
		f := foldAll(t, target100, evidence...)
		assert.Equal(t, 6, f.Box)
	})
}

func TestFoldEvidence_WeightedAverages(t *testing.T) {
	// Self-assessed evidence weighs 0.3: almost reads 0.5, so 1 moves 30% of the way to it.
	f := foldAll(t, target100, take(onDay(0), SelfRatingClean, 100), take(onDay(0), SelfRatingAlmost, 100))
	assert.InDelta(t, 1+0.3*(0.5-1), f.Accuracy, 1e-9)
	assert.InDelta(t, 1+0.3*(0.5-1), f.Fluency, 1e-9)
}

func TestFoldEvidence_ExplorationIsNotForgetting(t *testing.T) {
	t.Run("a take that isn't clean above the best clean tempo isn't counted", func(t *testing.T) {
		before := foldAll(t, target100, take(onDay(0), SelfRatingClean, 110))
		after, err := FoldEvidence(before, take(onDay(0).Add(time.Minute), SelfRatingStruggled, 115), target100)
		require.NoError(t, err)

		assert.Equal(t, before.Counted, after.Counted)
		assert.Equal(t, before.Accuracy, after.Accuracy)
		assert.Equal(t, before.Box, after.Box)
		assert.Equal(t, 2, after.Attempts)
		assert.Equal(t, onDay(0).Add(time.Minute), *after.LastAt)
	})
	t.Run("a take that isn't clean at or below the best clean tempo is a miss", func(t *testing.T) {
		before := foldAll(t, target100, take(onDay(0), SelfRatingClean, 110))
		after, err := FoldEvidence(before, take(onDay(0).Add(time.Minute), SelfRatingStruggled, 110), target100)
		require.NoError(t, err)

		assert.Equal(t, before.Counted+1, after.Counted)
		assert.Less(t, after.Accuracy, before.Accuracy)
	})
	t.Run("with no clean take yet, every take counts", func(t *testing.T) {
		f := foldAll(t, target100, take(onDay(0), SelfRatingStruggled, 140))
		assert.Equal(t, 1, f.Counted)
	})
}

func TestFoldEvidence_TemposCompareAtEffectiveSpeed(t *testing.T) {
	t.Run("a clean take in eighths beats a faster-looking tempo in quarters", func(t *testing.T) {
		f := foldAll(t, target100, take(onDay(0), SelfRatingClean, 110), takeIn(onDay(1), SelfRatingClean, 60, NoteValueEighths))
		assert.Equal(t, 60, *f.BestCleanBPM)
		assert.Equal(t, NoteValueEighths, *f.BestCleanNoteValue)
	})
	t.Run("a clean take slower at effective speed keeps the best as played", func(t *testing.T) {
		f := foldAll(t, target100, takeIn(onDay(0), SelfRatingClean, 60, NoteValueEighths), take(onDay(1), SelfRatingClean, 110))
		assert.Equal(t, 60, *f.BestCleanBPM)
		assert.Equal(t, NoteValueEighths, *f.BestCleanNoteValue)
	})
	t.Run("a clean take in quarters keeps quarters as its note value", func(t *testing.T) {
		f := foldAll(t, target100, takeIn(onDay(0), SelfRatingClean, 60, NoteValueEighths), take(onDay(1), SelfRatingClean, 130))
		assert.Equal(t, 130, *f.BestCleanBPM)
		assert.Equal(t, NoteValueQuarters, *f.BestCleanNoteValue)
	})
	t.Run("a clean take tying the best at effective speed replaces it, so the student resumes in its note value", func(t *testing.T) {
		f := foldAll(t, target100, take(onDay(0), SelfRatingClean, 160), takeIn(onDay(1), SelfRatingClean, 80, NoteValueEighths))
		assert.Equal(t, 80, *f.BestCleanBPM)
		assert.Equal(t, NoteValueEighths, *f.BestCleanNoteValue)
	})
	t.Run("a take in eighths that isn't clean above the edge at effective speed isn't counted", func(t *testing.T) {
		before := foldAll(t, target100, take(onDay(0), SelfRatingClean, 110))
		after, err := FoldEvidence(before, takeIn(onDay(0).Add(time.Minute), SelfRatingStruggled, 60, NoteValueEighths), target100)
		require.NoError(t, err)
		assert.Equal(t, before.Counted, after.Counted)
	})
	t.Run("a take in quarters slower than a best clean tempo in eighths is a miss", func(t *testing.T) {
		before := foldAll(t, target100, takeIn(onDay(0), SelfRatingClean, 60, NoteValueEighths))
		after, err := FoldEvidence(before, take(onDay(0).Add(time.Minute), SelfRatingStruggled, 110), target100)
		require.NoError(t, err)
		assert.Equal(t, before.Counted+1, after.Counted)
	})
	t.Run("halves count at half the tempo", func(t *testing.T) {
		before := foldAll(t, target100, take(onDay(0), SelfRatingClean, 60))
		after, err := FoldEvidence(before, takeIn(onDay(0).Add(time.Minute), SelfRatingStruggled, 100, NoteValueHalves), target100)
		require.NoError(t, err)
		assert.Equal(t, before.Counted+1, after.Counted, "100 in halves is 50, slower than the best clean 60")
	})
	t.Run("fluency compares the effective speed with the target", func(t *testing.T) {
		f := foldAll(t, target100, takeIn(onDay(0), SelfRatingClean, 40, NoteValueEighths))
		assert.InDelta(t, 0.8, f.Fluency, 1e-9)
	})
}

func TestNoteValue_Factor(t *testing.T) {
	cases := []struct {
		v    *NoteValue
		want float64
	}{
		{nil, 1},
		{ptr(NoteValueHalves), 0.5},
		{ptr(NoteValueQuarters), 1},
		{ptr(NoteValueEighths), 2},
		{ptr(NoteValueTriplets), 3},
		{ptr(NoteValueSixteenths), 4},
		{ptr(NoteValueQuintuplets), 5},
		{ptr(NoteValueSextuplets), 6},
		{ptr(NoteValueThirtySeconds), 8},
	}
	for _, tc := range cases {
		assert.InDelta(t, tc.want, EffectiveSpeed(1, tc.v), 1e-9, "%v", tc.v)
	}
}

func TestFoldEvidence_Levels(t *testing.T) {
	t.Run("three clean takes are accurate", func(t *testing.T) {
		f := foldAll(t, target100, take(onDay(0), SelfRatingClean, 60), take(onDay(1), SelfRatingClean, 60), take(onDay(3), SelfRatingClean, 60))
		assert.Equal(t, KnowledgeLevelAccurate, f.Level())
	})
	t.Run("five clean takes near the target tempo are fluent", func(t *testing.T) {
		// Two takes come before they're due, so the item only reaches box 3.
		f := foldAll(t, target100, take(onDay(0), SelfRatingClean, 90), take(onDay(0), SelfRatingClean, 90),
			take(onDay(1), SelfRatingClean, 90), take(onDay(1), SelfRatingClean, 90), take(onDay(3), SelfRatingClean, 90))
		require.Equal(t, 3, f.Box)
		assert.Equal(t, KnowledgeLevelFluent, f.Level())
	})
	t.Run("five clean takes far below the target tempo stay accurate", func(t *testing.T) {
		f := foldAll(t, target100, take(onDay(0), SelfRatingClean, 60), take(onDay(1), SelfRatingClean, 60),
			take(onDay(3), SelfRatingClean, 60), take(onDay(7), SelfRatingClean, 60), take(onDay(15), SelfRatingClean, 60))
		assert.Equal(t, KnowledgeLevelAccurate, f.Level())
	})
	t.Run("fluent in box 5 is retained", func(t *testing.T) {
		f := foldAll(t, target100, take(onDay(0), SelfRatingClean, 100), take(onDay(1), SelfRatingClean, 100),
			take(onDay(3), SelfRatingClean, 100), take(onDay(7), SelfRatingClean, 100), take(onDay(15), SelfRatingClean, 100))
		require.Equal(t, 5, f.Box)
		assert.Equal(t, KnowledgeLevelRetained, f.Level())
	})
	t.Run("exploration takes don't count toward the attempts a level needs", func(t *testing.T) {
		f := foldAll(t, target100, take(onDay(0), SelfRatingClean, 60), take(onDay(0), SelfRatingStruggled, 70),
			take(onDay(0), SelfRatingAlmost, 75))
		assert.Equal(t, 3, f.Attempts)
		assert.Equal(t, 1, f.Counted)
		assert.Equal(t, KnowledgeLevelLearning, f.Level())
	})
	t.Run("without a target tempo, a clean take is fully fluent", func(t *testing.T) {
		f := foldAll(t, ItemGoal{}, take(onDay(0), SelfRatingClean, 40))
		assert.InDelta(t, 1.0, f.Fluency, 1e-9)
	})
}

// minute is a clean minute of chord changes at, with cleanChanges at tempo
// cleanChanges BPM.
func minute(at time.Time, cleanChanges int) PracticeEvidence {
	return PracticeEvidence{
		ItemKey: "chord_change:" + diagramA + ":" + diagramB, Source: EvidenceSourceSelfAssessed,
		OccurredAt: at, GraderID: "self_rating.v1", Rating: SelfRatingClean, TempoBPM: ptr(cleanChanges), ChangesPerMinute: ptr(cleanChanges),
	}
}

func TestFoldEvidence_ChordChangesMeasureChangesPerMinute(t *testing.T) {
	sixty := ItemGoal{ChangeTargets: []ChangeTarget{{Version: 1, EffectiveFrom: onDay(0), ChangesPerMinute: 60}}}

	t.Run("a minute's fluency is its clean count over the target", func(t *testing.T) {
		f := foldAll(t, sixty, minute(onDay(0), 30))
		assert.InDelta(t, 0.5, f.Fluency, 1e-9)
		assert.Equal(t, 30, *f.BestChangesPerMinute)
		assert.Nil(t, f.BestCleanBPM)
	})
	t.Run("each minute is judged by the target in force when it was played", func(t *testing.T) {
		goal := ItemGoal{ChangeTargets: []ChangeTarget{
			{Version: 2, EffectiveFrom: onDay(1), ChangesPerMinute: 30},
			{Version: 1, EffectiveFrom: onDay(0), ChangesPerMinute: 60},
		}}
		f := foldAll(t, goal, minute(onDay(1), 30))
		assert.InDelta(t, 1.0, f.Fluency, 1e-9)
		f = foldAll(t, goal, minute(onDay(0), 30))
		assert.InDelta(t, 0.5, f.Fluency, 1e-9)
	})
	t.Run("keeps the last minute's tempo and count, and the clean minutes in a row", func(t *testing.T) {
		struggled := minute(onDay(2), 20)
		struggled.Rating, struggled.TempoBPM = SelfRatingStruggled, ptr(50)
		f := foldAll(t, sixty, minute(onDay(0), 40), minute(onDay(1), 45))
		assert.Equal(t, 45, *f.LastChangeTempoBPM)
		assert.Equal(t, 45, *f.LastChangesPerMinute)
		assert.Equal(t, 2, f.CleanMinutesInARow)

		f = foldAll(t, sixty, minute(onDay(0), 40), minute(onDay(1), 45), struggled)
		assert.Equal(t, 50, *f.LastChangeTempoBPM)
		assert.Equal(t, 20, *f.LastChangesPerMinute)
		assert.Equal(t, 0, f.CleanMinutesInARow)
		assert.Equal(t, SelfRatingStruggled, f.LastRating)
	})
	t.Run("a minute exploring above the best clean count is still the last minute", func(t *testing.T) {
		exploring := minute(onDay(1), 45)
		exploring.Rating, exploring.TempoBPM = SelfRatingStruggled, ptr(90)
		f := foldAll(t, sixty, minute(onDay(0), 40), exploring)
		assert.Equal(t, 90, *f.LastChangeTempoBPM)
		assert.Equal(t, 1, f.Counted, "the exploring minute doesn't count")
	})
	t.Run("a play-along take keeps no change minute", func(t *testing.T) {
		f := foldAll(t, target100, take(onDay(0), SelfRatingClean, 80))
		assert.Nil(t, f.LastChangeTempoBPM)
	})
	t.Run("five clean minutes near the target are fluent, far below it only accurate", func(t *testing.T) {
		near := make([]PracticeEvidence, 5)
		far := make([]PracticeEvidence, 5)
		for i := range near {
			near[i], far[i] = minute(onDay(i), 54), minute(onDay(i), 30)
		}
		assert.Equal(t, KnowledgeLevelFluent, foldAll(t, sixty, near...).Level())
		assert.Equal(t, KnowledgeLevelAccurate, foldAll(t, sixty, far...).Level())
	})
}

// tourTake is a take of a progression key tour in key at bpm.
func tourTake(at time.Time, rating SelfRating, bpm int, key string) PracticeEvidence {
	e := take(at, rating, bpm)
	e.ItemKey = "progression_keys:" + diagramA + ":I-V-vi-IV"
	e.MusicKey = key
	e.Response = PracticeResponse{Type: PracticeResponseSelfRating, Rating: rating, TempoBPM: ptr(bpm), MusicKey: key}
	return e
}

func TestFoldEvidence_ProgressionKeyTour(t *testing.T) {
	tour := ItemGoal{TargetTempoBPM: ptr(ProgressionKeysTargetTempoBPM)}
	at := func(tempo int, clean ...string) ItemFold {
		return ItemFold{TourTempoBPM: ptr(tempo), TourCleanKeys: clean}
	}
	fold := func(t *testing.T, f ItemFold, evidence ...PracticeEvidence) ItemFold {
		t.Helper()
		for _, e := range evidence {
			var err error
			f, err = FoldEvidence(f, e, tour)
			require.NoError(t, err)
		}
		return f
	}

	t.Run("a first clean take starts the tour at 60% of the target and makes its key clean", func(t *testing.T) {
		f := fold(t, ItemFold{}, tourTake(onDay(0), SelfRatingClean, 60, "C"))
		assert.Equal(t, 60, *f.TourTempoBPM)
		assert.Equal(t, []string{"C"}, f.TourCleanKeys)
		assert.Equal(t, NoteValueQuarters, *f.TourNoteValue)
	})
	t.Run("a clean take slower than the tour tempo doesn't make its key clean", func(t *testing.T) {
		f := fold(t, at(70), tourTake(onDay(0), SelfRatingClean, 65, "C"))
		assert.Empty(t, f.TourCleanKeys)
		assert.Equal(t, 70, *f.TourTempoBPM)
	})
	t.Run("a clean take faster than the tour tempo makes its key clean and keeps the tempo", func(t *testing.T) {
		f := fold(t, at(70), tourTake(onDay(0), SelfRatingClean, 80, "C"))
		assert.Equal(t, []string{"C"}, f.TourCleanKeys)
		assert.Equal(t, 70, *f.TourTempoBPM)
	})
	t.Run("a clean take in eighths counts at its effective speed and sets the note value", func(t *testing.T) {
		e := tourTake(onDay(0), SelfRatingClean, 40, "C")
		e.NoteValue = ptr(NoteValueEighths)
		f := fold(t, at(70), e)
		assert.Equal(t, []string{"C"}, f.TourCleanKeys)
		assert.Equal(t, NoteValueEighths, *f.TourNoteValue)
	})
	t.Run("a struggle lowers the tour tempo by 5 and keeps the clean keys", func(t *testing.T) {
		f := fold(t, at(70, "C", "F"), tourTake(onDay(0), SelfRatingStruggled, 70, "Bb"))
		assert.Equal(t, 65, *f.TourTempoBPM)
		assert.Equal(t, []string{"C", "F"}, f.TourCleanKeys)
	})
	t.Run("an almost take keeps the key and the tempo", func(t *testing.T) {
		f := fold(t, at(70), tourTake(onDay(0), SelfRatingAlmost, 70, "C"))
		assert.Equal(t, 70, *f.TourTempoBPM)
		assert.Empty(t, f.TourCleanKeys)
	})
	t.Run("a struggle never lowers the tour below 60% of the target", func(t *testing.T) {
		f := fold(t, at(60), tourTake(onDay(0), SelfRatingStruggled, 60, "C"))
		assert.Equal(t, 60, *f.TourTempoBPM)
	})
	t.Run("a struggle above the tour tempo explores and doesn't lower it", func(t *testing.T) {
		f := fold(t, at(70), tourTake(onDay(0), SelfRatingStruggled, 90, "C"))
		assert.Equal(t, 70, *f.TourTempoBPM)
	})
	t.Run("the last key made clean starts the tour again, 5 BPM faster, with no key clean", func(t *testing.T) {
		eleven := []string{"C", "F", "Bb", "Eb", "Ab", "Db", "F#", "B", "E", "A", "D"}
		f := fold(t, at(70, eleven...), tourTake(onDay(0), SelfRatingClean, 70, "G"))
		assert.Equal(t, 75, *f.TourTempoBPM)
		assert.Empty(t, f.TourCleanKeys)
	})
	t.Run("a finished tour never climbs past the target by itself", func(t *testing.T) {
		eleven := []string{"C", "F", "Bb", "Eb", "Ab", "Db", "F#", "B", "E", "A", "D"}
		f := fold(t, at(100, eleven...), tourTake(onDay(0), SelfRatingClean, 100, "G"))
		assert.Equal(t, 100, *f.TourTempoBPM)
		assert.Empty(t, f.TourCleanKeys)
	})
	t.Run("a key made clean twice is listed once", func(t *testing.T) {
		f := fold(t, at(70, "C"), tourTake(onDay(0), SelfRatingClean, 70, "C"))
		assert.Equal(t, []string{"C"}, f.TourCleanKeys)
	})
	t.Run("clean keys are kept in the tour's order", func(t *testing.T) {
		f := fold(t, at(70, "F"), tourTake(onDay(0), SelfRatingClean, 70, "C"))
		assert.Equal(t, []string{"C", "F"}, f.TourCleanKeys)
	})
	t.Run("a play-along take keeps no tour", func(t *testing.T) {
		f := foldAll(t, target100, take(onDay(0), SelfRatingClean, 80))
		assert.Nil(t, f.TourTempoBPM)
		assert.Nil(t, f.TourCleanKeys)
	})
	t.Run("five clean takes at the target are fluent, far below it only accurate", func(t *testing.T) {
		keys := []string{"C", "F", "Bb", "Eb", "Ab"}
		near := make([]PracticeEvidence, 5)
		far := make([]PracticeEvidence, 5)
		for i := range near {
			near[i], far[i] = tourTake(onDay(i), SelfRatingClean, 100, keys[i]), tourTake(onDay(i), SelfRatingClean, 60, keys[i])
		}
		assert.Equal(t, KnowledgeLevelFluent, fold(t, ItemFold{}, near...).Level())
		assert.Equal(t, KnowledgeLevelAccurate, fold(t, ItemFold{}, far...).Level())
	})
}

// guideTake is a guide-tone take, right or not, played at bpm in v (quarters when nil).
func guideTake(at time.Time, correct bool, bpm int, v *NoteValue) PracticeEvidence {
	return PracticeEvidence{
		ItemKey:    "guide_tone:" + guitarLayout + ":ii-V-I",
		Source:     EvidenceSourceAutoGraded,
		OccurredAt: at,
		GraderID:   "timed_taps.v1",
		Response:   PracticeResponse{Type: PracticeResponseTimedTaps, MusicKey: "C", Tone: "3", TempoBPM: ptr(bpm), NoteValue: v},
		Correct:    ptr(correct),
		LatencyMs:  ptr(400),
	}
}

func TestFoldEvidence_GuideTonesMeasureTempo(t *testing.T) {
	goal := ItemGoal{TargetTempoBPM: ptr(GuideToneTargetTempoBPM)}
	fiveDays := func(correct bool, bpm int, v *NoteValue) []PracticeEvidence {
		var evidence []PracticeEvidence
		for i := range 5 {
			evidence = append(evidence, guideTake(onDay(i), correct, bpm, v))
		}
		return evidence
	}

	cases := []struct {
		name     string
		evidence []PracticeEvidence
		fluency  float64
		level    KnowledgeLevel
	}{
		{"a right take's fluency is its tempo over the target", []PracticeEvidence{guideTake(onDay(0), true, 72, nil)}, 0.8, KnowledgeLevelLearning},
		{"a take in eighths counts at its effective speed", []PracticeEvidence{guideTake(onDay(0), true, 45, ptr(NoteValueEighths))}, 1, KnowledgeLevelLearning},
		{"a take faster than the target is fully fluent", []PracticeEvidence{guideTake(onDay(0), true, 120, nil)}, 1, KnowledgeLevelLearning},
		{"a wrong take judges fluency as none", []PracticeEvidence{guideTake(onDay(0), false, 90, nil)}, 0, KnowledgeLevelLearning},
		{"five right takes at 72 BPM are fluent", fiveDays(true, 72, nil), 0.8, KnowledgeLevelFluent},
		{"five right takes at 60 BPM stay accurate", fiveDays(true, 60, nil), 60.0 / 90, KnowledgeLevelAccurate},
		{"five right takes at 45 BPM in eighths are fluent", fiveDays(true, 45, ptr(NoteValueEighths)), 1, KnowledgeLevelFluent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := foldAll(t, goal, tc.evidence...)

			assert.InDelta(t, tc.fluency, f.Fluency, 1e-9)
			assert.Equal(t, tc.level, f.Level())
		})
	}

	t.Run("the last take's key is kept, right or not, so the next plan can avoid it", func(t *testing.T) {
		inG := guideTake(onDay(1), false, 60, nil)
		inG.Response.MusicKey = "G"

		assert.Equal(t, "G", foldAll(t, goal, guideTake(onDay(0), true, 60, nil), inG).LastMusicKey)
	})

	t.Run("a late hit within its bar is never held against the student", func(t *testing.T) {
		e := guideTake(onDay(0), true, 90, nil)
		e.LatencyMs = ptr(2000)

		assert.InDelta(t, 1, foldAll(t, goal, e).Fluency, 1e-9)
	})
}

func TestProgressionKeyOrder_IsTheCycleOfFourthsFromC(t *testing.T) {
	assert.Equal(t, []string{"C", "F", "Bb", "Eb", "Ab", "Db", "F#", "B", "E", "A", "D", "G"}, ProgressionKeyOrder)
}

func TestFoldEvidence_RejectsSourcesWithoutRulesYet(t *testing.T) {
	e := take(onDay(0), SelfRatingClean, 80)
	e.Source = EvidenceSourceTeacherReviewed
	_, err := FoldEvidence(ItemFold{}, e, target100)
	assert.ErrorIs(t, err, ErrUnsupportedEvidenceSource)
}

// answer is auto-graded evidence for an exercise: right or wrong, after latency ms.
func answer(at time.Time, correct bool, latency int) PracticeEvidence {
	return PracticeEvidence{
		ItemKey:    "exercise:" + diagramA,
		Source:     EvidenceSourceAutoGraded,
		OccurredAt: at,
		GraderID:   "exercise_option.v1",
		Correct:    ptr(correct),
		LatencyMs:  ptr(latency),
	}
}

var october1 = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

// fluent6s is a text exercise's default fluent time, in force since October 1st.
var fluent6s = ItemGoal{FluentTimes: []FluentTime{{Version: 1, EffectiveFrom: october1, FluentNetMs: 6000}}}

func TestFoldEvidence_AutoGradedAnswers(t *testing.T) {
	t.Run("a right answer is a hit", func(t *testing.T) {
		f := foldAll(t, fluent6s, answer(onDay(0), true, 4000))

		assert.Equal(t, 1, f.Counted)
		assert.InDelta(t, 1, f.Accuracy, 1e-9)
		assert.Equal(t, 1, f.Box)
	})

	t.Run("a wrong answer is a miss", func(t *testing.T) {
		f := foldAll(t, fluent6s, answer(onDay(0), true, 4000), answer(onDay(1), true, 4000), answer(onDay(3), false, 4000))

		assert.Equal(t, 1, f.Box)
		assert.Less(t, f.Accuracy, 1.0)
	})

	t.Run("a right answer within the fluent time is fully fluent", func(t *testing.T) {
		f := foldAll(t, fluent6s, answer(onDay(0), true, 5999))

		assert.InDelta(t, 1, f.Fluency, 1e-9)
	})

	t.Run("a slower right answer is fluent in proportion", func(t *testing.T) {
		f := foldAll(t, fluent6s, answer(onDay(0), true, 8000))

		assert.InDelta(t, 0.75, f.Fluency, 1e-9)
	})

	t.Run("the time is judged net of the tap time and the audio", func(t *testing.T) {
		e := answer(onDay(0), true, 11000)
		e.TapMs, e.AudioMs = ptr(300), ptr(6000)
		goal := ItemGoal{FluentTimes: []FluentTime{{Version: 1, EffectiveFrom: october1, FluentNetMs: 4000}}}

		judged, ok := JudgeTimed(e, goal)

		require.True(t, ok)
		assert.Equal(t, 4700, judged.NetMs)
		assert.False(t, judged.WithinFluentTime)
		assert.InDelta(t, 4000.0/4700, foldAll(t, goal, e).Fluency, 1e-9)
	})

	t.Run("a net time below zero counts as zero", func(t *testing.T) {
		e := answer(onDay(0), true, 200)
		e.TapMs = ptr(300)

		judged, ok := JudgeTimed(e, fluent6s)

		require.True(t, ok)
		assert.Equal(t, 0, judged.NetMs)
		assert.True(t, judged.WithinFluentTime)
	})

	t.Run("five right answers within the fluent time are fluent", func(t *testing.T) {
		var evidence []PracticeEvidence
		for i := range 5 {
			evidence = append(evidence, answer(onDay(0).Add(time.Duration(i)*time.Minute), true, 3000))
		}

		assert.Equal(t, KnowledgeLevelFluent, foldAll(t, fluent6s, evidence...).Level())
	})

	t.Run("five right answers far slower than the fluent time stay accurate", func(t *testing.T) {
		var evidence []PracticeEvidence
		for i := range 5 {
			evidence = append(evidence, answer(onDay(0).Add(time.Duration(i)*time.Minute), true, 12000))
		}

		assert.Equal(t, KnowledgeLevelAccurate, foldAll(t, fluent6s, evidence...).Level())
	})

	t.Run("an answer with no fluent time in force counts for accuracy only", func(t *testing.T) {
		before := answer(time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC), true, 1000)

		_, ok := JudgeTimed(before, fluent6s)
		f := foldAll(t, fluent6s, before)

		assert.False(t, ok)
		assert.Equal(t, 1, f.Counted)
		assert.InDelta(t, 1, f.Accuracy, 1e-9)
		assert.InDelta(t, 0, f.Fluency, 1e-9)
	})

	t.Run("a generated question is judged against the fluent time of its way, not its response type", func(t *testing.T) {
		goal := ItemGoal{FluentTimesByWay: map[string][]FluentTime{
			"numeral_to_chord": {{Version: 1, EffectiveFrom: october1, FluentNetMs: 3000}},
			"chord_to_numeral": {{Version: 1, EffectiveFrom: october1, FluentNetMs: 5000}},
		}}
		toChord, toNumeral := answer(onDay(0), true, 4000), answer(onDay(0), true, 4000)
		toChord.Response = PracticeResponse{Type: PracticeResponseGeneratedChoice, Way: "numeral_to_chord"}
		toNumeral.Response = PracticeResponse{Type: PracticeResponseGeneratedChoice, Way: "chord_to_numeral"}

		judgedToChord, ok := JudgeTimed(toChord, goal)
		require.True(t, ok)
		judgedToNumeral, ok := JudgeTimed(toNumeral, goal)
		require.True(t, ok)
		assert.Equal(t, 3000, judgedToChord.FluentTime.FluentNetMs)
		assert.Equal(t, 5000, judgedToNumeral.FluentTime.FluentNetMs)
	})

	t.Run("a cell's answer is judged against the fluent time of the way it was asked", func(t *testing.T) {
		goal := ItemGoal{FluentTimesByWay: map[string][]FluentTime{
			"name_the_note": {{Version: 1, EffectiveFrom: october1, FluentNetMs: 3000}},
			"find_the_note": {{Version: 1, EffectiveFrom: october1, FluentNetMs: 4000}},
		}}
		named, found := answer(onDay(0), true, 3500), answer(onDay(0), true, 3500)
		named.Response.Type, found.Response.Type = PracticeResponseNameTheNote, PracticeResponseFindTheNote

		judgedNamed, ok := JudgeTimed(named, goal)
		require.True(t, ok)
		judgedFound, ok := JudgeTimed(found, goal)
		require.True(t, ok)

		assert.False(t, judgedNamed.WithinFluentTime, "3.5 s is slower than naming's 3 s")
		assert.True(t, judgedFound.WithinFluentTime, "3.5 s is within finding's 4 s")
	})

	t.Run("right answers are counted per way of asking", func(t *testing.T) {
		named, found, wrong := answer(onDay(0), true, 2000), answer(onDay(1), true, 2000), answer(onDay(2), false, 2000)
		named.Response.Type, found.Response.Type, wrong.Response.Type = PracticeResponseNameTheNote, PracticeResponseFindTheNote, PracticeResponseFindTheNote

		f := foldAll(t, ItemGoal{}, named, found, wrong)

		assert.Equal(t, map[string]int{"name_the_note": 1, "find_the_note": 1}, f.RightByResponse)
	})

	t.Run("a generated question's right answers are counted per way, not per response type", func(t *testing.T) {
		toChord, toNumeral, again := answer(onDay(0), true, 2000), answer(onDay(1), true, 2000), answer(onDay(2), true, 2000)
		toChord.Response = PracticeResponse{Type: PracticeResponseGeneratedChoice, Way: "numeral_to_chord"}
		toNumeral.Response = PracticeResponse{Type: PracticeResponseGeneratedChoice, Way: "chord_to_numeral"}
		again.Response = toChord.Response

		f := foldAll(t, ItemGoal{}, toChord, toNumeral, again)

		assert.Equal(t, map[string]int{"numeral_to_chord": 2, "chord_to_numeral": 1}, f.RightByResponse)
	})

	t.Run("an answer without its verdict is refused", func(t *testing.T) {
		e := answer(onDay(0), true, 4000)
		e.Correct = nil

		_, err := FoldEvidence(ItemFold{}, e, fluent6s)

		assert.Error(t, err)
	})
}

func TestItemGoal_FluentTimeAt(t *testing.T) {
	november1 := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	goal := ItemGoal{FluentTimes: []FluentTime{
		{Version: 2, EffectiveFrom: november1, FluentNetMs: 4000},
		{Version: 1, EffectiveFrom: october1, FluentNetMs: 5000},
	}}

	t.Run("each answer is judged by the version in force when it was given", func(t *testing.T) {
		v, ok := goal.FluentTimeAt(time.Date(2026, 10, 31, 23, 59, 0, 0, time.UTC))
		require.True(t, ok)
		assert.Equal(t, 1, v.Version)

		v, ok = goal.FluentTimeAt(november1)
		require.True(t, ok)
		assert.Equal(t, 2, v.Version)
	})

	t.Run("before the first version, none is in force", func(t *testing.T) {
		_, ok := goal.FluentTimeAt(october1.Add(-time.Second))
		assert.False(t, ok)
	})

	t.Run("a new version never takes back an earlier answer's fluency", func(t *testing.T) {
		early := answer(time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC), true, 4800)
		onlyV1 := ItemGoal{FluentTimes: goal.FluentTimes[1:]}

		assert.Equal(t, foldAll(t, onlyV1, early), foldAll(t, goal, early))
	})
}

func TestRebuildFold_FoldsInTimeOrderAndTiesInArrivalOrder(t *testing.T) {
	// Arrival order: Wednesday, then Monday and Tuesday late; two takes share Monday's time.
	monday := take(onDay(0), SelfRatingClean, 100)
	mondayAgain := take(onDay(0), SelfRatingStruggled, 100)
	tuesday := take(onDay(1), SelfRatingStruggled, 90)
	wednesday := take(onDay(2), SelfRatingClean, 100)

	got, err := RebuildFold([]PracticeEvidence{wednesday, monday, mondayAgain, tuesday}, target100)
	require.NoError(t, err)

	assert.Equal(t, foldAll(t, target100, monday, mondayAgain, tuesday, wednesday), got)
}

func TestDailySnapshots_KeepsTheStateAtTheEndOfEachDayPractised(t *testing.T) {
	// Arrival order: Wednesday, then Monday's two takes late. Tuesday has none.
	monday := take(onDay(0), SelfRatingClean, 80)
	mondayLater := take(onDay(0).Add(3*time.Hour), SelfRatingClean, 90)
	wednesday := take(onDay(2), SelfRatingStruggled, 100)

	got, err := DailySnapshots([]PracticeEvidence{wednesday, monday, mondayLater}, target100)
	require.NoError(t, err)

	mondayMidnight := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, []ItemSnapshot{
		{Day: mondayMidnight, Fold: foldAll(t, target100, monday, mondayLater)},
		{Day: mondayMidnight.AddDate(0, 0, 2), Fold: foldAll(t, target100, monday, mondayLater, wednesday)},
	}, got)
}

func TestDailySnapshots_DaysAreUTC(t *testing.T) {
	saoPaulo := time.FixedZone("BRT", -3*60*60)
	lateEvening := take(time.Date(2026, 10, 5, 22, 0, 0, 0, saoPaulo), SelfRatingClean, 80)

	got, err := DailySnapshots([]PracticeEvidence{lateEvening}, target100)
	require.NoError(t, err)

	require.Len(t, got, 1)
	assert.Equal(t, time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC), got[0].Day)
}

func TestDailySnapshots_NoEvidenceNoSnapshots(t *testing.T) {
	got, err := DailySnapshots(nil, target100)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestSnapshotDay_IsTheUTCMidnightOfTheDay(t *testing.T) {
	assert.Equal(t, time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), SnapshotDay(onDay(0)))
}

func TestWayAsked(t *testing.T) {
	cases := []struct {
		name string
		kind PracticeItemKind
		r    PracticeResponse
		want string
	}{
		{"a generated question, by its way", PracticeItemKindDiatonicChord, PracticeResponse{Type: PracticeResponseGeneratedChoice, Way: "chord_to_numeral"}, "chord_to_numeral"},
		{"a cell's note found everywhere, by its third way", PracticeItemKindFretboardCell, PracticeResponse{Type: PracticeResponseMultiTap}, "find_every_instance"},
		{"a guide-tone take, by its kind's one template", PracticeItemKindGuideTone, PracticeResponse{Type: PracticeResponseTimedTaps}, "tap_the_third"},
		{"a fretboard cell, by its response type", PracticeItemKindFretboardCell, PracticeResponse{Type: PracticeResponseNameTheNote}, "name_the_note"},
		{"another kind answering with multi_tap never borrows a template", PracticeItemKindDiagramShape, PracticeResponse{Type: PracticeResponseMultiTap}, "multi_tap"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, WayAsked(c.kind, c.r))
		})
	}
}
