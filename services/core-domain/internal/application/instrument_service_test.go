package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

func newInstrumentService(repo *fakeInstrumentRepository) *application.InstrumentService {
	return application.NewInstrumentService(repo, newFakeLanguageRepository(), idSequence())
}

var bilingualGuitar = map[string]string{"en": "Guitar", "pt_BR": "Violão"}

func TestInstrumentService_CreateInstrument(t *testing.T) {
	guitarTuning := []string{"E", "A", "D", "G", "B", "E"}
	six := 6

	t.Run("a teacher creates a fretted instrument named in every language", func(t *testing.T) {
		repo := newFakeInstrumentRepository()
		svc := newInstrumentService(repo)

		got, err := svc.CreateInstrument(context.Background(), teacherCaller(), bilingualGuitar, domain.InstrumentFamilyFretted, &six, guitarTuning, nil)

		require.NoError(t, err)
		assert.NotEmpty(t, got.ID)
		assert.Equal(t, domain.InstrumentFamilyFretted, got.Family)
		assert.Equal(t, domain.LocalizedText{"en": "Guitar", "pt_BR": "Violão"}, got.Names)
		stored, err := repo.GetByID(context.Background(), got.ID)
		require.NoError(t, err)
		assert.Equal(t, got, stored)
	})

	t.Run("an admin creates a keyboard instrument", func(t *testing.T) {
		svc := newInstrumentService(newFakeInstrumentRepository())

		got, err := svc.CreateInstrument(context.Background(), adminCaller(), map[string]string{"en": "Piano", "pt_BR": "Piano"}, domain.InstrumentFamilyKeyboard, nil, nil, &domain.KeyRange{Lowest: "A0", Highest: "C8"})

		require.NoError(t, err)
		assert.Equal(t, domain.InstrumentFamilyKeyboard, got.Family)
	})

	t.Run("a name missing one of the offered languages is rejected without persisting", func(t *testing.T) {
		repo := newFakeInstrumentRepository()
		svc := newInstrumentService(repo)

		_, err := svc.CreateInstrument(context.Background(), teacherCaller(), map[string]string{"en": "Guitar"}, domain.InstrumentFamilyFretted, &six, guitarTuning, nil)

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, "names", valErr.Fields[0].Field)
		list, listErr := repo.List(context.Background())
		require.NoError(t, listErr)
		assert.Empty(t, list)
	})

	t.Run(`the "any" marker is never an offered language`, func(t *testing.T) {
		svc := newInstrumentService(newFakeInstrumentRepository())

		_, err := svc.CreateInstrument(context.Background(), teacherCaller(), map[string]string{"en": "Guitar", "pt_BR": "Violão", "any": "Guitar"}, domain.InstrumentFamilyFretted, &six, guitarTuning, nil)

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, "names", valErr.Fields[0].Field)
	})

	t.Run("a student cannot create an instrument", func(t *testing.T) {
		repo := newFakeInstrumentRepository()
		svc := newInstrumentService(repo)

		_, err := svc.CreateInstrument(context.Background(), studentCaller(), bilingualGuitar, domain.InstrumentFamilyFretted, &six, guitarTuning, nil)

		require.ErrorIs(t, err, domain.ErrForbidden)
		list, listErr := repo.List(context.Background())
		require.NoError(t, listErr)
		assert.Empty(t, list)
	})

	t.Run("an invalid shape is rejected without persisting", func(t *testing.T) {
		repo := newFakeInstrumentRepository()
		svc := newInstrumentService(repo)

		_, err := svc.CreateInstrument(context.Background(), teacherCaller(), map[string]string{"en": "Piano", "pt_BR": "Piano"}, domain.InstrumentFamilyKeyboard, nil, guitarTuning, &domain.KeyRange{Lowest: "A0", Highest: "C8"})

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, "tuning", valErr.Fields[0].Field)
		list, listErr := repo.List(context.Background())
		require.NoError(t, listErr)
		assert.Empty(t, list)
	})
}

func TestInstrumentService_UpdateInstrumentNames(t *testing.T) {
	ctx := context.Background()
	six := 6
	seed := func(t *testing.T, repo *fakeInstrumentRepository) domain.Instrument {
		t.Helper()
		// An instrument that predates per-language names: English only.
		instrument := domain.Instrument{ID: "guitar", Names: domain.LocalizedText{"en": "Guitar"}, Family: domain.InstrumentFamilyFretted, StringCount: &six, Tuning: []string{"E", "A", "D", "G", "B", "E"}}
		repo.put(instrument)
		return instrument
	}

	t.Run("an admin replaces the names, leaving the family and shape alone", func(t *testing.T) {
		repo := newFakeInstrumentRepository()
		original := seed(t, repo)
		svc := newInstrumentService(repo)

		got, err := svc.UpdateInstrumentNames(ctx, adminCaller(), original.ID, bilingualGuitar)

		require.NoError(t, err)
		assert.Equal(t, domain.LocalizedText{"en": "Guitar", "pt_BR": "Violão"}, got.Names)
		assert.Equal(t, original.Family, got.Family)
		assert.Equal(t, original.Tuning, got.Tuning)
		stored, err := repo.GetByID(ctx, original.ID)
		require.NoError(t, err)
		assert.Equal(t, got, stored)
	})

	t.Run("names missing a language are rejected and the stored names are unchanged", func(t *testing.T) {
		repo := newFakeInstrumentRepository()
		original := seed(t, repo)
		svc := newInstrumentService(repo)

		_, err := svc.UpdateInstrumentNames(ctx, adminCaller(), original.ID, map[string]string{"en": "Guitar!"})

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, "names", valErr.Fields[0].Field)
		stored, getErr := repo.GetByID(ctx, original.ID)
		require.NoError(t, getErr)
		assert.Equal(t, original.Names, stored.Names)
	})

	t.Run("teachers and students cannot rename an instrument", func(t *testing.T) {
		for _, caller := range []domain.User{teacherCaller(), studentCaller()} {
			repo := newFakeInstrumentRepository()
			original := seed(t, repo)
			svc := newInstrumentService(repo)

			_, err := svc.UpdateInstrumentNames(ctx, caller, original.ID, bilingualGuitar)

			require.ErrorIs(t, err, domain.ErrForbidden, "role %s", caller.Role)
		}
	})

	t.Run("an unknown instrument is not found", func(t *testing.T) {
		svc := newInstrumentService(newFakeInstrumentRepository())

		_, err := svc.UpdateInstrumentNames(ctx, adminCaller(), "nope", bilingualGuitar)

		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestInstrumentService_ListInstruments(t *testing.T) {
	t.Run("returns every known instrument", func(t *testing.T) {
		repo := newFakeInstrumentRepository()
		repo.put(domain.Instrument{ID: "a", Names: domain.LocalizedText{"en": "Guitar"}, Family: domain.InstrumentFamilyFretted})
		repo.put(domain.Instrument{ID: "b", Names: domain.LocalizedText{"en": "Piano"}, Family: domain.InstrumentFamilyKeyboard})
		svc := newInstrumentService(repo)

		got, err := svc.ListInstruments(context.Background())

		require.NoError(t, err)
		require.Len(t, got, 2)
	})

	t.Run("returns an empty list when none exist", func(t *testing.T) {
		svc := newInstrumentService(newFakeInstrumentRepository())

		got, err := svc.ListInstruments(context.Background())

		require.NoError(t, err)
		assert.Empty(t, got)
	})
}
