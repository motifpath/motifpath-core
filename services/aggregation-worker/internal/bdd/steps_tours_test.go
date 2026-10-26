//go:build integration

package bdd

import (
	"context"
	"fmt"
	"slices"

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

	// ── The tour across sessions ───────────────────────────────────────────────
	// The worker folds the tour; where the next session starts is read off the
	// fold the way core plans it: the first key of the cycle not yet clean, at
	// the tour tempo.
	sc.Step(`^student "([^"]*)" plays "guitar"$`, w.studentPlaysGuitar)
	sc.Step(`^"([^"]*)" has never played the tour$`, w.hasNeverPlayedTheTour)
	sc.Step(`^the tour's tempo is (\d+) BPM$`, w.theToursTempoIs)
	sc.Step(`^"([^"]*)" rated takes of the tour clean in ([A-G][b#]?) and in ([A-G][b#]?) at (\d+) BPM$`, w.ratedTwoCleanTourTakes)
	sc.Step(`^"([^"]*)" rated a take of the tour (clean|almost|struggled) in ([A-G][b#]?) at (\d+) BPM$`, w.ratedATourTake)
	sc.Step(`^"([^"]*)" rated a take of the tour clean in ([A-G][b#]?) at (\d+) BPM in ([a-z_]+)$`, w.ratedACleanTourTakeInNoteValue)
	sc.Step(`^"([^"]*)" rated takes of the tour clean in all 12 keys at (\d+) BPM$`, w.ratedEveryKeyClean)
	sc.Step(`^the tour is picked for "([^"]*)"'s session$`, w.theTourIsPicked)
	sc.Step(`^it starts in ([A-G][b#]?) at a tour tempo of (\d+) BPM$`, w.tourStartsIn)
	sc.Step(`^it starts in ([A-G][b#]?) at a tour tempo of (\d+) BPM in quarters, with a target of (\d+) BPM$`, w.tourStartsInQuartersWithTarget)
	sc.Step(`^no key of the tour is clean$`, w.noKeyIsClean)
	sc.Step(`^the keys ([A-G][b#]?) and ([A-G][b#]?) are clean$`, w.theKeysAreClean)
	sc.Step(`^its start note value is ([a-z_]+)$`, w.startNoteValueIs)
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

// tourFirstTempoBPM is a tour never played's tempo: 60% of the target.
const tourFirstTempoBPM = domain.ProgressionKeysTargetTempoBPM * 6 / 10

func (w *world) studentPlaysGuitar(string) error {
	w.tourOnGuitar("I-V-vi-IV")
	return nil
}

func (w *world) hasNeverPlayedTheTour(string) error { return nil }

// theToursTempoIs leaves the tour at bpm, with no key clean, as earlier takes
// would have.
func (w *world) theToursTempoIs(bpm int) error {
	return w.states.Put(context.Background(), w.studentID("alice"), w.tourKey, domain.ItemFold{TourTempoBPM: &bpm})
}

func (w *world) ratedTwoCleanTourTakes(student, first, second string, bpm int) error {
	if err := w.rateTourTake(student, w.tourKey, "clean", bpm, first); err != nil {
		return err
	}
	return w.rateTourTake(student, w.tourKey, "clean", bpm, second)
}

func (w *world) ratedATourTake(student, r, key string, bpm int) error {
	return w.rateTourTake(student, w.tourKey, r, bpm, key)
}

func (w *world) ratedACleanTourTakeInNoteValue(student, key string, bpm int, noteValue string) error {
	v := domain.NoteValue(noteValue)
	return w.answer(student, w.tourKey, domain.PracticeResponse{
		Type: domain.PracticeResponseSelfRating, Rating: domain.SelfRatingClean, TempoBPM: &bpm, NoteValue: &v, MusicKey: key,
	})
}

func (w *world) ratedEveryKeyClean(student string, bpm int) error {
	for _, key := range domain.ProgressionKeyOrder {
		if err := w.rateTourTake(student, w.tourKey, "clean", bpm, key); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) theTourIsPicked(student string) error {
	w.tourStudent = student
	return nil
}

// tourStart is where the next session starts the tour: the first key of the
// cycle not yet clean and the tour's tempo.
func (w *world) tourStart() (string, int, domain.ItemFold, error) {
	fold, err := w.fold(w.tourStudent, w.tourKey)
	if err != nil {
		return "", 0, fold, err
	}
	tempo := tourFirstTempoBPM
	if fold.TourTempoBPM != nil {
		tempo = *fold.TourTempoBPM
	}
	for _, key := range domain.ProgressionKeyOrder {
		if !slices.Contains(fold.TourCleanKeys, key) {
			return key, tempo, fold, nil
		}
	}
	return domain.ProgressionKeyOrder[0], tempo, fold, nil
}

func (w *world) tourStartsIn(key string, bpm int) error {
	start, tempo, _, err := w.tourStart()
	if err != nil {
		return err
	}
	if start != key || tempo != bpm {
		return fmt.Errorf("the tour starts in %s at %d BPM, want %s at %d BPM", start, tempo, key, bpm)
	}
	return nil
}

func (w *world) tourStartsInQuartersWithTarget(key string, bpm, target int) error {
	if target != domain.ProgressionKeysTargetTempoBPM {
		return fmt.Errorf("the target is %d BPM, want %d", domain.ProgressionKeysTargetTempoBPM, target)
	}
	if err := w.startNoteValueIs(string(domain.NoteValueQuarters)); err != nil {
		return err
	}
	return w.tourStartsIn(key, bpm)
}

func (w *world) noKeyIsClean() error {
	_, _, fold, err := w.tourStart()
	if err != nil {
		return err
	}
	if len(fold.TourCleanKeys) != 0 {
		return fmt.Errorf("the clean keys are %v, want none", fold.TourCleanKeys)
	}
	return nil
}

func (w *world) theKeysAreClean(first, second string) error {
	_, _, fold, err := w.tourStart()
	if err != nil {
		return err
	}
	if !slices.Equal(fold.TourCleanKeys, []string{first, second}) {
		return fmt.Errorf("the clean keys are %v, want %s and %s", fold.TourCleanKeys, first, second)
	}
	return nil
}

// startNoteValueIs reads the note value of the last clean take; quarters
// before any.
func (w *world) startNoteValueIs(noteValue string) error {
	_, _, fold, err := w.tourStart()
	if err != nil {
		return err
	}
	got := domain.NoteValueQuarters
	if fold.TourNoteValue != nil {
		got = *fold.TourNoteValue
	}
	if string(got) != noteValue {
		return fmt.Errorf("the start note value is %s, want %s", got, noteValue)
	}
	return nil
}
