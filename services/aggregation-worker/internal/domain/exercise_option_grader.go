package domain

import "slices"

// exerciseOptionV1 grades authored exercises answered by selecting options. The
// answer is right only when the options selected are exactly the exercise's correct
// ones, in any order (golden/practice-graders/exercise_option.v1.json). The latency,
// and the audio heard once, are kept for the fluent time.
type exerciseOptionV1 struct{}

func (exerciseOptionV1) ID() string { return "exercise_option.v1" }

func (exerciseOptionV1) Grade(key PracticeItemKey, r PracticeResponse, ref PracticeReference) GradeResult {
	if r.Type != PracticeResponseOptionChoice || len(r.OptionIDs) == 0 || r.LatencyMs == nil {
		return rejected(GradeRejectionResponseDoesNotFitItem)
	}
	exercise, ok := ref.Exercises[key.ExerciseID()]
	if !ok {
		return rejected(GradeRejectionUnknownReference)
	}
	for _, id := range r.OptionIDs {
		if !slices.Contains(exercise.OptionIDs, id) {
			return rejected(GradeRejectionUnknownOption)
		}
	}

	correct := len(r.OptionIDs) == len(exercise.CorrectOptionIDs)
	for _, id := range r.OptionIDs {
		correct = correct && slices.Contains(exercise.CorrectOptionIDs, id)
	}
	return GradeResult{Evidence: GradedEvidence{
		Source:    EvidenceSourceAutoGraded,
		Correct:   &correct,
		LatencyMs: r.LatencyMs,
		AudioMs:   r.AudioMs,
	}}
}
