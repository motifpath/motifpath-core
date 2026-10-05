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
	log := s.logger.With("event_id", answer.EventID, "student_id", answer.StudentID, "item_key", answer.ItemKey)

	key, err := domain.ParsePracticeItemKey(answer.ItemKey)
	if err != nil {
		log.WarnContext(ctx, "practice answer has an invalid item key, dropping")
		return nil
	}
	grader, ok := domain.GraderFor(key.Kind)
	if !ok {
		log.InfoContext(ctx, "no grader for this item kind yet, dropping", "kind", key.Kind)
		return nil
	}

	diagrams, err := s.reference.Diagrams(ctx, key.DiagramIDs())
	if err != nil {
		return err
	}
	result := grader.Grade(key, answer.Response, domain.PracticeReference{Diagrams: diagrams})
	if result.Rejection != "" {
		log.InfoContext(ctx, "practice answer rejected by its grader", "grader", grader.ID(), "reason", result.Rejection)
		return nil
	}

	evidence := toEvidence(answer, grader.ID(), result.Evidence)
	inserted, err := s.evidence.Insert(ctx, evidence)
	if err != nil {
		return err
	}
	return s.foldInto(ctx, evidence, goalOf(key, diagrams), inserted)
}

func (s *PracticeEvidenceService) foldInto(ctx context.Context, evidence domain.PracticeEvidence, goal domain.ItemGoal, inserted bool) error {
	fold, rulesVersion, found, err := s.states.Get(ctx, evidence.StudentID, evidence.ItemKey)
	if err != nil {
		return err
	}

	// Folding onto a state built by other rules would mix the two, so such a state
	// is rebuilt from the evidence, like a late or repeated answer.
	// The item's daily snapshots are rewritten along with it, from the day of the
	// earliest evidence that changed.
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
		GraderID:          graderID,
		Response:          answer.Response,
		Correct:           graded.Correct,
		LatencyMs:         graded.LatencyMs,
		TapMs:             answer.TapMs,
		Rating:            graded.Rating,
		TempoBPM:          graded.TempoBPM,
		ChangesPerMinute:  graded.ChangesPerMinute,
	}
}

// goalOf is what the item's fluency is measured against. A play-along is measured
// against its diagram's tempo. Chord changes have no source for a target rate yet,
// so any clean minute counts as fully fluent.
func goalOf(key domain.PracticeItemKey, diagrams map[string]domain.DiagramReference) domain.ItemGoal {
	if key.Kind != domain.PracticeItemKindPlayAlong {
		return domain.ItemGoal{}
	}
	return domain.ItemGoal{TargetTempoBPM: diagrams[key.DiagramIDs()[0]].TempoBPM}
}
