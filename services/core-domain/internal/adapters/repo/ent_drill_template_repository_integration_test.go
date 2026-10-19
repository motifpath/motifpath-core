//go:build integration

package repo

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestEntDrillTemplateRepository_List(t *testing.T) {
	ctx := context.Background()
	client := setupPostgres(t)
	repository := NewEntDrillTemplateRepository(client)

	t.Run("no templates lists none", func(t *testing.T) {
		got, err := repository.List(ctx)

		require.NoError(t, err)
		assert.Empty(t, got)
	})

	find := "fretboard_cell:find_the_note"
	name := "fretboard_cell:name_the_note"
	nameID, findID, audioID, heardID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	client.DrillTemplate.Create().SetID(nameID).SetKey(name).SetItemKind("fretboard_cell").SetResponseType("name_the_note").
		SetRung("know").SetSecondWay(find).SetTimed(true).SetNames(map[string]string{"en": "Name the note"}).SaveX(ctx)
	client.DrillTemplate.Create().SetID(findID).SetKey(find).SetItemKind("fretboard_cell").SetResponseType("find_the_note").
		SetRung("know").SetSecondWay(name).SetTimed(true).SetNames(map[string]string{"en": "Find the note"}).SaveX(ctx)
	client.DrillTemplate.Create().SetID(audioID).SetKey("exercise:audio_selection").SetItemKind("exercise").SetResponseType("option_choice").
		SetRung("hear").SetTimed(true).SetNames(map[string]string{"en": "Sound choice"}).SaveX(ctx)
	client.DrillTemplate.Create().SetID(heardID).SetKey("heard_progression:progression_by_number").SetItemKind("heard_progression").
		SetResponseType("generated_choice").SetRung("hear").SetComesBack(true).SetTimed(true).
		SetNames(map[string]string{"en": "Hear the progression"}).SaveX(ctx)

	t.Run("lists every template with its rung and second way, by key", func(t *testing.T) {
		got, err := repository.List(ctx)

		require.NoError(t, err)
		require.Len(t, got, 4)
		assert.Equal(t, domain.DrillTemplate{
			ID: audioID.String(), Key: "exercise:audio_selection", ItemKind: domain.PracticeItemKindExercise, ResponseType: "option_choice",
			Rung: domain.PracticeRungHear, Timed: true, Names: domain.LocalizedText{"en": "Sound choice"},
		}, got[0])
		assert.Equal(t, find, got[1].Key)
		require.NotNil(t, got[1].SecondWay)
		assert.Equal(t, name, *got[1].SecondWay)
		assert.Equal(t, domain.PracticeRungKnow, got[2].Rung)
	})

	t.Run("a template marked to come back comes back, and no other", func(t *testing.T) {
		got, err := repository.List(ctx)

		require.NoError(t, err)
		require.Len(t, got, 4)
		assert.Equal(t, []bool{false, false, false, true}, []bool{got[0].ComesBack, got[1].ComesBack, got[2].ComesBack, got[3].ComesBack})
	})
}
