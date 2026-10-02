package application

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// ExerciseService manages Exercise — a reusable, standalone practice item
// that may be linked to any number of Challenges and ContentNodes (as a path
// exercise), and selected into practice sessions by skill tag.
type ExerciseService struct {
	challenges ports.ChallengeRepository
	exercises  ports.ExerciseRepository
	nodes      ports.ContentNodeRepository
	knowledge  ports.KnowledgeNodeRepository
	diagrams   ports.DiagramRepository
	// instruments tells a diagram stimulus's family and string count, which
	// decide whether its answers are fretboard cells.
	instruments ports.InstrumentRepository
	// voices checks the voice a diagram stimulus plays with.
	voices ports.VoiceRepository
	// users names the creators ListExerciseCreators returns.
	users ports.UserRepository
	newID func() string
	now   func() time.Time
	// shuffle randomizes n elements in place via swap, matching
	// math/rand.Shuffle's signature — injected so tests can supply a
	// deterministic permutation instead of a real random one.
	shuffle func(n int, swap func(i, j int))
}

func NewExerciseService(
	challenges ports.ChallengeRepository,
	exercises ports.ExerciseRepository,
	nodes ports.ContentNodeRepository,
	knowledge ports.KnowledgeNodeRepository,
	diagrams ports.DiagramRepository,
	instruments ports.InstrumentRepository,
	voices ports.VoiceRepository,
	users ports.UserRepository,
	newID func() string,
	now func() time.Time,
	shuffle func(n int, swap func(i, j int)),
) *ExerciseService {
	return &ExerciseService{challenges: challenges, exercises: exercises, nodes: nodes, knowledge: knowledge, diagrams: diagrams, instruments: instruments, voices: voices, users: users, newID: newID, now: now, shuffle: shuffle}
}

// diagramRefRepos are the repositories a diagram reference is checked
// against.
func (s *ExerciseService) diagramRefRepos() diagramRefRepos {
	return diagramRefRepos{diagrams: s.diagrams, instruments: s.instruments, voices: s.voices}
}

// checkEmbeddedVoices checks the voice of every diagram embedded in the
// prompt, in an option's thumbnail or in a remediation's content, reported
// under the field it came from.
func (s *ExerciseService) checkEmbeddedVoices(ctx context.Context, prompt domain.PromptDocument, options []domain.Option, remediationTargets []domain.RemediationTarget) error {
	if err := checkEmbeddedPlaybackVoices(ctx, s.diagramRefRepos(), "prompt", prompt.EmbeddedDiagramRefs()); err != nil {
		return err
	}
	var thumbnails []domain.DiagramRef
	for _, option := range options {
		if option.DiagramRef != nil {
			thumbnails = append(thumbnails, *option.DiagramRef)
		}
	}
	if err := checkEmbeddedPlaybackVoices(ctx, s.diagramRefRepos(), "options", thumbnails); err != nil {
		return err
	}
	var remediation []domain.DiagramRef
	for _, target := range remediationTargets {
		remediation = append(remediation, embeddedRefs(target.RichContent)...)
	}
	return checkEmbeddedPlaybackVoices(ctx, s.diagramRefRepos(), "remediation_targets", remediation)
}

// CreateExercise creates a standalone exercise, not linked to any challenge
// or content node. Only teachers and admins may create exercises.
func (s *ExerciseService) CreateExercise(ctx context.Context, caller domain.User, title string, prompt domain.PromptDocument, exerciseType domain.ExerciseType, skillIDs, conceptIDs []string, imageURL, audioURL *string, diagramRef *domain.DiagramRef, diagramStackRef *domain.DiagramStackRef, options []domain.Option, estimatedDurationSeconds *int, remediationTargets []domain.RemediationTarget, languages []string, instrumentIDs *[]string) (domain.Exercise, error) {
	if !canManageContent(caller.Role) {
		return domain.Exercise{}, domain.ErrForbidden
	}

	if err := s.checkRemediationTargetsExist(ctx, remediationTargets); err != nil {
		return domain.Exercise{}, err
	}

	options, diagramRef, err := s.resolveDiagramOptions(ctx, exerciseType, diagramRef, diagramStackRef, options, nil)
	if err != nil {
		return domain.Exercise{}, err
	}

	exercise, err := domain.NewExercise(s.newID(), title, prompt, exerciseType, skillIDs, conceptIDs, imageURL, audioURL, diagramRef, diagramStackRef, options, estimatedDurationSeconds, remediationTargets, languages, s.now())
	if err != nil {
		return domain.Exercise{}, err
	}
	exercise.CreatedBy = caller.ID
	if instrumentIDs != nil {
		if exercise, err = s.forInstruments(ctx, exercise, *instrumentIDs); err != nil {
			return domain.Exercise{}, err
		}
	}
	if err := s.checkEmbeddedVoices(ctx, prompt, options, remediationTargets); err != nil {
		return domain.Exercise{}, err
	}
	if err := checkClassificationSuits(ctx, s.knowledge, skillIDs, conceptIDs, exercise.InstrumentIDs); err != nil {
		return domain.Exercise{}, err
	}
	if err := s.exercises.Create(ctx, exercise); err != nil {
		return domain.Exercise{}, err
	}
	// Re-fetched rather than returned as constructed: exercise.Languages/
	// Skills/Concepts only carry the request-supplied codes/ids until read
	// back with their rows joined in.
	return s.exercises.GetByID(ctx, exercise.ID)
}

// forInstruments returns exercise for instrumentIDs, each of which must
// reference an existing instrument.
func (s *ExerciseService) forInstruments(ctx context.Context, exercise domain.Exercise, instrumentIDs []string) (domain.Exercise, error) {
	exercise, err := exercise.ForInstruments(instrumentIDs)
	if err != nil {
		return domain.Exercise{}, err
	}
	if err := checkInstrumentsExist(ctx, s.instruments, instrumentIDs); err != nil {
		return domain.Exercise{}, err
	}
	return exercise, nil
}

// checkRemediationTargetsExist reports a domain.ValidationError under
// "remediation_targets" if any target's ContentNodeID does not reference an
// existing content node. Whether a target's shape (exactly one of
// content_node_id/rich_content) is valid is domain.NewExercise/Update's own
// concern — this only checks the existence a repository round-trip
// requires. Looks up every referenced id in a single batched GetByIDs call
// rather than one GetByID per target, since a caller may name the same
// content node more than once across targets and each lookup is otherwise a
// separate round trip.
func (s *ExerciseService) checkRemediationTargetsExist(ctx context.Context, targets []domain.RemediationTarget) error {
	ids := make([]string, 0, len(targets))
	for _, target := range targets {
		if target.ContentNodeID != nil && *target.ContentNodeID != "" {
			ids = append(ids, *target.ContentNodeID)
		}
	}
	if len(ids) == 0 {
		return nil
	}

	found, err := s.nodes.GetByIDs(ctx, ids)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, ok := found[id]; !ok {
			return domain.NewValidationError("remediation_targets", "references a content node that does not exist: "+id)
		}
	}
	return nil
}

// resolveDiagramOptions builds an image_recognition exercise's options from
// its diagram stimulus, replacing whatever options the caller supplied —
// the domain constructor/Update rejects a non-empty caller-supplied options
// list alongside a diagram stimulus, so this only ever overwrites an empty
// slice with the diagram-derived one. It also returns diagramRef as it is to
// be stored: older correct_intervals converted to correct_position_ids.
// Returns options and diagramRef unchanged when neither diagramRef nor
// diagramStackRef is given. A stack's entries must all reference diagrams on
// the same instrument; that requires a repository round trip, so is checked
// here rather than in the domain layer. previous is the exercise's options
// before an update (nil on create): a single diagram's option that stands for
// the same choice as one of them keeps its id, since students' recorded
// answers name options by id.
func (s *ExerciseService) resolveDiagramOptions(ctx context.Context, exerciseType domain.ExerciseType, diagramRef *domain.DiagramRef, diagramStackRef *domain.DiagramStackRef, options, previous []domain.Option) ([]domain.Option, *domain.DiagramRef, error) {
	if exerciseType != domain.ExerciseTypeImageRecognition || (diagramRef == nil && diagramStackRef == nil) {
		return options, diagramRef, nil
	}
	if len(options) > 0 {
		return nil, nil, domain.NewValidationError("options", "must be omitted when diagram_ref or diagram_stack_ref is given")
	}

	resolved, err := resolveDiagramRefs(ctx, s.diagramRefRepos(), diagramRef, diagramStackRef)
	if err != nil {
		return nil, nil, err
	}

	if diagramRef != nil {
		ref, derived, err := s.optionsFromStimulus(ctx, resolved[0].diagram, resolved[0].ref, previous)
		if err != nil {
			return nil, nil, err
		}
		return derived, &ref, nil
	}

	derived := []domain.Option{}
	for _, r := range resolved {
		derived = append(derived, s.optionsFromDiagram(r.diagram, r.ref)...)
	}
	return derived, diagramRef, nil
}

// optionsFromStimulus derives a single diagram stimulus's options. Its
// correct answers are positions of the diagram (older correct_intervals are
// converted to the drawn positions with those intervals, and the returned
// ref stores the positions instead). On a fretted instrument every cell of
// the diagram's answer window is an option, correct where a correct
// position sits; on any other, each position is one. An option standing for
// the same cell or position of this diagram as one of previous keeps its id.
func (s *ExerciseService) optionsFromStimulus(ctx context.Context, diagram domain.Diagram, ref domain.DiagramRef, previous []domain.Option) (domain.DiagramRef, []domain.Option, error) {
	ref, correct, err := correctPositions(diagram, ref)
	if err != nil {
		return ref, nil, err
	}
	instrument, err := s.instruments.GetByID(ctx, diagram.InstrumentID)
	if err != nil {
		return ref, nil, err
	}
	ids := previousOptionIDs(diagram.ID, previous)
	if instrument.Family != domain.InstrumentFamilyFretted || instrument.StringCount == nil {
		return ref, s.positionOptions(diagram, correct, ids), nil
	}
	return ref, s.cellOptions(diagram, *instrument.StringCount, correct, ids), nil
}

// previousIDs are the ids of an exercise's earlier options of one diagram,
// by the choice each stood for: a fretboard cell, or a position.
type previousIDs struct {
	byCell     map[domain.FretCell]string
	byPosition map[string]string
}

// previousOptionIDs indexes the options of previous derived from diagramID.
func previousOptionIDs(diagramID string, previous []domain.Option) previousIDs {
	ids := previousIDs{byCell: map[domain.FretCell]string{}, byPosition: map[string]string{}}
	for _, opt := range previous {
		if opt.DiagramID == nil || *opt.DiagramID != diagramID {
			continue
		}
		switch {
		case opt.FretCell != nil:
			ids.byCell[*opt.FretCell] = opt.ID
		case opt.DiagramPositionID != nil:
			ids.byPosition[*opt.DiagramPositionID] = opt.ID
		}
	}
	return ids
}

// idOr returns id when it names an earlier option, else a new one.
func (s *ExerciseService) idOr(id string, ok bool) string {
	if ok {
		return id
	}
	return s.newID()
}

// correctPositions returns ref with any older correct_intervals converted to
// correct_position_ids, and those ids as a set — refusing an id that isn't
// one of diagram's positions.
func correctPositions(diagram domain.Diagram, ref domain.DiagramRef) (domain.DiagramRef, map[string]bool, error) {
	if ref.CorrectPositionIDs == nil && ref.CorrectIntervals != nil {
		ids := drawnPositionIDsWithIntervals(diagram, ref)
		ref.CorrectPositionIDs = &ids
		ref.CorrectIntervals = nil
	}
	correct := map[string]bool{}
	if ref.CorrectPositionIDs == nil {
		return ref, correct, nil
	}
	for _, id := range *ref.CorrectPositionIDs {
		if !slices.ContainsFunc(diagram.Positions, func(p domain.Position) bool { return p.ID == id }) {
			return ref, nil, domain.NewValidationError("diagram_ref", "correct_position_ids names a position that isn't the diagram's: "+id)
		}
		correct[id] = true
	}
	return ref, correct, nil
}

// positionOptions is one option per position of diagram, for an instrument
// with no answer cells.
func (s *ExerciseService) positionOptions(diagram domain.Diagram, correct map[string]bool, previous previousIDs) []domain.Option {
	diagramID := diagram.ID
	options := make([]domain.Option, 0, len(diagram.Positions))
	for _, pos := range diagram.Positions {
		positionID := pos.ID
		id, reused := previous.byPosition[pos.ID]
		options = append(options, domain.Option{ID: s.idOr(id, reused), IsCorrect: correct[pos.ID], DiagramID: &diagramID, DiagramPositionID: &positionID})
	}
	return options
}

// cellOptions is one option per cell of diagram's answer window, naming the
// position that occupies it, if any, and correct where that position is.
func (s *ExerciseService) cellOptions(diagram domain.Diagram, stringCount int, correct map[string]bool, previous previousIDs) []domain.Option {
	occupant := map[domain.FretCell]string{}
	for _, pos := range diagram.Positions {
		if pos.String != nil && pos.Fret != nil {
			occupant[domain.FretCell{String: *pos.String, Fret: *pos.Fret}] = pos.ID
		}
	}
	diagramID := diagram.ID
	cells := domain.FrettedAnswerCells(diagram.Positions, diagram.Regions, stringCount)
	options := make([]domain.Option, 0, len(cells))
	for _, cell := range cells {
		id, reused := previous.byCell[cell]
		option := domain.Option{ID: s.idOr(id, reused), DiagramID: &diagramID, FretCell: &cell}
		if positionID, ok := occupant[cell]; ok {
			option.DiagramPositionID = &positionID
			option.IsCorrect = correct[positionID]
		}
		options = append(options, option)
	}
	return options
}

// drawnPositionIDsWithIntervals returns the ids of diagram's positions that
// ref draws (neither hidden nor filtered out by its subset) whose interval
// is among ref.CorrectIntervals — what correct_intervals always meant.
func drawnPositionIDsWithIntervals(diagram domain.Diagram, ref domain.DiagramRef) []string {
	ids := []string{}
	for _, pos := range diagram.Positions {
		if ref.Layers.HiddenPositionIDs != nil && slices.Contains(*ref.Layers.HiddenPositionIDs, pos.ID) {
			continue
		}
		if ref.Layers.Subset != nil && !slices.Contains(*ref.Layers.Subset, pos.Interval) {
			continue
		}
		if slices.Contains(*ref.CorrectIntervals, pos.Interval) {
			ids = append(ids, pos.ID)
		}
	}
	return ids
}

// optionsFromDiagram derives one Option per position of diagram that
// survives ref.Layers.Subset filtering, marked correct when its interval is
// among ref.CorrectIntervals.
func (s *ExerciseService) optionsFromDiagram(diagram domain.Diagram, ref domain.DiagramRef) []domain.Option {
	var subset map[string]struct{}
	if ref.Layers.Subset != nil {
		subset = make(map[string]struct{}, len(*ref.Layers.Subset))
		for _, interval := range *ref.Layers.Subset {
			subset[interval] = struct{}{}
		}
	}
	var correct map[string]struct{}
	if ref.CorrectIntervals != nil {
		correct = make(map[string]struct{}, len(*ref.CorrectIntervals))
		for _, interval := range *ref.CorrectIntervals {
			correct[interval] = struct{}{}
		}
	}

	diagramID := diagram.ID
	options := []domain.Option{}
	for _, pos := range diagram.Positions {
		if subset != nil {
			if _, visible := subset[pos.Interval]; !visible {
				continue
			}
		}
		_, isCorrect := correct[pos.Interval]
		positionID := pos.ID
		options = append(options, domain.Option{
			ID:                s.newID(),
			IsCorrect:         isCorrect,
			DiagramID:         &diagramID,
			DiagramPositionID: &positionID,
		})
	}
	return options
}

// GetExercise returns the exercise with the given id. Any authenticated user
// may retrieve an exercise.
func (s *ExerciseService) GetExercise(ctx context.Context, id string) (domain.Exercise, error) {
	return s.exercises.GetByID(ctx, id)
}

// ListExercises returns exercises from the reusable pool, optionally
// narrowed by filter (every zero-valued field means "no filter"). Only
// teachers and admins may list exercises — the pool is an
// authoring surface, unlike GetExercise which any authenticated user may
// call for a specific known id.
func (s *ExerciseService) ListExercises(ctx context.Context, caller domain.User, filter domain.ExerciseFilter, page domain.PageRequest) (domain.Page[domain.Exercise], error) {
	if !canManageContent(caller.Role) {
		return domain.Page[domain.Exercise]{}, domain.ErrForbidden
	}
	return s.exercises.List(ctx, filter, page)
}

// ListExerciseCreators returns the distinct creators of the exercises in the
// pool, so a picker can offer a complete creator filter without paging.
// Only teachers and admins may list them, as with the pool itself. A
// non-empty nameQuery keeps only the creators whose display name contains
// it; see namedCreators for matching and ordering.
func (s *ExerciseService) ListExerciseCreators(ctx context.Context, caller domain.User, nameQuery string) ([]Creator, error) {
	if !canManageContent(caller.Role) {
		return nil, domain.ErrForbidden
	}
	ids, err := s.exercises.ListCreatorIDs(ctx)
	if err != nil {
		return nil, err
	}
	return namedCreators(ctx, s.users, ids, nameQuery)
}

// UpdateExercise replaces the given exercise's title, prompt, skill/concept
// links, stimulus media, options, estimated duration, remediation targets,
// and languages. exercise_type cannot be changed, and the exercise's
// challenge/content-node links are untouched. Only teachers and admins may
// update an exercise. Returns domain.ErrNotFound if no exercise exists with
// the given id.
func (s *ExerciseService) UpdateExercise(ctx context.Context, caller domain.User, id, title string, prompt domain.PromptDocument, skillIDs, conceptIDs []string, imageURL, audioURL *string, diagramRef *domain.DiagramRef, diagramStackRef *domain.DiagramStackRef, options []domain.Option, estimatedDurationSeconds *int, remediationTargets []domain.RemediationTarget, languages []string, instrumentIDs *[]string) (domain.Exercise, error) {
	if !canManageContent(caller.Role) {
		return domain.Exercise{}, domain.ErrForbidden
	}

	existing, err := s.exercises.GetByID(ctx, id)
	if err != nil {
		return domain.Exercise{}, err
	}

	if err := s.checkRemediationTargetsExist(ctx, remediationTargets); err != nil {
		return domain.Exercise{}, err
	}

	options, diagramRef, err = s.resolveDiagramOptions(ctx, existing.ExerciseType, diagramRef, diagramStackRef, options, existing.Options)
	if err != nil {
		return domain.Exercise{}, err
	}

	updated, err := existing.Update(title, prompt, skillIDs, conceptIDs, imageURL, audioURL, diagramRef, diagramStackRef, options, estimatedDurationSeconds, remediationTargets, languages)
	if err != nil {
		return domain.Exercise{}, err
	}
	if instrumentIDs != nil {
		if updated, err = s.forInstruments(ctx, updated, *instrumentIDs); err != nil {
			return domain.Exercise{}, err
		}
	}
	if err := s.checkEmbeddedVoices(ctx, prompt, options, remediationTargets); err != nil {
		return domain.Exercise{}, err
	}
	if err := checkClassificationSuits(ctx, s.knowledge, skillIDs, conceptIDs, updated.InstrumentIDs); err != nil {
		return domain.Exercise{}, err
	}
	if instrumentIDs != nil {
		if err := s.checkLinkedNodesFit(ctx, updated); err != nil {
			return domain.Exercise{}, err
		}
	}

	if err := s.exercises.Update(ctx, updated); err != nil {
		return domain.Exercise{}, err
	}
	// Re-fetched rather than returned as updated: same construct-then-refetch
	// convention CreateExercise follows, for the same reason (see its comment).
	return s.exercises.GetByID(ctx, updated.ID)
}

// checkFit returns a domain.ErrConflict unless exercise suits node's
// instruments — see domain.Exercise.Suits.
func checkFit(exercise domain.Exercise, node domain.ContentNode) error {
	if exercise.Suits(node.InstrumentIDs) {
		return nil
	}
	return fmt.Errorf("%w: the exercise is for none of content node %s's instruments", domain.ErrConflict, node.ID)
}

// checkLinkedNodesFit returns a domain.ErrConflict unless exercise still
// suits every content node it is linked to, as a path exercise or through
// one of the node's challenges.
func (s *ExerciseService) checkLinkedNodesFit(ctx context.Context, exercise domain.Exercise) error {
	nodeIDs := slices.Clone(exercise.ContentNodeIDs)
	for _, challengeID := range exercise.ChallengeIDs {
		challenge, err := s.challenges.GetByID(ctx, challengeID)
		if err != nil {
			return err
		}
		nodeIDs = append(nodeIDs, challenge.ContentNodeID)
	}
	if len(nodeIDs) == 0 {
		return nil
	}
	nodes, err := s.nodes.GetByIDs(ctx, nodeIDs)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if err := checkFit(exercise, node); err != nil {
			return err
		}
	}
	return nil
}

// LinkExerciseToChallenge links an existing exercise into a challenge. Only
// teachers and admins may link exercises. Returns domain.ErrAlreadyExists if
// the exercise is already linked to the challenge.
func (s *ExerciseService) LinkExerciseToChallenge(ctx context.Context, caller domain.User, challengeID, exerciseID string) (domain.Exercise, error) {
	if !canManageContent(caller.Role) {
		return domain.Exercise{}, domain.ErrForbidden
	}

	challenge, err := s.challenges.GetByID(ctx, challengeID)
	if err != nil {
		return domain.Exercise{}, err
	}
	exercise, err := s.exercises.GetByID(ctx, exerciseID)
	if err != nil {
		return domain.Exercise{}, err
	}
	for _, id := range exercise.ChallengeIDs {
		if id == challengeID {
			return domain.Exercise{}, domain.ErrAlreadyExists
		}
	}
	node, err := s.nodes.GetByID(ctx, challenge.ContentNodeID)
	if err != nil {
		return domain.Exercise{}, err
	}
	if err := checkFit(exercise, node); err != nil {
		return domain.Exercise{}, err
	}

	if err := s.exercises.LinkChallenge(ctx, exerciseID, challengeID); err != nil {
		return domain.Exercise{}, err
	}
	return s.exercises.GetByID(ctx, exerciseID)
}

// UnlinkExerciseFromChallenge removes the link between an exercise and a
// challenge. Only teachers and admins may unlink exercises. Returns
// domain.ErrNotFound if the exercise is not currently linked to the
// challenge.
func (s *ExerciseService) UnlinkExerciseFromChallenge(ctx context.Context, caller domain.User, challengeID, exerciseID string) error {
	if !canManageContent(caller.Role) {
		return domain.ErrForbidden
	}

	if _, err := s.challenges.GetByID(ctx, challengeID); err != nil {
		return err
	}
	exercise, err := s.exercises.GetByID(ctx, exerciseID)
	if err != nil {
		return err
	}
	linked := false
	for _, id := range exercise.ChallengeIDs {
		if id == challengeID {
			linked = true
			break
		}
	}
	if !linked {
		return domain.ErrNotFound
	}

	return s.exercises.UnlinkChallenge(ctx, exerciseID, challengeID)
}

// ListExercisesForChallenge returns challengeID's linked exercises, in the
// order (and with the option order) the challenge's shuffle settings call
// for. Returns domain.ErrNotFound if no such challenge exists. Any
// authenticated user may list a challenge's exercises.
func (s *ExerciseService) ListExercisesForChallenge(ctx context.Context, challengeID string) ([]domain.Exercise, error) {
	challenge, err := s.challenges.GetByID(ctx, challengeID)
	if err != nil {
		return nil, err
	}

	exercises, err := s.exercises.ListByChallengeID(ctx, challengeID)
	if err != nil {
		return nil, err
	}

	if challenge.ShuffleExercises {
		s.shuffle(len(exercises), func(i, j int) { exercises[i], exercises[j] = exercises[j], exercises[i] })
	}
	if challenge.ShuffleOptions {
		for i := range exercises {
			s.shuffleOptions(exercises[i].Options)
		}
	}
	return exercises, nil
}

// LinkExerciseToContentNode links an existing exercise into a content node
// as a path exercise. Only teachers and admins may link exercises. Returns
// domain.ErrAlreadyExists if the exercise is already a path exercise on the
// node.
func (s *ExerciseService) LinkExerciseToContentNode(ctx context.Context, caller domain.User, contentNodeID, exerciseID string) (domain.Exercise, error) {
	if !canManageContent(caller.Role) {
		return domain.Exercise{}, domain.ErrForbidden
	}

	node, err := s.nodes.GetByID(ctx, contentNodeID)
	if err != nil {
		return domain.Exercise{}, err
	}
	exercise, err := s.exercises.GetByID(ctx, exerciseID)
	if err != nil {
		return domain.Exercise{}, err
	}
	for _, id := range exercise.ContentNodeIDs {
		if id == contentNodeID {
			return domain.Exercise{}, domain.ErrAlreadyExists
		}
	}
	if err := checkFit(exercise, node); err != nil {
		return domain.Exercise{}, err
	}

	if err := s.exercises.LinkContentNode(ctx, exerciseID, contentNodeID); err != nil {
		return domain.Exercise{}, err
	}
	return s.exercises.GetByID(ctx, exerciseID)
}

// UnlinkExerciseFromContentNode removes the path-exercise link between an
// exercise and a content node. Only teachers and admins may unlink
// exercises. Returns domain.ErrNotFound if the exercise is not currently
// linked to the node.
func (s *ExerciseService) UnlinkExerciseFromContentNode(ctx context.Context, caller domain.User, contentNodeID, exerciseID string) error {
	if !canManageContent(caller.Role) {
		return domain.ErrForbidden
	}

	if _, err := s.nodes.GetByID(ctx, contentNodeID); err != nil {
		return err
	}
	exercise, err := s.exercises.GetByID(ctx, exerciseID)
	if err != nil {
		return err
	}
	linked := false
	for _, id := range exercise.ContentNodeIDs {
		if id == contentNodeID {
			linked = true
			break
		}
	}
	if !linked {
		return domain.ErrNotFound
	}

	return s.exercises.UnlinkContentNode(ctx, exerciseID, contentNodeID)
}

// ListPathExercisesForContentNode returns contentNodeID's path exercises,
// always in link order. Returns domain.ErrNotFound if no such content node
// exists. Any authenticated user may list a node's path exercises.
func (s *ExerciseService) ListPathExercisesForContentNode(ctx context.Context, contentNodeID string) ([]domain.Exercise, error) {
	if _, err := s.nodes.GetByID(ctx, contentNodeID); err != nil {
		return nil, err
	}
	return s.exercises.ListByContentNodeID(ctx, contentNodeID)
}

// PracticeSession is a generated, skill-targeted set of exercises for
// self-directed practice. It is never persisted — StartPracticeSession
// returns a fresh selection and ID on every call.
type PracticeSession struct {
	ID        string
	SkillID   string
	Exercises []domain.Exercise
}

// StartPracticeSession selects up to count exercises linked to skillID, in
// random order with each exercise's options also randomized, under a fresh
// session ID. Returns fewer than count exercises if the linked pool is
// smaller. Any authenticated user may start a practice session.
func (s *ExerciseService) StartPracticeSession(ctx context.Context, skillID string, count int) (PracticeSession, error) {
	var errs []domain.FieldError
	if skillID == "" {
		errs = append(errs, domain.FieldError{Field: "skill_id", Reason: "must not be empty"})
	}
	if count < 1 || count > 50 {
		errs = append(errs, domain.FieldError{Field: "count", Reason: "must be between 1 and 50"})
	}
	if len(errs) > 0 {
		return PracticeSession{}, &domain.ValidationError{Fields: errs}
	}

	pool, err := s.exercises.ListBySkillID(ctx, skillID)
	if err != nil {
		return PracticeSession{}, err
	}

	s.shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	if len(pool) > count {
		pool = pool[:count]
	}
	for i := range pool {
		s.shuffleOptions(pool[i].Options)
	}

	return PracticeSession{ID: s.newID(), SkillID: skillID, Exercises: pool}, nil
}

func (s *ExerciseService) shuffleOptions(options []domain.Option) {
	s.shuffle(len(options), func(i, j int) { options[i], options[j] = options[j], options[i] })
}
