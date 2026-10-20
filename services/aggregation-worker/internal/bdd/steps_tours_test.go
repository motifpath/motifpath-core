//go:build integration

package bdd

import (
	"fmt"

	"github.com/cucumber/godog"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

// A progression key tour take is rated like a play-along, and also names the key
// it was played in.
func registerTourSteps(sc *godog.ScenarioContext, w *world) {
	// ── Grading ────────────────────────────────────────────────────────────────
	sc.Step(`^"([^"]*)" rates a take of the progression key tour of "([^"]*)" on guitar as "([^"]*)" at (\d+) BPM in ([A-G][b#]?)$`, w.ratesATourTake)
	sc.Step(`^"([^"]*)" rates a take of the progression key tour of "([^"]*)" on guitar as "([^"]*)" at (\d+) BPM without a key$`, w.ratesATourTakeWithoutAKey)
	sc.Step(`^"([^"]*)" has self-assessed evidence for that tour rated "([^"]*)" at (\d+) BPM in ([A-G][b#]?)$`, w.hasEvidenceForTheTour)
	sc.Step(`^"([^"]*)" has no evidence for that tour$`, w.hasNoEvidenceForTheTour)
	sc.Step(`^"([^"]*)" rates a take of play-along "([^"]*)" as "([^"]*)" at (\d+) BPM in ([A-G][b#]?)$`, w.ratesAPlayAlongTakeInAKey)

	// ── Knowledge state ────────────────────────────────────────────────────────
	sc.Step(`^"([^"]*)" practises the progression key tour of "([^"]*)" on guitar$`, w.practisesTheTour)
	sc.Step(`^"([^"]*)" rates (\d+) takes of it "([^"]*)" at (\d+) BPM on (\d+) different days, each in another key$`, w.ratesTourTakesOnDifferentDays)
	sc.Step(`^"([^"]*)"'s level for the progression key tour is "([^"]*)"$`, w.levelForTheTourIs)
}

// tourOnGuitar is the tour of progression on the scenario's guitar, whose layout
// exists in the reference snapshot from the moment a scenario names it.
func (w *world) tourOnGuitar(progression string) string {
	id := stableUUID("instrument", "guitar")
	if _, ok := w.reference.instruments[id]; !ok {
		w.reference.instruments[id] = domain.InstrumentReference{ID: id, Tuning: standardGuitar}
	}
	w.tourKey = "progression_keys:" + id + ":" + progression
	return w.tourKey
}

func (w *world) rateTourTake(student, itemKey, r string, bpm int, key string) error {
	rt, err := rating(r)
	if err != nil {
		return err
	}
	return w.answer(student, itemKey, domain.PracticeResponse{
		Type: domain.PracticeResponseSelfRating, Rating: rt, TempoBPM: &bpm, MusicKey: key,
	})
}

func (w *world) ratesATourTake(student, progression, r string, bpm int, key string) error {
	return w.rateTourTake(student, w.tourOnGuitar(progression), r, bpm, key)
}

func (w *world) ratesATourTakeWithoutAKey(student, progression, r string, bpm int) error {
	return w.rateTourTake(student, w.tourOnGuitar(progression), r, bpm, "")
}

func (w *world) ratesAPlayAlongTakeInAKey(student, diagram, r string, bpm int, key string) error {
	return w.rateTourTake(student, w.playAlongKey(diagram), r, bpm, key)
}

func (w *world) hasEvidenceForTheTour(student, r string, bpm int, key string) error {
	evidence, err := w.evidenceFor(student, w.tourKey)
	if err != nil {
		return err
	}
	if len(evidence) != 1 {
		return fmt.Errorf("want one piece of evidence, got %d", len(evidence))
	}
	e := evidence[0]
	if e.Source != domain.EvidenceSourceSelfAssessed || string(e.Rating) != r || e.TempoBPM == nil || *e.TempoBPM != bpm || e.MusicKey != key {
		return fmt.Errorf("evidence is %s rated %q at %v BPM in %q, want self_assessed rated %q at %d BPM in %q",
			e.Source, e.Rating, e.TempoBPM, e.MusicKey, r, bpm, key)
	}
	return nil
}

func (w *world) hasNoEvidenceForTheTour(student string) error {
	evidence, err := w.evidenceFor(student, w.tourKey)
	if err != nil {
		return err
	}
	if len(evidence) != 0 {
		return fmt.Errorf("want no evidence, got %d", len(evidence))
	}
	return nil
}

func (w *world) practisesTheTour(_, progression string) error {
	w.tourOnGuitar(progression)
	return nil
}

// ratesTourTakesOnDifferentDays rates one take a day, each in the next key of
// the tour.
func (w *world) ratesTourTakesOnDifferentDays(student string, takes int, r string, bpm, days int) error {
	if takes != days {
		return fmt.Errorf("one take a day: %d takes on %d days", takes, days)
	}
	for i := range takes {
		key := domain.ProgressionKeyOrder[i%len(domain.ProgressionKeyOrder)]
		if err := w.rateTourTake(student, w.tourKey, r, bpm, key); err != nil {
			return err
		}
		w.clock = w.clock.AddDate(0, 0, 1)
	}
	return nil
}

func (w *world) levelForTheTourIs(student, level string) error {
	return w.levelIs(student, w.tourKey, level)
}
