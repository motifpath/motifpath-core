package repo

import (
	"context"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/studentlearningstate"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntStudentLearningStateRepository persists each student's current
// course-enrollment/standalone-path pointer via ent/Postgres.
type EntStudentLearningStateRepository struct {
	client *ent.Client
}

func NewEntStudentLearningStateRepository(client *ent.Client) *EntStudentLearningStateRepository {
	return &EntStudentLearningStateRepository{client: client}
}

func (r *EntStudentLearningStateRepository) GetByStudentID(ctx context.Context, studentID string) (domain.StudentLearningState, error) {
	parsed, err := uuid.Parse(studentID)
	if err != nil {
		return domain.StudentLearningState{}, domain.ErrNotFound
	}

	row, err := r.client.StudentLearningState.Query().
		Where(studentlearningstate.StudentID(parsed)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.StudentLearningState{}, domain.ErrNotFound
		}
		return domain.StudentLearningState{}, err
	}

	var currentCourseEnrollmentID, currentStandalonePathID *string
	if row.CurrentCourseEnrollmentID != nil {
		s := row.CurrentCourseEnrollmentID.String()
		currentCourseEnrollmentID = &s
	}
	if row.CurrentStandalonePathID != nil {
		s := row.CurrentStandalonePathID.String()
		currentStandalonePathID = &s
	}

	return domain.StudentLearningState{
		StudentID:                 row.StudentID.String(),
		CurrentCourseEnrollmentID: currentCourseEnrollmentID,
		CurrentStandalonePathID:   currentStandalonePathID,
	}, nil
}

// Upsert creates or replaces the StudentLearningState row for
// state.StudentID, deleting any existing row first — the same
// delete-then-insert pattern PathAssignment's ReplaceActive used, since a
// student has at most one StudentLearningState row and every write of it
// replaces the row wholesale.
func (r *EntStudentLearningStateRepository) Upsert(ctx context.Context, state domain.StudentLearningState) error {
	studentID, err := uuid.Parse(state.StudentID)
	if err != nil {
		return err
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}

	if _, err := tx.StudentLearningState.Delete().Where(studentlearningstate.StudentID(studentID)).Exec(ctx); err != nil {
		return rollback(tx, err)
	}

	create := tx.StudentLearningState.Create().SetStudentID(studentID)
	if state.CurrentCourseEnrollmentID != nil {
		enrollmentID, err := uuid.Parse(*state.CurrentCourseEnrollmentID)
		if err != nil {
			return rollback(tx, err)
		}
		create = create.SetCurrentCourseEnrollmentID(enrollmentID)
	}
	if state.CurrentStandalonePathID != nil {
		pathID, err := uuid.Parse(*state.CurrentStandalonePathID)
		if err != nil {
			return rollback(tx, err)
		}
		create = create.SetCurrentStandalonePathID(pathID)
	}
	if _, err := create.Save(ctx); err != nil {
		return rollback(tx, err)
	}

	return tx.Commit()
}
