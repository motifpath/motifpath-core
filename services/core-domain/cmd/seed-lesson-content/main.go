// Command seed-lesson-content enriches the learning path already assigned to
// the local dev student so the lesson screen can be tested by hand: it gives
// the path's videos playable media, attaches timed expanded-content cues, and
// can reset the student's progress so a scenario can be replayed.
//
// It works on top of whatever `seed-dev-data` (or hand authoring) already
// created and never wipes anything: it only sets a node's media URL and
// replaces the video-timed cues on the nodes it manages, leaving every other
// row alone. Like `seed-dev-data`, it reuses the application services rather
// than raw SQL, targets the first registered user, and is run ad hoc against
// `make dev`'s docker-compose stack — it is not part of the service.
//
// Path positions (1-based) and what each is set up to exercise:
//
//	1  left as authored — a completed step, for the "watch again or review" flow
//	2  completed, self-hosted MP4
//	3  the step with a challenge, YouTube video, two timed cues
//	4  self-hosted MP4 with two overlapping cues (earliest start wins)
//	5  self-hosted MP4 with one image cue
//	6  a video URL that cannot load, for the load-failure state
//
// Usage:
//
//	go run ./cmd/seed-lesson-content                       # enrich, keep progress
//	go run ./cmd/seed-lesson-content -completed 2          # steps 1-2 done, step 3 in progress
//	go run ./cmd/seed-lesson-content -completed 3 -current not_started
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/motifpath/core-domain/internal/adapters/repo"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/contentnode"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/language"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

const (
	shortClipMP4  = "https://interactive-examples.mdn.mozilla.net/media/cc0-videos/flower.mp4"
	tenSecondMP4  = "https://samplelib.com/lib/preview/mp4/sample-10s.mp4"
	youtubeLesson = "https://www.youtube.com/watch?v=aqz-KE-bpKQ"
	// unloadableMP4 uses the reserved .invalid TLD, which never resolves, so
	// the player always reports a load failure.
	unloadableMP4 = "https://media.motifpath.invalid/missing-lesson.mp4"
)

// cueSpec is one timed expanded-content item. An image cue renders a
// placeholder picture captioned with text; a rich-text cue carries text as
// its body.
type cueSpec struct {
	kind       domain.ExpandedContentType
	text       string
	start, end int
}

// lessonSpec is what a path position is set up with. A nil mediaURL leaves the
// node's existing media alone.
type lessonSpec struct {
	mediaURL *string
	cues     []cueSpec
}

func strPtr(v string) *string { return &v }

func intPtr(v int) *int { return &v }

// lessonSpecs is keyed by 1-based path position.
var lessonSpecs = map[int]lessonSpec{
	2: {mediaURL: strPtr(tenSecondMP4)},
	3: {
		mediaURL: strPtr(youtubeLesson),
		cues: []cueSpec{
			{kind: domain.ExpandedContentTypeImage, text: "Cue A image", start: 3, end: 6},
			{kind: domain.ExpandedContentTypeRichText, text: "Cue B: a note that appears beside the video.", start: 8, end: 12},
		},
	},
	4: {
		mediaURL: strPtr(shortClipMP4),
		cues: []cueSpec{
			{kind: domain.ExpandedContentTypeImage, text: "Early cue", start: 1, end: 3},
			{kind: domain.ExpandedContentTypeRichText, text: "Overlapping cue: shown only once the early cue ends.", start: 2, end: 4},
		},
	},
	5: {
		mediaURL: strPtr(tenSecondMP4),
		cues:     []cueSpec{{kind: domain.ExpandedContentTypeImage, text: "Single cue", start: 2, end: 6}},
	},
	6: {mediaURL: strPtr(unloadableMP4)},
}

func getenvDefault(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func main() {
	completed := flag.Int("completed", -1, "reset progress so this many steps are completed; -1 leaves progress untouched")
	current := flag.String("current", "in_progress", "status of the step after the completed ones: in_progress or not_started")
	flag.Parse()

	if err := run(*completed, *current); err != nil {
		log.Fatal(err)
	}
}

// connections holds the open stores and closes them together.
type connections struct {
	ent   *ent.Client
	mongo *mongo.Client
	sqlDB *sql.DB
}

func connect() (*connections, error) {
	databaseURL := getenvDefault("DATABASE_URL", "postgres://motifpath:motifpath@localhost:5432/core_domain?sslmode=disable")
	mongoURI := getenvDefault("MONGO_URI", "mongodb://motifpath:motifpath@localhost:27017/?authSource=admin")

	sqlDB, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}
	mongoClient, err := mongo.Connect(options.Client().ApplyURI(mongoURI))
	if err != nil {
		if closeErr := sqlDB.Close(); closeErr != nil {
			log.Printf("failed to close postgres connection: %v", closeErr)
		}
		return nil, fmt.Errorf("connect to mongodb: %w", err)
	}
	return &connections{
		ent:   ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, sqlDB))),
		mongo: mongoClient,
		sqlDB: sqlDB,
	}, nil
}

func (c *connections) close(ctx context.Context) {
	if err := c.ent.Close(); err != nil {
		log.Printf("failed to close ent client: %v", err)
	}
	if err := c.sqlDB.Close(); err != nil {
		log.Printf("failed to close postgres connection: %v", err)
	}
	if err := c.mongo.Disconnect(ctx); err != nil {
		log.Printf("failed to disconnect mongodb client: %v", err)
	}
}

func run(completed int, current string) error {
	if current != "in_progress" && current != "not_started" {
		return fmt.Errorf("-current must be in_progress or not_started, got %q", current)
	}

	ctx := context.Background()
	conns, err := connect()
	if err != nil {
		return err
	}
	defer conns.close(ctx)

	newID := uuid.NewString
	now := func() time.Time { return time.Now().UTC() }
	contentService := application.NewContentService(
		repo.NewEntContentNodeRepository(conns.ent),
		repo.NewEntExpandedContentRepository(conns.ent),
		repo.NewEntSkillRepository(conns.ent),
		repo.NewEntConceptRepository(conns.ent),
		repo.NewEntContentNodeVersionRepository(conns.ent),
		newID, now,
	)

	defaults, err := loadDefaults(ctx, conns.ent, application.NewSkillService(repo.NewEntSkillRepository(conns.ent), newID), application.NewConceptService(repo.NewEntConceptRepository(conns.ent), newID))
	if err != nil {
		return err
	}

	studentID, err := firstUserID(ctx, conns.ent)
	if err != nil {
		return err
	}
	path, err := activePath(ctx, conns.ent, studentID)
	if err != nil {
		return err
	}
	log.Printf("enriching path %q (%d items) of user %s", path.Title, len(path.Items), studentID)

	// The tool acts as an admin so it can edit nodes it did not author; the
	// caller is never persisted.
	admin := domain.User{ID: newID(), Role: domain.RoleAdmin}

	if err := enrichSteps(ctx, contentService, admin, defaults, path.Items); err != nil {
		return err
	}
	if err := ensureExerciseLanguages(ctx, conns.sqlDB, defaults, path.Items); err != nil {
		return fmt.Errorf("ensure exercise languages: %w", err)
	}

	if completed >= 0 {
		db := conns.mongo.Database(getenvDefault("MONGO_DATABASE", "motifpath_events"))
		if err := resetProgress(ctx, db, studentID, path.Items, completed, current); err != nil {
			return fmt.Errorf("reset progress: %w", err)
		}
	}

	log.Println("done — reload the SPA's /path view")
	return nil
}

// enrichSteps prepares every step of the path: it makes sure the step is
// language-tagged, then applies the step's lesson spec if it has one.
func enrichSteps(ctx context.Context, svc *application.ContentService, admin domain.User, defaults classificationDefaults, items []domain.LearningPathItem) error {
	for _, item := range items {
		// Every step needs a language tag, including ones with no lesson
		// spec, or resetting progress would leave it locked.
		if err := defaults.ensureLanguage(ctx, item.ContentNodeID); err != nil {
			return fmt.Errorf("position %d: %w", item.Position, err)
		}
		spec, ok := lessonSpecs[item.Position]
		if !ok {
			continue
		}
		if err := applyLessonSpec(ctx, svc, admin, defaults, item, spec); err != nil {
			return fmt.Errorf("position %d: %w", item.Position, err)
		}
	}
	return nil
}

func activePath(ctx context.Context, client *ent.Client, studentID string) (domain.LearningPath, error) {
	state, err := repo.NewEntStudentLearningStateRepository(client).GetByStudentID(ctx, studentID)
	if err != nil {
		return domain.LearningPath{}, fmt.Errorf("find the current path of user %s (run seed-dev-data first): %w", studentID, err)
	}
	if state.CurrentStandalonePathID == nil {
		return domain.LearningPath{}, fmt.Errorf("user %s has no current standalone path set (run seed-dev-data first)", studentID)
	}
	studentPath, err := repo.NewEntStudentPathRepository(client).GetByID(ctx, *state.CurrentStandalonePathID)
	if err != nil {
		return domain.LearningPath{}, fmt.Errorf("load student path %s: %w", *state.CurrentStandalonePathID, err)
	}
	path, err := repo.NewEntLearningPathRepository(client).GetByID(ctx, studentPath.SourceTemplateID)
	if err != nil {
		return domain.LearningPath{}, fmt.Errorf("load learning path %s: %w", studentPath.SourceTemplateID, err)
	}
	return path, nil
}

// classificationDefaults fills in the tags a node is missing. Nodes created
// before skills, concepts and languages became mandatory have none, and the
// service refuses to save such a node until it has at least one of each.
type classificationDefaults struct {
	skillID   string
	conceptID string
	// client is used to link a language directly, because steps with no lesson
	// spec are never saved through the content service. A node with no
	// language tag is locked for every student that has not already completed
	// it, so every step must be tagged.
	client *ent.Client
}

// defaultLanguageCode is the language-agnostic tag, so the seeded lessons are
// reachable whatever locale the student uses.
const defaultLanguageCode = domain.LanguageCodeAny

// ensureLanguage links the default language to the node unless it already has
// one.
func (d classificationDefaults) ensureLanguage(ctx context.Context, nodeID string) error {
	id, err := uuid.Parse(nodeID)
	if err != nil {
		return fmt.Errorf("parse node id %q: %w", nodeID, err)
	}
	tagged, err := d.client.ContentNode.Query().Where(contentnode.ID(id), contentnode.HasLanguages()).Exist(ctx)
	if err != nil {
		return fmt.Errorf("check languages on %s: %w", nodeID, err)
	}
	if tagged {
		return nil
	}
	lang, err := d.client.Language.Query().Where(language.Code(defaultLanguageCode)).Only(ctx)
	if err != nil {
		return fmt.Errorf("find language %q: %w", defaultLanguageCode, err)
	}
	if _, err := d.client.ContentNode.UpdateOneID(id).AddLanguageIDs(lang.ID).Save(ctx); err != nil {
		return fmt.Errorf("link language %q to %s: %w", defaultLanguageCode, nodeID, err)
	}
	log.Printf("node %s: linked language %q", nodeID, defaultLanguageCode)
	return nil
}

// ensureExerciseLanguage links langID to the exercise. Unlike ensureLanguage
// it does not check first — callers already know the exercise has none
// (exercisesMissingLanguage only returns those) — and it takes the language
// id rather than resolving it itself, so tagging many exercises resolves the
// language row once, not once per exercise.
func (d classificationDefaults) ensureExerciseLanguage(ctx context.Context, exerciseID string, langID uuid.UUID) error {
	id, err := uuid.Parse(exerciseID)
	if err != nil {
		return fmt.Errorf("parse exercise id %q: %w", exerciseID, err)
	}
	if _, err := d.client.Exercise.UpdateOneID(id).AddLanguageIDs(langID).Save(ctx); err != nil {
		return fmt.Errorf("link language %q to exercise %s: %w", defaultLanguageCode, exerciseID, err)
	}
	log.Printf("exercise %s: linked language %q", exerciseID, defaultLanguageCode)
	return nil
}

// ensureExerciseLanguages tags every exercise reachable from the path — linked
// directly to a node as a path exercise, or through one of a node's
// challenges — that has no language. The path-progress lock checks a linked
// exercise's language as well as the node's own, so an untagged exercise can
// silently lock a step whose video is perfectly playable and correctly
// tagged itself.
func ensureExerciseLanguages(ctx context.Context, sqlDB *sql.DB, defaults classificationDefaults, items []domain.LearningPathItem) error {
	nodeIDs := make([]string, len(items))
	for i, item := range items {
		nodeIDs[i] = item.ContentNodeID
	}
	ids, err := exercisesMissingLanguage(ctx, sqlDB, nodeIDs)
	if err != nil {
		return fmt.Errorf("find exercises with no language: %w", err)
	}
	if len(ids) == 0 {
		return nil
	}

	lang, err := defaults.client.Language.Query().Where(language.Code(defaultLanguageCode)).Only(ctx)
	if err != nil {
		return fmt.Errorf("find language %q: %w", defaultLanguageCode, err)
	}
	for _, id := range ids {
		if err := defaults.ensureExerciseLanguage(ctx, id, lang.ID); err != nil {
			return err
		}
	}
	return nil
}

// exercisesMissingLanguage returns the ids of exercises that have no language
// row and are reachable from nodeIDs either as a path exercise
// (content_node_exercises) or as one of a node's challenge's exercises
// (challenges + challenge_exercises). Plain SQL, not the ent client: this is
// a read across three junction tables for a local dev tool, not a query the
// application layer needs to expose.
func exercisesMissingLanguage(ctx context.Context, sqlDB *sql.DB, nodeIDs []string) ([]string, error) {
	const query = `
		SELECT DISTINCT e.id::text
		FROM exercises e
		JOIN content_node_exercises x ON x.exercise_id = e.id
		WHERE x.content_node_id = ANY($1)
		  AND NOT EXISTS (SELECT 1 FROM exercise_languages el WHERE el.exercise_id = e.id)
		UNION
		SELECT DISTINCT e.id::text
		FROM exercises e
		JOIN challenge_exercises ce ON ce.exercise_id = e.id
		JOIN challenges c ON c.id = ce.challenge_id
		WHERE c.content_node_id = ANY($1)
		  AND NOT EXISTS (SELECT 1 FROM exercise_languages el WHERE el.exercise_id = e.id)
	`
	rows, err := sqlDB.QueryContext(ctx, query, pq.Array(nodeIDs))
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("failed to close rows: %v", closeErr)
		}
	}()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func loadDefaults(ctx context.Context, client *ent.Client, skills *application.SkillService, concepts *application.ConceptService) (classificationDefaults, error) {
	skillList, err := skills.ListSkills(ctx)
	if err != nil {
		return classificationDefaults{}, fmt.Errorf("list skills: %w", err)
	}
	conceptList, err := concepts.ListConcepts(ctx)
	if err != nil {
		return classificationDefaults{}, fmt.Errorf("list concepts: %w", err)
	}
	if len(skillList) == 0 || len(conceptList) == 0 {
		return classificationDefaults{}, fmt.Errorf("the database has no skills or concepts to tag legacy nodes with; run seed-dev-data first")
	}
	return classificationDefaults{skillID: skillList[0].ID, conceptID: conceptList[0].ID, client: client}, nil
}

// applyLessonSpec sets the node's media URL (when the spec has one) and
// replaces its video-timed cues with the spec's. Paragraph-triggered cues are
// left alone.
func applyLessonSpec(ctx context.Context, svc *application.ContentService, admin domain.User, defaults classificationDefaults, item domain.LearningPathItem, spec lessonSpec) error {
	if spec.mediaURL != nil {
		if err := setMedia(ctx, svc, admin, defaults, item.ContentNodeID, *spec.mediaURL); err != nil {
			return err
		}
		log.Printf("position %d: media set to %s", item.Position, *spec.mediaURL)
	}

	if err := replaceTimedCues(ctx, svc, admin, item.ContentNodeID, spec.cues); err != nil {
		return err
	}
	if len(spec.cues) > 0 {
		log.Printf("position %d: %d timed cue(s) set", item.Position, len(spec.cues))
	}
	return nil
}

func setMedia(ctx context.Context, svc *application.ContentService, admin domain.User, defaults classificationDefaults, nodeID, mediaURL string) error {
	node, err := svc.GetContentNode(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("load content node %s: %w", nodeID, err)
	}
	if node.ContentType != domain.ContentTypeVideo {
		return fmt.Errorf("node %s is %s, not a video — refusing to set a video URL", node.ID, node.ContentType)
	}
	skillIDs, conceptIDs, languages := node.Classification.SkillIDs(), node.Classification.ConceptIDs(), languageCodes(node.Languages)
	if len(skillIDs) == 0 || len(conceptIDs) == 0 || len(languages) == 0 {
		log.Printf("node %s has no skill, concept or language tags; filling the missing ones with defaults", node.ID)
		skillIDs = orDefault(skillIDs, defaults.skillID)
		conceptIDs = orDefault(conceptIDs, defaults.conceptID)
		languages = orDefault(languages, defaultLanguageCode)
	}
	if _, err := svc.UpdateContentNode(ctx, admin, node.ID, node.Title, skillIDs, conceptIDs, node.Classification.DifficultyLevel, languages, &mediaURL, node.RichContent); err != nil {
		return fmt.Errorf("set media URL on %s: %w", node.ID, err)
	}
	return nil
}

func languageCodes(languages []domain.Language) []string {
	codes := make([]string, len(languages))
	for i, l := range languages {
		codes[i] = l.Code
	}
	return codes
}

// orDefault returns values, or a one-element list holding fallback when
// values is empty.
func orDefault(values []string, fallback string) []string {
	if len(values) == 0 {
		return []string{fallback}
	}
	return values
}

func replaceTimedCues(ctx context.Context, svc *application.ContentService, admin domain.User, nodeID string, cues []cueSpec) error {
	existing, err := svc.ListExpandedContent(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("list cues for %s: %w", nodeID, err)
	}
	for _, cue := range existing {
		if cue.TriggerAtSeconds == nil {
			continue
		}
		if err := svc.DeleteExpandedContent(ctx, admin, cue.ID); err != nil {
			return fmt.Errorf("delete cue %s: %w", cue.ID, err)
		}
	}
	for _, cue := range cues {
		if err := createCue(ctx, svc, admin, nodeID, cue); err != nil {
			return err
		}
	}
	return nil
}

func createCue(ctx context.Context, svc *application.ContentService, admin domain.User, nodeID string, cue cueSpec) error {
	var (
		mediaURL *string
		rich     *domain.PromptDocument
	)
	if cue.kind == domain.ExpandedContentTypeRichText {
		doc := domain.NewPlainTextPrompt(cue.text)
		rich = &doc
	} else {
		mediaURL = strPtr("https://placehold.co/640x360/png?text=" + url.QueryEscape(cue.text))
	}
	if _, err := svc.CreateExpandedContent(ctx, admin, nodeID, cue.kind, mediaURL, rich, intPtr(cue.start), intPtr(cue.end), nil, nil, strPtr(cue.text)); err != nil {
		return fmt.Errorf("create %s cue %q on %s: %w", cue.kind, cue.text, nodeID, err)
	}
	return nil
}

// resetProgress rewrites the student's completion aggregates for the path so
// the first `completed` steps are completed and the next one has the given
// status. Steps after it get no aggregate, which the API reports as locked.
func resetProgress(ctx context.Context, db *mongo.Database, studentID string, items []domain.LearningPathItem, completed int, current string) error {
	if completed > len(items) {
		return fmt.Errorf("-completed %d exceeds the path length %d", completed, len(items))
	}
	nodeIDs := make([]string, len(items))
	for i, item := range items {
		nodeIDs[i] = item.ContentNodeID
	}
	collection := db.Collection("aggregates")

	if _, err := collection.DeleteMany(ctx, bson.D{
		{Key: "student_id", Value: studentID},
		{Key: "content_node_id", Value: bson.D{{Key: "$in", Value: nodeIDs}}},
	}); err != nil {
		return err
	}

	for i, nodeID := range nodeIDs {
		var status string
		switch {
		case i < completed:
			status = "completed"
		case i == completed:
			status = current
		default:
			continue
		}
		if _, err := collection.InsertOne(ctx, bson.D{
			{Key: "student_id", Value: studentID},
			{Key: "content_node_id", Value: nodeID},
			{Key: "status", Value: status},
		}); err != nil {
			return err
		}
	}
	log.Printf("progress reset: %d completed, then %s", completed, current)
	return nil
}

func firstUserID(ctx context.Context, client *ent.Client) (string, error) {
	row, err := client.User.Query().First(ctx)
	if err != nil {
		return "", fmt.Errorf("find a registered user (sign in once through the SPA first): %w", err)
	}
	return row.ID.String(), nil
}
