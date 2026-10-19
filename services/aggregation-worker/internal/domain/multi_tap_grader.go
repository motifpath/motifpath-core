package domain

// multiTapV1 grades a note found everywhere in a fret window: right only when the
// cells tapped are exactly the cells of the window, on every string, that sound the
// note in any octave. A cell missed or an extra one is wrong; a tap outside the
// window or on a string the layout doesn't have is rejected.
type multiTapV1 struct{}

func (multiTapV1) ID() string { return "multi_tap.v1" }

// fretWindowEnd is a window's highest fret: the open string to fret 5 at the nut,
// otherwise its first fret and the next four.
func fretWindowEnd(from int) int {
	if from == 0 {
		return 5
	}
	return from + 4
}

// placesOf is every cell of the window that sounds the note, by string from 1,
// then fret from the lowest.
func placesOf(note string, instrument InstrumentReference, from, to int) []AnswerCell {
	notePitch, _ := pitchClass(note)
	var places []AnswerCell
	for str := 1; str <= len(instrument.Tuning); str++ {
		for fret := from; fret <= to; fret++ {
			if pc, ok := cellPitchClass(instrument, str, fret); ok && pc == notePitch {
				places = append(places, AnswerCell{String: str, Fret: fret})
			}
		}
	}
	return places
}

func (multiTapV1) Grade(key PracticeItemKey, r PracticeResponse, ref PracticeReference) GradeResult {
	if key.Kind != PracticeItemKindFretboardNote || r.Type != PracticeResponseMultiTap || r.FretWindowFrom == nil || r.LatencyMs == nil {
		return rejected(GradeRejectionResponseDoesNotFitItem)
	}
	instrument, ok := ref.Instruments[key.LayoutInstrumentID()]
	if !ok {
		return rejected(GradeRejectionUnknownReference)
	}
	from := *r.FretWindowFrom
	to := fretWindowEnd(from)
	for _, c := range r.Cells {
		if _, ok := cellPitchClass(instrument, c.String, c.Fret); !ok || c.Fret < from || c.Fret > to {
			return rejected(GradeRejectionInvalidCell)
		}
	}

	note := key.parts[1]
	places := placesOf(note, instrument, from, to)
	// Compared as sets, so a place tapped twice never stands in for one missed.
	tapped := map[AnswerCell]bool{}
	for _, c := range r.Cells {
		tapped[c] = true
	}
	correct := len(tapped) == len(places)
	for _, p := range places {
		correct = correct && tapped[p]
	}

	return GradeResult{Evidence: GradedEvidence{
		Source:    EvidenceSourceAutoGraded,
		Correct:   &correct,
		LatencyMs: r.LatencyMs,
		AnswerKey: &AnswerKey{NoteName: note, FretWindowFrom: &from, Cells: places},
	}}
}
