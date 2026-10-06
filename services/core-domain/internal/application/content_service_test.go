package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

var fixedCreatedAt = time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)

func newContentService(nodes *fakeContentNodeRepository, expanded *fakeExpandedContentRepository) *application.ContentService {
	return newContentServiceWithClassification(nodes, expanded, seededKnowledgeNodeRepository())
}

func newContentServiceWithClassification(nodes *fakeContentNodeRepository, expanded *fakeExpandedContentRepository, knowledge *fakeKnowledgeNodeRepository) *application.ContentService {
	return newContentServiceWithVersions(nodes, expanded, knowledge, newFakeContentNodeVersionRepository())
}

func newContentServiceWithVersions(nodes *fakeContentNodeRepository, expanded *fakeExpandedContentRepository, knowledge *fakeKnowledgeNodeRepository, versions *fakeContentNodeVersionRepository) *application.ContentService {
	return newContentServiceWithDiagrams(nodes, expanded, knowledge, versions, newFakeDiagramRepository())
}

func newContentServiceWithDiagrams(nodes *fakeContentNodeRepository, expanded *fakeExpandedContentRepository, knowledge *fakeKnowledgeNodeRepository, versions *fakeContentNodeVersionRepository, diagrams *fakeDiagramRepository) *application.ContentService {
	return application.NewContentService(nodes, expanded, knowledge, versions, diagrams, seededInstrumentRepository(), newFakeVoiceRepository(), idSequence(), func() time.Time { return fixedCreatedAt })
}

// seededKnowledgeNodeRepository returns a fake pre-populated with the ids
// every classification-shaped test references — skills "skill-1"/"skill-2"
// and concepts "concept-1"/"concept-2", all for every instrument — so
// existence checks pass without every test needing to seed them.
func seededKnowledgeNodeRepository() *fakeKnowledgeNodeRepository {
	nodes := newFakeKnowledgeNodeRepository()
	for _, id := range []string{"skill-1", "skill-2"} {
		nodes.put(domain.KnowledgeNode{ID: id, Kind: domain.KnowledgeNodeKindSkill, Key: id})
	}
	for _, id := range []string{"concept-1", "concept-2"} {
		nodes.put(domain.KnowledgeNode{ID: id, Kind: domain.KnowledgeNodeKindConcept, Key: id})
	}
	return nodes
}

func teacherCaller() domain.User      { return domain.User{ID: "teacher-1", Role: domain.RoleTeacher} }
func otherTeacherCaller() domain.User { return domain.User{ID: "teacher-2", Role: domain.RoleTeacher} }
func adminCaller() domain.User        { return domain.User{ID: "admin-1", Role: domain.RoleAdmin} }
func studentCaller() domain.User      { return domain.User{ID: "student-1", Role: domain.RoleStudent} }

func TestContentService_CreateContentNode(t *testing.T) {
	t.Run("a teacher creates a video content node with classification", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		node, err := svc.CreateContentNode(context.Background(), teacherCaller(), application.ContentNodeInput{Title: "Introduction to Triad Shapes", ContentType: domain.ContentTypeVideo, SkillIDs: []string{"skill-1"}, ConceptIDs: []string{"concept-1"}, Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, MediaURL: videoMediaURL(), RichContent: nil})

		require.NoError(t, err)
		assert.Equal(t, "teacher-1", node.TeacherID)
		assert.Equal(t, domain.ReviewStatePending, node.Classification.ReviewState)
	})

	t.Run("an admin creates a content node", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.CreateContentNode(context.Background(), adminCaller(), application.ContentNodeInput{Title: "Sweep Picking Fundamentals", ContentType: domain.ContentTypeVideo, SkillIDs: []string{"skill-1"}, ConceptIDs: []string{"concept-1"}, Difficulty: domain.DifficultyLevelAdvanced, Languages: []string{"en"}, MediaURL: videoMediaURL(), RichContent: nil})

		require.NoError(t, err)
	})

	t.Run("a student cannot create a content node", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.CreateContentNode(context.Background(), studentCaller(), application.ContentNodeInput{Title: "Title", ContentType: domain.ContentTypeVideo, SkillIDs: []string{"skill-1"}, ConceptIDs: []string{"concept-1"}, Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, MediaURL: videoMediaURL(), RichContent: nil})

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("creating a content node without a title is rejected", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.CreateContentNode(context.Background(), teacherCaller(), application.ContentNodeInput{Title: "", ContentType: domain.ContentTypeVideo, SkillIDs: []string{"skill-1"}, ConceptIDs: []string{"concept-1"}, Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, MediaURL: videoMediaURL(), RichContent: nil})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assert.Equal(t, "title", valErr.Fields[0].Field)
	})

	t.Run("creating a content node without classification is rejected", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.CreateContentNode(context.Background(), teacherCaller(), application.ContentNodeInput{Title: "Title", ContentType: domain.ContentTypeVideo, SkillIDs: nil, ConceptIDs: nil, Difficulty: "", Languages: []string{"en"}, MediaURL: videoMediaURL(), RichContent: nil})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assert.Equal(t, "classification", valErr.Fields[0].Field)
	})

	t.Run("creating a content node with an unrecognised difficulty level is rejected", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.CreateContentNode(context.Background(), teacherCaller(), application.ContentNodeInput{Title: "Title", ContentType: domain.ContentTypeVideo, SkillIDs: []string{"skill-1"}, ConceptIDs: []string{"concept-1"}, Difficulty: domain.DifficultyLevel("master"), Languages: []string{"en"}, MediaURL: videoMediaURL(), RichContent: nil})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assert.Equal(t, "difficulty_level", valErr.Fields[0].Field)
	})

	t.Run("creating a content node with no skills is rejected", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.CreateContentNode(context.Background(), teacherCaller(), application.ContentNodeInput{Title: "Title", ContentType: domain.ContentTypeVideo, SkillIDs: nil, ConceptIDs: []string{"concept-1"}, Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, MediaURL: videoMediaURL(), RichContent: nil})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "skill_ids")
	})

	t.Run("creating a content node with no concepts is rejected", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.CreateContentNode(context.Background(), teacherCaller(), application.ContentNodeInput{Title: "Title", ContentType: domain.ContentTypeVideo, SkillIDs: []string{"skill-1"}, ConceptIDs: nil, Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, MediaURL: videoMediaURL(), RichContent: nil})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "concept_ids")
	})

	t.Run("creating a content node with a skill id that does not exist is rejected", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.CreateContentNode(context.Background(), teacherCaller(), application.ContentNodeInput{Title: "Title", ContentType: domain.ContentTypeVideo, SkillIDs: []string{"missing-skill"}, ConceptIDs: []string{"concept-1"}, Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, MediaURL: videoMediaURL(), RichContent: nil})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "skill_ids")
	})

	t.Run("creating a content node with a concept id that does not exist is rejected", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.CreateContentNode(context.Background(), teacherCaller(), application.ContentNodeInput{Title: "Title", ContentType: domain.ContentTypeVideo, SkillIDs: []string{"skill-1"}, ConceptIDs: []string{"missing-concept"}, Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, MediaURL: videoMediaURL(), RichContent: nil})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "concept_ids")
	})

	classificationRules := []struct {
		name        string
		skillIDs    []string
		conceptIDs  []string
		instruments []string
		wantField   string // empty means the node is created
	}{
		{name: "a concept given as a skill is rejected", skillIDs: []string{"concept-2"}, conceptIDs: []string{"concept-1"}, wantField: "skill_ids"},
		{name: "a skill given as a concept is rejected", skillIDs: []string{"skill-1"}, conceptIDs: []string{"skill-2"}, wantField: "concept_ids"},
		{name: "guitar content may use an every-instrument skill", skillIDs: []string{"skill-1"}, conceptIDs: []string{"concept-1"}, instruments: []string{"guitar"}},
		{name: "guitar-and-piano content may use a guitar skill", skillIDs: []string{"palm-muting"}, conceptIDs: []string{"concept-1"}, instruments: []string{"guitar", "piano"}},
		{name: "piano content cannot use a guitar skill", skillIDs: []string{"palm-muting"}, conceptIDs: []string{"concept-1"}, instruments: []string{"piano"}, wantField: "skill_ids"},
		{name: "every-instrument content cannot use a guitar skill", skillIDs: []string{"palm-muting"}, conceptIDs: []string{"concept-1"}, wantField: "skill_ids"},
		{name: "piano content cannot use a guitar concept", skillIDs: []string{"skill-1"}, conceptIDs: []string{"fretboard"}, instruments: []string{"piano"}, wantField: "concept_ids"},
	}
	for _, tt := range classificationRules {
		t.Run(tt.name, func(t *testing.T) {
			knowledge := seededKnowledgeNodeRepository()
			knowledge.put(domain.KnowledgeNode{ID: "palm-muting", Kind: domain.KnowledgeNodeKindSkill, Key: "palm-muting", InstrumentIDs: []string{"guitar"}})
			knowledge.put(domain.KnowledgeNode{ID: "fretboard", Kind: domain.KnowledgeNodeKindConcept, Key: "fretboard", InstrumentIDs: []string{"guitar"}})
			svc := newContentServiceWithClassification(newFakeContentNodeRepository(), newFakeExpandedContentRepository(), knowledge)

			_, err := svc.CreateContentNode(context.Background(), teacherCaller(), application.ContentNodeInput{Title: "Title", ContentType: domain.ContentTypeVideo, SkillIDs: tt.skillIDs, ConceptIDs: tt.conceptIDs, InstrumentIDs: tt.instruments, Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, MediaURL: videoMediaURL()})

			if tt.wantField == "" {
				require.NoError(t, err)
				return
			}
			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assertHasField(t, valErr, tt.wantField)
		})
	}

	t.Run("updating a content node's instruments so a skill no longer suits them is rejected", func(t *testing.T) {
		knowledge := seededKnowledgeNodeRepository()
		knowledge.put(domain.KnowledgeNode{ID: "palm-muting", Kind: domain.KnowledgeNodeKindSkill, Key: "palm-muting", InstrumentIDs: []string{"guitar"}})
		svc := newContentServiceWithClassification(newFakeContentNodeRepository(), newFakeExpandedContentRepository(), knowledge)
		input := application.ContentNodeInput{Title: "Palm muting", ContentType: domain.ContentTypeVideo, SkillIDs: []string{"palm-muting"}, ConceptIDs: []string{"concept-1"}, InstrumentIDs: []string{"guitar"}, Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, MediaURL: videoMediaURL()}
		node, err := svc.CreateContentNode(context.Background(), teacherCaller(), input)
		require.NoError(t, err)

		input.InstrumentIDs = []string{"piano"}
		_, err = svc.UpdateContentNode(context.Background(), teacherCaller(), node.ID, input)

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assertHasField(t, valErr, "skill_ids")
	})
}

func TestContentService_CreateContentNodeBody(t *testing.T) {
	malformed := domain.PromptDocument{Type: "not-a-doc"}

	tests := []struct {
		name        string
		contentType domain.ContentType
		mediaURL    *string
		richContent *domain.PromptDocument
		wantField   string
	}{
		{"a video node with a media url is created", domain.ContentTypeVideo, videoMediaURL(), nil, ""},
		{"an article node with a rich content body is created", domain.ContentTypeArticle, nil, articleBody(), ""},
		{"a video node without a media url is rejected", domain.ContentTypeVideo, nil, nil, "media_url"},
		{"a video node with an empty media url is rejected", domain.ContentTypeVideo, strPtr(""), nil, "media_url"},
		{"a video node with a plain http media url is created", domain.ContentTypeVideo, strPtr("http://cdn.example.com/lesson.mp4"), nil, ""},
		{"a video node with a YouTube link is created", domain.ContentTypeVideo, strPtr("https://www.youtube.com/watch?v=dQw4w9WgXcQ"), nil, ""},
		{"a video node whose media url is not a URL is rejected", domain.ContentTypeVideo, strPtr("not a url"), nil, "media_url"},
		{"a video node with a relative media url is rejected", domain.ContentTypeVideo, strPtr("/videos/lesson.mp4"), nil, "media_url"},
		{"a video node with a javascript media url is rejected", domain.ContentTypeVideo, strPtr("javascript:alert(1)"), nil, "media_url"},
		{"a video node with an ftp media url is rejected", domain.ContentTypeVideo, strPtr("ftp://cdn.example.com/lesson.mp4"), nil, "media_url"},
		{"a video node with a media url that has no host is rejected", domain.ContentTypeVideo, strPtr("https://"), nil, "media_url"},
		{"a video node carrying rich content is rejected", domain.ContentTypeVideo, videoMediaURL(), articleBody(), "rich_content"},
		{"an article node without a body is rejected", domain.ContentTypeArticle, nil, nil, "rich_content"},
		{"an article node carrying a media url is rejected", domain.ContentTypeArticle, videoMediaURL(), articleBody(), "media_url"},
		{"an article node with a malformed body is rejected", domain.ContentTypeArticle, nil, &malformed, "rich_content"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

			node, err := svc.CreateContentNode(context.Background(), teacherCaller(), application.ContentNodeInput{Title: "Title", ContentType: tt.contentType, SkillIDs: []string{"skill-1"}, ConceptIDs: []string{"concept-1"}, Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, MediaURL: tt.mediaURL, RichContent: tt.richContent})

			if tt.wantField == "" {
				require.NoError(t, err)
				assert.Equal(t, tt.mediaURL, node.MediaURL)
				assert.Equal(t, tt.richContent, node.RichContent)
				return
			}
			var valErr *domain.ValidationError
			require.True(t, errors.As(err, &valErr))
			assertHasField(t, valErr, tt.wantField)
		})
	}
}

func TestContentService_UpdateContentNodeBody(t *testing.T) {
	newArticleNode := func() domain.ContentNode {
		node := articleNode("node-1")
		node.RichContent = articleBody()
		return node
	}
	revised := richTextContent("Revised body")

	tests := []struct {
		name        string
		node        domain.ContentNode
		mediaURL    *string
		richContent *domain.PromptDocument
		wantField   string
	}{
		{"a teacher replaces a video's media url", videoNode("node-1"), strPtr("https://cdn.motifpath.io/videos/new.mp4"), nil, ""},
		{"a teacher replaces an article's body", newArticleNode(), nil, &revised, ""},
		{"updating a video without a media url is rejected", videoNode("node-1"), nil, nil, "media_url"},
		{"updating a video with a javascript media url is rejected", videoNode("node-1"), strPtr("javascript:alert(1)"), nil, "media_url"},
		{"updating a video with a media url that is not a URL is rejected", videoNode("node-1"), strPtr("not a url"), nil, "media_url"},
		{"updating an article without a body is rejected", newArticleNode(), nil, nil, "rich_content"},
		{"updating an article with a media url is rejected", newArticleNode(), videoMediaURL(), &revised, "media_url"},
		{"updating a video with rich content is rejected", videoNode("node-1"), videoMediaURL(), &revised, "rich_content"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes := newFakeContentNodeRepository()
			nodes.put(tt.node)
			svc := newContentService(nodes, newFakeExpandedContentRepository())

			got, err := svc.UpdateContentNode(context.Background(), teacherCaller(), "node-1", application.ContentNodeInput{Title: "Title", SkillIDs: []string{"skill-1"}, ConceptIDs: []string{"concept-1"}, Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, MediaURL: tt.mediaURL, RichContent: tt.richContent})

			if tt.wantField == "" {
				require.NoError(t, err)
				assert.Equal(t, tt.mediaURL, got.MediaURL)
				assert.Equal(t, tt.richContent, got.RichContent)
				return
			}
			var valErr *domain.ValidationError
			require.True(t, errors.As(err, &valErr))
			assertHasField(t, valErr, tt.wantField)
		})
	}
}

func TestContentService_GetContentNode(t *testing.T) {
	t.Run("any authenticated user retrieves a content node by id", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		node := domain.ContentNode{ID: "node-1", Title: "Intro"}
		nodes.put(node)
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		got, err := svc.GetContentNode(context.Background(), "node-1")

		require.NoError(t, err)
		assert.Equal(t, node, got)
	})

	t.Run("retrieving a content node that does not exist returns not found", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.GetContentNode(context.Background(), "missing")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestContentService_ListContentNodes(t *testing.T) {
	firstPage := domain.PageRequest{Limit: 20, Offset: 0}

	t.Run("a teacher lists all content nodes", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		nodes.put(articleNode("node-2"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		got, err := svc.ListContentNodes(context.Background(), teacherCaller(), domain.ContentNodeFilter{}, firstPage)

		require.NoError(t, err)
		assert.Len(t, got.Items, 2)
		assert.Equal(t, 2, got.Total)
	})

	t.Run("an admin lists all content nodes", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		got, err := svc.ListContentNodes(context.Background(), adminCaller(), domain.ContentNodeFilter{}, firstPage)

		require.NoError(t, err)
		assert.Len(t, got.Items, 1)
	})

	t.Run("filters by content type", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		nodes.put(articleNode("node-2"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		got, err := svc.ListContentNodes(context.Background(), teacherCaller(), domain.ContentNodeFilter{ContentType: domain.ContentTypeArticle}, firstPage)

		require.NoError(t, err)
		require.Len(t, got.Items, 1)
		assert.Equal(t, "node-2", got.Items[0].ID)
	})

	t.Run("filters by skill", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{ID: "node-1", Classification: domain.Classification{Skills: []domain.KnowledgeNode{{ID: "skill-1"}}}})
		nodes.put(domain.ContentNode{ID: "node-2", Classification: domain.Classification{Skills: []domain.KnowledgeNode{{ID: "skill-2"}}}})
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		got, err := svc.ListContentNodes(context.Background(), teacherCaller(), domain.ContentNodeFilter{SkillID: "skill-2"}, firstPage)

		require.NoError(t, err)
		require.Len(t, got.Items, 1)
		assert.Equal(t, "node-2", got.Items[0].ID)
	})

	t.Run("filters by concept", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{ID: "node-1", Classification: domain.Classification{Concepts: []domain.KnowledgeNode{{ID: "concept-1"}}}})
		nodes.put(domain.ContentNode{ID: "node-2", Classification: domain.Classification{Concepts: []domain.KnowledgeNode{{ID: "concept-2"}}}})
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		got, err := svc.ListContentNodes(context.Background(), teacherCaller(), domain.ContentNodeFilter{ConceptID: "concept-2"}, firstPage)

		require.NoError(t, err)
		require.Len(t, got.Items, 1)
		assert.Equal(t, "node-2", got.Items[0].ID)
	})

	t.Run("searches by title text", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{ID: "node-1", Title: "Open Chords"})
		nodes.put(domain.ContentNode{ID: "node-2", Title: "Scales"})
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		got, err := svc.ListContentNodes(context.Background(), teacherCaller(), domain.ContentNodeFilter{Query: "chords"}, firstPage)

		require.NoError(t, err)
		require.Len(t, got.Items, 1)
		assert.Equal(t, "node-1", got.Items[0].ID)
	})

	t.Run("returns the requested page and the filtered total", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		for _, id := range []string{"node-1", "node-2", "node-3", "node-4", "node-5"} {
			nodes.put(domain.ContentNode{ID: id, Title: id})
		}
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		got, err := svc.ListContentNodes(context.Background(), teacherCaller(), domain.ContentNodeFilter{}, domain.PageRequest{Limit: 2, Offset: 2})

		require.NoError(t, err)
		assert.Equal(t, 5, got.Total)
		assert.Equal(t, []string{"node-3", "node-4"}, []string{got.Items[0].ID, got.Items[1].ID})
	})

	t.Run("listing when none exist returns an empty page", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		got, err := svc.ListContentNodes(context.Background(), teacherCaller(), domain.ContentNodeFilter{}, firstPage)

		require.NoError(t, err)
		assert.Empty(t, got.Items)
		assert.Zero(t, got.Total)
	})

	t.Run("a student cannot list content nodes", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.ListContentNodes(context.Background(), studentCaller(), domain.ContentNodeFilter{}, firstPage)

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestContentService_UpdateContentNode(t *testing.T) {
	t.Run("a teacher updates a content node's title and classification", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{
			ID: "node-1", TeacherID: "teacher-1", ContentType: domain.ContentTypeVideo,
			Classification: domain.Classification{Skills: []domain.KnowledgeNode{{ID: "skill-1"}}, Concepts: []domain.KnowledgeNode{{ID: "concept-1"}}, DifficultyLevel: domain.DifficultyLevelBeginner, ReviewState: domain.ReviewStateConfirmed},
		})
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		got, err := svc.UpdateContentNode(context.Background(), teacherCaller(), "node-1", application.ContentNodeInput{Title: "Revised title", SkillIDs: []string{"skill-1"}, ConceptIDs: []string{"concept-1"}, Difficulty: domain.DifficultyLevelIntermediate, Languages: []string{"en"}, MediaURL: videoMediaURL(), RichContent: nil})

		require.NoError(t, err)
		assert.Equal(t, "Revised title", got.Title)
		assert.Equal(t, domain.DifficultyLevelIntermediate, got.Classification.DifficultyLevel)
	})

	t.Run("a teacher adds a second skill to a content node's classification", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{
			ID: "node-1", TeacherID: "teacher-1", ContentType: domain.ContentTypeVideo,
			Classification: domain.Classification{Skills: []domain.KnowledgeNode{{ID: "skill-1"}}, Concepts: []domain.KnowledgeNode{{ID: "concept-1"}}, DifficultyLevel: domain.DifficultyLevelBeginner},
		})
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		got, err := svc.UpdateContentNode(context.Background(), teacherCaller(), "node-1", application.ContentNodeInput{Title: "Title", SkillIDs: []string{"skill-1", "skill-2"}, ConceptIDs: []string{"concept-1"}, Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, MediaURL: videoMediaURL(), RichContent: nil})

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"skill-1", "skill-2"}, got.Classification.SkillIDs())
	})

	t.Run("updating does not change content type", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		got, err := svc.UpdateContentNode(context.Background(), teacherCaller(), "node-1", application.ContentNodeInput{Title: "Revised title", SkillIDs: []string{"skill-1"}, ConceptIDs: []string{"concept-1"}, Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, MediaURL: videoMediaURL(), RichContent: nil})

		require.NoError(t, err)
		assert.Equal(t, domain.ContentTypeVideo, got.ContentType)
	})

	t.Run("updating does not reset an admin-confirmed review state", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{
			ID: "node-1", TeacherID: "teacher-1", ContentType: domain.ContentTypeVideo,
			Classification: domain.Classification{Skills: []domain.KnowledgeNode{{ID: "skill-1"}}, Concepts: []domain.KnowledgeNode{{ID: "concept-1"}}, DifficultyLevel: domain.DifficultyLevelBeginner, ReviewState: domain.ReviewStateConfirmed},
		})
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		got, err := svc.UpdateContentNode(context.Background(), teacherCaller(), "node-1", application.ContentNodeInput{Title: "Revised title", SkillIDs: []string{"skill-1"}, ConceptIDs: []string{"concept-1"}, Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, MediaURL: videoMediaURL(), RichContent: nil})

		require.NoError(t, err)
		assert.Equal(t, domain.ReviewStateConfirmed, got.Classification.ReviewState)
	})

	t.Run("updating without a title is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		_, err := svc.UpdateContentNode(context.Background(), teacherCaller(), "node-1", application.ContentNodeInput{Title: "", SkillIDs: []string{"skill-1"}, ConceptIDs: []string{"concept-1"}, Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, MediaURL: videoMediaURL(), RichContent: nil})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "title")
	})

	t.Run("updating without classification is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		_, err := svc.UpdateContentNode(context.Background(), teacherCaller(), "node-1", application.ContentNodeInput{Title: "Title", SkillIDs: nil, ConceptIDs: nil, Difficulty: "", Languages: []string{"en"}, MediaURL: videoMediaURL(), RichContent: nil})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "classification")
	})

	t.Run("a student cannot update a content node", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		_, err := svc.UpdateContentNode(context.Background(), studentCaller(), "node-1", application.ContentNodeInput{Title: "Hijacked title", SkillIDs: []string{"skill-1"}, ConceptIDs: []string{"concept-1"}, Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, MediaURL: videoMediaURL(), RichContent: nil})

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("a teacher cannot update another teacher's content node", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1")) // owned by teacher-1
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		_, err := svc.UpdateContentNode(context.Background(), otherTeacherCaller(), "node-1", application.ContentNodeInput{Title: "Hijacked title", SkillIDs: []string{"skill-1"}, ConceptIDs: []string{"concept-1"}, Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, MediaURL: videoMediaURL(), RichContent: nil})

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("an admin can update any teacher's content node", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1")) // owned by teacher-1
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		got, err := svc.UpdateContentNode(context.Background(), adminCaller(), "node-1", application.ContentNodeInput{Title: "Revised by admin", SkillIDs: []string{"skill-1"}, ConceptIDs: []string{"concept-1"}, Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, MediaURL: videoMediaURL(), RichContent: nil})

		require.NoError(t, err)
		assert.Equal(t, "Revised by admin", got.Title)
	})

	t.Run("updating a content node that does not exist returns not found", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.UpdateContentNode(context.Background(), teacherCaller(), "missing", application.ContentNodeInput{Title: "Title", SkillIDs: []string{"skill-1"}, ConceptIDs: []string{"concept-1"}, Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, MediaURL: videoMediaURL(), RichContent: nil})

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func videoNode(id string) domain.ContentNode {
	return domain.ContentNode{ID: id, TeacherID: "teacher-1", ContentType: domain.ContentTypeVideo}
}

func articleNode(id string) domain.ContentNode {
	return domain.ContentNode{ID: id, TeacherID: "teacher-1", ContentType: domain.ContentTypeArticle}
}

func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }

func videoMediaURL() *string {
	return strPtr("https://cdn.motifpath.io/videos/triad-shapes-intro.mp4")
}

func articleBody() *domain.PromptDocument {
	doc := richTextContent("Chord theory explains how notes combine into triads.")
	return &doc
}

func TestContentService_CreateExpandedContent(t *testing.T) {
	t.Run("a teacher adds an image to a video lesson at a specific timestamp", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		item, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/img.png"), nil, nil, nil,
			intPtr(150), intPtr(165), nil, nil, nil)

		require.NoError(t, err)
		assert.Equal(t, "node-1", item.ContentNodeID)
	})

	t.Run("a teacher adds an image to an article at a specific paragraph", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(articleNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		item, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/img.png"), nil, nil, nil,
			nil, nil, intPtr(3), intPtr(8000), nil)

		require.NoError(t, err)
		assert.Equal(t, "node-1", item.ContentNodeID)
	})

	t.Run("adding expanded content to a video node without trigger_at_seconds is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		_, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/img.png"), nil, nil, nil,
			nil, nil, intPtr(3), intPtr(5000), nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "trigger_at_seconds")
	})

	t.Run("hide_at_seconds not greater than trigger_at_seconds is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		_, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/img.png"), nil, nil, nil,
			intPtr(150), intPtr(150), nil, nil, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "hide_at_seconds")
	})

	t.Run("article node without trigger_at_paragraph is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(articleNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		_, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/img.png"), nil, nil, nil,
			intPtr(90), intPtr(100), nil, nil, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "trigger_at_paragraph")
	})

	t.Run("article node with trigger_at_paragraph zero is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(articleNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		_, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/img.png"), nil, nil, nil,
			nil, nil, intPtr(0), intPtr(5000), nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "trigger_at_paragraph")
	})

	t.Run("article node without duration_ms is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(articleNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		_, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/img.png"), nil, nil, nil,
			nil, nil, intPtr(3), nil, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "duration_ms")
	})

	t.Run("a student cannot add expanded content", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		_, err := svc.CreateExpandedContent(context.Background(), studentCaller(), "node-1",
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/img.png"), nil, nil, nil,
			intPtr(150), intPtr(165), nil, nil, nil)

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("adding expanded content to a non-existent content node returns not found", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "missing",
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/img.png"), nil, nil, nil,
			intPtr(150), intPtr(165), nil, nil, nil)

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func seedContentDiagram(t *testing.T, diagrams *fakeDiagramRepository, id, instrumentID string) domain.Diagram {
	t.Helper()
	diagram := domain.Diagram{ID: id, InstrumentID: instrumentID, Names: domain.LocalizedText{"en": id}, Positions: []domain.Position{{ID: "pos-1", Interval: "R", NoteName: "A"}}}
	require.NoError(t, diagrams.Create(context.Background(), diagram))
	return diagram
}

func TestContentService_CreateExpandedContent_Diagram(t *testing.T) {
	t.Run("a teacher adds a diagram to a video lesson at a specific timestamp", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		diagrams := newFakeDiagramRepository()
		seedContentDiagram(t, diagrams, "diagram-1", "guitar")
		svc := newContentServiceWithDiagrams(nodes, newFakeExpandedContentRepository(), seededKnowledgeNodeRepository(), newFakeContentNodeVersionRepository(), diagrams)

		item, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeDiagram, nil, nil,
			&domain.DiagramRef{DiagramID: "diagram-1", Layers: domain.DiagramLayers{Intervals: true}}, nil,
			intPtr(150), intPtr(165), nil, nil, nil)

		require.NoError(t, err)
		assert.Equal(t, domain.ExpandedContentTypeDiagram, item.ContentType)
	})

	t.Run("a teacher adds a stack of two diagrams to an article at a specific paragraph", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(articleNode("node-1"))
		diagrams := newFakeDiagramRepository()
		seedContentDiagram(t, diagrams, "diagram-1", "guitar")
		seedContentDiagram(t, diagrams, "diagram-2", "guitar")
		svc := newContentServiceWithDiagrams(nodes, newFakeExpandedContentRepository(), seededKnowledgeNodeRepository(), newFakeContentNodeVersionRepository(), diagrams)

		item, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeDiagram, nil, nil, nil,
			&domain.DiagramStackRef{Stack: []domain.DiagramRef{
				{DiagramID: "diagram-1", Layers: domain.DiagramLayers{Intervals: true}},
				{DiagramID: "diagram-2", Layers: domain.DiagramLayers{Intervals: true}},
			}},
			nil, nil, intPtr(3), intPtr(8000), nil)

		require.NoError(t, err)
		require.NotNil(t, item.DiagramStackRef)
		assert.Len(t, item.DiagramStackRef.Stack, 2)
	})

	t.Run("adding a diagram stack from two different instruments is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		diagrams := newFakeDiagramRepository()
		seedContentDiagram(t, diagrams, "diagram-1", "guitar")
		seedContentDiagram(t, diagrams, "diagram-2", "piano")
		svc := newContentServiceWithDiagrams(nodes, newFakeExpandedContentRepository(), seededKnowledgeNodeRepository(), newFakeContentNodeVersionRepository(), diagrams)

		_, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeDiagram, nil, nil, nil,
			&domain.DiagramStackRef{Stack: []domain.DiagramRef{
				{DiagramID: "diagram-1", Layers: domain.DiagramLayers{Intervals: true}},
				{DiagramID: "diagram-2", Layers: domain.DiagramLayers{Intervals: true}},
			}},
			intPtr(150), intPtr(165), nil, nil, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "diagram_stack_ref")
	})

	t.Run("a diagram_ref pointing at a non-existent diagram is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentServiceWithDiagrams(nodes, newFakeExpandedContentRepository(), seededKnowledgeNodeRepository(), newFakeContentNodeVersionRepository(), newFakeDiagramRepository())

		_, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeDiagram, nil, nil,
			&domain.DiagramRef{DiagramID: "missing", Layers: domain.DiagramLayers{Intervals: true}}, nil,
			intPtr(150), intPtr(165), nil, nil, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "diagram_ref")
	})

	t.Run("a diagram_stack_ref entry pointing at a non-existent diagram is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		diagrams := newFakeDiagramRepository()
		seedContentDiagram(t, diagrams, "diagram-1", "guitar")
		svc := newContentServiceWithDiagrams(nodes, newFakeExpandedContentRepository(), seededKnowledgeNodeRepository(), newFakeContentNodeVersionRepository(), diagrams)

		_, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeDiagram, nil, nil, nil,
			&domain.DiagramStackRef{Stack: []domain.DiagramRef{
				{DiagramID: "diagram-1", Layers: domain.DiagramLayers{Intervals: true}},
				{DiagramID: "missing", Layers: domain.DiagramLayers{Intervals: true}},
			}},
			intPtr(150), intPtr(165), nil, nil, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "diagram_stack_ref")
	})

	t.Run("creating a diagram item without a diagram reference is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		_, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeDiagram, nil, nil, nil, nil,
			intPtr(150), intPtr(165), nil, nil, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "diagram_ref")
	})

	t.Run("a diagram plays with its own voice, tempo and looping", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		diagrams := newFakeDiagramRepository()
		seedContentDiagram(t, diagrams, "diagram-1", "guitar")
		svc := newContentServiceWithDiagrams(nodes, newFakeExpandedContentRepository(), seededKnowledgeNodeRepository(), newFakeContentNodeVersionRepository(), diagrams)
		playback := &domain.DiagramRefPlayback{Direction: domain.DiagramPlaybackDirectionReversed, TempoBPM: intPtr(60), VoiceID: strPtr("acoustic-guitar"), Loop: true}

		item, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeDiagram, nil, nil,
			&domain.DiagramRef{DiagramID: "diagram-1", Layers: domain.DiagramLayers{Intervals: true}, Playback: playback}, nil,
			intPtr(150), intPtr(165), nil, nil, nil)

		require.NoError(t, err)
		require.NotNil(t, item.DiagramRef)
		assert.Equal(t, playback, item.DiagramRef.Playback)
	})

	for _, tt := range []struct{ name, voiceID string }{
		{name: "a voice of another instrument family", voiceID: "piano"},
		{name: "a voice that does not exist", voiceID: "banjo"},
	} {
		t.Run("a diagram playing with "+tt.name+" is rejected", func(t *testing.T) {
			nodes := newFakeContentNodeRepository()
			nodes.put(videoNode("node-1"))
			diagrams := newFakeDiagramRepository()
			seedContentDiagram(t, diagrams, "diagram-1", "guitar")
			svc := newContentServiceWithDiagrams(nodes, newFakeExpandedContentRepository(), seededKnowledgeNodeRepository(), newFakeContentNodeVersionRepository(), diagrams)

			_, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
				domain.ExpandedContentTypeDiagram, nil, nil,
				&domain.DiagramRef{DiagramID: "diagram-1", Layers: domain.DiagramLayers{Intervals: true}, Playback: &domain.DiagramRefPlayback{Direction: domain.DiagramPlaybackDirectionAsAuthored, VoiceID: strPtr(tt.voiceID)}}, nil,
				intPtr(150), intPtr(165), nil, nil, nil)

			var valErr *domain.ValidationError
			require.True(t, errors.As(err, &valErr))
			assertHasField(t, valErr, "diagram_ref")
		})
	}

	for _, tt := range []struct {
		name, playbackID string
		wantErr          bool
	}{
		{name: "a diagram playing one of its playbacks is accepted", playbackID: "pb-arp"},
		{name: "a diagram playing a playback it doesn't have is rejected", playbackID: "pb-gone", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			nodes := newFakeContentNodeRepository()
			nodes.put(videoNode("node-1"))
			diagrams := newFakeDiagramRepository()
			seedContentDiagram(t, diagrams, "diagram-1", "guitar")
			givePlaybacks(t, diagrams, "diagram-1", "pb-strum", "pb-arp")
			svc := newContentServiceWithDiagrams(nodes, newFakeExpandedContentRepository(), seededKnowledgeNodeRepository(), newFakeContentNodeVersionRepository(), diagrams)
			playback := &domain.DiagramRefPlayback{Direction: domain.DiagramPlaybackDirectionAsAuthored, PlaybackID: strPtr(tt.playbackID)}

			item, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
				domain.ExpandedContentTypeDiagram, nil, nil,
				&domain.DiagramRef{DiagramID: "diagram-1", Layers: domain.DiagramLayers{Intervals: true}, Playback: playback}, nil,
				intPtr(150), intPtr(165), nil, nil, nil)

			if tt.wantErr {
				var valErr *domain.ValidationError
				require.True(t, errors.As(err, &valErr))
				assertHasField(t, valErr, "diagram_ref")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, playback, item.DiagramRef.Playback)
		})
	}

	t.Run("a stack entry's playback is ignored, since a stack doesn't play", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(articleNode("node-1"))
		diagrams := newFakeDiagramRepository()
		seedContentDiagram(t, diagrams, "diagram-1", "guitar")
		seedContentDiagram(t, diagrams, "diagram-2", "guitar")
		svc := newContentServiceWithDiagrams(nodes, newFakeExpandedContentRepository(), seededKnowledgeNodeRepository(), newFakeContentNodeVersionRepository(), diagrams)

		_, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeDiagram, nil, nil, nil,
			&domain.DiagramStackRef{Stack: []domain.DiagramRef{
				{DiagramID: "diagram-1", Layers: domain.DiagramLayers{Intervals: true}, Playback: &domain.DiagramRefPlayback{Direction: domain.DiagramPlaybackDirectionAsAuthored, VoiceID: strPtr("piano")}},
				{DiagramID: "diagram-2", Layers: domain.DiagramLayers{Intervals: true}},
			}},
			nil, nil, intPtr(3), intPtr(8000), nil)

		require.NoError(t, err)
	})

	t.Run("creating a diagram item that also carries a media URL is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		diagrams := newFakeDiagramRepository()
		seedContentDiagram(t, diagrams, "diagram-1", "guitar")
		svc := newContentServiceWithDiagrams(nodes, newFakeExpandedContentRepository(), seededKnowledgeNodeRepository(), newFakeContentNodeVersionRepository(), diagrams)

		_, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeDiagram, strPtr("https://cdn.example.com/img.png"), nil,
			&domain.DiagramRef{DiagramID: "diagram-1", Layers: domain.DiagramLayers{Intervals: true}}, nil,
			intPtr(150), intPtr(165), nil, nil, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "media_url")
	})
}

func richTextContent(text string) domain.PromptDocument {
	return domain.PromptDocument{
		Type: "doc",
		Content: []domain.PromptNode{
			{Type: domain.PromptNodeTypeParagraph, Content: []domain.PromptNode{
				{Type: domain.PromptNodeTypeText, Text: text},
			}},
		},
	}
}

func TestContentService_CreateExpandedContent_RichText(t *testing.T) {
	t.Run("a teacher adds rich text content to a video lesson at a specific timestamp", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())
		rich := richTextContent("thumb behind the neck")

		item, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeRichText, nil, &rich, nil, nil, intPtr(150), intPtr(165), nil, nil, nil)

		require.NoError(t, err)
		assert.Equal(t, domain.ExpandedContentTypeRichText, item.ContentType)
		require.NotNil(t, item.RichContent)
		assert.Equal(t, rich, *item.RichContent)
	})

	t.Run("a teacher adds rich text content to an article at a specific paragraph", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(articleNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())
		rich := richTextContent("standard tuning, low to high")

		item, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeRichText, nil, &rich, nil, nil, nil, nil, intPtr(2), intPtr(6000), nil)

		require.NoError(t, err)
		assert.Equal(t, domain.ExpandedContentTypeRichText, item.ContentType)
	})

	t.Run("rich text content may embed a video or audio node", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())
		src := "https://cdn.example.com/clip.mp4"
		rich := domain.PromptDocument{
			Type: "doc",
			Content: []domain.PromptNode{
				{Type: domain.PromptNodeTypeVideo, Attrs: &domain.PromptNodeAttrs{Src: &src}},
			},
		}

		item, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeRichText, nil, &rich, nil, nil, intPtr(150), intPtr(165), nil, nil, nil)

		require.NoError(t, err)
		require.NotNil(t, item.RichContent)
		require.Len(t, item.RichContent.Content, 1)
		assert.Equal(t, domain.PromptNodeTypeVideo, item.RichContent.Content[0].Type)
	})

	t.Run("creating a rich_text item without rich content is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		_, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeRichText, nil, nil, nil, nil, intPtr(150), intPtr(165), nil, nil, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "rich_content")
	})

	t.Run("creating an image item that also carries rich content is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())
		rich := richTextContent("x")

		_, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/img.png"), &rich, nil, nil, intPtr(150), intPtr(165), nil, nil, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "rich_content")
	})

	t.Run("creating a rich_text item that also carries a media URL is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())
		rich := richTextContent("x")

		_, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeRichText, strPtr("https://cdn.example.com/img.png"), &rich, nil, nil, intPtr(150), intPtr(165), nil, nil, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "media_url")
	})
}

func TestContentService_UpdateExpandedContent(t *testing.T) {
	t.Run("a teacher updates an expanded content item's timing and caption", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())
		created, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/img.png"), nil, nil, nil, intPtr(150), intPtr(165), nil, nil, nil)
		require.NoError(t, err)

		caption := "Updated timing"
		updated, err := svc.UpdateExpandedContent(context.Background(), teacherCaller(), created.ID,
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/img.png"), nil, nil, nil, intPtr(160), intPtr(180), nil, nil, &caption)

		require.NoError(t, err)
		require.NotNil(t, updated.TriggerAtSeconds)
		assert.Equal(t, 160, *updated.TriggerAtSeconds)
		require.NotNil(t, updated.HideAtSeconds)
		assert.Equal(t, 180, *updated.HideAtSeconds)
		require.NotNil(t, updated.Caption)
		assert.Equal(t, "Updated timing", *updated.Caption)
	})

	t.Run("a teacher updates an article item's paragraph and duration", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(articleNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())
		created, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/img.png"), nil, nil, nil, nil, nil, intPtr(1), intPtr(5000), nil)
		require.NoError(t, err)

		updated, err := svc.UpdateExpandedContent(context.Background(), teacherCaller(), created.ID,
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/img.png"), nil, nil, nil, nil, nil, intPtr(2), intPtr(6000), nil)

		require.NoError(t, err)
		require.NotNil(t, updated.TriggerAtParagraph)
		assert.Equal(t, 2, *updated.TriggerAtParagraph)
		require.NotNil(t, updated.DurationMS)
		assert.Equal(t, 6000, *updated.DurationMS)
	})

	t.Run("updating a video item with an inconsistent hide time is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())
		created, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/img.png"), nil, nil, nil, intPtr(150), intPtr(165), nil, nil, nil)
		require.NoError(t, err)

		_, err = svc.UpdateExpandedContent(context.Background(), teacherCaller(), created.ID,
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/img.png"), nil, nil, nil, intPtr(150), intPtr(150), nil, nil, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "hide_at_seconds")
	})

	t.Run("updating without a media URL is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())
		created, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/img.png"), nil, nil, nil, intPtr(150), intPtr(165), nil, nil, nil)
		require.NoError(t, err)

		_, err = svc.UpdateExpandedContent(context.Background(), teacherCaller(), created.ID,
			domain.ExpandedContentTypeImage, nil, nil, nil, nil, intPtr(150), intPtr(165), nil, nil, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "media_url")
	})

	t.Run("a student cannot update an expanded content item", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())
		created, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/img.png"), nil, nil, nil, intPtr(150), intPtr(165), nil, nil, nil)
		require.NoError(t, err)

		_, err = svc.UpdateExpandedContent(context.Background(), studentCaller(), created.ID,
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/img.png"), nil, nil, nil, intPtr(150), intPtr(165), nil, nil, nil)

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("a teacher cannot update another teacher's expanded content item", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1")) // owned by teacher-1
		svc := newContentService(nodes, newFakeExpandedContentRepository())
		created, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/img.png"), nil, nil, nil, intPtr(150), intPtr(165), nil, nil, nil)
		require.NoError(t, err)

		_, err = svc.UpdateExpandedContent(context.Background(), otherTeacherCaller(), created.ID,
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/img.png"), nil, nil, nil, intPtr(150), intPtr(165), nil, nil, nil)

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("updating an item that does not exist returns not found", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.UpdateExpandedContent(context.Background(), teacherCaller(), "missing",
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/img.png"), nil, nil, nil, intPtr(150), intPtr(165), nil, nil, nil)

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestContentService_DeleteExpandedContent(t *testing.T) {
	t.Run("a teacher deletes an expanded content item", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())
		created, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/img.png"), nil, nil, nil, intPtr(150), intPtr(165), nil, nil, nil)
		require.NoError(t, err)

		err = svc.DeleteExpandedContent(context.Background(), teacherCaller(), created.ID)
		require.NoError(t, err)

		_, err = svc.GetExpandedContent(context.Background(), created.ID)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a student cannot delete an expanded content item", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())
		created, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/img.png"), nil, nil, nil, intPtr(150), intPtr(165), nil, nil, nil)
		require.NoError(t, err)

		err = svc.DeleteExpandedContent(context.Background(), studentCaller(), created.ID)

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("a teacher cannot delete another teacher's expanded content item", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1")) // owned by teacher-1
		svc := newContentService(nodes, newFakeExpandedContentRepository())
		created, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/img.png"), nil, nil, nil, intPtr(150), intPtr(165), nil, nil, nil)
		require.NoError(t, err)

		err = svc.DeleteExpandedContent(context.Background(), otherTeacherCaller(), created.ID)

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("deleting an item that does not exist returns not found", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		err := svc.DeleteExpandedContent(context.Background(), teacherCaller(), "missing")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestContentService_ListExpandedContent(t *testing.T) {
	t.Run("listing expanded content for a video node returns items", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		expanded := newFakeExpandedContentRepository()
		svc := newContentService(nodes, expanded)

		_, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/a.png"), nil, nil, nil, intPtr(90), intPtr(100), nil, nil, nil)
		require.NoError(t, err)
		_, err = svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/b.png"), nil, nil, nil, intPtr(150), intPtr(160), nil, nil, nil)
		require.NoError(t, err)

		items, err := svc.ListExpandedContent(context.Background(), "node-1")

		require.NoError(t, err)
		assert.Len(t, items, 2)
	})

	t.Run("listing for a non-existent content node returns not found", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.ListExpandedContent(context.Background(), "missing")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestContentService_GetExpandedContent(t *testing.T) {
	t.Run("any authenticated user retrieves a specific expanded content item", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		expanded := newFakeExpandedContentRepository()
		svc := newContentService(nodes, expanded)

		created, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, strPtr("https://cdn.example.com/a.png"), nil, nil, nil, intPtr(90), intPtr(100), nil, nil, nil)
		require.NoError(t, err)

		got, err := svc.GetExpandedContent(context.Background(), created.ID)

		require.NoError(t, err)
		assert.Equal(t, created, got)
	})

	t.Run("retrieving an expanded content item that does not exist returns not found", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.GetExpandedContent(context.Background(), "missing")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestContentService_PublishContentNode(t *testing.T) {
	t.Run("a teacher publishes a content node's draft for the first time", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-01"))
		versions := newFakeContentNodeVersionRepository()
		svc := newContentServiceWithVersions(nodes, newFakeExpandedContentRepository(), seededKnowledgeNodeRepository(), versions)

		version, err := svc.PublishContentNode(context.Background(), teacherCaller(), "node-01")

		require.NoError(t, err)
		assert.Equal(t, 1, version.VersionNumber)
		assert.Equal(t, "node-01", version.ContentNodeID)

		latest, err := versions.GetLatestByContentNodeID(context.Background(), "node-01")
		require.NoError(t, err)
		assert.Equal(t, 1, latest.VersionNumber)
	})

	t.Run("an admin publishes a content node", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-01"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		version, err := svc.PublishContentNode(context.Background(), adminCaller(), "node-01")

		require.NoError(t, err)
		assert.Equal(t, 1, version.VersionNumber)
	})

	t.Run("publishing a content node again creates a new version", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-01"))
		versions := newFakeContentNodeVersionRepository()
		svc := newContentServiceWithVersions(nodes, newFakeExpandedContentRepository(), seededKnowledgeNodeRepository(), versions)

		_, err := svc.PublishContentNode(context.Background(), teacherCaller(), "node-01")
		require.NoError(t, err)

		edited := nodes.byID["node-01"]
		edited.Title = "Revised title"
		nodes.byID["node-01"] = edited

		second, err := svc.PublishContentNode(context.Background(), teacherCaller(), "node-01")
		require.NoError(t, err)
		assert.Equal(t, 2, second.VersionNumber)
		assert.Equal(t, "Revised title", second.Title)
	})

	t.Run("a student cannot publish a content node", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-01"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		_, err := svc.PublishContentNode(context.Background(), studentCaller(), "node-01")

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("a teacher who did not create the content node, and is not an admin, cannot publish it", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-02")) // owned by teacher-1
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		_, err := svc.PublishContentNode(context.Background(), otherTeacherCaller(), "node-02")

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("publishing a content node that does not exist returns not found", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.PublishContentNode(context.Background(), teacherCaller(), "missing")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func assertHasField(t *testing.T, valErr *domain.ValidationError, field string) {
	t.Helper()
	for _, f := range valErr.Fields {
		if f.Field == field {
			return
		}
	}
	t.Fatalf("expected field %q among validation errors, got %+v", field, valErr.Fields)
}

func TestContentService_ListContentNodeVersions(t *testing.T) {
	publishTwice := func(t *testing.T, svc *application.ContentService) {
		t.Helper()
		_, err := svc.PublishContentNode(context.Background(), teacherCaller(), "node-01")
		require.NoError(t, err)
		_, err = svc.PublishContentNode(context.Background(), teacherCaller(), "node-01")
		require.NoError(t, err)
	}

	t.Run("the creating teacher lists versions newest first", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-01"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())
		publishTwice(t, svc)

		got, err := svc.ListContentNodeVersions(context.Background(), teacherCaller(), "node-01")

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, 2, got[0].VersionNumber)
		assert.Equal(t, 1, got[1].VersionNumber)
	})

	t.Run("an admin lists a node's versions", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-01"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())
		publishTwice(t, svc)

		got, err := svc.ListContentNodeVersions(context.Background(), adminCaller(), "node-01")

		require.NoError(t, err)
		assert.Len(t, got, 2)
	})

	t.Run("a version published before snapshots were stored falls back to the node's current classification and languages", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		node := videoNode("node-01")
		node.Classification = domain.Classification{
			Skills:          []domain.KnowledgeNode{{ID: "skill-1", Kind: domain.KnowledgeNodeKindSkill, Key: "fingerpicking"}},
			DifficultyLevel: domain.DifficultyLevelIntermediate,
			ReviewState:     domain.ReviewStateConfirmed,
		}
		node.Languages = []domain.Language{{Code: "en", Name: "English"}}
		nodes.put(node)
		versions := newFakeContentNodeVersionRepository()
		require.NoError(t, versions.Create(context.Background(), domain.ContentNodeVersion{
			ID: "legacy", ContentNodeID: "node-01", VersionNumber: 1, Title: "Old title", ContentType: domain.ContentTypeVideo,
		}))
		svc := newContentServiceWithVersions(nodes, newFakeExpandedContentRepository(), seededKnowledgeNodeRepository(), versions)

		got, err := svc.ListContentNodeVersions(context.Background(), teacherCaller(), "node-01")

		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "Old title", got[0].Title)
		assert.Equal(t, node.Classification, got[0].Classification)
		assert.Equal(t, node.Languages, got[0].Languages)
	})

	t.Run("a version with its own snapshot keeps it", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		node := videoNode("node-01")
		node.Classification = domain.Classification{DifficultyLevel: domain.DifficultyLevelExpert, ReviewState: domain.ReviewStateConfirmed}
		nodes.put(node)
		versions := newFakeContentNodeVersionRepository()
		snapshot := domain.Classification{DifficultyLevel: domain.DifficultyLevelBeginner, ReviewState: domain.ReviewStatePending}
		require.NoError(t, versions.Create(context.Background(), domain.ContentNodeVersion{
			ID: "v1", ContentNodeID: "node-01", VersionNumber: 1, Classification: snapshot,
		}))
		svc := newContentServiceWithVersions(nodes, newFakeExpandedContentRepository(), seededKnowledgeNodeRepository(), versions)

		got, err := svc.ListContentNodeVersions(context.Background(), teacherCaller(), "node-01")

		require.NoError(t, err)
		assert.Equal(t, snapshot, got[0].Classification)
	})

	t.Run("a never-published node has an empty history", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-01"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		got, err := svc.ListContentNodeVersions(context.Background(), teacherCaller(), "node-01")

		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("a teacher who did not create the node is forbidden", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-01"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		_, err := svc.ListContentNodeVersions(context.Background(), otherTeacherCaller(), "node-01")

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("a student is forbidden", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-01"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		_, err := svc.ListContentNodeVersions(context.Background(), studentCaller(), "node-01")

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("a node that does not exist is not found", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.ListContentNodeVersions(context.Background(), teacherCaller(), "missing")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}
