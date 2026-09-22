//go:build integration

package repo

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/domain"
)

var fixedAt = time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC).Truncate(time.Microsecond)

func strPtr(s string) *string { return &s }

// seedSkill/seedConcept create a real Skill/Concept row via the ent
// repository — ContentNode/Exercise's skill_ids/concept_ids reference these
// by a real foreign key, so integration tests need actual rows to link
// against, not just an arbitrary uuid.
func seedSkill(t *testing.T, ctx context.Context, client *ent.Client, name string) domain.Skill {
	t.Helper()
	skill := domain.Skill{ID: uuid.NewString(), Name: name}
	require.NoError(t, NewEntSkillRepository(client).Create(ctx, skill))
	return skill
}

func seedConcept(t *testing.T, ctx context.Context, client *ent.Client, name string) domain.Concept {
	t.Helper()
	concept := domain.Concept{ID: uuid.NewString(), Name: name}
	require.NoError(t, NewEntConceptRepository(client).Create(ctx, concept))
	return concept
}

func TestEntUserRepository_CreateAndGet(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	repo := NewEntUserRepository(client)

	user := domain.User{ID: uuid.NewString(), ClerkUserID: "clerk-alice", Role: domain.RoleStudent, Locale: domain.Language{Code: "en", Name: "English"}, RegisteredAt: fixedAt}
	require.NoError(t, repo.Create(ctx, user))

	byClerk, err := repo.GetByClerkUserID(ctx, "clerk-alice")
	require.NoError(t, err)
	assert.Equal(t, user, byClerk)

	byID, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, user, byID)

	err = repo.Create(ctx, domain.User{ID: uuid.NewString(), ClerkUserID: "clerk-alice", Role: domain.RoleTeacher, Locale: domain.Language{Code: "en"}, RegisteredAt: fixedAt})
	assert.ErrorIs(t, err, domain.ErrAlreadyExists)

	_, err = repo.GetByID(ctx, uuid.NewString())
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestEntUserRepository_Create_UnknownLocaleRejected(t *testing.T) {
	repo := NewEntUserRepository(setupPostgres(t))

	err := repo.Create(context.Background(), domain.User{ID: uuid.NewString(), ClerkUserID: "clerk-bob", Role: domain.RoleStudent, Locale: domain.Language{Code: "xx"}, RegisteredAt: fixedAt})

	var valErr *domain.ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "locale", valErr.Fields[0].Field)
}

func TestEntUserRepository_UpdateLocale(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	repo := NewEntUserRepository(client)

	user := domain.User{ID: uuid.NewString(), ClerkUserID: "clerk-carol", Role: domain.RoleStudent, Locale: domain.Language{Code: "en"}, RegisteredAt: fixedAt}
	require.NoError(t, repo.Create(ctx, user))

	require.NoError(t, repo.UpdateLocale(ctx, user.ID, "pt_BR"))

	got, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.Language{Code: "pt_BR", Name: "Portuguese (Brazil)"}, got.Locale)

	err = repo.UpdateLocale(ctx, uuid.NewString(), "en")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestEntContentNodeRepository_CreateAndGet(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	repo := NewEntContentNodeRepository(client)

	skill := seedSkill(t, ctx, client, "triad-shapes-"+uuid.NewString())
	concept := seedConcept(t, ctx, client, "chord-theory-"+uuid.NewString())

	teacherID := uuid.NewString()
	node := domain.ContentNode{
		ID: uuid.NewString(), TeacherID: teacherID, Title: "Intro", ContentType: domain.ContentTypeVideo,
		Classification: domain.Classification{Skills: []domain.Skill{skill}, Concepts: []domain.Concept{concept}, DifficultyLevel: domain.DifficultyLevelBeginner, ReviewState: domain.ReviewStatePending},
		Languages:      []domain.Language{{Code: "en"}, {Code: "pt_BR"}},
		CreatedAt:      fixedAt,
	}
	require.NoError(t, repo.Create(ctx, node))

	got, err := repo.GetByID(ctx, node.ID)
	require.NoError(t, err)
	node.Languages = []domain.Language{{Code: "en", Name: "English"}, {Code: "pt_BR", Name: "Portuguese (Brazil)"}}
	assert.ElementsMatch(t, node.Languages, got.Languages)
	got.Languages = node.Languages
	assert.Equal(t, node, got)

	node2 := node
	node2.ID = uuid.NewString()
	node2.Languages = []domain.Language{{Code: "any"}}
	require.NoError(t, repo.Create(ctx, node2))

	byIDs, err := repo.GetByIDs(ctx, []string{node.ID, node2.ID, uuid.NewString()})
	require.NoError(t, err)
	assert.Len(t, byIDs, 2)
	assert.Contains(t, byIDs, node.ID)
	assert.Contains(t, byIDs, node2.ID)
	assert.Equal(t, []domain.Language{{Code: "any", Name: "Language-agnostic"}}, byIDs[node2.ID].Languages)
}

// TestEntContentNodeRepository_Update_ReplacesLanguages confirms Update fully
// replaces the node's language tags rather than ignoring them or merging with
// the old set — the same complete-replacement contract it already gives skills
// and concepts. A node created with no languages (older data) must be able to
// gain them, since a node with none is locked for every student who has not
// already completed it.
func TestEntContentNodeRepository_Update_ReplacesLanguages(t *testing.T) {
	tests := []struct {
		name     string
		existing []domain.Language
		updated  []domain.Language
		want     []domain.Language
	}{
		{
			name:     "replaces the existing languages",
			existing: []domain.Language{{Code: "en"}},
			updated:  []domain.Language{{Code: "pt_BR"}},
			want:     []domain.Language{{Code: "pt_BR", Name: "Portuguese (Brazil)"}},
		},
		{
			name:     "adds languages to a node that has none",
			existing: nil,
			updated:  []domain.Language{{Code: "any"}},
			want:     []domain.Language{{Code: "any", Name: "Language-agnostic"}},
		},
		{
			name:     "keeps the languages that are still listed and drops the rest",
			existing: []domain.Language{{Code: "en"}, {Code: "pt_BR"}},
			updated:  []domain.Language{{Code: "en"}},
			want:     []domain.Language{{Code: "en", Name: "English"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := setupPostgres(t)
			repo := NewEntContentNodeRepository(client)

			node := domain.ContentNode{
				ID: uuid.NewString(), TeacherID: uuid.NewString(), Title: "Node " + uuid.NewString(), ContentType: domain.ContentTypeVideo,
				Classification: domain.Classification{
					Skills:          []domain.Skill{seedSkill(t, ctx, client, "skill-"+uuid.NewString())},
					Concepts:        []domain.Concept{seedConcept(t, ctx, client, "concept-"+uuid.NewString())},
					DifficultyLevel: domain.DifficultyLevelBeginner, ReviewState: domain.ReviewStatePending,
				},
				Languages: tt.existing,
				CreatedAt: fixedAt,
			}
			require.NoError(t, repo.Create(ctx, node))

			node.Languages = tt.updated
			require.NoError(t, repo.Update(ctx, node))

			got, err := repo.GetByID(ctx, node.ID)
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.want, got.Languages)
		})
	}
}

func TestEntContentNodeRepository_PersistsAndUpdatesBody(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	repo := NewEntContentNodeRepository(client)

	video := seedContentNode(t, ctx, repo)
	video.MediaURL = strPtr("https://cdn.motifpath.io/videos/triad-shapes-intro.mp4")
	require.NoError(t, repo.Update(ctx, video))

	gotVideo, err := repo.GetByID(ctx, video.ID)
	require.NoError(t, err)
	assert.Equal(t, video.MediaURL, gotVideo.MediaURL)
	assert.Nil(t, gotVideo.RichContent)

	body := domain.NewPlainTextPrompt("Chord theory explains how notes combine into triads.")
	article := seedContentNode(t, ctx, repo)
	article.ContentType = domain.ContentTypeArticle
	article.RichContent = &body
	articleID := uuid.NewString()
	article.ID = articleID
	require.NoError(t, repo.Create(ctx, article))

	gotArticle, err := repo.GetByID(ctx, articleID)
	require.NoError(t, err)
	assert.Nil(t, gotArticle.MediaURL)
	require.NotNil(t, gotArticle.RichContent)
	assert.Equal(t, body, *gotArticle.RichContent)

	revised := domain.NewPlainTextPrompt("Revised body")
	article.RichContent = &revised
	require.NoError(t, repo.Update(ctx, article))
	gotRevised, err := repo.GetByID(ctx, articleID)
	require.NoError(t, err)
	require.NotNil(t, gotRevised.RichContent)
	assert.Equal(t, revised, *gotRevised.RichContent)

	listed, err := repo.GetByIDs(ctx, []string{video.ID, articleID})
	require.NoError(t, err)
	assert.Equal(t, video.MediaURL, listed[video.ID].MediaURL)
	assert.Equal(t, revised, *listed[articleID].RichContent)
}

func TestEntLanguageRepository_GetByCode(t *testing.T) {
	repo := NewEntLanguageRepository(setupPostgres(t))
	ctx := context.Background()

	lang, err := repo.GetByCode(ctx, "pt_BR")
	require.NoError(t, err)
	assert.Equal(t, domain.Language{Code: "pt_BR", Name: "Portuguese (Brazil)"}, lang)

	_, err = repo.GetByCode(ctx, "xx")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestEntChallengeRepository_CreateAndGet(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	nodeRepo := NewEntContentNodeRepository(client)
	repo := NewEntChallengeRepository(client)

	node := seedContentNode(t, ctx, nodeRepo)
	timeThreshold := 120000

	challenge := domain.Challenge{
		ID: uuid.NewString(), ContentNodeID: node.ID, SubjectSkillID: strPtr(uuid.NewString()), PassThreshold: 70,
		TimeThresholdMS: &timeThreshold, CreatedAt: fixedAt,
	}
	require.NoError(t, repo.Create(ctx, challenge))

	got, err := repo.GetByID(ctx, challenge.ID)
	require.NoError(t, err)
	assert.Equal(t, challenge, got)
}

func TestEntChallengeRepository_CreateWithShufflingAndListByContentNodeID(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	nodeRepo := NewEntContentNodeRepository(client)
	repo := NewEntChallengeRepository(client)

	node := seedContentNode(t, ctx, nodeRepo)
	otherNode := seedContentNode(t, ctx, nodeRepo)

	shuffled := domain.Challenge{
		ID: uuid.NewString(), ContentNodeID: node.ID, SubjectSkillID: strPtr(uuid.NewString()), PassThreshold: 70,
		ShuffleExercises: true, ShuffleOptions: true, CreatedAt: fixedAt,
	}
	require.NoError(t, repo.Create(ctx, shuffled))
	require.NoError(t, repo.Create(ctx, domain.Challenge{
		ID: uuid.NewString(), ContentNodeID: otherNode.ID, SubjectSkillID: strPtr(uuid.NewString()), PassThreshold: 70, CreatedAt: fixedAt,
	}))

	got, err := repo.GetByID(ctx, shuffled.ID)
	require.NoError(t, err)
	assert.True(t, got.ShuffleExercises)
	assert.True(t, got.ShuffleOptions)

	list, err := repo.ListByContentNodeID(ctx, node.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, shuffled.ID, list[0].ID)

	empty, err := repo.ListByContentNodeID(ctx, uuid.NewString())
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestEntExerciseRepository_CreateAndGet(t *testing.T) {
	ctx := context.Background()
	imageURL := "https://cdn.example.com/fretboard/c-major-triad.png"
	duration := 45
	client := setupPostgres(t)
	repo := NewEntExerciseRepository(client)
	skill := seedSkill(t, ctx, client, "triad-shapes-"+uuid.NewString())
	concept := seedConcept(t, ctx, client, "chord-theory-"+uuid.NewString())

	exercise := domain.Exercise{
		ID: uuid.NewString(), Title: "Root position of a C major triad", Prompt: domain.NewPlainTextPrompt("Identify the chord"),
		ExerciseType: domain.ExerciseTypeImageRecognition, Skills: []domain.Skill{skill}, Concepts: []domain.Concept{concept}, ImageURL: &imageURL,
		EstimatedDurationSeconds: &duration,
		Options: []domain.Option{
			{ID: uuid.NewString(), IsCorrect: true, Region: &domain.OptionRegion{X: 0.2, Y: 0.3, Width: 0.1, Height: 0.1, Shape: domain.OptionRegionShapeRectangle}},
			{ID: uuid.NewString(), IsCorrect: false, Region: &domain.OptionRegion{X: 0.5, Y: 0.3, Width: 0.1, Height: 0.1, Shape: domain.OptionRegionShapeRectangle}},
		},
		ChallengeIDs:   []string{},
		ContentNodeIDs: []string{},
		Languages:      []domain.Language{{Code: "any"}},
		CreatedAt:      fixedAt,
	}
	require.NoError(t, repo.Create(ctx, exercise))

	got, err := repo.GetByID(ctx, exercise.ID)
	require.NoError(t, err)
	assert.Equal(t, exercise.Title, got.Title)
	assert.Equal(t, exercise.ExerciseType, got.ExerciseType)
	assert.Equal(t, exercise.Skills, got.Skills)
	assert.Equal(t, exercise.Concepts, got.Concepts)
	assert.Equal(t, exercise.EstimatedDurationSeconds, got.EstimatedDurationSeconds)
	assert.ElementsMatch(t, exercise.Options, got.Options)
	assert.Empty(t, got.ChallengeIDs)
	assert.Empty(t, got.ContentNodeIDs)
	assert.Equal(t, []domain.Language{{Code: "any", Name: "Language-agnostic"}}, got.Languages)
}

// TestEntExerciseRepository_Update_ReplacesLanguages confirms Update fully
// replaces the exercise's language tags rather than merging with the old
// set — the same complete-replacement contract Update already gives Options.
func TestEntExerciseRepository_Update_ReplacesLanguages(t *testing.T) {
	ctx := context.Background()
	repo := NewEntExerciseRepository(setupPostgres(t))
	label := "A major"

	exercise := domain.Exercise{
		ID: uuid.NewString(), Title: "Name the chord", Prompt: domain.NewPlainTextPrompt("Name this chord"),
		ExerciseType: domain.ExerciseTypeTextResponse,
		Options:      []domain.Option{{ID: uuid.NewString(), IsCorrect: true, Label: &label}},
		ChallengeIDs: []string{}, ContentNodeIDs: []string{},
		Languages: []domain.Language{{Code: "en"}},
		CreatedAt: fixedAt,
	}
	require.NoError(t, repo.Create(ctx, exercise))

	exercise.Languages = []domain.Language{{Code: "pt_BR"}}
	require.NoError(t, repo.Update(ctx, exercise))

	got, err := repo.GetByID(ctx, exercise.ID)
	require.NoError(t, err)
	assert.Equal(t, []domain.Language{{Code: "pt_BR", Name: "Portuguese (Brazil)"}}, got.Languages)
}

// TestEntExerciseRepository_LegacyPlainTextPromptShim writes a prompt column
// value directly, bypassing Create's JSON marshaling — the shape a row
// written before prompts became structured documents would still have.
// GetByID must still succeed, wrapping that plain text as a single-paragraph
// document instead of failing to parse it as JSON.
func TestEntExerciseRepository_LegacyPlainTextPromptShim(t *testing.T) {
	ctx := context.Background()
	client := setupPostgres(t)
	repo := NewEntExerciseRepository(client)

	id := uuid.New()
	const legacyPrompt = "Identify the root position of a C major triad"
	require.NoError(t, client.Exercise.Create().
		SetID(id).
		SetTitle("Root position of a C major triad").
		SetPrompt(legacyPrompt).
		SetExerciseType("text_response").
		SetCreatedAt(fixedAt).
		Exec(ctx))

	got, err := repo.GetByID(ctx, id.String())
	require.NoError(t, err)
	assert.Equal(t, domain.NewPlainTextPrompt(legacyPrompt), got.Prompt)
}

func TestEntExerciseRepository_ListByChallengeID(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	nodeRepo := NewEntContentNodeRepository(client)
	challengeRepo := NewEntChallengeRepository(client)
	repo := NewEntExerciseRepository(client)

	node := seedContentNode(t, ctx, nodeRepo)
	challenge := domain.Challenge{ID: uuid.NewString(), ContentNodeID: node.ID, SubjectSkillID: strPtr(uuid.NewString()), PassThreshold: 70, CreatedAt: fixedAt}
	require.NoError(t, challengeRepo.Create(ctx, challenge))
	otherChallenge := domain.Challenge{ID: uuid.NewString(), ContentNodeID: node.ID, SubjectSkillID: strPtr(uuid.NewString()), PassThreshold: 70, CreatedAt: fixedAt}
	require.NoError(t, challengeRepo.Create(ctx, otherChallenge))

	label := "A major"
	linked := domain.Exercise{ID: uuid.NewString(), Title: "Name the chord", Prompt: domain.NewPlainTextPrompt("Name this chord"), ExerciseType: domain.ExerciseTypeTextResponse, Options: []domain.Option{{ID: uuid.NewString(), IsCorrect: true, Label: &label}}, ChallengeIDs: []string{}, CreatedAt: fixedAt}
	require.NoError(t, repo.Create(ctx, linked))
	unlinked := domain.Exercise{ID: uuid.NewString(), Title: "Name another chord", Prompt: domain.NewPlainTextPrompt("Name this other chord"), ExerciseType: domain.ExerciseTypeTextResponse, Options: []domain.Option{{ID: uuid.NewString(), IsCorrect: true, Label: &label}}, ChallengeIDs: []string{}, CreatedAt: fixedAt}
	require.NoError(t, repo.Create(ctx, unlinked))
	require.NoError(t, repo.LinkChallenge(ctx, linked.ID, challenge.ID))
	require.NoError(t, repo.LinkChallenge(ctx, unlinked.ID, otherChallenge.ID))

	list, err := repo.ListByChallengeID(ctx, challenge.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, linked.ID, list[0].ID)

	empty, err := repo.ListByChallengeID(ctx, uuid.NewString())
	require.NoError(t, err)
	assert.Empty(t, empty)
}

// TestEntExerciseRepository_ListByChallengeIDs confirms the batched lookup
// groups each linked exercise under the right challenge id, leaves a
// challenge with no links absent from the result, and ignores unknown ids
// passed alongside real ones.
func TestEntExerciseRepository_ListByChallengeIDs(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	nodeRepo := NewEntContentNodeRepository(client)
	challengeRepo := NewEntChallengeRepository(client)
	repo := NewEntExerciseRepository(client)

	node := seedContentNode(t, ctx, nodeRepo)
	challengeA := domain.Challenge{ID: uuid.NewString(), ContentNodeID: node.ID, SubjectSkillID: strPtr(uuid.NewString()), PassThreshold: 70, CreatedAt: fixedAt}
	require.NoError(t, challengeRepo.Create(ctx, challengeA))
	challengeB := domain.Challenge{ID: uuid.NewString(), ContentNodeID: node.ID, SubjectSkillID: strPtr(uuid.NewString()), PassThreshold: 70, CreatedAt: fixedAt}
	require.NoError(t, challengeRepo.Create(ctx, challengeB))
	challengeC := domain.Challenge{ID: uuid.NewString(), ContentNodeID: node.ID, SubjectSkillID: strPtr(uuid.NewString()), PassThreshold: 70, CreatedAt: fixedAt}
	require.NoError(t, challengeRepo.Create(ctx, challengeC))

	label := "A major"
	exA := domain.Exercise{ID: uuid.NewString(), Title: "Ex A", Prompt: domain.NewPlainTextPrompt("p"), ExerciseType: domain.ExerciseTypeTextResponse, Options: []domain.Option{{ID: uuid.NewString(), IsCorrect: true, Label: &label}}, ChallengeIDs: []string{}, CreatedAt: fixedAt}
	require.NoError(t, repo.Create(ctx, exA))
	exB := domain.Exercise{ID: uuid.NewString(), Title: "Ex B", Prompt: domain.NewPlainTextPrompt("p"), ExerciseType: domain.ExerciseTypeTextResponse, Options: []domain.Option{{ID: uuid.NewString(), IsCorrect: true, Label: &label}}, ChallengeIDs: []string{}, CreatedAt: fixedAt}
	require.NoError(t, repo.Create(ctx, exB))
	require.NoError(t, repo.LinkChallenge(ctx, exA.ID, challengeA.ID))
	require.NoError(t, repo.LinkChallenge(ctx, exB.ID, challengeB.ID))

	byChallenge, err := repo.ListByChallengeIDs(ctx, []string{challengeA.ID, challengeB.ID, challengeC.ID, uuid.NewString()})
	require.NoError(t, err)
	require.Len(t, byChallenge, 2)
	require.Len(t, byChallenge[challengeA.ID], 1)
	assert.Equal(t, exA.ID, byChallenge[challengeA.ID][0].ID)
	require.Len(t, byChallenge[challengeB.ID], 1)
	assert.Equal(t, exB.ID, byChallenge[challengeB.ID][0].ID)
	assert.NotContains(t, byChallenge, challengeC.ID)

	empty, err := repo.ListByChallengeIDs(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

// TestEntExerciseRepository_ListByChallengeID_PreservesLinkOrder guards
// against relying on incidental Postgres row order: an implicit ent
// many-to-many join table carries no sequence column, so a plain
// "WHERE challenge_id = ..." SELECT has no ordering guarantee at all. This
// links enough exercises, in a deliberately non-ID-sorted order, that a
// query without a real ORDER BY would very likely (though not
// deterministically) return them out of order.
func TestEntExerciseRepository_ListByChallengeID_PreservesLinkOrder(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	nodeRepo := NewEntContentNodeRepository(client)
	challengeRepo := NewEntChallengeRepository(client)
	repo := NewEntExerciseRepository(client)

	node := seedContentNode(t, ctx, nodeRepo)
	challenge := domain.Challenge{ID: uuid.NewString(), ContentNodeID: node.ID, SubjectSkillID: strPtr(uuid.NewString()), PassThreshold: 70, CreatedAt: fixedAt}
	require.NoError(t, challengeRepo.Create(ctx, challenge))

	label := "A major"
	const linkCount = 8
	linkedIDs := make([]string, linkCount)
	for i := range linkCount {
		ex := domain.Exercise{ID: uuid.NewString(), Title: "Exercise", Prompt: domain.NewPlainTextPrompt("Prompt"), ExerciseType: domain.ExerciseTypeTextResponse, Options: []domain.Option{{ID: uuid.NewString(), IsCorrect: true, Label: &label}}, ChallengeIDs: []string{}, CreatedAt: fixedAt}
		require.NoError(t, repo.Create(ctx, ex))
		linkedIDs[i] = ex.ID
	}
	// Link in reverse-ID order so link order and ID order disagree —
	// otherwise an accidental "ORDER BY id" fallback in Postgres could mask
	// a missing real ORDER BY.
	for i := linkCount - 1; i >= 0; i-- {
		require.NoError(t, repo.LinkChallenge(ctx, linkedIDs[i], challenge.ID))
	}
	wantOrder := make([]string, linkCount)
	for i, id := range linkedIDs {
		wantOrder[linkCount-1-i] = id
	}

	list, err := repo.ListByChallengeID(ctx, challenge.ID)
	require.NoError(t, err)
	require.Len(t, list, linkCount)
	gotOrder := make([]string, linkCount)
	for i, ex := range list {
		gotOrder[i] = ex.ID
	}
	assert.Equal(t, wantOrder, gotOrder)
}

func TestEntExerciseRepository_LinkAndUnlinkContentNode(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	nodeRepo := NewEntContentNodeRepository(client)
	repo := NewEntExerciseRepository(client)

	nodeA := seedContentNode(t, ctx, nodeRepo)
	nodeB := seedContentNode(t, ctx, nodeRepo)

	label := "A major"
	exercise := domain.Exercise{ID: uuid.NewString(), Title: "Name the chord", Prompt: domain.NewPlainTextPrompt("Name this chord"), ExerciseType: domain.ExerciseTypeTextResponse, Options: []domain.Option{{ID: uuid.NewString(), IsCorrect: true, Label: &label}}, ChallengeIDs: []string{}, CreatedAt: fixedAt}
	require.NoError(t, repo.Create(ctx, exercise))

	require.NoError(t, repo.LinkContentNode(ctx, exercise.ID, nodeA.ID))
	require.NoError(t, repo.LinkContentNode(ctx, exercise.ID, nodeB.ID))

	got, err := repo.GetByID(ctx, exercise.ID)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{nodeA.ID, nodeB.ID}, got.ContentNodeIDs)

	listA, err := repo.ListByContentNodeID(ctx, nodeA.ID)
	require.NoError(t, err)
	require.Len(t, listA, 1)
	assert.Equal(t, exercise.ID, listA[0].ID)

	require.NoError(t, repo.UnlinkContentNode(ctx, exercise.ID, nodeA.ID))

	got, err = repo.GetByID(ctx, exercise.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{nodeB.ID}, got.ContentNodeIDs)
}

// TestEntExerciseRepository_ListByContentNodeID_PreservesLinkOrder mirrors
// TestEntExerciseRepository_ListByChallengeID_PreservesLinkOrder for path
// exercises — a teacher-authored sequence, so link order must be a real
// queried column, not incidental Postgres row order.
func TestEntExerciseRepository_ListByContentNodeID_PreservesLinkOrder(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	nodeRepo := NewEntContentNodeRepository(client)
	repo := NewEntExerciseRepository(client)

	node := seedContentNode(t, ctx, nodeRepo)

	label := "A major"
	const linkCount = 8
	linkedIDs := make([]string, linkCount)
	for i := range linkCount {
		ex := domain.Exercise{ID: uuid.NewString(), Title: "Exercise", Prompt: domain.NewPlainTextPrompt("Prompt"), ExerciseType: domain.ExerciseTypeTextResponse, Options: []domain.Option{{ID: uuid.NewString(), IsCorrect: true, Label: &label}}, ChallengeIDs: []string{}, CreatedAt: fixedAt}
		require.NoError(t, repo.Create(ctx, ex))
		linkedIDs[i] = ex.ID
	}
	for i := linkCount - 1; i >= 0; i-- {
		require.NoError(t, repo.LinkContentNode(ctx, linkedIDs[i], node.ID))
	}
	wantOrder := make([]string, linkCount)
	for i, id := range linkedIDs {
		wantOrder[linkCount-1-i] = id
	}

	list, err := repo.ListByContentNodeID(ctx, node.ID)
	require.NoError(t, err)
	require.Len(t, list, linkCount)
	gotOrder := make([]string, linkCount)
	for i, ex := range list {
		gotOrder[i] = ex.ID
	}
	assert.Equal(t, wantOrder, gotOrder)
}

func TestEntExerciseRepository_ListBySkillID(t *testing.T) {
	ctx := context.Background()
	client := setupPostgres(t)
	repo := NewEntExerciseRepository(client)
	concept := seedConcept(t, ctx, client, "concept-"+uuid.NewString())

	skillA := seedSkill(t, ctx, client, "alternate_picking-"+uuid.NewString())
	skillB := seedSkill(t, ctx, client, "hybrid_picking-"+uuid.NewString())

	label := "A major"
	linked := domain.Exercise{ID: uuid.NewString(), Title: "Linked", Prompt: domain.NewPlainTextPrompt("Prompt"), ExerciseType: domain.ExerciseTypeTextResponse, Skills: []domain.Skill{skillA}, Concepts: []domain.Concept{concept}, Options: []domain.Option{{ID: uuid.NewString(), IsCorrect: true, Label: &label}}, ChallengeIDs: []string{}, CreatedAt: fixedAt}
	require.NoError(t, repo.Create(ctx, linked))
	unlinked := domain.Exercise{ID: uuid.NewString(), Title: "Unlinked", Prompt: domain.NewPlainTextPrompt("Prompt"), ExerciseType: domain.ExerciseTypeTextResponse, Skills: []domain.Skill{skillB}, Concepts: []domain.Concept{concept}, Options: []domain.Option{{ID: uuid.NewString(), IsCorrect: true, Label: &label}}, ChallengeIDs: []string{}, CreatedAt: fixedAt}
	require.NoError(t, repo.Create(ctx, unlinked))

	list, err := repo.ListBySkillID(ctx, skillA.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, linked.ID, list[0].ID)

	empty, err := repo.ListBySkillID(ctx, uuid.NewString())
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestEntExerciseRepository_LinkAndUnlinkChallenge(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	nodeRepo := NewEntContentNodeRepository(client)
	challengeRepo := NewEntChallengeRepository(client)
	repo := NewEntExerciseRepository(client)

	node := seedContentNode(t, ctx, nodeRepo)
	challengeA := domain.Challenge{ID: uuid.NewString(), ContentNodeID: node.ID, SubjectSkillID: strPtr(uuid.NewString()), PassThreshold: 70, CreatedAt: fixedAt}
	require.NoError(t, challengeRepo.Create(ctx, challengeA))
	challengeB := domain.Challenge{ID: uuid.NewString(), ContentNodeID: node.ID, SubjectSkillID: strPtr(uuid.NewString()), PassThreshold: 70, CreatedAt: fixedAt}
	require.NoError(t, challengeRepo.Create(ctx, challengeB))

	label := "A major"
	exercise := domain.Exercise{
		ID: uuid.NewString(), Title: "Name the chord", Prompt: domain.NewPlainTextPrompt("Name this chord"),
		ExerciseType: domain.ExerciseTypeTextResponse,
		Options:      []domain.Option{{ID: uuid.NewString(), IsCorrect: true, Label: &label}},
		ChallengeIDs: []string{},
		CreatedAt:    fixedAt,
	}
	require.NoError(t, repo.Create(ctx, exercise))

	require.NoError(t, repo.LinkChallenge(ctx, exercise.ID, challengeA.ID))
	require.NoError(t, repo.LinkChallenge(ctx, exercise.ID, challengeB.ID))

	got, err := repo.GetByID(ctx, exercise.ID)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{challengeA.ID, challengeB.ID}, got.ChallengeIDs)

	require.NoError(t, repo.UnlinkChallenge(ctx, exercise.ID, challengeA.ID))

	got, err = repo.GetByID(ctx, exercise.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{challengeB.ID}, got.ChallengeIDs)
}

func TestEntExpandedContentRepository_CreateGetAndList(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	nodeRepo := NewEntContentNodeRepository(client)
	repo := NewEntExpandedContentRepository(client)

	node := seedContentNode(t, ctx, nodeRepo)

	first, second, third := 210, 90, 150
	hideFirst, hideSecond, hideThird := first+10, second+10, third+10
	items := []domain.ExpandedContent{
		{ID: uuid.NewString(), ContentNodeID: node.ID, ContentType: domain.ExpandedContentTypeImage, MediaURL: strPtr("https://cdn/a.png"), TriggerAtSeconds: &first, HideAtSeconds: &hideFirst, CreatedAt: fixedAt},
		{ID: uuid.NewString(), ContentNodeID: node.ID, ContentType: domain.ExpandedContentTypeImage, MediaURL: strPtr("https://cdn/b.png"), TriggerAtSeconds: &second, HideAtSeconds: &hideSecond, CreatedAt: fixedAt},
		{ID: uuid.NewString(), ContentNodeID: node.ID, ContentType: domain.ExpandedContentTypeImage, MediaURL: strPtr("https://cdn/c.png"), TriggerAtSeconds: &third, HideAtSeconds: &hideThird, CreatedAt: fixedAt},
	}
	for _, item := range items {
		require.NoError(t, repo.Create(ctx, item))
	}

	got, err := repo.GetByID(ctx, items[0].ID)
	require.NoError(t, err)
	assert.Equal(t, items[0], got)

	listed, err := repo.ListByContentNode(ctx, node.ID)
	require.NoError(t, err)
	require.Len(t, listed, 3)
	assert.Equal(t, second, *listed[0].TriggerAtSeconds)
	assert.Equal(t, third, *listed[1].TriggerAtSeconds)
	assert.Equal(t, first, *listed[2].TriggerAtSeconds)
}

func TestEntLearningPathRepository_CreateAndGet(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	nodeRepo := NewEntContentNodeRepository(client)
	repo := NewEntLearningPathRepository(client)

	node1 := seedContentNode(t, ctx, nodeRepo)
	node2 := seedContentNode(t, ctx, nodeRepo)

	sectionLabel := "Open chords"
	path := domain.LearningPath{
		ID: uuid.NewString(), TeacherID: uuid.NewString(), Title: "Week 1",
		Items: []domain.LearningPathItem{
			{Position: 1, ContentNodeID: node1.ID, Title: node1.Title, ContentType: node1.ContentType, SectionLabel: &sectionLabel},
			{Position: 2, ContentNodeID: node2.ID, Title: node2.Title, ContentType: node2.ContentType},
		},
		CreatedAt: fixedAt,
	}
	require.NoError(t, repo.Create(ctx, path))

	got, err := repo.GetByID(ctx, path.ID)
	require.NoError(t, err)
	assert.Equal(t, path, got)
	// section_label round-trips exactly — the set label on item 1 and the
	// absent label on item 2 (stored NULL, read back as nil).
	require.NotNil(t, got.Items[0].SectionLabel)
	assert.Equal(t, "Open chords", *got.Items[0].SectionLabel)
	assert.Nil(t, got.Items[1].SectionLabel)

	_, err = repo.GetByID(ctx, uuid.NewString())
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// TestEntLearningPathRepository_List confirms List batches its item and
// content-node lookups across every path (rather than one GetByID-style
// round trip per path) while still returning each path's items in the
// right order and never leaking another path's items into it.
func TestEntLearningPathRepository_List(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	nodeRepo := NewEntContentNodeRepository(client)
	repo := NewEntLearningPathRepository(client)

	// A fresh database with no paths yet returns a non-nil empty slice, not
	// nil — callers (ultimately the HTTP JSON response) must see `[]`, not
	// `null`.
	empty, err := repo.List(ctx)
	require.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)

	node1 := seedContentNode(t, ctx, nodeRepo)
	node2 := seedContentNode(t, ctx, nodeRepo)
	node3 := seedContentNode(t, ctx, nodeRepo)

	pathA := domain.LearningPath{
		ID: uuid.NewString(), TeacherID: uuid.NewString(), Title: "Path A",
		Items: []domain.LearningPathItem{
			{Position: 1, ContentNodeID: node1.ID, Title: node1.Title, ContentType: node1.ContentType},
			{Position: 2, ContentNodeID: node2.ID, Title: node2.Title, ContentType: node2.ContentType},
		},
		CreatedAt: fixedAt,
	}
	require.NoError(t, repo.Create(ctx, pathA))
	pathB := domain.LearningPath{
		ID: uuid.NewString(), TeacherID: uuid.NewString(), Title: "Path B",
		Items: []domain.LearningPathItem{
			{Position: 1, ContentNodeID: node3.ID, Title: node3.Title, ContentType: node3.ContentType},
		},
		CreatedAt: fixedAt,
	}
	require.NoError(t, repo.Create(ctx, pathB))

	list, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 2)

	byID := map[string]domain.LearningPath{}
	for _, p := range list {
		byID[p.ID] = p
	}
	require.Contains(t, byID, pathA.ID)
	require.Contains(t, byID, pathB.ID)
	assert.Equal(t, pathA, byID[pathA.ID])
	assert.Equal(t, pathB, byID[pathB.ID])
}

func TestEntStudentPathRepository(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	nodeRepo := NewEntContentNodeRepository(client)
	pathRepo := NewEntLearningPathRepository(client)
	versionRepo := NewEntContentNodeVersionRepository(client)
	repo := NewEntStudentPathRepository(client)

	node := seedContentNode(t, ctx, nodeRepo)
	template := seedLearningPath(t, ctx, pathRepo, node)
	studentID := uuid.NewString()
	assignedBy := uuid.NewString()

	version := domain.ContentNodeVersion{
		ID: uuid.NewString(), ContentNodeID: node.ID, VersionNumber: 1,
		Title: node.Title, ContentType: node.ContentType, PublishedBy: assignedBy, PublishedAt: fixedAt,
	}
	require.NoError(t, versionRepo.Create(ctx, version))

	sp := domain.StudentPath{
		ID: uuid.NewString(), StudentID: studentID, SourceTemplateID: template.ID,
		Title: template.Title, AssignedBy: assignedBy, AssignedAt: fixedAt,
		Items: []domain.StudentPathItemRecord{
			{Position: 1, ContentNodeID: node.ID, ContentNodeVersionID: version.ID},
		},
	}
	require.NoError(t, repo.Create(ctx, sp))

	fetched, err := repo.GetByID(ctx, sp.ID)
	require.NoError(t, err)
	assert.Equal(t, sp, fetched)

	active, err := repo.ListActiveStandaloneByStudentID(ctx, studentID)
	require.NoError(t, err)
	require.Len(t, active, 1)
	assert.Equal(t, sp.ID, active[0].ID)

	require.NoError(t, repo.Archive(ctx, sp.ID, fixedAt.Add(time.Hour)))
	fetched, err = repo.GetByID(ctx, sp.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched.ArchivedAt)

	active, err = repo.ListActiveStandaloneByStudentID(ctx, studentID)
	require.NoError(t, err)
	assert.Empty(t, active)
}

func TestEntContentNodeVersionRepository_GetLatestByContentNodeID(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	nodeRepo := NewEntContentNodeRepository(client)
	repo := NewEntContentNodeVersionRepository(client)

	node := seedContentNode(t, ctx, nodeRepo)
	publishedBy := uuid.NewString()

	_, err := repo.GetLatestByContentNodeID(ctx, node.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)

	v1 := domain.ContentNodeVersion{ID: uuid.NewString(), ContentNodeID: node.ID, VersionNumber: 1, Title: node.Title, ContentType: node.ContentType, PublishedBy: publishedBy, PublishedAt: fixedAt}
	require.NoError(t, repo.Create(ctx, v1))

	latest, err := repo.GetLatestByContentNodeID(ctx, node.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, latest.VersionNumber)

	v2 := domain.ContentNodeVersion{ID: uuid.NewString(), ContentNodeID: node.ID, VersionNumber: 2, Title: "Revised", ContentType: node.ContentType, PublishedBy: publishedBy, PublishedAt: fixedAt.Add(time.Hour)}
	require.NoError(t, repo.Create(ctx, v2))

	latest, err = repo.GetLatestByContentNodeID(ctx, node.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, latest.VersionNumber)
	assert.Equal(t, "Revised", latest.Title)
}

func TestEntCourseVersionRepository_GetLatestByCourseID(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	nodeRepo := NewEntContentNodeRepository(client)
	pathRepo := NewEntLearningPathRepository(client)
	courseRepo := NewEntCourseRepository(client)
	repo := NewEntCourseVersionRepository(client)

	node := seedContentNode(t, ctx, nodeRepo)
	path := seedLearningPath(t, ctx, pathRepo, node)

	course := domain.Course{
		ID: uuid.NewString(), CreatedBy: uuid.NewString(), Title: "Fingerstyle Journey",
		Summary: "From first chords to a repertoire.", Level: domain.DifficultyLevelBeginner,
		Status: domain.CourseStatusDraft, CreatedAt: fixedAt,
		Checkpoints: []domain.CourseCheckpoint{{Position: 1, LearningPathID: path.ID, EffectiveTitle: path.Title}},
	}
	require.NoError(t, courseRepo.Create(ctx, course))

	_, err := repo.GetLatestByCourseID(ctx, course.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)

	v1 := domain.CourseVersion{
		ID: uuid.NewString(), CourseID: course.ID, VersionNumber: 1,
		TitleSnapshot: course.Title, SummarySnapshot: course.Summary, LevelSnapshot: course.Level,
		Checkpoints:                []domain.CourseVersionCheckpoint{{Position: 1, LearningPathID: path.ID, EffectiveTitle: path.Title}},
		PublishedAt:                fixedAt,
		AvailableForNewEnrollments: true,
	}
	require.NoError(t, repo.Create(ctx, v1))

	latest, err := repo.GetLatestByCourseID(ctx, course.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, latest.VersionNumber)
	assert.Equal(t, "Fingerstyle Journey", latest.TitleSnapshot)
	require.Len(t, latest.Checkpoints, 1)
	assert.Equal(t, path.ID, latest.Checkpoints[0].LearningPathID)
	assert.True(t, latest.AvailableForNewEnrollments)

	v2 := domain.CourseVersion{
		ID: uuid.NewString(), CourseID: course.ID, VersionNumber: 2,
		TitleSnapshot: "Revised Journey", SummarySnapshot: course.Summary, LevelSnapshot: course.Level,
		Checkpoints:                []domain.CourseVersionCheckpoint{{Position: 1, LearningPathID: path.ID, EffectiveTitle: path.Title}},
		PublishedAt:                fixedAt.Add(time.Hour),
		AvailableForNewEnrollments: true,
	}
	require.NoError(t, repo.Create(ctx, v2))

	latest, err = repo.GetLatestByCourseID(ctx, course.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, latest.VersionNumber)
	assert.Equal(t, "Revised Journey", latest.TitleSnapshot)
}

func TestEntCourseRepository_UpdateStatus(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	nodeRepo := NewEntContentNodeRepository(client)
	pathRepo := NewEntLearningPathRepository(client)
	repo := NewEntCourseRepository(client)

	node := seedContentNode(t, ctx, nodeRepo)
	path := seedLearningPath(t, ctx, pathRepo, node)

	course := domain.Course{
		ID: uuid.NewString(), CreatedBy: uuid.NewString(), Title: "Fingerstyle Journey",
		Summary: "From first chords to a repertoire.", Level: domain.DifficultyLevelBeginner,
		Status: domain.CourseStatusDraft, CreatedAt: fixedAt,
		Checkpoints: []domain.CourseCheckpoint{{Position: 1, LearningPathID: path.ID, EffectiveTitle: path.Title}},
	}
	require.NoError(t, repo.Create(ctx, course))

	require.NoError(t, repo.UpdateStatus(ctx, course.ID, domain.CourseStatusPublished))

	got, err := repo.GetByID(ctx, course.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.CourseStatusPublished, got.Status)

	err = repo.UpdateStatus(ctx, uuid.NewString(), domain.CourseStatusPublished)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestEntStudentLearningStateRepository(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	repo := NewEntStudentLearningStateRepository(client)
	studentID := uuid.NewString()

	_, err := repo.GetByStudentID(ctx, studentID)
	require.ErrorIs(t, err, domain.ErrNotFound)

	pathID := uuid.NewString()
	state := domain.StudentLearningState{StudentID: studentID}.WithCurrentStandalonePath(pathID)
	require.NoError(t, repo.Upsert(ctx, state))

	fetched, err := repo.GetByStudentID(ctx, studentID)
	require.NoError(t, err)
	assert.Equal(t, pathID, *fetched.CurrentStandalonePathID)
	assert.Nil(t, fetched.CurrentCourseEnrollmentID)

	enrollmentID := uuid.NewString()
	require.NoError(t, repo.Upsert(ctx, fetched.WithCurrentCourseEnrollment(enrollmentID)))

	fetched, err = repo.GetByStudentID(ctx, studentID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentID, *fetched.CurrentCourseEnrollmentID)
	assert.Nil(t, fetched.CurrentStandalonePathID)
}

func seedContentNode(t *testing.T, ctx context.Context, repo *EntContentNodeRepository) domain.ContentNode {
	t.Helper()
	skill := seedSkill(t, ctx, repo.client, "skill-"+uuid.NewString())
	concept := seedConcept(t, ctx, repo.client, "concept-"+uuid.NewString())
	node := domain.ContentNode{
		ID: uuid.NewString(), TeacherID: uuid.NewString(), Title: "Node " + uuid.NewString(), ContentType: domain.ContentTypeVideo,
		Classification: domain.Classification{Skills: []domain.Skill{skill}, Concepts: []domain.Concept{concept}, DifficultyLevel: domain.DifficultyLevelBeginner, ReviewState: domain.ReviewStatePending},
		CreatedAt:      fixedAt,
	}
	require.NoError(t, repo.Create(ctx, node))
	return node
}

func seedLearningPath(t *testing.T, ctx context.Context, repo *EntLearningPathRepository, node domain.ContentNode) domain.LearningPath {
	t.Helper()
	path := domain.LearningPath{
		ID: uuid.NewString(), TeacherID: uuid.NewString(), Title: "Path " + uuid.NewString(),
		Items:     []domain.LearningPathItem{{Position: 1, ContentNodeID: node.ID, Title: node.Title, ContentType: node.ContentType}},
		CreatedAt: fixedAt,
	}
	require.NoError(t, repo.Create(ctx, path))
	return path
}
