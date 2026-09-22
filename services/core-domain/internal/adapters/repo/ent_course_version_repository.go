package repo

import (
	"context"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/courseversion"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/courseversioncheckpoint"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntCourseVersionRepository persists CourseVersion and
// CourseVersionCheckpoint snapshots via ent/Postgres. A checkpoint
// snapshot's items are never persisted here — resolving them live from the
// referenced LearningPath at read time is the application layer's job.
type EntCourseVersionRepository struct {
	client *ent.Client
}

func NewEntCourseVersionRepository(client *ent.Client) *EntCourseVersionRepository {
	return &EntCourseVersionRepository{client: client}
}

func (r *EntCourseVersionRepository) Create(ctx context.Context, v domain.CourseVersion) error {
	id, err := uuid.Parse(v.ID)
	if err != nil {
		return err
	}
	courseID, err := uuid.Parse(v.CourseID)
	if err != nil {
		return err
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}

	if _, err := tx.CourseVersion.Create().
		SetID(id).
		SetCourseID(courseID).
		SetVersionNumber(v.VersionNumber).
		SetTitleSnapshot(v.TitleSnapshot).
		SetSummarySnapshot(v.SummarySnapshot).
		SetLevelSnapshot(courseversion.LevelSnapshot(v.LevelSnapshot)).
		SetAvailableForNewEnrollments(v.AvailableForNewEnrollments).
		SetPublishedAt(v.PublishedAt).
		Save(ctx); err != nil {
		return rollback(tx, err)
	}

	if err := createCourseVersionCheckpoints(ctx, tx, id, v.Checkpoints); err != nil {
		return rollback(tx, err)
	}

	return tx.Commit()
}

func (r *EntCourseVersionRepository) GetLatestByCourseID(ctx context.Context, courseID string) (domain.CourseVersion, error) {
	parsed, err := uuid.Parse(courseID)
	if err != nil {
		return domain.CourseVersion{}, domain.ErrNotFound
	}

	row, err := r.client.CourseVersion.Query().
		Where(courseversion.CourseID(parsed)).
		Order(ent.Desc(courseversion.FieldVersionNumber)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.CourseVersion{}, domain.ErrNotFound
		}
		return domain.CourseVersion{}, err
	}

	checkpointRows, err := r.client.CourseVersionCheckpoint.Query().
		Where(courseversioncheckpoint.CourseVersionID(row.ID)).
		Order(courseversioncheckpoint.ByPosition()).
		All(ctx)
	if err != nil {
		return domain.CourseVersion{}, err
	}

	checkpoints := make([]domain.CourseVersionCheckpoint, len(checkpointRows))
	for i, cp := range checkpointRows {
		checkpoints[i] = domain.CourseVersionCheckpoint{
			Position:       cp.Position,
			LearningPathID: cp.LearningPathID.String(),
			EffectiveTitle: cp.EffectiveTitle,
		}
	}

	return domain.CourseVersion{
		ID:                         row.ID.String(),
		CourseID:                   row.CourseID.String(),
		VersionNumber:              row.VersionNumber,
		TitleSnapshot:              row.TitleSnapshot,
		SummarySnapshot:            row.SummarySnapshot,
		LevelSnapshot:              domain.DifficultyLevel(row.LevelSnapshot),
		Checkpoints:                checkpoints,
		PublishedAt:                row.PublishedAt,
		AvailableForNewEnrollments: row.AvailableForNewEnrollments,
	}, nil
}

func (r *EntCourseVersionRepository) IsLearningPathReferenced(ctx context.Context, learningPathID string) (bool, error) {
	parsed, err := uuid.Parse(learningPathID)
	if err != nil {
		return false, nil
	}

	return r.client.CourseVersionCheckpoint.Query().
		Where(courseversioncheckpoint.LearningPathID(parsed)).
		Exist(ctx)
}

// createCourseVersionCheckpoints bulk-inserts checkpoints under
// courseVersionID within tx.
func createCourseVersionCheckpoints(ctx context.Context, tx *ent.Tx, courseVersionID uuid.UUID, checkpoints []domain.CourseVersionCheckpoint) error {
	if len(checkpoints) == 0 {
		return nil
	}
	builders := make([]*ent.CourseVersionCheckpointCreate, len(checkpoints))
	for i, cp := range checkpoints {
		learningPathID, err := uuid.Parse(cp.LearningPathID)
		if err != nil {
			return err
		}
		builders[i] = tx.CourseVersionCheckpoint.Create().
			SetID(uuid.New()).
			SetCourseVersionID(courseVersionID).
			SetLearningPathID(learningPathID).
			SetPosition(cp.Position).
			SetEffectiveTitle(cp.EffectiveTitle)
	}
	_, err := tx.CourseVersionCheckpoint.CreateBulk(builders...).Save(ctx)
	return err
}
