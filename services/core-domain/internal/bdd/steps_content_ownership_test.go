//go:build integration

package bdd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	appHTTP "github.com/motifpath/core-domain/internal/adapters/http"
	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// ownershipWorld is what the content ownership scenarios remember: the items
// they created by title, the copies learners hold, the commercial content
// they read, and what the store held before a backfill rerun.
type ownershipWorld struct {
	content     map[string]domain.ContentRef
	copies      map[string]string
	read        map[string]generated.CommercialContent
	firstResult []byte
	before      fakeCommercialState
	lastLegacy  domain.ContentRef
}

func newOwnershipWorld() *ownershipWorld {
	return &ownershipWorld{
		content: map[string]domain.ContentRef{}, copies: map[string]string{}, read: map[string]generated.CommercialContent{},
	}
}

func registerContentOwnershipSteps(sc *godog.ScenarioContext, w *world) {
	// Corrections are rolled out for these scenarios unless one says the
	// rollout is OFF, so each scenario states only what it is about.
	sc.Before(func(ctx context.Context, scenario *godog.Scenario) (context.Context, error) {
		for _, tag := range scenario.Tags {
			if tag.Name == "@MOT-118" {
				w.setOwnershipRollout(true)
			}
		}
		return ctx, nil
	})

	// Creating content
	sc.Step(`^(\w+) creates the path "([^"]+)"$`, func(name, title string) error { return w.createOwnedPath(name, title, nil) })
	sc.Step(`^(\w+) created the path "([^"]+)"$`, func(name, title string) error { return w.createOwnedPath(name, title, nil) })
	sc.Step(`^(\w+) creates the path "([^"]+)" with an extra field naming the platform as owner$`, func(name, title string) error {
		return w.createOwnedPath(name, title, map[string]any{"owner_kind": "platform", "owner_id": domain.CatalogProfileID})
	})
	sc.Step(`^(\w+) creates the exercise "([^"]+)"$`, w.createOwnedExercise)
	sc.Step(`^(\w+) created the exercise "([^"]+)"$`, w.createOwnedExercise)
	sc.Step(`^(\w+) created and published the path "([^"]+)"$`, w.createAndPublishPath)
	sc.Step(`^(\w+) created the path "([^"]+)" containing a lesson that embeds "([^"]+)"$`, w.createPathEmbedding)
	sc.Step(`^(\w+) holds a copy of "([^"]+)"$`, w.holdsCopy)

	// Changes that must not move ownership
	sc.Step(`^(\w+) renames "([^"]+)" to "([^"]+)" and replaces its items$`, w.renameAndReplacePath)
	sc.Step(`^(\w+) is promoted to admin$`, w.promoteToAdmin)
	sc.Step(`^(\w+) deleted the path "([^"]+)"$`, w.deleteOwnedPath)

	// Reads
	sc.Step(`^(\w+) reads the commercial content of "([^"]+)"$`, w.readCommercialContent)
	sc.Step(`^(\w+) reads the commercial content of "([^"]+)" and of "([^"]+)"$`, func(name, first, second string) error {
		if err := w.readCommercialContent(name, first); err != nil {
			return err
		}
		return w.readCommercialContent(name, second)
	})

	// Corrections
	sc.Step(`^the content-ownership rollout is OFF$`, func() error { w.setOwnershipRollout(false); return nil })
	sc.Step(`^(\w+) corrects the ownership of "([^"]+)" to teacher (\w+) at ownership revision (\d+) with reason "([^"]+)"$`,
		func(name, title, teacher string, revision int, reason string) error {
			return w.correctOwnership(name, title, "teacher", w.userID(teacher), revision, reason, "")
		})
	sc.Step(`^(\w+) corrects the ownership of "([^"]+)" to teacher (\w+) at ownership revision (\d+)$`,
		func(name, title, teacher string, revision int) error {
			return w.correctOwnership(name, title, "teacher", w.userID(teacher), revision, "rights reviewed", "")
		})
	sc.Step(`^(\w+) corrects the ownership of "([^"]+)" to the platform at ownership revision (\d+)$`,
		func(name, title string, revision int) error {
			return w.correctOwnership(name, title, "platform", domain.CatalogProfileID, revision, "rights reviewed", "")
		})
	sc.Step(`^(\w+) has already corrected the ownership of "([^"]+)" once$`, func(name, title string) error {
		if err := w.correctOwnership(name, title, "platform", domain.CatalogProfileID, 1, "rights reviewed", ""); err != nil {
			return err
		}
		return w.commercialResponseIs(http.StatusOK)
	})
	sc.Step(`^(\w+) corrected the ownership of "([^"]+)" to the platform with idempotency key (\w+)$`, func(name, title, key string) error {
		if err := w.correctOwnership(name, title, "platform", domain.CatalogProfileID, 1, "rights reviewed", key); err != nil {
			return err
		}
		if err := w.commercialResponseIs(http.StatusOK); err != nil {
			return err
		}
		w.ownership.firstResult = w.commercial.lastBody
		return nil
	})
	sc.Step(`^(\w+) sends the same correction again with idempotency key (\w+)$`, func(_, _ string) error {
		last := w.commercial.lastRequest
		return w.commercialSend(last.persona, last.method, last.path, last.key, last.body)
	})
	sc.Step(`^the response repeats the original command result$`, w.repeatsOriginalCommandResult)
	sc.Step(`^the command is applied with an evidence entry$`, w.commandAppliedWithEvidence)

	// Ownership assertions
	sc.Step(`^the ownership of "([^"]+)" has owner kind "(\w+)" and the MotifPath Catalog as owner$`, func(title, kind string) error {
		return w.ownershipIs(title, kind, domain.CatalogProfileID, 0)
	})
	sc.Step(`^the ownership of "([^"]+)" (?:still )?has owner kind "(\w+)" and (\w+) as owner$`, func(title, kind, owner string) error {
		return w.ownershipIs(title, kind, w.userID(owner), 0)
	})
	sc.Step(`^the ownership of "([^"]+)" (?:still )?has (\w+) as owner and ownership revision (\d+)$`, func(title, owner string, revision int) error {
		return w.ownershipIs(title, "", w.userID(owner), revision)
	})
	sc.Step(`^the ownership of "([^"]+)" has owner kind "(\w+)", (\w+) as owner and ownership revision (\d+)$`, func(title, kind, owner string, revision int) error {
		return w.ownershipIs(title, kind, w.userID(owner), revision)
	})
	sc.Step(`^its creator is (\w+), its source is "(\w+)" and its ownership revision is (\d+)$`, w.creatorSourceRevision)
	sc.Step(`^its creator is still (\w+)$`, func(name string) error { return w.creatorIs(w.lastOwnedTitle(), w.userID(name)) })
	sc.Step(`^the ownership revision of "([^"]+)" is still (\d+)$`, func(title string, revision int) error {
		return w.ownershipIs(title, "", "", revision)
	})
	sc.Step(`^the ownership history of "([^"]+)" still shows platform ownership at revision (\d+)$`, w.historyShowsPlatform)
	sc.Step(`^there is one ownership record for "([^"]+)" and none for (\w+)'s copy$`, w.oneRecordNoneForCopy)
	sc.Step(`^"([^"]+)" has owner kind "(\w+)" and "([^"]+)" has owner kind "(\w+)"$`, w.readOwnerKinds)

	// Backfill
	sc.Step(`^a course created by (\w+) exists without an ownership record$`, w.legacyCourse)
	sc.Step(`^an exercise created by the MotifPath Catalog profile exists without an ownership record$`, func() error {
		return w.legacyExercise(domain.CatalogProfileID)
	})
	sc.Step(`^a path whose teacher is (\w+) exists without an ownership record$`, w.legacyPath)
	sc.Step(`^an exercise with no recorded creator exists without an ownership record$`, func() error { return w.legacyExercise("") })
	sc.Step(`^the ownership backfill (?:runs|has run)$`, w.runOwnershipBackfill)
	sc.Step(`^the ownership backfill runs again$`, func() error {
		w.ownership.before = w.commercial.store.state.clone()
		return w.runOwnershipBackfill()
	})
	sc.Step(`^(\w+) has corrected the ownership of one backfilled path to teacher (\w+)$`, w.correctBackfilledPath)
	sc.Step(`^that course has owner kind "(\w+)", the MotifPath Catalog as owner, (\w+) as creator and source "(\w+)"$`,
		func(kind, creator, source string) error {
			return w.legacyOwnershipIs(kind, ptr(domain.CatalogProfileID), ptr(w.userID(creator)), source)
		})
	sc.Step(`^that exercise has owner kind "(\w+)" and the MotifPath Catalog as owner$`, func(kind string) error {
		return w.legacyOwnershipIs(kind, ptr(domain.CatalogProfileID), ptr(domain.CatalogProfileID), "reviewed_legacy")
	})
	sc.Step(`^that path has owner kind "(\w+)", (\w+) as owner and source "(\w+)"$`, func(kind, owner, source string) error {
		return w.legacyOwnershipIs(kind, ptr(w.userID(owner)), ptr(w.userID(owner)), source)
	})
	sc.Step(`^that exercise has owner kind "(\w+)", no owner and no creator$`, func(kind string) error {
		return w.legacyOwnershipIs(kind, nil, nil, "reviewed_legacy")
	})
	sc.Step(`^learners can still practise that exercise$`, w.unresolvedExerciseStillPractisable)
	sc.Step(`^no ownership record or evidence entry is added or changed$`, w.backfillChangedNothing)
}

func ptr(value string) *string { return &value }

func (w *world) userID(name string) string {
	id, ok := w.userMotifID[name]
	if !ok {
		panic(fmt.Sprintf("setup: %q is not registered", name))
	}
	return id.String()
}

func (w *world) setOwnershipRollout(on bool) {
	w.commercial.store.state.flags[domain.FlagName("content_ownership")] = domain.FlagSetting{
		Enabled: on, Audience: domain.RolloutAudience{AllUsers: on},
	}
}

// ── Creating content through the API ─────────────────────────────────────────

// send makes one request as persona and returns its status and body, without
// touching the response the scenario's own Then steps check.
func (w *world) send(persona, method, path string, body any) (int, []byte, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return 0, nil, err
	}
	router := generated.HandlerWithOptions(appHTTP.NewStrictHandler(w.handler), generated.ChiServerOptions{})
	ctx := appHTTP.WithClerkUserID(context.Background(), clerkSub(persona))
	req := httptest.NewRequestWithContext(ctx, method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Code, rec.Body.Bytes(), nil
}

func (w *world) create(persona, path string, body any) (uuid.UUID, error) {
	status, raw, err := w.send(persona, http.MethodPost, path, body)
	if err != nil {
		return uuid.Nil, err
	}
	if status != http.StatusCreated && status != http.StatusOK {
		return uuid.Nil, fmt.Errorf("POST %s as %s: %d %s", path, persona, status, raw)
	}
	var created map[string]any
	if err := json.Unmarshal(raw, &created); err != nil {
		return uuid.Nil, err
	}
	for _, key := range []string{"id", "content_node_id", "exercise_id", "learning_path_id", "course_id"} {
		if value, ok := created[key].(string); ok {
			return uuid.Parse(value)
		}
	}
	return uuid.Nil, fmt.Errorf("POST %s answered no id: %s", path, raw)
}

func (w *world) createLesson(persona, title string) (uuid.UUID, error) {
	return w.create(persona, "/content-nodes", generated.CreateContentNodeRequest{
		Title: title, ContentType: generated.CreateContentNodeRequestContentTypeVideo, MediaUrl: defaultMediaURL(),
		Classification: generated.ClassificationInput{SkillIds: w.skillIDsFor("skill-1"), ConceptIds: w.conceptIDsFor("concept-1"), DifficultyLevel: generated.ClassificationInputDifficultyLevelBeginner},
		LanguageCodes:  []string{"en"},
	})
}

func pathBody(title string, lessonID uuid.UUID) map[string]any {
	return map[string]any{
		"title": title, "level": "beginner", "instrument_ids": []string{},
		"items": []map[string]any{{"content_node_id": lessonID}},
	}
}

func (w *world) createOwnedPath(name, title string, extra map[string]any) error {
	lesson, err := w.createLesson(name, title+" lesson")
	if err != nil {
		return err
	}
	body := pathBody(title, lesson)
	for k, v := range extra {
		body[k] = v
	}
	id, err := w.create(name, "/learning-paths", body)
	if err != nil {
		return err
	}
	w.ownership.content[title] = domain.ContentRef{Kind: domain.ContentPath, ID: id.String()}
	return nil
}

func (w *world) createOwnedExercise(name, title string) error {
	id, err := w.create(name, "/exercises", w.exerciseBody(title))
	if err != nil {
		return err
	}
	w.ownership.content[title] = domain.ContentRef{Kind: domain.ContentExercise, ID: id.String()}
	return nil
}

func (w *world) createAndPublishPath(name, title string) error {
	lesson, err := w.createLesson(name, title+" lesson")
	if err != nil {
		return err
	}
	if err := w.publish(name, "/content-nodes/"+lesson.String()+"/publish"); err != nil {
		return err
	}
	body := pathBody(title, lesson)
	body["summary"], body["language"] = "A complete path", "en"
	id, err := w.create(name, "/learning-paths", body)
	if err != nil {
		return err
	}
	w.ownership.content[title] = domain.ContentRef{Kind: domain.ContentPath, ID: id.String()}
	// Only an administrator publishes a path to the catalog.
	return w.publish(w.anyAdmin(), "/learning-paths/"+id.String()+"/publish")
}

func (w *world) publish(name, path string) error {
	status, raw, err := w.send(name, http.MethodPost, path, map[string]any{})
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return fmt.Errorf("POST %s as %s: %d %s", path, name, status, raw)
	}
	return nil
}

func (w *world) createPathEmbedding(name, title, exercise string) error {
	lesson, err := w.createLesson(name, title+" lesson")
	if err != nil {
		return err
	}
	exerciseRef, ok := w.ownership.content[exercise]
	if !ok {
		return fmt.Errorf("no exercise %q was created", exercise)
	}
	status, raw, err := w.send(name, http.MethodPost, "/content-nodes/"+lesson.String()+"/exercises/"+exerciseRef.ID, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return fmt.Errorf("embed %q in %q: %d %s", exercise, title, status, raw)
	}
	id, err := w.create(name, "/learning-paths", pathBody(title, lesson))
	if err != nil {
		return err
	}
	w.ownership.content[title] = domain.ContentRef{Kind: domain.ContentPath, ID: id.String()}
	return nil
}

func (w *world) holdsCopy(name, title string) error {
	w.ensureRegistered(name, domain.RoleStudent)
	template := w.ownership.content[title]
	status, raw, err := w.send(name, http.MethodPost, "/students/me/student-paths", map[string]any{"learning_path_id": template.ID})
	if err != nil {
		return err
	}
	if status != http.StatusCreated && status != http.StatusOK {
		return fmt.Errorf("%s copies %q: %d %s", name, title, status, raw)
	}
	var copied generated.StudentPath
	if err := json.Unmarshal(raw, &copied); err != nil {
		return err
	}
	w.ownership.copies[name] = copied.StudentPathId.String()
	return w.rememberCopy(name)
}

func (w *world) renameAndReplacePath(name, title, renamed string) error {
	ref := w.ownership.content[title]
	lesson, err := w.createLesson(name, renamed+" lesson")
	if err != nil {
		return err
	}
	status, raw, err := w.send(name, http.MethodPut, "/learning-paths/"+ref.ID, pathBody(renamed, lesson))
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("replace %q: %d %s", title, status, raw)
	}
	w.ownership.content[renamed] = ref
	return nil
}

func (w *world) deleteOwnedPath(name, title string) error {
	ref := w.ownership.content[title]
	status, raw, err := w.send(name, http.MethodDelete, "/learning-paths/"+ref.ID, nil)
	if err != nil {
		return err
	}
	if status != http.StatusNoContent {
		return fmt.Errorf("delete %q: %d %s", title, status, raw)
	}
	return nil
}

func (w *world) promoteToAdmin(name string) error {
	user, err := w.users.GetByID(context.Background(), w.userID(name))
	if err != nil {
		return err
	}
	user.Role = domain.RoleAdmin
	w.users.put(user)
	return nil
}

// ── Reads and corrections ────────────────────────────────────────────────────

func (w *world) contentPath(title string) string {
	ref := w.ownership.content[title]
	return "/commercial-content/" + string(ref.Kind) + "/" + ref.ID
}

func (w *world) readCommercialContent(name, title string) error {
	if err := w.commercialSend(name, http.MethodGet, w.contentPath(title), "", nil); err != nil {
		return err
	}
	if w.commercial.lastStatus != http.StatusOK {
		return nil
	}
	var content generated.CommercialContent
	if err := json.Unmarshal(w.commercial.lastBody, &content); err != nil {
		return err
	}
	w.ownership.read[title] = content
	return nil
}

func (w *world) correctOwnership(name, title, ownerKind, ownerID string, revision int, reason, key string) error {
	body, err := json.Marshal(map[string]any{
		"expected_revision": revision, "owner_kind": ownerKind, "owner_id": ownerID, "reason": reason,
	})
	if err != nil {
		return err
	}
	w.ownership.lastLegacy = w.ownership.content[title]
	return w.commercialSend(name, http.MethodPost, w.contentPath(title)+"/ownership", idempotencyKey(key), body)
}

func (w *world) repeatsOriginalCommandResult() error {
	if err := w.commercialResponseIs(http.StatusOK); err != nil {
		return err
	}
	if !bytes.Equal(bytes.TrimSpace(w.commercial.lastBody), bytes.TrimSpace(w.ownership.firstResult)) {
		return fmt.Errorf("replay answered %s, the original was %s", w.commercial.lastBody, w.ownership.firstResult)
	}
	return nil
}

func (w *world) commandAppliedWithEvidence() error {
	if err := w.commercialResponseIs(http.StatusOK); err != nil {
		return err
	}
	var command generated.Command
	if err := json.Unmarshal(w.commercial.lastBody, &command); err != nil {
		return err
	}
	if command.Status != generated.CommandStatusApplied {
		return fmt.Errorf("command status is %s", command.Status)
	}
	for _, record := range w.commercial.store.state.audit {
		if record.Entry.AuditEntryID == command.EvidenceEntryId.String() {
			return nil
		}
	}
	return fmt.Errorf("no evidence entry %s was recorded", command.EvidenceEntryId)
}

// ── Ownership assertions ─────────────────────────────────────────────────────

// currentOwnership reads an item's ownership through the API as an admin.
func (w *world) currentOwnership(title string) (generated.Ownership, error) {
	ref, ok := w.ownership.content[title]
	if !ok {
		return generated.Ownership{}, fmt.Errorf("no content %q was created", title)
	}
	admin := w.anyAdmin()
	status, raw, err := w.send(admin, http.MethodGet, "/commercial-content/"+string(ref.Kind)+"/"+ref.ID, nil)
	if err != nil {
		return generated.Ownership{}, err
	}
	if status != http.StatusOK {
		return generated.Ownership{}, fmt.Errorf("%s reads %q: %d %s", admin, title, status, raw)
	}
	var content generated.CommercialContent
	if err := json.Unmarshal(raw, &content); err != nil {
		return generated.Ownership{}, err
	}
	return content.Ownership, nil
}

func (w *world) anyAdmin() string {
	for name, id := range w.userMotifID {
		user, err := w.users.GetByID(context.Background(), id.String())
		if err == nil && user.Role == domain.RoleAdmin {
			return name
		}
	}
	panic("setup: the scenario has no admin")
}

func (w *world) lastOwnedTitle() string {
	for title, ref := range w.ownership.content {
		if ref == w.ownership.lastLegacy {
			return title
		}
	}
	panic("setup: no content was corrected")
}

func (w *world) ownershipIs(title, kind, ownerID string, revision int) error {
	ownership, err := w.currentOwnership(title)
	if err != nil {
		return err
	}
	if kind != "" && string(ownership.OwnerKind) != kind {
		return fmt.Errorf("%q has owner kind %s, want %s", title, ownership.OwnerKind, kind)
	}
	if ownerID != "" && (ownership.OwnerId == nil || ownership.OwnerId.String() != ownerID) {
		return fmt.Errorf("%q is owned by %v, want %s", title, ownership.OwnerId, ownerID)
	}
	if revision != 0 && ownership.Revision != revision {
		return fmt.Errorf("%q is at ownership revision %d, want %d", title, ownership.Revision, revision)
	}
	return nil
}

func (w *world) creatorIs(title, creatorID string) error {
	ownership, err := w.currentOwnership(title)
	if err != nil {
		return err
	}
	if ownership.CreatorId == nil || ownership.CreatorId.String() != creatorID {
		return fmt.Errorf("%q was created by %v, want %s", title, ownership.CreatorId, creatorID)
	}
	return nil
}

func (w *world) creatorSourceRevision(name, source string, revision int) error {
	for title := range w.ownership.content {
		ownership, err := w.currentOwnership(title)
		if err != nil {
			return err
		}
		if ownership.CreatorId == nil || ownership.CreatorId.String() != w.userID(name) {
			return fmt.Errorf("%q was created by %v, want %s", title, ownership.CreatorId, name)
		}
		if string(ownership.Source) != source || ownership.Revision != revision {
			return fmt.Errorf("%q has source %s at revision %d, want %s at %d", title, ownership.Source, ownership.Revision, source, revision)
		}
	}
	return nil
}

// historyShowsPlatform checks the evidence an admin can traverse for the
// item: the correction kept what it replaced.
func (w *world) historyShowsPlatform(title string, revision int) error {
	ref := w.ownership.content[title]
	for _, record := range w.commercial.store.state.audit {
		entry := record.Entry
		if entry.Operation.ResourceID != ref.ID || entry.Operation.Action != application.ActionCorrectContentOwnership || entry.Outcome != domain.AuditOutcomeApplied {
			continue
		}
		var before struct {
			OwnerKind string `json:"owner_kind"`
			Revision  int    `json:"revision"`
		}
		if err := json.Unmarshal(entry.BeforeState, &before); err != nil {
			return err
		}
		if before.OwnerKind == "platform" && before.Revision == revision {
			return nil
		}
	}
	return fmt.Errorf("no evidence for %q shows platform ownership at revision %d", title, revision)
}

func (w *world) oneRecordNoneForCopy(title, learner string) error {
	if err := w.commercialResponseIs(http.StatusOK); err != nil {
		return err
	}
	ref := w.ownership.content[title]
	if got := len(w.commercial.store.state.ownership[ref]); got != 1 {
		return fmt.Errorf("%q has %d ownership records, want 1", title, got)
	}
	copyID := w.ownership.copies[learner]
	for owned := range w.commercial.store.state.ownership {
		if owned.ID == copyID {
			return fmt.Errorf("%s's copy has an ownership record of its own", learner)
		}
	}
	return nil
}

func (w *world) readOwnerKinds(first, firstKind, second, secondKind string) error {
	for title, kind := range map[string]string{first: firstKind, second: secondKind} {
		content, ok := w.ownership.read[title]
		if !ok {
			return fmt.Errorf("%q was not read", title)
		}
		if string(content.Ownership.OwnerKind) != kind {
			return fmt.Errorf("%q has owner kind %s, want %s", title, content.Ownership.OwnerKind, kind)
		}
	}
	return nil
}

// ── Backfill ─────────────────────────────────────────────────────────────────

func (w *world) legacyCourse(creator string) error {
	id := uuid.NewString()
	w.courses.put(domain.Course{ID: id, Title: "Legacy course", CreatedBy: w.userID(creator), CreatedAt: fixedNow})
	w.ownership.lastLegacy = domain.ContentRef{Kind: domain.ContentCourse, ID: id}
	return nil
}

func (w *world) legacyExercise(creator string) error {
	id := uuid.NewString()
	w.exercises.put(domain.Exercise{ID: id, Title: "Legacy exercise", ExerciseType: domain.ExerciseTypeTextResponse, CreatedBy: creator, CreatedAt: fixedNow})
	w.ownership.lastLegacy = domain.ContentRef{Kind: domain.ContentExercise, ID: id}
	return nil
}

func (w *world) legacyPath(teacher string) error {
	id := uuid.NewString()
	w.paths.put(domain.LearningPath{ID: id, TeacherID: w.userID(teacher), Title: "Legacy path", CreatedAt: fixedNow})
	w.ownership.lastLegacy = domain.ContentRef{Kind: domain.ContentPath, ID: id}
	return nil
}

func (w *world) runOwnershipBackfill() error {
	backfill := application.NewOwnershipBackfill(fakeOwnershipBackfillStore{w: w}, bddEnvironment(), func() time.Time { return fixedNow }, uuid.NewString)
	_, err := backfill.Run(context.Background())
	return err
}

func (w *world) correctBackfilledPath(name, teacher string) error {
	if err := w.legacyPath(teacher); err != nil {
		return err
	}
	if err := w.runOwnershipBackfill(); err != nil {
		return err
	}
	w.ownership.content["backfilled path"] = w.ownership.lastLegacy
	if err := w.correctOwnership(name, "backfilled path", "teacher", w.userID(teacher), 1, "rights reviewed", ""); err != nil {
		return err
	}
	return w.commercialResponseIs(http.StatusOK)
}

func (w *world) legacyOwnershipIs(kind string, ownerID, creatorID *string, source string) error {
	history := w.commercial.store.state.ownership[w.ownership.lastLegacy]
	if len(history) != 1 {
		return fmt.Errorf("%s has %d ownership records, want 1", w.ownership.lastLegacy.ID, len(history))
	}
	ownership := history[0]
	if string(ownership.OwnerKind) != kind || string(ownership.Source) != source {
		return fmt.Errorf("owner kind %s from %s, want %s from %s", ownership.OwnerKind, ownership.Source, kind, source)
	}
	if !reflect.DeepEqual(ownership.OwnerID, ownerID) {
		return fmt.Errorf("owner %v, want %v", deref(ownership.OwnerID), deref(ownerID))
	}
	if !reflect.DeepEqual(ownership.CreatorID, creatorID) {
		return fmt.Errorf("creator %v, want %v", deref(ownership.CreatorID), deref(creatorID))
	}
	return nil
}

func deref(value *string) string {
	if value == nil {
		return "<none>"
	}
	return *value
}

// unresolvedExerciseStillPractisable checks that attributing an exercise as
// unresolved left it as learners get it: still stored, unchanged.
func (w *world) unresolvedExerciseStillPractisable() error {
	exercise, err := w.exercises.GetByID(context.Background(), w.ownership.lastLegacy.ID)
	if err != nil {
		return fmt.Errorf("the unresolved exercise is no longer available: %w", err)
	}
	if exercise.Title != "Legacy exercise" {
		return fmt.Errorf("the unresolved exercise changed: %+v", exercise)
	}
	return nil
}

func (w *world) backfillChangedNothing() error {
	before, after := w.ownership.before, w.commercial.store.state
	if !reflect.DeepEqual(before.ownership, after.ownership) {
		return fmt.Errorf("ownership records changed on the rerun")
	}
	if len(before.audit) != len(after.audit) {
		return fmt.Errorf("the rerun added %d evidence entries", len(after.audit)-len(before.audit))
	}
	return nil
}

// fakeOwnershipBackfillStore finds unowned content in the scenario's
// repositories, as the Postgres store does across their tables.
type fakeOwnershipBackfillStore struct {
	w *world
}

func (s fakeOwnershipBackfillStore) ListUnowned(_ context.Context, limit int) ([]ports.UnownedContent, error) {
	w := s.w
	var unowned []ports.UnownedContent
	add := func(kind domain.ContentKind, id string, author *string) {
		ref := domain.ContentRef{Kind: kind, ID: id}
		if len(w.commercial.store.state.ownership[ref]) > 0 || len(unowned) >= limit {
			return
		}
		item := ports.UnownedContent{Content: ref}
		if author != nil && *author != "" {
			user, err := w.users.GetByID(context.Background(), *author)
			item.Author = &domain.LegacyAuthor{ID: *author, IsAdmin: err == nil && user.Role == domain.RoleAdmin}
		}
		unowned = append(unowned, item)
	}
	for _, c := range w.courses.all() {
		add(domain.ContentCourse, c.ID, &c.CreatedBy)
	}
	for _, p := range w.paths.all() {
		add(domain.ContentPath, p.ID, &p.TeacherID)
	}
	for _, n := range w.nodes.all() {
		add(domain.ContentContentNode, n.ID, &n.TeacherID)
	}
	for _, e := range w.exercises.all() {
		add(domain.ContentExercise, e.ID, &e.CreatedBy)
	}
	return unowned, nil
}

func (s fakeOwnershipBackfillStore) RecordBackfilled(_ context.Context, record ports.OwnershipRecord) error {
	if len(s.w.commercial.store.state.ownership[record.Ownership.Content]) > 0 {
		return nil
	}
	return s.w.commercial.store.recordOwnership(record)
}
