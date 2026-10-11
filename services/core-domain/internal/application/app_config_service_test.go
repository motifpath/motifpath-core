package application_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

const (
	spGuitar      = "11111111-1111-4111-8111-111111111111"
	spBass        = "22222222-2222-4222-8222-222222222222"
	spFromZero    = "33333333-3333-4333-8333-333333333333" // published beginner path, en, guitar
	spFirstChords = "44444444-4444-4444-8444-444444444444" // published beginner path, en, guitar
	spBasics      = "55555555-5555-4555-8555-555555555555" // published beginner course, en, guitar
	spDraftPath   = "66666666-6666-4666-8666-666666666666" // draft path
	spRetired     = "77777777-7777-4777-8777-777777777777" // retired course
	spSolos       = "88888888-8888-4888-8888-888888888888" // published intermediate course, en, guitar
	spUnknown     = "99999999-9999-4999-8999-999999999999" // names nothing
)

var spNow = time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)

type startingPointsFixture struct {
	instruments *fakeInstrumentRepository
	languages   *fakeLanguageRepository
	paths       *fakeLearningPathRepository
	courses     *fakeCourseRepository
	config      *fakeAppConfigRepository
}

func newStartingPointsFixture() startingPointsFixture {
	f := startingPointsFixture{
		instruments: newFakeInstrumentRepository(),
		languages:   newFakeLanguageRepository(),
		paths:       newFakeLearningPathRepository(),
		courses:     newFakeCourseRepository(),
		config:      newFakeAppConfigRepository(),
	}
	f.instruments.put(domain.Instrument{ID: spGuitar})
	f.instruments.put(domain.Instrument{ID: spBass})
	beginner := domain.DifficultyLevelBeginner
	f.paths.put(domain.LearningPath{ID: spFromZero, Title: "Electric guitar from zero", Status: domain.LearningPathStatusPublished, Level: &beginner, Language: strPtr("en"), InstrumentIDs: []string{spGuitar}})
	f.paths.put(domain.LearningPath{ID: spFirstChords, Title: "Electric guitar first chords", Status: domain.LearningPathStatusPublished, Level: &beginner, Language: strPtr("en"), InstrumentIDs: []string{spGuitar}})
	f.paths.put(domain.LearningPath{ID: spDraftPath, Title: "Draft", Status: domain.LearningPathStatusDraft})
	f.courses.put(domain.Course{ID: spBasics, Title: "Electric guitar basics", Status: domain.CourseStatusPublished, Level: domain.DifficultyLevelBeginner, Language: "en", InstrumentIDs: []string{spGuitar}})
	f.courses.put(domain.Course{ID: spRetired, Title: "Retired", Status: domain.CourseStatusRetired, Level: domain.DifficultyLevelBeginner, Language: "en", InstrumentIDs: []string{spGuitar}})
	f.courses.put(domain.Course{ID: spSolos, Title: "Electric guitar solos", Status: domain.CourseStatusPublished, Level: domain.DifficultyLevelIntermediate, Language: "en", InstrumentIDs: []string{spGuitar}})
	return f
}

func (f startingPointsFixture) configService() *application.AppConfigService {
	return application.NewAppConfigService(f.config, func() time.Time { return spNow },
		application.NewStartingPointsKey(f.instruments, f.languages, f.paths, f.courses))
}

// spList renders one starting_points list as JSON; items are "kind:id".
func spList(instrument, experience, language string, items ...string) string {
	rendered := make([]string, len(items))
	for i, item := range items {
		kind, id := item[:len(item)-37], item[len(item)-36:]
		rendered[i] = fmt.Sprintf(`{"kind":%q,"id":%q}`, kind, id)
	}
	return fmt.Sprintf(`{"instrument_id":%q,"experience":%q,"language":%q,"items":[%s]}`,
		instrument, experience, language, strings.Join(rendered, ","))
}

func spValue(lists ...string) []byte {
	return []byte(`{"lists":[` + strings.Join(lists, ",") + `]}`)
}

func fieldNames(t *testing.T, err error) []string {
	t.Helper()
	var verr *domain.ValidationError
	require.ErrorAs(t, err, &verr)
	names := make([]string, len(verr.Fields))
	for i, f := range verr.Fields {
		names[i] = f.Field
	}
	return names
}

func TestAppConfigService_Set(t *testing.T) {
	cases := []struct {
		name       string
		caller     domain.User
		key        string
		value      []byte
		wantErr    error
		wantFields []string
	}{
		{name: "an admin sets a valid list", value: spValue(spList(spGuitar, "never_played", "en", "course:"+spBasics, "path:"+spFromZero))},
		{name: "an entry may be at another level than the experience", value: spValue(spList(spGuitar, "never_played", "en", "course:"+spSolos))},
		{name: "lists for different combinations sit side by side", value: spValue(
			spList(spGuitar, "never_played", "en", "path:"+spFromZero),
			spList(spGuitar, "plays_well", "en", "course:"+spSolos),
			spList(spGuitar, "never_played", "pt_BR", "path:"+spFromZero))},
		{name: "a value that is not JSON is refused", value: []byte(`{"lists":`), wantFields: []string{"value"}},
		{name: "a value without lists is refused", value: []byte(`{}`), wantFields: []string{"lists"}},
		{name: "a list without instrument_id is refused", value: []byte(`{"lists":[{"experience":"never_played","language":"en","items":[{"kind":"path","id":"` + spFromZero + `"}]}]}`), wantFields: []string{"lists[0].instrument_id"}},
		{name: "a list without experience is refused", value: []byte(`{"lists":[{"instrument_id":"` + spGuitar + `","language":"en","items":[{"kind":"path","id":"` + spFromZero + `"}]}]}`), wantFields: []string{"lists[0].experience"}},
		{name: "a list without language is refused", value: []byte(`{"lists":[{"instrument_id":"` + spGuitar + `","experience":"never_played","items":[{"kind":"path","id":"` + spFromZero + `"}]}]}`), wantFields: []string{"lists[0].language"}},
		{name: "a list without items is refused", value: []byte(`{"lists":[{"instrument_id":"` + spGuitar + `","experience":"never_played","language":"en"}]}`), wantFields: []string{"lists[0].items"}},
		{name: "a list with no entries is refused", value: spValue(spList(spGuitar, "never_played", "en")), wantFields: []string{"lists[0].items"}},
		{name: "an unknown experience is refused", value: spValue(spList(spGuitar, "expert_player", "en", "path:"+spFromZero)), wantFields: []string{"lists[0].experience"}},
		{name: "an entry of an unknown kind is refused", value: spValue(spList(spGuitar, "never_played", "en", "song:"+spFromZero)), wantFields: []string{"lists[0].items[0].kind"}},
		{name: "an instrument_id that is not a UUID is refused", value: []byte(`{"lists":[{"instrument_id":"guitar","experience":"never_played","language":"en","items":[{"kind":"path","id":"` + spFromZero + `"}]}]}`), wantFields: []string{"lists[0].instrument_id"}},
		{name: "a repeated entry within a list is refused", value: spValue(spList(spGuitar, "never_played", "en", "path:"+spFromZero, "path:"+spFromZero)), wantFields: []string{"lists[0].items[1]"}},
		{name: "two lists for the same combination are refused", value: spValue(
			spList(spGuitar, "never_played", "en", "path:"+spFromZero),
			spList(spGuitar, "never_played", "en", "course:"+spBasics)), wantFields: []string{"lists[1]"}},
		{name: "an instrument that does not exist is refused", value: spValue(spList(spUnknown, "never_played", "en", "path:"+spFromZero)), wantFields: []string{"lists[0].instrument_id"}},
		{name: "an unknown language is refused", value: spValue(spList(spGuitar, "never_played", "xx", "path:"+spFromZero)), wantFields: []string{"lists[0].language"}},
		{name: "the language-agnostic marker is refused", value: spValue(spList(spGuitar, "never_played", domain.LanguageCodeAny, "path:"+spFromZero)), wantFields: []string{"lists[0].language"}},
		{name: "a draft path is refused", value: spValue(spList(spGuitar, "never_played", "en", "path:"+spDraftPath)), wantFields: []string{"lists[0].items[0].id"}},
		{name: "a retired course is refused", value: spValue(spList(spGuitar, "never_played", "en", "course:"+spRetired)), wantFields: []string{"lists[0].items[0].id"}},
		{name: "a course carrying a path's id is refused", value: spValue(spList(spGuitar, "never_played", "en", "course:"+spFromZero)), wantFields: []string{"lists[0].items[0].id"}},
		{name: "an id that names nothing is refused", value: spValue(spList(spGuitar, "never_played", "en", "path:"+spUnknown)), wantFields: []string{"lists[0].items[0].id"}},
		{name: "every problem is reported at once", value: []byte(`{"lists":[{"instrument_id":"` + spGuitar + `","experience":"never_played","items":[{"kind":"path","id":"` + spFromZero + `"}]},` + spList(spGuitar, "plays_well", "en") + `]}`), wantFields: []string{"lists[0].language", "lists[1].items"}},
		{name: "a key the service does not know is not found", key: "featured_song", value: []byte(`{}`), wantErr: domain.ErrNotFound},
		{name: "a teacher cannot change settings", caller: teacherCaller(), value: spValue(spList(spGuitar, "never_played", "en", "path:"+spFromZero)), wantErr: domain.ErrForbidden},
		{name: "a student cannot change settings", caller: studentCaller(), value: spValue(spList(spGuitar, "never_played", "en", "path:"+spFromZero)), wantErr: domain.ErrForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newStartingPointsFixture()
			svc := f.configService()
			caller, key := tc.caller, tc.key
			if caller.ID == "" {
				caller = adminCaller()
			}
			if key == "" {
				key = application.StartingPointsConfigKey
			}

			got, err := svc.Set(context.Background(), caller, key, tc.value)

			stored, getErr := f.config.Get(context.Background(), key)
			switch {
			case tc.wantErr != nil:
				require.ErrorIs(t, err, tc.wantErr)
				assert.ErrorIs(t, getErr, domain.ErrNotFound, "nothing is stored")
			case tc.wantFields != nil:
				assert.Equal(t, tc.wantFields, fieldNames(t, err))
				assert.ErrorIs(t, getErr, domain.ErrNotFound, "nothing is stored")
			default:
				require.NoError(t, err)
				require.NoError(t, getErr)
				assert.JSONEq(t, string(tc.value), string(stored.Value))
				assert.Equal(t, adminCaller().ID, stored.UpdatedBy)
				assert.Equal(t, spNow, stored.UpdatedAt)
				require.NotNil(t, got.Entry)
				assert.Equal(t, stored, *got.Entry)
			}
		})
	}
}

func TestAppConfigService_SetReplacesTheWholeValue(t *testing.T) {
	f := newStartingPointsFixture()
	svc := f.configService()
	ctx := context.Background()
	_, err := svc.Set(ctx, adminCaller(), application.StartingPointsConfigKey, spValue(
		spList(spGuitar, "never_played", "en", "path:"+spFromZero),
		spList(spGuitar, "plays_well", "en", "course:"+spSolos)))
	require.NoError(t, err)

	only := spValue(spList(spGuitar, "never_played", "en", "path:"+spFirstChords))
	_, err = svc.Set(ctx, adminCaller(), application.StartingPointsConfigKey, only)

	require.NoError(t, err)
	stored, err := f.config.Get(ctx, application.StartingPointsConfigKey)
	require.NoError(t, err)
	assert.JSONEq(t, string(only), string(stored.Value))
}

func TestAppConfigService_Get(t *testing.T) {
	cases := []struct {
		name      string
		caller    domain.User
		key       string
		set       bool
		wantErr   error
		wantEntry bool
	}{
		{name: "an admin reads a setting that was set", set: true, wantEntry: true},
		{name: "a known setting never set has no value", wantEntry: false},
		{name: "a key the service does not know is not found", key: "featured_song", wantErr: domain.ErrNotFound},
		{name: "a teacher cannot read settings", caller: teacherCaller(), wantErr: domain.ErrForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newStartingPointsFixture()
			svc := f.configService()
			ctx := context.Background()
			caller, key := tc.caller, tc.key
			if caller.ID == "" {
				caller = adminCaller()
			}
			if key == "" {
				key = application.StartingPointsConfigKey
			}
			value := spValue(spList(spGuitar, "never_played", "en", "path:"+spFromZero))
			if tc.set {
				_, err := svc.Set(ctx, adminCaller(), application.StartingPointsConfigKey, value)
				require.NoError(t, err)
			}

			got, err := svc.Get(ctx, caller, key)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, key, got.Key)
			if !tc.wantEntry {
				assert.Nil(t, got.Entry)
				return
			}
			require.NotNil(t, got.Entry)
			assert.JSONEq(t, string(value), string(got.Entry.Value))
		})
	}
}

func TestAppConfigService_List(t *testing.T) {
	t.Run("an admin lists every known key, set or not", func(t *testing.T) {
		f := newStartingPointsFixture()
		svc := f.configService()

		got, err := svc.List(context.Background(), adminCaller())

		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, application.StartingPointsConfigKey, got[0].Key)
		assert.Nil(t, got[0].Entry)
	})
	t.Run("a set key carries who and when", func(t *testing.T) {
		f := newStartingPointsFixture()
		svc := f.configService()
		ctx := context.Background()
		_, err := svc.Set(ctx, adminCaller(), application.StartingPointsConfigKey, spValue(spList(spGuitar, "never_played", "en", "path:"+spFromZero)))
		require.NoError(t, err)

		got, err := svc.List(ctx, adminCaller())

		require.NoError(t, err)
		require.Len(t, got, 1)
		require.NotNil(t, got[0].Entry)
		assert.Equal(t, adminCaller().ID, got[0].Entry.UpdatedBy)
		assert.Equal(t, spNow, got[0].Entry.UpdatedAt)
	})
	t.Run("a student cannot list settings", func(t *testing.T) {
		f := newStartingPointsFixture()

		_, err := f.configService().List(context.Background(), studentCaller())

		require.ErrorIs(t, err, domain.ErrForbidden)
	})
}
