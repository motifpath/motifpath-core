// Command seed-full populates a fresh local dev database with a broad,
// self-contained combination matrix of MotifPath's domain state — every
// CourseStatus, every CourseEnrollmentStatus, standalone paths both current
// and archived, and every ExerciseType — rather than the single happy-path
// student `seed-dev-data` seeds. It creates its own synthetic teacher and
// student users (fake clerk_user_id values, never real Clerk identities),
// so it never depends on anyone having signed in through the SPA first —
// meant to run right after a schema reset (`make db:reset`), reusing the
// same application services and domain constructors the HTTP handlers use,
// not raw SQL, so every invariant they enforce holds exactly as it would
// via the real API.
//
// If ADMIN_CLERK_USER_ID is set and no user with that clerk_user_id exists
// yet, registers it as an admin — lets each teammate bootstrap their own
// real Clerk identity as admin after a reset without hardcoding anyone's
// personal id here. Already-existing users (e.g. restored from a backup
// taken before the reset) are left untouched.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"net/url"
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
	"github.com/motifpath/core-domain/internal/ports"
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

// services bundles every application service seeding needs — passed around
// as one value rather than a long, repeated parameter list.
type services struct {
	identity    *application.IdentityService
	content     *application.ContentService
	path        *application.LearningPathService
	studentPath *application.StudentPathService
	course      *application.CourseService
	enrollment  *application.CourseEnrollmentService
	challenge   *application.ChallengeService
	exercise    *application.ExerciseService
	skill       *application.SkillService
	concept     *application.ConceptService
}

func run() error {
	ctx := context.Background()

	resources, closeResources, err := connectResources(ctx)
	if err != nil {
		return err
	}
	defer closeResources()

	svc, deps := wireServices(resources)

	admin, adminIsFresh, err := ensureAdmin(ctx, resources.entClient, deps.userRepo, deps.newID, deps.now)
	if err != nil {
		return fmt.Errorf("ensure admin: %w", err)
	}

	return seedAll(ctx, svc, deps, resources, admin, adminIsFresh)
}

// resources bundles the raw connections and their ent/mongo clients —
// everything connectResources opens and wireServices' repositories are
// built from.
type resources struct {
	entClient *ent.Client
	mongoDB   *mongo.Database
}

// connectResources opens the Postgres and MongoDB connections seeding
// needs, returning a single cleanup func that closes both in order rather
// than requiring the caller to manage two separate defers.
func connectResources(ctx context.Context) (resources, func(), error) {
	databaseURL := getenvDefault("DATABASE_URL", "postgres://motifpath:motifpath@localhost:5432/core_domain?sslmode=disable")
	mongoURI := getenvDefault("MONGO_URI", "mongodb://motifpath:motifpath@localhost:27017/?authSource=admin")
	mongoDatabase := getenvDefault("MONGO_DATABASE", "motifpath_events")

	sqlDB, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return resources{}, nil, fmt.Errorf("connect to postgres: %w", err)
	}
	entClient := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, sqlDB)))

	mongoClient, err := mongo.Connect(options.Client().ApplyURI(mongoURI))
	if err != nil {
		if closeErr := sqlDB.Close(); closeErr != nil {
			log.Printf("failed to close postgres connection: %v", closeErr)
		}
		return resources{}, nil, fmt.Errorf("connect to mongodb: %w", err)
	}

	closeFn := func() {
		if closeErr := entClient.Close(); closeErr != nil {
			log.Printf("failed to close ent client: %v", closeErr)
		}
		if closeErr := mongoClient.Disconnect(ctx); closeErr != nil {
			log.Printf("failed to disconnect mongodb client: %v", closeErr)
		}
	}

	return resources{entClient: entClient, mongoDB: mongoClient.Database(mongoDatabase)}, closeFn, nil
}

// seedDeps bundles the repositories and id/time helpers seedAll's steps
// need directly, alongside the application services already bundled in
// services.
type seedDeps struct {
	userRepo             ports.UserRepository
	courseEnrollmentRepo *repo.EntCourseEnrollmentRepository
	newID                func() string
	now                  func() time.Time
	teacher              domain.User
	classifier           *classificationSeeder
}

// wireServices builds every repository and application service seeding
// needs from an already-open resources, and the shared seedDeps values
// (id/time generators, the throwaway authoring "teacher" identity, and the
// skill/concept classifier) every seed step below is threaded through.
func wireServices(res resources) (services, seedDeps) {
	entClient := res.entClient
	userRepo := repo.NewEntUserRepository(entClient)
	languageRepo := repo.NewEntLanguageRepository(entClient)
	nodeRepo := repo.NewEntContentNodeRepository(entClient)
	expandedRepo := repo.NewEntExpandedContentRepository(entClient)
	pathRepo := repo.NewEntLearningPathRepository(entClient)
	studentPathRepo := repo.NewEntStudentPathRepository(entClient)
	contentNodeVersionRepo := repo.NewEntContentNodeVersionRepository(entClient)
	studentLearningStateRepo := repo.NewEntStudentLearningStateRepository(entClient)
	courseRepo := repo.NewEntCourseRepository(entClient)
	courseVersionRepo := repo.NewEntCourseVersionRepository(entClient)
	courseEnrollmentRepo := repo.NewEntCourseEnrollmentRepository(entClient)
	challengeRepo := repo.NewEntChallengeRepository(entClient)
	exerciseRepo := repo.NewEntExerciseRepository(entClient)
	skillRepo := repo.NewEntSkillRepository(entClient)
	conceptRepo := repo.NewEntConceptRepository(entClient)
	diagramRepo := repo.NewEntDiagramRepository(entClient)

	newID := uuid.NewString
	now := func() time.Time { return time.Now().UTC() }

	// A real completion reader, not nil: unlike seed-dev-data (which only
	// ever assigns paths), this script also switches current pointers
	// through SetCurrentPath, and resolving a course-enrollment pointer
	// always runs checkAndAdvanceCheckpoint — which needs a working
	// CompletionStateReader even when nothing is actually completed yet.
	completionReader := repo.NewMongoCompletionStateReader(res.mongoDB)

	studentPathService := application.NewStudentPathService(userRepo, pathRepo, studentPathRepo, contentNodeVersionRepo, studentLearningStateRepo, courseEnrollmentRepo, courseVersionRepo, nodeRepo, exerciseRepo, completionReader, newID, now)

	svc := services{
		identity:    application.NewIdentityService(userRepo, languageRepo, newID, now),
		content:     application.NewContentService(nodeRepo, expandedRepo, skillRepo, conceptRepo, contentNodeVersionRepo, diagramRepo, newID, now),
		path:        application.NewLearningPathService(nodeRepo, pathRepo, courseVersionRepo, newID, now),
		studentPath: studentPathService,
		course:      application.NewCourseService(pathRepo, courseRepo, courseVersionRepo, newID, now),
		enrollment:  application.NewCourseEnrollmentService(courseRepo, courseVersionRepo, pathRepo, studentPathRepo, courseEnrollmentRepo, studentPathService, studentLearningStateRepo, completionReader, newID, now),
		challenge:   application.NewChallengeService(nodeRepo, challengeRepo, exerciseRepo, newID, now),
		exercise:    application.NewExerciseService(challengeRepo, exerciseRepo, nodeRepo, skillRepo, conceptRepo, diagramRepo, newID, now, rand.Shuffle),
		skill:       application.NewSkillService(skillRepo, newID),
		concept:     application.NewConceptService(conceptRepo, newID),
	}

	teacher := domain.User{ID: newID(), Role: domain.RoleTeacher}
	return svc, seedDeps{
		userRepo:             userRepo,
		courseEnrollmentRepo: courseEnrollmentRepo,
		newID:                newID,
		now:                  now,
		teacher:              teacher,
		classifier:           &classificationSeeder{skills: svc.skill, concepts: svc.concept, teacher: teacher},
	}
}

// seedAll runs every seeding step in dependency order, logging a one-line
// summary after each.
func seedAll(ctx context.Context, svc services, deps seedDeps, res resources, admin domain.User, adminIsFresh bool) error {
	teacher, classifier := deps.teacher, deps.classifier

	students, err := seedStudents(ctx, svc.identity)
	if err != nil {
		return fmt.Errorf("seed students: %w", err)
	}
	log.Printf("seeded %d synthetic students", len(students))

	nodes, err := seedContentNodes(ctx, teacher, svc.content, classifier)
	if err != nil {
		return fmt.Errorf("seed content nodes: %w", err)
	}
	log.Printf("seeded %d content nodes (video + article) across every difficulty level", len(nodes))

	videoIntermediateChallenge, err := seedExercisesAllTypes(ctx, teacher, svc.challenge, svc.exercise, classifier, nodes)
	if err != nil {
		return fmt.Errorf("seed exercises: %w", err)
	}
	log.Println("seeded one exercise of every exercise_type, linked to a challenge")

	templateA, err := svc.path.CreateLearningPath(ctx, teacher, "Open Position Foundations", []application.PathItemInput{
		{ContentNodeID: nodes["video-beginner"].ID},
		{ContentNodeID: nodes["article-beginner"].ID},
	})
	if err != nil {
		return fmt.Errorf("create template A: %w", err)
	}
	templateB, err := svc.path.CreateLearningPath(ctx, teacher, "Improvisation Essentials", []application.PathItemInput{
		{ContentNodeID: nodes["video-intermediate"].ID},
		{ContentNodeID: nodes["article-advanced"].ID},
	})
	if err != nil {
		return fmt.Errorf("create template B: %w", err)
	}
	log.Printf("seeded 2 learning path templates: %q, %q", templateA.Title, templateB.Title)

	courses, err := seedCourses(ctx, teacher, svc.course, templateA.ID, templateB.ID)
	if err != nil {
		return fmt.Errorf("seed courses: %w", err)
	}
	log.Printf("seeded courses: draft=%s published(2 checkpoints)=%s single-checkpoint=%s retired=%s",
		courses.draft.ID, courses.published.ID, courses.single.ID, courses.retired.ID)

	brunoEnrollmentID, err := seedEnrollments(ctx, svc, deps.courseEnrollmentRepo, students, courses, nodes, res.mongoDB)
	if err != nil {
		return fmt.Errorf("seed enrollments: %w", err)
	}
	log.Println("seeded enrollments: active@checkpoint1, active@checkpoint2, completed, abandoned")

	if err := seedStandalonePaths(ctx, svc, students, templateA.ID, templateB.ID, brunoEnrollmentID, nodes, res.mongoDB); err != nil {
		return fmt.Errorf("seed standalone paths: %w", err)
	}
	log.Println("seeded standalone paths: current, and archived-while-course-active")

	if adminIsFresh {
		if err := seedAdminZeroUser(ctx, svc, deps, admin, courses, nodes, videoIntermediateChallenge.ID, res.mongoDB); err != nil {
			return fmt.Errorf("seed admin zero-user state: %w", err)
		}
		log.Printf("seeded a course enrollment and a standalone path (with a completed, playable node) for admin zero-user %s", admin.ID)
	}

	log.Println("done")
	return nil
}

// ensureAdmin creates ADMIN_CLERK_USER_ID (if set) as an admin, but only
// when no user with that clerk_user_id already exists — running this
// against a database where that identity was already restored from a
// pre-reset backup must be a no-op, never a duplicate-key failure. Goes
// through userRepo directly rather than IdentityService.RegisterUser:
// domain.NewUser deliberately refuses to construct a User with role admin
// (self-registration can never grant admin — promotion is always an
// out-of-band operation), so this mirrors what a hand-run SQL insert would
// do instead.
//
// Returns the admin User (zero value if ADMIN_CLERK_USER_ID isn't set) and
// whether it was created by this call — seedAll only gives this identity a
// course enrollment and standalone path (its "zero-user" state) when it was
// just created, never when it's a restored pre-reset identity that may
// already carry real progress of its own.
func ensureAdmin(ctx context.Context, client *ent.Client, userRepo ports.UserRepository, newID func() string, now func() time.Time) (domain.User, bool, error) {
	clerkUserID := os.Getenv("ADMIN_CLERK_USER_ID")
	if clerkUserID == "" {
		log.Println("ADMIN_CLERK_USER_ID not set — skipping admin bootstrap; restore your own admin row separately")
		return domain.User{}, false, nil
	}
	row, err := client.User.Query().Where(user.ClerkUserIDEQ(clerkUserID)).Only(ctx)
	if err == nil {
		log.Printf("admin %s already present — leaving it untouched", clerkUserID)
		return domain.User{
			ID:           row.ID.String(),
			ClerkUserID:  row.ClerkUserID,
			Role:         domain.Role(row.Role),
			RegisteredAt: row.RegisteredAt,
		}, false, nil
	}
	if !ent.IsNotFound(err) {
		return domain.User{}, false, err
	}
	admin := domain.User{
		ID:           newID(),
		ClerkUserID:  clerkUserID,
		Role:         domain.RoleAdmin,
		Locale:       domain.Language{Code: "en"},
		RegisteredAt: now(),
	}
	if err := userRepo.Create(ctx, admin); err != nil {
		return domain.User{}, false, fmt.Errorf("create admin %s: %w", clerkUserID, err)
	}
	log.Printf("created %s as admin", clerkUserID)
	return admin, true, nil
}

// classificationSeeder resolves plain skill/concept names to real Skill/
// Concept tree node ids, creating a fresh root node the first time a given
// name is seen in this run and reusing it thereafter. Mirrors
// seed-dev-data's identically-named helper — this script isn't the API, so
// a same-run cache is enough for the known, disjoint set of root names it
// controls. Re-running against a database that already has these root
// names fails on the sibling-uniqueness check, same limitation as
// seed-dev-data: this tool targets a freshly-reset database.
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

// seedStudentSpec is one synthetic student this script registers — a fake
// clerk_user_id, never a real Clerk identity.
type seedStudentSpec struct {
	key         string // stable lookup key into the returned map
	clerkUserID string
}

func seedStudents(ctx context.Context, identity *application.IdentityService) (map[string]domain.User, error) {
	specs := []seedStudentSpec{
		{"alice", "seed_student_alice"},
		{"bruno", "seed_student_bruno"},
		{"carla", "seed_student_carla"},
	}
	result := make(map[string]domain.User, len(specs))
	for _, spec := range specs {
		u, err := identity.RegisterUser(ctx, spec.clerkUserID, domain.RoleStudent, "en")
		if err != nil {
			return nil, fmt.Errorf("register student %q: %w", spec.key, err)
		}
		result[spec.key] = u
	}
	return result, nil
}

// seedContentNodes creates one video and one article node at a spread of
// difficulty levels, keyed by a descriptive lookup key — every content
// node created here is published immediately, since every downstream
// consumer (learning path items, course checkpoints) needs a published
// node to resolve.
func seedContentNodes(ctx context.Context, teacher domain.User, content *application.ContentService, classifier *classificationSeeder) (map[string]domain.ContentNode, error) {
	// A real, publicly reachable sample video — cdn.motifpath.io doesn't
	// resolve to anything, so a node seeded with it can never actually play.
	// Matches the sample host cmd/seed-lesson-content already relies on.
	seedVideoURL := "https://samplelib.com/lib/preview/mp4/sample-10s.mp4"
	type spec struct {
		key         string
		title       string
		contentType domain.ContentType
		difficulty  domain.DifficultyLevel
		skill       string
		concept     string
	}
	specs := []spec{
		{"video-beginner", "Open position C major scale", domain.ContentTypeVideo, domain.DifficultyLevelBeginner, "Scales", "Major scale fingerings"},
		{"article-beginner", "Reading a chord chart", domain.ContentTypeArticle, domain.DifficultyLevelBeginner, "Reading", "Chord chart notation"},
		{"video-intermediate", "Call-and-response phrasing", domain.ContentTypeVideo, domain.DifficultyLevelIntermediate, "Improvisation", "Phrasing"},
		{"article-advanced", "Modal interchange in blues turnarounds", domain.ContentTypeArticle, domain.DifficultyLevelAdvanced, "Harmony", "Modal interchange"},
	}

	result := make(map[string]domain.ContentNode, len(specs))
	for _, s := range specs {
		skillID, err := classifier.skillID(ctx, s.skill)
		if err != nil {
			return nil, err
		}
		conceptID, err := classifier.conceptID(ctx, s.concept)
		if err != nil {
			return nil, err
		}
		var mediaURL *string
		var richContent *domain.PromptDocument
		if s.contentType == domain.ContentTypeVideo {
			mediaURL = &seedVideoURL
		} else {
			doc := domain.NewPlainTextPrompt("Seed placeholder body for " + s.title + ".")
			richContent = &doc
		}
		node, err := content.CreateContentNode(ctx, teacher, s.title, s.contentType, []string{skillID}, []string{conceptID}, s.difficulty, []string{"en"}, mediaURL, richContent)
		if err != nil {
			return nil, fmt.Errorf("create content node %q: %w", s.title, err)
		}
		if _, err := content.PublishContentNode(ctx, teacher, node.ID); err != nil {
			return nil, fmt.Errorf("publish content node %q: %w", s.title, err)
		}
		result[s.key] = node
	}
	return result, nil
}

// seedExercisesAllTypes creates one exercise of every domain.ExerciseType,
// each carrying the option shape that type requires (region for
// image_recognition, per-option image/audio for image_choice/
// audio_selection, a text label otherwise), and links each into a shared
// practice challenge on the video-intermediate node.
func seedExercisesAllTypes(ctx context.Context, teacher domain.User, challengeSvc *application.ChallengeService, exerciseSvc *application.ExerciseService, classifier *classificationSeeder, nodes map[string]domain.ContentNode) (domain.Challenge, error) {
	// subject_skill_id must be one of the parent content node's own linked
	// skills — video-intermediate was seeded with "Improvisation" in
	// seedContentNodes, so the challenge's subject reuses that same id
	// rather than an unrelated skill.
	subjectSkillID, err := classifier.skillID(ctx, "Improvisation")
	if err != nil {
		return domain.Challenge{}, err
	}
	challenge, err := challengeSvc.CreateChallenge(ctx, teacher, nodes["video-intermediate"].ID, &subjectSkillID, nil, 70, nil, false, false)
	if err != nil {
		return domain.Challenge{}, fmt.Errorf("create shared practice challenge: %w", err)
	}

	skillID, err := classifier.skillID(ctx, "Ear training")
	if err != nil {
		return domain.Challenge{}, err
	}
	conceptID, err := classifier.conceptID(ctx, "Interval recognition")
	if err != nil {
		return domain.Challenge{}, err
	}
	label := func(s string) *string { return &s }
	// Real, publicly reachable sample media — see seedContentNodes' video URL
	// comment above for why cdn.motifpath.io can't be used here.
	imageURL := "https://placehold.co/640x360/png?text=Fretboard"
	audioURL := "https://samplelib.com/lib/preview/mp3/sample-3s.mp3"

	type spec struct {
		title        string
		exerciseType domain.ExerciseType
		imageURL     *string
		audioURL     *string
		options      []domain.Option
	}
	specs := []spec{
		{
			title: "Name the interval — text response", exerciseType: domain.ExerciseTypeTextResponse,
			options: []domain.Option{
				{ID: uuid.NewString(), IsCorrect: true, Label: label("Perfect fifth")},
				{ID: uuid.NewString(), IsCorrect: false, Label: label("Major third")},
			},
		},
		{
			title: "Identify the recorded interval", exerciseType: domain.ExerciseTypeAudioRecognition, audioURL: &audioURL,
			options: []domain.Option{
				{ID: uuid.NewString(), IsCorrect: true, Label: label("Minor third")},
				{ID: uuid.NewString(), IsCorrect: false, Label: label("Major sixth")},
			},
		},
		{
			title: "Tap the root note on the fretboard", exerciseType: domain.ExerciseTypeImageRecognition, imageURL: &imageURL,
			// Four regions, one per image quadrant, each 18% of the image's
			// width/height — big, easy-to-hit tap targets rather than the
			// tiny 6% markers a real diagram's precise hotspots would use.
			options: []domain.Option{
				{ID: uuid.NewString(), IsCorrect: true, Region: &domain.OptionRegion{X: 0.10, Y: 0.15, Width: 0.18, Height: 0.18, Shape: domain.OptionRegionShapeCircle}},
				{ID: uuid.NewString(), IsCorrect: false, Region: &domain.OptionRegion{X: 0.62, Y: 0.15, Width: 0.18, Height: 0.18, Shape: domain.OptionRegionShapeCircle}},
				{ID: uuid.NewString(), IsCorrect: false, Region: &domain.OptionRegion{X: 0.10, Y: 0.60, Width: 0.18, Height: 0.18, Shape: domain.OptionRegionShapeCircle}},
				{ID: uuid.NewString(), IsCorrect: false, Region: &domain.OptionRegion{X: 0.62, Y: 0.60, Width: 0.18, Height: 0.18, Shape: domain.OptionRegionShapeCircle}},
			},
		},
		{
			title: "Pick the matching chord shape", exerciseType: domain.ExerciseTypeImageChoice,
			options: []domain.Option{
				{ID: uuid.NewString(), IsCorrect: true, ImageURL: &imageURL},
				{ID: uuid.NewString(), IsCorrect: false, ImageURL: &imageURL},
			},
		},
		{
			title: "Pick the matching recorded lick", exerciseType: domain.ExerciseTypeAudioSelection,
			options: []domain.Option{
				{ID: uuid.NewString(), IsCorrect: true, AudioURL: &audioURL},
				{ID: uuid.NewString(), IsCorrect: false, AudioURL: &audioURL},
			},
		},
	}

	for _, s := range specs {
		exercise, err := exerciseSvc.CreateExercise(ctx, teacher, s.title, domain.NewPlainTextPrompt(s.title), s.exerciseType,
			[]string{skillID}, []string{conceptID}, s.imageURL, s.audioURL, nil, nil, s.options, nil, nil, []string{"en"})
		if err != nil {
			return domain.Challenge{}, fmt.Errorf("create %s exercise: %w", s.exerciseType, err)
		}
		if _, err := exerciseSvc.LinkExerciseToChallenge(ctx, teacher, challenge.ID, exercise.ID); err != nil {
			return domain.Challenge{}, fmt.Errorf("link %s exercise to challenge: %w", s.exerciseType, err)
		}
	}
	return challenge, nil
}

// seededCourses is the four courses seedCourses creates, one per
// domain.CourseStatus plus a second published course kept to a single
// checkpoint for scenarios that want a course to complete or abandon
// quickly.
type seededCourses struct {
	draft     domain.Course
	published domain.Course
	single    domain.Course
	retired   domain.Course
}

func seedCourses(ctx context.Context, teacher domain.User, courseSvc *application.CourseService, templateAID, templateBID string) (seededCourses, error) {
	admin := domain.User{ID: uuid.NewString(), Role: domain.RoleAdmin}

	draft, err := courseSvc.CreateCourse(ctx, teacher, "Draft Course — Never Published", "A course still being authored.", domain.DifficultyLevelBeginner,
		[]application.CheckpointInput{{LearningPathID: templateAID}})
	if err != nil {
		return seededCourses{}, fmt.Errorf("create draft course: %w", err)
	}

	published, err := courseSvc.CreateCourse(ctx, teacher, "Fingerstyle Foundations", "Two checkpoints, from open position to improvisation.", domain.DifficultyLevelBeginner,
		[]application.CheckpointInput{{LearningPathID: templateAID}, {LearningPathID: templateBID}})
	if err != nil {
		return seededCourses{}, fmt.Errorf("create published course: %w", err)
	}
	if _, err := courseSvc.PublishCourse(ctx, admin, published.ID); err != nil {
		return seededCourses{}, fmt.Errorf("publish published course: %w", err)
	}

	single, err := courseSvc.CreateCourse(ctx, teacher, "Rhythm Basics", "One checkpoint, for quick complete/abandon scenarios.", domain.DifficultyLevelBeginner,
		[]application.CheckpointInput{{LearningPathID: templateBID}})
	if err != nil {
		return seededCourses{}, fmt.Errorf("create single-checkpoint course: %w", err)
	}
	if _, err := courseSvc.PublishCourse(ctx, admin, single.ID); err != nil {
		return seededCourses{}, fmt.Errorf("publish single-checkpoint course: %w", err)
	}

	retired, err := courseSvc.CreateCourse(ctx, teacher, "Retired Classics", "Published once, then retired.", domain.DifficultyLevelBeginner,
		[]application.CheckpointInput{{LearningPathID: templateAID}})
	if err != nil {
		return seededCourses{}, fmt.Errorf("create retired course: %w", err)
	}
	if _, err := courseSvc.PublishCourse(ctx, admin, retired.ID); err != nil {
		return seededCourses{}, fmt.Errorf("publish retired course (before retiring): %w", err)
	}
	if _, err := courseSvc.RetireCourse(ctx, admin, retired.ID); err != nil {
		return seededCourses{}, fmt.Errorf("retire retired course: %w", err)
	}

	return seededCourses{draft: draft, published: published, single: single, retired: retired}, nil
}

// seedEnrollments drives every domain.CourseEnrollmentStatus, plus an
// active enrollment past its first checkpoint: alice ends active at
// checkpoint 1, with checkpoint 1's first item already completed so her
// path isn't shown as entirely untouched; bruno self-enrolls then is
// advanced straight to checkpoint 2 (bypassing the real completion-discovery
// flow, which needs a real CompletionStateReader this script doesn't wire
// up — the resulting row shape is identical to what
// checkAndAdvanceCheckpoint would have produced, just reached directly);
// carla's enrollment is marked completed; alice also picks up a second,
// abandoned enrollment in the single-checkpoint course, so her own
// enrollment list alone already shows active + abandoned.
func seedEnrollments(ctx context.Context, svc services, enrollmentRepo *repo.EntCourseEnrollmentRepository, students map[string]domain.User, courses seededCourses, nodes map[string]domain.ContentNode, mongoDB *mongo.Database) (string, error) {
	aliceCtx := students["alice"]
	brunoCtx := students["bruno"]
	carlaCtx := students["carla"]

	if _, err := svc.enrollment.CreateCourseEnrollment(ctx, aliceCtx, courses.published.ID); err != nil {
		return "", fmt.Errorf("enroll alice in published course: %w", err)
	}
	// checkpoint 1 is templateA, whose first item is video-beginner.
	if err := seedCompletionStatuses(ctx, mongoDB, aliceCtx.ID, map[string]string{
		nodes["video-beginner"].ID: "completed",
	}); err != nil {
		return "", err
	}

	brunoEnrollment, err := svc.enrollment.CreateCourseEnrollment(ctx, brunoCtx, courses.published.ID)
	if err != nil {
		return "", fmt.Errorf("enroll bruno in published course: %w", err)
	}
	latestPublished, err := svc.course.LatestVersion(ctx, courses.published.ID)
	if err != nil {
		return "", fmt.Errorf("resolve published course's latest version: %w", err)
	}
	teacher := domain.User{ID: uuid.NewString(), Role: domain.RoleTeacher}
	template, err := resolveCheckpointTemplate(ctx, svc, teacher, latestPublished, 2)
	if err != nil {
		return "", fmt.Errorf("resolve checkpoint 2 template: %w", err)
	}
	checkpoint2Path, err := svc.studentPath.CopyTemplateForCheckpoint(ctx, brunoCtx.ID, template, brunoCtx.ID, brunoEnrollment.ID, 2)
	if err != nil {
		return "", fmt.Errorf("copy checkpoint 2 template for bruno: %w", err)
	}
	if err := enrollmentRepo.AdvanceCheckpoint(ctx, brunoEnrollment.ID, checkpoint2Path.ID, 2); err != nil {
		return "", fmt.Errorf("advance bruno to checkpoint 2: %w", err)
	}

	carlaEnrollment, err := svc.enrollment.CreateCourseEnrollment(ctx, carlaCtx, courses.single.ID)
	if err != nil {
		return "", fmt.Errorf("enroll carla in single-checkpoint course: %w", err)
	}
	if err := enrollmentRepo.Complete(ctx, carlaEnrollment.ID); err != nil {
		return "", fmt.Errorf("mark carla's enrollment completed: %w", err)
	}

	aliceSecondEnrollment, err := svc.enrollment.CreateCourseEnrollment(ctx, aliceCtx, courses.single.ID)
	if err != nil {
		return "", fmt.Errorf("enroll alice in single-checkpoint course: %w", err)
	}
	if err := enrollmentRepo.Abandon(ctx, aliceSecondEnrollment.ID); err != nil {
		return "", fmt.Errorf("mark alice's second enrollment abandoned: %w", err)
	}

	return brunoEnrollment.ID, nil
}

// resolveCheckpointTemplate looks up the LearningPath template behind
// version's checkpoint at position.
func resolveCheckpointTemplate(ctx context.Context, svc services, caller domain.User, version domain.CourseVersion, position int) (domain.LearningPath, error) {
	for _, cp := range version.Checkpoints {
		if cp.Position == position {
			return svc.path.GetLearningPath(ctx, caller, cp.LearningPathID)
		}
	}
	return domain.LearningPath{}, fmt.Errorf("no checkpoint at position %d", position)
}

// seedStandalonePaths gives carla a current standalone path — with its
// first item already completed, so it isn't shown as entirely untouched —
// alongside her already-completed course enrollment, and gives bruno a
// standalone path that ends up archived while his course enrollment is
// current — assigning always sets current unconditionally, so bruno's path
// is switched back off current before archiving it, exercising the same
// "archive a non-current path" path the application layer's conflict guard
// allows unconditionally.
func seedStandalonePaths(ctx context.Context, svc services, students map[string]domain.User, templateAID, templateBID, brunoEnrollmentID string, nodes map[string]domain.ContentNode, mongoDB *mongo.Database) error {
	teacher := domain.User{ID: uuid.NewString(), Role: domain.RoleTeacher}

	carlaCtx := students["carla"]
	templateA, err := svc.path.GetLearningPath(ctx, teacher, templateAID)
	if err != nil {
		return fmt.Errorf("resolve template A: %w", err)
	}
	if _, err := svc.studentPath.AssignLearningPath(ctx, teacher, carlaCtx.ID, templateA.ID); err != nil {
		return fmt.Errorf("assign standalone path to carla: %w", err)
	}

	brunoCtx := students["bruno"]
	templateB, err := svc.path.GetLearningPath(ctx, teacher, templateBID)
	if err != nil {
		return fmt.Errorf("resolve template B: %w", err)
	}
	brunoStandalone, err := svc.studentPath.AssignLearningPath(ctx, teacher, brunoCtx.ID, templateB.ID)
	if err != nil {
		return fmt.Errorf("assign standalone path to bruno: %w", err)
	}

	if _, err := svc.studentPath.SetCurrentPath(ctx, brunoCtx, application.SetCurrentPathInput{CourseEnrollmentID: &brunoEnrollmentID}); err != nil {
		return fmt.Errorf("switch bruno's current back to his course enrollment: %w", err)
	}
	if _, err := svc.studentPath.ArchiveStandaloneStudentPath(ctx, brunoCtx, brunoStandalone.ID); err != nil {
		return fmt.Errorf("archive bruno's now-non-current standalone path: %w", err)
	}

	// carla's standalone path is templateA, whose first item is
	// video-beginner.
	return seedCompletionStatuses(ctx, mongoDB, carlaCtx.ID, map[string]string{
		nodes["video-beginner"].ID: "completed",
	})
}

// seedAdminZeroUser enrolls the freshly-bootstrapped admin
// (ADMIN_CLERK_USER_ID) in a course and a standalone path exactly like the
// synthetic students — so signing in as yourself after a reset shows real,
// populated state instead of an empty dashboard.
//
// Completion is tracked per (student, content node), not per path — so the
// course enrollment's checkpoint deliberately gets no completion status of
// its own here, even though "enrolled in a course" is otherwise all this
// step needs: video-intermediate is also the standalone path's second item
// below, and marking it completed in one context would silently mark it
// completed in the other too, leaving nothing "current" to open there.
//
// The standalone path is built from video-beginner and video-intermediate
// only — never an article — and both get timed cues plus their own
// practice challenge. templateA/B (used by the synthetic students) always
// put an article second, which the lesson screen has no view for yet; once
// that article's predecessor is marked completed it becomes "current" and
// opening it fails with "this lesson isn't available yet". Marking one item
// of a two-video path completed never has that problem, and the node that
// ends up completed, and the one that ends up current, both have something
// to watch and practice instead of an empty lesson screen.
//
// Every exercise seeded anywhere (video-intermediate's own 5 plus
// video-beginner's 2) also gets linked into video-intermediate's challenge,
// on top of whatever challenge it was originally created for — so the admin
// zero-user's current lesson always has the full practice pool available,
// not just the handful seeded specifically for it.
func seedAdminZeroUser(ctx context.Context, svc services, deps seedDeps, admin domain.User, courses seededCourses, nodes map[string]domain.ContentNode, videoIntermediateChallengeID string, mongoDB *mongo.Database) error {
	teacher, classifier := deps.teacher, deps.classifier

	if _, err := svc.enrollment.CreateCourseEnrollment(ctx, admin, courses.single.ID); err != nil {
		return fmt.Errorf("enroll admin in single-checkpoint course: %w", err)
	}

	if err := seedVideoBeginnerChallenge(ctx, teacher, svc.challenge, svc.exercise, classifier, nodes["video-beginner"]); err != nil {
		return fmt.Errorf("seed video-beginner's own practice challenge: %w", err)
	}
	if err := seedTimedCues(ctx, svc.content, teacher, nodes["video-beginner"].ID, "Open position C major scale"); err != nil {
		return fmt.Errorf("seed timed cues for video-beginner: %w", err)
	}
	if err := seedTimedCues(ctx, svc.content, teacher, nodes["video-intermediate"].ID, "Call-and-response phrasing"); err != nil {
		return fmt.Errorf("seed timed cues for video-intermediate: %w", err)
	}
	if err := linkAllExercisesToChallenge(ctx, teacher, svc.exercise, videoIntermediateChallengeID); err != nil {
		return fmt.Errorf("link every exercise to video-intermediate's challenge: %w", err)
	}

	adminPath, err := svc.path.CreateLearningPath(ctx, teacher, "Admin Zero-User Path", []application.PathItemInput{
		{ContentNodeID: nodes["video-beginner"].ID},
		{ContentNodeID: nodes["video-intermediate"].ID},
	})
	if err != nil {
		return fmt.Errorf("create admin's standalone path: %w", err)
	}
	if _, err := svc.studentPath.AssignLearningPath(ctx, teacher, admin.ID, adminPath.ID); err != nil {
		return fmt.Errorf("assign standalone path to admin: %w", err)
	}
	return seedCompletionStatuses(ctx, mongoDB, admin.ID, map[string]string{
		nodes["video-beginner"].ID: "completed",
	})
}

// seedVideoBeginnerChallenge gives node its own practice challenge with two
// text_response exercises — mirroring the richer challenge
// seedExercisesAllTypes already gives video-intermediate, so whichever of
// the two ends up "current" for the admin zero-user always has exercises to
// practice, not just a video.
func seedVideoBeginnerChallenge(ctx context.Context, teacher domain.User, challengeSvc *application.ChallengeService, exerciseSvc *application.ExerciseService, classifier *classificationSeeder, node domain.ContentNode) error {
	// video-beginner was seeded with skill "Scales" in seedContentNodes, so
	// the challenge's subject reuses that same id rather than an unrelated
	// skill.
	subjectSkillID, err := classifier.skillID(ctx, "Scales")
	if err != nil {
		return err
	}
	challenge, err := challengeSvc.CreateChallenge(ctx, teacher, node.ID, &subjectSkillID, nil, 70, nil, false, false)
	if err != nil {
		return fmt.Errorf("create challenge: %w", err)
	}

	conceptID, err := classifier.conceptID(ctx, "Major scale fingerings")
	if err != nil {
		return err
	}
	label := func(s string) *string { return &s }
	specs := []struct {
		title   string
		options []domain.Option
	}{
		{
			title: "Which note is the root of a C major scale in open position?",
			options: []domain.Option{
				{ID: uuid.NewString(), IsCorrect: true, Label: label("C")},
				{ID: uuid.NewString(), IsCorrect: false, Label: label("G")},
			},
		},
		{
			title: "How many notes does a major scale have before it repeats an octave higher?",
			options: []domain.Option{
				{ID: uuid.NewString(), IsCorrect: true, Label: label("Seven")},
				{ID: uuid.NewString(), IsCorrect: false, Label: label("Five")},
			},
		},
	}
	for _, s := range specs {
		exercise, err := exerciseSvc.CreateExercise(ctx, teacher, s.title, domain.NewPlainTextPrompt(s.title), domain.ExerciseTypeTextResponse,
			[]string{subjectSkillID}, []string{conceptID}, nil, nil, nil, nil, s.options, nil, nil, []string{"en"})
		if err != nil {
			return fmt.Errorf("create exercise %q: %w", s.title, err)
		}
		if _, err := exerciseSvc.LinkExerciseToChallenge(ctx, teacher, challenge.ID, exercise.ID); err != nil {
			return fmt.Errorf("link exercise %q to challenge: %w", s.title, err)
		}
	}
	return nil
}

// seedTimedCues attaches two short image cues to a video content node, so
// its lesson screen has something in the timed-content rail instead of an
// empty one.
func seedTimedCues(ctx context.Context, contentSvc *application.ContentService, teacher domain.User, nodeID, caption string) error {
	cues := []struct{ start, end int }{
		{start: 2, end: 6},
		{start: 7, end: 10},
	}
	for i, cue := range cues {
		text := fmt.Sprintf("%s — cue %d", caption, i+1)
		mediaURL := "https://placehold.co/640x360/png?text=" + url.QueryEscape(text)
		start, end := cue.start, cue.end
		if _, err := contentSvc.CreateExpandedContent(ctx, teacher, nodeID, domain.ExpandedContentTypeImage, &mediaURL, nil, nil, nil, &start, &end, nil, nil, &text); err != nil {
			return fmt.Errorf("create cue %q: %w", text, err)
		}
	}
	return nil
}

// linkAllExercisesToChallenge links every exercise in the reusable pool to
// challengeID, skipping any already linked to it (LinkExerciseToChallenge
// refuses a duplicate link with domain.ErrAlreadyExists) — an exercise
// keeps every challenge it was already linked to; this only adds one more.
func linkAllExercisesToChallenge(ctx context.Context, teacher domain.User, exerciseSvc *application.ExerciseService, challengeID string) error {
	exercises, err := exerciseSvc.ListExercises(ctx, teacher, "", "")
	if err != nil {
		return fmt.Errorf("list exercises: %w", err)
	}
	for _, exercise := range exercises {
		alreadyLinked := false
		for _, id := range exercise.ChallengeIDs {
			if id == challengeID {
				alreadyLinked = true
				break
			}
		}
		if alreadyLinked {
			continue
		}
		if _, err := exerciseSvc.LinkExerciseToChallenge(ctx, teacher, challengeID, exercise.ID); err != nil {
			return fmt.Errorf("link exercise %s: %w", exercise.ID, err)
		}
	}
	return nil
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
