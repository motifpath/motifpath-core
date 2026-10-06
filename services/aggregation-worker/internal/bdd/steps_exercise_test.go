//go:build integration

package bdd

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

func registerExerciseSteps(sc *godog.ScenarioContext, w *world) {
	// ── Exercises and challenges ───────────────────────────────────────────────
	sc.Step(`^the exercise "([^"]*)" whose correct options are ((?:"[^"]*"(?:, | and )?)+) out of ((?:"[^"]*"(?:, | and )?)+)$`, w.theExercise)
	sc.Step(`^"([^"]*)" is taking the challenge "([^"]*)" of the content node "([^"]*)"$`, w.isTakingTheChallenge)
	sc.Step(`^"([^"]*)" answers exercise "([^"]*)" by selecting ((?:"[^"]*"(?:, | and )?)+)$`, w.answersBySelecting)
	sc.Step(`^"([^"]*)" answers exercise "([^"]*)" by selecting an option of another exercise$`, w.answersWithAnotherExercisesOption)
	sc.Step(`^"([^"]*)" answers the exercise "([^"]*)" in the challenge by selecting its correct option after (\d+) milliseconds$`, w.answersInTheChallenge)
	sc.Step(`^"([^"]*)" answers a listening exercise whose sound lasts (\d+) milliseconds by selecting its correct option after (\d+) milliseconds$`, w.answersAListeningExercise)
	sc.Step(`^"([^"]*)" took the challenge "([^"]*)" yesterday and answered "([^"]*)" wrong$`, w.tookTheChallengeYesterday)
	sc.Step(`^"([^"]*)" takes "([^"]*)" again and moves on from "([^"]*)" with its correct option selected$`, w.takesTheChallengeAgain)

	sc.Step(`^"([^"]*)" has auto-graded evidence for exercise "([^"]*)" that is (correct|wrong)$`, w.hasAutoGradedEvidenceForExercise)
	sc.Step(`^"([^"]*)" has auto-graded evidence for "([^"]*)" that is correct with a latency of (\d+) milliseconds$`, w.hasCorrectEvidenceWithLatency)
	sc.Step(`^the evidence names the challenge "([^"]*)" instead of a practice session$`, w.evidenceNamesTheChallenge)
	sc.Step(`^"([^"]*)" has auto-graded evidence for that exercise with a latency of (\d+) milliseconds and (\d+) milliseconds of audio$`, w.hasEvidenceWithAudio)
	sc.Step(`^the answer is rejected because the option is unknown$`, w.answerRejectedBecause(domain.GradeRejectionUnknownOption))
	sc.Step(`^"([^"]*)" has no evidence for exercise "([^"]*)"$`, w.hasNoEvidenceForExercise)
	sc.Step(`^"([^"]*)" has two pieces of evidence for "([^"]*)", the second one correct$`, w.hasTwoPiecesTheSecondCorrect)

	// ── Fluent times ───────────────────────────────────────────────────────────
	sc.Step(`^the drill template "([^"]*)" has version 1 (?:from source "[^"]*" )?with a fluent time of (\d+) milliseconds(?: net of tap time)?$`, w.templateHasVersion1)
	sc.Step(`^version 2 from source "[^"]*" with a fluent time of (\d+) milliseconds from (\d{4}-\d{2}-\d{2})$`, w.lastTemplateHasVersion2)
	sc.Step(`^student "([^"]*)" has a tap time of (\d+) milliseconds$`, w.hasATapTime)
	sc.Step(`^"([^"]*)" answers two different listening exercises correctly$`, w.answersTwoListeningExercises)
	sc.Step(`^"([^"]*)" answers a text exercise correctly in (\d+) milliseconds$`, w.answersATextExercise)
	sc.Step(`^"([^"]*)" answers a listening exercise whose sound lasts (\d+) milliseconds correctly in (\d+) milliseconds$`, w.answersAListeningExerciseCorrectly)
	sc.Step(`^"([^"]*)" answers a sound-choice exercise with (\d+) sound options of (\d+) milliseconds each correctly in (\d+) milliseconds$`, w.answersASoundChoiceExercise)
	sc.Step(`^"([^"]*)" answers an image-choice exercise correctly on (\d{4}-\d{2}-\d{2}) in (\d+) milliseconds$`, w.answersAnImageChoiceExerciseOn)

	sc.Step(`^both answers are judged against the fluent time of "([^"]*)"$`, w.bothJudgedAgainst)
	sc.Step(`^the answer took (\d+) milliseconds net of tap time(?: and audio)?$`, w.answerTookNet)
	sc.Step(`^it counts as within the fluent time$`, w.countsAs(true))
	sc.Step(`^it counts as slower than fluent$`, w.countsAs(false))
	sc.Step(`^the answer is judged against (\d+) milliseconds and counts as slower than fluent$`, w.judgedAgainstAndSlower)
}

// versionOneFrom is when a version 1 a scenario installs is in force: before any
// answer a scenario gives.
var versionOneFrom = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

var quoted = regexp.MustCompile(`"([^"]*)"`)

func labels(list string) []string {
	var out []string
	for _, m := range quoted.FindAllStringSubmatch(list, -1) {
		out = append(out, m[1])
	}
	return out
}

func optionID(exercise, label string) string { return stableUUID("option", exercise+":"+label) }

// exerciseKey names an exercise of a type, which exists from the moment a scenario
// mentions it: unless a step set its options, it has one right and one wrong one.
func (w *world) exerciseKey(name, exerciseType string) string {
	id := stableUUID("exercise", name)
	if _, ok := w.reference.exercises[id]; !ok {
		w.reference.exercises[id] = domain.ExerciseReference{
			ID: id, ExerciseType: exerciseType,
			OptionIDs:        []string{optionID(name, "right"), optionID(name, "wrong")},
			CorrectOptionIDs: []string{optionID(name, "right")},
		}
	}
	return "exercise:" + id
}

func (w *world) theExercise(name, correct, all string) error {
	id := stableUUID("exercise", name)
	ref := domain.ExerciseReference{ID: id, ExerciseType: "text_response"}
	for _, label := range labels(all) {
		ref.OptionIDs = append(ref.OptionIDs, optionID(name, label))
	}
	for _, label := range labels(correct) {
		ref.CorrectOptionIDs = append(ref.CorrectOptionIDs, optionID(name, label))
	}
	w.reference.exercises[id] = ref
	return nil
}

func (w *world) isTakingTheChallenge(student, challenge, node string) error {
	w.challenges[student] = &domain.TriggerContext{
		Source:        "challenge_sequence",
		ChallengeID:   stableUUID("challenge", challenge),
		ContentNodeID: stableUUID("node", node),
	}
	return nil
}

func (w *world) selecting(student, itemKey string, optionIDs []string, latency int, audio *int) error {
	return w.answer(student, itemKey, domain.PracticeResponse{
		Type: domain.PracticeResponseOptionChoice, OptionIDs: optionIDs, LatencyMs: &latency, AudioMs: audio,
	})
}

func (w *world) answersBySelecting(student, exercise, selected string) error {
	var ids []string
	for _, label := range labels(selected) {
		ids = append(ids, optionID(exercise, label))
	}
	return w.selecting(student, w.exerciseKey(exercise, "text_response"), ids, 4000, nil)
}

func (w *world) answersWithAnotherExercisesOption(student, exercise string) error {
	return w.selecting(student, w.exerciseKey(exercise, "text_response"), []string{optionID("another exercise", "C")}, 4000, nil)
}

func (w *world) answersInTheChallenge(student, exercise string, latency int) error {
	return w.selecting(student, w.exerciseKey(exercise, "text_response"), []string{optionID(exercise, "right")}, latency, nil)
}

// tookTheChallengeYesterday answers wrong in a run through the challenge a day before the next
// answer a scenario gives.
func (w *world) tookTheChallengeYesterday(student, challenge, exercise string) error {
	if err := w.isTakingTheChallenge(student, challenge, "challenge's node"); err != nil {
		return err
	}
	if err := w.selecting(student, w.exerciseKey(exercise, "text_response"), []string{optionID(exercise, "wrong")}, 4000, nil); err != nil {
		return err
	}
	w.clock = w.clock.Add(24 * time.Hour)
	return nil
}

// takesTheChallengeAgain answers in a new run through the challenge: what the client sends when
// the student moves on from the exercise with its correct option selected.
func (w *world) takesTheChallengeAgain(student, challenge, exercise string) error {
	if err := w.isTakingTheChallenge(student, challenge, "challenge's node"); err != nil {
		return err
	}
	return w.selecting(student, w.exerciseKey(exercise, "text_response"), []string{optionID(exercise, "right")}, 4000, nil)
}

func (w *world) answersAListeningExercise(student string, audio, latency int) error {
	return w.selecting(student, w.exerciseKey("listening exercise", "audio_recognition"), []string{optionID("listening exercise", "right")}, latency, &audio)
}

func (w *world) onlyEvidence(student, itemKey string) (domain.PracticeEvidence, error) {
	evidence, err := w.evidenceFor(student, itemKey)
	if err != nil {
		return domain.PracticeEvidence{}, err
	}
	if len(evidence) != 1 {
		return domain.PracticeEvidence{}, fmt.Errorf("want one piece of evidence, got %d", len(evidence))
	}
	e := evidence[0]
	if e.Source != domain.EvidenceSourceAutoGraded || e.GraderID != "exercise_option.v1" || e.Correct == nil {
		return e, fmt.Errorf("evidence is %s from %q, want auto_graded from exercise_option.v1", e.Source, e.GraderID)
	}
	return e, nil
}

func (w *world) hasAutoGradedEvidenceForExercise(student, exercise, verdict string) error {
	e, err := w.onlyEvidence(student, w.exerciseKey(exercise, "text_response"))
	if err != nil {
		return err
	}
	if *e.Correct != (verdict == "correct") {
		return fmt.Errorf("evidence is correct=%v, want %s", *e.Correct, verdict)
	}
	return nil
}

func (w *world) hasCorrectEvidenceWithLatency(student, exercise string, latency int) error {
	e, err := w.onlyEvidence(student, w.exerciseKey(exercise, "text_response"))
	if err != nil {
		return err
	}
	if !*e.Correct || e.LatencyMs == nil || *e.LatencyMs != latency {
		return fmt.Errorf("evidence is correct=%v after %v ms, want correct after %d ms", *e.Correct, e.LatencyMs, latency)
	}
	return nil
}

func (w *world) evidenceNamesTheChallenge(challenge string) error {
	e, err := w.onlyEvidence(w.lastStudent, w.lastItemKey)
	if err != nil {
		return err
	}
	if e.PracticeSessionID != "" {
		return fmt.Errorf("evidence names practice session %q", e.PracticeSessionID)
	}
	if e.TriggerContext == nil || e.TriggerContext.ChallengeID != stableUUID("challenge", challenge) {
		return fmt.Errorf("evidence names trigger context %+v, want challenge %q", e.TriggerContext, challenge)
	}
	return nil
}

func (w *world) hasEvidenceWithAudio(student string, latency, audio int) error {
	e, err := w.onlyEvidence(student, w.lastItemKey)
	if err != nil {
		return err
	}
	if e.LatencyMs == nil || *e.LatencyMs != latency || e.AudioMs == nil || *e.AudioMs != audio {
		return fmt.Errorf("evidence has latency %v and audio %v, want %d and %d", e.LatencyMs, e.AudioMs, latency, audio)
	}
	return nil
}

func (w *world) hasTwoPiecesTheSecondCorrect(student, exercise string) error {
	evidence, err := w.evidenceFor(student, w.exerciseKey(exercise, "text_response"))
	if err != nil {
		return err
	}
	if len(evidence) != 2 {
		return fmt.Errorf("want two pieces of evidence, got %d", len(evidence))
	}
	sort.Slice(evidence, func(i, j int) bool { return evidence[i].OccurredAt.Before(evidence[j].OccurredAt) })
	first, second := evidence[0], evidence[1]
	if first.Correct == nil || *first.Correct || second.Correct == nil || !*second.Correct {
		return fmt.Errorf("evidence is correct=%v then correct=%v, want wrong then correct", first.Correct, second.Correct)
	}
	return nil
}

func (w *world) hasNoEvidenceForExercise(student, exercise string) error {
	evidence, err := w.evidenceFor(student, w.exerciseKey(exercise, "text_response"))
	if err != nil {
		return err
	}
	if len(evidence) != 0 {
		return fmt.Errorf("want no evidence, got %d", len(evidence))
	}
	return nil
}

// ── Fluent times ─────────────────────────────────────────────────────────────

func (w *world) templateHasVersion1(template string, fluentMs int) error {
	w.reference.fluentTimes[template] = []domain.FluentTime{{Version: 1, EffectiveFrom: versionOneFrom, FluentNetMs: fluentMs}}
	w.lastTemplate = template
	return nil
}

func (w *world) lastTemplateHasVersion2(fluentMs int, from string) error {
	at, err := time.Parse(time.DateOnly, from)
	if err != nil {
		return err
	}
	w.reference.fluentTimes[w.lastTemplate] = append(w.reference.fluentTimes[w.lastTemplate],
		domain.FluentTime{Version: 2, EffectiveFrom: at, FluentNetMs: fluentMs})
	return nil
}

func (w *world) hasATapTime(student string, tapMs int) error {
	w.tapMs[student] = tapMs
	return nil
}

// answerCorrectly answers a fresh exercise of a type with its right option.
func (w *world) answerCorrectly(student, name, exerciseType string, latency int, audio *int) error {
	return w.selecting(student, w.exerciseKey(name, exerciseType), []string{optionID(name, "right")}, latency, audio)
}

// listeningLatency is slower than any fluent time a scenario sets, so the fold's
// fluency shows which fluent time it was judged against.
const listeningLatency = 7500

func (w *world) answersTwoListeningExercises(student string) error {
	for _, name := range []string{"listening exercise 1", "listening exercise 2"} {
		if err := w.answerCorrectly(student, name, "audio_recognition", listeningLatency, nil); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) answersATextExercise(student string, latency int) error {
	return w.answerCorrectly(student, "text exercise", "text_response", latency, nil)
}

func (w *world) answersAListeningExerciseCorrectly(student string, audio, latency int) error {
	return w.answerCorrectly(student, "listening exercise", "audio_recognition", latency, &audio)
}

func (w *world) answersASoundChoiceExercise(student string, options, eachMs, latency int) error {
	audio := options * eachMs
	return w.answerCorrectly(student, "sound-choice exercise", "audio_selection", latency, &audio)
}

func (w *world) answersAnImageChoiceExerciseOn(student, day string, latency int) error {
	at, err := time.Parse(time.DateOnly, day)
	if err != nil {
		return err
	}
	w.clock = at.Add(9 * time.Hour)
	return w.answerCorrectly(student, "image-choice exercise", "image_choice", latency, nil)
}

// judgement is how the worker's rules judge the latest answer's time, against
// the fluent times of its exercise's type.
func (w *world) judgement() (domain.TimedJudgement, error) {
	e, err := w.onlyEvidence(w.lastStudent, w.lastItemKey)
	if err != nil {
		return domain.TimedJudgement{}, err
	}
	exercise := w.reference.exercises[strings.TrimPrefix(e.ItemKey, "exercise:")]
	judged, ok := domain.JudgeTimed(e, domain.ItemGoal{FluentTimes: w.reference.fluentTimes[exercise.DrillTemplateKey()]})
	if !ok {
		return judged, fmt.Errorf("the answer was not judged against any fluent time")
	}
	return judged, nil
}

func (w *world) answerTookNet(netMs int) error {
	judged, err := w.judgement()
	if err != nil {
		return err
	}
	if judged.NetMs != netMs {
		return fmt.Errorf("the answer took %d ms net, want %d", judged.NetMs, netMs)
	}
	return nil
}

// countsAs checks the verdict and that the folded state agrees with it: a single
// right answer within the fluent time is fully fluent, a slower one isn't.
func (w *world) countsAs(within bool) func() error {
	return func() error {
		judged, err := w.judgement()
		if err != nil {
			return err
		}
		if judged.WithinFluentTime != within {
			return fmt.Errorf("within the fluent time = %v (%d ms net against %d ms), want %v",
				judged.WithinFluentTime, judged.NetMs, judged.FluentTime.FluentNetMs, within)
		}
		fold, err := w.lastFold()
		if err != nil {
			return err
		}
		if fullyFluent := fold.Fluency >= 1-1e-9; fullyFluent != within {
			return fmt.Errorf("the item's fluency is %.3f, which disagrees with the verdict", fold.Fluency)
		}
		return nil
	}
}

func (w *world) judgedAgainstAndSlower(fluentMs int) error {
	judged, err := w.judgement()
	if err != nil {
		return err
	}
	if judged.FluentTime.FluentNetMs != fluentMs {
		return fmt.Errorf("judged against %d ms, want %d", judged.FluentTime.FluentNetMs, fluentMs)
	}
	return w.countsAs(false)()
}

func (w *world) bothJudgedAgainst(template string) error {
	fluent := w.reference.fluentTimes[template]
	if len(fluent) == 0 {
		return fmt.Errorf("template %q has no fluent time", template)
	}
	net := listeningLatency - w.tapMs[w.lastStudent]
	want := math.Min(1, float64(fluent[0].FluentNetMs)/float64(net))
	for _, name := range []string{"listening exercise 1", "listening exercise 2"} {
		fold, _, _, err := w.states.Get(context.Background(), w.studentID(w.lastStudent), w.exerciseKey(name, "audio_recognition"))
		if err != nil {
			return err
		}
		if math.Abs(fold.Fluency-want) > 1e-9 {
			return fmt.Errorf("%s's fluency is %.3f, want %.3f from %q's fluent time", name, fold.Fluency, want, template)
		}
	}
	return nil
}
