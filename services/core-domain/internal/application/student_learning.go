package application

import (
	"cmp"
	"context"
	"errors"
	"slices"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// studentLearning reads what a student is learning: their active paths,
// the skills those teach and the instruments they are for.
type studentLearning struct {
	studentPaths   ports.StudentPathRepository
	enrollments    ports.CourseEnrollmentRepository
	contentNodes   ports.ContentNodeRepository
	learningPaths  ports.LearningPathRepository
	courseVersions ports.CourseVersionRepository
	instruments    ports.InstrumentRepository
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

// instrumentIDs lists the instruments of the student's active
// standalone paths and course enrollments, in the instruments' order. A
// path or course for every instrument adds none.
func (l studentLearning) instrumentIDs(ctx context.Context, studentID string) ([]string, error) {
	plays := map[string]bool{}
	if err := l.addPathInstruments(ctx, studentID, plays); err != nil {
		return nil, err
	}
	if err := l.addCourseInstruments(ctx, studentID, plays); err != nil {
		return nil, err
	}
	instruments, err := l.instruments.List(ctx)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, instrument := range instruments {
		if plays[instrument.ID] {
			ids = append(ids, instrument.ID)
		}
	}
	return ids, nil
}

// addPathInstruments marks in plays the instruments of the templates of
// the student's active standalone paths. A deleted template adds none.
func (l studentLearning) addPathInstruments(ctx context.Context, studentID string, plays map[string]bool) error {
	paths, err := l.studentPaths.ListActiveStandaloneByStudentID(ctx, studentID)
	if err != nil {
		return err
	}
	for _, p := range paths {
		template, err := l.learningPaths.GetByID(ctx, p.SourceTemplateID)
		if errors.Is(err, domain.ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		for _, id := range template.InstrumentIDs {
			plays[id] = true
		}
	}
	return nil
}

// addCourseInstruments marks in plays the instruments of the course
// versions the student is actively enrolled in.
func (l studentLearning) addCourseInstruments(ctx context.Context, studentID string, plays map[string]bool) error {
	enrollments, err := l.enrollments.ListActiveByStudentID(ctx, studentID)
	if err != nil {
		return err
	}
	for _, e := range enrollments {
		version, err := l.courseVersions.GetByCourseIDAndVersionNumber(ctx, e.CourseID, e.CourseVersionNumber)
		if errors.Is(err, domain.ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		for _, id := range version.InstrumentIDsSnapshot {
			plays[id] = true
		}
	}
	return nil
}
