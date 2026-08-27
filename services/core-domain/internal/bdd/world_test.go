//go:build integration

package bdd

import (
	"context"
	"crypto/sha1" //nolint:gosec // used only for deterministic test UUIDs, not security
	"fmt"
	"time"

	"github.com/google/uuid"

	appHTTP "github.com/motifpath/core-domain/internal/adapters/http"
	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

var fixedNow = time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)

// world holds all state for a single scenario. A fresh instance is created
// by InitializeScenario for every scenario godog runs, giving each scenario
// full isolation without an explicit teardown step.
type world struct {
	users       *fakeUserRepo
	nodes       *fakeContentNodeRepo
	challenges  *fakeChallengeRepo
	exercises   *fakeExerciseRepo
	expanded    *fakeExpandedContentRepo
	paths       *fakeLearningPathRepo
	assignments *fakePathAssignmentRepo
	completion  *fakeCompletionReader
	handler     *appHTTP.Handler

	// userMotifID caches the server-generated user_id for each display name
	// once registered — needed because, unlike the deterministic ids used
	// for content nodes/challenges/etc., a user's MotifPath id can't be
	// derived from its display name; it's assigned by IdentityService.
	userMotifID map[string]uuid.UUID

	hasToken bool
	clerkSub string // the "sub" claim of whichever identity is currently authenticated

	lastResp any
	lastErr  error

	// multiCreateIDs collects the ids returned by a "creates three X" step,
	// for the "three distinct identifiers are returned" assertion shared by
	// the exercises and expanded-content features.
	multiCreateIDs []uuid.UUID

	// lastNodeSlug is the slug of the most recently established content
	// node ("a video/article content node ... exists"), used by
	// expanded-content steps whose Gherkin text doesn't repeat the slug —
	// they rely on the immediately preceding Given for node context.
	lastNodeSlug string
}

func newWorld() *world {
	w := &world{
		users:       newFakeUserRepo(),
		nodes:       newFakeContentNodeRepo(),
		challenges:  newFakeChallengeRepo(),
		exercises:   newFakeExerciseRepo(),
		expanded:    newFakeExpandedContentRepo(),
		paths:       newFakeLearningPathRepo(),
		assignments: newFakePathAssignmentRepo(),
		completion:  newFakeCompletionReader(),
		userMotifID: map[string]uuid.UUID{},
	}

	newID := idSequence()
	now := func() time.Time { return fixedNow }

	identity := application.NewIdentityService(w.users, newID, now)
	content := application.NewContentService(w.nodes, w.expanded, newID, now)
	challenge := application.NewChallengeService(w.nodes, w.challenges, w.exercises, newID, now)
	path := application.NewLearningPathService(w.nodes, w.paths, newID, now)
	assignment := application.NewPathAssignmentService(w.users, w.paths, w.assignments, w.completion, newID, now)

	w.handler = appHTTP.NewHandler(identity, content, challenge, path, assignment)
	return w
}

// idSequence returns a deterministic newID func producing real UUID
// strings — the HTTP mapping layer's mustUUID assumes every domain id is a
// valid UUID (true in production, where cmd/main.go wires uuid.NewString),
// so a plain "gen-N" placeholder would panic there. Distinct namespace
// ("gen") from the deterministic content/challenge/exercise ids derived
// from Gherkin slugs, so the two id spaces never collide.
func idSequence() func() string {
	n := 0
	return func() string {
		n++
		return deterministicUUID("gen", fmt.Sprintf("%d", n)).String()
	}
}

// ctx builds the context for the "current" request: carrying the
// authenticated Clerk sub if one is set, matching what ClerkAuthMiddleware
// would attach to a real request.
func (w *world) ctx() context.Context {
	ctx := context.Background()
	if w.hasToken {
		ctx = appHTTP.WithClerkUserID(ctx, w.clerkSub)
	}
	return ctx
}

// deterministicUUID maps a human-readable test identifier (a name, a slug
// from the feature file, ...) to a stable UUID, so the same identifier
// always produces the same id within and across steps without threading
// real UUIDs through Gherkin text.
func deterministicUUID(parts ...string) uuid.UUID {
	h := sha1.New() //nolint:gosec // deterministic ID derivation, not a security use
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	sum := h.Sum(nil)
	var id uuid.UUID
	copy(id[:], sum[:16])
	id[6] = (id[6] & 0x0f) | 0x50 // version 5
	id[8] = (id[8] & 0x3f) | 0x80 // RFC 4122 variant
	return id
}

func clerkSub(name string) string       { return deterministicUUID("clerk", name).String() }
func nodeID(slug string) uuid.UUID      { return deterministicUUID("node", slug) }
func challengeID(slug string) uuid.UUID { return deterministicUUID("challenge", slug) }
func exerciseID(slug string) uuid.UUID  { return deterministicUUID("exercise", slug) }
func pathID(slug string) uuid.UUID      { return deterministicUUID("path", slug) }
func expandedID(slug string) uuid.UUID  { return deterministicUUID("expanded", slug) }

// ensureRegistered registers name (if not already) via the real RegisterUser
// handler path for student/teacher, or by seeding the repo directly for
// admin — self-registration as admin is rejected by domain.NewUser, mirroring
// how the spec says admin accounts are actually provisioned (directly in the
// database, never through registration). Returns the MotifPath user_id.
func (w *world) ensureRegistered(name string, role domain.Role) uuid.UUID {
	if id, ok := w.userMotifID[name]; ok {
		return id
	}

	if role == domain.RoleAdmin {
		id := deterministicUUID("motif-user", name)
		w.users.put(domain.User{ID: id.String(), ClerkUserID: clerkSub(name), Role: domain.RoleAdmin, RegisteredAt: fixedNow})
		w.userMotifID[name] = id
		return id
	}

	ctx := appHTTP.WithClerkUserID(context.Background(), clerkSub(name))
	resp, err := w.handler.RegisterUser(ctx, generated.RegisterUserRequestObject{
		Body: &generated.RegisterUserRequest{Role: generated.RegisterUserRequestRole(role)},
	})
	if err != nil {
		panic(fmt.Sprintf("setup: RegisterUser failed for %q: %v", name, err))
	}
	created, ok := resp.(generated.RegisterUser201JSONResponse)
	if !ok {
		panic(fmt.Sprintf("setup: RegisterUser for %q did not return 201, got %#v", name, resp))
	}
	w.userMotifID[name] = created.UserId
	return created.UserId
}

func (w *world) authenticateAs(name string, role domain.Role) {
	w.ensureRegistered(name, role)
	w.hasToken = true
	w.clerkSub = clerkSub(name)
}
