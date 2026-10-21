package domain

import (
	"fmt"
	"slices"
)

// multiTapV2 grades a fretboard cell asked to find every instance of its note in a
// fret window, on every cell the answer covers: each place of the note in the
// window is right when tapped and wrong when missed, and each other cell tapped is
// wrong. One time covers several taps, so the evidence counts for accuracy only and
// carries no latency. The planned cell, a tap outside the window, or a tap on a
// string the layout doesn't have is rejected.
type multiTapV2 struct{}

func (multiTapV2) ID() string { return "multi_tap.v2" }

func (multiTapV2) Grade(key PracticeItemKey, r PracticeResponse, ref PracticeReference) GradeResult {
	if key.Kind != PracticeItemKindFretboardCell || r.Type != PracticeResponseMultiTap || r.FretWindowFrom == nil || r.LatencyMs == nil {
		return rejected(GradeRejectionResponseDoesNotFitItem)
	}
	instrument, ok := ref.Instruments[key.LayoutInstrumentID()]
	if !ok {
		return rejected(GradeRejectionUnknownReference)
	}
	from := *r.FretWindowFrom
	to := fretWindowEnd(from)
	planned := AnswerCell{String: atoi(key.parts[1]), Fret: atoi(key.parts[2])}
	notePitch, ok := cellPitchClass(instrument, planned.String, planned.Fret)
	if !ok || !inWindow(planned, from, to) {
		return rejected(GradeRejectionInvalidCell)
	}
	for _, c := range r.Cells {
		if _, ok := cellPitchClass(instrument, c.String, c.Fret); !ok || !inWindow(c, from, to) {
			return rejected(GradeRejectionInvalidCell)
		}
	}

	note := sharpNames[notePitch]
	places := placesOf(note, instrument, from, to)
	tapped := map[AnswerCell]bool{}
	for _, c := range r.Cells {
		tapped[c] = true
	}
	var graded []ItemEvidence
	for _, cell := range coveredCells(places, r.Cells) {
		correct := slices.Contains(places, cell) && tapped[cell]
		str, fret := cell.String, cell.Fret
		graded = append(graded, ItemEvidence{
			ItemKey: fmt.Sprintf("%s:%s:%d:%d", PracticeItemKindFretboardCell, key.LayoutInstrumentID(), str, fret),
			Evidence: GradedEvidence{
				Source:    EvidenceSourceAutoGraded,
				Correct:   &correct,
				AnswerKey: &AnswerKey{String: &str, Fret: &fret, NoteName: note, FretWindowFrom: &from, Cells: places},
			},
		})
	}
	return GradeResult{EvidenceByItem: graded}
}

func inWindow(c AnswerCell, from, to int) bool { return c.Fret >= from && c.Fret <= to }

// coveredCells is every place of the note and every cell tapped, once each, by
// string from 1, then fret from the lowest.
func coveredCells(places, taps []AnswerCell) []AnswerCell {
	cells := slices.Clone(places)
	for _, c := range taps {
		if !slices.Contains(cells, c) {
			cells = append(cells, c)
		}
	}
	slices.SortFunc(cells, func(a, b AnswerCell) int {
		if a.String != b.String {
			return a.String - b.String
		}
		return a.Fret - b.Fret
	})
	return cells
}
