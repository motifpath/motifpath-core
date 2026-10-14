package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestPracticeRung_Valid(t *testing.T) {
	for _, r := range domain.PracticeRungs {
		assert.True(t, r.Valid(), r)
	}
	assert.False(t, domain.PracticeRung("listen").Valid())
	assert.False(t, domain.PracticeRung("").Valid())
}

func TestPracticeRungs_climbTheLadderInOrder(t *testing.T) {
	assert.Equal(t, []domain.PracticeRung{
		domain.PracticeRungKnow, domain.PracticeRungHear, domain.PracticeRungPlayClean, domain.PracticeRungPlayInTime,
		domain.PracticeRungMove, domain.PracticeRungTarget, domain.PracticeRungUse,
	}, domain.PracticeRungs)
}

func ptr(s string) *string { return &s }

func cellTemplates() []domain.DrillTemplate {
	return []domain.DrillTemplate{
		{Key: "fretboard_cell:name_the_note", ItemKind: domain.PracticeItemKindFretboardCell, Rung: domain.PracticeRungKnow, SecondWay: ptr("fretboard_cell:find_the_note")},
		{Key: "fretboard_cell:find_the_note", ItemKind: domain.PracticeItemKindFretboardCell, Rung: domain.PracticeRungKnow, SecondWay: ptr("fretboard_cell:name_the_note")},
		{Key: "exercise:text_response", ItemKind: domain.PracticeItemKindExercise, Rung: domain.PracticeRungKnow},
	}
}

func TestCheckDrillTemplates(t *testing.T) {
	t.Run("templates whose second ways ask the same kind of item install", func(t *testing.T) {
		require.NoError(t, domain.CheckDrillTemplates(cellTemplates()))
	})

	t.Run("a second way that asks another kind of item fails, naming the template", func(t *testing.T) {
		templates := append(cellTemplates(), domain.DrillTemplate{
			Key: "diatonic_chord:numeral_to_chord", ItemKind: "diatonic_chord", Rung: domain.PracticeRungKnow,
			SecondWay: ptr("fretboard_cell:find_the_note"),
		})
		err := domain.CheckDrillTemplates(templates)
		require.ErrorIs(t, err, domain.ErrInvalidDrillCatalog)
		assert.Contains(t, err.Error(), "diatonic_chord:numeral_to_chord")
	})

	t.Run("a second way no template has fails, naming the template", func(t *testing.T) {
		templates := cellTemplates()
		templates[0].SecondWay = ptr("fretboard_cell:hear_the_note")
		err := domain.CheckDrillTemplates(templates)
		require.ErrorIs(t, err, domain.ErrInvalidDrillCatalog)
		assert.Contains(t, err.Error(), "fretboard_cell:name_the_note")
	})

	t.Run("a template that is its own second way fails", func(t *testing.T) {
		templates := cellTemplates()
		templates[0].SecondWay = ptr("fretboard_cell:name_the_note")
		require.ErrorIs(t, domain.CheckDrillTemplates(templates), domain.ErrInvalidDrillCatalog)
	})

	t.Run("a template with no rung on the ladder fails, naming it", func(t *testing.T) {
		templates := cellTemplates()
		templates[2].Rung = "listen"
		err := domain.CheckDrillTemplates(templates)
		require.ErrorIs(t, err, domain.ErrInvalidDrillCatalog)
		assert.Contains(t, err.Error(), "exercise:text_response")
	})
}

func TestItemRung(t *testing.T) {
	templates := map[string]domain.DrillTemplate{}
	for _, tpl := range append(cellTemplates(), domain.DrillTemplate{Key: "exercise:audio_recognition", ItemKind: domain.PracticeItemKindExercise, Rung: domain.PracticeRungHear}) {
		templates[tpl.Key] = tpl
	}

	t.Run("a drill item takes its template's rung", func(t *testing.T) {
		item := domain.PracticeSessionItem{Kind: domain.PracticeItemKindExercise, Exercise: &domain.Exercise{ExerciseType: domain.ExerciseTypeAudioRecognition}}
		assert.Equal(t, domain.PracticeRungHear, domain.ItemRung(item, templates))
	})

	t.Run("a play-along is played in time", func(t *testing.T) {
		item := domain.PracticeSessionItem{Kind: domain.PracticeItemKindPlayAlong, PlayAlong: &domain.PlannedPlayAlong{}}
		assert.Equal(t, domain.PracticeRungPlayInTime, domain.ItemRung(item, templates))
	})

	t.Run("an item whose template isn't installed is know", func(t *testing.T) {
		item := domain.PracticeSessionItem{Kind: domain.PracticeItemKindExercise, Exercise: &domain.Exercise{ExerciseType: domain.ExerciseTypeImageChoice}}
		assert.Equal(t, domain.PracticeRungKnow, domain.ItemRung(item, templates))
	})
}
