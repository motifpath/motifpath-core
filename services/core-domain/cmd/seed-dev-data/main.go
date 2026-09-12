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
	"fmt"
	"log"
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
	assignmentRepo := repo.NewEntPathAssignmentRepository(entClient)

	newID := uuid.NewString
	now := func() time.Time { return time.Now().UTC() }

	contentService := application.NewContentService(nodeRepo, expandedRepo, newID, now)
	pathService := application.NewLearningPathService(nodeRepo, pathRepo, newID, now)
	assignmentService := application.NewPathAssignmentService(userRepo, pathRepo, assignmentRepo, nil, newID, now)

	student, err := findFirstStudent(ctx, entClient)
	if err != nil {
		return err
	}
	log.Printf("seeding a path for existing student %s (clerk_user_id=%s)", student.ID, student.ClerkUserID)

	teacher := domain.User{ID: newID(), Role: domain.RoleTeacher}

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

	var items []application.PathItemInput
	var nodeIDs []string
	for _, spec := range specs {
		node, err := contentService.CreateContentNode(ctx, teacher, spec.title, domain.ContentTypeVideo, spec.skill, spec.concept, spec.difficulty)
		if err != nil {
			return fmt.Errorf("create content node %q: %w", spec.title, err)
		}
		section := spec.section
		items = append(items, application.PathItemInput{ContentNodeID: node.ID, SectionLabel: &section})
		nodeIDs = append(nodeIDs, node.ID)
	}

	path, err := pathService.CreateLearningPath(ctx, teacher, "Blues Guitar Foundations", items)
	if err != nil {
		return fmt.Errorf("create learning path: %w", err)
	}
	log.Printf("created learning path %s with %d items", path.ID, len(path.Items))

	if _, err := assignmentService.AssignLearningPath(ctx, teacher, student.ID, path.ID); err != nil {
		return fmt.Errorf("assign learning path: %w", err)
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
	if err := seedCompletionStatuses(ctx, mongoClient.Database(mongoDatabase), student.ID, statuses); err != nil {
		return fmt.Errorf("seed completion statuses: %w", err)
	}
	log.Printf("seeded %d completion statuses in MongoDB aggregates", len(statuses))

	log.Println("done — reload the SPA's /path view to see it")
	return nil
}

func findFirstStudent(ctx context.Context, client *ent.Client) (domain.User, error) {
	row, err := client.User.Query().First(ctx)
	if err != nil {
		return domain.User{}, fmt.Errorf("find a registered user (sign in once through the SPA first): %w", err)
	}
	return domain.User{
		ID:           row.ID.String(),
		ClerkUserID:  row.ClerkUserID,
		Role:         domain.Role(row.Role),
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
