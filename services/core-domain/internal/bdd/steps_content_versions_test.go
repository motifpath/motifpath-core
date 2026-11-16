//go:build integration

package bdd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// releaseWorld is what the commercial release scenarios remember: each
// item's releases by revision as they were answered, and the copy a learner
// held before a release.
type releaseWorld struct {
	releases  map[string]map[int]generated.Release
	responses []releaseResponse
	copyBody  map[string][]byte
}

type releaseResponse struct {
	status int
	body   []byte
}

func newReleaseWorld() *releaseWorld {
	return &releaseWorld{releases: map[string]map[int]generated.Release{}, copyBody: map[string][]byte{}}
}

func registerContentVersionSteps(sc *godog.ScenarioContext, w *world) {
	// Releases, and the ownership they need, are rolled out for these
	// scenarios unless one says the rollout is OFF.
	sc.Before(func(ctx context.Context, scenario *godog.Scenario) (context.Context, error) {
		for _, tag := range scenario.Tags {
			if tag.Name == "@MOT-119" {
				w.setOwnershipRollout(true)
				w.setContentVersionsRollout(true)
			}
		}
		return ctx, nil
	})

	// Content
	sc.Step(`^(\w+) created and published the path "([^"]+)" with the lessons "([^"]+)" and "([^"]+)"$`, func(name, title, first, second string) error {
		return w.createPublishedPath(name, title, []string{first, second}, nil)
	})
	sc.Step(`^(\w+) created and published the path "([^"]+)" with the lesson "([^"]+)" containing her exercise "([^"]+)"$`, func(name, title, lesson, exercise string) error {
		if err := w.createOwnedExercise(name, exercise); err != nil {
			return err
		}
		return w.createPublishedPath(name, title, []string{lesson}, map[string]string{lesson: exercise})
	})
	sc.Step(`^(\w+) created and published the path "([^"]+)" with the lesson "([^"]+)" containing "([^"]+)"$`, func(name, title, lesson, exercise string) error {
		return w.createPublishedPath(name, title, []string{lesson}, map[string]string{lesson: exercise})
	})
	sc.Step(`^(\w+) created a course "([^"]+)" whose published version (\d+) names the path "([^"]+)"$`, w.createCourseWithVersion)
	sc.Step(`^(\w+) created the path "([^"]+)" without publishing it$`, func(name, title string) error { return w.createOwnedPath(name, title, nil) })
	sc.Step(`^a published path with no recorded creator exists with unresolved ownership$`, w.unresolvedPublishedPath)
	sc.Step(`^(\w+) (?:renames|renamed) "([^"]+)" to "([^"]+)"$`, w.renamePath)
	sc.Step(`^(\w+) unpublished and deleted "([^"]+)"$`, w.unpublishAndDelete)
	sc.Step(`^(\w+) corrects the ownership of "([^"]+)" to teacher (\w+)$`, func(name, title, teacher string) error {
		if err := w.correctOwnership(name, title, "teacher", w.userID(teacher), 1, "rights reviewed", ""); err != nil {
			return err
		}
		return w.commercialResponseIs(http.StatusOK)
	})
	sc.Step(`^the content-versions rollout is OFF$`, func() error { w.setContentVersionsRollout(false); return nil })

	// Publishing
	sc.Step(`^(\w+) publishes a commercial release of "([^"]+)" at commercial revision (\d+)$`, func(name, title string, revision int) error {
		return w.publishRelease(name, w.ownership.content[title], revision, "")
	})
	sc.Step(`^(\w+) publishes a commercial release of that path at commercial revision (\d+)$`, func(name string, revision int) error {
		return w.publishRelease(name, w.ownership.lastLegacy, revision, "")
	})
	sc.Step(`^(\w+) released the path "([^"]+)"$`, func(name, title string) error { return w.releasedPath(name, title, "") })
	sc.Step(`^(\w+) released the path "([^"]+)" with idempotency key (\w+)$`, w.releasedPath)
	sc.Step(`^(\w+) sends the same release request again with idempotency key (\w+)$`, func(_, _ string) error {
		last := w.commercial.lastRequest
		return w.commercialSend(last.persona, last.method, last.path, last.key, last.body)
	})
	sc.Step(`^(\w+) and (\w+) publish a commercial release of "([^"]+)" at the same time with different idempotency keys$`, w.publishTogether)

	// Reading
	sc.Step(`^(\w+) reads release revision (\d+) of "([^"]+)"$`, w.readRelease)

	// Outcomes
	sc.Step(`^"([^"]+)" has release revision (\d+) with (\w+) as the recorded owner$`, w.releasedWithOwner)
	sc.Step(`^the release lists "([^"]+)" and "([^"]+)" as dependencies (\w+) owns$`, func(first, second, owner string) error {
		return w.dependenciesOwnedBy([]string{first, second}, "teacher", w.userID(owner))
	})
	sc.Step(`^the release lists "([^"]+)" as a dependency the platform owns$`, func(title string) error {
		return w.dependenciesOwnedBy([]string{title}, "platform", domain.CatalogProfileID)
	})
	sc.Step(`^the commercial revision of "([^"]+)" is (?:still )?(\d+)$`, w.commercialRevisionIs)
	sc.Step(`^"([^"]+)" and "([^"]+)" each have release revision (\d+)$`, func(first, second string, revision int) error {
		for _, title := range []string{first, second} {
			if err := w.latestReleaseRevisionIs(title, revision); err != nil {
				return err
			}
		}
		return nil
	})
	sc.Step(`^the release of "([^"]+)" lists both of those releases$`, w.listsDependencyReleases)
	sc.Step(`^the release of "([^"]+)" names published version (\d+)$`, w.namesCourseVersion)
	sc.Step(`^release revision (\d+) of "([^"]+)" still has the title "([^"]+)"$`, w.releaseHasTitle)
	sc.Step(`^"([^"]+)" has release revision (\d+) with the title "([^"]+)"$`, func(title string, revision int, released string) error {
		if err := w.releasedWithOwner(title, revision, ""); err != nil {
			return err
		}
		return w.releaseHasTitle(revision, title, released)
	})
	sc.Step(`^release revision (\d+) can still be read with the title "([^"]+)"$`, w.releaseReadableWithTitle)
	sc.Step(`^the response returns release revision (\d+)$`, w.responseIsReleaseRevision)
	sc.Step(`^"([^"]+)" has exactly one release, revision (\d+)$`, w.exactlyOneRelease)
	sc.Step(`^release revision (\d+) of "([^"]+)" still records the platform as owner$`, w.releaseRecordsPlatform)
	sc.Step(`^(\w+) can read that release$`, w.canReadLastRelease)
	sc.Step(`^the release is returned unchanged$`, w.releaseReturnedUnchanged)
	sc.Step(`^(\w+)'s copy of "([^"]+)" is unchanged and has no update available$`, w.copyUnchanged)
	sc.Step(`^the release is refused because the path is not published$`, func() error { return w.refusedWith("not published") })
	sc.Step(`^the release is refused because the ownership is unresolved$`, func() error { return w.refusedWith("unresolved ownership") })
	sc.Step(`^the release is refused naming "([^"]+)" as a dependency (\w+) does not own$`, func(title, _ string) error {
		return w.refusedWith(title, w.ownership.content[title].ID, "another rights holder")
	})
	sc.Step(`^"([^"]+)" has no release$`, func(title string) error { return w.hasNoRelease(title) })
	sc.Step(`^neither "([^"]+)", "([^"]+)" nor "([^"]+)" has a release$`, func(first, second, third string) error {
		for _, title := range []string{first, second, third} {
			if err := w.hasNoRelease(title); err != nil {
				return err
			}
		}
		return nil
	})
	sc.Step(`^"([^"]+)" still has only release revision (\d+)$`, func(title string, revision int) error {
		releases := w.commercial.store.releasesOf(w.ownership.content[title])
		if len(releases) != 1 || releases[0].Revision != revision {
			return fmt.Errorf("%q has %d releases, want only revision %d", title, len(releases), revision)
		}
		return nil
	})
}

func (w *world) setContentVersionsRollout(on bool) {
	w.commercial.store.state.flags[domain.FlagName("content_versions")] = domain.FlagSetting{
		Enabled: on, Audience: domain.RolloutAudience{AllUsers: on},
	}
}

// ── Content as the scenarios describe it ─────────────────────────────────────

// createPublishedPath creates and publishes one lesson per title, embedding
// any exercise named for it, then a path of those lessons that an
// administrator publishes.
func (w *world) createPublishedPath(name, title string, lessons []string, exercises map[string]string) error {
	items := make([]map[string]any, 0, len(lessons))
	for _, lessonTitle := range lessons {
		lesson, err := w.createLesson(name, lessonTitle)
		if err != nil {
			return err
		}
		w.ownership.content[lessonTitle] = domain.ContentRef{Kind: domain.ContentContentNode, ID: lesson.String()}
		if exercise, ok := exercises[lessonTitle]; ok {
			if err := w.embed(name, lesson.String(), exercise); err != nil {
				return err
			}
		}
		if err := w.publish(name, "/content-nodes/"+lesson.String()+"/publish"); err != nil {
			return err
		}
		items = append(items, map[string]any{"content_node_id": lesson})
	}
	body := map[string]any{
		"title": title, "summary": "A complete path", "language": "en", "level": "beginner", "instrument_ids": []string{}, "items": items,
	}
	id, err := w.create(name, "/learning-paths", body)
	if err != nil {
		return err
	}
	w.ownership.content[title] = domain.ContentRef{Kind: domain.ContentPath, ID: id.String()}
	return w.publish(w.anyAdmin(), "/learning-paths/"+id.String()+"/publish")
}

func (w *world) embed(name, lessonID, exercise string) error {
	ref, ok := w.ownership.content[exercise]
	if !ok {
		return fmt.Errorf("no exercise %q was created", exercise)
	}
	status, raw, err := w.send(name, http.MethodPost, "/content-nodes/"+lessonID+"/exercises/"+ref.ID, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return fmt.Errorf("embed %q: %d %s", exercise, status, raw)
	}
	return nil
}

// createCourseWithVersion publishes the path, then publishes a course naming
// it until the course reaches the stated version.
func (w *world) createCourseWithVersion(name, title string, version int, pathTitle string) error {
	if err := w.createPublishedPath(name, pathTitle, []string{pathTitle + " lesson"}, nil); err != nil {
		return err
	}
	path := w.ownership.content[pathTitle]
	id, err := w.create(name, "/courses", map[string]any{
		"title": title, "summary": "Start here.", "level": "beginner", "language": "en", "instrument_ids": []string{},
		"checkpoints": []map[string]any{{"learning_path_id": path.ID}},
	})
	if err != nil {
		return err
	}
	w.ownership.content[title] = domain.ContentRef{Kind: domain.ContentCourse, ID: id.String()}
	for i := 0; i < version; i++ {
		if err := w.publish(name, "/courses/"+id.String()+"/publish"); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) unresolvedPublishedPath() error {
	id := uuid.NewString()
	w.paths.put(domain.LearningPath{ID: id, Title: "Legacy path", Status: domain.LearningPathStatusPublished, CreatedAt: fixedNow})
	ref := domain.ContentRef{Kind: domain.ContentPath, ID: id}
	w.commercial.store.state.ownership[ref] = []domain.ContentOwnership{domain.LegacyOwnership(ref, nil, uuid.NewString())}
	w.ownership.lastLegacy = ref
	return nil
}

// renamePath renames a path and keeps its lessons.
func (w *world) renamePath(name, title, renamed string) error {
	ref := w.ownership.content[title]
	path, err := w.paths.GetByID(context.Background(), ref.ID)
	if err != nil {
		return err
	}
	items := make([]map[string]any, 0, len(path.Items))
	for _, item := range path.Items {
		items = append(items, map[string]any{"content_node_id": item.ContentNodeID})
	}
	body := map[string]any{
		"title": renamed, "summary": path.Summary, "language": path.Language, "level": "beginner", "instrument_ids": []string{}, "items": items,
	}
	status, raw, err := w.send(name, http.MethodPut, "/learning-paths/"+ref.ID, body)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("rename %q: %d %s", title, status, raw)
	}
	w.ownership.content[renamed] = ref
	if w.release.releases[title] != nil {
		w.release.releases[renamed] = w.release.releases[title]
	}
	return nil
}

func (w *world) unpublishAndDelete(name, title string) error {
	ref := w.ownership.content[title]
	if err := w.publish(w.anyAdmin(), "/learning-paths/"+ref.ID+"/unpublish"); err != nil {
		return err
	}
	return w.deleteOwnedPath(name, title)
}

// ── Publishing and reading releases ──────────────────────────────────────────

func (w *world) publishRelease(name string, ref domain.ContentRef, revision int, key string) error {
	body, err := json.Marshal(map[string]any{"expected_revision": revision, "reason": "Offer for sale"})
	if err != nil {
		return err
	}
	path := "/commercial-content/" + string(ref.Kind) + "/" + ref.ID + "/releases"
	if err := w.commercialSend(name, http.MethodPost, path, idempotencyKey(key), body); err != nil {
		return err
	}
	w.release.responses = append(w.release.responses, releaseResponse{status: w.commercial.lastStatus, body: w.commercial.lastBody})
	if w.commercial.lastStatus != http.StatusOK {
		return nil
	}
	release, err := w.lastRelease()
	if err != nil {
		return err
	}
	w.rememberRelease(release)
	return nil
}

func (w *world) rememberRelease(release generated.Release) {
	for title, ref := range w.ownership.content {
		if ref.ID != release.Content.ContentId.String() {
			continue
		}
		if w.release.releases[title] == nil {
			w.release.releases[title] = map[int]generated.Release{}
		}
		w.release.releases[title][release.Revision] = release
	}
}

func (w *world) releasedPath(name, title, key string) error {
	if err := w.createAndPublishPath(name, title); err != nil {
		return err
	}
	if err := w.publishRelease(name, w.ownership.content[title], 1, key); err != nil {
		return err
	}
	if err := w.commercialResponseIs(http.StatusOK); err != nil {
		return err
	}
	w.ownership.firstResult = w.commercial.lastBody
	return nil
}

// publishTogether sends both publications against the same observed
// revision, as two callers who read the item at the same moment would.
func (w *world) publishTogether(first, second, title string) error {
	w.release.responses = nil
	for _, name := range []string{first, second} {
		if err := w.publishRelease(name, w.ownership.content[title], 1, ""); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) lastRelease() (generated.Release, error) {
	var release generated.Release
	if err := json.Unmarshal(w.commercial.lastBody, &release); err != nil {
		return generated.Release{}, err
	}
	return release, nil
}

func (w *world) remembered(title string, revision int) (generated.Release, error) {
	release, ok := w.release.releases[title][revision]
	if !ok {
		return generated.Release{}, fmt.Errorf("no release revision %d of %q was answered", revision, title)
	}
	return release, nil
}

func (w *world) readRelease(name string, revision int, title string) error {
	release, err := w.remembered(title, revision)
	if err != nil {
		return err
	}
	return w.commercialSend(name, http.MethodGet, "/commercial-releases/"+release.ReleaseId.String(), "", nil)
}

// ── Outcomes ─────────────────────────────────────────────────────────────────

func (w *world) releasedWithOwner(title string, revision int, owner string) error {
	if err := w.commercialResponseIs(http.StatusOK); err != nil {
		return err
	}
	release, err := w.lastRelease()
	if err != nil {
		return err
	}
	if release.Content.ContentId.String() != w.ownership.content[title].ID || release.Revision != revision {
		return fmt.Errorf("the release is revision %d of %s, want revision %d of %q", release.Revision, release.Content.ContentId, revision, title)
	}
	if owner != "" && (release.Ownership.OwnerId == nil || release.Ownership.OwnerId.String() != w.userID(owner)) {
		return fmt.Errorf("the release records owner %v, want %s", release.Ownership.OwnerId, owner)
	}
	return nil
}

func (w *world) dependenciesOwnedBy(titles []string, ownerKind, ownerID string) error {
	release, err := w.lastRelease()
	if err != nil {
		return err
	}
	for _, title := range titles {
		ref := w.ownership.content[title]
		found := false
		for _, dependency := range release.Dependencies {
			if dependency.Content.ContentId.String() != ref.ID {
				continue
			}
			found = true
			if dependency.Permission != generated.Own {
				return fmt.Errorf("%q is included as %s, want own", title, dependency.Permission)
			}
			stored, err := w.commercial.store.Release(context.Background(), dependency.ReleaseId.String())
			if err != nil {
				return fmt.Errorf("%q has no release of its own: %w", title, err)
			}
			if string(stored.Ownership.OwnerKind) != ownerKind || stored.Ownership.OwnerID == nil || *stored.Ownership.OwnerID != ownerID {
				return fmt.Errorf("%q's release records %s %v, want %s %s", title, stored.Ownership.OwnerKind, stored.Ownership.OwnerID, ownerKind, ownerID)
			}
		}
		if !found {
			return fmt.Errorf("the release does not list %q", title)
		}
	}
	return nil
}

func (w *world) commercialContentOf(title string) (generated.CommercialContent, error) {
	ref := w.ownership.content[title]
	status, raw, err := w.send(w.anyAdmin(), http.MethodGet, "/commercial-content/"+string(ref.Kind)+"/"+ref.ID, nil)
	if err != nil {
		return generated.CommercialContent{}, err
	}
	if status != http.StatusOK {
		return generated.CommercialContent{}, fmt.Errorf("read %q: %d %s", title, status, raw)
	}
	var content generated.CommercialContent
	return content, json.Unmarshal(raw, &content)
}

func (w *world) commercialRevisionIs(title string, revision int) error {
	content, err := w.commercialContentOf(title)
	if err != nil {
		return err
	}
	if content.Revision != revision {
		return fmt.Errorf("%q is at commercial revision %d, want %d", title, content.Revision, revision)
	}
	return nil
}

func (w *world) latestReleaseRevisionIs(title string, revision int) error {
	content, err := w.commercialContentOf(title)
	if err != nil {
		return err
	}
	if content.Release == nil || content.Release.Revision != revision {
		return fmt.Errorf("%q's latest release is %v, want revision %d", title, content.Release, revision)
	}
	return nil
}

func (w *world) listsDependencyReleases(title string) error {
	release, err := w.lastRelease()
	if err != nil {
		return err
	}
	for _, dependency := range []string{"Shells", "Shell voicings in C"} {
		content, err := w.commercialContentOf(dependency)
		if err != nil {
			return err
		}
		if content.Release == nil || !containsUUID(release.DependencyReleaseIds, content.Release.ReleaseId) {
			return fmt.Errorf("the release of %q does not list the release of %q", title, dependency)
		}
	}
	return nil
}

func containsUUID(ids []uuid.UUID, id uuid.UUID) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

func (w *world) namesCourseVersion(title string, version int) error {
	release, err := w.lastRelease()
	if err != nil {
		return err
	}
	published, err := w.courseVersions.GetByCourseIDAndVersionNumber(context.Background(), w.ownership.content[title].ID, version)
	if err != nil {
		return err
	}
	if release.CoreVersionRef == nil || *release.CoreVersionRef != published.ID {
		return fmt.Errorf("the release names version %v, want %s (version %d)", release.CoreVersionRef, published.ID, version)
	}
	return nil
}

// releaseHasTitle checks what a release covers: its content digest is the
// digest of the path as it is now, but titled as stated.
func (w *world) releaseHasTitle(revision int, title, released string) error {
	release, err := w.remembered(title, revision)
	if err != nil {
		return err
	}
	digest, err := w.pathDigestTitled(w.ownership.content[title], released)
	if err != nil {
		return err
	}
	if release.ContentSha256 != digest {
		return fmt.Errorf("release revision %d of %q does not cover the title %q", revision, title, released)
	}
	return nil
}

func (w *world) pathDigestTitled(ref domain.ContentRef, title string) (string, error) {
	snapshot, err := w.releaseSnapshot(ref)
	if err != nil {
		return "", err
	}
	state, ok := snapshot.State.(domain.PathReleaseState)
	if !ok {
		return "", fmt.Errorf("%s %s is not a path", ref.Kind, ref.ID)
	}
	state.Title = title
	snapshot.State = state
	return snapshot.ContentSHA256()
}

func (w *world) releaseReadableWithTitle(revision int, title string) error {
	var path string
	for name := range w.release.releases {
		if _, ok := w.release.releases[name][revision]; ok && w.ownership.content[name].Kind == domain.ContentPath {
			path = name
			break
		}
	}
	if err := w.readRelease(w.anyAdmin(), revision, path); err != nil {
		return err
	}
	if err := w.releaseReturnedUnchangedAs(path, revision); err != nil {
		return err
	}
	return w.releaseHasTitle(revision, path, title)
}

func (w *world) responseIsReleaseRevision(revision int) error {
	if err := w.commercialResponseIs(http.StatusOK); err != nil {
		return err
	}
	release, err := w.lastRelease()
	if err != nil {
		return err
	}
	if release.Revision != revision {
		return fmt.Errorf("the response is release revision %d, want %d", release.Revision, revision)
	}
	for title := range w.release.releases {
		if original, ok := w.release.releases[title][revision]; ok && original.Content == release.Content && original.ReleaseId != release.ReleaseId {
			return fmt.Errorf("the response is a new release %s, not %s", release.ReleaseId, original.ReleaseId)
		}
	}
	return nil
}

func (w *world) exactlyOneRelease(title string, revision int) error {
	var answered []uuid.UUID
	for _, response := range w.release.responses {
		if response.status != http.StatusOK {
			return fmt.Errorf("a publication answered %d %s", response.status, response.body)
		}
		var release generated.Release
		if err := json.Unmarshal(response.body, &release); err != nil {
			return err
		}
		answered = append(answered, release.ReleaseId)
	}
	stored := w.commercial.store.releasesOf(w.ownership.content[title])
	if len(stored) != 1 || stored[0].Revision != revision {
		return fmt.Errorf("%q has %d releases, want exactly revision %d", title, len(stored), revision)
	}
	for _, id := range answered {
		if id.String() != stored[0].ReleaseID {
			return fmt.Errorf("a publication answered release %s, not the one stored", id)
		}
	}
	return nil
}

func (w *world) releaseRecordsPlatform(revision int, title string) error {
	if err := w.readRelease(w.anyAdmin(), revision, title); err != nil {
		return err
	}
	if err := w.commercialResponseIs(http.StatusOK); err != nil {
		return err
	}
	release, err := w.lastRelease()
	if err != nil {
		return err
	}
	if release.Ownership.OwnerKind != generated.OwnershipOwnerKindPlatform {
		return fmt.Errorf("release revision %d of %q records %s as owner kind", revision, title, release.Ownership.OwnerKind)
	}
	return nil
}

func (w *world) canReadLastRelease(name string) error {
	last := w.commercial.lastRequest
	if err := w.commercialSend(name, http.MethodGet, last.path, "", nil); err != nil {
		return err
	}
	return w.commercialResponseIs(http.StatusOK)
}

func (w *world) releaseReturnedUnchanged() error {
	for title, revisions := range w.release.releases {
		for revision, release := range revisions {
			if strings.HasSuffix(w.commercial.lastRequest.path, release.ReleaseId.String()) {
				return w.releaseReturnedUnchangedAs(title, revision)
			}
		}
	}
	return errors.New("the scenario read no remembered release")
}

func (w *world) releaseReturnedUnchangedAs(title string, revision int) error {
	if err := w.commercialResponseIs(http.StatusOK); err != nil {
		return err
	}
	got, err := w.lastRelease()
	if err != nil {
		return err
	}
	original, err := w.remembered(title, revision)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(got, original) {
		return fmt.Errorf("release revision %d of %q changed:\n got %+v\nwant %+v", revision, title, got, original)
	}
	return nil
}

func (w *world) copyUnchanged(name, title string) error {
	if err := w.commercialResponseIs(http.StatusOK); err != nil {
		return err
	}
	copyID := w.ownership.copies[name]
	status, raw, err := w.send(name, http.MethodGet, "/students/me/student-paths/"+copyID, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("%s reads their copy of %q: %d %s", name, title, status, raw)
	}
	var copied generated.StudentPath
	if err := json.Unmarshal(raw, &copied); err != nil {
		return err
	}
	if copied.UpdateAvailable {
		return fmt.Errorf("%s's copy of %q has an update available", name, title)
	}
	if before := w.release.copyBody[name]; !bytes.Equal(bytes.TrimSpace(before), bytes.TrimSpace(raw)) {
		return fmt.Errorf("%s's copy of %q changed:\n%s\nwas\n%s", name, title, raw, before)
	}
	return nil
}

// rememberCopy keeps how a learner's copy reads now, to compare later.
func (w *world) rememberCopy(name string) error {
	status, raw, err := w.send(name, http.MethodGet, "/students/me/student-paths/"+w.ownership.copies[name], nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("%s reads their copy: %d %s", name, status, raw)
	}
	w.release.copyBody[name] = raw
	return nil
}

func (w *world) refusedWith(fragments ...string) error {
	if err := w.commercialResponseIs(http.StatusConflict); err != nil {
		return err
	}
	for _, fragment := range fragments {
		if !strings.Contains(string(w.commercial.lastBody), fragment) {
			return fmt.Errorf("the refusal %s does not mention %q", w.commercial.lastBody, fragment)
		}
	}
	return nil
}

func (w *world) hasNoRelease(title string) error {
	ref, ok := w.ownership.content[title]
	if !ok {
		return fmt.Errorf("no content %q was created", title)
	}
	if releases := w.commercial.store.releasesOf(ref); len(releases) != 0 {
		return fmt.Errorf("%q has %d releases", title, len(releases))
	}
	content, err := w.commercialContentOf(title)
	if err != nil {
		return err
	}
	if content.Release != nil {
		return fmt.Errorf("%q reports release %s", title, content.Release.ReleaseId)
	}
	return nil
}

// ── The world's content as a release reads it ────────────────────────────────

// releaseSnapshot reads an item's releasable state from the world's content
// repositories, as the Postgres store reads it from its tables.
func (w *world) releaseSnapshot(content domain.ContentRef) (domain.ReleaseSnapshot, error) {
	ctx := context.Background()
	switch content.Kind {
	case domain.ContentCourse:
		return w.courseReleaseSnapshot(ctx, content)
	case domain.ContentPath:
		return w.pathReleaseSnapshot(ctx, content)
	case domain.ContentContentNode:
		return w.lessonReleaseSnapshot(ctx, content)
	case domain.ContentExercise:
		return w.exerciseReleaseSnapshot(ctx, content)
	}
	return domain.ReleaseSnapshot{}, domain.ErrNotFound
}

func (w *world) courseReleaseSnapshot(ctx context.Context, content domain.ContentRef) (domain.ReleaseSnapshot, error) {
	course, err := w.courses.GetByID(ctx, content.ID)
	if err != nil {
		return domain.ReleaseSnapshot{}, err
	}
	version, err := w.courseVersions.GetLatestByCourseID(ctx, content.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ReleaseSnapshot{Content: content, Title: course.Title, State: domain.CourseReleaseState{Title: course.Title}}, nil
	}
	if err != nil {
		return domain.ReleaseSnapshot{}, err
	}
	state := domain.CourseReleaseState{
		VersionNumber: version.VersionNumber, Title: version.TitleSnapshot, Summary: version.SummarySnapshot,
		Level: string(version.LevelSnapshot), Language: version.LanguageSnapshot, Instruments: version.InstrumentIDsSnapshot,
		ThumbnailURL: version.ThumbnailURLSnapshot,
	}
	var children []domain.ContentRef
	for _, checkpoint := range version.Checkpoints {
		state.Checkpoints = append(state.Checkpoints, domain.CourseReleaseCheckpoint{PathID: checkpoint.LearningPathID, EffectiveTitle: checkpoint.EffectiveTitle})
		children = append(children, domain.ContentRef{Kind: domain.ContentPath, ID: checkpoint.LearningPathID})
	}
	versionRef := version.ID
	return domain.ReleaseSnapshot{Content: content, Title: course.Title, Published: true, VersionRef: &versionRef, State: state, Children: children}, nil
}

func (w *world) pathReleaseSnapshot(ctx context.Context, content domain.ContentRef) (domain.ReleaseSnapshot, error) {
	path, err := w.paths.GetByID(ctx, content.ID)
	if err != nil {
		return domain.ReleaseSnapshot{}, err
	}
	var level *string
	if path.Level != nil {
		value := string(*path.Level)
		level = &value
	}
	state := domain.PathReleaseState{
		Title: path.Title, Summary: path.Summary, Level: level, Language: path.Language,
		Instruments: path.InstrumentIDs, ThumbnailURL: path.ThumbnailURL,
	}
	var children []domain.ContentRef
	for _, item := range path.Items {
		state.Items = append(state.Items, domain.PathReleaseItem{LessonID: item.ContentNodeID, SectionLabel: item.SectionLabel})
		children = append(children, domain.ContentRef{Kind: domain.ContentContentNode, ID: item.ContentNodeID})
	}
	return domain.ReleaseSnapshot{
		Content: content, Title: path.Title, Published: path.Status == domain.LearningPathStatusPublished, State: state, Children: children,
	}, nil
}

func (w *world) lessonReleaseSnapshot(ctx context.Context, content domain.ContentRef) (domain.ReleaseSnapshot, error) {
	node, err := w.nodes.GetByID(ctx, content.ID)
	if err != nil {
		return domain.ReleaseSnapshot{}, err
	}
	w.exercises.mu.Lock()
	linked := append([]string{}, w.exercises.byNodeOrder[content.ID]...)
	w.exercises.mu.Unlock()
	children := make([]domain.ContentRef, 0, len(linked))
	for _, exerciseID := range linked {
		children = append(children, domain.ContentRef{Kind: domain.ContentExercise, ID: exerciseID})
	}
	version, err := w.versions.GetLatestByContentNodeID(ctx, content.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ReleaseSnapshot{Content: content, Title: node.Title, State: domain.LessonReleaseState{Title: node.Title}, Children: children}, nil
	}
	if err != nil {
		return domain.ReleaseSnapshot{}, err
	}
	versionRef := version.ID
	return domain.ReleaseSnapshot{
		Content: content, Title: node.Title, Published: true, VersionRef: &versionRef, Children: children,
		State: domain.LessonReleaseState{
			VersionNumber: version.VersionNumber, Title: version.Title, ContentType: string(version.ContentType),
			MediaURL: version.MediaURL, Instruments: version.InstrumentIDsSnapshot, ThumbnailURL: version.ThumbnailURLSnapshot,
		},
	}, nil
}

func (w *world) exerciseReleaseSnapshot(ctx context.Context, content domain.ContentRef) (domain.ReleaseSnapshot, error) {
	exercise, err := w.exercises.GetByID(ctx, content.ID)
	if err != nil {
		return domain.ReleaseSnapshot{}, err
	}
	prompt, err := json.Marshal(exercise.Prompt)
	if err != nil {
		return domain.ReleaseSnapshot{}, err
	}
	state := domain.ExerciseReleaseState{
		Title: exercise.Title, Prompt: string(prompt), ExerciseType: string(exercise.ExerciseType),
		ImageURL: exercise.ImageURL, AudioURL: exercise.AudioURL, EstimatedDurationSeconds: exercise.EstimatedDurationSeconds,
	}
	for _, option := range exercise.Options {
		state.Options = append(state.Options, domain.ExerciseReleaseOption{OptionID: option.ID, Label: option.Label, IsCorrect: option.IsCorrect})
	}
	return domain.ReleaseSnapshot{Content: content, Title: exercise.Title, Published: true, State: state}, nil
}
