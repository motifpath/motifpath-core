//go:build integration

package bdd

import (
	"fmt"

	"github.com/cucumber/godog"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

// A chord change is a minute at a tempo, one change on every beat: the student
// counts the changes that rang clean, and the grader rates the minute by the
// share of its beats that did. A single chord is rated by the student alone.
func registerChordSteps(sc *godog.ScenarioContext, w *world) {
	// ── Grading ────────────────────────────────────────────────────────────────
	sc.Step(`^the chord voicing diagrams "([^"]*)" and "([^"]*)"$`, w.theChordVoicingDiagrams)
	sc.Step(`^"([^"]*)" sends a minute of changes between "([^"]*)" and "([^"]*)" at (\d+) BPM with (\d+) clean changes$`, w.sendsAMinute)
	sc.Step(`^"([^"]*)" sends a minute of changes between "([^"]*)" and "([^"]*)" rated "([^"]*)" at (\d+) BPM with (\d+) clean changes$`, w.sendsARatedMinute)
	sc.Step(`^"([^"]*)" sends a minute of changes between "([^"]*)" and "([^"]*)" with (\d+) clean changes and no tempo$`, w.sendsAMinuteWithoutATempo)
	sc.Step(`^"([^"]*)" has self-assessed evidence for that chord change rated "([^"]*)" at (\d+) BPM with (\d+) clean changes$`, w.hasEvidenceForTheChordChange)
	sc.Step(`^"([^"]*)" has no evidence for that chord change$`, w.hasNoEvidenceForTheChordChange)

	// ── Knowledge state ────────────────────────────────────────────────────────
	sc.Step(`^"([^"]*)" practises the chord change "([^"]*)" to "([^"]*)"$`, w.practisesTheChordChange)
	sc.Step(`^"([^"]*)" plays (\d+) clean minutes of it on (\d+) different days, each (\d+) clean changes at (\d+) BPM$`, w.playsCleanMinutesOnDifferentDays)
	sc.Step(`^"([^"]*)"'s level for the chord change is "([^"]*)"$`, w.levelForTheChordChangeIs)
	sc.Step(`^"([^"]*)"'s best clean count on the chord change "([^"]*)" to "([^"]*)" is (\d+)$`, w.bestCleanCountIs)
	sc.Step(`^"([^"]*)" plays a minute of it with (\d+) clean changes at (\d+) BPM$`, w.playsAMinuteOfIt)
	sc.Step(`^the minute is not counted as a miss$`, w.takeIsNotCountedAsAMiss)
	sc.Step(`^the minute is counted as a miss$`, w.takeIsCountedAsAMiss)
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

func (w *world) sendMinute(student, itemKey string, r domain.SelfRating, tempo *int, count int) error {
	return w.answer(student, itemKey, domain.PracticeResponse{
		Type: domain.PracticeResponseSelfRating, Rating: r, TempoBPM: tempo, ChangesPerMinute: &count,
	})
}

// sendsAMinute sends the rating a client derives from the count, as the Practice
// Shell does; the grader derives its own either way.
func (w *world) sendsAMinute(student, from, to string, bpm, count int) error {
	w.chords.changeKey = w.chordChangeKey(from, to)
	return w.sendMinute(student, w.chords.changeKey, domain.SelfRatingClean, &bpm, count)
}

func (w *world) sendsARatedMinute(student, from, to, r string, bpm, count int) error {
	rt, err := rating(r)
	if err != nil {
		return err
	}
	w.chords.changeKey = w.chordChangeKey(from, to)
	return w.sendMinute(student, w.chords.changeKey, rt, &bpm, count)
}

func (w *world) sendsAMinuteWithoutATempo(student, from, to string, count int) error {
	w.chords.changeKey = w.chordChangeKey(from, to)
	return w.sendMinute(student, w.chords.changeKey, domain.SelfRatingClean, nil, count)
}

func (w *world) hasEvidenceForTheChordChange(student, r string, bpm, count int) error {
	evidence, err := w.evidenceFor(student, w.chords.changeKey)
	if err != nil {
		return err
	}
	if len(evidence) != 1 {
		return fmt.Errorf("want one piece of evidence, got %d", len(evidence))
	}
	e := evidence[0]
	if e.Source != domain.EvidenceSourceSelfAssessed || string(e.Rating) != r || e.TempoBPM == nil || *e.TempoBPM != bpm ||
		e.ChangesPerMinute == nil || *e.ChangesPerMinute != count {
		return fmt.Errorf("evidence is %s rated %q at %v BPM with %v changes, want self_assessed rated %q at %d BPM with %d",
			e.Source, e.Rating, e.TempoBPM, e.ChangesPerMinute, r, bpm, count)
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

func (w *world) playsCleanMinutesOnDifferentDays(student string, minutes, days, count, bpm int) error {
	if minutes != days {
		return fmt.Errorf("one minute a day: %d minutes on %d days", minutes, days)
	}
	for range minutes {
		if err := w.sendMinute(student, w.chords.changeKey, domain.SelfRatingClean, &bpm, count); err != nil {
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

// bestCleanCountIs plays a minute where every beat changed cleanly, at a tempo of
// count, so the count becomes the best clean one.
func (w *world) bestCleanCountIs(student, from, to string, count int) error {
	w.chords.changeKey = w.chordChangeKey(from, to)
	if err := w.sendMinute(student, w.chords.changeKey, domain.SelfRatingClean, &count, count); err != nil {
		return err
	}
	fold, err := w.fold(student, w.chords.changeKey)
	if err != nil {
		return err
	}
	if fold.BestChangesPerMinute == nil || *fold.BestChangesPerMinute != count {
		return fmt.Errorf("best clean count is %v, want %d", fold.BestChangesPerMinute, count)
	}
	return nil
}

func (w *world) playsAMinuteOfIt(student string, count, bpm int) error {
	return w.sendMinute(student, w.chords.changeKey, domain.SelfRatingClean, &bpm, count)
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
