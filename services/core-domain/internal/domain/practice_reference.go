package domain

import "slices"

// DiagramReference is what the practice graders may know about a Diagram
// (ADR-047): that it exists, the instruments it suits, and its playback
// tempo. Core keeps it in the read-only practice reference snapshot; it
// carries nothing shown to users.
type DiagramReference struct {
	ID string
	// InstrumentIDs lists every instrument the diagram suits, layout first.
	InstrumentIDs []string
	// TempoBPM is the playback tempo; nil when the diagram has no sequence.
	TempoBPM *int
}

// NewDiagramReference returns d's reference.
func NewDiagramReference(d Diagram) DiagramReference {
	return DiagramReference{ID: d.ID, InstrumentIDs: slices.Clone(d.InstrumentIDs), TempoBPM: d.TempoBPM}
}
