package application_test

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

// fakeSessionEnds is a ports.SessionEndReader over the sessions whose end
// the worker recorded, by student.
type fakeSessionEnds struct {
	ended map[string][]string
}

func (f *fakeSessionEnds) SessionEnded(_ context.Context, studentID, sessionID string) (bool, error) {
	for _, id := range f.ended[studentID] {
		if id == sessionID {
			return true, nil
		}
	}
	return false, nil
}

func TestPracticeSessionService_SessionSubjectProgress(t *testing.T) {
	// progressions puts the student on progressions, part_of harmony, with 5
	// know and 5 hear exercises, and the worker's record of their session
	// "s1"'s end.
	progressions := func(t *testing.T) (f *practiceFixture, know []string, ends *fakeSessionEnds) {
		f = newPracticeFixture(t)
		f.childSkill("progressions", "harmony")
		know = f.exercisesOf("know", "progressions", domain.ExerciseTypeTextResponse, 5)
		f.exercisesOf("hear", "progressions", domain.ExerciseTypeAudioRecognition, 5)
		f.onPath("progressions")
		ends = &fakeSessionEnds{ended: map[string][]string{studentCaller().ID: {"s1"}}}
		f.svc.WithSessionEnds(ends)
		return f, know, ends
	}
	learning := func(f *practiceFixture, keys ...string) {
		for _, k := range keys {
			f.stateOf(k, domain.PracticeItemState{Level: domain.KnowledgeLevelLearning, Counted: 2, Box: 1, DueAt: f.inDays(2)})
		}
	}
	read := func(t *testing.T, f *practiceFixture, sessionID string, nodeIDs ...string) domain.PracticeSessionSubjectProgress {
		t.Helper()
		progress, err := f.svc.SessionSubjectProgress(context.Background(), studentCaller(), sessionID, guitar(), nodeIDs)
		require.NoError(t, err)
		return progress
	}

	tests := []struct {
		name  string
		setUp func(f *practiceFixture, know []string)
		// wantRung and wantLevel are the plan's rung and level on it for progressions.
		wantRung  domain.PracticeRung
		wantLevel domain.KnowledgeLevel
	}{
		{"a subject new to the student is at its first rung, at level new", func(*practiceFixture, []string) {}, domain.PracticeRungKnow, domain.KnowledgeLevelNew},
		{"a learning rung stays the current one", func(f *practiceFixture, know []string) { learning(f, know...) }, domain.PracticeRungKnow, domain.KnowledgeLevelLearning},
		{"an accurate rung moves the subject to the next one", func(f *practiceFixture, know []string) { f.accurate(know...) }, domain.PracticeRungHear, domain.KnowledgeLevelNew},
	}
	for _, tt := range tests {
		t.Run("the plan names "+tt.name, func(t *testing.T) {
			f, know, _ := progressions(t)
			tt.setUp(f, know)

			plan := f.choose(t, guitar(), "progressions", 10)

			require.Len(t, plan.FocusRungs, len(plan.FocusNodes))
			assert.Equal(t, domain.RungLevel{Rung: tt.wantRung, Level: tt.wantLevel}, plan.FocusRungs[0])
		})
	}

	t.Run("Today's mix names each subject's rung too", func(t *testing.T) {
		f, know, _ := progressions(t)
		learning(f, know...)

		plan := f.compose(t, practiceGuitar, 10)

		require.Len(t, plan.FocusRungs, len(plan.FocusNodes))
		i := slices.IndexFunc(plan.FocusNodes, func(n domain.KnowledgeNode) bool { return n.ID == "progressions" })
		require.GreaterOrEqual(t, i, 0, "progressions is a subject")
		assert.Equal(t, domain.RungLevel{Rung: domain.PracticeRungKnow, Level: domain.KnowledgeLevelLearning}, plan.FocusRungs[i])
	})

	t.Run("once the session's end is recorded, it is caught up, with each rung's level now", func(t *testing.T) {
		f, know, _ := progressions(t)
		f.accurate(know...)

		progress := read(t, f, "s1", "progressions")

		assert.True(t, progress.CaughtUp)
		require.Len(t, progress.Subjects, 1)
		assert.Equal(t, domain.SubjectProgress{
			NodeID: "progressions", CurrentRung: domain.PracticeRungHear,
			Rungs: []domain.RungLevel{{Rung: domain.PracticeRungKnow, Level: domain.KnowledgeLevelAccurate}, {Rung: domain.PracticeRungHear, Level: domain.KnowledgeLevelNew}},
		}, progress.Subjects[0])
	})

	t.Run("before its end is recorded, a session is not caught up", func(t *testing.T) {
		f, _, _ := progressions(t)

		assert.False(t, read(t, f, "s2", "progressions").CaughtUp)
	})

	t.Run("another student's session is never caught up", func(t *testing.T) {
		f, _, ends := progressions(t)
		ends.ended["someone-else"] = []string{"s3"}

		assert.False(t, read(t, f, "s3", "progressions").CaughtUp)
	})

	t.Run("subjects come back in the order asked; a wide node over its subtree, one with no material without a rung", func(t *testing.T) {
		f, _, _ := progressions(t)
		f.skill("unpractised")

		progress := read(t, f, "s1", "unpractised", "progressions", "harmony")

		require.Len(t, progress.Subjects, 3)
		assert.Equal(t, domain.SubjectProgress{NodeID: "unpractised", Rungs: []domain.RungLevel{}}, progress.Subjects[0])
		assert.Equal(t, "progressions", progress.Subjects[1].NodeID)
		assert.Equal(t, progress.Subjects[1].Rungs, progress.Subjects[2].Rungs, "harmony's only items are its child's")
	})

	t.Run("in the head, every instrument's items count together", func(t *testing.T) {
		f, _, _ := progressions(t)
		f.childSkill("notes", "fretboard")
		f.accurate(f.cellsOn(practiceGuitar, "notes", 3, 6)...)
		learning(f, f.cellsOn(practiceBass, "notes", 3, 4)...)
		f.onPathFor([]string{practiceGuitar, practiceBass}, "progressions")

		progress, err := f.svc.SessionSubjectProgress(context.Background(), studentCaller(), "s1", nil, []string{"notes"})

		require.NoError(t, err)
		assert.Equal(t, domain.SubjectProgress{
			NodeID: "notes", CurrentRung: domain.PracticeRungKnow,
			Rungs: []domain.RungLevel{{Rung: domain.PracticeRungKnow, Level: domain.KnowledgeLevelLearning}},
		}, progress.Subjects[0])
	})

	errs := []struct {
		name    string
		nodeIDs []string
		want    error
	}{
		{"no subject is rejected", nil, domain.ErrValidation},
		{"more than three subjects are rejected", []string{"progressions", "harmony", "progressions", "harmony"}, domain.ErrValidation},
		{"a subject that doesn't exist is not found", []string{"missing"}, domain.ErrNotFound},
	}
	for _, tt := range errs {
		t.Run(tt.name, func(t *testing.T) {
			f, _, _ := progressions(t)

			_, err := f.svc.SessionSubjectProgress(context.Background(), studentCaller(), "s1", guitar(), tt.nodeIDs)

			require.ErrorIs(t, err, tt.want)
		})
	}
}
