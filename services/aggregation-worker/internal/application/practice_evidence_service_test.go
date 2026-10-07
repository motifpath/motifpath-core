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
	minorThird  = "00000000-0000-4000-8000-0000000000e1"
	rightOption = "00000000-0000-4000-8000-0000000000b1"
	wrongOption = "00000000-0000-4000-8000-0000000000a1"
)

var playAlongKey = "play_along:" + pentatonic

type fakeReference struct {
	diagrams    map[string]domain.DiagramReference
	exercises   map[string]domain.ExerciseReference
	instruments map[string]domain.InstrumentReference
	fluentTimes map[string][]domain.FluentTime
	err         error
}

func (f *fakeReference) Instruments(_ context.Context, ids []string) (map[string]domain.InstrumentReference, error) {
	if f.err != nil {
		return nil, f.err
	}
	found := map[string]domain.InstrumentReference{}
	for _, id := range ids {
		if i, ok := f.instruments[id]; ok {
			found[id] = i
		}
	}
	return found, nil
}

func (f *fakeReference) Exercises(_ context.Context, ids []string) (map[string]domain.ExerciseReference, error) {
	if f.err != nil {
		return nil, f.err
	}
	found := map[string]domain.ExerciseReference{}
	for _, id := range ids {
		if e, ok := f.exercises[id]; ok {
			found[id] = e
		}
	}
	return found, nil
}

func (f *fakeReference) FluentTimes(_ context.Context, templateKey string) ([]domain.FluentTime, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.fluentTimes[templateKey], nil
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

// fakeHistory keeps one snapshot per (student, item, day), replaced on a rewrite.
type fakeHistory struct {
	snapshots map[string]map[time.Time]domain.ItemFold
	putErr    error
}

func (f *fakeHistory) Put(_ context.Context, studentID, itemKey string, snapshots []domain.ItemSnapshot) error {
	if f.putErr != nil {
		return f.putErr
	}
	days := f.snapshots[studentID+"|"+itemKey]
	if days == nil {
		days = map[time.Time]domain.ItemFold{}
		f.snapshots[studentID+"|"+itemKey] = days
	}
	for _, s := range snapshots {
		days[s.Day] = s.Fold
	}
	return nil
}

type practiceFixture struct {
	reference *fakeReference
	evidence  *fakeEvidence
	states    *fakeStates
	history   *fakeHistory
	service   *application.PracticeEvidenceService
}

func newPracticeFixture() *practiceFixture {
	tempo := 100
	f := &practiceFixture{
		reference: &fakeReference{
			diagrams: map[string]domain.DiagramReference{
				pentatonic: {ID: pentatonic, TempoBPM: &tempo},
				cagedA: {
					ID: cagedA, LayoutInstrumentID: guitarLayout,
					ShapeFamily: "caged-grip", Shape: "A", FamilyMembers: []string{"C", "A", "G", "E", "D"},
					Positions: []domain.DiagramPosition{{String: 5, Fret: 3, Interval: "R"}, {String: 2, Fret: 5, Interval: "3"}},
				},
			},
			exercises: map[string]domain.ExerciseReference{
				minorThird: {ID: minorThird, ExerciseType: "text_response", OptionIDs: []string{wrongOption, rightOption}, CorrectOptionIDs: []string{rightOption}},
			},
			instruments: map[string]domain.InstrumentReference{
				guitarLayout: {ID: guitarLayout, Tuning: []string{"E2", "A2", "D3", "G3", "B3", "E4"}},
			},
			fluentTimes: map[string][]domain.FluentTime{
				"exercise:text_response":        {{Version: 1, EffectiveFrom: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), FluentNetMs: 6000}},
				"fretboard_cell:name_the_note":  {{Version: 1, EffectiveFrom: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), FluentNetMs: 3000}},
				"fretboard_cell:find_the_note":  {{Version: 1, EffectiveFrom: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), FluentNetMs: 4000}},
				"diagram_shape:name_the_shape":  {{Version: 1, EffectiveFrom: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), FluentNetMs: 4000}},
				"diagram_shape:find_the_degree": {{Version: 1, EffectiveFrom: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), FluentNetMs: 2000}},
			},
		},
		evidence: &fakeEvidence{},
		states:   &fakeStates{folds: map[string]domain.ItemFold{}, versions: map[string]int{}},
		history:  &fakeHistory{snapshots: map[string]map[time.Time]domain.ItemFold{}},
	}
	f.service = application.NewPracticeEvidenceService(f.reference, f.evidence, f.states, f.history,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	return f
}

// wantHistory is the snapshots a batch derivation from all the stored evidence gives.
func (f *practiceFixture) wantHistory(t *testing.T, itemKey string) map[time.Time]domain.ItemFold {
	t.Helper()
	stored, err := f.evidence.ListForItem(context.Background(), alice, itemKey)
	require.NoError(t, err)
	snapshots, err := domain.DailySnapshots(stored, domain.ItemGoal{TargetTempoBPM: f.reference.diagrams[pentatonic].TempoBPM})
	require.NoError(t, err)
	want := map[time.Time]domain.ItemFold{}
	for _, s := range snapshots {
		want[s.Day] = s.Fold
	}
	return want
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

func challengeAnswer(eventID string, at time.Time, optionID string, latency int) domain.PracticeAnswer {
	return domain.PracticeAnswer{
		EventID:        eventID,
		StudentID:      alice,
		OccurredAt:     at,
		TriggerContext: &domain.TriggerContext{Source: "challenge_sequence", ChallengeID: "c4a11e00-0000-4000-8000-000000000001"},
		ItemKey:        "exercise:" + minorThird,
		Response:       domain.PracticeResponse{Type: domain.PracticeResponseOptionChoice, OptionIDs: []string{optionID}, LatencyMs: &latency},
	}
}

func TestPracticeEvidenceService_GradesAndFoldsAnExerciseAnswer(t *testing.T) {
	f := newPracticeFixture()
	answer := challengeAnswer("e0000000-0000-4000-8000-000000000001", monday, rightOption, 8000)
	audio := 0
	answer.Response.AudioMs = &audio

	require.NoError(t, f.service.Process(context.Background(), answer))

	require.Len(t, f.evidence.stored, 1)
	e := f.evidence.stored[0]
	assert.Equal(t, domain.EvidenceSourceAutoGraded, e.Source)
	assert.Equal(t, "exercise_option.v1", e.GraderID)
	assert.Empty(t, e.PracticeSessionID)
	assert.Equal(t, answer.TriggerContext, e.TriggerContext, "a challenge answer names its challenge instead of a session")
	require.NotNil(t, e.Correct)
	assert.True(t, *e.Correct)
	assert.Equal(t, 8000, *e.LatencyMs)
	assert.Equal(t, &audio, e.AudioMs)

	fold, ok := f.fold(t, answer.ItemKey)
	require.True(t, ok)
	assert.Equal(t, 1, fold.Box)
	// Judged against text exercises' fluent time: 6000 of 8000 ms.
	assert.InDelta(t, 0.75, fold.Fluency, 1e-9)
}

const guitarLayout = "6ea2d087-ab9c-59dc-9657-8546025414d2"

// cellAnswer is alice's answer to the guitar cell on string 5, fret 3 (a C), after
// a tap check of 300 ms.
func cellAnswer(eventID string, at time.Time, response domain.PracticeResponse) domain.PracticeAnswer {
	tap := 300
	return domain.PracticeAnswer{
		EventID: eventID, StudentID: alice, OccurredAt: at, PracticeSessionID: "5e550000-0000-4000-8000-000000000001",
		ItemKey: "fretboard_cell:" + guitarLayout + ":5:3", Response: response, TapMs: &tap,
	}
}

func TestPracticeEvidenceService_GradesAndFoldsAFretboardCellAnswer(t *testing.T) {
	latency := 3500

	t.Run("a named note is graded, keeps its answer key and is judged by naming's fluent time", func(t *testing.T) {
		f := newPracticeFixture()
		answer := cellAnswer("e0000000-0000-4000-8000-0000000000c1", monday, domain.PracticeResponse{Type: domain.PracticeResponseNameTheNote, NoteName: "C", LatencyMs: &latency})

		require.NoError(t, f.service.Process(context.Background(), answer))

		require.Len(t, f.evidence.stored, 1)
		e := f.evidence.stored[0]
		assert.Equal(t, "fretboard_cell.v1", e.GraderID)
		require.NotNil(t, e.Correct)
		assert.True(t, *e.Correct)
		require.NotNil(t, e.AnswerKey)
		assert.Equal(t, "C", e.AnswerKey.NoteName)
		fold, ok := f.fold(t, answer.ItemKey)
		require.True(t, ok)
		// 3500 ms less the 300 ms tap is 3200 ms against naming's 3000 ms.
		assert.InDelta(t, 3000.0/3200, fold.Fluency, 1e-9)
		assert.Equal(t, 1, fold.RightByResponse[domain.PracticeResponseNameTheNote])
	})

	t.Run("a found note is judged by finding's fluent time", func(t *testing.T) {
		f := newPracticeFixture()
		five, three := 5, 3
		answer := cellAnswer("e0000000-0000-4000-8000-0000000000c2", monday, domain.PracticeResponse{Type: domain.PracticeResponseFindTheNote, String: &five, Fret: &three, LatencyMs: &latency})

		require.NoError(t, f.service.Process(context.Background(), answer))

		fold, ok := f.fold(t, answer.ItemKey)
		require.True(t, ok)
		assert.InDelta(t, 1, fold.Fluency, 1e-9, "3200 ms is within finding's 4000 ms")
	})

	t.Run("a cell on an instrument the snapshot doesn't know stores nothing", func(t *testing.T) {
		f := newPracticeFixture()
		delete(f.reference.instruments, guitarLayout)

		require.NoError(t, f.service.Process(context.Background(), cellAnswer("e0000000-0000-4000-8000-0000000000c3", monday,
			domain.PracticeResponse{Type: domain.PracticeResponseNameTheNote, NoteName: "C", LatencyMs: &latency})))

		assert.Empty(t, f.evidence.stored)
	})
}

// cagedA is C major's CAGED A grip at shift 3, with its root on string 5 at
// fret 3 and its 3 on string 2 at fret 5.
const cagedA = "928330d5-903e-572c-9d41-5fde99d51ed1"

// shapeAnswer is alice's answer to the cagedA shape, after a tap check of 300 ms.
func shapeAnswer(eventID string, response domain.PracticeResponse) domain.PracticeAnswer {
	tap := 300
	return domain.PracticeAnswer{
		EventID: eventID, StudentID: alice, OccurredAt: monday, PracticeSessionID: "5e550000-0000-4000-8000-000000000002",
		ItemKey: "diagram_shape:" + cagedA, Response: response, TapMs: &tap,
	}
}

func TestPracticeEvidenceService_GradesAndFoldsADiagramShapeAnswer(t *testing.T) {
	latency := 3500

	t.Run("a named shape is graded, keeps its answer key and is judged by naming's fluent time", func(t *testing.T) {
		f := newPracticeFixture()
		answer := shapeAnswer("e0000000-0000-4000-8000-0000000000d1", domain.PracticeResponse{Type: domain.PracticeResponseNameTheShape, Shape: "A", LatencyMs: &latency})

		require.NoError(t, f.service.Process(context.Background(), answer))

		require.Len(t, f.evidence.stored, 1)
		e := f.evidence.stored[0]
		assert.Equal(t, "diagram_shape.v1", e.GraderID)
		require.NotNil(t, e.Correct)
		assert.True(t, *e.Correct)
		assert.Equal(t, &domain.AnswerKey{ShapeFamily: "caged-grip", Shape: "A"}, e.AnswerKey)
		fold, ok := f.fold(t, answer.ItemKey)
		require.True(t, ok)
		assert.InDelta(t, 1, fold.Fluency, 1e-9, "3200 ms is within naming's 4000 ms")
		assert.Equal(t, 1, fold.RightByResponse[domain.PracticeResponseNameTheShape])
	})

	t.Run("a found degree is graded on the shape's layout and judged by finding's fluent time", func(t *testing.T) {
		f := newPracticeFixture()
		two, five := 2, 5
		answer := shapeAnswer("e0000000-0000-4000-8000-0000000000d2", domain.PracticeResponse{Type: domain.PracticeResponseFindTheDegree, Interval: "3", String: &two, Fret: &five, LatencyMs: &latency})

		require.NoError(t, f.service.Process(context.Background(), answer))

		require.Len(t, f.evidence.stored, 1)
		assert.True(t, *f.evidence.stored[0].Correct)
		fold, ok := f.fold(t, answer.ItemKey)
		require.True(t, ok)
		// 3500 ms less the 300 ms tap is 3200 ms against finding's 2000 ms.
		assert.InDelta(t, 2000.0/3200, fold.Fluency, 1e-9)
	})

	t.Run("a shape whose layout instrument the snapshot doesn't know stores nothing", func(t *testing.T) {
		f := newPracticeFixture()
		delete(f.reference.instruments, guitarLayout)
		two, five := 2, 5

		require.NoError(t, f.service.Process(context.Background(), shapeAnswer("e0000000-0000-4000-8000-0000000000d3",
			domain.PracticeResponse{Type: domain.PracticeResponseFindTheDegree, Interval: "3", String: &two, Fret: &five, LatencyMs: &latency})))

		assert.Empty(t, f.evidence.stored)
	})
}

func TestPracticeEvidenceService_AWrongExerciseAnswerIsAMiss(t *testing.T) {
	f := newPracticeFixture()

	require.NoError(t, f.service.Process(context.Background(), challengeAnswer("e0000000-0000-4000-8000-000000000001", monday, wrongOption, 3000)))

	require.Len(t, f.evidence.stored, 1)
	assert.False(t, *f.evidence.stored[0].Correct)
	fold, _ := f.fold(t, "exercise:"+minorThird)
	assert.InDelta(t, 0, fold.Accuracy, 1e-9)
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
		{"the exercise isn't in the reference", func() domain.PracticeAnswer {
			a := challengeAnswer("e0000000-0000-4000-8000-000000000001", monday, rightOption, 3000)
			a.ItemKey = "exercise:" + missingDiag
			return a
		}},
		{"an option the exercise doesn't have", func() domain.PracticeAnswer {
			return challengeAnswer("e0000000-0000-4000-8000-000000000001", monday, missingDiag, 3000)
		}},
		{"a fretboard cell's response doesn't fit its item", func() domain.PracticeAnswer {
			a := ratedTake("e0000000-0000-4000-8000-000000000001", monday, domain.SelfRatingClean, 90)
			a.ItemKey = "fretboard_cell:6ea2d087-ab9c-59dc-9657-8546025414d2:5:3"
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

func TestPracticeEvidenceService_KeepsADailySnapshotOfEachItem(t *testing.T) {
	mon := ratedTake("e0000000-0000-4000-8000-000000000001", monday, domain.SelfRatingClean, 90)
	monLater := ratedTake("e0000000-0000-4000-8000-000000000002", monday.Add(5*time.Hour), domain.SelfRatingAlmost, 95)
	tue := ratedTake("e0000000-0000-4000-8000-000000000003", monday.AddDate(0, 0, 1), domain.SelfRatingStruggled, 80)
	wed := ratedTake("e0000000-0000-4000-8000-000000000004", monday.AddDate(0, 0, 2), domain.SelfRatingClean, 90)
	mondayMidnight := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name       string
		deliveries []domain.PracticeAnswer
		wantDays   []time.Time
	}{
		{"one snapshot per day practised, holding the day's last state",
			[]domain.PracticeAnswer{mon, monLater, wed},
			[]time.Time{mondayMidnight, mondayMidnight.AddDate(0, 0, 2)}},
		{"a late answer rewrites the snapshots from its day on",
			[]domain.PracticeAnswer{mon, wed, tue},
			[]time.Time{mondayMidnight, mondayMidnight.AddDate(0, 0, 1), mondayMidnight.AddDate(0, 0, 2)}},
		{"a redelivered answer leaves the snapshots as they were",
			[]domain.PracticeAnswer{mon, wed, mon},
			[]time.Time{mondayMidnight, mondayMidnight.AddDate(0, 0, 2)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newPracticeFixture()
			for _, a := range c.deliveries {
				require.NoError(t, f.service.Process(context.Background(), a))
			}

			got := f.history.snapshots[alice+"|"+playAlongKey]
			assert.Equal(t, f.wantHistory(t, playAlongKey), got)
			assert.Len(t, got, len(c.wantDays))
			for _, day := range c.wantDays {
				assert.Contains(t, got, day)
			}
			state, _ := f.fold(t, playAlongKey)
			assert.Equal(t, state, got[c.wantDays[len(c.wantDays)-1]], "the latest snapshot is the item's state")
		})
	}
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
	t.Run("a snapshot write failure is returned for a retry", func(t *testing.T) {
		f := newPracticeFixture()
		f.history.putErr = boom
		err := f.service.Process(context.Background(), ratedTake("e0000000-0000-4000-8000-000000000001", monday, domain.SelfRatingClean, 90))
		assert.ErrorIs(t, err, boom)
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
		require.Equal(t, f.wantHistory(t, playAlongKey), f.history.snapshots[alice+"|"+playAlongKey], "seed %d", seed)
	}
}

func TestProcessEventService_HandsPracticeAnswersToTheEvidenceProcessor(t *testing.T) {
	f := newPracticeFixture()
	svc := application.NewProcessEventService(newFakeRepository(), f.service, newActivity())
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
	svc := application.NewProcessEventService(newFakeRepository(), f.service, newActivity())

	require.NoError(t, svc.Handle(context.Background(), domain.TrackingEvent{
		EventType: domain.EventTypePracticeItemAnswered,
		StudentID: alice,
	}))

	assert.Empty(t, f.evidence.stored)
}
