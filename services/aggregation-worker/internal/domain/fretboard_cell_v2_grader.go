package domain

// fretboardCellV2 grades fretboard cells as fretboardCellV1 does, and checks the
// four choices a note was named among: four different pitches that include the
// cell's note and the note named. The choices are kept in the answer key as sent.
// A note named without choices was named on the earlier twelve-note keypad, and is
// graded exactly as v1 graded it.
type fretboardCellV2 struct{}

func (fretboardCellV2) ID() string { return "fretboard_cell.v2" }

func (fretboardCellV2) Grade(key PracticeItemKey, r PracticeResponse, ref PracticeReference) GradeResult {
	result := fretboardCellV1{}.Grade(key, r, ref)
	if result.Rejection != "" || r.Type != PracticeResponseNameTheNote || r.Choices == nil {
		return result
	}
	cellPitch, _ := pitchClass(result.Evidence.AnswerKey.NoteName)
	named, _ := pitchClass(r.NoteName)
	if !validNoteChoices(r.Choices, cellPitch, named) {
		return rejected(GradeRejectionInvalidChoices)
	}
	result.Evidence.AnswerKey.Choices = r.Choices
	return result
}

// validNoteChoices reports whether choices are four different pitches that include
// both the cell's pitch and the one named.
func validNoteChoices(choices []string, cellPitch, named int) bool {
	if len(choices) != 4 {
		return false
	}
	seen := map[int]bool{}
	for _, c := range choices {
		pc, ok := pitchClass(c)
		if !ok || seen[pc] {
			return false
		}
		seen[pc] = true
	}
	return seen[cellPitch] && seen[named]
}
