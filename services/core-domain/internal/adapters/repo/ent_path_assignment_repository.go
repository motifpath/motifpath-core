package repo

import (
	"context"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/pathassignment"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntPathAssignmentRepository persists PathAssignment records via
// ent/Postgres.
type EntPathAssignmentRepository struct {
	client *ent.Client
}

func NewEntPathAssignmentRepository(client *ent.Client) *EntPathAssignmentRepository {
	return &EntPathAssignmentRepository{client: client}
}

// ReplaceActive deletes any existing assignment for the student and inserts
// assignment as the new one, in a single transaction — "assigning a path"
// always produces a fresh assignment_id, whether or not the student already
// had an active one.
func (r *EntPathAssignmentRepository) ReplaceActive(ctx context.Context, assignment domain.PathAssignment) error {
	id, err := uuid.Parse(assignment.ID)
	if err != nil {
		return err
	}
	studentID, err := uuid.Parse(assignment.StudentID)
	if err != nil {
		return err
	}
	learningPathID, err := uuid.Parse(assignment.LearningPathID)
	if err != nil {
		return err
	}
	assignedBy, err := uuid.Parse(assignment.AssignedBy)
	if err != nil {
		return err
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}

	if _, err := tx.PathAssignment.Delete().Where(pathassignment.StudentID(studentID)).Exec(ctx); err != nil {
		return rollback(tx, err)
	}

	if _, err := tx.PathAssignment.Create().
		SetID(id).
		SetStudentID(studentID).
		SetLearningPathID(learningPathID).
		SetAssignedBy(assignedBy).
		SetAssignedAt(assignment.AssignedAt).
		Save(ctx); err != nil {
		return rollback(tx, err)
	}

	return tx.Commit()
}

func (r *EntPathAssignmentRepository) GetActiveByStudentID(ctx context.Context, studentID string) (domain.PathAssignment, error) {
	parsed, err := uuid.Parse(studentID)
	if err != nil {
		return domain.PathAssignment{}, domain.ErrNotFound
	}
	row, err := r.client.PathAssignment.Query().Where(pathassignment.StudentID(parsed)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.PathAssignment{}, domain.ErrNotFound
		}
		return domain.PathAssignment{}, err
	}
	return domain.PathAssignment{
		ID:             row.ID.String(),
		StudentID:      row.StudentID.String(),
		LearningPathID: row.LearningPathID.String(),
		AssignedBy:     row.AssignedBy.String(),
		AssignedAt:     row.AssignedAt,
	}, nil
}
