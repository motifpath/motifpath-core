//go:build integration

package bdd

import (
	"fmt"
	"strings"

	"github.com/cucumber/godog"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

// A guide-tone take taps the third of each chord of a progression in time with its
// playback, graded bar by bar at the take's tempo and note value; a right take's
// fluency is its effective speed over the guide-tone target.
func registerGuideToneSteps(sc *godog.ScenarioContext, w *world) {
	// ── Grading ────────────────────────────────────────────────────────────────
	sc.Step(`^"([^"]*)" taps the third of each bar of "([^"]*)" in ([A-G][b#]?) at (\d+) BPM within its bar$`, w.tapsEveryThird)
	sc.Step(`^"([^"]*)" taps the third of "([^"]*)" at (\d+) milliseconds and of "([^"]*)" at (\d+) milliseconds in a take of "([^"]*)" in ([A-G][b#]?) at (\d+) BPM in ([a-z_]+)$`, w.tapsTwoThirdsInNoteValue)
	sc.Step(`^"([^"]*)" has auto-graded evidence for that guide tone that is (right|wrong)$`, w.hasGuideToneEvidence)
	sc.Step(`^the evidence's answer key marks all (\d+) bars hit$`, w.answerKeyMarksAllBarsHit)
	sc.Step(`^the evidence's answer key marks only the first bar hit$`, w.answerKeyMarksOnlyTheFirstBarHit)

	// ── Knowledge state ────────────────────────────────────────────────────────
	sc.Step(`^"([^"]*)" practises the guide tone of "([^"]*)" on guitar$`, w.practisesTheGuideTone)
	sc.Step(`^"([^"]*)" makes (\d+) right takes of it at (\d+) BPM on (\d+) different days$`, w.makesRightTakes)
	sc.Step(`^"([^"]*)" makes (\d+) right takes of it at (\d+) BPM in ([a-z_]+) on (\d+) different days$`, w.makesRightTakesInNoteValue)
	sc.Step(`^"([^"]*)" makes (\d+) right takes of it at (\d+) BPM on (\d+) different days, each hitting a bar (\d+) milliseconds after its downbeat$`, w.makesLateRightTakes)
	sc.Step(`^"([^"]*)"'s level for the guide tone is "([^"]*)"$`, w.levelForTheGuideToneIs)
}

// guideProgressions are the progressions the guide-tone scenarios name, as the
// progression catalog has them.
var guideProgressions = map[string][]string{
	"ii-V-I":    {"ii", "V", "I", "I"},
	"I-V-vi-IV": {"I", "V", "vi", "IV"},
}

// guideToneOnGuitar is the guide tone of progression on the scenario's guitar; the
// layout and the progression exist in the reference snapshot once a scenario names them.
func (w *world) guideToneOnGuitar(progression string) (string, error) {
	bars, ok := guideProgressions[progression]
	if !ok {
		return "", fmt.Errorf("no guide-tone progression %q in these steps", progression)
	}
	id := stableUUID("instrument", "guitar")
	if _, ok := w.reference.instruments[id]; !ok {
		w.reference.instruments[id] = domain.InstrumentReference{ID: id, Tuning: standardGuitar}
	}
	w.reference.progressions[progression] = domain.ProgressionReference{Key: progression, Bars: bars}
	w.guideKey = "guide_tone:" + id + ":" + progression
	return w.guideKey, nil
}

var pitchClasses = map[string]int{"C": 0, "Db": 1, "D": 2, "Eb": 3, "E": 4, "F": 5, "F#": 6, "Gb": 6, "G": 7, "Ab": 8, "A": 9, "Bb": 10, "B": 11}

// numeralThirds are each numeral's root above the key and its third above the root.
var numeralThirds = map[string][2]int{"I": {0, 4}, "ii": {2, 3}, "iii": {4, 3}, "IV": {5, 4}, "V": {7, 4}, "vi": {9, 3}, "viio": {11, 3}}

// thirdOfNumeral is the pitch class of the third of numeral's chord in key.
func thirdOfNumeral(key, numeral string) (int, error) {
	k, ok := pitchClasses[key]
	n, known := numeralThirds[numeral]
	if !ok || !known {
		return 0, fmt.Errorf("no third for %s in %s", numeral, key)
	}
	return (k + n[0] + n[1]) % 12, nil
}

// thirdOfChord is the pitch class of a major or minor chord symbol's third.
func thirdOfChord(symbol string) (int, error) {
	root, third := symbol, 4
	if strings.HasSuffix(symbol, "m") {
		root, third = strings.TrimSuffix(symbol, "m"), 3
	}
	r, ok := pitchClasses[root]
	if !ok {
		return 0, fmt.Errorf("no chord %q", symbol)
	}
	return (r + third) % 12, nil
}

// guitarOpenStrings are standard tuning's open strings as pitch classes, string 1 first.
var guitarOpenStrings = []int{4, 11, 7, 2, 9, 4}

// guitarTap is a tap at atMs on the lowest fret, then the highest string, that sounds pitch.
func guitarTap(pitch, atMs int) domain.TimedTap {
	for fret := range 12 {
		for s, open := range guitarOpenStrings {
			if (open+fret)%12 == pitch {
				return domain.TimedTap{AtMs: atMs, String: s + 1, Fret: fret}
			}
		}
	}
	panic("every pitch class is on the guitar")
}

// barMs is how long a bar lasts at bpm in quarters.
func barMs(bpm int) int { return 4 * 60_000 / bpm }

// takeHittingEveryThird is a take of the guide tone tapping each bar's third offset
// milliseconds after the bar's downbeat, except the first bar, tapped at late.
func (w *world) takeHittingEveryThird(progression, key string, bpm, offset, late int, v *domain.NoteValue) (domain.PracticeResponse, error) {
	length := 4 * 60_000.0 / domain.EffectiveSpeed(bpm, v)
	var taps []domain.TimedTap
	for i, numeral := range guideProgressions[progression] {
		pitch, err := thirdOfNumeral(key, numeral)
		if err != nil {
			return domain.PracticeResponse{}, err
		}
		at := offset
		if i == 0 {
			at = late
		}
		taps = append(taps, guitarTap(pitch, int(float64(i)*length)+at))
	}
	return domain.PracticeResponse{Type: domain.PracticeResponseTimedTaps, MusicKey: key, Tone: "3", TempoBPM: &bpm, NoteValue: v, Taps: taps}, nil
}

func (w *world) tapsEveryThird(student, progression, key string, bpm int) error {
	itemKey, err := w.guideToneOnGuitar(progression)
	if err != nil {
		return err
	}
	r, err := w.takeHittingEveryThird(progression, key, bpm, 100, 100, nil)
	if err != nil {
		return err
	}
	return w.answer(student, itemKey, r)
}

func (w *world) tapsTwoThirdsInNoteValue(student, first string, firstMs int, second string, secondMs int, progression, key string, bpm int, noteValue string) error {
	itemKey, err := w.guideToneOnGuitar(progression)
	if err != nil {
		return err
	}
	var taps []domain.TimedTap
	for _, t := range []struct {
		chord string
		at    int
	}{{first, firstMs}, {second, secondMs}} {
		pitch, err := thirdOfChord(t.chord)
		if err != nil {
			return err
		}
		taps = append(taps, guitarTap(pitch, t.at))
	}
	v := domain.NoteValue(noteValue)
	return w.answer(student, itemKey, domain.PracticeResponse{
		Type: domain.PracticeResponseTimedTaps, MusicKey: key, Tone: "3", TempoBPM: &bpm, NoteValue: &v, Taps: taps,
	})
}

// guideToneEvidence is the scenario's one piece of evidence for its guide tone.
func (w *world) guideToneEvidence(student string) (domain.PracticeEvidence, error) {
	evidence, err := w.evidenceFor(student, w.guideKey)
	if err != nil {
		return domain.PracticeEvidence{}, err
	}
	if len(evidence) != 1 {
		return domain.PracticeEvidence{}, fmt.Errorf("want one piece of evidence, got %d", len(evidence))
	}
	return evidence[0], nil
}

func (w *world) hasGuideToneEvidence(student, verdict string) error {
	e, err := w.guideToneEvidence(student)
	if err != nil {
		return err
	}
	if e.Source != domain.EvidenceSourceAutoGraded || e.Correct == nil || *e.Correct != (verdict == "right") {
		return fmt.Errorf("evidence is %s, correct %v, want auto_graded and %s", e.Source, e.Correct, verdict)
	}
	return nil
}

// barsHit is which bars of the last guide-tone evidence's answer key were hit.
func (w *world) barsHit() ([]bool, error) {
	e, err := w.guideToneEvidence(w.lastStudent)
	if err != nil {
		return nil, err
	}
	if e.AnswerKey == nil {
		return nil, fmt.Errorf("the evidence keeps no answer key")
	}
	hits := make([]bool, len(e.AnswerKey.Bars))
	for i, b := range e.AnswerKey.Bars {
		hits[i] = b.Hit
	}
	return hits, nil
}

func (w *world) answerKeyMarksAllBarsHit(bars int) error {
	hits, err := w.barsHit()
	if err != nil {
		return err
	}
	if len(hits) != bars {
		return fmt.Errorf("the answer key has %d bars, want %d", len(hits), bars)
	}
	for i, hit := range hits {
		if !hit {
			return fmt.Errorf("bar %d is missed", i+1)
		}
	}
	return nil
}

func (w *world) answerKeyMarksOnlyTheFirstBarHit() error {
	hits, err := w.barsHit()
	if err != nil {
		return err
	}
	for i, hit := range hits {
		if hit != (i == 0) {
			return fmt.Errorf("bars hit are %v, want only the first", hits)
		}
	}
	return nil
}

func (w *world) practisesTheGuideTone(_, progression string) error {
	_, err := w.guideToneOnGuitar(progression)
	return err
}

// makeRightTakes makes one right take a day, in C, each bar's third tapped 100 ms
// after its downbeat but the first bar's, tapped at late.
func (w *world) makeRightTakes(student string, takes, bpm, days, late int, v *domain.NoteValue) error {
	if takes != days {
		return fmt.Errorf("one take a day: %d takes on %d days", takes, days)
	}
	progression := w.guideKey[strings.LastIndex(w.guideKey, ":")+1:]
	for range takes {
		r, err := w.takeHittingEveryThird(progression, "C", bpm, 100, late, v)
		if err != nil {
			return err
		}
		if err := w.answer(student, w.guideKey, r); err != nil {
			return err
		}
		w.clock = w.clock.AddDate(0, 0, 1)
	}
	return nil
}

func (w *world) makesRightTakes(student string, takes, bpm, days int) error {
	return w.makeRightTakes(student, takes, bpm, days, 100, nil)
}

func (w *world) makesRightTakesInNoteValue(student string, takes, bpm int, noteValue string, days int) error {
	v := domain.NoteValue(noteValue)
	return w.makeRightTakes(student, takes, bpm, days, 100, &v)
}

func (w *world) makesLateRightTakes(student string, takes, bpm, days, late int) error {
	if late >= barMs(bpm) {
		return fmt.Errorf("a hit %d ms late falls outside a %d ms bar", late, barMs(bpm))
	}
	return w.makeRightTakes(student, takes, bpm, days, late, nil)
}

func (w *world) levelForTheGuideToneIs(student, level string) error {
	return w.levelIs(student, w.guideKey, level)
}
