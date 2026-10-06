package application

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// warmUpMinMinutes is the shortest session that starts with a warm-up, and
// warmUpMaxShare the most of it a warm-up may take. applicationMinMinutes
// is the shortest session that ends by applying a skill to music, and
// applicationMaxShare the most of it that ending may take.
const (
	warmUpMinMinutes      = 5
	warmUpMaxShare        = 0.25
	applicationMinMinutes = 10
	applicationMaxShare   = 0.25
	// reviewAheadDays is how far ahead a known item may be reviewed before
	// it falls due.
	reviewAheadDays = 7
)

// The shares of a session's focus time, in percent: the time left after
// its warm-up and its application ending.
const (
	dueSharePercent  = 60
	weakSharePercent = 25
	newSharePercent  = 15
)

// PracticeSessionService composes practice sessions from what a student
// knows. It offers play-alongs, basic diagrams with playback, authored
// exercises and fretboard cells: for the instrument in hand, or, in the
// head, everything but play-alongs for all the student's instruments. A
// teacher's custom diagrams are theirs alone to find, so they are never
// offered.
type PracticeSessionService struct {
	instruments ports.InstrumentRepository
	learning    studentLearning
	diagrams    ports.DiagramRepository
	exercises   ports.ExerciseRepository
	rollup      *KnowledgeRollupService
	tapChecks   ports.TapCheckReader
	feltRatings ports.FeltRatingReader
	newID       func() string
	now         func() time.Time
}

func NewPracticeSessionService(
	instruments ports.InstrumentRepository,
	studentPaths ports.StudentPathRepository,
	enrollments ports.CourseEnrollmentRepository,
	learningPaths ports.LearningPathRepository,
	courseVersions ports.CourseVersionRepository,
	contentNodes ports.ContentNodeRepository,
	diagrams ports.DiagramRepository,
	exercises ports.ExerciseRepository,
	rollup *KnowledgeRollupService,
	tapChecks ports.TapCheckReader,
	feltRatings ports.FeltRatingReader,
	newID func() string,
	now func() time.Time,
) *PracticeSessionService {
	return &PracticeSessionService{
		instruments: instruments,
		learning: studentLearning{
			studentPaths: studentPaths, enrollments: enrollments, contentNodes: contentNodes,
			learningPaths: learningPaths, courseVersions: courseVersions, instruments: instruments,
		},
		diagrams:    diagrams,
		exercises:   exercises,
		rollup:      rollup,
		tapChecks:   tapChecks,
		feltRatings: feltRatings,
		newID:       newID,
		now:         now,
	}
}

// practiceCandidate is an item that can be practised in a session, a
// play-along, an exercise or a fretboard cell, with the node it is offered
// for, the student's state on it, if any, and the instrument it was found
// for, which new items are balanced across.
type practiceCandidate struct {
	diagram      *domain.Diagram
	exercise     *domain.Exercise
	cell         *domain.FretboardCell
	nodeID       *string
	state        *domain.PracticeItemState
	instrumentID string
}

func (c practiceCandidate) key() string {
	switch {
	case c.diagram != nil:
		return domain.PlayAlongItemKey(c.diagram.ID)
	case c.cell != nil:
		return domain.FretboardCellItemKey(c.cell.LayoutInstrumentID, c.cell.String, c.cell.Fret)
	default:
		return domain.ExerciseItemKey(c.exercise.ID)
	}
}

// practised reports whether the student has a counted answer on c.
func (c practiceCandidate) practised() bool {
	return c.state != nil && c.state.Counted > 0
}

// skillIDs lists the skills a play-along or an exercise is classified
// under.
func (c practiceCandidate) skillIDs() []string {
	if c.diagram != nil {
		return c.diagram.SkillIDs()
	}
	if c.exercise != nil {
		return knowledgeNodeIDs(c.exercise.Skills)
	}
	return nil
}

func knowledgeNodeIDs(nodes []domain.KnowledgeNode) []string {
	ids := make([]string, len(nodes))
	for i, n := range nodes {
		ids[i] = n.ID
	}
	return ids
}

// ComposePlan composes a session of minutes for caller with instrumentID
// in hand, or in the head when instrumentID is nil:
//   - with the instrument in hand, a session of 5 minutes or more starts
//     with a warm-up on a play-along already played clean, and one of 10
//     minutes or more ends by applying a skill to music: a play-along on a
//     skill the focus items practise, or else on another skill of the
//     student's paths;
//   - in the head, the session covers all the student's instruments and
//     offers no play-along, so it has no warm-up and no ending;
//   - the focus time goes 60% to due items, most overdue first, 25% to weak
//     ones and at most 15% to new ones, taken in turn from each of the
//     student's instruments, from the skills of the student's paths. Due
//     and weak items take over each other's unused time;
//   - the time still left is shared half and half between reviewing known
//     items coming due within a week, soonest first, and stretching to
//     unseen items of nodes the student is ready to start and that connect
//     to what they are learning, their paths' skills first and taken in
//     turn from each instrument, each taking over the other's half when it
//     runs out.
//
// Each pick is fitted to the minutes by its estimated time; a session too
// short for any of them still offers the first. A session with nothing to
// offer is not found, as is an instrumentID that doesn't exist.
func (s *PracticeSessionService) ComposePlan(ctx context.Context, caller domain.User, instrumentID *string, minutes int) (domain.PracticeSessionPlan, error) {
	if err := domain.ValidatePracticeMinutes(minutes); err != nil {
		return domain.PracticeSessionPlan{}, err
	}
	instrumentIDs, err := s.sessionInstruments(ctx, caller.ID, instrumentID)
	if err != nil {
		return domain.PracticeSessionPlan{}, err
	}
	inHand := instrumentID != nil
	skillIDs, err := s.learning.pathSkillIDs(ctx, caller.ID)
	if err != nil {
		return domain.PracticeSessionPlan{}, err
	}
	maps, onPath, err := s.sessionCandidates(ctx, caller.ID, instrumentIDs, skillIDs, inHand)
	if err != nil {
		return domain.PracticeSessionPlan{}, err
	}

	c := newComposer(s.now(), minutes)
	if inHand && minutes >= warmUpMinMinutes {
		c.warmUp(onPath)
	}
	p := c.split(onPath)
	if inHand && minutes >= applicationMinMinutes {
		c.application(onPath, p)
	}
	c.focus(p)
	if c.remaining() > 0 {
		var stretch []practiceCandidate
		for i, id := range instrumentIDs {
			found, err := s.stretchCandidates(ctx, maps[i], id, inHand, skillIDs, onPath, c.picked, c.remaining())
			if err != nil {
				return domain.PracticeSessionPlan{}, err
			}
			stretch = append(stretch, found...)
		}
		c.catchUp(p.known, takeInTurn(stretch))
	}
	c.neverEmpty()
	return s.newPlan(ctx, caller.ID, instrumentID, minutes, c.plan())
}

// newPlan is the plan of the composed items, with its felt questions and
// asking for a tap check when one is due. A plan with no item is not found.
func (s *PracticeSessionService) newPlan(ctx context.Context, studentID string, instrumentID *string, minutes int, items []domain.PracticeSessionItem) (domain.PracticeSessionPlan, error) {
	if len(items) == 0 {
		return domain.PracticeSessionPlan{}, fmt.Errorf("%w: nothing to practise for this session", domain.ErrNotFound)
	}
	feltQuestions, err := s.feltQuestions(ctx, items)
	if err != nil {
		return domain.PracticeSessionPlan{}, err
	}
	tapCheckDue, err := s.tapCheckDue(ctx, studentID, items)
	if err != nil {
		return domain.PracticeSessionPlan{}, err
	}
	return domain.PracticeSessionPlan{
		ID: s.newID(), InstrumentID: instrumentID, Minutes: minutes, Items: items,
		FeltQuestions: feltQuestions, TapCheckDue: tapCheckDue,
	}, nil
}

// feltQuestions picks the plan's felt questions, reading felt ratings only
// when the plan has a timed drill.
func (s *PracticeSessionService) feltQuestions(ctx context.Context, items []domain.PracticeSessionItem) ([]string, error) {
	templates := domain.PlanDrillTemplates(items)
	if len(templates) == 0 {
		return []string{}, nil
	}
	feltRated, err := s.feltRatings.FeltRatedSessions(ctx, templates)
	if err != nil {
		return nil, err
	}
	return domain.FeltQuestions(items, feltRated), nil
}

// tapCheckDue reports whether the plan of items asks the student for a tap
// check, reading their tap checks only when it has a fretboard cell.
func (s *PracticeSessionService) tapCheckDue(ctx context.Context, studentID string, items []domain.PracticeSessionItem) (bool, error) {
	if !domain.TapCheckDue(items, nil, s.now()) {
		return false, nil
	}
	lastDone, found, err := s.tapChecks.LastTapCheck(ctx, studentID)
	if err != nil {
		return false, err
	}
	if !found {
		return true, nil
	}
	return domain.TapCheckDue(items, &lastDone, s.now()), nil
}

// sessionInstruments lists the instruments a session covers: the one in
// hand, which must exist, or, in the head, every instrument the student
// learns for. A student whose paths are all for every instrument has none,
// and is offered only the items for every instrument, under "".
func (s *PracticeSessionService) sessionInstruments(ctx context.Context, studentID string, instrumentID *string) ([]string, error) {
	if instrumentID != nil {
		if _, err := s.instruments.GetByID(ctx, *instrumentID); err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, fmt.Errorf("%w: no instrument exists with the given instrument_id", domain.ErrNotFound)
			}
			return nil, err
		}
		return []string{*instrumentID}, nil
	}
	ids, err := s.learning.instrumentIDs(ctx, studentID)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []string{""}, nil
	}
	return ids, nil
}

// sessionCandidates rolls the student's knowledge up for each of
// instrumentIDs and lists the path candidates for them all, each once, the
// first instrument it suits claiming it.
func (s *PracticeSessionService) sessionCandidates(ctx context.Context, studentID string, instrumentIDs, skillIDs []string, inHand bool) ([]KnowledgeMap, []practiceCandidate, error) {
	maps := make([]KnowledgeMap, len(instrumentIDs))
	var onPath []practiceCandidate
	seen := map[string]bool{}
	for i, id := range instrumentIDs {
		var err error
		if maps[i], err = s.rollup.Map(ctx, studentID, id); err != nil {
			return nil, nil, err
		}
		found, err := s.pathCandidates(ctx, skillIDs, id, maps[i], inHand)
		if err != nil {
			return nil, nil, err
		}
		for _, c := range found {
			if !seen[c.key()] {
				seen[c.key()] = true
				onPath = append(onPath, c)
			}
		}
	}
	return maps, onPath, nil
}

// pathCandidates lists, on each of the skills of the student's paths in
// path order, the play-alongs for instrumentID when it is in hand, then
// its exercises, then its fretboard cells from knowledge, each with the
// student's state from knowledge.
func (s *PracticeSessionService) pathCandidates(ctx context.Context, skillIDs []string, instrumentID string, knowledge KnowledgeMap, inHand bool) ([]practiceCandidate, error) {
	seen := map[string]bool{}
	var candidates []practiceCandidate
	for _, skillID := range skillIDs {
		found, err := s.skillCandidates(ctx, skillID, instrumentID, inHand)
		if err != nil {
			return nil, err
		}
		found = append(found, skillCells(knowledge, skillID)...)
		for _, c := range found {
			if seen[c.key()] {
				continue
			}
			seen[c.key()] = true
			c.instrumentID = instrumentID
			if state, ok := knowledge.States[c.key()]; ok {
				c.state = &state
			}
			candidates = append(candidates, c)
		}
	}
	return candidates, nil
}

// skillCandidates lists the play-alongs for instrumentID when it is in
// hand, then the exercises for it, classified under skillID, each offered
// for it. With no instrumentID, only the exercises for every instrument.
func (s *PracticeSessionService) skillCandidates(ctx context.Context, skillID, instrumentID string, inHand bool) ([]practiceCandidate, error) {
	var candidates []practiceCandidate
	if inHand {
		diagrams, err := s.listDiagrams(ctx, domain.DiagramListFilter{SkillID: skillID, InstrumentID: instrumentID, Kind: domain.DiagramKindBasic})
		if err != nil {
			return nil, err
		}
		for _, d := range diagrams {
			if playable(d) {
				candidates = append(candidates, practiceCandidate{diagram: &d, nodeID: &skillID})
			}
		}
	}
	filter := domain.ExerciseFilter{SkillID: skillID}
	if instrumentID != "" {
		filter.InstrumentIDs = []string{instrumentID}
	}
	exercises, err := s.listExercises(ctx, filter)
	if err != nil {
		return nil, err
	}
	for _, e := range exercises {
		if instrumentID == "" && len(e.InstrumentIDs) > 0 {
			continue
		}
		candidates = append(candidates, practiceCandidate{exercise: &e, nodeID: &skillID})
	}
	return candidates, nil
}

// skillCells lists the fretboard cells among knowledge's items classified
// under skillID, each offered for it.
func skillCells(knowledge KnowledgeMap, skillID string) []practiceCandidate {
	var cells []practiceCandidate
	for _, item := range knowledge.Items {
		if !slices.Contains(item.NodeIDs, skillID) {
			continue
		}
		if cell, ok := domain.ParseFretboardCellItemKey(item.ItemKey); ok {
			cells = append(cells, practiceCandidate{cell: &cell, nodeID: &skillID})
		}
	}
	return cells
}

// stretchCandidates lists unseen items for instrumentID not yet picked,
// about seconds of them, from the nodes the student is ready to start in
// the order to start them, the skills of their paths, pathSkillIDs, first.
// Each is offered for the node it starts. Play-alongs are offered only with
// the instrument in hand.
func (s *PracticeSessionService) stretchCandidates(ctx context.Context, knowledge KnowledgeMap, instrumentID string, inHand bool, pathSkillIDs []string, onPath []practiceCandidate, picked map[string]bool, seconds int) ([]practiceCandidate, error) {
	loaded := make(map[string]practiceCandidate, len(onPath))
	for _, c := range onPath {
		loaded[c.key()] = c
	}
	var stretch []practiceCandidate
	found := 0
	for _, pick := range stretchOrder(knowledge, pathSkillIDs, picked) {
		if found >= seconds {
			break
		}
		c, ok, err := s.loadedOr(ctx, loaded, pick.itemKey)
		if err != nil {
			return nil, err
		}
		if ok && (inHand || c.diagram == nil) {
			c.nodeID = &pick.nodeID
			c.instrumentID = instrumentID
			stretch = append(stretch, c)
			found += c.seconds()
		}
	}
	return stretch, nil
}

// stretchPick is an unseen item and the node it would be a stretch to.
type stretchPick struct {
	nodeID  string
	itemKey string
}

// stretchOrder lists the unseen items not in picked, each once, in the
// order to stretch to them: node by node, ranked with pathSkillIDs first.
func stretchOrder(knowledge KnowledgeMap, pathSkillIDs []string, picked map[string]bool) []stretchPick {
	skip := maps.Clone(picked)
	var order []stretchPick
	for _, nodeID := range domain.RankStretchNodes(knowledge.Nodes, knowledge.Standings, pathSkillIDs, knowledge.Applies) {
		for _, key := range knowledge.Subtrees[nodeID] {
			if state, ok := knowledge.States[key]; skip[key] || ok && state.Counted > 0 {
				continue
			}
			skip[key] = true
			order = append(order, stretchPick{nodeID: nodeID, itemKey: key})
		}
	}
	return order
}

// loadedOr is the candidate for itemKey in loaded, or else loads it.
func (s *PracticeSessionService) loadedOr(ctx context.Context, loaded map[string]practiceCandidate, itemKey string) (practiceCandidate, bool, error) {
	if c, ok := loaded[itemKey]; ok {
		return c, true, nil
	}
	return s.candidate(ctx, itemKey)
}

// candidate loads the play-along, exercise or fretboard cell itemKey
// names. It reports false for any other kind of item, and for one that no
// longer exists or can't be played along with.
func (s *PracticeSessionService) candidate(ctx context.Context, itemKey string) (practiceCandidate, bool, error) {
	if cell, ok := domain.ParseFretboardCellItemKey(itemKey); ok {
		return practiceCandidate{cell: &cell}, true, nil
	}
	if id, ok := strings.CutPrefix(itemKey, string(domain.PracticeItemKindPlayAlong)+":"); ok {
		d, err := s.diagrams.GetByID(ctx, id)
		if errors.Is(err, domain.ErrNotFound) {
			return practiceCandidate{}, false, nil
		}
		if err != nil {
			return practiceCandidate{}, false, err
		}
		return practiceCandidate{diagram: &d}, d.Kind == domain.DiagramKindBasic && playable(d), nil
	}
	if id, ok := strings.CutPrefix(itemKey, string(domain.PracticeItemKindExercise)+":"); ok {
		e, err := s.exercises.GetByID(ctx, id)
		if errors.Is(err, domain.ErrNotFound) {
			return practiceCandidate{}, false, nil
		}
		if err != nil {
			return practiceCandidate{}, false, err
		}
		return practiceCandidate{exercise: &e}, true, nil
	}
	return practiceCandidate{}, false, nil
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

func (s *PracticeSessionService) listExercises(ctx context.Context, filter domain.ExerciseFilter) ([]domain.Exercise, error) {
	page := domain.PageRequest{Limit: domain.MaxPageLimit}
	var all []domain.Exercise
	for {
		got, err := s.exercises.List(ctx, filter, page)
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

// playable reports whether d can be played along with: it has a sequence
// and a tempo.
func playable(d domain.Diagram) bool {
	return len(d.Sequence) > 0 && d.TempoBPM != nil
}

func bestClean(c practiceCandidate) *int {
	if c.state == nil {
		return nil
	}
	return c.state.BestCleanBPM
}

// startTempo is the tempo a play-along candidate's ladder starts at.
func startTempo(c practiceCandidate) int {
	return domain.PlayAlongStartTempo(*c.diagram.TempoBPM, bestClean(c))
}

// seconds estimates how long c takes as a pick, a play-along on its tempo
// ladder.
func (c practiceCandidate) seconds() int {
	switch {
	case c.diagram != nil:
		return domain.PlayAlongSeconds(*c.diagram, startTempo(c), false)
	case c.cell != nil:
		return domain.FretboardCellSeconds
	default:
		return domain.ExerciseSeconds(*c.exercise)
	}
}

// reasonOrder is the order a session's picks are offered in, between its
// warm-up and its application ending.
var reasonOrder = []domain.PracticePickReason{
	domain.PracticePickDue, domain.PracticePickWeak, domain.PracticePickNew,
	domain.PracticePickReviewAhead, domain.PracticePickStretch,
}

// composer builds a session's items within its time budget, offering each
// item at most once.
type composer struct {
	now    time.Time
	budget int
	used   int
	picked map[string]bool
	warm   *domain.PracticeSessionItem
	items  []domain.PracticeSessionItem
	ending *domain.PracticeSessionItem
	// first is the first pick refused for lack of time, offered anyway when
	// nothing else fits.
	first *domain.PracticeSessionItem
}

func newComposer(now time.Time, minutes int) *composer {
	return &composer{now: now, budget: minutes * 60, picked: map[string]bool{}}
}

func (c *composer) remaining() int { return c.budget - c.used }

// pools sorts a student's path items by what they need: due ones, most
// overdue first; weak ones and new ones in path order; and known ones
// coming due within a week, weak ones among them, soonest due first.
type pools struct {
	due, weak, fresh, known []practiceCandidate
}

func (c *composer) split(candidates []practiceCandidate) pools {
	var p pools
	for _, cand := range candidates {
		switch {
		case c.picked[cand.key()]:
		case !cand.practised():
			p.fresh = append(p.fresh, cand)
		case cand.state.Due(c.now):
			p.due = append(p.due, cand)
		default:
			if cand.state.Weak(c.now) {
				p.weak = append(p.weak, cand)
			}
			if cand.state.DueAt == nil || !cand.state.DueAt.After(c.now.AddDate(0, 0, reviewAheadDays)) {
				p.known = append(p.known, cand)
			}
		}
	}
	byDueAt := func(a, b practiceCandidate) int { return compareDueAt(a.state.DueAt, b.state.DueAt) }
	slices.SortStableFunc(p.due, byDueAt)
	slices.SortStableFunc(p.known, byDueAt)
	p.fresh = takeInTurn(p.fresh)
	return p
}

// takeInTurn reorders candidates one instrument at a time, in the order
// the instruments first appear, keeping each instrument's own order, so
// whatever share of them fits is balanced across the student's
// instruments, and no instrument fills a session before the next is
// reached.
func takeInTurn(candidates []practiceCandidate) []practiceCandidate {
	var order []string
	byInstrument := map[string][]practiceCandidate{}
	for _, c := range candidates {
		if _, ok := byInstrument[c.instrumentID]; !ok {
			order = append(order, c.instrumentID)
		}
		byInstrument[c.instrumentID] = append(byInstrument[c.instrumentID], c)
	}
	turns := make([]practiceCandidate, 0, len(candidates))
	for len(turns) < len(candidates) {
		for _, id := range order {
			if queue := byInstrument[id]; len(queue) > 0 {
				turns = append(turns, queue[0])
				byInstrument[id] = queue[1:]
			}
		}
	}
	return turns
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
// A due play-along is never the warm-up: its review belongs at the student's
// edge, not at a warm-up's easier tempo.
func (c *composer) warmUp(candidates []practiceCandidate) {
	var best *practiceCandidate
	for i, cand := range candidates {
		if cand.diagram == nil || cand.state == nil || cand.state.BestCleanBPM == nil || cand.state.Due(c.now) {
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
	item := c.item(*best, domain.PracticePickWarmUp, tempo, domain.PlayAlongSeconds(*best.diagram, tempo, true))
	if float64(item.EstimatedSeconds) > float64(c.budget)*warmUpMaxShare {
		return
	}
	c.reserve(item)
	c.warm = &item
}

// betterWarmUp prefers the higher shown level, then the more recently
// played.
func betterWarmUp(a, b practiceCandidate, now time.Time) bool {
	la, lb := a.state.ShownLevel(now).Rank(), b.state.ShownLevel(now).Rank()
	if la != lb {
		return la > lb
	}
	return compareDueAt(a.state.LastAt, b.state.LastAt) > 0
}

// application reserves the session's ending: one play-along applying a
// skill of the first focus item that has one, or else of the student's
// paths, if it takes no more than a quarter of the session. Like the
// warm-up, a due play-along is never the ending: its review belongs in the
// focus block.
func (c *composer) application(onPath []practiceCandidate, p pools) {
	var pick *practiceCandidate
	for _, pool := range [][]practiceCandidate{p.due, p.weak, p.fresh} {
		for _, focus := range pool {
			if pick = c.endingFor(onPath, &focus); pick != nil {
				break
			}
		}
		if pick != nil {
			break
		}
	}
	if pick == nil {
		pick = c.endingFor(onPath, nil)
	}
	if pick == nil {
		return
	}
	item := c.pick(*pick, domain.PracticePickApplication)
	if float64(item.EstimatedSeconds) > float64(c.budget)*applicationMaxShare {
		return
	}
	c.reserve(item)
	c.ending = &item
}

// endingFor finds a play-along on the student's paths, not yet picked and
// not due, that applies focus's skill and isn't focus itself; with no
// focus, any such play-along.
func (c *composer) endingFor(onPath []practiceCandidate, focus *practiceCandidate) *practiceCandidate {
	for i, cand := range onPath {
		if cand.diagram == nil || c.picked[cand.key()] || cand.state != nil && cand.state.Due(c.now) {
			continue
		}
		if focus != nil && (cand.key() == focus.key() || focus.nodeID == nil || !slices.Contains(cand.skillIDs(), *focus.nodeID)) {
			continue
		}
		return &onPath[i]
	}
	return nil
}

// focus shares what is left of the session between due, weak and new
// items. Due and weak items take over each other's unused share; new items
// never pass theirs.
func (c *composer) focus(p pools) {
	focus := c.remaining()
	dueEnd := c.used + focus*dueSharePercent/100
	weakEnd := dueEnd + focus*weakSharePercent/100
	c.fill(p.due, domain.PracticePickDue, dueEnd)
	c.fill(p.weak, domain.PracticePickWeak, weakEnd)
	c.fill(p.due, domain.PracticePickDue, weakEnd)
	c.fill(p.fresh, domain.PracticePickNew, c.used+focus*newSharePercent/100)
}

// catchUp splits what is left of the session between reviewing known items
// ahead and stretching; each takes over the other's half when it runs out.
func (c *composer) catchUp(known, stretch []practiceCandidate) {
	half := c.remaining() / 2
	c.fill(known, domain.PracticePickReviewAhead, c.used+half)
	c.fill(stretch, domain.PracticePickStretch, c.budget)
	c.fill(known, domain.PracticePickReviewAhead, c.budget)
}

// fill adds candidates with reason, in order, while they fit until limit
// seconds are used. The first one refused for lack of time is remembered
// for neverEmpty.
func (c *composer) fill(candidates []practiceCandidate, reason domain.PracticePickReason, limit int) {
	for _, cand := range candidates {
		if c.picked[cand.key()] {
			continue
		}
		item := c.pick(cand, reason)
		if c.used+item.EstimatedSeconds > limit {
			if c.first == nil {
				c.first = &item
			}
			continue
		}
		c.reserve(item)
		c.items = append(c.items, item)
	}
}

// neverEmpty offers the first pick refused for lack of time when nothing
// else fitted. It runs once every fill is done, so a pick too long for its
// own share can't take time a later fill would have used.
func (c *composer) neverEmpty() {
	if c.warm == nil && len(c.items) == 0 && c.ending == nil && c.first != nil {
		c.reserve(*c.first)
		c.items = append(c.items, *c.first)
	}
}

func (c *composer) reserve(item domain.PracticeSessionItem) {
	c.picked[item.ItemKey] = true
	c.used += item.EstimatedSeconds
}

// plan lists the session's items in order: the warm-up, the picks by
// reason, then the application ending.
func (c *composer) plan() []domain.PracticeSessionItem {
	var items []domain.PracticeSessionItem
	if c.warm != nil {
		items = append(items, *c.warm)
	}
	picks := slices.Clone(c.items)
	slices.SortStableFunc(picks, func(a, b domain.PracticeSessionItem) int {
		return cmp.Compare(slices.Index(reasonOrder, a.Reason), slices.Index(reasonOrder, b.Reason))
	})
	items = append(items, picks...)
	if c.ending != nil {
		items = append(items, *c.ending)
	}
	return items
}

// pick is cand picked for reason: a play-along on its tempo ladder, an
// exercise or a fretboard cell.
func (c *composer) pick(cand practiceCandidate, reason domain.PracticePickReason) domain.PracticeSessionItem {
	if cand.diagram == nil {
		return c.item(cand, reason, 0, cand.seconds())
	}
	tempo := startTempo(cand)
	return c.item(cand, reason, tempo, domain.PlayAlongSeconds(*cand.diagram, tempo, false))
}

// item is cand as a session item; tempo applies to a play-along only.
func (c *composer) item(cand practiceCandidate, reason domain.PracticePickReason, tempo, seconds int) domain.PracticeSessionItem {
	level := domain.KnowledgeLevelNew
	if cand.state != nil {
		level = cand.state.ShownLevel(c.now)
	}
	item := domain.PracticeSessionItem{
		ItemKey:          cand.key(),
		Kind:             domain.PracticeItemKindExercise,
		Reason:           reason,
		NodeID:           cand.nodeID,
		Level:            level,
		EstimatedSeconds: seconds,
		Exercise:         cand.exercise,
	}
	if cand.cell != nil {
		item.Kind = domain.PracticeItemKindFretboardCell
		item.FretboardCell = &domain.PlannedFretboardCell{FretboardCell: *cand.cell, Drill: domain.NextFretboardDrill(cand.state)}
	}
	if cand.diagram != nil {
		item.Kind = domain.PracticeItemKindPlayAlong
		item.PlayAlong = &domain.PlannedPlayAlong{
			DiagramID:         cand.diagram.ID,
			StartTempoBPM:     tempo,
			TargetTempoBPM:    *cand.diagram.TempoBPM,
			BestCleanTempoBPM: bestClean(cand),
		}
	}
	return item
}
