package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// pickingTree is a knowledge map with an area two levels deep, a sibling
// sub-skill and a concept area:
//
//	picking ─ alternate-picking ─ outside-picking
//	        └ sweep-picking
//	chords ─ triads
func pickingTree() *fakeKnowledgeNodeRepository {
	nodes := seededKnowledgeNodeRepository()
	under := func(parent string) *string { return &parent }
	nodes.put(domain.KnowledgeNode{ID: "picking", Kind: domain.KnowledgeNodeKindSkill, Key: "picking"})
	nodes.put(domain.KnowledgeNode{ID: "alternate-picking", Kind: domain.KnowledgeNodeKindSkill, Key: "alternate-picking", ParentID: under("picking")})
	nodes.put(domain.KnowledgeNode{ID: "outside-picking", Kind: domain.KnowledgeNodeKindSkill, Key: "outside-picking", ParentID: under("alternate-picking")})
	nodes.put(domain.KnowledgeNode{ID: "sweep-picking", Kind: domain.KnowledgeNodeKindSkill, Key: "sweep-picking", ParentID: under("picking")})
	nodes.put(domain.KnowledgeNode{ID: "chords", Kind: domain.KnowledgeNodeKindConcept, Key: "chords"})
	nodes.put(domain.KnowledgeNode{ID: "triads", Kind: domain.KnowledgeNodeKindConcept, Key: "triads", ParentID: under("chords")})
	return nodes
}

func skills(ids ...string) []domain.KnowledgeNode {
	nodes := make([]domain.KnowledgeNode, len(ids))
	for i, id := range ids {
		nodes[i] = domain.KnowledgeNode{ID: id}
	}
	return nodes
}

func itemIDs[T any](items []T, id func(T) string) []string {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = id(item)
	}
	return ids
}

func TestAuthoringLists_SubtreeMatching(t *testing.T) {
	ctx := context.Background()
	page := domain.PageRequest{Limit: 20}

	t.Run("lessons: a skill area finds lessons classified with a sub-skill at any depth", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{ID: "outside-drills", Title: "a", Classification: domain.Classification{Skills: skills("outside-picking")}})
		nodes.put(domain.ContentNode{ID: "triads", Title: "b", Classification: domain.Classification{Skills: skills("skill-1")}})
		svc := newContentServiceWithClassification(nodes, newFakeExpandedContentRepository(), pickingTree())

		got, err := svc.ListContentNodes(ctx, teacherCaller(), domain.ContentNodeFilter{SkillIDs: []string{"picking"}}, page)

		require.NoError(t, err)
		assert.Equal(t, []string{"outside-drills"}, itemIDs(got.Items, func(n domain.ContentNode) string { return n.ID }))
	})

	t.Run("lessons: several skills match any of them", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{ID: "one", Title: "a", Classification: domain.Classification{Skills: skills("skill-1")}})
		nodes.put(domain.ContentNode{ID: "two", Title: "b", Classification: domain.Classification{Skills: skills("skill-2")}})
		nodes.put(domain.ContentNode{ID: "sweep", Title: "c", Classification: domain.Classification{Skills: skills("sweep-picking")}})
		svc := newContentServiceWithClassification(nodes, newFakeExpandedContentRepository(), pickingTree())

		got, err := svc.ListContentNodes(ctx, teacherCaller(), domain.ContentNodeFilter{SkillIDs: []string{"skill-1", "skill-2"}}, page)

		require.NoError(t, err)
		assert.Equal(t, []string{"one", "two"}, itemIDs(got.Items, func(n domain.ContentNode) string { return n.ID }))
	})

	t.Run("exercises: a sub-skill matches neither its parent's items nor its siblings'", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "overview", Title: "a", Skills: skills("picking")})
		exercises.put(domain.Exercise{ID: "sweep", Title: "b", Skills: skills("sweep-picking")})
		exercises.put(domain.Exercise{ID: "outside", Title: "c", Skills: skills("outside-picking")})
		svc := application.NewExerciseService(newFakeChallengeRepository(), exercises, newFakeContentNodeRepository(), pickingTree(), newFakeDiagramRepository(), exerciseInstruments(), newFakeVoiceRepository(), exerciseUsers(), newFakePracticeReferenceWriter(), idSequence(), func() time.Time { return fixedCreatedAt }, noShuffle)

		got, err := svc.ListExercises(ctx, teacherCaller(), domain.ExerciseFilter{SkillIDs: []string{"alternate-picking"}}, page)

		require.NoError(t, err)
		assert.Equal(t, []string{"outside"}, itemIDs(got.Items, func(e domain.Exercise) string { return e.ID }))
	})

	t.Run("diagrams: a concept area finds diagrams classified with a sub-concept", func(t *testing.T) {
		f := newDiagramFixture()
		f.diagrams.knowledge = pickingTree()
		f.diagrams.put(domain.Diagram{ID: "c-triads", Kind: domain.DiagramKindBasic, Names: names("C triads"), Concepts: skills("triads")})
		f.diagrams.put(domain.Diagram{ID: "scale", Kind: domain.DiagramKindBasic, Names: names("Scale"), Concepts: skills("concept-1")})
		svc := application.NewDiagramService(f.diagrams, f.instruments, pickingTree(), newFakeLanguageRepository(), f.users, f.references, f.chords, newFakeExerciseRepository(), idSequence(), func() time.Time { return fixedCreatedAt })

		got, err := svc.ListDiagrams(ctx, adminCaller(), domain.DiagramListFilter{ConceptIDs: []string{"chords"}}, page)

		require.NoError(t, err)
		assert.Equal(t, []string{"c-triads"}, itemIDs(got.Items, func(d domain.Diagram) string { return d.ID }))
	})

	wantPicking := []string{"picking", "alternate-picking", "outside-picking", "sweep-picking"}

	t.Run("learning paths: the repository is asked for the whole area", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		svc := application.NewLearningPathService(newFakeContentNodeRepository(), paths, newFakeCourseVersionRepository(), newFakeContentNodeVersionRepository(), newFakeLanguageRepository(), newFakeUserRepository(), seededInstrumentRepository(), pickingTree(), idSequence(), func() time.Time { return fixedCreatedAt })

		_, err := svc.ListLearningPaths(ctx, adminCaller(), domain.LearningPathFilter{SkillIDs: []string{"picking"}, ConceptIDs: []string{"chords"}}, page)

		require.NoError(t, err)
		assert.ElementsMatch(t, wantPicking, paths.lastFilter.SkillIDs)
		assert.ElementsMatch(t, []string{"chords", "triads"}, paths.lastFilter.ConceptIDs)
	})

	t.Run("path catalog: the repository is asked for the whole area", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		svc := application.NewPathCatalogService(paths, newFakeUserRepository(), pickingTree())

		_, err := svc.ListCatalogPaths(ctx, domain.LearningPathFilter{SkillIDs: []string{"picking"}}, page)

		require.NoError(t, err)
		assert.ElementsMatch(t, wantPicking, paths.lastFilter.SkillIDs)
	})

	t.Run("courses and the course catalog: the repository is asked for the whole area", func(t *testing.T) {
		courses := newFakeCourseRepository()
		svc := application.NewCourseService(newFakeLearningPathRepository(), courses, newFakeCourseVersionRepository(), courseAuthors(), newFakeLanguageRepository(), seededInstrumentRepository(), pickingTree(), idSequence(), func() time.Time { return fixedCreatedAt })

		_, err := svc.ListCourses(ctx, adminCaller(), domain.CourseListFilter{SkillIDs: []string{"picking"}}, page)
		require.NoError(t, err)
		assert.ElementsMatch(t, wantPicking, courses.lastFilter.SkillIDs)

		_, err = svc.ListCatalogCourses(ctx, domain.CourseListFilter{ConceptIDs: []string{"chords"}}, page)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"chords", "triads"}, courses.lastFilter.ConceptIDs)
	})

	t.Run("a skill that does not exist matches nothing", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{ID: "one", Classification: domain.Classification{Skills: skills("skill-1")}})
		svc := newContentServiceWithClassification(nodes, newFakeExpandedContentRepository(), pickingTree())

		got, err := svc.ListContentNodes(ctx, teacherCaller(), domain.ContentNodeFilter{SkillIDs: []string{"no-such-skill"}}, page)

		require.NoError(t, err)
		assert.Empty(t, got.Items)
		assert.Zero(t, got.Total)
	})
}

func TestAuthoringLists_SortAndStatusValidation(t *testing.T) {
	ctx := context.Background()
	page := domain.PageRequest{Limit: 20}
	contentSvc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())
	exerciseSvc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())
	diagramSvc := newDiagramFixture().svc
	courseSvc := newCourseService(newFakeLearningPathRepository(), newFakeCourseRepository())

	tests := []struct {
		name  string
		field string
		list  func() error
	}{
		{name: "lessons refuse an unknown sort", field: "sort", list: func() error {
			_, err := contentSvc.ListContentNodes(ctx, teacherCaller(), domain.ContentNodeFilter{Sort: "popularity"}, page)
			return err
		}},
		{name: "lessons refuse an unknown status", field: "status", list: func() error {
			_, err := contentSvc.ListContentNodes(ctx, teacherCaller(), domain.ContentNodeFilter{Status: "archived"}, page)
			return err
		}},
		{name: "exercises refuse an unknown sort", field: "sort", list: func() error {
			_, err := exerciseSvc.ListExercises(ctx, teacherCaller(), domain.ExerciseFilter{Sort: "popularity"}, page)
			return err
		}},
		{name: "diagrams refuse an unknown sort", field: "sort", list: func() error {
			_, err := diagramSvc.ListDiagrams(ctx, teacherCaller(), domain.DiagramListFilter{Sort: "popularity"}, page)
			return err
		}},
		{name: "courses refuse an unknown sort", field: "sort", list: func() error {
			_, err := courseSvc.ListCourses(ctx, adminCaller(), domain.CourseListFilter{Sort: "popularity"}, page)
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.list()

			var validation *domain.ValidationError
			require.ErrorAs(t, err, &validation)
			assert.Equal(t, tt.field, validation.Fields[0].Field)
		})
	}
}

func TestAuthoringLists_LessonPublishingState(t *testing.T) {
	ctx := context.Background()
	input := application.ContentNodeInput{Title: "Open Chords", ContentType: domain.ContentTypeVideo, SkillIDs: []string{"skill-1"}, ConceptIDs: []string{"concept-1"}, Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, MediaURL: videoMediaURL()}
	revised := input
	revised.Title = "Open Chords, Revised"

	tests := []struct {
		name        string
		steps       func(svc *application.ContentService, id string) error
		wantVersion *int
		wantChanges bool
	}{
		{name: "a new lesson is a draft with unpublished changes", steps: func(*application.ContentService, string) error { return nil }, wantChanges: true},
		{name: "publishing records the version and clears the changes", steps: func(svc *application.ContentService, id string) error {
			_, err := svc.PublishContentNode(ctx, teacherCaller(), id)
			return err
		}, wantVersion: intPtr(1)},
		{name: "editing a published lesson gives it unpublished changes", steps: func(svc *application.ContentService, id string) error {
			if _, err := svc.PublishContentNode(ctx, teacherCaller(), id); err != nil {
				return err
			}
			_, err := svc.UpdateContentNode(ctx, teacherCaller(), id, revised)
			return err
		}, wantVersion: intPtr(1), wantChanges: true},
		{name: "saving a published lesson unchanged keeps it published", steps: func(svc *application.ContentService, id string) error {
			if _, err := svc.PublishContentNode(ctx, teacherCaller(), id); err != nil {
				return err
			}
			_, err := svc.UpdateContentNode(ctx, teacherCaller(), id, input)
			return err
		}, wantVersion: intPtr(1)},
		{name: "editing back to the published values clears the changes", steps: func(svc *application.ContentService, id string) error {
			if _, err := svc.PublishContentNode(ctx, teacherCaller(), id); err != nil {
				return err
			}
			if _, err := svc.UpdateContentNode(ctx, teacherCaller(), id, revised); err != nil {
				return err
			}
			_, err := svc.UpdateContentNode(ctx, teacherCaller(), id, input)
			return err
		}, wantVersion: intPtr(1)},
		{name: "republishing an edited lesson records the next version", steps: func(svc *application.ContentService, id string) error {
			if _, err := svc.PublishContentNode(ctx, teacherCaller(), id); err != nil {
				return err
			}
			if _, err := svc.UpdateContentNode(ctx, teacherCaller(), id, revised); err != nil {
				return err
			}
			_, err := svc.PublishContentNode(ctx, teacherCaller(), id)
			return err
		}, wantVersion: intPtr(2)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())
			created, err := svc.CreateContentNode(ctx, teacherCaller(), input)
			require.NoError(t, err)

			require.NoError(t, tt.steps(svc, created.ID))

			got, err := svc.GetContentNode(ctx, created.ID)
			require.NoError(t, err)
			assert.Equal(t, tt.wantVersion, got.LatestPublishedVersion)
			assert.Equal(t, tt.wantChanges, got.HasUnpublishedChanges)
		})
	}
}

func TestAuthoringLists_UpdatedAt(t *testing.T) {
	ctx := context.Background()
	earlier := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)

	t.Run("a new lesson was last changed when it was created", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		got, err := svc.CreateContentNode(ctx, teacherCaller(), application.ContentNodeInput{Title: "Open Chords", ContentType: domain.ContentTypeVideo, SkillIDs: []string{"skill-1"}, ConceptIDs: []string{"concept-1"}, Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, MediaURL: videoMediaURL()})

		require.NoError(t, err)
		assert.Equal(t, got.CreatedAt, got.UpdatedAt)
	})

	t.Run("editing a lesson records when it last changed", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		svc := newContentService(nodes, newFakeExpandedContentRepository())
		created, err := svc.CreateContentNode(ctx, teacherCaller(), application.ContentNodeInput{Title: "Open Chords", ContentType: domain.ContentTypeVideo, SkillIDs: []string{"skill-1"}, ConceptIDs: []string{"concept-1"}, Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, MediaURL: videoMediaURL()})
		require.NoError(t, err)
		stale := created
		stale.UpdatedAt = earlier
		nodes.put(stale)

		got, err := svc.UpdateContentNode(ctx, teacherCaller(), created.ID, application.ContentNodeInput{Title: "Open Chords in First Position", ContentType: domain.ContentTypeVideo, SkillIDs: []string{"skill-1"}, ConceptIDs: []string{"concept-1"}, Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, MediaURL: videoMediaURL()})

		require.NoError(t, err)
		assert.Equal(t, fixedCreatedAt, got.UpdatedAt)
	})

	t.Run("editing an exercise records when it last changed", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "ex-1", ExerciseType: domain.ExerciseTypeTextResponse, Title: "old", Prompt: domain.NewPlainTextPrompt("p"), Options: textResponseOptions(), CreatedAt: earlier, UpdatedAt: earlier})
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		got, err := svc.UpdateExercise(ctx, teacherCaller(), "ex-1", "new", domain.NewPlainTextPrompt("p"), []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)

		require.NoError(t, err)
		assert.Equal(t, earlier, got.CreatedAt)
		assert.Equal(t, fixedCreatedAt, got.UpdatedAt)
	})

	t.Run("a new diagram was last changed when it was created, and an edit moves it", func(t *testing.T) {
		f := newDiagramFixture()
		created, err := f.svc.CreateDiagram(ctx, teacherCaller(), "guitar", names("Original"), []domain.Position{frettedPos(6, 5)}, []string{"skill-1"}, []string{"concept-1"}, domain.DiagramOptions{LabelDisplay: domain.LabelDisplayInterval})
		require.NoError(t, err)
		assert.Equal(t, created.CreatedAt, created.UpdatedAt)
		stale, err := f.diagrams.GetByID(ctx, created.ID)
		require.NoError(t, err)
		stale.UpdatedAt = earlier
		f.diagrams.put(stale)

		got, err := f.svc.UpdateDiagram(ctx, teacherCaller(), created.ID, application.DiagramUpdate{Names: names("Renamed")})

		require.NoError(t, err)
		assert.Equal(t, fixedCreatedAt, got.UpdatedAt)
	})

	courseTests := []struct {
		name   string
		status domain.CourseStatus
		change func(svc *application.CourseService) error
	}{
		{name: "replacing a course's draft records when it last changed", status: domain.CourseStatusDraft, change: func(svc *application.CourseService) error {
			_, err := svc.ReplaceCourse(ctx, teacherCaller(), "course-1", application.CourseInput{Language: "en", Title: "New", Summary: "New summary", Level: domain.DifficultyLevelBeginner, Checkpoints: checkpointInputs("path-01", "path-02")})
			return err
		}},
		{name: "publishing a course records when it last changed", status: domain.CourseStatusDraft, change: func(svc *application.CourseService) error {
			_, err := svc.PublishCourse(ctx, adminCaller(), "course-1")
			return err
		}},
		{name: "retiring a course records when it last changed", status: domain.CourseStatusPublished, change: func(svc *application.CourseService) error {
			_, err := svc.RetireCourse(ctx, adminCaller(), "course-1")
			return err
		}},
	}
	for _, tt := range courseTests {
		t.Run(tt.name, func(t *testing.T) {
			courses := newFakeCourseRepository()
			course := fingerstyleCourseDraft()
			course.Status = tt.status
			course.UpdatedAt = earlier
			courses.put(course)
			svc := newCourseService(publishedPaths("path-01", "path-02"), courses)

			require.NoError(t, tt.change(svc))

			got, err := svc.GetCourse(ctx, adminCaller(), "course-1")
			require.NoError(t, err)
			assert.Equal(t, fixedCreatedAt, got.UpdatedAt)
		})
	}
}

func TestAuthoringLists_CreatorCounts(t *testing.T) {
	ctx := context.Background()

	t.Run("lesson creators carry how many lessons each made, in name order", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		for _, n := range []domain.ContentNode{
			{ID: "a", TeacherID: teacherCaller().ID}, {ID: "b", TeacherID: teacherCaller().ID}, {ID: "c", TeacherID: teacherCaller().ID},
			{ID: "d", TeacherID: otherTeacherCaller().ID},
		} {
			nodes.put(n)
		}
		svc := application.NewContentService(nodes, newFakeExpandedContentRepository(), seededKnowledgeNodeRepository(), newFakeContentNodeVersionRepository(), newFakeDiagramRepository(), seededInstrumentRepository(), newFakeVoiceRepository(), newFakeSongChartRepository(), exerciseUsers(), idSequence(), func() time.Time { return fixedCreatedAt })

		got, err := svc.ListContentNodeCreators(ctx, teacherCaller(), "")

		require.NoError(t, err)
		assert.Equal(t, []application.Creator{
			{UserID: teacherCaller().ID, DisplayName: "Bob Ferreira", ItemCount: 3},
			{UserID: otherTeacherCaller().ID, DisplayName: "Carol Souza", ItemCount: 1},
		}, got)
	})

	t.Run("a student cannot list lesson creators", func(t *testing.T) {
		svc := application.NewContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository(), seededKnowledgeNodeRepository(), newFakeContentNodeVersionRepository(), newFakeDiagramRepository(), seededInstrumentRepository(), newFakeVoiceRepository(), newFakeSongChartRepository(), exerciseUsers(), idSequence(), func() time.Time { return fixedCreatedAt })

		_, err := svc.ListContentNodeCreators(ctx, studentCaller(), "")

		require.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("exercise creators carry how many exercises each made", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "a", CreatedBy: teacherCaller().ID})
		exercises.put(domain.Exercise{ID: "b", CreatedBy: otherTeacherCaller().ID})
		exercises.put(domain.Exercise{ID: "c", CreatedBy: otherTeacherCaller().ID})
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		got, err := svc.ListExerciseCreators(ctx, teacherCaller(), "")

		require.NoError(t, err)
		assert.Equal(t, []int{1, 2}, idsOfCounts(got))
	})

	t.Run("a diagram creator's count leaves out the diagrams the caller cannot see", func(t *testing.T) {
		f := newDiagramFixture()
		for _, kind := range []domain.DiagramKind{domain.DiagramKindBasic, domain.DiagramKindCustom, domain.DiagramKindCustom} {
			owner := otherTeacherCaller()
			if kind == domain.DiagramKindBasic {
				owner = adminCaller()
			}
			_, err := f.svc.CreateDiagram(ctx, owner, "guitar", names("D"), []domain.Position{frettedPos(6, 5)}, []string{"skill-1"}, []string{"concept-1"}, domain.DiagramOptions{Kind: kind})
			require.NoError(t, err)
		}

		got, err := f.svc.ListDiagramCreators(ctx, teacherCaller(), "")

		require.NoError(t, err)
		assert.Equal(t, []application.Creator{{UserID: adminCaller().ID, DisplayName: "Ana Admin", ItemCount: 1}}, got)
	})
}

func idsOfCounts(creators []application.Creator) []int {
	counts := make([]int, len(creators))
	for i, c := range creators {
		counts[i] = c.ItemCount
	}
	return counts
}

// withoutCounts drops each creator's ItemCount, for the tests that check
// only which creators are listed and in what order.
func withoutCounts(creators []application.Creator) []application.Creator {
	stripped := make([]application.Creator, len(creators))
	for i, c := range creators {
		c.ItemCount = 0
		stripped[i] = c
	}
	return stripped
}
