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

func TestFoldEvidence_ChordChangesMeasureChangesPerMinute(t *testing.T) {
	e := PracticeEvidence{
		ItemKey: "chord_change:" + diagramA + ":" + diagramB, Source: EvidenceSourceSelfAssessed,
		OccurredAt: onDay(0), GraderID: "self_rating.v1", Rating: SelfRatingClean, ChangesPerMinute: ptr(30),
	}
	f := foldAll(t, ItemGoal{TargetChangesPerMinute: ptr(60)}, e)
	assert.InDelta(t, 0.5, f.Fluency, 1e-9)
	assert.Equal(t, 30, *f.BestChangesPerMinute)
	assert.Nil(t, f.BestCleanBPM)
}

func TestFoldEvidence_RejectsSourcesWithoutRulesYet(t *testing.T) {
	for _, source := range []EvidenceSource{EvidenceSourceAutoGraded, EvidenceSourceTeacherReviewed} {
		e := take(onDay(0), SelfRatingClean, 80)
		e.Source = source
		_, err := FoldEvidence(ItemFold{}, e, target100)
		assert.ErrorIs(t, err, ErrUnsupportedEvidenceSource, source)
	}
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
