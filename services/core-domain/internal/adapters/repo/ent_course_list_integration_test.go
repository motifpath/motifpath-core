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

type courseListFixture struct {
	t        *testing.T
	ctx      context.Context
	nodes    *EntContentNodeRepository
	paths    *EntLearningPathRepository
	courses  *EntCourseRepository
	versions *EntCourseVersionRepository
}

func newCourseListFixture(t *testing.T) courseListFixture {
	client := setupPostgres(t)
	return courseListFixture{
		t:        t,
		ctx:      context.Background(),
		nodes:    NewEntContentNodeRepository(client),
		paths:    NewEntLearningPathRepository(client),
		courses:  NewEntCourseRepository(client),
		versions: NewEntCourseVersionRepository(client),
	}
}

// pathWith seeds a learning path holding one content node classified with
// the given skills and concepts.
func (f courseListFixture) pathWith(skills []domain.Skill, concepts []domain.Concept) domain.LearningPath {
	f.t.Helper()
	node := domain.ContentNode{
		ID: uuid.NewString(), TeacherID: uuid.NewString(), Title: "Node " + uuid.NewString(), ContentType: domain.ContentTypeVideo,
		Classification: domain.Classification{Skills: skills, Concepts: concepts, DifficultyLevel: domain.DifficultyLevelBeginner, ReviewState: domain.ReviewStatePending},
		CreatedAt:      fixedAt,
	}
	require.NoError(f.t, f.nodes.Create(f.ctx, node))
	return seedLearningPath(f.t, f.ctx, f.paths, node)
}

func checkpointsOf(paths ...domain.LearningPath) []domain.CourseCheckpoint {
	out := make([]domain.CourseCheckpoint, len(paths))
	for i, p := range paths {
		out[i] = domain.CourseCheckpoint{Position: i + 1, LearningPathID: p.ID, EffectiveTitle: p.Title}
	}
	return out
}

func versionCheckpointsOf(paths ...domain.LearningPath) []domain.CourseVersionCheckpoint {
	out := make([]domain.CourseVersionCheckpoint, len(paths))
	for i, p := range paths {
		out[i] = domain.CourseVersionCheckpoint{Position: i + 1, LearningPathID: p.ID, EffectiveTitle: p.Title}
	}
	return out
}

// draft seeds a draft course with the given live checkpoints.
func (f courseListFixture) draft(title, summary string, level domain.DifficultyLevel, createdBy string, live ...domain.LearningPath) domain.Course {
	f.t.Helper()
	course := domain.Course{
		ID: uuid.NewString(), CreatedBy: createdBy, Title: title, Summary: summary, Level: level,
		Status: domain.CourseStatusDraft, CreatedAt: fixedAt, Checkpoints: checkpointsOf(live...),
	}
	require.NoError(f.t, f.courses.Create(f.ctx, course))
	return course
}

// publish snapshots version 1 of course with the given snapshot fields and
// checkpoints and marks the course published; the live row is left as is.
func (f courseListFixture) publish(course domain.Course, title string, level domain.DifficultyLevel, snapshot ...domain.LearningPath) {
	f.t.Helper()
	require.NoError(f.t, f.versions.Create(f.ctx, domain.CourseVersion{
		ID: uuid.NewString(), CourseID: course.ID, VersionNumber: 1,
		TitleSnapshot: title, SummarySnapshot: course.Summary, LevelSnapshot: level,
		Checkpoints: versionCheckpointsOf(snapshot...), PublishedAt: fixedAt, AvailableForNewEnrollments: true,
	}))
	require.NoError(f.t, f.courses.UpdateStatus(f.ctx, course.ID, domain.CourseStatusPublished))
}

func (f courseListFixture) list(filter domain.CourseListFilter, page domain.PageRequest) domain.Page[domain.Course] {
	f.t.Helper()
	got, err := f.courses.List(f.ctx, filter, page)
	require.NoError(f.t, err)
	return got
}

func courseIDs(page domain.Page[domain.Course]) []string {
	ids := make([]string, len(page.Items))
	for i, c := range page.Items {
		ids[i] = c.ID
	}
	return ids
}

var firstCoursePage = domain.PageRequest{Limit: 20}

func TestEntCourseRepository_List_PagesOrdersAndFiltersLiveDrafts(t *testing.T) {
	f := newCourseListFixture(t)
	path := f.pathWith(nil, nil)
	bob, carol := uuid.NewString(), uuid.NewString()

	fingerstyle := f.draft("Fingerstyle Journey", "From first chords.", domain.DifficultyLevelBeginner, bob, path)
	mastery := f.draft("Fingerstyle Mastery", "Repertoire.", domain.DifficultyLevelExpert, bob, path)
	strumming := f.draft("Strumming Basics", "Rhythm and 100% feel.", domain.DifficultyLevelBeginner, carol, path)

	t.Run("orders by title and pages with the total of every match", func(t *testing.T) {
		got := f.list(domain.CourseListFilter{}, domain.PageRequest{Limit: 2, Offset: 1})

		assert.Equal(t, 3, got.Total)
		assert.Equal(t, []string{mastery.ID, strumming.ID}, courseIDs(got))
		require.Len(t, got.Items[0].Checkpoints, 1)
	})

	t.Run("an offset past the end returns an empty non-nil page with the total", func(t *testing.T) {
		got := f.list(domain.CourseListFilter{}, domain.PageRequest{Limit: 20, Offset: 50})

		assert.NotNil(t, got.Items)
		assert.Empty(t, got.Items)
		assert.Equal(t, 3, got.Total)
	})

	t.Run("search matches title or summary case-insensitively, and treats percent literally", func(t *testing.T) {
		byTitle := f.list(domain.CourseListFilter{Query: "FINGERSTYLE"}, firstCoursePage)
		bySummary := f.list(domain.CourseListFilter{Query: "repertoire"}, firstCoursePage)
		literalPercent := f.list(domain.CourseListFilter{Query: "100%"}, firstCoursePage)
		wildcardOnly := f.list(domain.CourseListFilter{Query: "%"}, firstCoursePage)

		assert.Equal(t, []string{fingerstyle.ID, mastery.ID}, courseIDs(byTitle))
		assert.Equal(t, []string{mastery.ID}, courseIDs(bySummary))
		assert.Equal(t, []string{strumming.ID}, courseIDs(literalPercent))
		assert.Equal(t, []string{strumming.ID}, courseIDs(wildcardOnly))
	})

	t.Run("levels are any-of", func(t *testing.T) {
		got := f.list(domain.CourseListFilter{Levels: []domain.DifficultyLevel{domain.DifficultyLevelBeginner, domain.DifficultyLevelAdvanced}}, firstCoursePage)

		assert.Equal(t, []string{fingerstyle.ID, strumming.ID}, courseIDs(got))
	})

	t.Run("created_by, status, level and text combine, and the total reflects them", func(t *testing.T) {
		draft := domain.CourseStatusDraft
		got := f.list(domain.CourseListFilter{
			Status: &draft, CreatedBy: bob, Query: "fingerstyle",
			Levels: []domain.DifficultyLevel{domain.DifficultyLevelBeginner},
		}, domain.PageRequest{Limit: 1})

		assert.Equal(t, 1, got.Total)
		assert.Equal(t, []string{fingerstyle.ID}, courseIDs(got))
	})
}

func TestEntCourseRepository_List_PublishedViewReadsTheLatestVersion(t *testing.T) {
	f := newCourseListFixture(t)
	path := f.pathWith(nil, nil)

	// The live draft was edited after publishing: new title and level.
	edited := f.draft("Edited Title", "Summary.", domain.DifficultyLevelExpert, uuid.NewString(), path)
	f.publish(edited, "Published Title", domain.DifficultyLevelBeginner, path)

	other := f.draft("Zebra Course", "Summary.", domain.DifficultyLevelBeginner, uuid.NewString(), path)
	f.publish(other, "Alpha Course", domain.DifficultyLevelBeginner, path)

	published := domain.CourseStatusPublished
	student := func(filter domain.CourseListFilter) domain.Page[domain.Course] {
		filter.Status = &published
		filter.PublishedView = true
		return f.list(filter, firstCoursePage)
	}

	t.Run("text search matches the published title, not the live draft's", func(t *testing.T) {
		assert.Equal(t, []string{edited.ID}, courseIDs(student(domain.CourseListFilter{Query: "published title"})))
		assert.Empty(t, student(domain.CourseListFilter{Query: "edited title"}).Items)
	})

	t.Run("levels match the published level, not the live draft's", func(t *testing.T) {
		got := student(domain.CourseListFilter{Levels: []domain.DifficultyLevel{domain.DifficultyLevelExpert}})

		assert.Empty(t, got.Items)
		assert.Zero(t, got.Total)
	})

	t.Run("results are ordered by the published title", func(t *testing.T) {
		got := student(domain.CourseListFilter{})

		assert.Equal(t, []string{other.ID, edited.ID}, courseIDs(got))
	})

	t.Run("the live view still reads the live draft", func(t *testing.T) {
		got := f.list(domain.CourseListFilter{Query: "edited title"}, firstCoursePage)

		assert.Equal(t, []string{edited.ID}, courseIDs(got))
	})
}

func TestEntCourseRepository_List_FiltersByClassification(t *testing.T) {
	f := newCourseListFixture(t)
	client := f.courses.client
	fingerpicking := seedSkill(t, f.ctx, client, "fingerpicking-"+uuid.NewString())
	altPicking := seedSkill(t, f.ctx, client, "alt-picking-"+uuid.NewString())
	syncopation := seedConcept(t, f.ctx, client, "syncopation-"+uuid.NewString())
	swing := seedConcept(t, f.ctx, client, "swing-"+uuid.NewString())
	creator := uuid.NewString()

	fingerpickingPath := f.pathWith([]domain.Skill{fingerpicking}, []domain.Concept{syncopation})
	altPickingPath := f.pathWith([]domain.Skill{altPicking}, []domain.Concept{swing})
	skillOnlyPath := f.pathWith([]domain.Skill{fingerpicking}, nil)
	unclassifiedPath := f.pathWith(nil, nil)

	seedPublished := func(title string, paths ...domain.LearningPath) domain.Course {
		course := f.draft(title, "s", domain.DifficultyLevelBeginner, creator, paths...)
		f.publish(course, title, course.Level, paths...)
		return course
	}
	fingerstyle := seedPublished("Fingerstyle", fingerpickingPath)
	picking := seedPublished("Picking", altPickingPath)
	skillOnly := seedPublished("Skill Only", skillOnlyPath)
	multiStage := seedPublished("Multi Stage", unclassifiedPath, altPickingPath)
	theory := seedPublished("Theory", unclassifiedPath)

	published := domain.CourseStatusPublished
	student := func(filter domain.CourseListFilter) []string {
		filter.Status = &published
		filter.PublishedView = true
		return courseIDs(f.list(filter, firstCoursePage))
	}

	t.Run("a skill matches any course with a classified node in any checkpoint", func(t *testing.T) {
		got := student(domain.CourseListFilter{SkillIDs: []string{altPicking.ID}})

		assert.Equal(t, []string{multiStage.ID, picking.ID}, got)
	})

	t.Run("several skills are any-of", func(t *testing.T) {
		got := student(domain.CourseListFilter{SkillIDs: []string{fingerpicking.ID, altPicking.ID}})

		assert.ElementsMatch(t, []string{fingerstyle.ID, picking.ID, skillOnly.ID, multiStage.ID}, got)
		assert.NotContains(t, got, theory.ID)
	})

	t.Run("several concepts are any-of", func(t *testing.T) {
		got := student(domain.CourseListFilter{ConceptIDs: []string{syncopation.ID, swing.ID}})

		assert.ElementsMatch(t, []string{fingerstyle.ID, picking.ID, multiStage.ID}, got)
	})

	t.Run("skills and concepts must both match on the same content node", func(t *testing.T) {
		got := student(domain.CourseListFilter{SkillIDs: []string{fingerpicking.ID}, ConceptIDs: []string{syncopation.ID}})

		// skillOnly has the skill but no concept; picking has neither pairing.
		assert.Equal(t, []string{fingerstyle.ID}, got)
	})

	t.Run("a skill with no matching course returns nothing", func(t *testing.T) {
		unused := seedSkill(t, f.ctx, client, "unused-"+uuid.NewString())

		got := f.list(domain.CourseListFilter{SkillIDs: []string{unused.ID}}, firstCoursePage)

		assert.Empty(t, got.Items)
		assert.Zero(t, got.Total)
	})

	t.Run("classification combines with paging and reports the filtered total", func(t *testing.T) {
		got := f.list(domain.CourseListFilter{SkillIDs: []string{fingerpicking.ID, altPicking.ID}, PublishedView: true},
			domain.PageRequest{Limit: 2, Offset: 0})

		assert.Equal(t, 4, got.Total)
		assert.Len(t, got.Items, 2)
	})

	t.Run("the published view reads the latest version's checkpoints and the live view the draft's", func(t *testing.T) {
		// Published against the fingerpicking path, then the live draft is
		// replaced with the alt-picking path (unpublished edit).
		drifted := seedPublished("Drifted", fingerpickingPath)
		drifted.Checkpoints = checkpointsOf(altPickingPath)
		require.NoError(t, f.courses.Replace(f.ctx, drifted))
		altOnly := []string{altPicking.ID}
		fingerOnly := []string{fingerpicking.ID}

		assert.NotContains(t, student(domain.CourseListFilter{SkillIDs: altOnly}), drifted.ID)
		assert.Contains(t, student(domain.CourseListFilter{SkillIDs: fingerOnly}), drifted.ID)
		assert.Contains(t, courseIDs(f.list(domain.CourseListFilter{SkillIDs: altOnly}, firstCoursePage)), drifted.ID)
		assert.NotContains(t, courseIDs(f.list(domain.CourseListFilter{SkillIDs: fingerOnly, CreatedBy: creator, Query: "drifted"}, firstCoursePage)), drifted.ID)
	})
}


func TestEntCourseRepository_ListCreatorIDs(t *testing.T) {
	f := newCourseListFixture(t)
	path := f.pathWith(nil, nil)
	bob, carol, dave := uuid.NewString(), uuid.NewString(), uuid.NewString()

	f.publish(f.draft("Fingerstyle Journey", "", domain.DifficultyLevelBeginner, bob, path), "Fingerstyle Journey", domain.DifficultyLevelBeginner, path)
	f.publish(f.draft("Fingerstyle Mastery", "", domain.DifficultyLevelExpert, bob, path), "Fingerstyle Mastery", domain.DifficultyLevelExpert, path)
	f.draft("Strumming Basics", "", domain.DifficultyLevelBeginner, carol, path)
	retired := f.draft("Theory Primer", "", domain.DifficultyLevelBeginner, dave, path)
	f.publish(retired, "Theory Primer", domain.DifficultyLevelBeginner, path)
	require.NoError(t, f.courses.UpdateStatus(f.ctx, retired.ID, domain.CourseStatusRetired))

	published := domain.CourseStatusPublished
	tests := []struct {
		name   string
		filter domain.CourseListFilter
		want   []string
	}{
		{name: "every creator once, whatever the status", filter: domain.CourseListFilter{}, want: []string{bob, carol, dave}},
		{name: "only creators of a course in the given status", filter: domain.CourseListFilter{Status: &published}, want: []string{bob}},
		{name: "narrowed to one creator", filter: domain.CourseListFilter{CreatedBy: carol}, want: []string{carol}},
		{name: "an empty, non-nil list when nothing matches", filter: domain.CourseListFilter{CreatedBy: uuid.NewString()}, want: []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := f.courses.ListCreatorIDs(f.ctx, tc.filter)

			require.NoError(t, err)
			assert.NotNil(t, got)
			assert.ElementsMatch(t, tc.want, got)
		})
	}
}
