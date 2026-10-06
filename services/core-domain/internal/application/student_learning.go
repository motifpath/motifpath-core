package application

import (
	"cmp"
	"context"
	"slices"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// studentLearning reads what a student is learning: their active paths
// and the skills those teach.
type studentLearning struct {
	studentPaths ports.StudentPathRepository
	enrollments  ports.CourseEnrollmentRepository
	contentNodes ports.ContentNodeRepository
}

// pathSkillIDs lists the skills taught on the student's active paths, in
// path order, without repeats.
func (l studentLearning) pathSkillIDs(ctx context.Context, studentID string) ([]string, error) {
	paths, err := l.activePaths(ctx, studentID)
	if err != nil {
		return nil, err
	}
	var nodeIDs []string
	for _, p := range paths {
		for _, item := range p.Items {
			nodeIDs = append(nodeIDs, item.ContentNodeID)
		}
	}
	nodes, err := l.contentNodes.GetByIDs(ctx, nodeIDs)
	if err != nil {
		return nil, err
	}
	var skillIDs []string
	for _, id := range nodeIDs {
		for _, skillID := range nodes[id].Classification.SkillIDs() {
			if !slices.Contains(skillIDs, skillID) {
				skillIDs = append(skillIDs, skillID)
			}
		}
	}
	return skillIDs, nil
}

// activePaths lists the student's active standalone paths by assignment,
// then the active checkpoint of each active course enrollment.
func (l studentLearning) activePaths(ctx context.Context, studentID string) ([]domain.StudentPath, error) {
	paths, err := l.studentPaths.ListActiveStandaloneByStudentID(ctx, studentID)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(paths, func(a, b domain.StudentPath) int {
		return cmp.Or(a.AssignedAt.Compare(b.AssignedAt), cmp.Compare(a.ID, b.ID))
	})
	enrollments, err := l.enrollments.ListActiveByStudentID(ctx, studentID)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(enrollments, func(a, b domain.CourseEnrollment) int { return cmp.Compare(a.ID, b.ID) })
	for _, e := range enrollments {
		if e.ActiveCheckpointStudentPathID == nil {
			continue
		}
		path, err := l.studentPaths.GetByID(ctx, *e.ActiveCheckpointStudentPathID)
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}
