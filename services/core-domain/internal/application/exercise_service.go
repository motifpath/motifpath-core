package application

import (
	"context"
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
	skills     ports.SkillRepository
	concepts   ports.ConceptRepository
	diagrams   ports.DiagramRepository
	newID      func() string
	now        func() time.Time
	// shuffle randomizes n elements in place via swap, matching
	// math/rand.Shuffle's signature — injected so tests can supply a
	// deterministic permutation instead of a real random one.
	shuffle func(n int, swap func(i, j int))
}

func NewExerciseService(
	challenges ports.ChallengeRepository,
	exercises ports.ExerciseRepository,
	nodes ports.ContentNodeRepository,
	skills ports.SkillRepository,
	concepts ports.ConceptRepository,
	diagrams ports.DiagramRepository,
	newID func() string,
	now func() time.Time,
	shuffle func(n int, swap func(i, j int)),
) *ExerciseService {
	return &ExerciseService{challenges: challenges, exercises: exercises, nodes: nodes, skills: skills, concepts: concepts, diagrams: diagrams, newID: newID, now: now, shuffle: shuffle}
}

// CreateExercise creates a standalone exercise, not linked to any challenge
// or content node. Only teachers and admins may create exercises.
func (s *ExerciseService) CreateExercise(ctx context.Context, caller domain.User, title string, prompt domain.PromptDocument, exerciseType domain.ExerciseType, skillIDs, conceptIDs []string, imageURL, audioURL *string, diagramRef *domain.DiagramRef, diagramStackRef *domain.DiagramStackRef, options []domain.Option, estimatedDurationSeconds *int, remediationTargets []domain.RemediationTarget, languages []string) (domain.Exercise, error) {
	if !canManageContent(caller.Role) {
		return domain.Exercise{}, domain.ErrForbidden
	}

	if err := s.checkRemediationTargetsExist(ctx, remediationTargets); err != nil {
		return domain.Exercise{}, err
	}

	options, err := s.resolveDiagramOptions(ctx, exerciseType, diagramRef, diagramStackRef, options)
	if err != nil {
		return domain.Exercise{}, err
	}

	exercise, err := domain.NewExercise(s.newID(), title, prompt, exerciseType, skillIDs, conceptIDs, imageURL, audioURL, diagramRef, diagramStackRef, options, estimatedDurationSeconds, remediationTargets, languages, s.now())
	if err != nil {
		return domain.Exercise{}, err
	}
	if err := checkSkillsAndConceptsExist(ctx, s.skills, s.concepts, skillIDs, conceptIDs); err != nil {
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
// slice with the diagram-derived one. Returns options unchanged when
// neither diagramRef nor diagramStackRef is given. A stack's entries must
// all reference diagrams on the same instrument; that requires a repository
// round trip, so is checked here rather than in the domain layer.
func (s *ExerciseService) resolveDiagramOptions(ctx context.Context, exerciseType domain.ExerciseType, diagramRef *domain.DiagramRef, diagramStackRef *domain.DiagramStackRef, options []domain.Option) ([]domain.Option, error) {
	if exerciseType != domain.ExerciseTypeImageRecognition || (diagramRef == nil && diagramStackRef == nil) {
		return options, nil
	}
	if len(options) > 0 {
		return nil, domain.NewValidationError("options", "must be omitted when diagram_ref or diagram_stack_ref is given")
	}

	resolved, err := resolveDiagramRefs(ctx, s.diagrams, diagramRef, diagramStackRef)
	if err != nil {
		return nil, err
	}

	derived := []domain.Option{}
	for _, r := range resolved {
		derived = append(derived, s.optionsFromDiagram(r.diagram, r.ref)...)
	}
	return derived, nil
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
// narrowed by skillID and/or exerciseType (either may be "" for "no
// filter"). Only teachers and admins may list exercises — the pool is an
// authoring surface, unlike GetExercise which any authenticated user may
// call for a specific known id.
func (s *ExerciseService) ListExercises(ctx context.Context, caller domain.User, filter domain.ExerciseFilter, page domain.PageRequest) (domain.Page[domain.Exercise], error) {
	if !canManageContent(caller.Role) {
		return domain.Page[domain.Exercise]{}, domain.ErrForbidden
	}
	return s.exercises.List(ctx, filter, page)
}

// UpdateExercise replaces the given exercise's title, prompt, skill/concept
// links, stimulus media, options, estimated duration, remediation targets,
// and languages. exercise_type cannot be changed, and the exercise's
// challenge/content-node links are untouched. Only teachers and admins may
// update an exercise. Returns domain.ErrNotFound if no exercise exists with
// the given id.
func (s *ExerciseService) UpdateExercise(ctx context.Context, caller domain.User, id, title string, prompt domain.PromptDocument, skillIDs, conceptIDs []string, imageURL, audioURL *string, diagramRef *domain.DiagramRef, diagramStackRef *domain.DiagramStackRef, options []domain.Option, estimatedDurationSeconds *int, remediationTargets []domain.RemediationTarget, languages []string) (domain.Exercise, error) {
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

	options, err = s.resolveDiagramOptions(ctx, existing.ExerciseType, diagramRef, diagramStackRef, options)
	if err != nil {
		return domain.Exercise{}, err
	}

	updated, err := existing.Update(title, prompt, skillIDs, conceptIDs, imageURL, audioURL, diagramRef, diagramStackRef, options, estimatedDurationSeconds, remediationTargets, languages)
	if err != nil {
		return domain.Exercise{}, err
	}
	if err := checkSkillsAndConceptsExist(ctx, s.skills, s.concepts, skillIDs, conceptIDs); err != nil {
		return domain.Exercise{}, err
	}

	if err := s.exercises.Update(ctx, updated); err != nil {
		return domain.Exercise{}, err
	}
	// Re-fetched rather than returned as updated: same construct-then-refetch
	// convention CreateExercise follows, for the same reason (see its comment).
	return s.exercises.GetByID(ctx, updated.ID)
}

// LinkExerciseToChallenge links an existing exercise into a challenge. Only
// teachers and admins may link exercises. Returns domain.ErrAlreadyExists if
// the exercise is already linked to the challenge.
func (s *ExerciseService) LinkExerciseToChallenge(ctx context.Context, caller domain.User, challengeID, exerciseID string) (domain.Exercise, error) {
	if !canManageContent(caller.Role) {
		return domain.Exercise{}, domain.ErrForbidden
	}

	if _, err := s.challenges.GetByID(ctx, challengeID); err != nil {
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

	if _, err := s.nodes.GetByID(ctx, contentNodeID); err != nil {
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
