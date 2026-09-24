// Command seed-dev-data populates the local dev Postgres + MongoDB with a
// realistic student learning path, reusing the same application services
// and domain constructors the HTTP handlers use (not raw SQL), so the
// invariants they enforce (position assignment, section-label
// normalisation, single-active-assignment) hold exactly as they would via
// the real API. It targets whichever student is already registered in the
// local dev database (from having signed in once through the SPA) and is
// meant to be run once, ad hoc, against `make dev`'s docker-compose stack —
// it is not part of the service and is not wired into any build.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/motifpath/core-domain/internal/adapters/repo"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/user"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

func getenvDefault(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()

	databaseURL := getenvDefault("DATABASE_URL", "postgres://motifpath:motifpath@localhost:5432/core_domain?sslmode=disable")
	mongoURI := getenvDefault("MONGO_URI", "mongodb://motifpath:motifpath@localhost:27017/?authSource=admin")
	mongoDatabase := getenvDefault("MONGO_DATABASE", "motifpath_events")

	sqlDB, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer func() {
		if closeErr := sqlDB.Close(); closeErr != nil {
			log.Printf("failed to close postgres connection: %v", closeErr)
		}
	}()

	entClient := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, sqlDB)))
	defer func() {
		if closeErr := entClient.Close(); closeErr != nil {
			log.Printf("failed to close ent client: %v", closeErr)
		}
	}()

	mongoClient, err := mongo.Connect(options.Client().ApplyURI(mongoURI))
	if err != nil {
		return fmt.Errorf("connect to mongodb: %w", err)
	}
	defer func() {
		if closeErr := mongoClient.Disconnect(ctx); closeErr != nil {
			log.Printf("failed to disconnect mongodb client: %v", closeErr)
		}
	}()

	userRepo := repo.NewEntUserRepository(entClient)
	nodeRepo := repo.NewEntContentNodeRepository(entClient)
	expandedRepo := repo.NewEntExpandedContentRepository(entClient)
	pathRepo := repo.NewEntLearningPathRepository(entClient)
	studentPathRepo := repo.NewEntStudentPathRepository(entClient)
	contentNodeVersionRepo := repo.NewEntContentNodeVersionRepository(entClient)
	studentLearningStateRepo := repo.NewEntStudentLearningStateRepository(entClient)
	courseEnrollmentRepo := repo.NewEntCourseEnrollmentRepository(entClient)
	courseVersionRepo := repo.NewEntCourseVersionRepository(entClient)
	challengeRepo := repo.NewEntChallengeRepository(entClient)
	exerciseRepo := repo.NewEntExerciseRepository(entClient)
	skillRepo := repo.NewEntSkillRepository(entClient)
	conceptRepo := repo.NewEntConceptRepository(entClient)
	diagramRepo := repo.NewEntDiagramRepository(entClient)

	newID := uuid.NewString
	now := func() time.Time { return time.Now().UTC() }

	contentService := application.NewContentService(nodeRepo, expandedRepo, skillRepo, conceptRepo, contentNodeVersionRepo, diagramRepo, newID, now)
	pathService := application.NewLearningPathService(nodeRepo, pathRepo, courseVersionRepo, newID, now)
	studentPathService := application.NewStudentPathService(userRepo, pathRepo, studentPathRepo, contentNodeVersionRepo, studentLearningStateRepo, courseEnrollmentRepo, courseVersionRepo, nodeRepo, exerciseRepo, nil, newID, now)
	challengeService := application.NewChallengeService(nodeRepo, challengeRepo, exerciseRepo, newID, now)
	exerciseService := application.NewExerciseService(challengeRepo, exerciseRepo, nodeRepo, skillRepo, conceptRepo, diagramRepo, newID, now, rand.Shuffle)
	skillService := application.NewSkillService(skillRepo, newID)
	conceptService := application.NewConceptService(conceptRepo, newID)

	student, err := findFirstStudent(ctx, entClient)
	if err != nil {
		return err
	}
	log.Printf("seeding a path for existing student %s (clerk_user_id=%s)", student.ID, student.ClerkUserID)

	identityService := application.NewIdentityService(userRepo, repo.NewEntLanguageRepository(entClient), newID, now)
	teacher, err := seedTeacher(ctx, identityService)
	if err != nil {
		return err
	}

	classifier := &classificationSeeder{skills: skillService, concepts: conceptService, teacher: teacher}
	nodeIDs, err := seedPathAndProgress(ctx, teacher, student, contentService, pathService, studentPathService, classifier, mongoClient.Database(mongoDatabase))
	if err != nil {
		return err
	}

	// A challenge + exercises on the in-progress node (nodeIDs[2]) so
	// /path/nodes/:nodeId/practice has something real to run against.
	practiceNodeID := nodeIDs[2]
	if err := seedPracticeChallenge(ctx, teacher, challengeService, exerciseService, classifier, practiceNodeID); err != nil {
		return fmt.Errorf("seed practice challenge: %w", err)
	}
	log.Printf("seeded a challenge + exercises on content node %s", practiceNodeID)

	log.Println("done — reload the SPA's /path view to see it")
	return nil
}

// classificationSeeder resolves plain skill/concept names to real Skill/
// Concept tree node ids, creating a fresh root node the first time a given
// name is seen in this run and reusing it thereafter. The API deliberately
// has no find-or-create endpoint (an ambiguous operation once names aren't
// globally unique across the tree), but this script isn't the API: it seeds
// a known, disjoint set of root-level names it fully controls, so a
// same-run cache is enough. Re-running this script against a database that
// already has these root names would fail on the sibling-uniqueness check —
// a real limitation, left as-is since this tool targets a fresh dev
// database.
type classificationSeeder struct {
	skills     *application.SkillService
	concepts   *application.ConceptService
	teacher    domain.User
	skillIDs   map[string]string
	conceptIDs map[string]string
}

func (c *classificationSeeder) skillID(ctx context.Context, name string) (string, error) {
	if c.skillIDs == nil {
		c.skillIDs = map[string]string{}
	}
	if id, ok := c.skillIDs[name]; ok {
		return id, nil
	}
	skill, err := c.skills.CreateSkill(ctx, c.teacher, name, nil)
	if err != nil {
		return "", fmt.Errorf("create skill %q: %w", name, err)
	}
	c.skillIDs[name] = skill.ID
	return skill.ID, nil
}

func (c *classificationSeeder) conceptID(ctx context.Context, name string) (string, error) {
	if c.conceptIDs == nil {
		c.conceptIDs = map[string]string{}
	}
	if id, ok := c.conceptIDs[name]; ok {
		return id, nil
	}
	concept, err := c.concepts.CreateConcept(ctx, c.teacher, name, nil)
	if err != nil {
		return "", fmt.Errorf("create concept %q: %w", name, err)
	}
	c.conceptIDs[name] = concept.ID
	return concept.ID, nil
}

// seedPathAndProgress creates the six-node learning path, assigns it to
// student, and seeds Mongo completion aggregates so the first two nodes show
// completed and the third shows in-progress. Returns the created content
// node ids in path order.
func seedPathAndProgress(
	ctx context.Context,
	teacher, student domain.User,
	contentService *application.ContentService,
	pathService *application.LearningPathService,
	studentPathService *application.StudentPathService,
	classifier *classificationSeeder,
	mongoDB *mongo.Database,
) ([]string, error) {
	type nodeSpec struct {
		title, skill, concept string
		difficulty            domain.DifficultyLevel
		section               string
	}
	specs := []nodeSpec{
		{"Open position C major scale", "Scales", "Major scale fingerings", domain.DifficultyLevelBeginner, "Fundamentals"},
		{"Alternate picking basics", "Picking technique", "Alternate picking", domain.DifficultyLevelBeginner, "Fundamentals"},
		{"Minor pentatonic shape 1", "Scales", "Pentatonic fingerings", domain.DifficultyLevelIntermediate, "Fundamentals"},
		{"Call-and-response phrasing", "Improvisation", "Phrasing", domain.DifficultyLevelIntermediate, "Improvisation"},
		{"Bending into the blue note", "Improvisation", "String bending", domain.DifficultyLevelIntermediate, "Improvisation"},
		{"12-bar blues solo, backing track", "Improvisation", "Full-length solo", domain.DifficultyLevelAdvanced, "Improvisation"},
	}

	// A real, publicly reachable sample video — cdn.motifpath.io doesn't
	// resolve to anything, so a node seeded with it can never actually play.
	seedVideoURL := "https://samplelib.com/lib/preview/mp4/sample-10s.mp4"

	var items []application.PathItemInput
	var nodeIDs []string
	for _, spec := range specs {
		skillID, err := classifier.skillID(ctx, spec.skill)
		if err != nil {
			return nil, err
		}
		conceptID, err := classifier.conceptID(ctx, spec.concept)
		if err != nil {
			return nil, err
		}
		node, err := contentService.CreateContentNode(ctx, teacher, spec.title, domain.ContentTypeVideo, []string{skillID}, []string{conceptID}, spec.difficulty, []string{"en"}, &seedVideoURL, nil)
		if err != nil {
			return nil, fmt.Errorf("create content node %q: %w", spec.title, err)
		}
		section := spec.section
		items = append(items, application.PathItemInput{ContentNodeID: node.ID, SectionLabel: &section})
		nodeIDs = append(nodeIDs, node.ID)
	}

	path, err := pathService.CreateLearningPath(ctx, teacher, "Blues Guitar Foundations", items)
	if err != nil {
		return nil, fmt.Errorf("create learning path: %w", err)
	}
	log.Printf("created learning path %s with %d items", path.ID, len(path.Items))

	for _, item := range path.Items {
		if _, err := contentService.PublishContentNode(ctx, teacher, item.ContentNodeID); err != nil {
			return nil, fmt.Errorf("publish content node %s: %w", item.ContentNodeID, err)
		}
	}

	if _, err := studentPathService.AssignLearningPath(ctx, teacher, student.ID, path.ID); err != nil {
		return nil, fmt.Errorf("assign learning path: %w", err)
	}
	log.Printf("assigned path %s to student %s", path.ID, student.ID)

	// Two completed, one in-progress; BuildStudentPathItems locks everything
	// after the first non-completed item, so the remaining three items
	// render as locked without needing their own aggregate rows.
	statuses := map[string]string{
		nodeIDs[0]: "completed",
		nodeIDs[1]: "completed",
		nodeIDs[2]: "in_progress",
	}
	if err := seedCompletionStatuses(ctx, mongoDB, student.ID, statuses); err != nil {
		return nil, fmt.Errorf("seed completion statuses: %w", err)
	}
	log.Printf("seeded %d completion statuses in MongoDB aggregates", len(statuses))

	return nodeIDs, nil
}

func seedPracticeChallenge(ctx context.Context, teacher domain.User, challengeService *application.ChallengeService, exerciseService *application.ExerciseService, classifier *classificationSeeder, contentNodeID string) error {
	// "Scales" was already created as a root skill while seeding nodeIDs[2]
	// (the practice node) above — classifier.skillID returns that same id
	// rather than creating a second one, since it caches by name for this run.
	subjectSkillID, err := classifier.skillID(ctx, "Scales")
	if err != nil {
		return err
	}
	challenge, err := challengeService.CreateChallenge(ctx, teacher, contentNodeID, &subjectSkillID, nil, 70, nil, false, false)
	if err != nil {
		return fmt.Errorf("create challenge: %w", err)
	}

	label := func(s string) *string { return &s }
	type exerciseSpec struct {
		prompt  string
		options []domain.Option
	}
	specs := []exerciseSpec{
		{
			prompt: "Which fret marks the root note of minor pentatonic shape 1 on the low E string?",
			options: []domain.Option{
				{ID: uuid.NewString(), IsCorrect: true, Label: label("5th fret")},
				{ID: uuid.NewString(), IsCorrect: false, Label: label("3rd fret")},
				{ID: uuid.NewString(), IsCorrect: false, Label: label("7th fret")},
			},
		},
		{
			prompt: "How many notes per string does minor pentatonic shape 1 use?",
			options: []domain.Option{
				{ID: uuid.NewString(), IsCorrect: false, Label: label("Three")},
				{ID: uuid.NewString(), IsCorrect: true, Label: label("Two")},
				{ID: uuid.NewString(), IsCorrect: false, Label: label("Four")},
			},
		},
		{
			prompt: "Shifting shape 1 up twelve frets lands on the same shape an octave higher — true or false?",
			options: []domain.Option{
				{ID: uuid.NewString(), IsCorrect: true, Label: label("True")},
				{ID: uuid.NewString(), IsCorrect: false, Label: label("False")},
			},
		},
	}

	pentatonicSkillID, err := classifier.skillID(ctx, "Pentatonic shapes")
	if err != nil {
		return err
	}
	pentatonicConceptID, err := classifier.conceptID(ctx, "Pentatonic fingerings")
	if err != nil {
		return err
	}
	for _, spec := range specs {
		exercise, err := exerciseService.CreateExercise(ctx, teacher, "Pentatonic shape 1 — "+spec.prompt, domain.NewPlainTextPrompt(spec.prompt), domain.ExerciseTypeTextResponse,
			[]string{pentatonicSkillID}, []string{pentatonicConceptID}, nil, nil, nil, nil, spec.options, nil, nil, []string{"en"})
		if err != nil {
			return fmt.Errorf("create exercise: %w", err)
		}
		if _, err := exerciseService.LinkExerciseToChallenge(ctx, teacher, challenge.ID, exercise.ID); err != nil {
			return fmt.Errorf("link exercise %s to challenge %s: %w", exercise.ID, challenge.ID, err)
		}
	}

	return nil
}

// seedTeacherClerkUserID is the fake clerk_user_id of the synthetic teacher
// that authors this script's content and assigns its path — never a real
// Clerk identity.
const seedTeacherClerkUserID = "seed_dev_teacher"

// seedTeacher returns the synthetic teacher, registering it with a name on
// the first run and reusing it on every later one, so what it authors
// always points at a real, named user.
func seedTeacher(ctx context.Context, identity *application.IdentityService) (domain.User, error) {
	teacher, err := identity.ResolveCaller(ctx, seedTeacherClerkUserID, "")
	if err == nil {
		return teacher, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return domain.User{}, fmt.Errorf("look up seed teacher: %w", err)
	}
	teacher, err = identity.RegisterUser(ctx, seedTeacherClerkUserID, domain.RoleTeacher, "en", "Tomás Ribeiro")
	if err != nil {
		return domain.User{}, fmt.Errorf("register seed teacher: %w", err)
	}
	return teacher, nil
}

func findFirstStudent(ctx context.Context, client *ent.Client) (domain.User, error) {
	row, err := client.User.Query().Where(user.RoleEQ(user.Role(domain.RoleStudent))).First(ctx)
	if err != nil {
		return domain.User{}, fmt.Errorf("find a registered student (sign in once through the SPA as a student first): %w", err)
	}
	return domain.User{
		ID:           row.ID.String(),
		ClerkUserID:  row.ClerkUserID,
		Role:         domain.Role(row.Role),
		DisplayName:  row.DisplayName,
		RegisteredAt: row.RegisteredAt,
	}, nil
}

func seedCompletionStatuses(ctx context.Context, db *mongo.Database, studentID string, statuses map[string]string) error {
	collection := db.Collection("aggregates")
	for contentNodeID, status := range statuses {
		_, err := collection.UpdateOne(ctx,
			bson.D{{Key: "student_id", Value: studentID}, {Key: "content_node_id", Value: contentNodeID}},
			bson.D{{Key: "$set", Value: bson.D{
				{Key: "student_id", Value: studentID},
				{Key: "content_node_id", Value: contentNodeID},
				{Key: "status", Value: status},
			}}},
			options.UpdateOne().SetUpsert(true),
		)
		if err != nil {
			return err
		}
	}
	return nil
}
