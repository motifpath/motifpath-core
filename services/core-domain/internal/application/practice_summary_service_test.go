package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// fakePracticeActivityReader is an in-memory ports.PracticeActivityReader
// for one student.
type fakePracticeActivityReader struct {
	sessions    []domain.FinishedPracticeSession
	completions []time.Time
	snapshots   map[string]domain.PracticeItemSnapshot
	snapshotsAt time.Time
	err         error
}

func (f *fakePracticeActivityReader) FinishedSessions(_ context.Context, _ string, since time.Time) ([]domain.FinishedPracticeSession, error) {
	var out []domain.FinishedPracticeSession
	for _, s := range f.sessions {
		if !s.EndedAt.Before(since) {
			out = append(out, s)
		}
	}
	return out, f.err
}

func (f *fakePracticeActivityReader) CompletionTimes(_ context.Context, _ string, since time.Time) ([]time.Time, error) {
	var out []time.Time
	for _, t := range f.completions {
		if !t.Before(since) {
			out = append(out, t)
		}
	}
	return out, f.err
}

func (f *fakePracticeActivityReader) SnapshotsAt(_ context.Context, _ string, itemKeys []string, at time.Time) (map[string]domain.PracticeItemSnapshot, error) {
	f.snapshotsAt = at
	out := map[string]domain.PracticeItemSnapshot{}
	for _, k := range itemKeys {
		if s, ok := f.snapshots[k]; ok {
			out[k] = s
		}
	}
	return out, f.err
}

// summaryFixture is a practice fixture with a summary service over it.
type summaryFixture struct {
	*practiceFixture
	svc      *application.PracticeSummaryService
	activity *fakePracticeActivityReader
}

func newSummaryFixture(t *testing.T) *summaryFixture {
	f := &summaryFixture{
		practiceFixture: newPracticeFixture(t),
		activity:        &fakePracticeActivityReader{snapshots: map[string]domain.PracticeItemSnapshot{}},
	}
	now := func() time.Time { return f.now }
	rollup := application.NewKnowledgeRollupService(f.knowledgeNodes, f.edges, practiceItemSource{f: f.practiceFixture}, f.states, now)
	f.svc = application.NewPracticeSummaryService(
		f.instruments, f.studentPaths, f.enrollments, f.learningPaths, f.courseVersions, f.contentNodes, rollup, f.activity, now,
	)
	return f
}

func (f *summaryFixture) summary(t *testing.T, instrumentID *string, timeZone string) application.PracticeSummary {
	t.Helper()
	got, err := f.svc.Summary(context.Background(), studentCaller(), instrumentID, timeZone)
	require.NoError(t, err)
	return got
}

func TestPracticeSummaryService_Summary(t *testing.T) {
	guitar := practiceGuitar
	saoPaulo, err := time.LoadLocation("America/Sao_Paulo")
	require.NoError(t, err)

	t.Run("the student's instruments come from their active paths and courses, in the instruments' order", func(t *testing.T) {
		f := newSummaryFixture(t)
		f.onPathFor([]string{practiceGuitar}, "skill-1")
		f.onPathFor(nil, "skill-2")
		require.NoError(t, f.enrollments.Create(context.Background(), domain.CourseEnrollment{
			ID: "enrollment-1", StudentID: studentCaller().ID, CourseID: "course-1", CourseVersionNumber: 2, Status: domain.CourseEnrollmentStatusActive,
		}))
		require.NoError(t, f.courseVersions.Create(context.Background(), domain.CourseVersion{ID: "v2", CourseID: "course-1", VersionNumber: 2, InstrumentIDsSnapshot: []string{practiceBass}}))

		got := f.summary(t, &guitar, "")

		assert.Equal(t, []string{practiceBass, practiceGuitar}, got.StudentInstrumentIDs, "a path for every instrument adds none")
	})

	t.Run("practice days count sessions finished with the instrument in hand, by the student's calendar", func(t *testing.T) {
		f := newSummaryFixture(t)
		f.now = time.Date(2026, 10, 5, 15, 0, 0, 0, time.UTC) // Monday noon in São Paulo
		bass := practiceBass
		f.activity.sessions = []domain.FinishedPracticeSession{
			{InstrumentID: &guitar, EndedAt: time.Date(2026, 10, 5, 13, 0, 0, 0, time.UTC)},
			{InstrumentID: &guitar, EndedAt: time.Date(2026, 10, 3, 2, 30, 0, 0, time.UTC)}, // Friday 23:30 local
			{InstrumentID: &guitar, EndedAt: time.Date(2026, 10, 2, 20, 0, 0, 0, time.UTC)}, // Friday too
			{InstrumentID: &bass, EndedAt: time.Date(2026, 10, 4, 13, 0, 0, 0, time.UTC)},
			{EndedAt: time.Date(2026, 10, 4, 13, 0, 0, 0, time.UTC)}, // in the head
		}

		got := f.summary(t, &guitar, "America/Sao_Paulo")

		assert.Equal(t, 2, got.PracticeDaysLast7)
	})

	t.Run("progress compares each skill now with its state when the last 7 days began", func(t *testing.T) {
		f := newSummaryFixture(t)
		f.now = time.Date(2026, 10, 5, 15, 0, 0, 0, time.UTC)
		f.onPathFor([]string{practiceGuitar}, "skill-1")
		keys := f.exerciseBatch("e", "skill-1", 2, 30)
		f.stateOf(keys[0], domain.PracticeItemState{Level: domain.KnowledgeLevelAccurate, Counted: 4, Box: 2, DueAt: f.inDays(2), Accuracy: 0.9})
		f.stateOf(keys[1], domain.PracticeItemState{Level: domain.KnowledgeLevelAccurate, Counted: 4, Box: 2, DueAt: f.inDays(2), Accuracy: 0.82})
		f.activity.snapshots[keys[0]] = domain.PracticeItemSnapshot{ItemKey: keys[0], Counted: 2, Accuracy: 0.72}
		f.activity.snapshots[keys[1]] = domain.PracticeItemSnapshot{ItemKey: keys[1], Counted: 2, Accuracy: 0.72}

		got := f.summary(t, &guitar, "America/Sao_Paulo")

		assert.Equal(t, time.Date(2026, 9, 29, 0, 0, 0, 0, saoPaulo), f.activity.snapshotsAt)
		require.Len(t, got.Progress, 1)
		assert.Equal(t, "skill-1", got.Progress[0].NodeID)
		assert.InDelta(t, 0.72, got.Progress[0].Before, 1e-9)
		assert.InDelta(t, 0.86, got.Progress[0].After, 1e-9)
	})

	t.Run("next steps are the top three, with how many there are in all", func(t *testing.T) {
		f := newSummaryFixture(t)
		f.onPathFor([]string{practiceGuitar}, "skill-1", "skill-2", "skill-3", "skill-4", "skill-5")
		for _, id := range []string{"skill-1", "skill-2", "skill-3", "skill-4", "skill-5"} {
			f.exerciseBatch(id, id, 1, 30)
		}

		got := f.summary(t, &guitar, "")

		require.Len(t, got.NextSteps, 3)
		assert.Equal(t, 5, got.NextStepsTotal)
		assert.Equal(t, domain.PracticeNextStepReadyToStart, got.NextSteps[0].Kind)
		assert.Equal(t, "skill-1", got.NextSteps[0].Node.ID)
	})

	t.Run("a skill unconnected to what the student is learning is no next step", func(t *testing.T) {
		f := newSummaryFixture(t)
		f.onPathFor([]string{practiceGuitar}, "skill-1")
		f.exerciseBatch("unlinked", "skill-unlinked", 1, 30)

		got := f.summary(t, &guitar, "")

		assert.Zero(t, got.NextStepsTotal)
	})

	t.Run("nodes are grouped, each with its standing and its children", func(t *testing.T) {
		f := newSummaryFixture(t)
		parent := "area"
		f.knowledgeNodes.put(domain.KnowledgeNode{ID: parent, Kind: domain.KnowledgeNodeKindSkill, Key: parent, InstrumentIDs: []string{practiceGuitar}})
		f.knowledgeNodes.put(domain.KnowledgeNode{ID: "leaf", Kind: domain.KnowledgeNodeKindSkill, Key: "leaf", ParentID: &parent, InstrumentIDs: []string{practiceGuitar}})
		f.exercise("e-leaf", "leaf", 30, practiceGuitar)

		got := f.summary(t, &guitar, "")

		require.Len(t, got.Groups, 1)
		require.NotNil(t, got.Groups[0].Area)
		assert.Equal(t, parent, got.Groups[0].Area.ID)
		assert.Equal(t, []string{"leaf"}, got.Children[parent])
		assert.Equal(t, 1, got.Standings["leaf"].Total)
	})

	t.Run("an unknown time zone is rejected on time_zone", func(t *testing.T) {
		f := newSummaryFixture(t)

		_, err := f.svc.Summary(context.Background(), studentCaller(), &guitar, "Mars/Olympus_Mons")

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		require.Len(t, valErr.Fields, 1)
		assert.Equal(t, "time_zone", valErr.Fields[0].Field)
	})

	t.Run("an instrument that doesn't exist is not found", func(t *testing.T) {
		f := newSummaryFixture(t)
		missing := "missing"

		_, err := f.svc.Summary(context.Background(), studentCaller(), &missing, "")

		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a failure reading activity fails the request", func(t *testing.T) {
		f := newSummaryFixture(t)
		f.activity.err = errors.New("mongo down")

		_, err := f.svc.Summary(context.Background(), studentCaller(), &guitar, "")

		require.Error(t, err)
	})
}

func TestPracticeSummaryService_Overview(t *testing.T) {
	guitar, bass := practiceGuitar, practiceBass

	t.Run("days across instruments, learning days, and a card per instrument", func(t *testing.T) {
		f := newSummaryFixture(t)
		f.now = time.Date(2026, 10, 5, 15, 0, 0, 0, time.UTC)
		f.onPathFor([]string{practiceGuitar, practiceBass}, "skill-1")
		f.exercise("e-1", "skill-1", 30, practiceGuitar)
		f.activity.sessions = []domain.FinishedPracticeSession{
			{InstrumentID: &guitar, EndedAt: time.Date(2026, 10, 5, 13, 0, 0, 0, time.UTC)},
			{InstrumentID: &bass, EndedAt: time.Date(2026, 10, 5, 14, 0, 0, 0, time.UTC)},
			{InstrumentID: &bass, EndedAt: time.Date(2026, 10, 4, 14, 0, 0, 0, time.UTC)},
		}
		f.activity.completions = []time.Time{
			time.Date(2026, 10, 1, 14, 0, 0, 0, time.UTC),
			time.Date(2026, 10, 2, 14, 0, 0, 0, time.UTC),
			time.Date(2026, 10, 2, 18, 0, 0, 0, time.UTC),
		}

		got, err := f.svc.Overview(context.Background(), studentCaller(), "America/Sao_Paulo")

		require.NoError(t, err)
		assert.Equal(t, 2, got.PracticeDaysLast7, "two instruments on one day count one day")
		assert.Equal(t, 2, got.LearningDaysLast7)
		require.Len(t, got.Instruments, 2)
		assert.Equal(t, practiceBass, got.Instruments[0].InstrumentID)
		assert.Equal(t, 2, got.Instruments[0].PracticeDaysLast7)
		assert.Nil(t, got.Instruments[0].TopNextStep, "skill-1 has nothing to practise on bass")
		assert.Equal(t, practiceGuitar, got.Instruments[1].InstrumentID)
		assert.Equal(t, 1, got.Instruments[1].PracticeDaysLast7)
		require.NotNil(t, got.Instruments[1].TopNextStep)
		assert.Equal(t, "skill-1", got.Instruments[1].TopNextStep.Node.ID)
	})

	t.Run("an unknown time zone is rejected on time_zone", func(t *testing.T) {
		f := newSummaryFixture(t)

		_, err := f.svc.Overview(context.Background(), studentCaller(), "Mars/Olympus_Mons")

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		require.Len(t, valErr.Fields, 1)
		assert.Equal(t, "time_zone", valErr.Fields[0].Field)
	})
}

func TestPracticeSummaryService_FretboardMap(t *testing.T) {
	cell := func(str, fret int) string { return domain.FretboardCellItemKey(practiceGuitar, str, fret) }

	t.Run("every cell that suits the instrument, with the student's level, and nothing else", func(t *testing.T) {
		f := newSummaryFixture(t)
		f.skill("root-strings")
		f.cells[practiceGuitar] = []domain.ClassifiedItem{
			{ItemKey: cell(6, 1), NodeIDs: []string{"root-strings"}},
			{ItemKey: cell(6, 0), NodeIDs: []string{"root-strings"}},
		}
		f.exercise("name-the-third", "root-strings", 30)
		due := f.now.AddDate(0, 0, 2)
		f.states.put(studentCaller().ID, domain.PracticeItemState{ItemKey: cell(6, 1), RulesVersion: domain.PracticeRulesVersion, Level: domain.KnowledgeLevelAccurate, Counted: 3, Box: 2, DueAt: &due})

		got, err := f.svc.FretboardMap(context.Background(), studentCaller(), practiceGuitar)

		require.NoError(t, err)
		assert.Equal(t, practiceGuitar, got.InstrumentID)
		require.NotNil(t, got.LayoutInstrumentID)
		assert.Equal(t, practiceGuitar, *got.LayoutInstrumentID)
		require.Len(t, got.Cells, 2)
		assert.Equal(t, cell(6, 0), got.Cells[0].ItemKey)
		assert.Equal(t, domain.KnowledgeLevelNew, got.Cells[0].Level)
		assert.Equal(t, cell(6, 1), got.Cells[1].ItemKey)
		assert.Equal(t, domain.KnowledgeLevelAccurate, got.Cells[1].Level)
	})

	t.Run("an instrument without cells has an empty map", func(t *testing.T) {
		f := newSummaryFixture(t)

		got, err := f.svc.FretboardMap(context.Background(), studentCaller(), practiceBass)

		require.NoError(t, err)
		assert.Nil(t, got.LayoutInstrumentID)
		assert.Empty(t, got.Cells)
	})

	t.Run("an instrument that doesn't exist is not found", func(t *testing.T) {
		f := newSummaryFixture(t)

		_, err := f.svc.FretboardMap(context.Background(), studentCaller(), "missing")

		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a failure reading states fails the request", func(t *testing.T) {
		f := newSummaryFixture(t)
		f.cells[practiceGuitar] = []domain.ClassifiedItem{{ItemKey: cell(6, 0)}}
		f.states.err = errors.New("mongo down")

		_, err := f.svc.FretboardMap(context.Background(), studentCaller(), practiceGuitar)

		require.Error(t, err)
	})
}
