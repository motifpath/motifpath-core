package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/commercialcontentstate"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/commercialrelease"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/contentnodeexercise"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/contentnodeversion"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/courseversion"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/courseversioncheckpoint"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/exercise"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/exerciseoption"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/learningpath"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/learningpathitem"
	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

type entCommercialReleasesTx struct {
	tx *ent.Tx
}

func (t entCommercialTx) Releases() ports.CommercialReleasesTx {
	return entCommercialReleasesTx(t)
}

func (s *EntCommercialStore) CommercialState(ctx context.Context, content domain.ContentRef) (domain.CommercialState, error) {
	contentID, err := uuid.Parse(content.ID)
	if err != nil {
		return domain.NewCommercialState(content), nil
	}
	row, err := s.client.CommercialContentState.Query().
		Where(commercialcontentstate.ContentKindEQ(commercialcontentstate.ContentKind(content.Kind)), commercialcontentstate.ContentID(contentID)).
		Only(ctx)
	if ent.IsNotFound(err) {
		return domain.NewCommercialState(content), nil
	}
	if err != nil {
		return domain.CommercialState{}, err
	}
	return stateFromRow(row), nil
}

func (s *EntCommercialStore) Release(ctx context.Context, releaseID string) (domain.CommercialRelease, error) {
	return readRelease(ctx, s.client.CommercialRelease, releaseID)
}

func (r entCommercialReleasesTx) Release(ctx context.Context, releaseID string) (domain.CommercialRelease, error) {
	return readRelease(ctx, r.tx.CommercialRelease, releaseID)
}

func readRelease(ctx context.Context, releases *ent.CommercialReleaseClient, releaseID string) (domain.CommercialRelease, error) {
	id, err := uuid.Parse(releaseID)
	if err != nil {
		return domain.CommercialRelease{}, domain.ErrNotFound
	}
	row, err := releases.Get(ctx, id)
	if ent.IsNotFound(err) {
		return domain.CommercialRelease{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.CommercialRelease{}, err
	}
	return releaseFromRow(row)
}

// lockStateQuery creates an item's first commercial state if it has none, so
// there is always a row to lock.
const lockStateInsert = `INSERT INTO commercial_content_states (id, content_kind, content_id, revision, updated_at)
VALUES ($1, $2, $3, 1, now()) ON CONFLICT (content_kind, content_id) DO NOTHING`

const lockStateSelect = `SELECT id FROM commercial_content_states
WHERE content_kind = $1 AND content_id = $2 FOR UPDATE`

func (r entCommercialReleasesTx) LockStates(ctx context.Context, contents []domain.ContentRef) ([]domain.CommercialState, error) {
	states := make([]domain.CommercialState, 0, len(contents))
	for _, content := range contents {
		contentID, err := uuid.Parse(content.ID)
		if err != nil {
			return nil, fmt.Errorf("%w: content id %q", domain.ErrNotFound, content.ID)
		}
		if _, err := r.tx.ExecContext(ctx, lockStateInsert, uuid.New(), string(content.Kind), contentID); err != nil {
			return nil, fmt.Errorf("create commercial state: %w", err)
		}
		rows, err := r.tx.QueryContext(ctx, lockStateSelect, string(content.Kind), contentID)
		if err != nil {
			return nil, fmt.Errorf("lock commercial state: %w", err)
		}
		id, err := scanSingleUUID(rows)
		if err != nil {
			return nil, fmt.Errorf("lock commercial state: %w", err)
		}
		row, err := r.tx.CommercialContentState.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		states = append(states, stateFromRow(row))
	}
	return states, nil
}

func (r entCommercialReleasesTx) SaveState(ctx context.Context, state domain.CommercialState) error {
	contentID, err := uuid.Parse(state.Content.ID)
	if err != nil {
		return fmt.Errorf("commercial state content id: %w", err)
	}
	latest, err := parseOptionalUUID(state.LatestReleaseID)
	if err != nil {
		return fmt.Errorf("commercial state latest release: %w", err)
	}
	updated, err := r.tx.CommercialContentState.Update().
		Where(commercialcontentstate.ContentKindEQ(commercialcontentstate.ContentKind(state.Content.Kind)), commercialcontentstate.ContentID(contentID)).
		SetRevision(state.Revision).
		SetNillableLatestReleaseID(latest).
		Save(ctx)
	if err != nil {
		return err
	}
	if updated != 1 {
		return fmt.Errorf("commercial state of %s %s was not locked first", state.Content.Kind, state.Content.ID)
	}
	return nil
}

func (r entCommercialReleasesTx) Insert(ctx context.Context, release domain.CommercialRelease) error {
	row, err := releaseRow(release)
	if err != nil {
		return err
	}
	err = r.tx.CommercialRelease.Create().
		SetID(row.ID).
		SetContentKind(row.ContentKind).
		SetContentID(row.ContentID).
		SetRevision(row.Revision).
		SetManifestSha256(row.ManifestSha256).
		SetContentSha256(row.ContentSha256).
		SetNillableCoreVersionRef(row.CoreVersionRef).
		SetOwnership(row.Ownership).
		SetDependencies(row.Dependencies).
		SetEvidenceEntryID(row.EvidenceEntryID).
		SetCreatedAt(row.CreatedAt).
		Exec(ctx)
	if ent.IsConstraintError(err) {
		return fmt.Errorf("%w: release revision %d of %s %s is already stored", domain.ErrConflict, release.Revision, release.Content.Kind, release.Content.ID)
	}
	return err
}

func stateFromRow(row *ent.CommercialContentState) domain.CommercialState {
	return domain.CommercialState{
		Content:         domain.ContentRef{Kind: domain.ContentKind(row.ContentKind), ID: row.ContentID.String()},
		Revision:        row.Revision,
		LatestReleaseID: optionalUUIDString(row.LatestReleaseID),
	}
}

// releasedOwnership is the stored form of a release's ownership snapshot.
type releasedOwnership struct {
	OwnerKind       string  `json:"owner_kind"`
	OwnerID         *string `json:"owner_id"`
	CreatorID       *string `json:"creator_id"`
	Source          string  `json:"source"`
	EvidenceEntryID string  `json:"evidence_entry_id"`
	Revision        int     `json:"revision"`
}

// releasedDependency is the stored form of one release dependency.
type releasedDependency struct {
	ContentKind     string `json:"content_kind"`
	ContentID       string `json:"content_id"`
	ContentSHA256   string `json:"content_sha256"`
	ReleaseID       string `json:"release_id"`
	Permission      string `json:"permission"`
	EvidenceEntryID string `json:"evidence_entry_id"`
}

func releaseRow(release domain.CommercialRelease) (*ent.CommercialRelease, error) {
	id, err := uuid.Parse(release.ReleaseID)
	if err != nil {
		return nil, fmt.Errorf("release id: %w", err)
	}
	contentID, err := uuid.Parse(release.Content.ID)
	if err != nil {
		return nil, fmt.Errorf("release content id: %w", err)
	}
	evidenceID, err := uuid.Parse(release.EvidenceEntryID)
	if err != nil {
		return nil, fmt.Errorf("release evidence id: %w", err)
	}
	o := release.Ownership
	ownership, err := json.Marshal(releasedOwnership{
		OwnerKind: string(o.OwnerKind), OwnerID: o.OwnerID, CreatorID: o.CreatorID,
		Source: string(o.Source), EvidenceEntryID: o.EvidenceEntryID, Revision: o.Revision,
	})
	if err != nil {
		return nil, err
	}
	stored := make([]releasedDependency, 0, len(release.Dependencies))
	for _, d := range release.Dependencies {
		stored = append(stored, releasedDependency{
			ContentKind: string(d.Content.Kind), ContentID: d.Content.ID, ContentSHA256: d.ContentSHA256,
			ReleaseID: d.ReleaseID, Permission: string(d.Permission), EvidenceEntryID: d.EvidenceEntryID,
		})
	}
	dependencies, err := json.Marshal(stored)
	if err != nil {
		return nil, err
	}
	return &ent.CommercialRelease{
		ID: id, ContentKind: commercialrelease.ContentKind(release.Content.Kind), ContentID: contentID,
		Revision: release.Revision, ManifestSha256: release.ManifestSHA256, ContentSha256: release.ContentSHA256,
		CoreVersionRef: release.CoreVersionRef, Ownership: ownership, Dependencies: dependencies,
		EvidenceEntryID: evidenceID, CreatedAt: release.CreatedAt,
	}, nil
}

func releaseFromRow(row *ent.CommercialRelease) (domain.CommercialRelease, error) {
	content := domain.ContentRef{Kind: domain.ContentKind(row.ContentKind), ID: row.ContentID.String()}
	var ownership releasedOwnership
	if err := json.Unmarshal(row.Ownership, &ownership); err != nil {
		return domain.CommercialRelease{}, fmt.Errorf("release %s ownership: %w", row.ID, err)
	}
	var stored []releasedDependency
	if err := json.Unmarshal(row.Dependencies, &stored); err != nil {
		return domain.CommercialRelease{}, fmt.Errorf("release %s dependencies: %w", row.ID, err)
	}
	dependencies := make([]domain.ReleaseDependency, 0, len(stored))
	for _, d := range stored {
		dependencies = append(dependencies, domain.ReleaseDependency{
			Content:       domain.ContentRef{Kind: domain.ContentKind(d.ContentKind), ID: d.ContentID},
			ContentSHA256: d.ContentSHA256, ReleaseID: d.ReleaseID,
			Permission: domain.DependencyPermission(d.Permission), EvidenceEntryID: d.EvidenceEntryID,
		})
	}
	return domain.CommercialRelease{
		ReleaseID: row.ID.String(), Content: content, Revision: row.Revision,
		ManifestSHA256: row.ManifestSha256, ContentSHA256: row.ContentSha256, CoreVersionRef: row.CoreVersionRef,
		Ownership: domain.ContentOwnership{
			Content: content, OwnerKind: domain.OwnerKind(ownership.OwnerKind), OwnerID: ownership.OwnerID,
			CreatorID: ownership.CreatorID, Source: domain.OwnershipSource(ownership.Source),
			EvidenceEntryID: ownership.EvidenceEntryID, Revision: ownership.Revision,
		},
		Dependencies: dependencies, EvidenceEntryID: row.EvidenceEntryID.String(), CreatedAt: row.CreatedAt.UTC(),
	}, nil
}

// Snapshot reads an item's learner-visible state by kind.
func (r entCommercialReleasesTx) Snapshot(ctx context.Context, content domain.ContentRef) (domain.ReleaseSnapshot, error) {
	id, err := uuid.Parse(content.ID)
	if err != nil {
		return domain.ReleaseSnapshot{}, domain.ErrNotFound
	}
	var snapshot domain.ReleaseSnapshot
	switch content.Kind {
	case domain.ContentCourse:
		snapshot, err = r.courseSnapshot(ctx, id)
	case domain.ContentPath:
		snapshot, err = r.pathSnapshot(ctx, id)
	case domain.ContentContentNode:
		snapshot, err = r.lessonSnapshot(ctx, id)
	case domain.ContentExercise:
		snapshot, err = r.exerciseSnapshot(ctx, id)
	default:
		return domain.ReleaseSnapshot{}, domain.ErrNotFound
	}
	if ent.IsNotFound(err) {
		return domain.ReleaseSnapshot{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.ReleaseSnapshot{}, err
	}
	snapshot.Content = content
	return snapshot, nil
}

// courseSnapshot is a course at its latest published version; a course with
// none is not published.
func (r entCommercialReleasesTx) courseSnapshot(ctx context.Context, id uuid.UUID) (domain.ReleaseSnapshot, error) {
	course, err := r.tx.Course.Get(ctx, id)
	if err != nil {
		return domain.ReleaseSnapshot{}, err
	}
	version, err := r.tx.CourseVersion.Query().Where(courseversion.CourseID(id)).
		Order(ent.Desc(courseversion.FieldVersionNumber)).First(ctx)
	if ent.IsNotFound(err) {
		return domain.ReleaseSnapshot{Title: course.Title, State: domain.CourseReleaseState{Title: course.Title}}, nil
	}
	if err != nil {
		return domain.ReleaseSnapshot{}, err
	}
	checkpoints, err := r.tx.CourseVersionCheckpoint.Query().Where(courseversioncheckpoint.CourseVersionID(version.ID)).
		Order(ent.Asc(courseversioncheckpoint.FieldPosition)).All(ctx)
	if err != nil {
		return domain.ReleaseSnapshot{}, err
	}
	state := domain.CourseReleaseState{
		VersionNumber: version.VersionNumber, Title: version.TitleSnapshot, Summary: version.SummarySnapshot,
		Level: string(version.LevelSnapshot), Language: version.LanguageSnapshot,
		Instruments: sortedStrings(version.InstrumentIdsSnapshot), ThumbnailURL: version.ThumbnailURLSnapshot,
		Checkpoints: make([]domain.CourseReleaseCheckpoint, 0, len(checkpoints)),
	}
	children := make([]domain.ContentRef, 0, len(checkpoints))
	for _, checkpoint := range checkpoints {
		state.Checkpoints = append(state.Checkpoints, domain.CourseReleaseCheckpoint{PathID: checkpoint.LearningPathID.String(), EffectiveTitle: checkpoint.EffectiveTitle})
		children = append(children, domain.ContentRef{Kind: domain.ContentPath, ID: checkpoint.LearningPathID.String()})
	}
	versionRef := version.ID.String()
	return domain.ReleaseSnapshot{Title: course.Title, Published: true, VersionRef: &versionRef, State: state, Children: children}, nil
}

// pathSnapshot is a path's live description and ordered lessons.
func (r entCommercialReleasesTx) pathSnapshot(ctx context.Context, id uuid.UUID) (domain.ReleaseSnapshot, error) {
	path, err := r.tx.LearningPath.Query().Where(learningpath.ID(id)).WithInstruments().Only(ctx)
	if err != nil {
		return domain.ReleaseSnapshot{}, err
	}
	items, err := r.tx.LearningPathItem.Query().Where(learningpathitem.LearningPathID(id)).
		Order(ent.Asc(learningpathitem.FieldPosition)).All(ctx)
	if err != nil {
		return domain.ReleaseSnapshot{}, err
	}
	instruments := make([]string, 0, len(path.Edges.Instruments))
	for _, instrument := range path.Edges.Instruments {
		instruments = append(instruments, instrument.ID.String())
	}
	var level *string
	if path.Level != nil {
		value := string(*path.Level)
		level = &value
	}
	state := domain.PathReleaseState{
		Title: path.Title, Summary: path.Summary, Level: level, Language: path.Language,
		Instruments: sortedStrings(instruments), ThumbnailURL: path.ThumbnailURL,
		Items: make([]domain.PathReleaseItem, 0, len(items)),
	}
	children := make([]domain.ContentRef, 0, len(items))
	for _, item := range items {
		state.Items = append(state.Items, domain.PathReleaseItem{LessonID: item.ContentNodeID.String(), SectionLabel: item.SectionLabel})
		children = append(children, domain.ContentRef{Kind: domain.ContentContentNode, ID: item.ContentNodeID.String()})
	}
	return domain.ReleaseSnapshot{
		Title: path.Title, Published: path.Status == learningpath.StatusPublished, State: state, Children: children,
	}, nil
}

// lessonSnapshot is a lesson at its latest published version with the
// exercises it contains; a lesson with no published version is not published.
func (r entCommercialReleasesTx) lessonSnapshot(ctx context.Context, id uuid.UUID) (domain.ReleaseSnapshot, error) {
	node, err := r.tx.ContentNode.Get(ctx, id)
	if err != nil {
		return domain.ReleaseSnapshot{}, err
	}
	links, err := r.tx.ContentNodeExercise.Query().Where(contentnodeexercise.ContentNodeID(id)).
		Order(ent.Asc(contentnodeexercise.FieldLinkedAt), ent.Asc(contentnodeexercise.FieldExerciseID)).All(ctx)
	if err != nil {
		return domain.ReleaseSnapshot{}, err
	}
	children := make([]domain.ContentRef, 0, len(links))
	for _, link := range links {
		children = append(children, domain.ContentRef{Kind: domain.ContentExercise, ID: link.ExerciseID.String()})
	}
	version, err := r.tx.ContentNodeVersion.Query().Where(contentnodeversion.ContentNodeID(id)).
		Order(ent.Desc(contentnodeversion.FieldVersionNumber)).First(ctx)
	if ent.IsNotFound(err) {
		return domain.ReleaseSnapshot{Title: node.Title, State: domain.LessonReleaseState{Title: node.Title}, Children: children}, nil
	}
	if err != nil {
		return domain.ReleaseSnapshot{}, err
	}
	versionRef := version.ID.String()
	return domain.ReleaseSnapshot{
		Title: node.Title, Published: true, VersionRef: &versionRef, Children: children,
		State: domain.LessonReleaseState{
			VersionNumber: version.VersionNumber, Title: version.Title, ContentType: string(version.ContentType),
			MediaURL: version.MediaURL, RichContent: version.RichContent, Languages: version.LanguagesSnapshot,
			Instruments: sortedStrings(version.InstrumentIdsSnapshot), ThumbnailURL: version.ThumbnailURLSnapshot,
		},
	}, nil
}

// exerciseSnapshot is an exercise's live content and answer key. Exercises
// have no published state: learners get them through their lessons.
func (r entCommercialReleasesTx) exerciseSnapshot(ctx context.Context, id uuid.UUID) (domain.ReleaseSnapshot, error) {
	row, err := r.tx.Exercise.Query().Where(exercise.ID(id)).WithLanguages().Only(ctx)
	if err != nil {
		return domain.ReleaseSnapshot{}, err
	}
	options, err := r.tx.ExerciseOption.Query().Where(exerciseoption.ExerciseID(id)).Order(ent.Asc(exerciseoption.FieldID)).All(ctx)
	if err != nil {
		return domain.ReleaseSnapshot{}, err
	}
	languages := make([]string, 0, len(row.Edges.Languages))
	for _, language := range row.Edges.Languages {
		languages = append(languages, language.Code)
	}
	state := domain.ExerciseReleaseState{
		Title: row.Title, Prompt: row.Prompt, ExerciseType: string(row.ExerciseType),
		ImageURL: row.ImageURL, AudioURL: row.AudioURL, EstimatedDurationSeconds: row.EstimatedDurationSeconds,
		DiagramRef: row.DiagramRef, DiagramStackRef: row.DiagramStackRef,
		Languages: sortedStrings(languages), Options: make([]domain.ExerciseReleaseOption, 0, len(options)),
	}
	for _, option := range options {
		state.Options = append(state.Options, releaseOption(option))
	}
	return domain.ReleaseSnapshot{Title: row.Title, Published: true, State: state}, nil
}

func releaseOption(o *ent.ExerciseOption) domain.ExerciseReleaseOption {
	var shape *string
	if o.RegionShape != nil {
		value := string(*o.RegionShape)
		shape = &value
	}
	return domain.ExerciseReleaseOption{
		OptionID: o.ID.String(), Label: o.Label, IsCorrect: o.IsCorrect, ImageURL: o.ImageURL, AudioURL: o.AudioURL,
		RegionX: o.RegionX, RegionY: o.RegionY, RegionWidth: o.RegionWidth, RegionHeight: o.RegionHeight, RegionShape: shape,
		DiagramRef: o.DiagramRef, DiagramID: optionalUUIDString(o.DiagramID), DiagramPositionID: optionalUUIDString(o.DiagramPositionID),
		CellString: o.CellString, CellFret: o.CellFret,
	}
}

func sortedStrings(values []string) []string {
	sorted := append([]string{}, values...)
	sort.Strings(sorted)
	return sorted
}
