package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// ActionPublishCommercialRelease is the evidence action of a release
// publication.
const ActionPublishCommercialRelease = "publish_commercial_release"

// releasePolicyRevision is the release policy publications are decided under.
const releasePolicyRevision = "commercial-releases-r1"

const releasePublicationIntentVersion = 1

// PublishReleaseRequest asks for an item's release, against the commercial
// revision the caller observed.
type PublishReleaseRequest struct {
	ExpectedRevision int    `json:"expected_revision"`
	Reason           string `json:"reason"`
}

// releaseReceipt is what a publication records as its result. A replay reads
// the release it names, which never changes.
type releaseReceipt struct {
	ReleaseID string `json:"release_id"`
}

// PublishRelease releases an item and every item it depends on, for its
// current owner or an administrator while the content-versions rollout
// allows them. An unchanged manifest returns the latest release; otherwise a
// stale observed revision is a conflict. A same-intent replay returns the
// original release.
func (s *CommercialContentService) PublishRelease(ctx context.Context, caller domain.User, kind, id, idempotencyKey string, request PublishReleaseRequest) (domain.CommercialRelease, error) {
	content, err := parseContentRef(kind, id)
	if err != nil {
		return domain.CommercialRelease{}, err
	}
	if request.ExpectedRevision < 1 {
		return domain.CommercialRelease{}, domain.NewValidationError("expected_revision", "must be at least 1")
	}
	if strings.TrimSpace(request.Reason) == "" {
		return domain.CommercialRelease{}, domain.NewValidationError("reason", "a publication states its reason")
	}
	body, err := json.Marshal(request)
	if err != nil {
		return domain.CommercialRelease{}, domain.NewValidationError("body", err.Error())
	}
	intent := CommercialIntent{
		Key: domain.OperationKey{
			Environment: s.environment, ActorKind: domain.AuditActorUser, ActorID: caller.ID,
			Action: ActionPublishCommercialRelease, ResourceKind: string(content.Kind), ResourceID: content.ID,
			CallerKey: idempotencyKey,
		},
		Version: releasePublicationIntentVersion, Body: body, Reason: request.Reason, PolicyRevision: releasePolicyRevision,
	}
	authorize := func(context.Context) error {
		if caller.Role != domain.RoleAdmin && caller.Role != domain.RoleTeacher {
			return fmt.Errorf("%w: only an owner or an administrator publishes a release", domain.ErrForbidden)
		}
		return nil
	}
	receipt, err := s.commands.Run(ctx, intent, authorize, s.releaseEffect(caller, content, request.ExpectedRevision))
	if err != nil {
		return domain.CommercialRelease{}, err
	}
	var result releaseReceipt
	if err := json.Unmarshal(receipt.Result, &result); err != nil {
		return domain.CommercialRelease{}, fmt.Errorf("%w: recorded receipt is unreadable: %w", domain.ErrUnavailable, err)
	}
	release, err := s.store.Release(ctx, result.ReleaseID)
	if err != nil {
		return domain.CommercialRelease{}, fmt.Errorf("%w: recorded release %s: %w", domain.ErrUnavailable, result.ReleaseID, err)
	}
	return release, nil
}

// GetRelease reads a stored release for an administrator, the item's current
// owner or the owner the release recorded, whatever the rollout state and
// after the item is deleted. Anyone else cannot tell it exists.
func (s *CommercialContentService) GetRelease(ctx context.Context, caller domain.User, releaseID string) (domain.CommercialRelease, error) {
	release, err := s.store.Release(ctx, releaseID)
	if err != nil {
		return domain.CommercialRelease{}, err
	}
	if caller.Role == domain.RoleAdmin || canReadCommercialContent(caller, release.Ownership) {
		return release, nil
	}
	current, err := s.store.CurrentOwnership(ctx, release.Content)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.CommercialRelease{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.CommercialRelease{}, err
	}
	if !canReadCommercialContent(caller, current) {
		return domain.CommercialRelease{}, domain.ErrNotFound
	}
	return release, nil
}

// releaseItem is one item of a publication's dependency graph.
type releaseItem struct {
	snapshot  domain.ReleaseSnapshot
	ownership domain.ContentOwnership
	state     domain.CommercialState
}

func (s *CommercialContentService) releaseEffect(caller domain.User, content domain.ContentRef, expectedRevision int) CommercialEffect {
	return func(ctx context.Context, tx ports.CommercialTx, run CommercialRun) (CommercialEffectResult, error) {
		revision, err := s.rollout.AuthorizeIntent(ctx, tx, domain.FlagName("content_versions"), domain.RolloutSubject{Kind: domain.RolloutSubjectUser, ID: caller.ID})
		if err != nil {
			return CommercialEffectResult{}, err
		}
		order, items, err := prepareRelease(ctx, tx, caller, content)
		if err != nil {
			return CommercialEffectResult{}, err
		}
		before := releaseStateView(items[content].state)
		release, err := s.releaseGraph(ctx, tx, run, order, items, expectedRevision)
		if err != nil {
			return CommercialEffectResult{}, err
		}
		result, err := json.Marshal(releaseReceipt{ReleaseID: release.ReleaseID})
		if err != nil {
			return CommercialEffectResult{}, err
		}
		return CommercialEffectResult{
			Result: result, AuthorizedRevision: revision, ConfigurationRevision: ownershipConfigurationRevision,
			Before: before, After: releaseView(release),
		}, nil
	}
}

// prepareRelease reads and locks the item's dependency graph, then refuses
// a caller who may not publish it and anything that cannot be released.
func prepareRelease(ctx context.Context, tx ports.CommercialTx, caller domain.User, content domain.ContentRef) ([]domain.ContentRef, map[domain.ContentRef]*releaseItem, error) {
	order, items, err := readReleaseGraph(ctx, tx, content)
	if err != nil {
		return nil, nil, err
	}
	if err := lockReleaseGraph(ctx, tx, items); err != nil {
		return nil, nil, err
	}
	root := items[content]
	if !canPublishRelease(caller, root.ownership) {
		return nil, nil, fmt.Errorf("%w: only the item's owner or an administrator publishes its release", domain.ErrForbidden)
	}
	for _, ref := range order {
		if err := domain.CheckReleasable(items[ref].snapshot, items[ref].ownership, root.ownership); err != nil {
			return nil, nil, err
		}
	}
	return order, items, nil
}

func canPublishRelease(caller domain.User, ownership domain.ContentOwnership) bool {
	return caller.Role == domain.RoleAdmin || canReadCommercialContent(caller, ownership)
}

// readReleaseGraph reads the item and every transitive dependency, nearest
// first. A missing root is not found; a missing dependency refuses the
// release, since what learners get would be incomplete.
func readReleaseGraph(ctx context.Context, tx ports.CommercialTx, root domain.ContentRef) ([]domain.ContentRef, map[domain.ContentRef]*releaseItem, error) {
	order := []domain.ContentRef{}
	items := map[domain.ContentRef]*releaseItem{}
	queue := []domain.ContentRef{root}
	for len(queue) > 0 {
		ref := queue[0]
		queue = queue[1:]
		if _, seen := items[ref]; seen {
			continue
		}
		snapshot, err := tx.Releases().Snapshot(ctx, ref)
		if errors.Is(err, domain.ErrNotFound) && ref != root {
			return nil, nil, fmt.Errorf("%w: dependency %s %s no longer exists", domain.ErrConflict, ref.Kind, ref.ID)
		}
		if err != nil {
			return nil, nil, err
		}
		items[ref] = &releaseItem{snapshot: snapshot}
		order = append(order, ref)
		queue = append(queue, snapshot.Children...)
	}
	return order, items, nil
}

// lockReleaseGraph locks every item's commercial state, then its ownership,
// in one global order (courses, paths, lessons, exercises, then by id), so
// concurrent publications sharing dependencies wait for each other instead of
// deadlocking.
func lockReleaseGraph(ctx context.Context, tx ports.CommercialTx, items map[domain.ContentRef]*releaseItem) error {
	refs := make([]domain.ContentRef, 0, len(items))
	for ref := range items {
		refs = append(refs, ref)
	}
	sort.Slice(refs, func(i, j int) bool {
		if releaseLockRank(refs[i].Kind) != releaseLockRank(refs[j].Kind) {
			return releaseLockRank(refs[i].Kind) < releaseLockRank(refs[j].Kind)
		}
		return refs[i].ID < refs[j].ID
	})
	states, err := tx.Releases().LockStates(ctx, refs)
	if err != nil {
		return err
	}
	for i, ref := range refs {
		items[ref].state = states[i]
		ownership, err := tx.Ownership().LockCurrent(ctx, ref)
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("%w: %s %q (%s) has unresolved ownership", domain.ErrConflict, ref.Kind, items[ref].snapshot.Title, ref.ID)
		}
		if err != nil {
			return err
		}
		items[ref].ownership = ownership
	}
	return nil
}

func releaseLockRank(kind domain.ContentKind) int {
	switch kind {
	case domain.ContentCourse:
		return 0
	case domain.ContentPath:
		return 1
	case domain.ContentContentNode:
		return 2
	case domain.ContentExercise:
		return 3
	default:
		return 4
	}
}

// releaseGraph releases dependencies before the items that depend on them,
// reusing each latest release whose manifest is unchanged, and returns the
// root's release. Only a new root release is checked against the observed
// commercial revision.
func (s *CommercialContentService) releaseGraph(ctx context.Context, tx ports.CommercialTx, run CommercialRun, order []domain.ContentRef, items map[domain.ContentRef]*releaseItem, expectedRevision int) (domain.CommercialRelease, error) {
	released := map[domain.ContentRef]domain.CommercialRelease{}
	for i := len(order) - 1; i >= 0; i-- {
		ref := order[i]
		release, err := s.releaseOne(ctx, tx, run, ref, items, released, i == 0, expectedRevision)
		if err != nil {
			return domain.CommercialRelease{}, err
		}
		released[ref] = release
	}
	return released[order[0]], nil
}

func (s *CommercialContentService) releaseOne(ctx context.Context, tx ports.CommercialTx, run CommercialRun, ref domain.ContentRef, items map[domain.ContentRef]*releaseItem, released map[domain.ContentRef]domain.CommercialRelease, isRoot bool, expectedRevision int) (domain.CommercialRelease, error) {
	item := items[ref]
	contentSHA, err := item.snapshot.ContentSHA256()
	if err != nil {
		return domain.CommercialRelease{}, err
	}
	dependencies := transitiveDependencies(ref, items, released)
	manifest, err := domain.ReleaseManifestSHA256(ref, contentSHA, item.snapshot.VersionRef, item.ownership, dependencies)
	if err != nil {
		return domain.CommercialRelease{}, err
	}
	latest, err := latestRelease(ctx, tx, item.state)
	if err != nil {
		return domain.CommercialRelease{}, err
	}
	release, created := domain.NextRelease(latest, domain.CommercialRelease{
		ReleaseID: s.commands.newID(), Content: ref, ManifestSHA256: manifest, ContentSHA256: contentSHA,
		CoreVersionRef: item.snapshot.VersionRef, Ownership: item.ownership, Dependencies: dependencies,
		EvidenceEntryID: run.EvidenceEntryID, CreatedAt: s.commands.now().UTC(),
	})
	if !created {
		return release, nil
	}
	next := item.state.AdvancedTo(release.ReleaseID)
	if isRoot {
		if next, err = item.state.Advance(expectedRevision, release.ReleaseID); err != nil {
			return domain.CommercialRelease{}, err
		}
	}
	if err := tx.Releases().Insert(ctx, release); err != nil {
		return domain.CommercialRelease{}, err
	}
	if err := tx.Releases().SaveState(ctx, next); err != nil {
		return domain.CommercialRelease{}, err
	}
	return release, nil
}

func latestRelease(ctx context.Context, tx ports.CommercialTx, state domain.CommercialState) (*domain.CommercialRelease, error) {
	if state.LatestReleaseID == nil {
		return nil, nil
	}
	latest, err := tx.Releases().Release(ctx, *state.LatestReleaseID)
	if err != nil {
		return nil, fmt.Errorf("%w: latest release %s: %w", domain.ErrUnavailable, *state.LatestReleaseID, err)
	}
	return &latest, nil
}

// transitiveDependencies lists every item ref depends on, nearest first, each
// at the release this publication gave it.
func transitiveDependencies(ref domain.ContentRef, items map[domain.ContentRef]*releaseItem, released map[domain.ContentRef]domain.CommercialRelease) []domain.ReleaseDependency {
	dependencies := []domain.ReleaseDependency{}
	seen := map[domain.ContentRef]bool{ref: true}
	queue := append([]domain.ContentRef{}, items[ref].snapshot.Children...)
	for len(queue) > 0 {
		child := queue[0]
		queue = queue[1:]
		if seen[child] {
			continue
		}
		seen[child] = true
		release := released[child]
		dependencies = append(dependencies, domain.ReleaseDependency{
			Content: child, ContentSHA256: release.ContentSHA256, ReleaseID: release.ReleaseID,
			Permission: domain.PermissionOwn, EvidenceEntryID: items[child].ownership.EvidenceEntryID,
		})
		queue = append(queue, items[child].snapshot.Children...)
	}
	return dependencies
}

type releaseStateEvidence struct {
	ContentKind     string  `json:"content_kind"`
	ContentID       string  `json:"content_id"`
	Revision        int     `json:"revision"`
	LatestReleaseID *string `json:"latest_release_id"`
}

type releaseEvidence struct {
	ReleaseID            string   `json:"release_id"`
	ContentKind          string   `json:"content_kind"`
	ContentID            string   `json:"content_id"`
	Revision             int      `json:"revision"`
	ManifestSHA256       string   `json:"manifest_sha256"`
	DependencyReleaseIDs []string `json:"dependency_release_ids"`
}

func releaseStateView(state domain.CommercialState) json.RawMessage {
	return mustMarshalEvidence(releaseStateEvidence{
		ContentKind: string(state.Content.Kind), ContentID: state.Content.ID, Revision: state.Revision, LatestReleaseID: state.LatestReleaseID,
	})
}

func releaseView(release domain.CommercialRelease) json.RawMessage {
	return mustMarshalEvidence(releaseEvidence{
		ReleaseID: release.ReleaseID, ContentKind: string(release.Content.Kind), ContentID: release.Content.ID,
		Revision: release.Revision, ManifestSHA256: release.ManifestSHA256, DependencyReleaseIDs: release.DependencyReleaseIDs(),
	})
}

func mustMarshalEvidence[T releaseStateEvidence | releaseEvidence](view T) json.RawMessage {
	raw, err := json.Marshal(view)
	if err != nil {
		// Only strings, ints and string slices are marshalled, which cannot
		// fail; a failure here is a programming error.
		panic(fmt.Sprintf("marshal release evidence: %v", err))
	}
	return raw
}
