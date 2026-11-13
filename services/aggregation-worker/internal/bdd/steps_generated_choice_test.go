//go:build integration

package bdd

import (
	"fmt"
	"regexp"

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
	sc.Step(`^"([^"]*)" answers the chart progression "([^"]*)" in (\S+), asked (\w+), by picking "([^"]*)" among ((?:"[^"]*"(?:, | and )?)+)$`, w.answersTheChartProgression)
	sc.Step(`^"([^"]*)" has auto-graded evidence for that question that is (right|wrong)$`, w.hasEvidenceForThatQuestion)
	sc.Step(`^"([^"]*)" answers the heard chord quality "([^"]*)", asked (\w+), by picking "([^"]*)" among "([^"]*)", "([^"]*)" and "([^"]*)"$`, w.answersTheHeardChordQuality)
	sc.Step(`^"([^"]*)" answers the heard chord quality "([^"]*)" in (\S+), asked (\w+), by picking "([^"]*)" among "([^"]*)", "([^"]*)" and "([^"]*)"$`, w.answersTheHeardChordQualityInAKey)
}

// answersTheHeardChordQuality answers a chord's quality heard alone, in no key.
func (w *world) answersTheHeardChordQuality(student, quality, way, choice, o1, o2, o3 string) error {
	return w.answer(student, "heard_chord_quality:"+quality, generatedChoice(way, "", choice, o1, o2, o3))
}

// answersTheHeardChordQualityInAKey answers a chord quality with a key, which a
// chord heard alone never has.
func (w *world) answersTheHeardChordQualityInAKey(student, quality, key, way, choice, o1, o2, o3 string) error {
	return w.answer(student, "heard_chord_quality:"+quality, generatedChoice(way, key, choice, o1, o2, o3))
}

// answersTheChartProgression answers a chart progression, asked in any key,
// among options that may include made-up progressions.
func (w *world) answersTheChartProgression(student, progression, key, way, choice, options string) error {
	var offered []string
	for _, m := range quotedOption.FindAllStringSubmatch(options, -1) {
		offered = append(offered, m[1])
	}
	return w.answer(student, "chart_progression:"+progression, generatedChoice(way, key, choice, offered...))
}

var quotedOption = regexp.MustCompile(`"([^"]*)"`)

func (w *world) hasEvidenceForThatQuestion(student, verdict string) error {
	evidence, err := w.evidenceFor(student, w.lastItemKey)
	if err != nil {
		return err
	}
	if len(evidence) == 0 {
		return fmt.Errorf("no evidence for %s", w.lastItemKey)
	}
	e := evidence[len(evidence)-1]
	if e.Source != domain.EvidenceSourceAutoGraded || e.Correct == nil || *e.Correct != (verdict == "right") {
		return fmt.Errorf("evidence is %s, correct %v, want auto_graded and %s", e.Source, e.Correct, verdict)
	}
	return nil
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
