package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestSubjectRungs(t *testing.T) {
	later := rollupNow.AddDate(0, 0, 5)
	accurate := func(keys ...string) map[string]domain.PracticeItemState {
		states := map[string]domain.PracticeItemState{}
		for _, k := range keys {
			states[k] = domain.PracticeItemState{ItemKey: k, Level: domain.KnowledgeLevelAccurate, Counted: 3, Box: 2, DueAt: &later, RulesVersion: domain.PracticeRulesVersion}
		}
		return states
	}
	know, hear, playClean := itemKeys("know", 5), itemKeys("hear", 5), itemKeys("clean", 5)

	tests := []struct {
		name        string
		material    map[domain.PracticeRung][]string
		states      map[string]domain.PracticeItemState
		wantCurrent domain.PracticeRung
		wantNew     []domain.PracticeRung
	}{
		{
			name:        "new to the subject: its first two rungs",
			material:    map[domain.PracticeRung][]string{domain.PracticeRungKnow: know, domain.PracticeRungHear: hear, domain.PracticeRungPlayClean: playClean},
			wantCurrent: domain.PracticeRungKnow,
			wantNew:     []domain.PracticeRung{domain.PracticeRungKnow, domain.PracticeRungHear},
		},
		{
			name:        "a rung accurate: the next two",
			material:    map[domain.PracticeRung][]string{domain.PracticeRungKnow: know, domain.PracticeRungHear: hear, domain.PracticeRungPlayClean: playClean},
			states:      accurate(know...),
			wantCurrent: domain.PracticeRungHear,
			wantNew:     []domain.PracticeRung{domain.PracticeRungHear, domain.PracticeRungPlayClean},
		},
		{
			name:        "80% of a rung accurate is accurate",
			material:    map[domain.PracticeRung][]string{domain.PracticeRungKnow: know, domain.PracticeRungHear: hear},
			states:      accurate(know[:4]...),
			wantCurrent: domain.PracticeRungHear,
			wantNew:     []domain.PracticeRung{domain.PracticeRungHear},
		},
		{
			name:        "rungs with no material are skipped",
			material:    map[domain.PracticeRung][]string{domain.PracticeRungKnow: know, domain.PracticeRungPlayClean: playClean},
			wantCurrent: domain.PracticeRungKnow,
			wantNew:     []domain.PracticeRung{domain.PracticeRungKnow, domain.PracticeRungPlayClean},
		},
		{
			name:        "every rung accurate: its top rung",
			material:    map[domain.PracticeRung][]string{domain.PracticeRungKnow: know},
			states:      accurate(know[:4]...),
			wantCurrent: domain.PracticeRungKnow,
			wantNew:     []domain.PracticeRung{domain.PracticeRungKnow},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current, newRungs := domain.SubjectRungs(tt.material, tt.states, rollupNow)
			assert.Equal(t, tt.wantCurrent, current)
			assert.Equal(t, tt.wantNew, newRungs)
		})
	}

	t.Run("no material: no rung", func(t *testing.T) {
		current, newRungs := domain.SubjectRungs(nil, nil, rollupNow)
		assert.Empty(t, current)
		assert.Empty(t, newRungs)
	})
}
