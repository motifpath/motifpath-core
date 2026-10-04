//go:build integration

package bdd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/cucumber/godog"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

func registerPracticeSteps(sc *godog.ScenarioContext, w *world) {
	// ── Background ─────────────────────────────────────────────────────────────
	// The fretboard steps set up scenarios that need the fretboard grader and the
	// timed thresholds; the play-along scenarios that share their Background
	// don't depend on them.
	sc.Step(`^the instrument "([^"]*)" in standard tuning ((?:[A-G]#?\d ?)+)$`, w.instrumentInTuning)
	sc.Step(`^student "([^"]*)" is practising in practice session "([^"]*)"$`, w.studentIsPractisingInSession)
	sc.Step(`^student "([^"]*)" practises the guitar fretboard cell on string (\d+) at fret (\d+)$`, w.studentPractisesCell)
	sc.Step(`^the fluent time for naming a note is (\d+) milliseconds net of tap time$`, w.fluentTimeForNamingANote)

	// ── Play-along takes ───────────────────────────────────────────────────────
	sc.Step(`^the play-along diagram "([^"]*)"$`, w.thePlayAlongDiagram)
	sc.Step(`^"([^"]*)" rates a take of (?:play-along )?"([^"]*)" as "([^"]*)" at (\d+) BPM$`, w.ratesATake)
	sc.Step(`^"([^"]*)" rates a take of play-along "([^"]*)" as "([^"]*)" without a tempo$`, w.ratesATakeWithoutATempo)
	sc.Step(`^"([^"]*)" practises the play-along "([^"]*)" and it is in box (\d+)$`, w.practisesThePlayAlongInBox)
	sc.Step(`^"([^"]*)"'s best clean tempo on "([^"]*)" is (\d+) BPM$`, w.bestCleanTempoIs)

	// ── Outcomes ───────────────────────────────────────────────────────────────
	sc.Step(`^"([^"]*)" has self-assessed evidence for play-along "([^"]*)" rated "([^"]*)" at (\d+) BPM$`, w.hasSelfAssessedEvidence)
	sc.Step(`^"([^"]*)" has no evidence for play-along "([^"]*)"$`, w.hasNoEvidenceForPlayAlong)
	sc.Step(`^the answer is rejected because its measure is missing$`, w.answerRejectedBecause(domain.GradeRejectionMeasureMissing))
	sc.Step(`^"([^"]*)" stays in box (\d+)$`, w.itemStaysInBox)
	sc.Step(`^the take is not counted as a miss$`, w.takeIsNotCountedAsAMiss)
	sc.Step(`^the take is counted as a miss$`, w.takeIsCountedAsAMiss)
}

func (w *world) instrumentInTuning(_, tuning string) error {
	if len(strings.Fields(tuning)) == 0 {
		return fmt.Errorf("instrument has no strings")
	}
	return nil
}

func (w *world) studentIsPractisingInSession(student, session string) error {
	w.sessions[student] = stableUUID("session", session)
	return nil
}

func (w *world) studentPractisesCell(student string, _, _ int) error {
	w.studentID(student)
	return nil
}

func (w *world) fluentTimeForNamingANote(ms int) error {
	if ms <= 0 {
		return fmt.Errorf("fluent time must be positive, got %d", ms)
	}
	return nil
}

func (w *world) thePlayAlongDiagram(diagram string) error {
	w.playAlongKey(diagram)
	return nil
}

func rating(r string) (domain.SelfRating, error) {
	switch rt := domain.SelfRating(r); rt {
	case domain.SelfRatingStruggled, domain.SelfRatingAlmost, domain.SelfRatingClean:
		return rt, nil
	}
	return "", fmt.Errorf("unknown rating %q", r)
}

func (w *world) ratesATake(student, diagram, r string, bpm int) error {
	rt, err := rating(r)
	if err != nil {
		return err
	}
	return w.answer(student, w.playAlongKey(diagram), domain.PracticeResponse{
		Type: domain.PracticeResponseSelfRating, Rating: rt, TempoBPM: &bpm,
	})
}

func (w *world) ratesATakeWithoutATempo(student, diagram, r string) error {
	rt, err := rating(r)
	if err != nil {
		return err
	}
	return w.answer(student, w.playAlongKey(diagram), domain.PracticeResponse{
		Type: domain.PracticeResponseSelfRating, Rating: rt,
	})
}

// practisesThePlayAlongInBox plays clean takes at the diagram's tempo, each when
// the item falls due, until the item reaches the box.
func (w *world) practisesThePlayAlongInBox(student, diagram string, box int) error {
	itemKey := w.playAlongKey(diagram)
	for range box {
		if err := w.ratesATake(student, diagram, string(domain.SelfRatingClean), playAlongTargetBPM); err != nil {
			return err
		}
		fold, err := w.fold(student, itemKey)
		if err != nil {
			return err
		}
		if fold.Box == box {
			return nil
		}
		w.clock = *fold.DueAt
	}
	return fmt.Errorf("%q didn't reach box %d", diagram, box)
}

func (w *world) bestCleanTempoIs(student, diagram string, bpm int) error {
	if err := w.ratesATake(student, diagram, string(domain.SelfRatingClean), bpm); err != nil {
		return err
	}
	fold, err := w.fold(student, w.playAlongKey(diagram))
	if err != nil {
		return err
	}
	if fold.BestCleanBPM == nil || *fold.BestCleanBPM != bpm {
		return fmt.Errorf("best clean tempo is %v, want %d", fold.BestCleanBPM, bpm)
	}
	return nil
}

func (w *world) hasSelfAssessedEvidence(student, diagram, r string, bpm int) error {
	evidence, err := w.evidenceFor(student, w.playAlongKey(diagram))
	if err != nil {
		return err
	}
	if len(evidence) != 1 {
		return fmt.Errorf("want one piece of evidence, got %d", len(evidence))
	}
	e := evidence[0]
	if e.Source != domain.EvidenceSourceSelfAssessed || string(e.Rating) != r || e.TempoBPM == nil || *e.TempoBPM != bpm {
		return fmt.Errorf("evidence is %s rated %q at %v BPM, want self_assessed rated %q at %d BPM",
			e.Source, e.Rating, e.TempoBPM, r, bpm)
	}
	if e.GraderID != "self_rating.v1" {
		return fmt.Errorf("evidence was graded by %q, want self_rating.v1", e.GraderID)
	}
	return nil
}

func (w *world) hasNoEvidenceForPlayAlong(student, diagram string) error {
	evidence, err := w.evidenceFor(student, w.playAlongKey(diagram))
	if err != nil {
		return err
	}
	if len(evidence) != 0 {
		return fmt.Errorf("want no evidence, got %d", len(evidence))
	}
	return nil
}

func (w *world) answerRejectedBecause(reason domain.GradeRejection) func() error {
	return func() error {
		if reasons := w.logs.rejectionReasons(); !slices.Contains(reasons, string(reason)) {
			return fmt.Errorf("want a rejection for %s, got %v", reason, reasons)
		}
		return nil
	}
}

func (w *world) lastFold() (domain.ItemFold, error) {
	return w.fold(w.lastStudent, w.lastItemKey)
}

func (w *world) itemStaysInBox(_ string, box int) error {
	after, err := w.lastFold()
	if err != nil {
		return err
	}
	if w.before.Box != box || after.Box != box {
		return fmt.Errorf("box went from %d to %d, want it to stay in %d", w.before.Box, after.Box, box)
	}
	return nil
}

func (w *world) takeIsNotCountedAsAMiss() error {
	after, err := w.lastFold()
	if err != nil {
		return err
	}
	if after.Counted != w.before.Counted || after.Accuracy != w.before.Accuracy || after.Box != w.before.Box {
		return fmt.Errorf("the take counted: counted %d → %d, accuracy %.2f → %.2f, box %d → %d",
			w.before.Counted, after.Counted, w.before.Accuracy, after.Accuracy, w.before.Box, after.Box)
	}
	return nil
}

func (w *world) takeIsCountedAsAMiss() error {
	after, err := w.lastFold()
	if err != nil {
		return err
	}
	if after.Counted != w.before.Counted+1 || after.Accuracy >= w.before.Accuracy || after.Box != 1 {
		return fmt.Errorf("the take wasn't a miss: counted %d → %d, accuracy %.2f → %.2f, box %d",
			w.before.Counted, after.Counted, w.before.Accuracy, after.Accuracy, after.Box)
	}
	return nil
}
