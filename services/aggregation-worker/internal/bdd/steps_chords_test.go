//go:build integration

package bdd

import (
	"context"
	"fmt"

	"github.com/cucumber/godog"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

// A chord change is a take of ten cycles at a tempo, one change on every beat:
// the student picks the share of changes that rang clean, and the grader rates
// the take by it. A minute counted before takes reads as such a share. A single
// chord is rated by the student alone; a bass root and fifth keeps its tempo.
func registerChordSteps(sc *godog.ScenarioContext, w *world) {
	// ── Grading ────────────────────────────────────────────────────────────────
	sc.Step(`^the chord voicing diagrams "([^"]*)" and "([^"]*)"$`, w.theChordVoicingDiagrams)
	sc.Step(`^"([^"]*)" sends a take of changes between "([^"]*)" and "([^"]*)" at (\d+) BPM with (\d+) percent clean$`, w.sendsATake)
	sc.Step(`^"([^"]*)" sends a take of changes between "([^"]*)" and "([^"]*)" rated "([^"]*)" at (\d+) BPM with (\d+) percent clean$`, w.sendsARatedTake)
	sc.Step(`^"([^"]*)" sends a take of changes between "([^"]*)" and "([^"]*)" with (\d+) percent clean and no tempo$`, w.sendsATakeWithoutATempo)
	sc.Step(`^"([^"]*)" sends a minute of changes between "([^"]*)" and "([^"]*)" at (\d+) BPM with (\d+) clean changes$`, w.sendsACountedMinute)
	sc.Step(`^"([^"]*)" has self-assessed evidence for that chord change rated "([^"]*)" at (\d+) BPM with (\d+) percent clean$`, w.hasEvidenceForTheChordChange)
	sc.Step(`^"([^"]*)" has no evidence for that chord change$`, w.hasNoEvidenceForTheChordChange)
	sc.Step(`^"([^"]*)" has evidence from before ten-cycle takes for the change between "([^"]*)" and "([^"]*)": (\d+) clean changes at (\d+) BPM$`, w.hasEvidenceFromBeforeTakes)
	sc.Step(`^the change's record is read$`, w.theChangesRecordIsRead)
	sc.Step(`^that minute reads as a take at (\d+) BPM with (\d+) percent clean$`, w.thatMinuteReadsAsATake)
	sc.Step(`^"([^"]*)" rates a take of the root and fifth of "([^"]*)" on the "([^"]*)" layout "([^"]*)" at (\d+) BPM$`, w.ratesARootAndFifth)
	sc.Step(`^"([^"]*)" rates a take of the root and fifth of "([^"]*)" on the "([^"]*)" layout "([^"]*)" with no tempo$`, w.ratesARootAndFifthWithoutATempo)
	sc.Step(`^"([^"]*)" has self-assessed evidence for that root and fifth rated "([^"]*)" at (\d+) BPM$`, w.hasEvidenceForTheRootAndFifth)
	sc.Step(`^"([^"]*)" has no evidence for that root and fifth$`, w.hasNoEvidenceForTheRootAndFifth)

	// ── Knowledge state ────────────────────────────────────────────────────────
	sc.Step(`^"([^"]*)" practises the chord change "([^"]*)" to "([^"]*)"$`, w.practisesTheChordChange)
	sc.Step(`^"([^"]*)" plays (\d+) takes of it on (\d+) different days, each at (\d+) BPM with every change clean$`, w.playsCleanTakesOnDifferentDays)
	sc.Step(`^"([^"]*)"'s level for the chord change is "([^"]*)"$`, w.levelForTheChordChangeIs)
	sc.Step(`^"([^"]*)"'s best clean tempo on the chord change "([^"]*)" to "([^"]*)" is (\d+) BPM$`, w.bestCleanChangeTempoIs)
	sc.Step(`^"([^"]*)" plays a take of it at (\d+) BPM with (\d+) percent of the changes clean$`, w.playsATakeOfIt)
	sc.Step(`^"([^"]*)" practises the single chord "([^"]*)"$`, w.practisesTheSingleChord)
	sc.Step(`^"([^"]*)" rates (\d+) takes of it "([^"]*)" on (\d+) different days$`, w.ratesTakesOfTheSingleChordOnDifferentDays)
	sc.Step(`^"([^"]*)"'s level for the single chord is "([^"]*)"$`, w.levelForTheSingleChordIs)
}

// chordDiagram is a chord voicing diagram, which exists from the moment a
// scenario names it.
func (w *world) chordDiagram(name string) string {
	id := stableUUID("diagram", name)
	if _, ok := w.reference.diagrams[id]; !ok {
		w.reference.diagrams[id] = domain.DiagramReference{ID: id}
	}
	return id
}

func (w *world) chordChangeKey(from, to string) string {
	return "chord_change:" + w.chordDiagram(from) + ":" + w.chordDiagram(to)
}

func (w *world) theChordVoicingDiagrams(from, to string) error {
	w.chords.changeKey = w.chordChangeKey(from, to)
	return nil
}

func (w *world) sendTake(student, itemKey string, r domain.SelfRating, tempo *int, share int) error {
	return w.answer(student, itemKey, domain.PracticeResponse{
		Type: domain.PracticeResponseSelfRating, Rating: r, TempoBPM: tempo, CleanSharePercent: &share,
	})
}

// sendsATake sends the rating a client derives from the share, as the Practice
// Shell does; the grader derives its own either way.
func (w *world) sendsATake(student, from, to string, bpm, share int) error {
	w.chords.changeKey = w.chordChangeKey(from, to)
	return w.sendTake(student, w.chords.changeKey, domain.ChangeShareRating(share), &bpm, share)
}

func (w *world) sendsARatedTake(student, from, to, r string, bpm, share int) error {
	rt, err := rating(r)
	if err != nil {
		return err
	}
	w.chords.changeKey = w.chordChangeKey(from, to)
	return w.sendTake(student, w.chords.changeKey, rt, &bpm, share)
}

func (w *world) sendsATakeWithoutATempo(student, from, to string, share int) error {
	w.chords.changeKey = w.chordChangeKey(from, to)
	return w.sendTake(student, w.chords.changeKey, domain.ChangeShareRating(share), nil, share)
}

// sendsACountedMinute sends a minute counted the way apps did before takes.
func (w *world) sendsACountedMinute(student, from, to string, bpm, count int) error {
	w.chords.changeKey = w.chordChangeKey(from, to)
	return w.answer(student, w.chords.changeKey, domain.PracticeResponse{
		Type: domain.PracticeResponseSelfRating, Rating: domain.SelfRatingClean, TempoBPM: &bpm, ChangesPerMinute: &count,
	})
}

func (w *world) hasEvidenceForTheChordChange(student, r string, bpm, share int) error {
	evidence, err := w.evidenceFor(student, w.chords.changeKey)
	if err != nil {
		return err
	}
	if len(evidence) != 1 {
		return fmt.Errorf("want one piece of evidence, got %d", len(evidence))
	}
	e := evidence[0]
	if e.Source != domain.EvidenceSourceSelfAssessed || string(e.Rating) != r || e.TempoBPM == nil || *e.TempoBPM != bpm ||
		e.CleanSharePercent == nil || *e.CleanSharePercent != share {
		return fmt.Errorf("evidence is %s rated %q at %v BPM with %v percent, want self_assessed rated %q at %d BPM with %d",
			e.Source, e.Rating, e.TempoBPM, e.CleanSharePercent, r, bpm, share)
	}
	return nil
}

// hasEvidenceFromBeforeTakes stores a minute the way the grader of its time kept
// it: rated from its count by that grader's rule, clean from 90 percent.
func (w *world) hasEvidenceFromBeforeTakes(student, from, to string, count, bpm int) error {
	w.chords.changeKey = w.chordChangeKey(from, to)
	w.lastStudent = student
	r := domain.SelfRatingStruggled
	switch {
	case count*10 >= bpm*9:
		r = domain.SelfRatingClean
	case count*10 >= bpm*6:
		r = domain.SelfRatingAlmost
	}
	w.eventNo++
	_, err := w.evidence.Insert(context.Background(), domain.PracticeEvidence{
		EvidenceID: fmt.Sprintf("e0000000-0000-4000-8000-%012d", w.eventNo), StudentID: w.studentID(student),
		ItemKey: w.chords.changeKey, Source: domain.EvidenceSourceSelfAssessed, OccurredAt: w.clock, GraderID: "self_rating.v1",
		Response: domain.PracticeResponse{Type: domain.PracticeResponseSelfRating, Rating: r, TempoBPM: &bpm, ChangesPerMinute: &count},
		Rating:   r, TempoBPM: &bpm, ChangesPerMinute: &count,
	})
	return err
}

// theChangesRecordIsRead folds the change's evidence under the current rules, as
// the worker does when a fold is stale.
func (w *world) theChangesRecordIsRead() error {
	evidence, err := w.evidenceFor(w.lastStudent, w.chords.changeKey)
	if err != nil {
		return err
	}
	targets, err := w.reference.ChangeTargets(context.Background())
	if err != nil {
		return err
	}
	fold, err := domain.RebuildFold(evidence, domain.ItemGoal{ChangeTargets: targets})
	if err != nil {
		return err
	}
	w.chords.record = fold
	w.chords.recordEvidence = evidence
	return nil
}

func (w *world) thatMinuteReadsAsATake(bpm, share int) error {
	if len(w.chords.recordEvidence) != 1 {
		return fmt.Errorf("want one minute, got %d", len(w.chords.recordEvidence))
	}
	e := w.chords.recordEvidence[0]
	if got := domain.LegacyCleanShare(*e.ChangesPerMinute, *e.TempoBPM); got != share {
		return fmt.Errorf("the minute reads as %d percent, want %d", got, share)
	}
	f := w.chords.record
	if f.LastChangeTempoBPM == nil || *f.LastChangeTempoBPM != bpm || f.LastRating != domain.ChangeShareRating(share) {
		return fmt.Errorf("the record's last take is %v BPM rated %q, want %d BPM rated %q",
			f.LastChangeTempoBPM, f.LastRating, bpm, domain.ChangeShareRating(share))
	}
	return nil
}

// rootFifthKey names a root and fifth on a bass layout, which exists in the
// reference snapshot from the moment a scenario names it.
func (w *world) rootFifthKey(chord, layout string) string {
	id := stableUUID("instrument", layout)
	if _, ok := w.reference.instruments[id]; !ok {
		w.reference.instruments[id] = domain.InstrumentReference{ID: id, Tuning: []string{"E1", "A1", "D2", "G2"}}
	}
	w.chords.rootFifthKey = "root_fifth:" + id + ":" + chord
	return w.chords.rootFifthKey
}

func (w *world) ratesARootAndFifth(student, chord, layout, r string, bpm int) error {
	rt, err := rating(r)
	if err != nil {
		return err
	}
	return w.answer(student, w.rootFifthKey(chord, layout), domain.PracticeResponse{Type: domain.PracticeResponseSelfRating, Rating: rt, TempoBPM: &bpm})
}

func (w *world) ratesARootAndFifthWithoutATempo(student, chord, layout, r string) error {
	rt, err := rating(r)
	if err != nil {
		return err
	}
	return w.answer(student, w.rootFifthKey(chord, layout), domain.PracticeResponse{Type: domain.PracticeResponseSelfRating, Rating: rt})
}

func (w *world) hasEvidenceForTheRootAndFifth(student, r string, bpm int) error {
	evidence, err := w.evidenceFor(student, w.chords.rootFifthKey)
	if err != nil {
		return err
	}
	if len(evidence) != 1 {
		return fmt.Errorf("want one piece of evidence, got %d", len(evidence))
	}
	e := evidence[0]
	if e.Source != domain.EvidenceSourceSelfAssessed || string(e.Rating) != r || e.TempoBPM == nil || *e.TempoBPM != bpm {
		return fmt.Errorf("evidence is %s rated %q at %v BPM, want self_assessed rated %q at %d BPM", e.Source, e.Rating, e.TempoBPM, r, bpm)
	}
	return nil
}

func (w *world) hasNoEvidenceForTheRootAndFifth(student string) error {
	evidence, err := w.evidenceFor(student, w.chords.rootFifthKey)
	if err != nil {
		return err
	}
	if len(evidence) != 0 {
		return fmt.Errorf("want no evidence, got %d", len(evidence))
	}
	return nil
}

func (w *world) hasNoEvidenceForTheChordChange(student string) error {
	evidence, err := w.evidenceFor(student, w.chords.changeKey)
	if err != nil {
		return err
	}
	if len(evidence) != 0 {
		return fmt.Errorf("want no evidence, got %d", len(evidence))
	}
	return nil
}

func (w *world) practisesTheChordChange(_, from, to string) error {
	w.chords.changeKey = w.chordChangeKey(from, to)
	return nil
}

func (w *world) playsCleanTakesOnDifferentDays(student string, takes, days, bpm int) error {
	if takes != days {
		return fmt.Errorf("one take a day: %d takes on %d days", takes, days)
	}
	for range takes {
		if err := w.sendTake(student, w.chords.changeKey, domain.SelfRatingClean, &bpm, 100); err != nil {
			return err
		}
		w.clock = w.clock.AddDate(0, 0, 1)
	}
	return nil
}

func (w *world) levelIs(student, itemKey, level string) error {
	fold, err := w.fold(student, itemKey)
	if err != nil {
		return err
	}
	if got := fold.Level(); string(got) != level {
		return fmt.Errorf("the level is %q, want %q", got, level)
	}
	return nil
}

func (w *world) levelForTheChordChangeIs(student, level string) error {
	return w.levelIs(student, w.chords.changeKey, level)
}

// bestCleanChangeTempoIs plays a take where every change rang clean at bpm, so it
// becomes the best clean tempo.
func (w *world) bestCleanChangeTempoIs(student, from, to string, bpm int) error {
	w.chords.changeKey = w.chordChangeKey(from, to)
	if err := w.sendTake(student, w.chords.changeKey, domain.SelfRatingClean, &bpm, 100); err != nil {
		return err
	}
	fold, err := w.fold(student, w.chords.changeKey)
	if err != nil {
		return err
	}
	if fold.BestCleanBPM == nil || *fold.BestCleanBPM != bpm {
		return fmt.Errorf("best clean tempo is %v, want %d", fold.BestCleanBPM, bpm)
	}
	return nil
}

func (w *world) playsATakeOfIt(student string, bpm, share int) error {
	return w.sendTake(student, w.chords.changeKey, domain.ChangeShareRating(share), &bpm, share)
}

func (w *world) practisesTheSingleChord(_, voicing string) error {
	w.chords.voicingKey = "chord_voicing:" + w.chordDiagram(voicing)
	return nil
}

func (w *world) ratesTakesOfTheSingleChordOnDifferentDays(student string, takes int, r string, days int) error {
	if takes != days {
		return fmt.Errorf("one take a day: %d takes on %d days", takes, days)
	}
	rt, err := rating(r)
	if err != nil {
		return err
	}
	for range takes {
		if err := w.answer(student, w.chords.voicingKey, domain.PracticeResponse{Type: domain.PracticeResponseSelfRating, Rating: rt}); err != nil {
			return err
		}
		w.clock = w.clock.AddDate(0, 0, 1)
	}
	return nil
}

func (w *world) levelForTheSingleChordIs(student, level string) error {
	return w.levelIs(student, w.chords.voicingKey, level)
}
