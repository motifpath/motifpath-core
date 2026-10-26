package application

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"math/rand/v2"
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
// exercises, fretboard cells and catalog diagram shapes: for the instrument
// in hand, or, in the head, everything but play-alongs for all the
// student's instruments. A
// teacher's custom diagrams are theirs alone to find, so they are never
// offered.
type PracticeSessionService struct {
	instruments ports.InstrumentRepository
	learning    studentLearning
	diagrams    ports.DiagramRepository
	// composingDiagrams is diagrams keeping what it reads, while composing.
	composingDiagrams *composingDiagrams
	// composingExercises is exercises keeping what it reads, while composing.
	composingExercises *composingExercises
	exercises          ports.ExerciseRepository
	rollup             *KnowledgeRollupService
	templates          ports.DrillTemplateRepository
	tapChecks          ports.TapCheckReader
	feltRatings        ports.FeltRatingReader
	headCount          ports.HeadSessionCounter
	// sources find the generated drill items, a source per drill family.
	sources []DrillSource
	// harmony is what the harmony drills read; nil when they aren't
	// installed.
	harmony *harmonyDrills
	// lastInHand tells the items of a student's last in-hand session; nil
	// when no songs are installed.
	lastInHand ports.LastInHandSessionReader
	newID      func() string
	now        func() time.Time
	// shuffle puts equally urgent candidates in random order.
	shuffle func(n int, swap func(i, j int))
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
	templates ports.DrillTemplateRepository,
	tapChecks ports.TapCheckReader,
	feltRatings ports.FeltRatingReader,
	headCount ports.HeadSessionCounter,
	newID func() string,
	now func() time.Time,
) *PracticeSessionService {
	s := &PracticeSessionService{
		instruments: instruments,
		learning: studentLearning{
			studentPaths: studentPaths, enrollments: enrollments, contentNodes: contentNodes,
			learningPaths: learningPaths, courseVersions: courseVersions, instruments: instruments,
		},
		diagrams:    diagrams,
		exercises:   exercises,
		rollup:      rollup,
		templates:   templates,
		tapChecks:   tapChecks,
		feltRatings: feltRatings,
		headCount:   headCount,
		newID:       newID,
		now:         now,
		shuffle:     rand.Shuffle,
	}
	s.sources = []DrillSource{diagramShapeSource{service: s}, fretboardCellSource{}}
	return s
}

// WithDrillSources adds sources of further generated drill families, whose
// items are offered after those of the families already found.
func (s *PracticeSessionService) WithDrillSources(sources ...DrillSource) *PracticeSessionService {
	s.sources = append(s.sources, sources...)
	return s
}

// composingSource is a drill source that keeps what it reads for one
// composition, read afresh for the next.
type composingSource interface {
	DrillSource
	composing(s *PracticeSessionService) DrillSource
}

// forComposition is s keeping what it reads, with a fresh copy of each
// source that does, for one composition or one list of subjects.
func (s *PracticeSessionService) forComposition() *PracticeSessionService {
	composing := *s
	composing.composingDiagrams = newComposingDiagrams(s.diagrams)
	composing.diagrams = composing.composingDiagrams
	composing.composingExercises = newComposingExercises(s.exercises)
	composing.exercises = composing.composingExercises
	if s.harmony != nil {
		composing.harmony = s.harmony.forComposition()
	}
	composing.sources = make([]DrillSource, len(s.sources))
	for i, src := range s.sources {
		if c, ok := src.(composingSource); ok {
			src = c.composing(&composing)
		}
		composing.sources[i] = src
	}
	return &composing
}

// WithShuffle replaces how equally urgent candidates are put in random
// order, so a test can tell what is picked without chance.
func (s *PracticeSessionService) WithShuffle(shuffle func(n int, swap func(i, j int))) *PracticeSessionService {
	s.shuffle = shuffle
	return s
}

// practiceCandidate is an item that can be practised in a session, a
// play-along, an exercise or a generated drill item, such as a fretboard
// cell or a diagram shape, with the node it is offered for, the student's
// state on it, if any, and the instrument it was found for, which new items
// are balanced across.
type practiceCandidate struct {
	diagram      *domain.Diagram
	exercise     *domain.Exercise
	drill        domain.DrillItem
	nodeID       *string
	state        *domain.PracticeItemState
	instrumentID string
}

func (c practiceCandidate) key() string {
	switch {
	case c.diagram != nil:
		return domain.PlayAlongItemKey(c.diagram.ID)
	case c.drill != nil:
		return c.drill.ItemKey()
	default:
		return domain.ExerciseItemKey(c.exercise.ID)
	}
}

// suitsSession reports whether c belongs in a session with the instrument
// in hand (inHand) or in the head. In hand is for playing: play-alongs and
// exercises. In the head is for recall on the screen: exercises, no
// play-along. A generated drill item says which it suits.
func (c practiceCandidate) suitsSession(inHand bool) bool {
	if c.drill != nil {
		return c.drill.SuitsSession(inHand)
	}
	return inHand || c.diagram == nil
}

// cell is c's fretboard cell; nil when c is no fretboard cell.
func (c practiceCandidate) cell() *domain.FretboardCell {
	if cell, ok := c.drill.(domain.FretboardCell); ok {
		return &cell
	}
	return nil
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
// in hand, or in the head when instrumentID is nil: Today's mix, whose
// subjects the composer picks, or, with focusNodeID, a session on the one
// subject the student chose (see composeChosen).
//
// Today's mix:
//   - with the instrument in hand, the session is for playing: play-alongs
//     and exercises, never a fretboard cell or a diagram shape, which are
//     recalled in the head; a session of 5 minutes or more starts
//     with a warm-up on a play-along already played clean, and one of 10
//     minutes or more ends by applying a subject to music: a play-along on
//     one of them, the main subject first, or no ending;
//   - in the head, the session covers all the student's instruments and
//     is for recall on the screen: fretboard cells, diagram shapes and
//     exercises, no play-along, so it has no warm-up and no ending;
//   - it practises at most domain.MaxFocusNodes subjects, of the skills of
//     the student's paths: see pickSubjects. Due items of other nodes form
//     a review block, right after the warm-up, of at most
//     reviewMaxItems items and a fifth of the focus time;
//   - the focus time goes 60% to due items, the review block's first, most
//     overdue first, 25% to weak ones and at most 15% to new ones, taken in
//     turn from each of the student's instruments. Due and weak items take
//     over each other's unused time;
//   - the time still left is shared half and half between reviewing known
//     items coming due within a week, soonest first, and stretching to
//     unseen items of nodes the student is ready to start and that connect
//     to what they are learning, their paths' skills first and taken in
//     turn from each instrument, each taking over the other's half when it
//     runs out. Only the subjects' items fill it, so it may end short;
//   - new and stretch items of a subject come from its current rung and the
//     next one: see domain.SubjectRungs;
//   - each fretboard drill of each layout is asked within one window of
//     frets, and only its cells inside that window are picked: see
//     fretWindows.
//
// Each pick is fitted to the minutes by its estimated time; a session too
// short for any of them still offers the first. A session with nothing to
// offer is not found, as is an instrumentID that doesn't exist. Every pick
// carries the rung its drill trains, and the plan names its subjects.
func (s *PracticeSessionService) ComposePlan(ctx context.Context, caller domain.User, instrumentID, focusNodeID *string, minutes int) (domain.PracticeSessionPlan, error) {
	if err := domain.ValidatePracticeMinutes(minutes); err != nil {
		return domain.PracticeSessionPlan{}, err
	}
	s = s.forComposition()
	instrumentIDs, err := s.sessionInstruments(ctx, caller.ID, instrumentID)
	if err != nil {
		return domain.PracticeSessionPlan{}, err
	}
	maps, err := s.knowledgeMaps(ctx, caller.ID, instrumentIDs)
	if err != nil {
		return domain.PracticeSessionPlan{}, err
	}
	templates, err := s.drillTemplates(ctx)
	if err != nil {
		return domain.PracticeSessionPlan{}, err
	}
	c := newComposer(s.now(), minutes, s.shuffle, templates)
	if c.newTurn, err = s.newCellTurn(ctx, caller.ID, instrumentID != nil); err != nil {
		return domain.PracticeSessionPlan{}, err
	}
	if c.lastInHand, err = s.lastInHandItems(ctx, caller.ID, instrumentID != nil); err != nil {
		return domain.PracticeSessionPlan{}, err
	}
	tunings := s.layoutTunings(ctx)
	c.tuningOf = tunings.of
	var boxes *chordBoxes
	if s.harmony != nil {
		// Harmony questions are read in the head only; a tour played in hand
		// still sounds its chords in their voicings.
		if instrumentID == nil {
			if c.reading, c.bassInstrumentID, err = s.harmony.reading(ctx, instrumentIDs, c.newTurn); err != nil {
				return domain.PracticeSessionPlan{}, err
			}
		}
		boxes = s.harmony.boxes(ctx)
		c.voicing = boxes.diagramOf
	}
	var plan domain.PracticeSessionPlan
	if focusNodeID != nil {
		plan, err = s.composeChosen(ctx, c, caller.ID, instrumentID, instrumentIDs, maps, *focusNodeID)
	} else {
		plan, err = s.composeMix(ctx, c, caller.ID, instrumentID, instrumentIDs, maps)
	}
	if err := firstError(err, boxes.failure(), tunings.err); err != nil {
		return domain.PracticeSessionPlan{}, err
	}
	return plan, nil
}

// lastInHandItems are the items of the student's last in-hand session, which an
// in-hand session reads for its song; none in the head or with no songs
// installed.
func (s *PracticeSessionService) lastInHandItems(ctx context.Context, studentID string, inHand bool) ([]string, error) {
	if !inHand || s.lastInHand == nil {
		return nil, nil
	}
	return s.lastInHand.LastInHandSessionItemKeys(ctx, studentID)
}

// firstError is the first of errs that isn't nil: a composition's own error
// before a lookup it deferred.
func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// layoutTuningCache looks up fretboard layouts' tunings once each during one
// composition. A layout that no longer exists has no tuning; any other
// failure is kept in err for the composition to return.
type layoutTuningCache struct {
	ctx     context.Context
	service *PracticeSessionService
	tunings map[string][]string
	err     error
}

func (s *PracticeSessionService) layoutTunings(ctx context.Context) *layoutTuningCache {
	return &layoutTuningCache{ctx: ctx, service: s, tunings: map[string][]string{}}
}

func (l *layoutTuningCache) of(layoutInstrumentID string) []string {
	if tuning, ok := l.tunings[layoutInstrumentID]; ok {
		return tuning
	}
	instrument, err := l.service.instruments.GetByID(l.ctx, layoutInstrumentID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) && l.err == nil {
		l.err = err
	}
	l.tunings[layoutInstrumentID] = instrument.Tuning
	return instrument.Tuning
}

// composeMix composes Today's mix with c: see ComposePlan.
func (s *PracticeSessionService) composeMix(ctx context.Context, c *composer, studentID string, instrumentID *string, instrumentIDs []string, maps []KnowledgeMap) (domain.PracticeSessionPlan, error) {
	inHand := instrumentID != nil
	skillIDs, err := s.learning.pathSkillIDs(ctx, studentID)
	if err != nil {
		return domain.PracticeSessionPlan{}, err
	}
	onPath, err := s.sessionCandidates(ctx, maps, instrumentIDs, skillIDs, inHand)
	if err != nil {
		return domain.PracticeSessionPlan{}, err
	}

	c.setNewRungs(onPath)
	if inHand && c.budget >= warmUpMinMinutes*60 {
		c.warmUp(onPath)
	}
	p := c.split(onPath)
	c.pickSubjects(p)
	p = c.fretWindows(c.subjectPools(p))
	if inHand && c.budget >= applicationMinMinutes*60 {
		c.application(onPath, c.subjects, slices.Concat(p.due, p.weak, p.fresh))
	}
	c.focus(p)
	if c.remaining() > 0 {
		var stretch []practiceCandidate
		for i, id := range instrumentIDs {
			found, err := s.stretchCandidates(ctx, maps[i], id, inHand, skillIDs, onPath, c.picked, c.stretchFits, c.remaining(), s.shuffle)
			if err != nil {
				return domain.PracticeSessionPlan{}, err
			}
			stretch = append(stretch, found...)
		}
		c.catchUp(p.known, c.stretchWindows(c.stretchSubjects(stretch)))
	}
	c.neverEmpty()
	return s.newPlan(ctx, studentID, instrumentID, c, domain.PracticeFocusSuggested, maps)
}

// drillTemplates reads the installed drill templates, by key.
func (s *PracticeSessionService) drillTemplates(ctx context.Context) (map[string]domain.DrillTemplate, error) {
	list, err := s.templates.List(ctx)
	if err != nil {
		return nil, err
	}
	templates := make(map[string]domain.DrillTemplate, len(list))
	for _, t := range list {
		templates[t.Key] = t
	}
	return templates, nil
}

// newCellTurn reports whether a session in the head is a new-cell turn: the
// student has started an odd number of them so far, so every other session
// moves a drill with new cells to where they are. A session with the
// instrument in hand asks no fretboard cell, so it never is one.
func (s *PracticeSessionService) newCellTurn(ctx context.Context, studentID string, inHand bool) (bool, error) {
	if inHand {
		return false, nil
	}
	started, err := s.headCount.HeadSessionsStarted(ctx, studentID)
	if err != nil {
		return false, err
	}
	return started%2 == 1, nil
}

// newPlan is the plan c composed, each item with its rung, naming c's
// subjects from maps, with its felt questions and asking for a tap check
// when one is due. A plan with no item is not found.
func (s *PracticeSessionService) newPlan(ctx context.Context, studentID string, instrumentID *string, c *composer, source domain.PracticeFocusSource, maps []KnowledgeMap) (domain.PracticeSessionPlan, error) {
	items := c.plan()
	if len(items) == 0 {
		return domain.PracticeSessionPlan{}, fmt.Errorf("%w: nothing to practise for this session", domain.ErrNotFound)
	}
	for i := range items {
		items[i].Rung = domain.ItemRung(items[i], c.templates)
	}
	nodes := map[string]domain.KnowledgeNode{}
	for _, m := range maps {
		for _, n := range m.Nodes {
			nodes[n.ID] = n
		}
	}
	feltQuestions, err := s.feltQuestions(ctx, timedItems(items, c.templates))
	if err != nil {
		return domain.PracticeSessionPlan{}, err
	}
	tapCheckDue, err := s.tapCheckDue(ctx, studentID, items)
	if err != nil {
		return domain.PracticeSessionPlan{}, err
	}
	return domain.PracticeSessionPlan{
		ID: s.newID(), InstrumentID: instrumentID, Minutes: c.budget / 60, Items: items,
		FocusSource: source, FocusNodes: c.focusNodes(items, nodes),
		FeltQuestions: feltQuestions, TapCheckDue: tapCheckDue,
	}, nil
}

// timedItems are the items of a plan whose drill template is timed: an
// untimed one, such as finding every instance of a note, has no fluent time
// for a felt rating to calibrate.
func timedItems(items []domain.PracticeSessionItem, templates map[string]domain.DrillTemplate) []domain.PracticeSessionItem {
	var timed []domain.PracticeSessionItem
	for _, item := range items {
		if t, ok := templates[item.DrillTemplateKey()]; ok && !t.Timed {
			continue
		}
		timed = append(timed, item)
	}
	return timed
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

// knowledgeMaps rolls the student's knowledge up for each of
// instrumentIDs.
func (s *PracticeSessionService) knowledgeMaps(ctx context.Context, studentID string, instrumentIDs []string) ([]KnowledgeMap, error) {
	maps := make([]KnowledgeMap, len(instrumentIDs))
	for i, id := range instrumentIDs {
		var err error
		if maps[i], err = s.rollup.Map(ctx, studentID, id); err != nil {
			return nil, err
		}
	}
	return maps, nil
}

// sessionCandidates lists the candidates on skillIDs for each of
// instrumentIDs, whose knowledge is in maps, each once, the first
// instrument it suits claiming it.
func (s *PracticeSessionService) sessionCandidates(ctx context.Context, maps []KnowledgeMap, instrumentIDs, skillIDs []string, inHand bool) ([]practiceCandidate, error) {
	var candidates []practiceCandidate
	seen := map[string]bool{}
	for i, id := range instrumentIDs {
		found, err := s.pathCandidates(ctx, skillIDs, id, maps[i], inHand)
		if err != nil {
			return nil, err
		}
		for _, c := range found {
			if !seen[c.key()] {
				seen[c.key()] = true
				candidates = append(candidates, c)
			}
		}
	}
	return candidates, nil
}

// pathCandidates lists, on each of the skills of the student's paths in
// path order, the play-alongs for instrumentID when it is in hand, then
// its exercises, then its generated drill items, family by family (diagram
// shapes, then fretboard cells from knowledge), each with the student's
// state from knowledge.
func (s *PracticeSessionService) pathCandidates(ctx context.Context, skillIDs []string, instrumentID string, knowledge KnowledgeMap, inHand bool) ([]practiceCandidate, error) {
	seen := map[string]bool{}
	var candidates []practiceCandidate
	for _, skillID := range skillIDs {
		found, err := s.skillCandidates(ctx, skillID, isConcept(knowledge.Nodes, skillID), instrumentID, inHand)
		if err != nil {
			return nil, err
		}
		drills, err := s.drillCandidates(ctx, skillID, instrumentID, inHand, knowledge)
		if err != nil {
			return nil, err
		}
		found = append(found, drills...)
		for _, c := range found {
			if seen[c.key()] || !c.suitsSession(inHand) {
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

// drillCandidates lists the generated drill items classified under skillID,
// family by family, each offered for it: see DrillSource.SkillItems.
func (s *PracticeSessionService) drillCandidates(ctx context.Context, skillID, instrumentID string, inHand bool, knowledge KnowledgeMap) ([]practiceCandidate, error) {
	var candidates []practiceCandidate
	for _, src := range s.sources {
		items, err := src.SkillItems(ctx, skillID, instrumentID, inHand, knowledge)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if readyFor(knowledge, item) {
				candidates = append(candidates, practiceCandidate{drill: item, nodeID: &skillID})
			}
		}
	}
	return candidates, nil
}

// skillCandidates lists the play-alongs for instrumentID when it is in
// hand, then the exercises for it, classified under skillID, a concept
// when concept, each offered for it. With no instrumentID, only the
// exercises for every instrument. Chord catalog voicings play like
// play-alongs but aren't practised on their own, so only general diagrams
// are offered.
func (s *PracticeSessionService) skillCandidates(ctx context.Context, skillID string, concept bool, instrumentID string, inHand bool) ([]practiceCandidate, error) {
	candidates, err := s.playAlongCandidates(ctx, skillID, concept, instrumentID, inHand)
	if err != nil {
		return nil, err
	}
	filter := domain.ExerciseFilter{SkillIDs: []string{skillID}}
	if concept {
		filter = domain.ExerciseFilter{ConceptIDs: []string{skillID}}
	}
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

// playAlongCandidates lists the play-alongs for instrumentID classified
// under skillID, a concept when concept, when it is in hand; none in the
// head.
func (s *PracticeSessionService) playAlongCandidates(ctx context.Context, skillID string, concept bool, instrumentID string, inHand bool) ([]practiceCandidate, error) {
	if !inHand || instrumentID == "" {
		return nil, nil
	}
	filter := domain.DiagramListFilter{SkillIDs: []string{skillID}, InstrumentID: instrumentID, Kind: domain.DiagramKindBasic, Purpose: domain.DiagramPurposeFilterGeneral}
	if concept {
		filter.SkillIDs, filter.ConceptIDs = nil, []string{skillID}
	}
	diagrams, err := s.listDiagrams(ctx, filter)
	if err != nil {
		return nil, err
	}
	var playAlongs []practiceCandidate
	for _, d := range diagrams {
		if playable(d) {
			playAlongs = append(playAlongs, practiceCandidate{diagram: &d, nodeID: &skillID})
		}
	}
	return playAlongs, nil
}

// stretchCandidates lists unseen items for instrumentID not yet picked and
// that fit, about seconds of them, from the nodes the student is ready to
// start in the order to start them, the skills of their paths,
// pathSkillIDs, first.
// Each is offered for the node it starts, if it suits the session: see
// suitsSession.
func (s *PracticeSessionService) stretchCandidates(ctx context.Context, knowledge KnowledgeMap, instrumentID string, inHand bool, pathSkillIDs []string, onPath []practiceCandidate, picked map[string]bool, fits func(practiceCandidate) bool, seconds int, shuffle func(n int, swap func(i, j int))) ([]practiceCandidate, error) {
	loaded := make(map[string]practiceCandidate, len(onPath))
	for _, c := range onPath {
		loaded[c.key()] = c
	}
	var stretch []practiceCandidate
	found := 0
	for _, pick := range stretchOrder(knowledge, pathSkillIDs, picked, shuffle) {
		if found >= seconds {
			break
		}
		c, ok, err := s.loadedOr(ctx, loaded, pick.itemKey)
		if err != nil {
			return nil, err
		}
		c.nodeID = &pick.nodeID
		if ok && c.suitsSession(inHand) && !c.endsSessionOnly() && (c.drill == nil || readyFor(knowledge, c.drill)) && fits(c) {
			c.instrumentID = instrumentID
			stretch = append(stretch, c)
			found += c.seconds()
		}
	}
	return stretch, nil
}

// isConcept reports whether the node id among nodes is a concept, whose
// items are classified apart from skills'.
func isConcept(nodes []domain.KnowledgeNode, id string) bool {
	return slices.ContainsFunc(nodes, func(n domain.KnowledgeNode) bool { return n.ID == id && n.Kind == domain.KnowledgeNodeKindConcept })
}

// stretchPick is an unseen item and the node it would be a stretch to.
type stretchPick struct {
	nodeID  string
	itemKey string
}

// stretchOrder lists the unseen items not in picked, each once, in the
// order to stretch to them: node by node, ranked with pathSkillIDs first,
// and at random within a node.
func stretchOrder(knowledge KnowledgeMap, pathSkillIDs []string, picked map[string]bool, shuffle func(n int, swap func(i, j int))) []stretchPick {
	skip := maps.Clone(picked)
	var order []stretchPick
	for _, nodeID := range domain.RankStretchNodes(knowledge.Nodes, knowledge.Standings, pathSkillIDs, knowledge.Applies) {
		keys := slices.Clone(knowledge.Subtrees[nodeID])
		shuffle(len(keys), func(i, j int) { keys[i], keys[j] = keys[j], keys[i] })
		for _, key := range keys {
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

// diagram loads the diagram id; false when it no longer exists.
func (s *PracticeSessionService) diagram(ctx context.Context, id string) (domain.Diagram, bool, error) {
	d, err := s.diagrams.GetByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Diagram{}, false, nil
	}
	return d, err == nil, err
}

// candidate loads the play-along, exercise or generated drill item itemKey
// names. It reports false for any other kind of item, and for one that no
// longer exists, can't be played along with or is no longer a drill item.
func (s *PracticeSessionService) candidate(ctx context.Context, itemKey string) (practiceCandidate, bool, error) {
	for _, src := range s.sources {
		item, ok, err := src.Item(ctx, itemKey)
		if err != nil || ok {
			return practiceCandidate{drill: item}, ok, err
		}
	}
	if id, ok := strings.CutPrefix(itemKey, string(domain.PracticeItemKindPlayAlong)+":"); ok {
		d, found, err := s.diagram(ctx, id)
		return practiceCandidate{diagram: &d}, found && d.Kind == domain.DiagramKindBasic && playable(d), err
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

// playable reports whether d can be played along with: it has a default
// playback, which always has steps and a tempo.
func playable(d domain.Diagram) bool {
	_, ok := d.DefaultPlayback()
	return ok
}

// targetTempo is the tempo a playable diagram's play-along aims for: its
// default playback's.
func targetTempo(d domain.Diagram) int {
	playback, _ := d.DefaultPlayback()
	return playback.TempoBPM
}

// bestClean is the candidate's best clean tempo as played, nil with no clean
// take, and its note value: quarters without one.
func bestClean(c practiceCandidate) (*int, domain.TempoNoteValue) {
	if c.state == nil {
		return nil, domain.TempoNoteValueQuarters
	}
	return c.state.BestCleanTempo()
}

// startTempo is the tempo a play-along candidate's ladder starts at, in the
// note value of the best clean take, so the student resumes where they left off.
func startTempo(c practiceCandidate) (int, domain.TempoNoteValue) {
	bpm, noteValue := bestClean(c)
	return domain.PlayAlongStartTempo(targetTempo(*c.diagram), bpm), noteValue
}

// seconds estimates how long c takes as a pick, a play-along on its tempo
// ladder.
func (c practiceCandidate) seconds() int {
	switch {
	case c.diagram != nil:
		tempo, noteValue := startTempo(c)
		return domain.PlayAlongSeconds(*c.diagram, tempo, noteValue, false)
	case c.drill != nil:
		if timed, ok := c.drill.(domain.TimedByWay); ok {
			return timed.SecondsAsked(domain.NextDrillWay(c.drill, c.state))
		}
		return c.drill.Seconds()
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
	now     time.Time
	budget  int
	used    int
	shuffle func(n int, swap func(i, j int))
	picked  map[string]bool
	// drills counts the picks of each generated drill, which stop at
	// domain.MaxGeneratedDrillItems.
	drills map[string]int
	warm   *domain.PracticeSessionItem
	items  []domain.PracticeSessionItem
	ending *domain.PracticeSessionItem
	// first is the first pick refused for lack of time, offered anyway when
	// nothing else fits.
	first *domain.PracticeSessionItem
	// windows holds the fret window of each fretboard drill of each layout,
	// and newTurn whether this session is a new-cell turn.
	windows map[drillWindowKey]domain.FretWindow
	newTurn bool
	// reading is how this session's harmony questions draw chords, and
	// voicing finds a chord's box; nil when no box is drawn.
	reading domain.GeneratedChordDrawing
	voicing func(chord string) (string, bool)
	// templates are the installed drill templates by key, which tell the
	// rung of each pick.
	templates map[string]domain.DrillTemplate
	// subjects are the nodes the session practises, main first; subjectOf
	// is the subject a candidate serves, and newRungs the rungs each
	// subject's new items come from.
	subjects  []string
	subjectOf func(practiceCandidate) string
	newRungs  map[string][]domain.PracticeRung
	// review is Today's mix review block: due items of other nodes.
	review []domain.PracticeSessionItem
	// aheadAnyTime lets known items be reviewed ahead however far off they
	// fall due, as a session on one chosen subject does.
	aheadAnyTime bool
	// confusable holds, by item key, the keys of the items each pick is
	// mistaken for.
	confusable map[string][]string
	// bassInstrumentID is the bass this session reads chords on, whose
	// voice plays a root line under a question asked by ear; empty on guitar.
	bassInstrumentID string
	// comebacks holds, by item key, each pick asked a way that comes back,
	// planned again for its second time later in the session.
	comebacks map[string]domain.PracticeSessionItem
	// tuningOf is the tuning of a fretboard layout, which spells a cell's
	// note; nil when nothing is planned. everyInstance holds each layout's
	// notes already asked to find every instance, keyed layout:note.
	tuningOf      func(layoutInstrumentID string) []string
	everyInstance map[string]bool
	// chosenNode is the one subject a session the student chose practises;
	// empty in Today's mix.
	chosenNode string
	// lastInHand are the items of the student's last in-hand session, whose
	// song isn't played again while another fits.
	lastInHand []string
}

// drillWindowKey is a fretboard drill of a layout, which takes one window.
type drillWindowKey struct {
	layoutInstrumentID string
	drill              domain.FretboardDrill
}

// windowKey is the drill and layout cand is asked in; false when it is no
// fretboard cell.
func windowKey(cand practiceCandidate) (drillWindowKey, bool) {
	cell := cand.cell()
	if cell == nil {
		return drillWindowKey{}, false
	}
	return drillWindowKey{layoutInstrumentID: cell.LayoutInstrumentID, drill: domain.FretboardDrill(domain.NextDrillWay(cell, cand.state))}, true
}

func newComposer(now time.Time, minutes int, shuffle func(n int, swap func(i, j int)), templates map[string]domain.DrillTemplate) *composer {
	return &composer{
		now: now, budget: minutes * 60, shuffle: shuffle, picked: map[string]bool{}, drills: map[string]int{},
		windows: map[drillWindowKey]domain.FretWindow{}, templates: templates,
		subjectOf: candidateNode, newRungs: map[string][]domain.PracticeRung{}, confusable: map[string][]string{},
		comebacks: map[string]domain.PracticeSessionItem{}, everyInstance: map[string]bool{},
	}
}

// everyInstanceKey names a cell's note on its layout, for the cells asked to
// find every instance; false when its note can't be read from the layout's
// tuning, so it never stands in for another cell's.
func (c *composer) everyInstanceKey(cell *domain.FretboardCell) (string, bool) {
	if c.tuningOf == nil {
		return "", false
	}
	note, ok := domain.CellNote(c.tuningOf(cell.LayoutInstrumentID), cell.String, cell.Fret)
	return cell.LayoutInstrumentID + ":" + note, ok
}

// wayOf is the way cand is asked: the next one (domain.NextDrillWay), except
// that one finding every instance of a note covers every cell of that note,
// so a second cell of a note already asked that way is asked the way it has
// the next fewest right answers instead. It reports false when that way's
// window doesn't hold the cell, which then isn't picked.
func (c *composer) wayOf(cand practiceCandidate) (string, bool) {
	if cand.drill == nil {
		return "", true
	}
	way := domain.NextDrillWay(cand.drill, cand.state)
	cell := cand.cell()
	if cell == nil || way != string(domain.FretboardDrillFindEveryInstance) {
		return way, true
	}
	if key, ok := c.everyInstanceKey(cell); !ok || !c.everyInstance[key] {
		return way, true
	}
	way = nextWayBut(cell, cand.state, way)
	w, picked := c.windows[drillWindowKey{layoutInstrumentID: cell.LayoutInstrumentID, drill: domain.FretboardDrill(way)}]
	return way, !picked || w.Contains(cell.Fret)
}

// cellWindow is the window cell is asked within through way: its drill's on
// its layout. A drill first met here, as when a cell moves on from finding
// every instance, takes the window that holds the cell.
func (c *composer) cellWindow(cell *domain.FretboardCell, way string) domain.FretWindow {
	k := drillWindowKey{layoutInstrumentID: cell.LayoutInstrumentID, drill: domain.FretboardDrill(way)}
	if w, picked := c.windows[k]; picked {
		return w
	}
	c.windows[k] = domain.PickFretWindow(nil, nil, []int{cell.Fret}, c.newTurn)
	return c.windows[k]
}

// nextWayBut is the way item has the fewest right answers on, the first of
// its ways on a tie, leaving out but.
func nextWayBut(item domain.DrillItem, state *domain.PracticeItemState, but string) string {
	next := ""
	for _, way := range item.Ways() {
		if way == but {
			continue
		}
		if next == "" || (state != nil && state.RightByResponse[way] < state.RightByResponse[next]) {
			next = way
		}
	}
	return next
}

func (c *composer) remaining() int { return c.budget - c.used }

// pools sorts a student's path items by what they need: due ones, most
// overdue day first; weak ones and new ones in path order; and known ones
// coming due within a week, weak ones among them, soonest day first. Items
// equally urgent, due the same day or of the same node, come in random
// order, so a session never walks the generator's order string by string.
type pools struct {
	due, weak, fresh, known []practiceCandidate
	// review holds the due items of nodes other than the subjects.
	review []practiceCandidate
}

func (c *composer) split(candidates []practiceCandidate) pools {
	var p pools
	for _, cand := range candidates {
		switch {
		case c.picked[cand.key()] || cand.endsSessionOnly():
		case !cand.practised():
			if c.atNewRung(cand) {
				p.fresh = append(p.fresh, cand)
			}
		case cand.state.Due(c.now):
			p.due = append(p.due, cand)
		default:
			if cand.state.Weak(c.now) {
				p.weak = append(p.weak, cand)
			}
			if c.aheadAnyTime || cand.state.DueAt == nil || !cand.state.DueAt.After(c.now.AddDate(0, 0, reviewAheadDays)) {
				p.known = append(p.known, cand)
			}
		}
	}
	byDueDay := func(a, b practiceCandidate) int { return compareDueDay(a.state.DueAt, b.state.DueAt) }
	for _, pool := range [][]practiceCandidate{p.due, p.known} {
		c.shuffleCandidates(pool)
		slices.SortStableFunc(pool, byDueDay)
	}
	p.weak = c.shuffleWithinNodes(p.weak)
	p.fresh = takeInTurn(c.shuffleWithinNodes(p.fresh))
	return p
}

// fretWindows picks the window of every fretboard drill of every layout in
// p, each cell's drill being the way it is asked next, and keeps only the
// cells inside their drill's window: see domain.PickFretWindow, fed with
// the frets of the drill's due, weak and new cells. A drill with only cells
// to review ahead takes the window at the nut.
func (c *composer) fretWindows(p pools) pools {
	type tally struct{ due, weak, fresh []int }
	tallies := map[drillWindowKey]*tally{}
	for _, pool := range [][]practiceCandidate{p.review, p.due, p.weak, p.fresh, p.known} {
		for _, cand := range pool {
			if k, ok := windowKey(cand); ok && tallies[k] == nil {
				tallies[k] = &tally{}
			}
		}
	}
	add := func(pool []practiceCandidate, frets func(*tally) *[]int) {
		for _, cand := range pool {
			if k, ok := windowKey(cand); ok {
				f := frets(tallies[k])
				*f = append(*f, cand.cell().Fret)
			}
		}
	}
	add(p.review, func(t *tally) *[]int { return &t.due })
	add(p.due, func(t *tally) *[]int { return &t.due })
	add(p.weak, func(t *tally) *[]int { return &t.weak })
	add(p.fresh, func(t *tally) *[]int { return &t.fresh })
	for k, t := range tallies {
		c.windows[k] = domain.PickFretWindow(t.due, t.weak, t.fresh, c.newTurn)
	}
	// The new cells are taken in turn from each instrument again: those
	// left out would otherwise leave one instrument's turns to the other.
	return pools{review: c.inWindows(p.review), due: c.inWindows(p.due), weak: c.inWindows(p.weak), fresh: takeInTurn(c.inWindows(p.fresh)), known: c.inWindows(p.known)}
}

// stretchWindows picks the window of each fretboard drill first met in
// stretch, from its cells, all new, and keeps only the stretch cells inside
// their drill's window, taken in turn from each instrument again.
func (c *composer) stretchWindows(stretch []practiceCandidate) []practiceCandidate {
	fresh := map[drillWindowKey][]int{}
	for _, cand := range stretch {
		if k, ok := windowKey(cand); ok {
			if _, picked := c.windows[k]; !picked {
				fresh[k] = append(fresh[k], cand.cell().Fret)
			}
		}
	}
	for k, frets := range fresh {
		c.windows[k] = domain.PickFretWindow(nil, nil, frets, c.newTurn)
	}
	return takeInTurn(c.inWindows(stretch))
}

// inWindow reports whether cand is no fretboard cell, a cell of a drill with
// no window yet, or a cell inside its drill's window.
func (c *composer) inWindow(cand practiceCandidate) bool {
	k, ok := windowKey(cand)
	if !ok {
		return true
	}
	w, picked := c.windows[k]
	return !picked || w.Contains(cand.cell().Fret)
}

func (c *composer) inWindows(candidates []practiceCandidate) []practiceCandidate {
	var kept []practiceCandidate
	for _, cand := range candidates {
		if c.inWindow(cand) {
			kept = append(kept, cand)
		}
	}
	return kept
}

func (c *composer) shuffleCandidates(candidates []practiceCandidate) {
	c.shuffle(len(candidates), func(i, j int) { candidates[i], candidates[j] = candidates[j], candidates[i] })
}

// shuffleWithinNodes keeps candidates' nodes in the order they first
// appear, and puts each node's candidates in random order, except that
// items introduced in a set order keep that order among themselves.
func (c *composer) shuffleWithinNodes(candidates []practiceCandidate) []practiceCandidate {
	var order []string
	byNode := map[string][]practiceCandidate{}
	for _, cand := range candidates {
		node := ""
		if cand.nodeID != nil {
			node = *cand.nodeID
		}
		if _, ok := byNode[node]; !ok {
			order = append(order, node)
		}
		byNode[node] = append(byNode[node], cand)
	}
	shuffled := make([]practiceCandidate, 0, len(candidates))
	for _, node := range order {
		group := byNode[node]
		c.shuffleCandidates(group)
		shuffled = append(shuffled, inIntroductionOrder(group)...)
	}
	return shuffled
}

// inIntroductionOrder puts the candidates introduced in a set order back in
// that order, in the places they hold among the others.
func inIntroductionOrder(candidates []practiceCandidate) []practiceCandidate {
	var places []int
	var ranked []practiceCandidate
	for i, cand := range candidates {
		if _, ok := cand.drill.(domain.IntroducedInOrder); ok {
			places = append(places, i)
			ranked = append(ranked, cand)
		}
	}
	slices.SortStableFunc(ranked, func(a, b practiceCandidate) int {
		return a.drill.(domain.IntroducedInOrder).IntroductionRank() - b.drill.(domain.IntroducedInOrder).IntroductionRank()
	})
	for n, i := range places {
		candidates[i] = ranked[n]
	}
	return candidates
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

// compareDueAt orders dates soonest first, no date before any.
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

// compareDueDay orders review dates by their day (UTC), soonest first, no
// date before any. Items answered in one session fall due seconds apart, so
// ordering by the exact time would replay that session's order.
func compareDueDay(a, b *time.Time) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return -1
	case b == nil:
		return 1
	default:
		return dayOf(*a).Compare(dayOf(*b))
	}
}

func dayOf(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
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
	bestBPM, noteValue := best.state.BestCleanTempo()
	tempo := domain.WarmUpTempo(targetTempo(*best.diagram), *bestBPM)
	item := c.item(*best, domain.PracticePickWarmUp, tempo, domain.PlayAlongSeconds(*best.diagram, tempo, noteValue, true))
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

// application reserves the session's ending: a song or a play-along applying
// one of subjects (see subjectEnding); in a chosen session, a song of its
// subject at the use rung, else any play-along of onPath; with no subjects,
// any play-along of onPath; if it takes no more than a quarter of the
// session. Like the warm-up, a due play-along is never the ending: its review
// belongs in the focus block.
func (c *composer) application(onPath []practiceCandidate, subjects []string, focus []practiceCandidate) {
	var pick *practiceCandidate
	switch {
	case c.chosenNode != "":
		if pick = c.songEnding(onPath, "", c.chosenNode); pick == nil {
			pick = c.endingFor(onPath, "")
		}
	case subjects != nil:
		pick = c.subjectEnding(onPath, subjects, focus)
	default:
		pick = c.endingFor(onPath, "")
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

// subjectEnding finds the ending of the first of subjects that has one and
// that another item of focus practises, or else the first that has one: a
// song when the subject is at the use rung (see songEnding), otherwise a
// play-along applying it, as for endingFor. Its node is that subject.
func (c *composer) subjectEnding(onPath []practiceCandidate, subjects []string, focus []practiceCandidate) *practiceCandidate {
	for _, besides := range []bool{true, false} {
		for _, subject := range subjects {
			pick := c.songEnding(onPath, subject, subject)
			if pick == nil {
				pick = c.endingFor(onPath, subject)
			}
			if pick == nil || besides && !practisedBesides(focus, subject, pick.key()) {
				continue
			}
			ending := *pick
			ending.nodeID = &subject
			return &ending
		}
	}
	return nil
}

// practisedBesides reports whether an item of focus other than itemKey is
// offered for subject.
func practisedBesides(focus []practiceCandidate, subject, itemKey string) bool {
	return slices.ContainsFunc(focus, func(cand practiceCandidate) bool {
		return candidateNode(cand) == subject && cand.key() != itemKey
	})
}

// endingFor finds a play-along of onPath, not yet picked and not due, that
// applies the skill subject; with no subject (""), any such play-along.
func (c *composer) endingFor(onPath []practiceCandidate, subject string) *practiceCandidate {
	for i, cand := range onPath {
		if cand.diagram == nil || c.picked[cand.key()] || cand.state != nil && cand.state.Due(c.now) {
			continue
		}
		if subject != "" && !slices.Contains(cand.skillIDs(), subject) {
			continue
		}
		return &onPath[i]
	}
	return nil
}

// focus shares what is left of the session between due, weak and new
// items, the review block taking its due items first. Due and weak items
// take over each other's unused share; new items never pass theirs.
func (c *composer) focus(p pools) {
	focus := c.remaining()
	dueEnd := c.used + focus*dueSharePercent/100
	c.review = c.take(p.review, domain.PracticePickDue, min(c.used+focus/reviewMaxShareDivisor, dueEnd), reviewMaxItems)
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
// seconds are used: see take.
func (c *composer) fill(candidates []practiceCandidate, reason domain.PracticePickReason, limit int) {
	c.items = append(c.items, c.take(candidates, reason, limit, len(candidates))...)
}

// take reserves at most most of candidates with reason, in order, while
// they fit until limit seconds are used, passing over a generated drill
// already asked as often as a session asks one. The first one refused for
// lack of time is remembered for neverEmpty.
func (c *composer) take(candidates []practiceCandidate, reason domain.PracticePickReason, limit, most int) []domain.PracticeSessionItem {
	var taken []domain.PracticeSessionItem
	for _, cand := range candidates {
		if len(taken) >= most {
			break
		}
		if c.picked[cand.key()] {
			continue
		}
		if _, fits := c.wayOf(cand); !fits {
			continue
		}
		item := c.pick(cand, reason)
		if drill, most, capped := item.SessionCap(); capped && c.drills[drill] >= most {
			continue
		}
		if c.used+c.cost(item) > limit {
			if c.first == nil {
				c.first = &item
			}
			continue
		}
		c.reserve(item)
		taken = append(taken, item)
	}
	return taken
}

// neverEmpty offers the first pick refused for lack of time when nothing
// else fitted. It runs once every fill is done, so a pick too long for its
// own share can't take time a later fill would have used.
func (c *composer) neverEmpty() {
	if c.warm == nil && len(c.review) == 0 && len(c.items) == 0 && c.ending == nil && c.first != nil {
		c.reserve(*c.first)
		c.items = append(c.items, *c.first)
	}
}

func (c *composer) reserve(item domain.PracticeSessionItem) {
	if cell := item.FretboardCell; cell != nil && cell.Drill == domain.FretboardDrillFindEveryInstance {
		if key, ok := c.everyInstanceKey(&cell.FretboardCell); ok {
			c.everyInstance[key] = true
		}
	}
	c.picked[item.ItemKey] = true
	c.used += c.cost(item)
	if drill, _, capped := item.SessionCap(); capped {
		c.drills[drill]++
	}
}

// plan lists the session's items in order: the warm-up, the review block,
// the picks, each with their drills taking turns but the main subject's
// confusable items next to each other, then the application ending. An
// item asked a way that comes back is asked again a few items later.
func (c *composer) plan() []domain.PracticeSessionItem {
	var items []domain.PracticeSessionItem
	if c.warm != nil {
		items = append(items, *c.warm)
	}
	items = append(items, takeTurns(c.review)...)
	picks := slices.Clone(c.items)
	slices.SortStableFunc(picks, func(a, b domain.PracticeSessionItem) int {
		return cmp.Compare(slices.Index(reasonOrder, a.Reason), slices.Index(reasonOrder, b.Reason))
	})
	items = append(items, chordsBeforeChanges(c.confusablesTogether(takeTurns(picks)))...)
	items = withComebacks(items, c.comebacks, c.confusable)
	if c.ending != nil {
		items = append(items, *c.ending)
	}
	return items
}

// pick is cand picked for reason: a play-along on its tempo ladder, an
// exercise, a fretboard cell or a diagram shape.
func (c *composer) pick(cand practiceCandidate, reason domain.PracticePickReason) domain.PracticeSessionItem {
	if cand.diagram == nil {
		return c.item(cand, reason, 0, cand.seconds())
	}
	tempo, noteValue := startTempo(cand)
	return c.item(cand, reason, tempo, domain.PlayAlongSeconds(*cand.diagram, tempo, noteValue, false))
}

// item is cand as a session item; tempo applies to a play-along only, in the
// note value of its best clean take.
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
	if confusable, ok := cand.drill.(domain.ConfusableItem); ok {
		c.confusable[item.ItemKey] = confusable.ConfusableWith()
	}
	if cand.drill != nil {
		planning := domain.DrillPlanning{Pick: rand.IntN, Level: level, Reading: c.reading, Voicing: c.voicing, BassInstrumentID: c.bassInstrumentID, State: cand.state}
		// take already passed over a cell whose way doesn't fit its window.
		way, _ := c.wayOf(cand)
		if cell := cand.cell(); cell != nil {
			window := c.cellWindow(cell, way)
			planning.Window = &window
		}
		if timed, ok := cand.drill.(domain.TimedByWay); ok {
			item.EstimatedSeconds = timed.SecondsAsked(way)
		}
		item.Ask(cand.drill.Plan(way, planning))
		if c.comesBack(item) {
			again := item
			again.Ask(cand.drill.Plan(item.Way, planning))
			c.comebacks[item.ItemKey] = again
		}
	}
	if cand.diagram != nil {
		item.Kind = domain.PracticeItemKindPlayAlong
		bestBPM, noteValue := bestClean(cand)
		var bestNoteValue *domain.TempoNoteValue
		if bestBPM != nil {
			bestNoteValue = &noteValue
		}
		item.PlayAlong = &domain.PlannedPlayAlong{
			DiagramID:          cand.diagram.ID,
			StartTempoBPM:      tempo,
			StartNoteValue:     noteValue,
			TargetTempoBPM:     targetTempo(*cand.diagram),
			BestCleanTempoBPM:  bestBPM,
			BestCleanNoteValue: bestNoteValue,
		}
	}
	return item
}

// chordsBeforeChanges moves each single chord that a later chord change uses
// to just before that change, so a chord is played clean before it is changed
// to in time. Everything else keeps its order.
func chordsBeforeChanges(items []domain.PracticeSessionItem) []domain.PracticeSessionItem {
	ordered := slices.Clone(items)
	for i := 0; i < len(ordered); i++ {
		change := ordered[i].ChordChange
		if change == nil {
			continue
		}
		for j := i + 1; j < len(ordered); j++ {
			chord := ordered[j].ChordVoicing
			if chord == nil || chord.DiagramID != change.FromDiagramID && chord.DiagramID != change.ToDiagramID {
				continue
			}
			moved := ordered[j]
			ordered = slices.Insert(slices.Delete(ordered, j, j+1), i, moved)
			i++
		}
	}
	return ordered
}

// takeTurns orders picks so their drills take turns: each next pick comes
// from the drill with the most picks left, the earliest such drill on a
// tie, other than the previous pick's drill while another has picks left.
// Within a drill the picks keep their order, except that a fretboard cell
// right after another is never on the same string of the same layout while
// a cell of that drill on another string is left. Each exercise type is a
// drill, and the play-alongs are one.
func takeTurns(picks []domain.PracticeSessionItem) []domain.PracticeSessionItem {
	var drills []string
	queues := map[string][]domain.PracticeSessionItem{}
	for _, item := range picks {
		drill := item.DrillTemplateKey()
		if drill == "" {
			drill = string(item.Kind)
		}
		if _, ok := queues[drill]; !ok {
			drills = append(drills, drill)
		}
		queues[drill] = append(queues[drill], item)
	}
	ordered := make([]domain.PracticeSessionItem, 0, len(picks))
	previous := ""
	var lastCell *domain.PlannedFretboardCell
	for len(ordered) < len(picks) {
		next := nextDrill(drills, queues, previous, len(picks)-len(ordered))
		queue := queues[next]
		i := nextInQueue(queue, lastCell)
		item := queue[i]
		queues[next] = slices.Delete(slices.Clone(queue), i, i+1)
		ordered = append(ordered, item)
		previous, lastCell = next, item.FretboardCell
	}
	return ordered
}

// nextDrill is the drill with the most picks left in queues, the earliest
// in drills on a tie, other than previous unless it holds all left picks.
func nextDrill(drills []string, queues map[string][]domain.PracticeSessionItem, previous string, left int) string {
	next := ""
	for _, drill := range drills {
		n := len(queues[drill])
		if n == 0 || drill == previous && n < left {
			continue
		}
		if next == "" || n > len(queues[next]) {
			next = drill
		}
	}
	return next
}

// nextInQueue is the index of queue's next pick: its first, or after a
// fretboard cell, its first that isn't on that cell's string, if any.
func nextInQueue(queue []domain.PracticeSessionItem, lastCell *domain.PlannedFretboardCell) int {
	if lastCell == nil {
		return 0
	}
	if j := slices.IndexFunc(queue, func(item domain.PracticeSessionItem) bool { return !sameString(item.FretboardCell, lastCell) }); j >= 0 {
		return j
	}
	return 0
}

// sameString reports whether cell is a fretboard cell on last's string of
// the same layout.
func sameString(cell, last *domain.PlannedFretboardCell) bool {
	return cell != nil && cell.LayoutInstrumentID == last.LayoutInstrumentID && cell.String == last.String
}

// readyFor reports whether the student may meet item: true unless it waits on
// a node the student hasn't reached the level of yet.
func readyFor(knowledge KnowledgeMap, item domain.DrillItem) bool {
	waiting, ok := item.(domain.WaitingItem)
	if !ok {
		return true
	}
	nodeKey, level, waits := waiting.WaitsOn()
	if !waits {
		return true
	}
	i := slices.IndexFunc(knowledge.Nodes, func(n domain.KnowledgeNode) bool { return n.Key == nodeKey })
	if i < 0 {
		return false
	}
	reached := knowledge.Standings[knowledge.Nodes[i].ID].Level
	return reached != nil && reached.Rank() >= level.Rank()
}
