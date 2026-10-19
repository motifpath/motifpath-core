package application_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

func (f startingPointsFixture) startingPoints() *application.StartingPointsService {
	return application.NewStartingPointsService(f.config, f.languages, f.paths, f.courses)
}

// setStartingPoints stores a starting_points value through the admin service, so the read
// always starts from a value the validator accepted.
func (f startingPointsFixture) setStartingPoints(t *testing.T, lists ...string) {
	t.Helper()
	_, err := f.configService().Set(context.Background(), adminCaller(), application.StartingPointsConfigKey, spValue(lists...))
	require.NoError(t, err)
}

// ids renders a result as "kind:id" in order.
func ids(result application.StartingPoints) []string {
	out := make([]string, len(result.Items))
	for i, item := range result.Items {
		switch item.Kind {
		case domain.StartingPointKindPath:
			out[i] = "path:" + item.Path.ID
		case domain.StartingPointKindCourse:
			out[i] = "course:" + item.Course.ID
		}
	}
	return out
}

func TestStartingPointsService_List(t *testing.T) {
	cases := []struct {
		name       string
		setup      func(t *testing.T, f startingPointsFixture)
		instrument string
		experience string
		language   string
		want       []string
		wantSource application.StartingPointsSource
	}{
		{
			name: "a curated list answers in its order",
			setup: func(t *testing.T, f startingPointsFixture) {
				f.setStartingPoints(t, spList(spGuitar, "never_played", "en", "course:"+spBasics, "path:"+spFromZero))
			},
			want:       []string{"course:" + spBasics, "path:" + spFromZero},
			wantSource: application.StartingPointsSourceCurated,
		},
		{
			name: "a curated list shows every entry the team chose",
			setup: func(t *testing.T, f startingPointsFixture) {
				items := make([]string, 7)
				for i := range items {
					id := fmt.Sprintf("aaaaaaaa-aaaa-4aaa-8aaa-%012d", i)
					f.paths.put(domain.LearningPath{ID: id, Title: fmt.Sprintf("P%d", i), Status: domain.LearningPathStatusPublished, Language: strPtr("en")})
					items[i] = "path:" + id
				}
				f.setStartingPoints(t, spList(spGuitar, "never_played", "en", items...))
			},
			want: []string{
				"path:aaaaaaaa-aaaa-4aaa-8aaa-000000000000", "path:aaaaaaaa-aaaa-4aaa-8aaa-000000000001",
				"path:aaaaaaaa-aaaa-4aaa-8aaa-000000000002", "path:aaaaaaaa-aaaa-4aaa-8aaa-000000000003",
				"path:aaaaaaaa-aaaa-4aaa-8aaa-000000000004", "path:aaaaaaaa-aaaa-4aaa-8aaa-000000000005",
				"path:aaaaaaaa-aaaa-4aaa-8aaa-000000000006",
			},
			wantSource: application.StartingPointsSourceCurated,
		},
		{
			name: "a curated entry no longer published is skipped",
			setup: func(t *testing.T, f startingPointsFixture) {
				f.setStartingPoints(t, spList(spGuitar, "never_played", "en", "path:"+spFromZero, "course:"+spBasics))
				f.courses.put(domain.Course{ID: spBasics, Title: "Electric guitar basics", Status: domain.CourseStatusRetired, Level: domain.DifficultyLevelBeginner, Language: "en"})
			},
			want:       []string{"path:" + spFromZero},
			wantSource: application.StartingPointsSourceCurated,
		},
		{
			name: "a curated list with nothing still published falls back to the catalog",
			setup: func(t *testing.T, f startingPointsFixture) {
				f.setStartingPoints(t, spList(spGuitar, "never_played", "en", "course:"+spBasics))
				f.courses.put(domain.Course{ID: spBasics, Title: "Electric guitar basics", Status: domain.CourseStatusRetired, Level: domain.DifficultyLevelBeginner, Language: "en"})
			},
			want:       []string{"path:" + spFirstChords, "path:" + spFromZero},
			wantSource: application.StartingPointsSourceCatalog,
		},
		{
			name: "a list is only for its own language",
			setup: func(t *testing.T, f startingPointsFixture) {
				f.setStartingPoints(t, spList(spGuitar, "never_played", "en", "course:"+spBasics))
			},
			language:   "pt_BR",
			want:       []string{},
			wantSource: application.StartingPointsSourceCatalog,
		},
		{
			name: "a list is only for its own instrument",
			setup: func(t *testing.T, f startingPointsFixture) {
				f.setStartingPoints(t, spList(spBass, "never_played", "en", "course:"+spBasics))
			},
			want:       []string{"path:" + spFirstChords, "path:" + spFromZero, "course:" + spBasics},
			wantSource: application.StartingPointsSourceCatalog,
		},
		{
			name:       "without a list the catalog answers, paths first, by title, at the experience's levels",
			want:       []string{"path:" + spFirstChords, "path:" + spFromZero, "course:" + spBasics},
			wantSource: application.StartingPointsSourceCatalog,
		},
		{
			name: "plays a bit maps to early intermediate and intermediate",
			setup: func(t *testing.T, f startingPointsFixture) {
				early := domain.DifficultyLevelEarlyIntermediate
				f.paths.put(domain.LearningPath{ID: spUnknown, Title: "Riffs", Status: domain.LearningPathStatusPublished, Level: &early, Language: strPtr("en"), InstrumentIDs: []string{spGuitar}})
			},
			experience: "plays_a_bit",
			want:       []string{"path:" + spUnknown, "course:" + spSolos},
			wantSource: application.StartingPointsSourceCatalog,
		},
		{
			name: "the catalog answer is for the instrument asked, or for every instrument",
			setup: func(t *testing.T, f startingPointsFixture) {
				beginner := domain.DifficultyLevelBeginner
				f.paths.put(domain.LearningPath{ID: spUnknown, Title: "Any instrument", Status: domain.LearningPathStatusPublished, Level: &beginner, Language: strPtr("en")})
			},
			instrument: spBass,
			want:       []string{"path:" + spUnknown},
			wantSource: application.StartingPointsSourceCatalog,
		},
		{
			name: "the catalog answer shows at most five",
			setup: func(t *testing.T, f startingPointsFixture) {
				beginner := domain.DifficultyLevelBeginner
				for i := range 4 {
					id := fmt.Sprintf("bbbbbbbb-bbbb-4bbb-8bbb-%012d", i)
					f.paths.put(domain.LearningPath{ID: id, Title: fmt.Sprintf("A%d", i), Status: domain.LearningPathStatusPublished, Level: &beginner, Language: strPtr("en"), InstrumentIDs: []string{spGuitar}})
				}
			},
			want: []string{
				"path:bbbbbbbb-bbbb-4bbb-8bbb-000000000000", "path:bbbbbbbb-bbbb-4bbb-8bbb-000000000001",
				"path:bbbbbbbb-bbbb-4bbb-8bbb-000000000002", "path:bbbbbbbb-bbbb-4bbb-8bbb-000000000003",
				"path:" + spFirstChords,
			},
			wantSource: application.StartingPointsSourceCatalog,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newStartingPointsFixture()
			if tc.setup != nil {
				tc.setup(t, f)
			}
			instrument, experience, language := tc.instrument, tc.experience, tc.language
			if instrument == "" {
				instrument = spGuitar
			}
			if experience == "" {
				experience = "never_played"
			}
			if language == "" {
				language = "en"
			}

			got, err := f.startingPoints().List(context.Background(), instrument, domain.StartingPointExperience(experience), language)

			require.NoError(t, err)
			assert.Equal(t, tc.want, ids(got))
			assert.Equal(t, tc.wantSource, got.Source)
		})
	}
}

func TestStartingPointsService_ListRejectsBadQuestions(t *testing.T) {
	cases := []struct {
		name       string
		experience string
		language   string
		wantField  string
	}{
		{name: "an unknown experience", experience: "expert_player", language: "en", wantField: "experience"},
		{name: "an unknown language", experience: "never_played", language: "xx", wantField: "language"},
		{name: "the language-agnostic marker", experience: "never_played", language: domain.LanguageCodeAny, wantField: "language"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newStartingPointsFixture()

			_, err := f.startingPoints().List(context.Background(), spGuitar, domain.StartingPointExperience(tc.experience), tc.language)

			assert.Equal(t, []string{tc.wantField}, fieldNames(t, err))
		})
	}
}
