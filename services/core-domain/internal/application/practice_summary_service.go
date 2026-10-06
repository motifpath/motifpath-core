package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"
	// The service resolves students' IANA time zones wherever it runs,
	// including images without the system's zone database.
	_ "time/tzdata"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// topNextSteps is how many next steps the practice home shows before "see
// all".
const topNextSteps = 3

// PracticeSummaryService derives the practice home from a student's
// evidence and activity. Reading it changes nothing.
type PracticeSummaryService struct {
	instruments    ports.InstrumentRepository
	learning       studentLearning
	learningPaths  ports.LearningPathRepository
	courseVersions ports.CourseVersionRepository
	rollup         *KnowledgeRollupService
	activity       ports.PracticeActivityReader
	now            func() time.Time
}

func NewPracticeSummaryService(
	instruments ports.InstrumentRepository,
	studentPaths ports.StudentPathRepository,
	enrollments ports.CourseEnrollmentRepository,
	learningPaths ports.LearningPathRepository,
	courseVersions ports.CourseVersionRepository,
	contentNodes ports.ContentNodeRepository,
	rollup *KnowledgeRollupService,
	activity ports.PracticeActivityReader,
	now func() time.Time,
) *PracticeSummaryService {
	return &PracticeSummaryService{
		instruments:    instruments,
		learning:       studentLearning{studentPaths: studentPaths, enrollments: enrollments, contentNodes: contentNodes},
		learningPaths:  learningPaths,
		courseVersions: courseVersions,
		rollup:         rollup,
		activity:       activity,
		now:            now,
	}
}

// PracticeSummary is the practice home for one instrument.
type PracticeSummary struct {
	// InstrumentID is nil for the nodes that suit any instrument only.
	InstrumentID         *string
	StudentInstrumentIDs []string
	PracticeDaysLast7    int
	// Progress lists the skills that improved in the last 7 days, most
	// improved first.
	Progress []domain.SkillProgress
	// NextSteps holds the top next steps, and NextStepsTotal counts them
	// all.
	NextSteps      []domain.PracticeNextStep
	NextStepsTotal int
	Groups         []domain.PracticeNodeGroup
	// Nodes, Standings and Children describe every node the summary
	// mentions, keyed by node id: Children lists a node's children with
	// something to practise, shown by their own levels.
	Nodes     map[string]domain.KnowledgeNode
	Standings map[string]domain.NodeStanding
	Children  map[string][]string
}

// PracticeOverview is the practice home's first view, across instruments.
type PracticeOverview struct {
	PracticeDaysLast7 int
	LearningDaysLast7 int
	Instruments       []PracticeInstrumentCard
}

// PracticeInstrumentCard is one instrument at a glance.
type PracticeInstrumentCard struct {
	InstrumentID      string
	PracticeDaysLast7 int
	// TopNextStep is the instrument's first next step; nil when there is
	// none.
	TopNextStep *domain.PracticeNextStep
}

// FretboardMap returns how well caller knows each fretboard cell of
// instrumentID, an instrument that must exist. An instrument whose layout
// has no generated cells has an empty map.
func (s *PracticeSummaryService) FretboardMap(ctx context.Context, caller domain.User, instrumentID string) (domain.FretboardMap, error) {
	if err := s.requireInstrument(ctx, &instrumentID); err != nil {
		return domain.FretboardMap{}, err
	}
	return s.rollup.FretboardMap(ctx, caller.ID, instrumentID)
}

// Summary returns caller's practice summary for instrumentID (nil: the
// nodes that suit any instrument only), counting days in timeZone (empty:
// UTC):
//   - practice days: days in the last 7 with a session finished with that
//     instrument in hand;
//   - progress: each skill now against its state when the last 7 days
//     began;
//   - next steps: the top three, and how many there are;
//   - every node with something to practise, grouped by area.
//
// An unknown time zone is a validation error on time_zone, and an
// instrumentID that doesn't exist is not found.
func (s *PracticeSummaryService) Summary(ctx context.Context, caller domain.User, instrumentID *string, timeZone string) (PracticeSummary, error) {
	loc, err := practiceLocation(timeZone)
	if err != nil {
		return PracticeSummary{}, err
	}
	if err := s.requireInstrument(ctx, instrumentID); err != nil {
		return PracticeSummary{}, err
	}
	instrumentIDs, err := s.studentInstrumentIDs(ctx, caller.ID)
	if err != nil {
		return PracticeSummary{}, err
	}
	pathSkillIDs, err := s.learning.pathSkillIDs(ctx, caller.ID)
	if err != nil {
		return PracticeSummary{}, err
	}
	view, err := s.rollup.Map(ctx, caller.ID, deref(instrumentID))
	if err != nil {
		return PracticeSummary{}, err
	}
	now := s.now()
	start := domain.Last7DaysStart(now, loc)
	sessions, err := s.activity.FinishedSessions(ctx, caller.ID, start)
	if err != nil {
		return PracticeSummary{}, err
	}
	progress, err := s.progress(ctx, caller.ID, view, start)
	if err != nil {
		return PracticeSummary{}, err
	}

	steps := domain.RankNextSteps(view, pathSkillIDs)
	nodes, children := describeNodes(view)
	return PracticeSummary{
		InstrumentID:         instrumentID,
		StudentInstrumentIDs: instrumentIDs,
		PracticeDaysLast7:    domain.DaysInLast7(sessionEnds(sessions, instrumentID), now, loc),
		Progress:             progress,
		NextSteps:            steps[:min(topNextSteps, len(steps))],
		NextStepsTotal:       len(steps),
		Groups:               domain.GroupPracticeNodes(view.Nodes, view.Standings),
		Nodes:                nodes,
		Standings:            view.Standings,
		Children:             children,
	}, nil
}

// requireInstrument reports an instrumentID that doesn't exist as not
// found; nil, for the nodes that suit any instrument, always exists.
func (s *PracticeSummaryService) requireInstrument(ctx context.Context, instrumentID *string) error {
	if instrumentID == nil {
		return nil
	}
	_, err := s.instruments.GetByID(ctx, *instrumentID)
	if errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("%w: no instrument exists with the given instrument_id", domain.ErrNotFound)
	}
	return err
}

// describeNodes keys view's nodes by id, and lists each node's children
// with something to practise.
func describeNodes(view domain.KnowledgeView) (map[string]domain.KnowledgeNode, map[string][]string) {
	nodes := make(map[string]domain.KnowledgeNode, len(view.Nodes))
	children := map[string][]string{}
	for _, n := range view.Nodes {
		nodes[n.ID] = n
		if n.ParentID != nil && view.Standings[n.ID].Total > 0 {
			children[*n.ParentID] = append(children[*n.ParentID], n.ID)
		}
	}
	return nodes, children
}

// Overview returns caller's practice overview, counting days in timeZone
// (empty: UTC): practice days on any instrument, learning days, and one
// card per instrument of the student with its practice days and its top
// next step. An unknown time zone is a validation error on time_zone.
func (s *PracticeSummaryService) Overview(ctx context.Context, caller domain.User, timeZone string) (PracticeOverview, error) {
	loc, err := practiceLocation(timeZone)
	if err != nil {
		return PracticeOverview{}, err
	}
	instrumentIDs, err := s.studentInstrumentIDs(ctx, caller.ID)
	if err != nil {
		return PracticeOverview{}, err
	}
	pathSkillIDs, err := s.learning.pathSkillIDs(ctx, caller.ID)
	if err != nil {
		return PracticeOverview{}, err
	}
	now := s.now()
	start := domain.Last7DaysStart(now, loc)
	sessions, err := s.activity.FinishedSessions(ctx, caller.ID, start)
	if err != nil {
		return PracticeOverview{}, err
	}
	completions, err := s.activity.CompletionTimes(ctx, caller.ID, start)
	if err != nil {
		return PracticeOverview{}, err
	}

	ends := make([]time.Time, len(sessions))
	for i, session := range sessions {
		ends[i] = session.EndedAt
	}
	overview := PracticeOverview{
		PracticeDaysLast7: domain.DaysInLast7(ends, now, loc),
		LearningDaysLast7: domain.DaysInLast7(completions, now, loc),
	}
	for _, id := range instrumentIDs {
		view, err := s.rollup.Map(ctx, caller.ID, id)
		if err != nil {
			return PracticeOverview{}, err
		}
		card := PracticeInstrumentCard{InstrumentID: id, PracticeDaysLast7: domain.DaysInLast7(sessionEnds(sessions, &id), now, loc)}
		if steps := domain.RankNextSteps(view, pathSkillIDs); len(steps) > 0 {
			card.TopNextStep = &steps[0]
		}
		overview.Instruments = append(overview.Instruments, card)
	}
	return overview, nil
}

// progress lists how the leaf skills in view improved since start, most
// improved first.
func (s *PracticeSummaryService) progress(ctx context.Context, studentID string, view domain.KnowledgeView, start time.Time) ([]domain.SkillProgress, error) {
	var skills []domain.KnowledgeNode
	var keys []string
	for _, n := range view.Nodes {
		if n.Kind == domain.KnowledgeNodeKindSkill && !view.Wide(n.ID) && view.Practised(n.ID) {
			skills = append(skills, n)
			keys = append(keys, view.Subtrees[n.ID]...)
		}
	}
	if len(skills) == 0 {
		return nil, nil
	}
	slices.Sort(keys)
	before, err := s.activity.SnapshotsAt(ctx, studentID, slices.Compact(keys), start)
	if err != nil {
		return nil, err
	}
	var lines []domain.SkillProgress
	for _, n := range skills {
		lines = append(lines, domain.SkillProgressLines(n, view.Subtrees[n.ID], view.States, before)...)
	}
	domain.RankSkillProgress(lines)
	return lines, nil
}

// studentInstrumentIDs lists the instruments of the student's active
// standalone paths and course enrollments, in the instruments' order. A
// path or course for every instrument adds none.
func (s *PracticeSummaryService) studentInstrumentIDs(ctx context.Context, studentID string) ([]string, error) {
	plays := map[string]bool{}
	if err := s.addPathInstruments(ctx, studentID, plays); err != nil {
		return nil, err
	}
	if err := s.addCourseInstruments(ctx, studentID, plays); err != nil {
		return nil, err
	}
	instruments, err := s.instruments.List(ctx)
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
func (s *PracticeSummaryService) addPathInstruments(ctx context.Context, studentID string, plays map[string]bool) error {
	paths, err := s.learning.studentPaths.ListActiveStandaloneByStudentID(ctx, studentID)
	if err != nil {
		return err
	}
	for _, p := range paths {
		template, err := s.learningPaths.GetByID(ctx, p.SourceTemplateID)
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
func (s *PracticeSummaryService) addCourseInstruments(ctx context.Context, studentID string, plays map[string]bool) error {
	enrollments, err := s.learning.enrollments.ListActiveByStudentID(ctx, studentID)
	if err != nil {
		return err
	}
	for _, e := range enrollments {
		version, err := s.courseVersions.GetByCourseIDAndVersionNumber(ctx, e.CourseID, e.CourseVersionNumber)
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

// sessionEnds lists when the sessions with instrumentID in hand (nil: in
// the head) ended.
func sessionEnds(sessions []domain.FinishedPracticeSession, instrumentID *string) []time.Time {
	var ends []time.Time
	for _, session := range sessions {
		if deref(session.InstrumentID) == deref(instrumentID) {
			ends = append(ends, session.EndedAt)
		}
	}
	return ends
}

// practiceLocation resolves a student's IANA time zone, UTC when none is
// given.
func practiceLocation(timeZone string) (*time.Location, error) {
	if timeZone == "" {
		return time.UTC, nil
	}
	loc, err := time.LoadLocation(timeZone)
	if err != nil || timeZone == "Local" {
		return nil, domain.NewValidationError("time_zone", "must be a known IANA time zone, such as America/Sao_Paulo")
	}
	return loc, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
