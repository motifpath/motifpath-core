//go:build integration

package bdd

import (
	"fmt"

	"github.com/cucumber/godog"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

// catalogDiagrams are the basic guitar catalog diagrams the shape scenarios name,
// as core's practice_reference snapshot keeps them, by their catalog name. Their
// layout is the scenario's guitar, set when a step names them.
var catalogDiagrams = map[string]domain.DiagramReference{
	"C major — CAGED A, shift 3": {
		ID:          "928330d5-903e-572c-9d41-5fde99d51ed1",
		ShapeFamily: "caged-grip", Shape: "A", FamilyMembers: []string{"C", "A", "G", "E", "D"},
		Positions: []domain.DiagramPosition{
			{String: 5, Fret: 3, Interval: "R"}, {String: 4, Fret: 5, Interval: "5"}, {String: 3, Fret: 5, Interval: "R"},
			{String: 2, Fret: 5, Interval: "3"}, {String: 1, Fret: 3, Interval: "5"},
		},
	},
	// A chromatic map belongs to no shape family.
	"C Chromatic map — Frets 0–12": {
		ID: "67ff6d61-d8ba-5f44-96e1-c3b520f04ffa",
	},
}

func registerShapeSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the catalog (?:shape|diagram) "([^"]*)"$`, w.theCatalogDiagram)
	sc.Step(`^"([^"]*)" answers that (?:shape|diagram as a shape) by naming the shape "([^"]*)" after (\d+) milliseconds$`, w.answersThatShapeByNaming)
	sc.Step(`^"([^"]*)" answers that shape by finding the degree "([^"]*)" on string (\d+) at fret (\d+) after (\d+) milliseconds$`, w.answersThatShapeByFinding)
	sc.Step(`^"([^"]*)" has auto-graded evidence for that shape that is (correct|wrong) with a latency of (\d+) milliseconds$`, w.hasEvidenceForThatShape)
	sc.Step(`^"([^"]*)" has no evidence for that (?:shape|diagram)$`, w.hasNoEvidenceForThatShape)
	sc.Step(`^the evidence's answer key is the family "([^"]*)", member "([^"]*)"$`, w.answerKeyIsTheShape)
	sc.Step(`^the evidence's answer key is the degree "([^"]*)" at string (\d+), fret (\d+)$`, w.answerKeyIsTheDegree)
	sc.Step(`^the answer is rejected because the degree is not in the shape$`, w.answerRejectedBecause(domain.GradeRejectionDegreeNotInShape))
	sc.Step(`^the answer is rejected because the reference is unknown$`, w.answerRejectedBecause(domain.GradeRejectionUnknownReference))
}

// theCatalogDiagram puts the named catalog diagram in the reference snapshot and
// makes it the one the next answers are about.
func (w *world) theCatalogDiagram(name string) error {
	d, ok := catalogDiagrams[name]
	if !ok {
		return fmt.Errorf("no catalog diagram %q in the scenarios' catalog", name)
	}
	guitar := stableUUID("instrument", "guitar")
	d.InstrumentIDs, d.LayoutInstrumentID = []string{guitar}, guitar
	w.reference.diagrams[d.ID] = d
	w.shapeKey = "diagram_shape:" + d.ID
	return nil
}

func (w *world) answersThatShapeByNaming(student, shape string, latency int) error {
	return w.answer(student, w.shapeKey, domain.PracticeResponse{Type: domain.PracticeResponseNameTheShape, Shape: shape, LatencyMs: &latency})
}

func (w *world) answersThatShapeByFinding(student, interval string, str, fret, latency int) error {
	return w.answer(student, w.shapeKey, domain.PracticeResponse{
		Type: domain.PracticeResponseFindTheDegree, Interval: interval, String: &str, Fret: &fret, LatencyMs: &latency,
	})
}

func (w *world) hasEvidenceForThatShape(student, verdict string, latency int) error {
	e, err := w.onlyEvidenceFor(student, w.shapeKey)
	if err != nil {
		return err
	}
	if e.Source != domain.EvidenceSourceAutoGraded || e.Correct == nil {
		return fmt.Errorf("evidence is %s with no verdict, want auto-graded", e.Source)
	}
	if *e.Correct != (verdict == "correct") {
		return fmt.Errorf("the answer is correct = %v, want %s", *e.Correct, verdict)
	}
	if e.LatencyMs == nil || *e.LatencyMs != latency {
		return fmt.Errorf("latency is %v, want %d ms", e.LatencyMs, latency)
	}
	return nil
}

func (w *world) hasNoEvidenceForThatShape(student string) error {
	evidence, err := w.evidenceFor(student, w.shapeKey)
	if err != nil {
		return err
	}
	if len(evidence) != 0 {
		return fmt.Errorf("want no evidence for the shape, got %d", len(evidence))
	}
	return nil
}

func (w *world) answerKeyIsTheShape(family, member string) error {
	e, err := w.onlyEvidenceFor(w.lastStudent, w.shapeKey)
	if err != nil {
		return err
	}
	if k := e.AnswerKey; k == nil || k.ShapeFamily != family || k.Shape != member {
		return fmt.Errorf("answer key is %+v, want family %s, member %s", k, family, member)
	}
	return nil
}

func (w *world) answerKeyIsTheDegree(interval string, str, fret int) error {
	e, err := w.onlyEvidenceFor(w.lastStudent, w.shapeKey)
	if err != nil {
		return err
	}
	want := []domain.AnswerCell{{String: str, Fret: fret}}
	if k := e.AnswerKey; k == nil || k.Interval != interval || fmt.Sprint(k.Cells) != fmt.Sprint(want) {
		return fmt.Errorf("answer key is %+v, want degree %s at %v", k, interval, want)
	}
	return nil
}

// onlyEvidenceFor is the student's one piece of evidence for an item.
func (w *world) onlyEvidenceFor(student, itemKey string) (domain.PracticeEvidence, error) {
	evidence, err := w.evidenceFor(student, itemKey)
	if err != nil {
		return domain.PracticeEvidence{}, err
	}
	if len(evidence) != 1 {
		return domain.PracticeEvidence{}, fmt.Errorf("want one piece of evidence for %s, got %d", itemKey, len(evidence))
	}
	return evidence[0], nil
}
