package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// ReleaseState is the learner-visible state of one kind of content, as a
// commercial release covers it. Only the types below implement it.
type ReleaseState interface {
	releaseStateKind() ContentKind
}

// CourseReleaseState is a course's latest published version: what that
// version shows learners and the paths its checkpoints name, in order.
type CourseReleaseState struct {
	VersionNumber int                       `json:"version_number"`
	Title         string                    `json:"title"`
	Summary       string                    `json:"summary"`
	Level         string                    `json:"level"`
	Language      string                    `json:"language"`
	Instruments   []string                  `json:"instruments"`
	ThumbnailURL  *string                   `json:"thumbnail_url"`
	Checkpoints   []CourseReleaseCheckpoint `json:"checkpoints"`
}

// CourseReleaseCheckpoint is one path a published course version names.
type CourseReleaseCheckpoint struct {
	PathID         string `json:"path_id"`
	EffectiveTitle string `json:"effective_title"`
}

// PathReleaseState is a path's live learner-visible state: its description
// and its ordered lessons with their section labels.
type PathReleaseState struct {
	Title        string            `json:"title"`
	Summary      *string           `json:"summary"`
	Level        *string           `json:"level"`
	Language     *string           `json:"language"`
	Instruments  []string          `json:"instruments"`
	ThumbnailURL *string           `json:"thumbnail_url"`
	Items        []PathReleaseItem `json:"items"`
}

// PathReleaseItem is one lesson of a path, in path order.
type PathReleaseItem struct {
	LessonID     string  `json:"lesson_id"`
	SectionLabel *string `json:"section_label"`
}

// LessonReleaseState is a lesson's latest published version.
type LessonReleaseState struct {
	VersionNumber int      `json:"version_number"`
	Title         string   `json:"title"`
	ContentType   string   `json:"content_type"`
	MediaURL      *string  `json:"media_url"`
	RichContent   *string  `json:"rich_content"`
	Languages     *string  `json:"languages"`
	Instruments   []string `json:"instruments"`
	ThumbnailURL  *string  `json:"thumbnail_url"`
}

// ExerciseReleaseState is an exercise's live learner-visible content and its
// answer key.
type ExerciseReleaseState struct {
	Title                    string                  `json:"title"`
	Prompt                   string                  `json:"prompt"`
	ExerciseType             string                  `json:"exercise_type"`
	ImageURL                 *string                 `json:"image_url"`
	AudioURL                 *string                 `json:"audio_url"`
	EstimatedDurationSeconds *int                    `json:"estimated_duration_seconds"`
	DiagramRef               *string                 `json:"diagram_ref"`
	DiagramStackRef          *string                 `json:"diagram_stack_ref"`
	RemediationTargets       *string                 `json:"remediation_targets"`
	Languages                []string                `json:"languages"`
	Options                  []ExerciseReleaseOption `json:"options"`
}

// ExerciseReleaseOption is one answer option with whether it is correct.
// Options are listed in a stable order.
type ExerciseReleaseOption struct {
	OptionID          string   `json:"option_id"`
	Label             *string  `json:"label"`
	IsCorrect         bool     `json:"is_correct"`
	ImageURL          *string  `json:"image_url"`
	AudioURL          *string  `json:"audio_url"`
	RegionX           *float64 `json:"region_x"`
	RegionY           *float64 `json:"region_y"`
	RegionWidth       *float64 `json:"region_width"`
	RegionHeight      *float64 `json:"region_height"`
	RegionShape       *string  `json:"region_shape"`
	DiagramRef        *string  `json:"diagram_ref"`
	DiagramID         *string  `json:"diagram_id"`
	DiagramPositionID *string  `json:"diagram_position_id"`
	CellString        *int     `json:"cell_string"`
	CellFret          *int     `json:"cell_fret"`
}

func (CourseReleaseState) releaseStateKind() ContentKind   { return ContentCourse }
func (PathReleaseState) releaseStateKind() ContentKind     { return ContentPath }
func (LessonReleaseState) releaseStateKind() ContentKind   { return ContentContentNode }
func (ExerciseReleaseState) releaseStateKind() ContentKind { return ContentExercise }

// ReleaseSnapshot is an item as core reads it to release it: whether learners
// can get it, the published version it is released at (courses and lessons
// only), its learner-visible state and the items it directly depends on.
type ReleaseSnapshot struct {
	Content ContentRef
	// Title names the item in refusals; the state carries what is released.
	Title string
	// Published is true for a published path, a course or lesson with a
	// published version, and every exercise.
	Published  bool
	VersionRef *string
	State      ReleaseState
	Children   []ContentRef
}

// ContentSHA256 is the canonical digest of the item's released state.
func (s ReleaseSnapshot) ContentSHA256() (string, error) {
	if s.State == nil || s.State.releaseStateKind() != s.Content.Kind {
		return "", fmt.Errorf("release snapshot of %s %s has no %s state", s.Content.Kind, s.Content.ID, s.Content.Kind)
	}
	state, err := json.Marshal(s.State)
	if err != nil {
		return "", err
	}
	return canonicalSHA256(json.Marshal(struct {
		Kind       ContentKind     `json:"kind"`
		ContentID  string          `json:"content_id"`
		VersionRef *string         `json:"version_ref"`
		State      json.RawMessage `json:"state"`
	}{s.Content.Kind, s.Content.ID, s.VersionRef, state}))
}

// CheckReleasable refuses an item that learners cannot get, whose ownership
// is unresolved, or whose rights holder differs from the released item's:
// until reuse permissions exist a release only covers the holder's own
// content. ownership is the item's; root is the released item's.
func CheckReleasable(snapshot ReleaseSnapshot, ownership, root ContentOwnership) error {
	name := fmt.Sprintf("%s %q (%s)", snapshot.Content.Kind, snapshot.Title, snapshot.Content.ID)
	switch {
	case !snapshot.Published:
		return fmt.Errorf("%w: %s is not published", ErrConflict, name)
	case ownership.OwnerKind == OwnerUnresolved || root.OwnerKind == OwnerUnresolved:
		return fmt.Errorf("%w: %s has unresolved ownership", ErrConflict, name)
	case !sameRightsHolder(ownership, root):
		return fmt.Errorf("%w: %s has another rights holder, so it cannot be released with this item yet", ErrConflict, name)
	}
	return nil
}

func sameRightsHolder(a, b ContentOwnership) bool {
	return a.OwnerKind == b.OwnerKind && a.OwnerID != nil && b.OwnerID != nil && *a.OwnerID == *b.OwnerID
}

// DependencyPermission is the reviewed right under which a release includes
// a dependency.
type DependencyPermission string

const (
	PermissionOwn             DependencyPermission = "own"
	PermissionFreeReuse       DependencyPermission = "free_reuse"
	PermissionPlatformPremium DependencyPermission = "platform_premium"
)

// ReleaseDependency is one transitive dependency of a release, at its own
// release.
type ReleaseDependency struct {
	Content         ContentRef
	ContentSHA256   string
	ReleaseID       string
	Permission      DependencyPermission
	EvidenceEntryID string
}

// CommercialRelease is an immutable manifest of what an owner offered at one
// moment. It is never changed once stored.
type CommercialRelease struct {
	ReleaseID      string
	Content        ContentRef
	Revision       int
	ManifestSHA256 string
	ContentSHA256  string
	CoreVersionRef *string
	Ownership      ContentOwnership
	// Dependencies lists every transitive dependency, nearest first.
	Dependencies    []ReleaseDependency
	EvidenceEntryID string
	CreatedAt       time.Time
}

// DependencyReleaseIDs lists the releases of the dependencies, in order.
func (r CommercialRelease) DependencyReleaseIDs() []string {
	ids := make([]string, 0, len(r.Dependencies))
	for _, dependency := range r.Dependencies {
		ids = append(ids, dependency.ReleaseID)
	}
	return ids
}

// ReleaseManifestSHA256 digests what a release offers: the item's state, its
// rights holder and each dependency at its release. Evidence ids and times
// are not part of the offer, so an unchanged item has an unchanged manifest.
func ReleaseManifestSHA256(content ContentRef, contentSHA256 string, versionRef *string, ownership ContentOwnership, dependencies []ReleaseDependency) (string, error) {
	type manifestDependency struct {
		Kind          ContentKind          `json:"kind"`
		ContentID     string               `json:"content_id"`
		ContentSHA256 string               `json:"content_sha256"`
		ReleaseID     string               `json:"release_id"`
		Permission    DependencyPermission `json:"permission"`
	}
	deps := make([]manifestDependency, 0, len(dependencies))
	for _, d := range dependencies {
		deps = append(deps, manifestDependency{d.Content.Kind, d.Content.ID, d.ContentSHA256, d.ReleaseID, d.Permission})
	}
	return canonicalSHA256(json.Marshal(struct {
		Kind              ContentKind          `json:"kind"`
		ContentID         string               `json:"content_id"`
		ContentSHA256     string               `json:"content_sha256"`
		VersionRef        *string              `json:"version_ref"`
		OwnerKind         OwnerKind            `json:"owner_kind"`
		OwnerID           *string              `json:"owner_id"`
		OwnershipRevision int                  `json:"ownership_revision"`
		Dependencies      []manifestDependency `json:"dependencies"`
	}{content.Kind, content.ID, contentSHA256, versionRef, ownership.OwnerKind, ownership.OwnerID, ownership.Revision, deps}))
}

// NextRelease decides what publishing candidate gives: the latest release
// when its manifest is unchanged, otherwise candidate as the next revision.
// created reports whether candidate is a new release to store.
func NextRelease(latest *CommercialRelease, candidate CommercialRelease) (release CommercialRelease, created bool) {
	if latest == nil {
		candidate.Revision = 1
		return candidate, true
	}
	if latest.ManifestSHA256 == candidate.ManifestSHA256 {
		return *latest, false
	}
	candidate.Revision = latest.Revision + 1
	return candidate, true
}

// CommercialState is an item's commercial revision and latest release. The
// revision is 1 until the first release and goes up by one each time a new
// release becomes the item's latest.
type CommercialState struct {
	Content         ContentRef
	Revision        int
	LatestReleaseID *string
}

// NewCommercialState is the state of an item never released.
func NewCommercialState(content ContentRef) CommercialState {
	return CommercialState{Content: content, Revision: 1}
}

// Advance makes releaseID the latest release of the item a caller published,
// against the commercial revision they observed; any other is a conflict.
func (s CommercialState) Advance(expectedRevision int, releaseID string) (CommercialState, error) {
	if expectedRevision != s.Revision {
		return CommercialState{}, fmt.Errorf("%w: commercial revision is %d, not %d", ErrConflict, s.Revision, expectedRevision)
	}
	return s.AdvancedTo(releaseID), nil
}

// AdvancedTo makes releaseID the latest release of a dependency released
// along with the item a caller published.
func (s CommercialState) AdvancedTo(releaseID string) CommercialState {
	id := releaseID
	s.Revision++
	s.LatestReleaseID = &id
	return s
}

// canonicalSHA256 digests marshalled JSON in canonical form, so field order
// never changes a digest.
func canonicalSHA256(raw []byte, err error) (string, error) {
	if err != nil {
		return "", err
	}
	canonical, err := canonicalJSON(raw)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}
