package domain

import "slices"

// PracticeItemKindDiagramShape is a catalog diagram's shape recalled in the
// head: named among its family's members, or one of its degrees found on it.
const PracticeItemKindDiagramShape PracticeItemKind = "diagram_shape"

// DiagramShapeItemKey is the item key of recalling diagramID's shape. One
// diagram is one item: the same shape at another root has other positions.
func DiagramShapeItemKey(diagramID string) string {
	return string(PracticeItemKindDiagramShape) + ":" + diagramID
}

// DiagramShapeFamily is a practice drill catalog family of shapes, such as
// the five CAGED grips or the four triad qualities. Reference data,
// installed by migration.
type DiagramShapeFamily struct {
	ID string
	// Key names the family, such as caged-grip.
	Key   string
	Names LocalizedText
	// Members lists the family's shapes in catalog order: the options a
	// shape to name offers, in the order it offers them.
	Members []DiagramShapeMember
}

// DiagramShapeMember is one shape of a family, such as the A shape of the
// CAGED grips.
type DiagramShapeMember struct {
	// Shape is the member's key, such as "A" or "minor", sent back as a
	// named shape.
	Shape string
	Names LocalizedText
}

// DiagramShape is what makes a catalog diagram a drill shape: the family it
// belongs to and the member it is.
type DiagramShape struct {
	Family DiagramShapeFamily
	Shape  string
}

// Member is the family member s is; false when the family doesn't list it.
func (s DiagramShape) Member() (DiagramShapeMember, bool) {
	for _, m := range s.Family.Members {
		if m.Shape == s.Shape {
			return m, true
		}
	}
	return DiagramShapeMember{}, false
}

// DiagramShapeReference is what the diagram shape grader knows about a drill
// shape: the layout its positions are on, its family and member, the
// family's members a named shape must be one of, and every position with
// its interval from the root.
type DiagramShapeReference struct {
	LayoutInstrumentID string
	Family             string
	Shape              string
	FamilyMembers      []string
	// Positions are in the diagram's order.
	Positions []ShapePosition
}

// ShapePosition is one marker of a drill shape.
type ShapePosition struct {
	String   int
	Fret     int
	Interval string
}

// newDiagramShapeReference is d's shape reference; nil when d isn't a drill
// shape. A position without a string and fret has no place on the fretboard,
// so it is left out.
func newDiagramShapeReference(d Diagram) *DiagramShapeReference {
	if d.Shape == nil {
		return nil
	}
	members := make([]string, len(d.Shape.Family.Members))
	for i, m := range d.Shape.Family.Members {
		members[i] = m.Shape
	}
	positions := make([]ShapePosition, 0, len(d.Positions))
	for _, p := range d.Positions {
		if p.String == nil || p.Fret == nil {
			continue
		}
		positions = append(positions, ShapePosition{String: *p.String, Fret: *p.Fret, Interval: p.Interval})
	}
	return &DiagramShapeReference{
		LayoutInstrumentID: d.InstrumentID,
		Family:             d.Shape.Family.Key,
		Shape:              d.Shape.Shape,
		FamilyMembers:      members,
		Positions:          positions,
	}
}

// DiagramShapeDrill is a way of asking a diagram shape.
type DiagramShapeDrill string

const (
	// DiagramShapeDrillNameTheShape shows the shape without its name; the
	// student picks it among its family's members.
	DiagramShapeDrillNameTheShape DiagramShapeDrill = "name_the_shape"
	// DiagramShapeDrillFindTheDegree shows the shape with its root marked;
	// the student taps the asked degree.
	DiagramShapeDrillFindTheDegree DiagramShapeDrill = "find_the_degree"
)

// DiagramShapeSeconds is how long a diagram shape is estimated to take in a
// session.
const DiagramShapeSeconds = 10

// NextDiagramShapeDrill is the way to ask a shape next: the one with the
// fewer right answers on it, naming the shape on a tie, so both ways get
// practised and a shape never answered right starts from naming it. state is
// nil for a shape never practised.
func NextDiagramShapeDrill(state *PracticeItemState) DiagramShapeDrill {
	if state == nil {
		return DiagramShapeDrillNameTheShape
	}
	named := state.RightByResponse[string(DiagramShapeDrillNameTheShape)]
	found := state.RightByResponse[string(DiagramShapeDrillFindTheDegree)]
	if found < named {
		return DiagramShapeDrillFindTheDegree
	}
	return DiagramShapeDrillNameTheShape
}

// PlannedDiagramShape is a diagram shape as a session asks it.
type PlannedDiagramShape struct {
	DiagramID string
	Drill     DiagramShapeDrill
	// Family is the shape's family key.
	Family string
	// Options are every member of the family, in catalog order, for a shape
	// to name, including members with no shape at this root, so the choices
	// never narrow the answer down; empty for a degree to find.
	Options []DiagramShapeMember
	// AskedInterval is the degree to find; nil for a shape to name.
	AskedInterval *string
}

// PlanDiagramShape is d, a drill shape, asked through drill. A degree to find
// is one of the shape's intervals other than its root, since the root is
// shown: pick chooses among them, given how many there are. A shape with
// nothing but roots has no degree to find, so it is named instead.
func PlanDiagramShape(d Diagram, drill DiagramShapeDrill, pick func(n int) int) PlannedDiagramShape {
	planned := PlannedDiagramShape{DiagramID: d.ID, Drill: drill, Family: d.Shape.Family.Key}
	if drill == DiagramShapeDrillFindTheDegree {
		if degrees := shapeDegrees(d); len(degrees) > 0 {
			asked := degrees[pick(len(degrees))]
			planned.AskedInterval = &asked
			planned.Options = []DiagramShapeMember{}
			return planned
		}
		planned.Drill = DiagramShapeDrillNameTheShape
	}
	planned.Options = d.Shape.Family.Members
	return planned
}

// shapeDegrees lists d's intervals other than its root, each once, in the
// order its positions first show them.
func shapeDegrees(d Diagram) []string {
	var degrees []string
	for _, p := range d.Positions {
		if p.Interval != "R" && !slices.Contains(degrees, p.Interval) {
			degrees = append(degrees, p.Interval)
		}
	}
	return degrees
}
