package application_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/aggregation-worker/internal/application"
	"github.com/motifpath/aggregation-worker/internal/domain"
)

const (
	alice       = "a11ce000-0000-4000-8000-000000000001"
	pentatonic  = "00000000-0000-4000-8000-0000000000f1"
	missingDiag = "00000000-0000-4000-8000-0000000000ff"
)

var playAlongKey = "play_along:" + pentatonic

type fakeReference struct {
	diagrams map[string]domain.DiagramReference
	err      error
}

func (f *fakeReference) Diagrams(_ context.Context, ids []string) (map[string]domain.DiagramReference, error) {
	if f.err != nil {
		return nil, f.err
	}
	found := map[string]domain.DiagramReference{}
	for _, id := range ids {
		if d, ok := f.diagrams[id]; ok {
			found[id] = d
		}
	}
	return found, nil
}

type fakeEvidence struct {
	stored    []domain.PracticeEvidence
	insertErr error
}

func (f *fakeEvidence) Insert(_ context.Context, e domain.PracticeEvidence) (bool, error) {
	if f.insertErr != nil {
		return false, f.insertErr
	}
	for _, s := range f.stored {
		if s.EvidenceID == e.EvidenceID {
			return false, nil
		}
	}
	f.stored = append(f.stored, e)
	return true, nil
}

func (f *fakeEvidence) ListForItem(_ context.Context, studentID, itemKey string) ([]domain.PracticeEvidence, error) {
	var out []domain.PracticeEvidence
	for _, s := range f.stored {
		if s.StudentID == studentID && s.ItemKey == itemKey {
			out = append(out, s)
		}
	}
	return out, nil
}

type fakeStates struct {
	folds    map[string]domain.ItemFold
	versions map[string]int
	puts     int
	putErr   error
}

func (f *fakeStates) Get(_ context.Context, studentID, itemKey string) (domain.ItemFold, int, bool, error) {
	fold, ok := f.folds[studentID+"|"+itemKey]
	return fold, f.versions[studentID+"|"+itemKey], ok, nil
}

func (f *fakeStates) Put(_ context.Context, studentID, itemKey string, fold domain.ItemFold) error {
	if f.putErr != nil {
		return f.putErr
	}
	f.puts++
	f.folds[studentID+"|"+itemKey] = fold
	f.versions[studentID+"|"+itemKey] = domain.PracticeRulesVersion
	return nil
}

type practiceFixture struct {
	reference *fakeReference
	evidence  *fakeEvidence
	states    *fakeStates
	service   *application.PracticeEvidenceService
}

func newPracticeFixture() *practiceFixture {
	tempo := 100
	f := &practiceFixture{
		reference: &fakeReference{diagrams: map[string]domain.DiagramReference{
			pentatonic: {ID: pentatonic, TempoBPM: &tempo},
		}},
		evidence: &fakeEvidence{},
		states:   &fakeStates{folds: map[string]domain.ItemFold{}, versions: map[string]int{}},
	}
	f.service = application.NewPracticeEvidenceService(f.reference, f.evidence, f.states,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	return f
}

func (f *practiceFixture) fold(t *testing.T, itemKey string) (domain.ItemFold, bool) {
	t.Helper()
	fold, _, ok, err := f.states.Get(context.Background(), alice, itemKey)
	require.NoError(t, err)
	return fold, ok
}

var monday = time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)

func ratedTake(eventID string, at time.Time, rating domain.SelfRating, bpm int) domain.PracticeAnswer {
	return domain.PracticeAnswer{
		EventID:           eventID,
		StudentID:         alice,
		OccurredAt:        at,
		PracticeSessionID: "5e551000-0000-4000-8000-000000000001",
		ItemKey:           playAlongKey,
		Response:          domain.PracticeResponse{Type: domain.PracticeResponseSelfRating, Rating: rating, TempoBPM: &bpm},
	}
}

func TestPracticeEvidenceService_GradesAndFoldsATake(t *testing.T) {
	f := newPracticeFixture()
	answer := ratedTake("e0000000-0000-4000-8000-000000000001", monday, domain.SelfRatingClean, 50)

	require.NoError(t, f.service.Process(context.Background(), answer))

	require.Len(t, f.evidence.stored, 1)
	e := f.evidence.stored[0]
	assert.Equal(t, answer.EventID, e.EvidenceID)
	assert.Equal(t, alice, e.StudentID)
	assert.Equal(t, playAlongKey, e.ItemKey)
	assert.Equal(t, domain.EvidenceSourceSelfAssessed, e.Source)
	assert.Equal(t, monday, e.OccurredAt)
	assert.Equal(t, answer.PracticeSessionID, e.PracticeSessionID)
	assert.Equal(t, "self_rating.v1", e.GraderID)
	assert.Equal(t, answer.Response, e.Response)
	assert.Equal(t, domain.SelfRatingClean, e.Rating)
	assert.Equal(t, 50, *e.TempoBPM)

	fold, ok := f.fold(t, playAlongKey)
	require.True(t, ok)
	assert.Equal(t, 1, fold.Box)
	// Fluency is measured against the diagram's own tempo: 50 of 100 BPM.
	assert.InDelta(t, 0.5, fold.Fluency, 1e-9)
}

func TestPracticeEvidenceService_StoresNothingWhenItCannotGrade(t *testing.T) {
	cases := []struct {
		name   string
		answer func() domain.PracticeAnswer
	}{
		{"the grader rejects the response", func() domain.PracticeAnswer {
			a := ratedTake("e0000000-0000-4000-8000-000000000001", monday, domain.SelfRatingClean, 90)
			a.Response.TempoBPM = nil
			return a
		}},
		{"the diagram isn't in the reference", func() domain.PracticeAnswer {
			a := ratedTake("e0000000-0000-4000-8000-000000000001", monday, domain.SelfRatingClean, 90)
			a.ItemKey = "play_along:" + missingDiag
			return a
		}},
		{"the item kind has no grader yet", func() domain.PracticeAnswer {
			latency := 1800
			a := ratedTake("e0000000-0000-4000-8000-000000000001", monday, domain.SelfRatingClean, 90)
			a.ItemKey = "fretboard_cell:6ea2d087-ab9c-59dc-9657-8546025414d2:5:3"
			a.Response = domain.PracticeResponse{Type: domain.PracticeResponseNameTheNote, NoteName: "C", LatencyMs: &latency}
			return a
		}},
		{"the item key is malformed", func() domain.PracticeAnswer {
			a := ratedTake("e0000000-0000-4000-8000-000000000001", monday, domain.SelfRatingClean, 90)
			a.ItemKey = "play_along:nope"
			return a
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newPracticeFixture()

			// Dropped, not retried: redelivering it can never grade it differently.
			require.NoError(t, f.service.Process(context.Background(), c.answer()))

			assert.Empty(t, f.evidence.stored)
			assert.Empty(t, f.states.folds)
		})
	}
}

func TestPracticeEvidenceService_TheSameAnswerTwiceCountsOnce(t *testing.T) {
	f := newPracticeFixture()
	answer := ratedTake("e0000000-0000-4000-8000-000000000001", monday, domain.SelfRatingClean, 90)

	require.NoError(t, f.service.Process(context.Background(), answer))
	once, _ := f.fold(t, playAlongKey)
	require.NoError(t, f.service.Process(context.Background(), answer))

	assert.Len(t, f.evidence.stored, 1)
	twice, _ := f.fold(t, playAlongKey)
	assert.Equal(t, once, twice)
}

func TestPracticeEvidenceService_ALateAnswerIsPlacedInTimeOrder(t *testing.T) {
	mon := ratedTake("e0000000-0000-4000-8000-000000000001", monday, domain.SelfRatingClean, 90)
	tue := ratedTake("e0000000-0000-4000-8000-000000000002", monday.AddDate(0, 0, 1), domain.SelfRatingStruggled, 80)
	wed := ratedTake("e0000000-0000-4000-8000-000000000003", monday.AddDate(0, 0, 2), domain.SelfRatingClean, 90)

	inOrder := newPracticeFixture()
	for _, a := range []domain.PracticeAnswer{mon, tue, wed} {
		require.NoError(t, inOrder.service.Process(context.Background(), a))
	}
	late := newPracticeFixture()
	for _, a := range []domain.PracticeAnswer{mon, wed, tue} {
		require.NoError(t, late.service.Process(context.Background(), a))
	}

	want, _ := inOrder.fold(t, playAlongKey)
	got, _ := late.fold(t, playAlongKey)
	assert.Equal(t, want, got)
}

func TestPracticeEvidenceService_RebuildsAStateFoldedUnderOtherRules(t *testing.T) {
	f := newPracticeFixture()
	earlier := ratedTake("e0000000-0000-4000-8000-000000000001", monday, domain.SelfRatingClean, 90)
	require.NoError(t, f.service.Process(context.Background(), earlier))
	// The same evidence, but its state was folded by rules this worker no longer runs.
	stale := domain.ItemFold{Attempts: 7, Counted: 7, Accuracy: 0.1, Box: 4, LastAt: &monday}
	f.states.folds[alice+"|"+playAlongKey] = stale
	f.states.versions[alice+"|"+playAlongKey] = domain.PracticeRulesVersion - 1

	later := ratedTake("e0000000-0000-4000-8000-000000000002", monday.AddDate(0, 0, 1), domain.SelfRatingClean, 90)
	require.NoError(t, f.service.Process(context.Background(), later))

	want, err := domain.RebuildFold(f.evidence.stored, domain.ItemGoal{TargetTempoBPM: f.reference.diagrams[pentatonic].TempoBPM})
	require.NoError(t, err)
	got, _ := f.fold(t, playAlongKey)
	assert.Equal(t, want, got)
	assert.Equal(t, domain.PracticeRulesVersion, f.states.versions[alice+"|"+playAlongKey])
}

func TestPracticeEvidenceService_Failures(t *testing.T) {
	boom := errors.New("boom")

	t.Run("a reference read failure is returned for a retry", func(t *testing.T) {
		f := newPracticeFixture()
		f.reference.err = boom
		err := f.service.Process(context.Background(), ratedTake("e0000000-0000-4000-8000-000000000001", monday, domain.SelfRatingClean, 90))
		assert.ErrorIs(t, err, boom)
	})
	t.Run("an evidence write failure is returned for a retry", func(t *testing.T) {
		f := newPracticeFixture()
		f.evidence.insertErr = boom
		err := f.service.Process(context.Background(), ratedTake("e0000000-0000-4000-8000-000000000001", monday, domain.SelfRatingClean, 90))
		assert.ErrorIs(t, err, boom)
		assert.Empty(t, f.states.folds)
	})
	t.Run("a redelivery after a failed state write folds the stored evidence", func(t *testing.T) {
		f := newPracticeFixture()
		answer := ratedTake("e0000000-0000-4000-8000-000000000001", monday, domain.SelfRatingClean, 90)
		f.states.putErr = boom
		require.ErrorIs(t, f.service.Process(context.Background(), answer), boom)
		require.Len(t, f.evidence.stored, 1)

		f.states.putErr = nil
		require.NoError(t, f.service.Process(context.Background(), answer))

		fold, ok := f.fold(t, playAlongKey)
		require.True(t, ok)
		assert.Equal(t, 1, fold.Counted)
	})
}

// TestPracticeEvidenceService_IncrementalFoldEqualsBatch is the processor's guarantee:
// whatever order and however often evidence arrives, the stored state is the one a
// batch derivation from all the evidence gives.
func TestPracticeEvidenceService_IncrementalFoldEqualsBatch(t *testing.T) {
	ratings := []domain.SelfRating{domain.SelfRatingStruggled, domain.SelfRatingAlmost, domain.SelfRatingClean}
	for seed := range uint64(300) {
		rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))

		var history []domain.PracticeAnswer
		at := monday
		for i := range 1 + rng.IntN(25) {
			// Some takes share a timestamp; ties fold in arrival order.
			at = at.Add(time.Duration(rng.IntN(4)) * 12 * time.Hour)
			history = append(history, ratedTake(fmt.Sprintf("e0000000-0000-4000-8000-%012d", i), at,
				ratings[rng.IntN(len(ratings))], 60+5*rng.IntN(10)))
		}
		deliveries := append([]domain.PracticeAnswer(nil), history...)
		for range rng.IntN(4) {
			deliveries = append(deliveries, history[rng.IntN(len(history))])
		}
		// Mostly in order, with some answers arriving late.
		for range rng.IntN(5) {
			i, j := rng.IntN(len(deliveries)), rng.IntN(len(deliveries))
			deliveries[i], deliveries[j] = deliveries[j], deliveries[i]
		}

		f := newPracticeFixture()
		for _, a := range deliveries {
			require.NoError(t, f.service.Process(context.Background(), a))
		}

		stored, err := f.evidence.ListForItem(context.Background(), alice, playAlongKey)
		require.NoError(t, err)
		require.Len(t, stored, len(history), "seed %d", seed)
		want, err := domain.RebuildFold(stored, domain.ItemGoal{TargetTempoBPM: f.reference.diagrams[pentatonic].TempoBPM})
		require.NoError(t, err)
		got, _ := f.fold(t, playAlongKey)
		require.Equal(t, want, got, "seed %d", seed)
	}
}

func TestProcessEventService_HandsPracticeAnswersToTheEvidenceProcessor(t *testing.T) {
	f := newPracticeFixture()
	svc := application.NewProcessEventService(newFakeRepository(), f.service)
	answer := ratedTake("e0000000-0000-4000-8000-000000000001", monday, domain.SelfRatingClean, 90)

	require.NoError(t, svc.Handle(context.Background(), domain.TrackingEvent{
		EventType:      domain.EventTypePracticeItemAnswered,
		StudentID:      alice,
		PracticeAnswer: &answer,
	}))

	assert.Len(t, f.evidence.stored, 1)
}

func TestProcessEventService_DropsAPracticeAnswerWithoutItsAnswer(t *testing.T) {
	f := newPracticeFixture()
	svc := application.NewProcessEventService(newFakeRepository(), f.service)

	require.NoError(t, svc.Handle(context.Background(), domain.TrackingEvent{
		EventType: domain.EventTypePracticeItemAnswered,
		StudentID: alice,
	}))

	assert.Empty(t, f.evidence.stored)
}
