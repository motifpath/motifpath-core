package domain

import (
	"regexp"
	"strconv"
	"strings"
)

// fretboardCellV1 grades fretboard cells, asked two ways: name the note of the cell
// shown, or find the note asked on the string asked. A cell's pitch is its string's
// open pitch raised by the fret, in semitones; strings count from 1, the
// highest-pitched. Naming is right in any spelling of the cell's pitch; finding is
// right on the asked string at any octave. The latency is kept for the fluent time,
// and the asked cell and its note as the answer key.
type fretboardCellV1 struct{}

func (fretboardCellV1) ID() string { return "fretboard_cell.v1" }

// sharpNames spells each pitch class with sharps, C first.
var sharpNames = [12]string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

// naturalPitch is each natural note's pitch class.
var naturalPitch = map[byte]int{'C': 0, 'D': 2, 'E': 4, 'F': 5, 'G': 7, 'A': 9, 'B': 11}

// noteSpelling is a note name: a letter, then up to two sharps or flats.
var noteSpelling = regexp.MustCompile(`^([A-G])(#{0,2}|b{0,2})$`)

// pitchClass reads a note name, or a tuning entry with its octave (E2, F#3), into
// its pitch class, and reports false for anything else.
func pitchClass(note string) (int, bool) {
	m := noteSpelling.FindStringSubmatch(strings.TrimRight(note, "0123456789"))
	if m == nil {
		return 0, false
	}
	pc := naturalPitch[m[1][0]] + strings.Count(m[2], "#") - strings.Count(m[2], "b")
	return ((pc % 12) + 12) % 12, true
}

func (fretboardCellV1) Grade(key PracticeItemKey, r PracticeResponse, ref PracticeReference) GradeResult {
	if (r.Type != PracticeResponseNameTheNote && r.Type != PracticeResponseFindTheNote) || r.LatencyMs == nil {
		return rejected(GradeRejectionResponseDoesNotFitItem)
	}
	str, fret := atoi(key.parts[1]), atoi(key.parts[2])
	instrument, ok := ref.Instruments[key.LayoutInstrumentID()]
	if !ok {
		return rejected(GradeRejectionUnknownReference)
	}
	strings := len(instrument.Tuning)
	if str < 1 || str > strings {
		return rejected(GradeRejectionInvalidCell)
	}
	open, ok := pitchClass(instrument.Tuning[strings-str])
	if !ok {
		return rejected(GradeRejectionUnknownReference)
	}
	cellPitch := (open + fret) % 12

	var correct bool
	var rejection GradeRejection
	if r.Type == PracticeResponseNameTheNote {
		correct, rejection = nameTheNote(r, cellPitch)
	} else {
		correct, rejection = findTheNote(r, strings, str, fret)
	}
	if rejection != "" {
		return rejected(rejection)
	}
	return GradeResult{Evidence: GradedEvidence{
		Source:    EvidenceSourceAutoGraded,
		Correct:   &correct,
		LatencyMs: r.LatencyMs,
		AnswerKey: &AnswerKey{String: &str, Fret: &fret, NoteName: sharpNames[cellPitch]},
	}}
}

// nameTheNote is right when the named note, in any spelling, is the cell's pitch.
func nameTheNote(r PracticeResponse, cellPitch int) (bool, GradeRejection) {
	named, ok := pitchClass(r.NoteName)
	if !ok {
		return false, GradeRejectionResponseDoesNotFitItem
	}
	return named == cellPitch, ""
}

// findTheNote is right when the tap is on the asked string at the asked fret or an
// octave of it; a tap off the instrument is rejected.
func findTheNote(r PracticeResponse, strings, str, fret int) (bool, GradeRejection) {
	if r.String == nil || r.Fret == nil {
		return false, GradeRejectionResponseDoesNotFitItem
	}
	if *r.String < 1 || *r.String > strings || *r.Fret < 0 {
		return false, GradeRejectionInvalidCell
	}
	return *r.String == str && (*r.Fret-fret)%12 == 0, ""
}

// atoi reads a key segment the item key pattern already checked is a number.
func atoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return n
}
