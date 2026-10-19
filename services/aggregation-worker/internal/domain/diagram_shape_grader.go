package domain

import "slices"

// diagramShapeV1 grades catalog shapes recalled in the head, asked two ways: name
// the shape among its family's members, or find an asked degree among the
// shape's positions. The root is shown, so it is never asked. The latency is
// kept for the fluent time, and the shape's member, or the asked degree and
// where it is, as the answer key.
type diagramShapeV1 struct{}

func (diagramShapeV1) ID() string { return "diagram_shape.v1" }

func (diagramShapeV1) Grade(key PracticeItemKey, r PracticeResponse, ref PracticeReference) GradeResult {
	if (r.Type != PracticeResponseNameTheShape && r.Type != PracticeResponseFindTheDegree) || r.LatencyMs == nil {
		return rejected(GradeRejectionResponseDoesNotFitItem)
	}
	diagram, ok := ref.Diagrams[key.DiagramIDs()[0]]
	if !ok || diagram.ShapeFamily == "" {
		return rejected(GradeRejectionUnknownReference)
	}
	if r.Type == PracticeResponseNameTheShape {
		return nameTheShape(r, diagram)
	}
	return findTheDegree(r, diagram, ref)
}

// nameTheShape is right when the named member is the diagram's; a member its
// family doesn't have is rejected.
func nameTheShape(r PracticeResponse, diagram DiagramReference) GradeResult {
	if r.Shape == "" {
		return rejected(GradeRejectionResponseDoesNotFitItem)
	}
	if !slices.Contains(diagram.FamilyMembers, r.Shape) {
		return rejected(GradeRejectionUnknownOption)
	}
	correct := r.Shape == diagram.Shape
	return GradeResult{Evidence: GradedEvidence{
		Source:    EvidenceSourceAutoGraded,
		Correct:   &correct,
		LatencyMs: r.LatencyMs,
		AnswerKey: &AnswerKey{ShapeFamily: diagram.ShapeFamily, Shape: diagram.Shape},
	}}
}

// findTheDegree is right when the tap is one of the shape's positions with the
// asked interval. A tap anywhere else is wrong, the same pitch off the shape
// included; a tap off the instrument, or a degree the shape doesn't offer, is
// rejected.
func findTheDegree(r PracticeResponse, diagram DiagramReference, ref PracticeReference) GradeResult {
	if r.Interval == "" || r.String == nil || r.Fret == nil {
		return rejected(GradeRejectionResponseDoesNotFitItem)
	}
	instrument, ok := ref.Instruments[diagram.LayoutInstrumentID]
	if !ok {
		return rejected(GradeRejectionUnknownReference)
	}
	if *r.String < 1 || *r.String > len(instrument.Tuning) || *r.Fret < 0 {
		return rejected(GradeRejectionInvalidCell)
	}
	var cells []AnswerCell
	if r.Interval != "R" {
		for _, p := range diagram.Positions {
			if p.Interval == r.Interval {
				cells = append(cells, AnswerCell{String: p.String, Fret: p.Fret})
			}
		}
	}
	if len(cells) == 0 {
		return rejected(GradeRejectionDegreeNotInShape)
	}
	correct := slices.Contains(cells, AnswerCell{String: *r.String, Fret: *r.Fret})
	return GradeResult{Evidence: GradedEvidence{
		Source:    EvidenceSourceAutoGraded,
		Correct:   &correct,
		LatencyMs: r.LatencyMs,
		AnswerKey: &AnswerKey{Interval: r.Interval, Cells: cells},
	}}
}
