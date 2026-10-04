package application

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// warmUpMinMinutes is the shortest session that starts with a warm-up, and
// warmUpMaxShare the most of it a warm-up may take.
const (
	warmUpMinMinutes = 5
	warmUpMaxShare   = 0.25
)

// PracticeSessionService composes practice sessions from what a student
// knows. This version offers play-alongs only: basic diagrams with playback
// on the skills of the student's paths, for the instrument in hand. A
// teacher's custom diagrams are theirs alone to find, so they are never
// offered.
type PracticeSessionService struct {
	instruments  ports.InstrumentRepository
	studentPaths ports.StudentPathRepository
	enrollments  ports.CourseEnrollmentRepository
	contentNodes ports.ContentNodeRepository
	diagrams     ports.DiagramRepository
	states       ports.PracticeItemStateReader
	newID        func() string
	now          func() time.Time
}

func NewPracticeSessionService(
	instruments ports.InstrumentRepository,
	studentPaths ports.StudentPathRepository,
	enrollments ports.CourseEnrollmentRepository,
	contentNodes ports.ContentNodeRepository,
	diagrams ports.DiagramRepository,
	states ports.PracticeItemStateReader,
	newID func() string,
	now func() time.Time,
) *PracticeSessionService {
	return &PracticeSessionService{
		instruments:  instruments,
		studentPaths: studentPaths,
		enrollments:  enrollments,
		contentNodes: contentNodes,
		diagrams:     diagrams,
		states:       states,
		newID:        newID,
		now:          now,
	}
}

// playAlongCandidate is a diagram that can be played along with, the
// node it is offered for, and the student's state on it, if any.
type playAlongCandidate struct {
	diagram domain.Diagram
	nodeID  *string
	state   *domain.PracticeItemState
}

func (c playAlongCandidate) key() string { return domain.PlayAlongItemKey(c.diagram.ID) }

// ComposePlan composes a session of minutes for caller with instrumentID
// in hand. The plan is never empty:
//   - a session of 5 minutes or more starts with a warm-up on a play-along
//     already played clean;
//   - then come due play-alongs, most overdue first, then new ones;
//   - a student caught up on their paths reviews known play-alongs ahead
//     and stretches to play-alongs for the instrument beyond their paths,
//     half the time each.
//
// Each pick is fitted to the minutes by its estimated time; a session too
// short for any of them still offers the first. Every play-along needs an
// instrument in hand, so a session in the head has nothing to offer yet and
// is not found, as is an instrumentID that doesn't exist.
func (s *PracticeSessionService) ComposePlan(ctx context.Context, caller domain.User, instrumentID *string, minutes int) (domain.PracticeSessionPlan, error) {
	if err := domain.ValidatePracticeMinutes(minutes); err != nil {
		return domain.PracticeSessionPlan{}, err
	}
	if instrumentID == nil {
		return domain.PracticeSessionPlan{}, fmt.Errorf("%w: nothing to practise without an instrument in hand yet", domain.ErrNotFound)
	}
	if _, err := s.instruments.GetByID(ctx, *instrumentID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.PracticeSessionPlan{}, fmt.Errorf("%w: no instrument exists with the given instrument_id", domain.ErrNotFound)
		}
		return domain.PracticeSessionPlan{}, err
	}

	onPath, err := s.pathCandidates(ctx, caller.ID, *instrumentID)
	if err != nil {
		return domain.PracticeSessionPlan{}, err
	}

	c := newComposer(s.now(), minutes)
	if minutes >= warmUpMinMinutes {
		c.warmUp(onPath)
	}
	due, fresh, known := c.split(onPath)
	c.fill(due, domain.PracticePickDue, c.budget)
	c.fill(fresh, domain.PracticePickNew, c.budget)
	if len(due) == 0 && len(fresh) == 0 {
		if err := s.catchUp(ctx, c, caller.ID, *instrumentID, onPath, known); err != nil {
			return domain.PracticeSessionPlan{}, err
		}
	}
	if len(c.items) == 0 {
		return domain.PracticeSessionPlan{}, fmt.Errorf("%w: no play-along for this instrument", domain.ErrNotFound)
	}

	return domain.PracticeSessionPlan{ID: s.newID(), InstrumentID: instrumentID, Minutes: minutes, Items: c.items}, nil
}

// catchUp splits what is left of a caught-up session between reviewing
// known play-alongs ahead, soonest due first, and stretching to unseen
// play-alongs for the instrument beyond the student's paths; each takes
// over the other's share when it runs out.
func (s *PracticeSessionService) catchUp(ctx context.Context, c *composer, studentID, instrumentID string, onPath, known []playAlongCandidate) error {
	stretch, err := s.stretchCandidates(ctx, studentID, instrumentID, onPath, c.remaining())
	if err != nil {
		return err
	}
	half := c.remaining() / 2
	c.fill(known, domain.PracticePickReviewAhead, c.used+half)
	c.fill(stretch, domain.PracticePickStretch, c.budget)
	c.fill(known, domain.PracticePickReviewAhead, c.budget)
	return nil
}

// pathCandidates lists the play-alongs for instrumentID on the skills of
// the student's paths, in path order, each with the student's state.
func (s *PracticeSessionService) pathCandidates(ctx context.Context, studentID, instrumentID string) ([]playAlongCandidate, error) {
	skillIDs, err := s.pathSkillIDs(ctx, studentID)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var candidates []playAlongCandidate
	for _, skillID := range skillIDs {
		diagrams, err := s.listDiagrams(ctx, domain.DiagramListFilter{SkillID: skillID, InstrumentID: instrumentID, Kind: domain.DiagramKindBasic})
		if err != nil {
			return nil, err
		}
		for _, d := range diagrams {
			if seen[d.ID] || !playable(d) {
				continue
			}
			seen[d.ID] = true
			candidates = append(candidates, playAlongCandidate{diagram: d, nodeID: &skillID})
		}
	}
	return candidates, s.attachStates(ctx, studentID, candidates)
}

// stretchCandidates lists unseen play-alongs for instrumentID that aren't
// already on the student's paths, a page at a time, until about seconds of
// them are found.
func (s *PracticeSessionService) stretchCandidates(ctx context.Context, studentID, instrumentID string, onPath []playAlongCandidate, seconds int) ([]playAlongCandidate, error) {
	skip := map[string]bool{}
	for _, c := range onPath {
		skip[c.diagram.ID] = true
	}
	var stretch []playAlongCandidate
	found := 0
	page := domain.PageRequest{Limit: domain.MaxPageLimit}
	for found < seconds {
		got, err := s.diagrams.List(ctx, domain.DiagramListFilter{InstrumentID: instrumentID, Kind: domain.DiagramKindBasic}, page)
		if err != nil {
			return nil, err
		}
		unseen, err := s.unseenPlayAlongs(ctx, studentID, got.Items, skip)
		if err != nil {
			return nil, err
		}
		for _, c := range unseen {
			stretch = append(stretch, c)
			found += domain.PlayAlongSeconds(c.diagram, startTempo(c), false)
		}
		page.Offset += len(got.Items)
		if len(got.Items) == 0 || page.Offset >= got.Total {
			break
		}
	}
	return stretch, nil
}

// unseenPlayAlongs keeps the diagrams that can be played along with, aren't
// in skip and the student has never practised, each offered for its first
// skill.
func (s *PracticeSessionService) unseenPlayAlongs(ctx context.Context, studentID string, diagrams []domain.Diagram, skip map[string]bool) ([]playAlongCandidate, error) {
	var batch []playAlongCandidate
	for _, d := range diagrams {
		if skip[d.ID] || !playable(d) {
			continue
		}
		var nodeID *string
		if skillIDs := d.SkillIDs(); len(skillIDs) > 0 {
			nodeID = &skillIDs[0]
		}
		batch = append(batch, playAlongCandidate{diagram: d, nodeID: nodeID})
	}
	if err := s.attachStates(ctx, studentID, batch); err != nil {
		return nil, err
	}
	unseen := batch[:0]
	for _, c := range batch {
		if c.state == nil {
			unseen = append(unseen, c)
		}
	}
	return unseen, nil
}

// pathSkillIDs lists the skills taught on the student's active paths, in
// path order, without repeats.
func (s *PracticeSessionService) pathSkillIDs(ctx context.Context, studentID string) ([]string, error) {
	paths, err := s.activePaths(ctx, studentID)
	if err != nil {
		return nil, err
	}
	var nodeIDs []string
	for _, p := range paths {
		for _, item := range p.Items {
			nodeIDs = append(nodeIDs, item.ContentNodeID)
		}
	}
	nodes, err := s.contentNodes.GetByIDs(ctx, nodeIDs)
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
func (s *PracticeSessionService) activePaths(ctx context.Context, studentID string) ([]domain.StudentPath, error) {
	paths, err := s.studentPaths.ListActiveStandaloneByStudentID(ctx, studentID)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(paths, func(a, b domain.StudentPath) int {
		return cmp.Or(a.AssignedAt.Compare(b.AssignedAt), cmp.Compare(a.ID, b.ID))
	})
	enrollments, err := s.enrollments.ListActiveByStudentID(ctx, studentID)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(enrollments, func(a, b domain.CourseEnrollment) int { return cmp.Compare(a.ID, b.ID) })
	for _, e := range enrollments {
		if e.ActiveCheckpointStudentPathID == nil {
			continue
		}
		path, err := s.studentPaths.GetByID(ctx, *e.ActiveCheckpointStudentPathID)
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func (s *PracticeSessionService) listDiagrams(ctx context.Context, filter domain.DiagramListFilter) ([]domain.Diagram, error) {
	page := domain.PageRequest{Limit: domain.MaxPageLimit}
	var all []domain.Diagram
	for {
		got, err := s.diagrams.List(ctx, filter, page)
		if err != nil {
			return nil, err
		}
		all = append(all, got.Items...)
		page.Offset += len(got.Items)
		if len(got.Items) == 0 || page.Offset >= got.Total {
			return all, nil
		}
	}
}

// attachStates fills in the student's state on each candidate that has one.
func (s *PracticeSessionService) attachStates(ctx context.Context, studentID string, candidates []playAlongCandidate) error {
	if len(candidates) == 0 {
		return nil
	}
	keys := make([]string, len(candidates))
	for i, c := range candidates {
		keys[i] = c.key()
	}
	states, err := s.states.GetStates(ctx, studentID, keys)
	if err != nil {
		return err
	}
	for i, c := range candidates {
		if state, ok := states[c.key()]; ok {
			candidates[i].state = &state
		}
	}
	return nil
}

// playable reports whether d can be played along with: it has a sequence
// and a tempo.
func playable(d domain.Diagram) bool {
	return len(d.Sequence) > 0 && d.TempoBPM != nil
}

func bestClean(c playAlongCandidate) *int {
	if c.state == nil {
		return nil
	}
	return c.state.BestCleanBPM
}

func startTempo(c playAlongCandidate) int {
	return domain.PlayAlongStartTempo(*c.diagram.TempoBPM, bestClean(c))
}

// composer builds a session's items within its time budget, offering each
// play-along at most once.
type composer struct {
	now    time.Time
	budget int
	used   int
	picked map[string]bool
	items  []domain.PracticeSessionItem
	// first is the first pick refused for lack of time, offered anyway when
	// nothing else fits.
	first *domain.PracticeSessionItem
}

func newComposer(now time.Time, minutes int) *composer {
	return &composer{now: now, budget: minutes * 60, picked: map[string]bool{}}
}

func (c *composer) remaining() int { return c.budget - c.used }

// split sorts the student's path play-alongs into due ones, most overdue
// first, new ones in path order, and known ones not yet due, soonest due
// first.
func (c *composer) split(candidates []playAlongCandidate) (due, fresh, known []playAlongCandidate) {
	for _, cand := range candidates {
		switch {
		case c.picked[cand.key()]:
		case cand.state == nil || cand.state.Counted == 0:
			fresh = append(fresh, cand)
		case cand.state.Due(c.now):
			due = append(due, cand)
		default:
			known = append(known, cand)
		}
	}
	byDueAt := func(a, b playAlongCandidate) int { return compareDueAt(a.state.DueAt, b.state.DueAt) }
	slices.SortStableFunc(due, byDueAt)
	slices.SortStableFunc(known, byDueAt)
	return due, fresh, known
}

// compareDueAt orders review dates soonest first, no date before any.
func compareDueAt(a, b *time.Time) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return -1
	case b == nil:
		return 1
	default:
		return a.Compare(*b)
	}
}

// warmUp opens the session with the best-known play-along already played
// clean, if there is one and it takes no more than a quarter of the session.
func (c *composer) warmUp(candidates []playAlongCandidate) {
	var best *playAlongCandidate
	for i, cand := range candidates {
		if cand.state == nil || cand.state.BestCleanBPM == nil {
			continue
		}
		if best == nil || betterWarmUp(cand, *best, c.now) {
			best = &candidates[i]
		}
	}
	if best == nil {
		return
	}
	tempo := domain.WarmUpTempo(*best.diagram.TempoBPM, *best.state.BestCleanBPM)
	item := c.item(*best, domain.PracticePickWarmUp, tempo, domain.PlayAlongSeconds(best.diagram, tempo, true))
	if float64(item.EstimatedSeconds) > float64(c.budget)*warmUpMaxShare {
		return
	}
	c.add(item)
}

// betterWarmUp prefers the higher shown level, then the more recently
// played.
func betterWarmUp(a, b playAlongCandidate, now time.Time) bool {
	la, lb := levelRank(a.state.ShownLevel(now)), levelRank(b.state.ShownLevel(now))
	if la != lb {
		return la > lb
	}
	return compareDueAt(a.state.LastAt, b.state.LastAt) > 0
}

func levelRank(l domain.KnowledgeLevel) int {
	return slices.Index([]domain.KnowledgeLevel{
		domain.KnowledgeLevelNew, domain.KnowledgeLevelLearning, domain.KnowledgeLevelAccurate,
		domain.KnowledgeLevelFluent, domain.KnowledgeLevelRetained,
	}, l)
}

// fill adds candidates with reason, in order, while they fit until limit
// seconds are used.
func (c *composer) fill(candidates []playAlongCandidate, reason domain.PracticePickReason, limit int) {
	for _, cand := range candidates {
		if c.picked[cand.key()] {
			continue
		}
		tempo := startTempo(cand)
		item := c.item(cand, reason, tempo, domain.PlayAlongSeconds(cand.diagram, tempo, false))
		if c.used+item.EstimatedSeconds > limit {
			if c.first == nil {
				c.first = &item
			}
			continue
		}
		c.add(item)
	}
	if len(c.items) == 0 && c.first != nil {
		c.add(*c.first)
	}
}

func (c *composer) add(item domain.PracticeSessionItem) {
	c.picked[item.ItemKey] = true
	c.used += item.EstimatedSeconds
	c.items = append(c.items, item)
}

func (c *composer) item(cand playAlongCandidate, reason domain.PracticePickReason, tempo, seconds int) domain.PracticeSessionItem {
	level := domain.KnowledgeLevelNew
	if cand.state != nil {
		level = cand.state.ShownLevel(c.now)
	}
	return domain.PracticeSessionItem{
		ItemKey:          cand.key(),
		Kind:             domain.PracticeItemKindPlayAlong,
		Reason:           reason,
		NodeID:           cand.nodeID,
		Level:            level,
		EstimatedSeconds: seconds,
		PlayAlong: &domain.PlannedPlayAlong{
			DiagramID:         cand.diagram.ID,
			StartTempoBPM:     tempo,
			TargetTempoBPM:    *cand.diagram.TempoBPM,
			BestCleanTempoBPM: bestClean(cand),
		},
	}
}
