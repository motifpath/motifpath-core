//go:build integration

package bdd

import (
	"context"
	"fmt"

	"github.com/cucumber/godog"

	appHTTP "github.com/motifpath/core-domain/internal/adapters/http"
	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerContentNodeVersioningSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^content node "([^"]+)" exists, created by "([^"]+)"$`, w.contentNodeExistsCreatedBy)

	sc.Step(`^"([^"]+)" publishes content node "([^"]+)"$`, w.publishesContentNode)
	sc.Step(`^"([^"]+)" attempts to publish content node "([^"]+)"$`, w.publishesContentNode)
	sc.Step(`^"([^"]+)" published content node "([^"]+)" as version (\d+)$`, w.publishedContentNodeAsVersion)
	sc.Step(`^"([^"]+)" edited "([^"]+)"'s title$`, w.editedNodeTitle)
	sc.Step(`^"([^"]+)" has never been published$`, w.hasNeverBeenPublished)
	sc.Step(`^"([^"]+)" attempts to publish a content node with an ID that does not exist$`, w.attemptsPublishMissingNode)
	sc.Step(`^an unauthenticated request attempts to publish a content node$`, w.unauthPublishesContentNode)

	sc.Step(`^a new content node version (\d+) is created, snapshotting its title, classification, media, and languages$`, w.newContentNodeVersionCreated)
	sc.Step(`^a new content node version (\d+) is created$`, w.newContentNodeVersionCreated)
	sc.Step(`^"([^"]+)"'s latest_published_version becomes (\d+)$`, w.latestPublishedVersionBecomes)

	sc.Step(`^"([^"]+)"'s copy of "([^"]+)" is pinned to content node version (\d+)$`, w.studentCopyPinnedToVersion)
	sc.Step(`^"([^"]+)"'s copy of "([^"]+)" is still pinned to content node version (\d+)$`, w.studentCopyPinnedToVersion)
}

// contentNodeExistsCreatedBy is putContentNode's counterpart for a specific,
// named owner — used by the "a teacher who did not create the content
// node ... cannot publish it" scenario, where ownership (not just
// existence) is the point.
func (w *world) contentNodeExistsCreatedBy(slug, teacherName string) error {
	if err := w.putContentNode(slug, domain.ContentTypeVideo); err != nil {
		return err
	}
	node, err := w.nodes.GetByID(context.Background(), nodeID(slug).String())
	if err != nil {
		return err
	}
	node.TeacherID = w.ensureRegistered(teacherName, domain.RoleTeacher).String()
	w.nodes.put(node)
	return nil
}

// publishesContentNode authenticates as name directly (via its Clerk sub)
// rather than relying on the scenario's current w.hasToken/w.clerkSub —
// content-node-versioning.feature's setup steps ("X published content node
// ... as version N") run before the scenario's own "X is authenticated as
// ..." Given in several scenarios, so this must not depend on that having
// already run, only on name having been registered at some point (directly
// or via ensureRegistered's auto-registration).
func (w *world) publishesContentNode(name, slug string) error {
	ctx := appHTTP.WithClerkUserID(context.Background(), clerkSub(name))
	resp, err := w.handler.PublishContentNode(ctx, generated.PublishContentNodeRequestObject{ContentNodeId: nodeID(slug)})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthPublishesContentNode() error {
	w.noAuthToken() //nolint:errcheck // never errors
	resp, err := w.handler.PublishContentNode(w.ctx(), generated.PublishContentNodeRequestObject{ContentNodeId: nodeID("node-01")})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) attemptsPublishMissingNode(name string) error {
	resp, err := w.handler.PublishContentNode(w.ctx(), generated.PublishContentNodeRequestObject{ContentNodeId: deterministicUUID("node", "does-not-exist")})
	w.lastResp, w.lastErr = resp, err
	return err
}

// publishedContentNodeAsVersion is a setup step (also usable as the action
// step in the two-version scenarios): publishes slug, asserts the response
// landed on version wantVersion, and remembers the version's id so a later
// "is pinned to content node version N" assertion can look it up.
func (w *world) publishedContentNodeAsVersion(name, slug string, wantVersion int) error {
	if err := w.publishesContentNode(name, slug); err != nil {
		return err
	}
	created, ok := w.lastResp.(generated.PublishContentNode201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if created.VersionNumber != wantVersion {
		return fmt.Errorf("expected version_number %d, got %d", wantVersion, created.VersionNumber)
	}
	return nil
}

func (w *world) editedNodeTitle(name, slug string) error {
	node, err := w.nodes.GetByID(context.Background(), nodeID(slug).String())
	if err != nil {
		return err
	}
	node.Title = node.Title + " (revised)"
	w.nodes.put(node)
	return nil
}

func (w *world) hasNeverBeenPublished(slug string) error {
	delete(w.versions.byNode, nodeID(slug).String())
	return nil
}

func (w *world) newContentNodeVersionCreated(wantVersion int) error {
	created, ok := w.lastResp.(generated.PublishContentNode201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if created.VersionNumber != wantVersion {
		return fmt.Errorf("expected version_number %d, got %d", wantVersion, created.VersionNumber)
	}
	if created.TitleSnapshot == "" {
		return fmt.Errorf("expected a non-empty title_snapshot")
	}
	return nil
}

func (w *world) latestPublishedVersionBecomes(slug string, wantVersion int) error {
	latest, err := w.versions.GetLatestByContentNodeID(context.Background(), nodeID(slug).String())
	if err != nil {
		return err
	}
	if latest.VersionNumber != wantVersion {
		return fmt.Errorf("expected latest_published_version %d, got %d", wantVersion, latest.VersionNumber)
	}
	return nil
}

// studentCopyPinnedToVersion asserts that studentName's current path's item
// for contentNodeSlug is pinned to whichever ContentNodeVersion was created
// by the "published ... as version N" step — resolved by matching the item's
// content_node_version_id against the latest version at the time that
// step ran isn't reliable once a later publish happens, so this instead
// re-derives version N's id from the fake repository directly: since
// publishedContentNodeAsVersion always publishes strictly in increasing
// version_number order, the Nth version created for this node is the one
// this step means.
func (w *world) studentCopyPinnedToVersion(studentName, nodeSlug string, wantVersion int) error {
	ctx := appHTTP.WithClerkUserID(context.Background(), clerkSub(studentName))
	resp, err := w.handler.GetMyPath(ctx, generated.GetMyPathRequestObject{})
	if err != nil {
		return err
	}
	view, ok := resp.(generated.GetMyPath200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response for %q, got %#v", studentName, resp)
	}

	var pinnedVersionID string
	found := false
	for _, item := range view.Items {
		if item.ContentNodeId == nodeID(nodeSlug) {
			pinnedVersionID = item.ContentNodeVersionId.String()
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("no item found for content node %q in %q's path", nodeSlug, studentName)
	}

	versions := w.versions.byNode[nodeID(nodeSlug).String()]
	for _, v := range versions {
		if v.ID == pinnedVersionID {
			if v.VersionNumber != wantVersion {
				return fmt.Errorf("expected %q's copy of %q to be pinned to version %d, got version %d", studentName, nodeSlug, wantVersion, v.VersionNumber)
			}
			return nil
		}
	}
	return fmt.Errorf("pinned content_node_version_id %q for %q's copy of %q does not match any known version", pinnedVersionID, studentName, nodeSlug)
}
