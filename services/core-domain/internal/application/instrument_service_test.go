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
	return application.NewInstrumentService(repo, newFakeVoiceRepository(), newFakeLanguageRepository(), idSequence())
}

var (
	bilingualGuitar = map[string]string{"en": "Guitar", "pt_BR": "Violão"}
	guitarTuning    = []string{"E2", "A2", "D3", "G3", "B3", "E4"}
)

func TestInstrumentService_CreateInstrument(t *testing.T) {
	six := 6

	t.Run("a teacher creates a fretted instrument named in every language", func(t *testing.T) {
		repo := newFakeInstrumentRepository()
		svc := newInstrumentService(repo)

		got, err := svc.CreateInstrument(context.Background(), teacherCaller(), bilingualGuitar, domain.InstrumentFamilyFretted, &six, guitarTuning, nil, "acoustic-guitar")

		require.NoError(t, err)
		assert.NotEmpty(t, got.ID)
		assert.Equal(t, domain.InstrumentFamilyFretted, got.Family)
		assert.Equal(t, domain.LocalizedText{"en": "Guitar", "pt_BR": "Violão"}, got.Names)
		assert.Equal(t, "acoustic-guitar", got.DefaultVoiceID)
		stored, err := repo.GetByID(context.Background(), got.ID)
		require.NoError(t, err)
		assert.Equal(t, got, stored)
	})

	t.Run("an admin creates a keyboard instrument", func(t *testing.T) {
		svc := newInstrumentService(newFakeInstrumentRepository())

		got, err := svc.CreateInstrument(context.Background(), adminCaller(), map[string]string{"en": "Piano", "pt_BR": "Piano"}, domain.InstrumentFamilyKeyboard, nil, nil, &domain.KeyRange{Lowest: "A0", Highest: "C8"}, "piano")

		require.NoError(t, err)
		assert.Equal(t, domain.InstrumentFamilyKeyboard, got.Family)
		assert.Equal(t, "piano", got.DefaultVoiceID)
	})

	rejected := []struct {
		name           string
		names          map[string]string
		family         domain.InstrumentFamily
		stringCount    *int
		tuning         []string
		keyRange       *domain.KeyRange
		defaultVoiceID string
		wantField      string
	}{
		{name: "a name missing one of the offered languages", names: map[string]string{"en": "Guitar"}, family: domain.InstrumentFamilyFretted, stringCount: &six, tuning: guitarTuning, defaultVoiceID: "acoustic-guitar", wantField: "names"},
		{name: `the "any" marker as a language`, names: map[string]string{"en": "Guitar", "pt_BR": "Violão", "any": "Guitar"}, family: domain.InstrumentFamilyFretted, stringCount: &six, tuning: guitarTuning, defaultVoiceID: "acoustic-guitar", wantField: "names"},
		{name: "an invalid shape", names: map[string]string{"en": "Piano", "pt_BR": "Piano"}, family: domain.InstrumentFamilyKeyboard, tuning: guitarTuning, keyRange: &domain.KeyRange{Lowest: "A0", Highest: "C8"}, defaultVoiceID: "piano", wantField: "tuning"},
		{name: "a tuning without octaves", names: bilingualGuitar, family: domain.InstrumentFamilyFretted, stringCount: &six, tuning: []string{"E", "A", "D", "G", "B", "E"}, defaultVoiceID: "acoustic-guitar", wantField: "tuning"},
		{name: "no default voice", names: bilingualGuitar, family: domain.InstrumentFamilyFretted, stringCount: &six, tuning: guitarTuning, wantField: "default_voice_id"},
		{name: "a default voice that does not exist", names: bilingualGuitar, family: domain.InstrumentFamilyFretted, stringCount: &six, tuning: guitarTuning, defaultVoiceID: "banjo", wantField: "default_voice_id"},
		{name: "a default voice of another family", names: bilingualGuitar, family: domain.InstrumentFamilyFretted, stringCount: &six, tuning: guitarTuning, defaultVoiceID: "piano", wantField: "default_voice_id"},
	}
	for _, tt := range rejected {
		t.Run(tt.name+" is rejected without persisting", func(t *testing.T) {
			repo := newFakeInstrumentRepository()
			svc := newInstrumentService(repo)

			_, err := svc.CreateInstrument(context.Background(), teacherCaller(), tt.names, tt.family, tt.stringCount, tt.tuning, tt.keyRange, tt.defaultVoiceID)

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, tt.wantField, valErr.Fields[0].Field)
			list, listErr := repo.List(context.Background())
			require.NoError(t, listErr)
			assert.Empty(t, list)
		})
	}

	t.Run("a student cannot create an instrument", func(t *testing.T) {
		repo := newFakeInstrumentRepository()
		svc := newInstrumentService(repo)

		_, err := svc.CreateInstrument(context.Background(), studentCaller(), bilingualGuitar, domain.InstrumentFamilyFretted, &six, guitarTuning, nil, "acoustic-guitar")

		require.ErrorIs(t, err, domain.ErrForbidden)
		list, listErr := repo.List(context.Background())
		require.NoError(t, listErr)
		assert.Empty(t, list)
	})
}

func TestInstrumentService_UpdateInstrument(t *testing.T) {
	ctx := context.Background()
	six := 6
	seed := func(t *testing.T, repo *fakeInstrumentRepository) domain.Instrument {
		t.Helper()
		// An instrument that predates per-language names: English only.
		instrument := domain.Instrument{ID: "guitar", Names: domain.LocalizedText{"en": "Guitar"}, Family: domain.InstrumentFamilyFretted, StringCount: &six, Tuning: guitarTuning, DefaultVoiceID: "acoustic-guitar"}
		repo.put(instrument)
		return instrument
	}

	t.Run("an admin replaces the names, leaving the family, shape and voice alone", func(t *testing.T) {
		repo := newFakeInstrumentRepository()
		original := seed(t, repo)
		svc := newInstrumentService(repo)

		got, err := svc.UpdateInstrument(ctx, adminCaller(), original.ID, bilingualGuitar, nil)

		require.NoError(t, err)
		assert.Equal(t, domain.LocalizedText{"en": "Guitar", "pt_BR": "Violão"}, got.Names)
		assert.Equal(t, original.Family, got.Family)
		assert.Equal(t, original.Tuning, got.Tuning)
		assert.Equal(t, "acoustic-guitar", got.DefaultVoiceID)
		stored, err := repo.GetByID(ctx, original.ID)
		require.NoError(t, err)
		assert.Equal(t, got, stored)
	})

	t.Run("an admin changes the default voice to another voice of the family", func(t *testing.T) {
		repo := newFakeInstrumentRepository()
		original := seed(t, repo)
		svc := newInstrumentService(repo)
		electric := "electric-guitar"

		got, err := svc.UpdateInstrument(ctx, adminCaller(), original.ID, bilingualGuitar, &electric)

		require.NoError(t, err)
		assert.Equal(t, "electric-guitar", got.DefaultVoiceID)
		stored, err := repo.GetByID(ctx, original.ID)
		require.NoError(t, err)
		assert.Equal(t, "electric-guitar", stored.DefaultVoiceID)
	})

	rejected := []struct {
		name           string
		names          map[string]string
		defaultVoiceID string
		wantField      string
	}{
		{name: "names missing a language", names: map[string]string{"en": "Guitar!"}, defaultVoiceID: "acoustic-guitar", wantField: "names"},
		{name: "a default voice of another family", names: bilingualGuitar, defaultVoiceID: "piano", wantField: "default_voice_id"},
		{name: "a default voice that does not exist", names: bilingualGuitar, defaultVoiceID: "banjo", wantField: "default_voice_id"},
	}
	for _, tt := range rejected {
		t.Run(tt.name+" is rejected and the stored instrument is unchanged", func(t *testing.T) {
			repo := newFakeInstrumentRepository()
			original := seed(t, repo)
			svc := newInstrumentService(repo)

			_, err := svc.UpdateInstrument(ctx, adminCaller(), original.ID, tt.names, &tt.defaultVoiceID)

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, tt.wantField, valErr.Fields[0].Field)
			stored, getErr := repo.GetByID(ctx, original.ID)
			require.NoError(t, getErr)
			assert.Equal(t, original, stored)
		})
	}

	t.Run("teachers and students cannot update an instrument", func(t *testing.T) {
		for _, caller := range []domain.User{teacherCaller(), studentCaller()} {
			repo := newFakeInstrumentRepository()
			original := seed(t, repo)
			svc := newInstrumentService(repo)

			_, err := svc.UpdateInstrument(ctx, caller, original.ID, bilingualGuitar, nil)

			require.ErrorIs(t, err, domain.ErrForbidden, "role %s", caller.Role)
		}
	})

	t.Run("an unknown instrument is not found", func(t *testing.T) {
		svc := newInstrumentService(newFakeInstrumentRepository())

		_, err := svc.UpdateInstrument(ctx, adminCaller(), "nope", bilingualGuitar, nil)

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

func TestVoiceService_ListVoices(t *testing.T) {
	svc := application.NewVoiceService(newFakeVoiceRepository(), "https://media.example.com")

	got, err := svc.ListVoices(context.Background())

	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, []string{"acoustic-guitar", "electric-guitar", "piano"}, []string{got[0].Voice.ID, got[1].Voice.ID, got[2].Voice.ID}, "voices are ordered by id")
	assert.Equal(t, []application.VoiceSample{
		{Pitch: 40, URL: "https://media.example.com/audio/voices/acoustic-guitar/40.mp3"},
		{Pitch: 43, URL: "https://media.example.com/audio/voices/acoustic-guitar/43.mp3"},
	}, got[0].Samples, "each sample's address is derived from the voice id and pitch, lowest pitch first")
}
