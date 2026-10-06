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
// knows. It offers play-alongs, basic diagrams with playback, and authored
// exercises, for the instrument in hand. A teacher's custom diagrams are
// theirs alone to find, so they are never offered.
type PracticeSessionService struct {
	instruments  ports.InstrumentRepository
	studentPaths ports.StudentPathRepository
	enrollments  ports.CourseEnrollmentRepository
	contentNodes ports.ContentNodeRepository
	diagrams     ports.DiagramRepository
	exercises    ports.ExerciseRepository
	rollup       *KnowledgeRollupService
	newID        func() string
	now          func() time.Time
}

func NewPracticeSessionService(
	instruments ports.InstrumentRepository,
	studentPaths ports.StudentPathRepository,
	enrollments ports.CourseEnrollmentRepository,
	contentNodes ports.ContentNodeRepository,
	diagrams ports.DiagramRepository,
	exercises ports.ExerciseRepository,
	rollup *KnowledgeRollupService,
	newID func() string,
	now func() time.Time,
) *PracticeSessionService {
	return &PracticeSessionService{
		instruments:  instruments,
		studentPaths: studentPaths,
		enrollments:  enrollments,
		contentNodes: contentNodes,
		diagrams:     diagrams,
		exercises:    exercises,
		rollup:       rollup,
		newID:        newID,
		now:          now,
	}
}

func (s *PracticeSessionService) learning() studentLearning {
	return studentLearning{studentPaths: s.studentPaths, enrollments: s.enrollments, contentNodes: s.contentNodes}
}

// practiceCandidate is an item that can be practised in a session, either
// a play-along or an exercise, with the node it is offered for and the
// student's state on it, if any.
type practiceCandidate struct {
	diagram  *domain.Diagram
	exercise *domain.Exercise
	nodeID   *string
	state    *domain.PracticeItemState
}

func (c practiceCandidate) key() string {
	if c.diagram != nil {
		return domain.PlayAlongItemKey(c.diagram.ID)
	}
	return domain.ExerciseItemKey(c.exercise.ID)
}

// practised reports whether the student has a counted answer on c.
func (c practiceCandidate) practised() bool {
	return c.state != nil && c.state.Counted > 0
}

// skillIDs lists the skills c is classified under.
func (c practiceCandidate) skillIDs() []string {
	if c.diagram != nil {
		return c.diagram.SkillIDs()
	}
	return knowledgeNodeIDs(c.exercise.Skills)
}

func knowledgeNodeIDs(nodes []domain.KnowledgeNode) []string {
	ids := make([]string, len(nodes))
	for i, n := range nodes {
		ids[i] = n.ID
	}
	return ids
}

// ComposePlan composes a session of minutes for caller with instrumentID
// in hand:
//   - a session of 5 minutes or more starts with a warm-up on a play-along
//     already played clean;
//   - a session of 10 minutes or more ends by applying a skill to music: a
//     play-along on a skill the focus items practise, or else on another
//     skill of the student's paths;
//   - the focus time between them goes 60% to due items, most overdue
//     first, 25% to weak ones and at most 15% to new ones, from the skills
//     of the student's paths. Due and weak items take over each other's
//     unused time;
//   - the time still left is shared half and half between reviewing known
//     items coming due within a week, soonest first, and stretching to
//     unseen items of nodes the student is ready to start and that connect
//     to what they are learning, their paths' skills first, each taking
//     over the other's half when it runs out.
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

	knowledge, err := s.rollup.Map(ctx, caller.ID, *instrumentID)
	if err != nil {
		return domain.PracticeSessionPlan{}, err
	}
	skillIDs, err := s.learning().pathSkillIDs(ctx, caller.ID)
	if err != nil {
		return domain.PracticeSessionPlan{}, err
	}
	onPath, err := s.pathCandidates(ctx, skillIDs, *instrumentID, knowledge.States)
	if err != nil {
		return domain.PracticeSessionPlan{}, err
	}

	c := newComposer(s.now(), minutes)
	if minutes >= warmUpMinMinutes {
		c.warmUp(onPath)
	}
	p := c.split(onPath)
	if minutes >= applicationMinMinutes {
		c.application(onPath, p)
	}
	c.focus(p)
	if c.remaining() > 0 {
		stretch, err := s.stretchCandidates(ctx, knowledge, skillIDs, onPath, c.picked, c.remaining())
		if err != nil {
			return domain.PracticeSessionPlan{}, err
		}
		c.catchUp(p.known, stretch)
	}
	c.neverEmpty()
	items := c.plan()
	if len(items) == 0 {
		return domain.PracticeSessionPlan{}, fmt.Errorf("%w: nothing to practise for this instrument", domain.ErrNotFound)
	}

	return domain.PracticeSessionPlan{ID: s.newID(), InstrumentID: instrumentID, Minutes: minutes, Items: items}, nil
}

// pathCandidates lists the play-alongs, then the exercises, for
// instrumentID on each of the skills of the student's paths, in path
// order, each with the student's state from states.
func (s *PracticeSessionService) pathCandidates(ctx context.Context, skillIDs []string, instrumentID string, states map[string]domain.PracticeItemState) ([]practiceCandidate, error) {
	seen := map[string]bool{}
	var candidates []practiceCandidate
	for _, skillID := range skillIDs {
		found, err := s.skillCandidates(ctx, skillID, instrumentID)
		if err != nil {
			return nil, err
		}
		for _, c := range found {
			if seen[c.key()] {
				continue
			}
			seen[c.key()] = true
			if state, ok := states[c.key()]; ok {
				c.state = &state
			}
			candidates = append(candidates, c)
		}
	}
	return candidates, nil
}

// skillCandidates lists the play-alongs, then the exercises, for
// instrumentID classified under skillID, each offered for it.
func (s *PracticeSessionService) skillCandidates(ctx context.Context, skillID, instrumentID string) ([]practiceCandidate, error) {
	diagrams, err := s.listDiagrams(ctx, domain.DiagramListFilter{SkillID: skillID, InstrumentID: instrumentID, Kind: domain.DiagramKindBasic})
	if err != nil {
		return nil, err
	}
	exercises, err := s.listExercises(ctx, domain.ExerciseFilter{SkillID: skillID, InstrumentIDs: []string{instrumentID}})
	if err != nil {
		return nil, err
	}
	var candidates []practiceCandidate
	for _, d := range diagrams {
		if playable(d) {
			candidates = append(candidates, practiceCandidate{diagram: &d, nodeID: &skillID})
		}
	}
	for _, e := range exercises {
		candidates = append(candidates, practiceCandidate{exercise: &e, nodeID: &skillID})
	}
	return candidates, nil
}

// stretchCandidates lists unseen items not yet picked, about seconds of
// them, from the nodes the student is ready to start in the order to start
// them, the skills of their paths, pathSkillIDs, first. Each is offered for
// the node it starts.
func (s *PracticeSessionService) stretchCandidates(ctx context.Context, knowledge KnowledgeMap, pathSkillIDs []string, onPath []practiceCandidate, picked map[string]bool, seconds int) ([]practiceCandidate, error) {
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
		if ok {
			c.nodeID = &pick.nodeID
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

// candidate loads the play-along or exercise itemKey names. It reports
// false for any other kind of item, and for one that no longer exists or
// can't be played along with.
func (s *PracticeSessionService) candidate(ctx context.Context, itemKey string) (practiceCandidate, bool, error) {
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

// seconds estimates how long c takes as a pick on the tempo ladder.
func (c practiceCandidate) seconds() int {
	if c.diagram != nil {
		return domain.PlayAlongSeconds(*c.diagram, startTempo(c), false)
	}
	return domain.ExerciseSeconds(*c.exercise)
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
	return p
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

// pick is cand picked for reason: a play-along on its tempo ladder, or an
// exercise.
func (c *composer) pick(cand practiceCandidate, reason domain.PracticePickReason) domain.PracticeSessionItem {
	if cand.diagram == nil {
		return c.item(cand, reason, 0, domain.ExerciseSeconds(*cand.exercise))
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
