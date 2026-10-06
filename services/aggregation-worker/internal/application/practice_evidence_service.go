package application

import (
	"context"
	"log/slog"

	"github.com/motifpath/aggregation-worker/internal/domain"
	"github.com/motifpath/aggregation-worker/internal/ports"
)

// PracticeEvidenceService is the evidence processor: it grades each practice answer
// into evidence and folds that evidence into the item's state. It is the single
// writer for each student's practice state, because motifpath.events is keyed by
// student_id, so one consumer sees all of a student's events in order.
type PracticeEvidenceService struct {
	reference ports.PracticeReferenceReader
	evidence  ports.PracticeEvidenceRepository
	states    ports.PracticeItemStateRepository
	history   ports.PracticeItemHistoryRepository
	logger    *slog.Logger
}

func NewPracticeEvidenceService(
	reference ports.PracticeReferenceReader,
	evidence ports.PracticeEvidenceRepository,
	states ports.PracticeItemStateRepository,
	history ports.PracticeItemHistoryRepository,
	logger *slog.Logger,
) *PracticeEvidenceService {
	return &PracticeEvidenceService{reference: reference, evidence: evidence, states: states, history: history, logger: logger}
}

// Process grades one answer and folds it. An answer that can't be graded (a
// malformed key, a kind with no grader yet, a rejected response) is logged and
// dropped: a redelivery would grade it the same way. The events collection keeps
// it, so a later grader can still replay it. Storage failures are returned, and
// the consumer retries the message before it moves on to the next one.
//
// Every step is safe to repeat. A duplicate inserts no evidence and rebuilds the
// item, which also repairs a state write that failed after the evidence was stored.
// A state folded under other mastery rules is rebuilt too, the next time the item
// is answered.
func (s *PracticeEvidenceService) Process(ctx context.Context, answer domain.PracticeAnswer) error {
	_, err := s.process(ctx, answer)
	return err
}

// process is Process, returning the timed drill template the answer practised:
// empty when it practised none or couldn't be graded.
func (s *PracticeEvidenceService) process(ctx context.Context, answer domain.PracticeAnswer) (string, error) {
	log := s.logger.With("event_id", answer.EventID, "student_id", answer.StudentID, "item_key", answer.ItemKey)

	key, err := domain.ParsePracticeItemKey(answer.ItemKey)
	if err != nil {
		log.WarnContext(ctx, "practice answer has an invalid item key, dropping")
		return "", nil
	}
	grader, ok := domain.GraderFor(key.Kind)
	if !ok {
		log.InfoContext(ctx, "no grader for this item kind yet, dropping", "kind", key.Kind)
		return "", nil
	}

	ref, err := s.referenceFor(ctx, key)
	if err != nil {
		return "", err
	}
	result := grader.Grade(key, answer.Response, ref)
	if result.Rejection != "" {
		log.InfoContext(ctx, "practice answer rejected by its grader", "grader", grader.ID(), "reason", result.Rejection)
		return "", nil
	}
	goal, err := s.goalOf(ctx, key, ref)
	if err != nil {
		return "", err
	}

	evidence := toEvidence(answer, grader.ID(), result.Evidence)
	inserted, err := s.evidence.Insert(ctx, evidence)
	if err != nil {
		return "", err
	}
	if err := s.foldInto(ctx, evidence, goal, inserted); err != nil {
		return "", err
	}
	return domain.TimedDrillTemplate(key, answer.Response, ref), nil
}

// referenceFor reads the reference data the item's key points at.
func (s *PracticeEvidenceService) referenceFor(ctx context.Context, key domain.PracticeItemKey) (domain.PracticeReference, error) {
	diagrams, err := s.reference.Diagrams(ctx, key.DiagramIDs())
	if err != nil {
		return domain.PracticeReference{}, err
	}
	ref := domain.PracticeReference{Diagrams: diagrams}
	if id := key.ExerciseID(); id != "" {
		if ref.Exercises, err = s.reference.Exercises(ctx, []string{id}); err != nil {
			return domain.PracticeReference{}, err
		}
	}
	if id := key.LayoutInstrumentID(); id != "" {
		if ref.Instruments, err = s.reference.Instruments(ctx, []string{id}); err != nil {
			return domain.PracticeReference{}, err
		}
	}
	return ref, nil
}

func (s *PracticeEvidenceService) foldInto(ctx context.Context, evidence domain.PracticeEvidence, goal domain.ItemGoal, inserted bool) error {
	fold, rulesVersion, found, err := s.states.Get(ctx, evidence.StudentID, evidence.ItemKey)
	if err != nil {
		return err
	}

	// Folding onto a state built by other rules would mix the two, so such a state
	// is rebuilt from the evidence, like a late or repeated answer. A rebuild
	// rewrites every one of the item's daily snapshots from its whole history; an
	// in-order answer only writes the snapshot of its own day.
	staleRules := found && rulesVersion != domain.PracticeRulesVersion
	late := fold.LastAt != nil && evidence.OccurredAt.Before(*fold.LastAt)
	var snapshots []domain.ItemSnapshot
	if inserted && !late && !staleRules {
		fold, err = domain.FoldEvidence(fold, evidence, goal)
		snapshots = []domain.ItemSnapshot{{Day: domain.SnapshotDay(evidence.OccurredAt), Fold: fold}}
	} else {
		var history []domain.PracticeEvidence
		if history, err = s.evidence.ListForItem(ctx, evidence.StudentID, evidence.ItemKey); err == nil {
			snapshots, err = domain.DailySnapshots(history, goal)
		}
		if len(snapshots) > 0 {
			fold = snapshots[len(snapshots)-1].Fold
		}
	}
	if err != nil {
		return err
	}
	if err := s.history.Put(ctx, evidence.StudentID, evidence.ItemKey, snapshots); err != nil {
		return err
	}
	return s.states.Put(ctx, evidence.StudentID, evidence.ItemKey, fold)
}

func toEvidence(answer domain.PracticeAnswer, graderID string, graded domain.GradedEvidence) domain.PracticeEvidence {
	return domain.PracticeEvidence{
		EvidenceID:        answer.EventID,
		StudentID:         answer.StudentID,
		ItemKey:           answer.ItemKey,
		Source:            graded.Source,
		OccurredAt:        answer.OccurredAt,
		PracticeSessionID: answer.PracticeSessionID,
		TriggerContext:    answer.TriggerContext,
		GraderID:          graderID,
		Response:          answer.Response,
		Correct:           graded.Correct,
		LatencyMs:         graded.LatencyMs,
		AudioMs:           graded.AudioMs,
		TapMs:             answer.TapMs,
		Rating:            graded.Rating,
		TempoBPM:          graded.TempoBPM,
		ChangesPerMinute:  graded.ChangesPerMinute,
		AnswerKey:         graded.AnswerKey,
	}
}

// goalOf is what the item's fluency is measured against. A play-along is measured
// against its diagram's tempo, an exercise against its type's fluent times, and a
// fretboard cell against the fluent times of each way it is asked. Chord changes
// have no source for a target rate yet, so any clean minute counts as fully fluent.
func (s *PracticeEvidenceService) goalOf(ctx context.Context, key domain.PracticeItemKey, ref domain.PracticeReference) (domain.ItemGoal, error) {
	switch key.Kind {
	case domain.PracticeItemKindPlayAlong:
		return domain.ItemGoal{TargetTempoBPM: ref.Diagrams[key.DiagramIDs()[0]].TempoBPM}, nil
	case domain.PracticeItemKindExercise:
		fluentTimes, err := s.reference.FluentTimes(ctx, ref.Exercises[key.ExerciseID()].DrillTemplateKey())
		return domain.ItemGoal{FluentTimes: fluentTimes}, err
	case domain.PracticeItemKindFretboardCell:
		goal := domain.ItemGoal{FluentTimesByResponse: map[domain.PracticeResponseType][]domain.FluentTime{}}
		for _, asked := range []domain.PracticeResponseType{domain.PracticeResponseNameTheNote, domain.PracticeResponseFindTheNote} {
			fluentTimes, err := s.reference.FluentTimes(ctx, string(domain.PracticeItemKindFretboardCell)+":"+string(asked))
			if err != nil {
				return domain.ItemGoal{}, err
			}
			goal.FluentTimesByResponse[asked] = fluentTimes
		}
		return goal, nil
	case domain.PracticeItemKindChordChange:
	}
	return domain.ItemGoal{}, nil
}
