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
	return application.NewInstrumentService(repo, idSequence())
}

func TestInstrumentService_CreateInstrument(t *testing.T) {
	guitarTuning := []string{"E", "A", "D", "G", "B", "E"}
	six := 6

	t.Run("a teacher creates a fretted instrument", func(t *testing.T) {
		repo := newFakeInstrumentRepository()
		svc := newInstrumentService(repo)

		got, err := svc.CreateInstrument(context.Background(), teacherCaller(), "Guitar", domain.InstrumentFamilyFretted, &six, guitarTuning, nil)

		require.NoError(t, err)
		assert.NotEmpty(t, got.ID)
		assert.Equal(t, domain.InstrumentFamilyFretted, got.Family)
		stored, err := repo.GetByID(context.Background(), got.ID)
		require.NoError(t, err)
		assert.Equal(t, got, stored)
	})

	t.Run("an admin creates a keyboard instrument", func(t *testing.T) {
		svc := newInstrumentService(newFakeInstrumentRepository())

		got, err := svc.CreateInstrument(context.Background(), adminCaller(), "Piano", domain.InstrumentFamilyKeyboard, nil, nil, &domain.KeyRange{Lowest: "A0", Highest: "C8"})

		require.NoError(t, err)
		assert.Equal(t, domain.InstrumentFamilyKeyboard, got.Family)
	})

	t.Run("a student cannot create an instrument", func(t *testing.T) {
		repo := newFakeInstrumentRepository()
		svc := newInstrumentService(repo)

		_, err := svc.CreateInstrument(context.Background(), studentCaller(), "Guitar", domain.InstrumentFamilyFretted, &six, guitarTuning, nil)

		require.ErrorIs(t, err, domain.ErrForbidden)
		list, listErr := repo.List(context.Background())
		require.NoError(t, listErr)
		assert.Empty(t, list)
	})

	t.Run("an invalid shape is rejected without persisting", func(t *testing.T) {
		repo := newFakeInstrumentRepository()
		svc := newInstrumentService(repo)

		_, err := svc.CreateInstrument(context.Background(), teacherCaller(), "Piano", domain.InstrumentFamilyKeyboard, nil, guitarTuning, &domain.KeyRange{Lowest: "A0", Highest: "C8"})

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, "tuning", valErr.Fields[0].Field)
		list, listErr := repo.List(context.Background())
		require.NoError(t, listErr)
		assert.Empty(t, list)
	})
}

func TestInstrumentService_ListInstruments(t *testing.T) {
	t.Run("returns every known instrument", func(t *testing.T) {
		repo := newFakeInstrumentRepository()
		repo.put(domain.Instrument{ID: "a", Name: "Guitar", Family: domain.InstrumentFamilyFretted})
		repo.put(domain.Instrument{ID: "b", Name: "Piano", Family: domain.InstrumentFamilyKeyboard})
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
