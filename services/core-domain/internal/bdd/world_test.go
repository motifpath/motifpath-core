//go:build integration

package bdd

import (
	"context"
	"crypto/sha1" //nolint:gosec // used only for deterministic test UUIDs, not security
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	appHTTP "github.com/motifpath/core-domain/internal/adapters/http"
	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

var fixedNow = time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)

// noShuffle is a shuffle func that never reorders anything — the BDD world's
// default, since most scenarios assert deterministic order. Shuffle-specific
// scenarios construct their own ExerciseService directly with a different
// shuffle func rather than going through world.handler.
func noShuffle(int, func(i, j int)) {}

// world holds all state for a single scenario. A fresh instance is created
// by InitializeScenario for every scenario godog runs, giving each scenario
// full isolation without an explicit teardown step.
type world struct {
	users         *fakeUserRepo
	nodes         *fakeContentNodeRepo
	challenges    *fakeChallengeRepo
	exercises     *fakeExerciseRepo
	expanded      *fakeExpandedContentRepo
	paths         *fakeLearningPathRepo
	studentPaths  *fakeStudentPathRepo
	courses       *fakeCourseRepo
	versions      *fakeContentNodeVersionRepo
	learningState *fakeStudentLearningStateRepo
	completion    *fakeCompletionReader
	skills        *fakeSkillRepo
	concepts      *fakeConceptRepo
	instruments   *fakeInstrumentRepo
	diagrams      *fakeDiagramRepo
	pgPinger      *fakePinger
	mongoPinger   *fakePinger
	handler       *appHTTP.Handler

	// health probe responses from the most recent "probe is checked" step
	livenessResp  generated.LivenessCheckResponseObject
	readinessResp generated.ReadinessCheckResponseObject

	// userMotifID caches the server-generated user_id for each display name
	// once registered — needed because, unlike the deterministic ids used
	// for content nodes/challenges/etc., a user's MotifPath id can't be
	// derived from its display name; it's assigned by IdentityService.
	userMotifID map[string]uuid.UUID

	// skillIDByName/conceptIDByName cache the id of every Skill/Concept
	// created or seeded so far, keyed by name — the "current" id for that
	// name, last-writer-wins. Two different branches may share a name (see
	// skills.feature/concepts.feature), so a step that creates a
	// same-named node under a different parent intentionally overwrites the
	// prior entry; that scenario asserts on the two returned entities
	// directly rather than re-resolving either by name afterward. Every
	// other scenario has at most one node per name, so last-writer-wins
	// never actually differs from "the one node with this name" for them.
	skillIDByName   map[string]uuid.UUID
	conceptIDByName map[string]uuid.UUID

	hasToken bool
	clerkSub string // the "sub" claim of whichever identity is currently authenticated

	// lastResp holds whichever generated ...ResponseObject the most recent
	// handler call returned — one of dozens of distinct generated types
	// across every operation this world drives, so `any` here is the
	// existing, deliberate exception to the "never use any" rule: BDD step
	// assertions type-switch on it, same as every other feature file's
	// steps already do.
	lastResp any
	lastErr  error

	// lastPromptSent holds whichever prompt document the most recent
	// create/update exercise step built, so a following "the exercise's
	// prompt preserves its ... structure" step can assert the response
	// echoes it back unchanged.
	lastPromptSent generated.PromptDocument

	// multiResp is lastResp's repeated-call counterpart, for a "does X
	// twice" or "does X and Y" step (repeated list calls, two generated
	// practice sessions) — same `any` exception as lastResp, same reason.
	multiResp []any

	// multiCreateIDs collects the ids returned by a "creates three X" step,
	// for the "three distinct identifiers are returned" assertion shared by
	// the exercises and expanded-content features.
	multiCreateIDs []uuid.UUID

	// lastNodeSlug is the slug of the most recently established content
	// node ("a video/article content node ... exists"), used by
	// expanded-content steps whose Gherkin text doesn't repeat the slug —
	// they rely on the immediately preceding Given for node context.
	lastNodeSlug string

	// lastExerciseSlug is the slug of the most recently linked exercise, for
	// a following "the exercise records ... among its linked challenges"
	// assertion after an action (like updating the challenge) whose own
	// lastResp isn't exercise-shaped and so can't be asserted on directly.
	lastExerciseSlug string

	// priorStudentPathID holds the id of a StudentPath created by an
	// earlier assign step in the same scenario, captured before a
	// following assign overwrites lastResp — needed by the "additive
	// assign" and "editing a copy" scenarios to refer back to it.
	priorStudentPathID string
}

func newWorld() *world {
	skills := newFakeSkillRepo()
	concepts := newFakeConceptRepo()
	w := &world{
		users:         newFakeUserRepo(),
		nodes:         newFakeContentNodeRepo(skills, concepts),
		challenges:    newFakeChallengeRepo(),
		exercises:     newFakeExerciseRepo(skills, concepts),
		expanded:      newFakeExpandedContentRepo(),
		paths:         newFakeLearningPathRepo(),
		studentPaths:  newFakeStudentPathRepo(),
		courses:       newFakeCourseRepo(),
		versions:      newFakeContentNodeVersionRepo(),
		learningState: newFakeStudentLearningStateRepo(),
		completion:    newFakeCompletionReader(),
		skills:        skills,
		concepts:      concepts,
		instruments:   newFakeInstrumentRepo(),
		diagrams:      newFakeDiagramRepo(skills, concepts),
		pgPinger:      &fakePinger{},
		mongoPinger:   &fakePinger{},
		userMotifID:   map[string]uuid.UUID{},

		skillIDByName:   map[string]uuid.UUID{},
		conceptIDByName: map[string]uuid.UUID{},
	}

	newID := idSequence()
	now := func() time.Time { return fixedNow }

	identity := application.NewIdentityService(w.users, newFakeLanguageRepo(), newID, now)
	content := application.NewContentService(w.nodes, w.expanded, w.skills, w.concepts, w.versions, newID, now)
	challenge := application.NewChallengeService(w.nodes, w.challenges, w.exercises, newID, now)
	exercise := application.NewExerciseService(w.challenges, w.exercises, w.nodes, w.skills, w.concepts, newID, now, noShuffle)
	skill := application.NewSkillService(w.skills, newID)
	concept := application.NewConceptService(w.concepts, newID)
	media := application.NewMediaService(w.exercises, &fakeMediaStorage{}, newID)
	path := application.NewLearningPathService(w.nodes, w.paths, newID, now)
	studentPath := application.NewStudentPathService(w.users, w.paths, w.studentPaths, w.versions, w.learningState, w.nodes, w.exercises, w.completion, newID, now)
	course := application.NewCourseService(w.paths, w.courses, newID, now)

	instrument := application.NewInstrumentService(w.instruments, newID)
	diagram := application.NewDiagramService(w.diagrams, w.instruments, w.skills, w.concepts, newID, now)

	w.handler = appHTTP.NewHandler(identity, content, challenge, exercise, skill, concept, media, path, studentPath, course, instrument, diagram, w.pgPinger, w.mongoPinger)
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

func clerkSub(name string) string        { return deterministicUUID("clerk", name).String() }
func nodeID(slug string) uuid.UUID       { return deterministicUUID("node", slug) }
func challengeID(slug string) uuid.UUID  { return deterministicUUID("challenge", slug) }
func exerciseID(slug string) uuid.UUID   { return deterministicUUID("exercise", slug) }
func pathID(slug string) uuid.UUID       { return deterministicUUID("path", slug) }
func expandedID(slug string) uuid.UUID   { return deterministicUUID("expanded", slug) }
func instrumentID(name string) uuid.UUID { return deterministicUUID("instrument", name) }
func diagramID(slug string) uuid.UUID    { return deterministicUUID("diagram", slug) }

// putSkill seeds a Skill directly into w.skills (mirroring how content nodes
// are seeded via w.nodes.put rather than the real handler) and registers it
// under name in w.skillIDByName. parentID nil means a root skill.
func (w *world) putSkill(name string, parentID *uuid.UUID) uuid.UUID {
	var id uuid.UUID
	var parentIDStr *string
	if parentID != nil {
		id = deterministicUUID("skill", parentID.String(), name)
		s := parentID.String()
		parentIDStr = &s
	} else {
		id = deterministicUUID("skill", "root", name)
	}
	w.skills.put(domain.Skill{ID: id.String(), Name: name, ParentID: parentIDStr})
	w.skillIDByName[name] = id
	return id
}

// skillIDFor resolves name to the id of the Skill previously created/seeded
// under that name, auto-creating it as a root skill on first reference —
// most content-node/exercise/challenge scenarios reference a skill by plain
// name with no prior "a skill exists" step, since which specific tree node
// it is doesn't matter to them.
func (w *world) skillIDFor(name string) uuid.UUID {
	if id, ok := w.skillIDByName[name]; ok {
		return id
	}
	return w.putSkill(name, nil)
}

// putConcept/conceptIDFor are putSkill/skillIDFor's counterparts for Concept.
func (w *world) putConcept(name string, parentID *uuid.UUID) uuid.UUID {
	var id uuid.UUID
	var parentIDStr *string
	if parentID != nil {
		id = deterministicUUID("concept", parentID.String(), name)
		s := parentID.String()
		parentIDStr = &s
	} else {
		id = deterministicUUID("concept", "root", name)
	}
	w.concepts.put(domain.Concept{ID: id.String(), Name: name, ParentID: parentIDStr})
	w.conceptIDByName[name] = id
	return id
}

func (w *world) conceptIDFor(name string) uuid.UUID {
	if id, ok := w.conceptIDByName[name]; ok {
		return id
	}
	return w.putConcept(name, nil)
}

// skillIDsFor/conceptIDsFor resolve a comma-separated Gherkin skill/concept
// list ("alternate-picking, string-muting") to its ids, in order.
func (w *world) skillIDsFor(list string) []uuid.UUID {
	names := splitCommaList(list)
	ids := make([]uuid.UUID, len(names))
	for i, name := range names {
		ids[i] = w.skillIDFor(name)
	}
	return ids
}

func (w *world) conceptIDsFor(list string) []uuid.UUID {
	names := splitCommaList(list)
	ids := make([]uuid.UUID, len(names))
	for i, name := range names {
		ids[i] = w.conceptIDFor(name)
	}
	return ids
}

func splitCommaList(list string) []string {
	var names []string
	for _, part := range strings.Split(list, ",") {
		names = append(names, strings.TrimSpace(part))
	}
	return names
}

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
