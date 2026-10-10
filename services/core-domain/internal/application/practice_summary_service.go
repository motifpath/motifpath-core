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
	instruments ports.InstrumentRepository
	learning    studentLearning
	rollup      *KnowledgeRollupService
	activity    ports.PracticeActivityReader
	songCharts  ports.SongChartRepository
	now         func() time.Time
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
	songCharts ports.SongChartRepository,
	now func() time.Time,
) *PracticeSummaryService {
	return &PracticeSummaryService{
		instruments: instruments,
		learning: studentLearning{
			studentPaths: studentPaths, enrollments: enrollments, contentNodes: contentNodes,
			learningPaths: learningPaths, courseVersions: courseVersions, instruments: instruments,
		},
		rollup:     rollup,
		activity:   activity,
		songCharts: songCharts,
		now:        now,
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
	PracticeDaysLast7         int
	LearningDaysLast7         int
	MinutesPractisedLast7     int
	MinutesPractisedPrevious7 int
	DayStreakCurrent          int
	DayStreakBest             int
	// SkillsUpLast7 counts each improved skill once, whatever instruments
	// it improved on.
	SkillsUpLast7    int
	SongsPlayedTotal int
	SongsPlayedLast7 int
	Instruments      []PracticeInstrumentCard
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
	instrumentIDs, err := s.learning.instrumentIDs(ctx, caller.ID)
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
// (empty: UTC): practice days on any instrument, learning days, minutes
// practised this week and the week before, the current and best day
// streaks, skills up, songs played, and one card per instrument of the
// student with its practice days and its top next step. An unknown time
// zone is a validation error on time_zone.
func (s *PracticeSummaryService) Overview(ctx context.Context, caller domain.User, timeZone string) (PracticeOverview, error) {
	loc, err := practiceLocation(timeZone)
	if err != nil {
		return PracticeOverview{}, err
	}
	instrumentIDs, err := s.learning.instrumentIDs(ctx, caller.ID)
	if err != nil {
		return PracticeOverview{}, err
	}
	pathSkillIDs, err := s.learning.pathSkillIDs(ctx, caller.ID)
	if err != nil {
		return PracticeOverview{}, err
	}
	now := s.now()
	start := domain.Last7DaysStart(now, loc)
	previousStart := domain.Previous7DaysStart(now, loc)
	// Every finished session ever, since the best streak can lie anywhere
	// in the student's history.
	sessions, err := s.activity.FinishedSessions(ctx, caller.ID, time.Time{})
	if err != nil {
		return PracticeOverview{}, err
	}
	completions, err := s.activity.CompletionTimes(ctx, caller.ID, start)
	if err != nil {
		return PracticeOverview{}, err
	}
	spans, err := s.activity.SessionSpans(ctx, caller.ID, previousStart)
	if err != nil {
		return PracticeOverview{}, err
	}

	ends := make([]time.Time, len(sessions))
	for i, session := range sessions {
		ends[i] = session.EndedAt
	}
	songs, err := s.songsPlayed(ctx, caller.ID, now, loc)
	if err != nil {
		return PracticeOverview{}, err
	}
	streak, bestStreak := domain.DayStreaks(ends, now, loc)
	views := make([]KnowledgeMap, len(instrumentIDs))
	for i, id := range instrumentIDs {
		if views[i], err = s.rollup.Map(ctx, caller.ID, id); err != nil {
			return PracticeOverview{}, err
		}
	}
	skillsUp, err := s.skillsUp(ctx, caller.ID, views, start)
	if err != nil {
		return PracticeOverview{}, err
	}
	overview := PracticeOverview{
		PracticeDaysLast7:         domain.DaysInLast7(ends, now, loc),
		LearningDaysLast7:         domain.DaysInLast7(completions, now, loc),
		MinutesPractisedLast7:     domain.MinutesPractised(spans, start, endOfToday(now, loc)),
		MinutesPractisedPrevious7: domain.MinutesPractised(spans, previousStart, start),
		DayStreakCurrent:          streak,
		DayStreakBest:             bestStreak,
		SkillsUpLast7:             skillsUp,
		SongsPlayedTotal:          songs.Total,
		SongsPlayedLast7:          songs.Last7,
	}
	for i, id := range instrumentIDs {
		card := PracticeInstrumentCard{InstrumentID: id, PracticeDaysLast7: domain.DaysInLast7(sessionEnds(sessions, &id), now, loc)}
		if steps := domain.RankNextSteps(views[i], pathSkillIDs); len(steps) > 0 {
			card.TopNextStep = &steps[0]
		}
		overview.Instruments = append(overview.Instruments, card)
	}
	return overview, nil
}

// songsPlayed counts the song charts studentID has marked as played that
// still exist: a played mark can name a chart that never existed, since the
// reader's events are taken as sent.
func (s *PracticeSummaryService) songsPlayed(ctx context.Context, studentID string, now time.Time, loc *time.Location) (domain.SongsPlayed, error) {
	completions, err := s.activity.SongChartCompletions(ctx, studentID)
	if err != nil || len(completions) == 0 {
		return domain.SongsPlayed{}, err
	}
	ids := make([]string, 0, len(completions))
	for _, c := range completions {
		ids = append(ids, c.SongChartID)
	}
	slices.Sort(ids)
	existing, err := s.songCharts.ExistingIDs(ctx, slices.Compact(ids))
	if err != nil {
		return domain.SongsPlayed{}, err
	}
	exists := make(map[string]bool, len(existing))
	for _, id := range existing {
		exists[id] = true
	}
	return domain.CountSongsPlayed(completions, exists, now, loc), nil
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

// skillsUp counts the distinct skills studentID improved since start, in
// any of the instruments' views or on the items that suit every
// instrument: a skill that improved on several instruments, or on several
// measures, is one skill up, and a student with no instrument still has
// theirs.
func (s *PracticeSummaryService) skillsUp(ctx context.Context, studentID string, instrumentViews []KnowledgeMap, start time.Time) (int, error) {
	// The empty instrument is the items that suit every instrument.
	everyInstrument, err := s.rollup.Map(ctx, studentID, "")
	if err != nil {
		return 0, err
	}
	skills := map[string]bool{}
	for _, view := range append([]KnowledgeMap{everyInstrument}, instrumentViews...) {
		progress, err := s.progress(ctx, studentID, view, start)
		if err != nil {
			return 0, err
		}
		for _, line := range progress {
			skills[line.NodeID] = true
		}
	}
	return len(skills), nil
}

// endOfToday is the local midnight that ends now's day in loc.
func endOfToday(now time.Time, loc *time.Location) time.Time {
	y, m, d := now.In(loc).Date()
	return time.Date(y, m, d+1, 0, 0, 0, 0, loc)
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
