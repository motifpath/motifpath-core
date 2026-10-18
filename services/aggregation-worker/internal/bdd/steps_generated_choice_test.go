//go:build integration

package bdd

import (
	"fmt"

	"github.com/cucumber/godog"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

// generatedChoiceLatencyMs is the time to answer a generated question, where a
// scenario doesn't say it.
const generatedChoiceLatencyMs = 2000

func registerGeneratedChoiceSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]*)" answers the heard chord (\w+) in (\S+), asked (\w+), by picking "([^"]*)" among "([^"]*)", "([^"]*)", "([^"]*)" and "([^"]*)"$`, w.answersTheHeardChord)
	sc.Step(`^"([^"]*)" answers the diatonic chord (\w+) in (\S+), asked (\w+), by picking "([^"]*)" among "([^"]*)", "([^"]*)", "([^"]*)" and "([^"]*)"$`, w.answersTheDiatonicChord)
	sc.Step(`^"([^"]*)" has no evidence for that question$`, w.hasNoEvidenceForThatQuestion)
}

// answersTheHeardChord answers a heard chord, asked in any key: the key comes with
// the answer.
func (w *world) answersTheHeardChord(student, numeral, key, way, choice, o1, o2, o3, o4 string) error {
	return w.answer(student, "heard_chord:"+numeral, generatedChoice(way, key, choice, o1, o2, o3, o4))
}

// answersTheDiatonicChord answers a diatonic chord, whose item fixes its key.
func (w *world) answersTheDiatonicChord(student, numeral, key, way, choice, o1, o2, o3, o4 string) error {
	return w.answer(student, "diatonic_chord:"+key+":"+numeral, generatedChoice(way, "", choice, o1, o2, o3, o4))
}

func generatedChoice(way, musicKey, choice string, options ...string) domain.PracticeResponse {
	latency := generatedChoiceLatencyMs
	return domain.PracticeResponse{
		Type: domain.PracticeResponseGeneratedChoice, Way: way, MusicKey: musicKey,
		Options: options, Choice: choice, LatencyMs: &latency,
	}
}

func (w *world) hasNoEvidenceForThatQuestion(student string) error {
	evidence, err := w.evidenceFor(student, w.lastItemKey)
	if err != nil {
		return err
	}
	if len(evidence) != 0 {
		return fmt.Errorf("want no evidence for %s, got %d", w.lastItemKey, len(evidence))
	}
	return nil
}
