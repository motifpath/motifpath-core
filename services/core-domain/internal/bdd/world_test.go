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
	users             *fakeUserRepo
	nodes             *fakeContentNodeRepo
	challenges        *fakeChallengeRepo
	exercises         *fakeExerciseRepo
	expanded          *fakeExpandedContentRepo
	paths             *fakeLearningPathRepo
	studentPaths      *fakeStudentPathRepo
	courses           *fakeCourseRepo
	courseVersions    *fakeCourseVersionRepo
	courseEnrollments *fakeCourseEnrollmentRepo
	versions          *fakeContentNodeVersionRepo
	learningState     *fakeStudentLearningStateRepo
	completion        *fakeCompletionReader
	knowledge         *fakeKnowledgeNodeRepo
	knowledgeEdges    *fakeKnowledgeEdgeRepo
	instruments       *fakeInstrumentRepo
	voices            *fakeVoiceRepo
	diagrams          *fakeDiagramRepo
	pgPinger          *fakePinger
	mongoPinger       *fakePinger
	handler           *appHTTP.Handler

	// health probe responses from the most recent "probe is checked" step
	livenessResp  generated.LivenessCheckResponseObject
	readinessResp generated.ReadinessCheckResponseObject

	// userMotifID caches the server-generated user_id for each display name
	// once registered — needed because, unlike the deterministic ids used
	// for content nodes/challenges/etc., a user's MotifPath id can't be
	// derived from its display name; it's assigned by IdentityService.
	userMotifID map[string]uuid.UUID

	// skillIDByName/conceptIDByName cache the id of every skill/concept
	// knowledge node created or seeded so far, keyed by the name a scenario
	// calls it — which is also the node's key, so it is unique.
	skillIDByName   map[string]uuid.UUID
	conceptIDByName map[string]uuid.UUID

	// lastEdgeID is "that" knowledge edge: the one most recently seeded or
	// created.
	lastEdgeID uuid.UUID

	hasToken bool
	clerkSub string // the "sub" claim of whichever identity is currently authenticated
	persona  string // the Gherkin name of whichever identity is currently authenticated

	// nameClaims holds the "name" claim each persona's session token
	// carries, set by the "is named" steps. A persona with no entry carries
	// defaultNameClaim(persona); an entry of "" is a token with no name.
	nameClaims map[string]string

	// lastResp holds whichever generated ...ResponseObject the most recent
	// handler call returned — one of dozens of distinct generated types
	// across every operation this world drives, so `any` here is the
	// existing, deliberate exception to the "never use any" rule: BDD step
	// assertions type-switch on it, same as every other feature file's
	// steps already do.
	lastResp any
	// lastPathSlug is the learning path the scenario last published,
	// unpublished or replaced by slug, so a later status check reads it.
	lastPathSlug string
	// lastEnroller is the persona who last enrolled in a path, the "her" of
	// the enrollment outcome steps.
	lastEnroller string
	// sharedLessonPath is the standalone path whose first lesson a course
	// also holds, for the "completed that lesson in the course" step.
	sharedLessonPath string
	lastErr          error

	// lastPromptSent holds whichever prompt document the most recent
	// create/update exercise step built, so a following "the exercise's
	// prompt preserves its ... structure" step can assert the response
	// echoes it back unchanged.
	lastPromptSent generated.PromptDocument

	// copySource holds the diagram a "saves a copy" step read before
	// creating its copy, so later steps can check the copy against it and
	// that the source itself did not change.
	copySource generated.Diagram

	// pendingDiagram is a create request a "creates a diagram ... at N BPM"
	// step started, which the "the sequence:" step after it completes and
	// sends.
	pendingDiagram *generated.CreateDiagramRequest

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

	// courseIDBySlug caches the server-generated course_id for every course
	// seeded via a "a course ... exists ..." step, keyed by the Gherkin
	// slug — needed because, unlike a learning path's deterministic
	// pathID(slug), a Course's id is assigned by CourseService at create
	// time.
	courseIDBySlug map[string]uuid.UUID

	// lastDeletedPathSlug is the slug most recently targeted by a "deletes
	// the learning path" step, so a following "the learning path is
	// deleted" Then step (whose DeleteLearningPath response carries no
	// body to identify it from) knows which id to check against w.paths.
	lastDeletedPathSlug string

	// lastCourseSlug is the slug most recently targeted by a course-scoped
	// action (create/get/replace/publish/retire), so a following "the
	// course's status becomes ..." Then step — whose triggering response
	// isn't always course-shaped (PublishCourse returns a CourseVersion,
	// not a Course) — knows which course to re-check via w.courses.
	lastCourseSlug string

	// lastRetrievedCourseCheckpoints holds the Checkpoints slice from the
	// most recent GetCourse call, so a following "replaces course ... with
	// the retrieved checkpoints reordered to: ..." step can resend them
	// (title overrides included) in a new order, matching a teacher
	// resubmitting exactly what they were shown.
	lastRetrievedCourseCheckpoints []generated.CourseCheckpoint

	// courseEnrollmentIDByKey caches every CourseEnrollment id created so
	// far, keyed by studentName+"|"+courseSlug — needed because, unlike a
	// course's id (courseIDBySlug), a scenario often names both the
	// student and the course together ("alice" abandons her
	// "fingerstyle-journey" enrollment) and more than one course
	// enrollment can exist per student within a single scenario.
	courseEnrollmentIDByKey map[string]string

	// courseEnrollmentIDByStudent caches the most recently created
	// CourseEnrollment id per student name, last-writer-wins — for the
	// step wordings that name only the student ("carol"'s enrollment)
	// without naming which course.
	courseEnrollmentIDByStudent map[string]string

	// lastCourseEnrollmentKey is the courseEnrollmentIDByKey key most
	// recently acted upon (enrolled, abandoned, completed a checkpoint
	// of), so a following Then step whose own wording names neither the
	// student nor the course ("the enrollment status becomes ...") can
	// still resolve which enrollment to check.
	lastCourseEnrollmentKey string

	// priorCourseEnrollmentID holds the id of a CourseEnrollment abandoned
	// earlier in the same scenario, captured before a following
	// re-enrollment overwrites lastResp — needed by the "re-enrolling
	// starts a fresh enrollment" scenario to assert the new id differs
	// from the old one.
	priorCourseEnrollmentID string

	// standalonePathIDByKey caches the id of every standalone StudentPath
	// assigned so far, keyed by studentName+"|"+pathSlug — needed because a
	// current-path-lifecycle scenario can hold more than one standalone
	// path at once ("alice archives her standalone 'strumming-path' copy"
	// while "open-chords-path" is also assigned), unlike priorStudentPathID
	// which only ever tracks the single most recent one.
	standalonePathIDByKey map[string]string
}

func newWorld() *world {
	knowledge := newFakeKnowledgeNodeRepo()
	nodes := newFakeContentNodeRepo(knowledge)
	paths := newFakeLearningPathRepo()
	paths.nodes = nodes
	courseVersions := newFakeCourseVersionRepo()
	w := &world{
		users:             newFakeUserRepo(),
		nodes:             nodes,
		challenges:        newFakeChallengeRepo(),
		exercises:         newFakeExerciseRepo(knowledge),
		expanded:          newFakeExpandedContentRepo(),
		paths:             paths,
		courses:           newFakeCourseRepo(paths, nodes, courseVersions),
		studentPaths:      newFakeStudentPathRepo(),
		courseVersions:    courseVersions,
		courseEnrollments: newFakeCourseEnrollmentRepo(),
		versions:          newFakeContentNodeVersionRepo(),
		learningState:     newFakeStudentLearningStateRepo(),
		completion:        newFakeCompletionReader(),
		knowledge:         knowledge,
		knowledgeEdges:    newFakeKnowledgeEdgeRepo(),
		instruments:       newFakeInstrumentRepo(),
		voices:            newFakeVoiceRepo(),
		diagrams:          newFakeDiagramRepo(knowledge),
		pgPinger:          &fakePinger{},
		mongoPinger:       &fakePinger{},
		userMotifID:       map[string]uuid.UUID{},

		skillIDByName:   map[string]uuid.UUID{},
		conceptIDByName: map[string]uuid.UUID{},
		courseIDBySlug:  map[string]uuid.UUID{},
		nameClaims:      map[string]string{},

		courseEnrollmentIDByKey:     map[string]string{},
		courseEnrollmentIDByStudent: map[string]string{},
		standalonePathIDByKey:       map[string]string{},
	}

	knowledge.edges, knowledge.nodes, knowledge.exercises, knowledge.diagrams, knowledge.challenges = w.knowledgeEdges, w.nodes, w.exercises, w.diagrams, w.challenges

	newID := idSequence()
	now := func() time.Time { return fixedNow }

	identity := application.NewIdentityService(w.users, newFakeLanguageRepo(), newID, now)
	content := application.NewContentService(w.nodes, w.expanded, w.knowledge, w.versions, w.diagrams, w.instruments, w.voices, newID, now)
	challenge := application.NewChallengeService(w.nodes, w.challenges, w.exercises, newID, now)
	exercise := application.NewExerciseService(w.challenges, w.exercises, w.nodes, w.knowledge, w.diagrams, w.instruments, w.voices, w.users, newID, now, noShuffle)
	knowledgeNode := application.NewKnowledgeNodeService(w.knowledge, w.instruments, newFakeLanguageRepo(), newID)
	knowledgeEdge := application.NewKnowledgeEdgeService(w.knowledgeEdges, w.knowledge, newID)
	media := application.NewMediaService(w.exercises, &fakeMediaStorage{}, newID)
	path := application.NewLearningPathService(w.nodes, w.paths, w.courseVersions, w.versions, newFakeLanguageRepo(), w.users, w.instruments, newID, now)
	studentPath := application.NewStudentPathService(w.users, w.paths, w.studentPaths, w.versions, w.learningState, w.courseEnrollments, w.courseVersions, w.nodes, w.exercises, w.completion, newID, now)
	course := application.NewCourseService(w.paths, w.courses, w.courseVersions, w.users, newFakeLanguageRepo(), w.instruments, newID, now)
	courseEnrollment := application.NewCourseEnrollmentService(w.courses, w.courseVersions, w.paths, w.studentPaths, w.courseEnrollments, studentPath, w.learningState, w.completion, newID, now)

	instrument := application.NewInstrumentService(w.instruments, w.voices, newFakeLanguageRepo(), newID)
	voice := application.NewVoiceService(w.voices, voiceSamplesBaseURL)
	diagram := application.NewDiagramService(w.diagrams, w.instruments, w.knowledge, newFakeLanguageRepo(), w.users, newID, now)

	w.handler = appHTTP.NewHandler(identity, content, challenge, exercise, knowledgeNode, knowledgeEdge, media, path, application.NewPathCatalogService(w.paths, w.users), studentPath, course, courseEnrollment, instrument, voice, diagram, w.pgPinger, w.mongoPinger)
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
	if !w.hasToken {
		return context.Background()
	}
	if w.persona != "" && clerkSub(w.persona) == w.clerkSub {
		return w.identityCtx(w.persona)
	}
	return appHTTP.WithClerkUserID(context.Background(), w.clerkSub)
}

// identityCtx builds the context of a request made with persona's session
// token: its Clerk sub, plus its "name" claim when it carries one — what
// ClerkAuthMiddleware would attach to a real request.
func (w *world) identityCtx(persona string) context.Context {
	ctx := appHTTP.WithClerkUserID(context.Background(), clerkSub(persona))
	if name := w.nameClaim(persona); name != "" {
		ctx = appHTTP.WithNameClaim(ctx, name)
	}
	return ctx
}

// nameClaim returns the "name" claim persona's session token carries.
func (w *world) nameClaim(persona string) string {
	if name, ok := w.nameClaims[persona]; ok {
		return name
	}
	return defaultNameClaim(persona)
}

// defaultNameClaim gives every persona a name without a step having to set
// one — registration requires it — by capitalizing the Gherkin name
// ("bob" → "Bob").
func defaultNameClaim(persona string) string {
	if persona == "" {
		return ""
	}
	return strings.ToUpper(persona[:1]) + persona[1:]
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

// putSkill seeds a skill directly into w.knowledge (mirroring how content
// nodes are seeded via w.nodes.put rather than the real handler) and
// registers it under name in w.skillIDByName. name becomes the node's key,
// slugged, and its name in every language; parentID nil means a root; no
// instrumentIDs means every instrument.
func (w *world) putSkill(name string, parentID *uuid.UUID, instrumentIDs ...string) uuid.UUID {
	id := w.putKnowledgeNode(domain.KnowledgeNodeKindSkill, name, parentID, instrumentIDs)
	w.skillIDByName[name] = id
	return id
}

// putConcept is putSkill's counterpart for concepts.
func (w *world) putConcept(name string, parentID *uuid.UUID, instrumentIDs ...string) uuid.UUID {
	id := w.putKnowledgeNode(domain.KnowledgeNodeKindConcept, name, parentID, instrumentIDs)
	w.conceptIDByName[name] = id
	return id
}

func (w *world) putKnowledgeNode(kind domain.KnowledgeNodeKind, name string, parentID *uuid.UUID, instrumentIDs []string) uuid.UUID {
	id := deterministicUUID(string(kind), name)
	var parentIDStr *string
	if parentID != nil {
		s := parentID.String()
		parentIDStr = &s
	}
	w.knowledge.put(domain.KnowledgeNode{
		ID: id.String(), Kind: kind, Key: slug(name),
		Names:    domain.LocalizedText{"en": name, "pt_BR": name},
		ParentID: parentIDStr, InstrumentIDs: instrumentIDs,
	})
	return id
}

// skillIDFor resolves name to the id of the skill previously created or
// seeded under that name, auto-creating it as a root skill for every
// instrument on first reference — most content-node/exercise/challenge
// scenarios reference a skill by plain name with no prior "a skill exists"
// step, since which tree node it is doesn't matter to them.
func (w *world) skillIDFor(name string) uuid.UUID {
	if id, ok := w.skillIDByName[name]; ok {
		return id
	}
	return w.putSkill(name, nil)
}

// conceptIDFor is skillIDFor's counterpart for concepts.
func (w *world) conceptIDFor(name string) uuid.UUID {
	if id, ok := w.conceptIDByName[name]; ok {
		return id
	}
	return w.putConcept(name, nil)
}

// slug turns a scenario's name for a node into a key: lowercase words
// joined by single hyphens.
func slug(name string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return !('a' <= r && r <= 'z' || '0' <= r && r <= '9')
	}), "-")
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
		w.users.put(domain.User{ID: id.String(), ClerkUserID: clerkSub(name), Role: domain.RoleAdmin, DisplayName: w.nameClaim(name), RegisteredAt: fixedNow})
		w.userMotifID[name] = id
		return id
	}

	resp, err := w.handler.RegisterUser(w.identityCtx(name), generated.RegisterUserRequestObject{
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
	w.persona = name
}

// createSeededCourse creates a course through the API for a Given step, recording
// creatorName as its creator. It creates as an admin, since a seeded course
// may use paths its creator didn't write, which a teacher's own request
// can't; seeding sets up state, it doesn't exercise authoring rules.
func (w *world) createSeededCourse(creatorName string, body *generated.CreateCourseRequest) (generated.CreateCourseResponseObject, error) {
	creatorID := w.ensureRegistered(creatorName, domain.RoleTeacher)
	w.ensureRegistered("course-seeding-admin", domain.RoleAdmin)
	adminCtx := appHTTP.WithClerkUserID(context.Background(), clerkSub("course-seeding-admin"))
	resp, err := w.handler.CreateCourse(adminCtx, generated.CreateCourseRequestObject{Body: body})
	if err != nil {
		return resp, err
	}
	if created, ok := resp.(generated.CreateCourse201JSONResponse); ok {
		course, err := w.courses.GetByID(context.Background(), created.CourseId.String())
		if err != nil {
			return resp, err
		}
		course.CreatedBy = creatorID.String()
		w.courses.put(course)
	}
	return resp, nil
}
