package domain

import (
	"fmt"
	"maps"
	"slices"
)

// DrillItem is a generated practice item: one thing a drill family makes
// from its reference data, such as a fretboard cell or a catalog diagram's
// shape, and asks through its family's ways. A session composes them all
// alike; a new family is a new DrillItem, never a change to the composer.
type DrillItem interface {
	ItemKey() string
	Kind() PracticeItemKind
	// Ways lists the response types of the drill templates asking it, the
	// one to start from first.
	Ways() []string
	// Seconds is how long it is estimated to take in a session.
	Seconds() int
	// SuitsSession reports whether it is practised with the instrument in
	// hand (inHand) or in the head.
	SuitsSession(inHand bool) bool
	// Plan asks it through way, one of its Ways.
	Plan(way string, planning DrillPlanning) PlannedDrill
}

// ConfusableItem is a drill item a learner mistakes for others, such as a
// progression for one that shares most of its chords. A session asks them
// next to each other, so the student learns to tell them apart.
type ConfusableItem interface {
	DrillItem
	// ConfusableWith lists the keys of the items it is mistaken for.
	ConfusableWith() []string
}

// DrillPlanning is what planning a drill item may draw on.
type DrillPlanning struct {
	// Pick picks one of n equally fitting choices.
	Pick func(n int) int
	// Window is the frets a fretboard drill is asked within; nil for any
	// other drill.
	Window *FretWindow
}

// PlannedDrill is a drill item as a session asks it.
type PlannedDrill interface {
	// Way is the response type it is asked through.
	Way() string
	// Stimulus is what the question shows and what answers it, checked by
	// CheckChallenge.
	Stimulus() DrillStimulus
	// AttachTo puts it on item, the session item asking it.
	AttachTo(item *PracticeSessionItem)
}

// NextDrillWay is the way to ask item next: the one with the fewest right
// answers on it, the first of its ways on a tie, so every way gets practised,
// an item never answered right starts from the first, and an item guessed
// right one way meets another. state is nil for an item never practised.
func NextDrillWay(item DrillItem, state *PracticeItemState) string {
	ways := item.Ways()
	next := ways[0]
	if state == nil {
		return next
	}
	for _, way := range ways[1:] {
		if state.RightByResponse[way] < state.RightByResponse[next] {
			next = way
		}
	}
	return next
}

// ItemKey is the cell's item key: see FretboardCellItemKey.
func (c FretboardCell) ItemKey() string {
	return FretboardCellItemKey(c.LayoutInstrumentID, c.String, c.Fret)
}

// Kind is a fretboard cell's.
func (FretboardCell) Kind() PracticeItemKind { return PracticeItemKindFretboardCell }

// Ways names the note first, then finds it.
func (FretboardCell) Ways() []string {
	return []string{string(FretboardDrillNameTheNote), string(FretboardDrillFindTheNote)}
}

// Seconds is FretboardCellSeconds.
func (FretboardCell) Seconds() int { return FretboardCellSeconds }

// SuitsSession holds in the head only: a cell is recalled on the screen.
func (FretboardCell) SuitsSession(inHand bool) bool { return !inHand }

// Plan asks the cell through way, within planning's window.
func (c FretboardCell) Plan(way string, planning DrillPlanning) PlannedDrill {
	planned := &PlannedFretboardCell{FretboardCell: c, Drill: FretboardDrill(way)}
	if planning.Window != nil {
		planned.Window = *planning.Window
	}
	return planned
}

// Way is the way the cell is asked.
func (p *PlannedFretboardCell) Way() string { return string(p.Drill) }

// Stimulus shows the cell when its note is named: the cell is what the
// question gives. To find the note, it says the note and the string and
// draws an empty window, the cell being the answer.
func (p *PlannedFretboardCell) Stimulus() DrillStimulus {
	place := Place{String: p.String, Fret: p.Fret}
	if p.Drill == FretboardDrillFindTheNote {
		return DrillStimulus{Prompt: []string{fmt.Sprintf("string %d", p.String)}, Answer: DrillAnswer{Places: []Place{place}}}
	}
	return DrillStimulus{Places: []StimulusPlace{{Place: place, Given: true}}}
}

// AttachTo puts the planned cell on item.
func (p *PlannedFretboardCell) AttachTo(item *PracticeSessionItem) {
	item.Kind = PracticeItemKindFretboardCell
	item.FretboardCell = p
}

// DiagramShapeItem is a catalog diagram the drill catalog makes a shape,
// as a drill item.
type DiagramShapeItem struct {
	Diagram Diagram
}

// ItemKey is the shape's item key: see DiagramShapeItemKey.
func (s DiagramShapeItem) ItemKey() string { return DiagramShapeItemKey(s.Diagram.ID) }

// Kind is a diagram shape's.
func (DiagramShapeItem) Kind() PracticeItemKind { return PracticeItemKindDiagramShape }

// Ways names the shape first, then finds a degree on it.
func (DiagramShapeItem) Ways() []string {
	return []string{string(DiagramShapeDrillNameTheShape), string(DiagramShapeDrillFindTheDegree)}
}

// Seconds is DiagramShapeSeconds.
func (DiagramShapeItem) Seconds() int { return DiagramShapeSeconds }

// SuitsSession holds in the head only: a shape is recalled on the screen.
func (DiagramShapeItem) SuitsSession(inHand bool) bool { return !inHand }

// Plan asks the shape through way: see PlanDiagramShape.
func (s DiagramShapeItem) Plan(way string, planning DrillPlanning) PlannedDrill {
	return plannedShape{
		PlannedDiagramShape: PlanDiagramShape(s.Diagram, DiagramShapeDrill(way), planning.Pick),
		positions:           newDiagramShapeReference(s.Diagram).Positions,
	}
}

// plannedShape is a planned diagram shape with the places its question
// draws.
type plannedShape struct {
	PlannedDiagramShape
	positions []ShapePosition
}

// Way is the way the shape is asked.
func (p plannedShape) Way() string { return string(p.Drill) }

// Stimulus draws the shape's places. Naming it draws them all alike and
// offers its family's members, its own name being the answer. Finding a
// degree labels the roots, which it gives, draws the rest alike, and says
// the degree, its places being the answer.
func (p plannedShape) Stimulus() DrillStimulus {
	s := DrillStimulus{Places: make([]StimulusPlace, len(p.positions))}
	for i, pos := range p.positions {
		s.Places[i] = StimulusPlace{Place: Place{String: pos.String, Fret: pos.Fret}}
	}
	if p.AskedInterval == nil {
		for _, m := range p.Options {
			s.Options = append(s.Options, StimulusOption{Key: m.Shape, Labels: localizedValues(m.Names)})
			if m.Shape == p.Shape {
				s.Answer = DrillAnswer{Options: []string{m.Shape}, Labels: localizedValues(m.Names)}
			}
		}
		return s
	}
	s.Prompt = []string{*p.AskedInterval}
	for i, pos := range p.positions {
		switch pos.Interval {
		case "R":
			s.Places[i].Mark, s.Places[i].Given = PlaceMark{Label: "R"}, true
		case *p.AskedInterval:
			s.Answer.Places = append(s.Answer.Places, s.Places[i].Place)
		}
	}
	return s
}

// AttachTo puts the planned shape on item.
func (p plannedShape) AttachTo(item *PracticeSessionItem) {
	item.Kind = PracticeItemKindDiagramShape
	item.DiagramShape = &p.PlannedDiagramShape
}

// localizedValues lists text's values by language code.
func localizedValues(text LocalizedText) []string {
	values := make([]string, 0, len(text))
	for _, lang := range slices.Sorted(maps.Keys(text)) {
		values = append(values, text[lang])
	}
	return values
}
