package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

// anyLanguage tags an exercise as language-agnostic, offered whatever the
// student's locale.
func anyLanguage() []domain.Language {
	return []domain.Language{{Code: domain.LanguageCodeAny}}
}

// ptStudent is the student of studentCaller with a pt-BR locale.
func ptStudent() domain.User {
	student := studentCaller()
	student.Locale = domain.Language{Code: "pt_BR"}
	return student
}

// exerciseIn puts a 30-second exercise for every instrument, classified
// under skillID, available in languageCodes.
func (f *practiceFixture) exerciseIn(id, skillID string, languageCodes ...string) {
	f.skill(skillID)
	seconds := 30
	languages := make([]domain.Language, len(languageCodes))
	for i, code := range languageCodes {
		languages[i] = domain.Language{Code: code}
	}
	require.NoError(f.t, f.exercises.Create(context.Background(), domain.Exercise{
		ID: id, Title: id, ExerciseType: domain.ExerciseTypeTextResponse,
		Skills: []domain.KnowledgeNode{{ID: skillID}}, EstimatedDurationSeconds: &seconds, Languages: languages,
	}))
}

func TestPracticeSessionService_ComposePlanLanguage(t *testing.T) {
	compose := func(t *testing.T, f *practiceFixture, focusNodeID *string) domain.PracticeSessionPlan {
		t.Helper()
		plan, err := f.svc.ComposePlan(context.Background(), ptStudent(), guitar(), focusNodeID, 10)
		require.NoError(t, err)
		return plan
	}

	t.Run("only exercises in the student's locale or for any language are picked new", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.exerciseIn("name-the-third", "skill-1", "en")
		f.exerciseIn("nomeie-a-terca", "skill-1", "pt_BR")
		f.exerciseIn("hear-the-third", "skill-1", domain.LanguageCodeAny)
		f.exerciseIn("both", "skill-1", "en", "pt_BR")

		plan := compose(t, f, nil)

		assert.ElementsMatch(t, []string{"exercise:nomeie-a-terca", "exercise:hear-the-third", "exercise:both"}, planKeys(plan))
	})

	t.Run("a due exercise not in the student's locale is not reviewed", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.exerciseIn("name-the-third", "skill-1", "en")
		f.exerciseIn("nomeie-a-terca", "skill-1", "pt_BR")
		f.due("exercise:name-the-third", "exercise:nomeie-a-terca")

		plan := compose(t, f, nil)

		assert.Equal(t, []string{"exercise:nomeie-a-terca"}, planKeys(plan))
	})

	t.Run("an exercise not in the student's locale is never stretched to", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.applies("skill-1", "skill-ready")
		f.exerciseIn("stretch-en", "skill-ready", "en")
		f.exerciseIn("stretch-pt", "skill-ready", "pt_BR")

		plan := compose(t, f, nil)

		assert.Equal(t, []string{"exercise:stretch-pt"}, planKeys(plan))
		assert.Equal(t, domain.PracticePickStretch, plan.Items[0].Reason)
	})

	t.Run("a chosen session holds only exercises in the student's locale", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.exerciseIn("name-the-progression", "progressions", "en")
		f.exerciseIn("nomeie-a-progressao", "progressions", "pt_BR")
		chosen := "progressions"

		plan := compose(t, f, &chosen)

		assert.Equal(t, []string{"exercise:nomeie-a-progressao"}, planKeys(plan))
	})

	t.Run("a chosen node whose only exercises aren't in the student's locale has nothing to practise", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.exerciseIn("name-the-progression", "progressions", "en")
		chosen := "progressions"

		_, err := f.svc.ComposePlan(context.Background(), ptStudent(), guitar(), &chosen, 10)

		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestPracticeSessionService_ListSubjectsLanguage(t *testing.T) {
	t.Run("a node whose only exercises aren't in the student's locale is not offered, and counts only the items they can be offered", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.childSkill("minor-triads", "harmony")
		f.exerciseIn("hear-minor", "minor-triads", "en")
		f.childSkill("progressions", "harmony")
		f.exerciseIn("name-the-progression", "progressions", "en")
		f.exerciseIn("nomeie-a-progressao", "progressions", "pt_BR")

		list, err := f.svc.ListSubjects(context.Background(), ptStudent(), guitar())

		require.NoError(t, err)
		counts := map[string]int{}
		for _, g := range list.Groups {
			for _, s := range g.Subjects {
				counts[s.Node.ID] = s.ItemCount
			}
		}
		assert.Equal(t, map[string]int{"harmony": 1, "progressions": 1}, counts)
	})
}
