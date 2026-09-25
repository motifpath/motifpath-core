package repo

import (
	"context"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/courseenrollment"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntCourseEnrollmentRepository persists CourseEnrollment records via
// ent/Postgres.
type EntCourseEnrollmentRepository struct {
	client *ent.Client
}

func NewEntCourseEnrollmentRepository(client *ent.Client) *EntCourseEnrollmentRepository {
	return &EntCourseEnrollmentRepository{client: client}
}

func (r *EntCourseEnrollmentRepository) Create(ctx context.Context, e domain.CourseEnrollment) error {
	id, err := uuid.Parse(e.ID)
	if err != nil {
		return err
	}
	studentID, err := uuid.Parse(e.StudentID)
	if err != nil {
		return err
	}
	courseID, err := uuid.Parse(e.CourseID)
	if err != nil {
		return err
	}

	create := r.client.CourseEnrollment.Create().
		SetID(id).
		SetStudentID(studentID).
		SetCourseID(courseID).
		SetCourseTitle(e.CourseTitle).
		SetNillableCourseThumbnailURL(e.CourseThumbnailURL).
		SetCourseVersionNumber(e.CourseVersionNumber).
		SetStatus(courseenrollment.Status(e.Status)).
		SetEnrolledAt(e.EnrolledAt).
		SetNillableActiveCheckpointPosition(e.ActiveCheckpointPosition)

	if e.ActiveCheckpointStudentPathID != nil {
		activeCheckpointID, err := uuid.Parse(*e.ActiveCheckpointStudentPathID)
		if err != nil {
			return err
		}
		create = create.SetActiveCheckpointStudentPathID(activeCheckpointID)
	}

	_, err = create.Save(ctx)
	return err
}

func (r *EntCourseEnrollmentRepository) GetByID(ctx context.Context, id string) (domain.CourseEnrollment, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.CourseEnrollment{}, domain.ErrNotFound
	}

	row, err := r.client.CourseEnrollment.Get(ctx, parsed)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.CourseEnrollment{}, domain.ErrNotFound
		}
		return domain.CourseEnrollment{}, err
	}

	return toDomainCourseEnrollment(row), nil
}

func (r *EntCourseEnrollmentRepository) GetActiveByCourseID(ctx context.Context, studentID, courseID string) (domain.CourseEnrollment, error) {
	parsedStudentID, err := uuid.Parse(studentID)
	if err != nil {
		return domain.CourseEnrollment{}, domain.ErrNotFound
	}
	parsedCourseID, err := uuid.Parse(courseID)
	if err != nil {
		return domain.CourseEnrollment{}, domain.ErrNotFound
	}

	row, err := r.client.CourseEnrollment.Query().
		Where(
			courseenrollment.StudentID(parsedStudentID),
			courseenrollment.CourseID(parsedCourseID),
			courseenrollment.StatusEQ(courseenrollment.StatusActive),
		).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.CourseEnrollment{}, domain.ErrNotFound
		}
		return domain.CourseEnrollment{}, err
	}

	return toDomainCourseEnrollment(row), nil
}

func (r *EntCourseEnrollmentRepository) ListByStudentID(ctx context.Context, studentID string) ([]domain.CourseEnrollment, error) {
	parsed, err := uuid.Parse(studentID)
	if err != nil {
		return nil, domain.ErrNotFound
	}

	rows, err := r.client.CourseEnrollment.Query().
		Where(courseenrollment.StudentID(parsed)).
		Order(courseenrollment.ByEnrolledAt()).
		All(ctx)
	if err != nil {
		return nil, err
	}

	return toDomainCourseEnrollments(rows), nil
}

func (r *EntCourseEnrollmentRepository) ListActiveByStudentID(ctx context.Context, studentID string) ([]domain.CourseEnrollment, error) {
	parsed, err := uuid.Parse(studentID)
	if err != nil {
		return nil, domain.ErrNotFound
	}

	rows, err := r.client.CourseEnrollment.Query().
		Where(
			courseenrollment.StudentID(parsed),
			courseenrollment.StatusEQ(courseenrollment.StatusActive),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}

	return toDomainCourseEnrollments(rows), nil
}

func (r *EntCourseEnrollmentRepository) Abandon(ctx context.Context, id string) error {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.ErrNotFound
	}

	n, err := r.client.CourseEnrollment.Update().
		Where(courseenrollment.ID(parsed)).
		SetStatus(courseenrollment.StatusAbandoned).
		ClearActiveCheckpointStudentPathID().
		ClearActiveCheckpointPosition().
		Save(ctx)
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *EntCourseEnrollmentRepository) AdvanceCheckpoint(ctx context.Context, id, studentPathID string, position int) error {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.ErrNotFound
	}
	parsedStudentPathID, err := uuid.Parse(studentPathID)
	if err != nil {
		return err
	}

	n, err := r.client.CourseEnrollment.Update().
		Where(courseenrollment.ID(parsed)).
		SetActiveCheckpointStudentPathID(parsedStudentPathID).
		SetActiveCheckpointPosition(position).
		Save(ctx)
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *EntCourseEnrollmentRepository) Complete(ctx context.Context, id string) error {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.ErrNotFound
	}

	n, err := r.client.CourseEnrollment.Update().
		Where(courseenrollment.ID(parsed)).
		SetStatus(courseenrollment.StatusCompleted).
		ClearActiveCheckpointStudentPathID().
		ClearActiveCheckpointPosition().
		Save(ctx)
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func toDomainCourseEnrollment(row *ent.CourseEnrollment) domain.CourseEnrollment {
	var activeCheckpointStudentPathID *string
	if row.ActiveCheckpointStudentPathID != nil {
		s := row.ActiveCheckpointStudentPathID.String()
		activeCheckpointStudentPathID = &s
	}

	return domain.CourseEnrollment{
		ID:                            row.ID.String(),
		StudentID:                     row.StudentID.String(),
		CourseID:                      row.CourseID.String(),
		CourseTitle:                   row.CourseTitle,
		CourseThumbnailURL:            row.CourseThumbnailURL,
		CourseVersionNumber:           row.CourseVersionNumber,
		Status:                        domain.CourseEnrollmentStatus(row.Status),
		ActiveCheckpointStudentPathID: activeCheckpointStudentPathID,
		ActiveCheckpointPosition:      row.ActiveCheckpointPosition,
		EnrolledAt:                    row.EnrolledAt,
	}
}

func toDomainCourseEnrollments(rows []*ent.CourseEnrollment) []domain.CourseEnrollment {
	result := make([]domain.CourseEnrollment, len(rows))
	for i, row := range rows {
		result[i] = toDomainCourseEnrollment(row)
	}
	return result
}
