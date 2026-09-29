package domain

import (
	"encoding/json"
	"fmt"
	"slices"
)

// DiagramPlaybackDirection is the order a diagram's sequence steps play in.
type DiagramPlaybackDirection string

const (
	DiagramPlaybackDirectionAsAuthored DiagramPlaybackDirection = "as_authored"
	DiagramPlaybackDirectionReversed   DiagramPlaybackDirection = "reversed"
)

// DiagramLayers decides which optional layers a DiagramRef shows, decorating
// the referenced Diagram's base positions. Its json tags exist because this
// type is serialized directly to and from the wire — via generic JSON round
// trips in the http adapter, both standalone and, nested inside
// PromptNodeAttrs, as part of a whole PromptDocument — rather than through a
// field-by-field wire-type mapper.
type DiagramLayers struct {
	// Label is what each drawn marker shows; nil falls back to Intervals
	// (false shows nothing, true the diagram's own label display).
	Label *DiagramLabel `json:"label"`
	// Intervals is the older label switch, read only when Label is nil.
	Intervals bool `json:"intervals"`
	// HiddenPositionIDs names positions of the diagram this usage doesn't
	// draw; ids that aren't the diagram's positions are ignored. A hidden
	// position of an exercise stimulus is still an answer cell.
	HiddenPositionIDs *[]string `json:"hidden_position_ids"`
	// Subset, when non-nil, names the interval values to show; positions
	// with any other interval are hidden. Nil shows every position. Older
	// than HiddenPositionIDs, and still honoured alongside it.
	Subset *[]string `json:"subset"`
	// ShapeOverlay, when non-nil, names a shape overlay style drawn around
	// the currently-visible positions. Nil shows no overlay.
	ShapeOverlay *string `json:"shape_overlay"`
}

// DiagramLabel is what a usage's drawn markers show: the interval, the note
// name, the custom label (falling back to the diagram's own label display
// where a position has none), or nothing.
type DiagramLabel string

const (
	DiagramLabelInterval DiagramLabel = "interval"
	DiagramLabelNote     DiagramLabel = "note"
	DiagramLabelCustom   DiagramLabel = "custom"
	DiagramLabelNone     DiagramLabel = "none"
)

// Valid reports whether l is a label mode this service knows.
func (l DiagramLabel) Valid() bool {
	switch l {
	case DiagramLabelInterval, DiagramLabelNote, DiagramLabelCustom, DiagramLabelNone:
		return true
	}
	return false
}

// DiagramStyling holds author-chosen colors for one DiagramRef usage. A nil
// field uses motifpath-web's default color for that role. See DiagramLayers'
// doc comment for why this type carries json tags.
type DiagramStyling struct {
	RootColor     *string `json:"root_color"`
	IntervalColor *string `json:"interval_color"`
}

// DiagramPlayback is how one usage plays its Diagram's sequence. A diagram
// with no sequence never plays, whatever this says. See DiagramLayers' doc
// comment for why this type carries json tags.
type DiagramPlayback struct {
	Direction DiagramPlaybackDirection `json:"direction"`
	// TempoBPM overrides the diagram's tempo; nil uses it.
	TempoBPM *int `json:"tempo_bpm"`
	// VoiceID overrides the instrument's default voice; nil uses it. It
	// must name a voice of the diagram's instrument family, which needs a
	// repository round trip, so that is an application-layer concern.
	VoiceID *string `json:"voice_id"`
	Loop    bool    `json:"loop"`
}

// UnmarshalJSON decodes a playback, giving one that names no direction the
// authored order — the same default wherever the ref comes from.
func (p *DiagramPlayback) UnmarshalJSON(data []byte) error {
	type plain DiagramPlayback
	decoded := plain{Direction: DiagramPlaybackDirectionAsAuthored}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if decoded.Direction == "" {
		decoded.Direction = DiagramPlaybackDirectionAsAuthored
	}
	*p = DiagramPlayback(decoded)
	return nil
}

// DiagramRef is one usage of a Diagram — its render config, never a stored
// variant of the diagram itself. The same Diagram can be pointed at by any
// number of DiagramRefs with different configs. See DiagramLayers' doc
// comment for why this type carries json tags.
type DiagramRef struct {
	DiagramID string `json:"diagram_id"`
	// RootOverride, when non-nil, transposes the diagram to this root note.
	// Nil uses the diagram's own authored root.
	RootOverride *string          `json:"root_override"`
	Layers       DiagramLayers    `json:"layers"`
	Styling      *DiagramStyling  `json:"styling"`
	Playback     *DiagramPlayback `json:"playback"`
	// CorrectPositionIDs names the diagram's positions, drawn or hidden,
	// that are correct answers. Meaningful, and required (unless the older
	// CorrectIntervals is given), only when this ref is an Exercise's
	// image_recognition stimulus — ignored everywhere else.
	CorrectPositionIDs *[]string `json:"correct_position_ids"`
	// CorrectIntervals is the older way to name correct answers: the drawn
	// positions with these intervals. A stimulus given it without
	// CorrectPositionIDs is converted to positions once, at save.
	CorrectIntervals *[]string `json:"correct_intervals"`
}

// DiagramStackRef composites two or more DiagramRefs into one view. Every
// entry must reference a Diagram on the same instrument — that requires a
// repository round trip, so it is an application-layer concern, not checked
// here. See DiagramLayers' doc comment for why this type carries json tags.
type DiagramStackRef struct {
	Stack []DiagramRef `json:"stack"`
}

// ValidateDiagramRef checks ref's own structural invariants: it needs a
// diagram_id (whether that id refers to an existing Diagram requires a
// repository round trip, so is an application-layer concern), and any
// playback config must name a valid direction, a tempo within the allowed
// range and a non-empty voice id when it overrides them.
func ValidateDiagramRef(ref DiagramRef) error {
	if ref.DiagramID == "" {
		return NewValidationError("diagram_id", "must not be empty")
	}
	if ref.Layers.Label != nil && !ref.Layers.Label.Valid() {
		return NewValidationError("layers", "label must be one of: interval, note, custom, none")
	}
	if ref.Playback != nil {
		if reason := playbackProblem(*ref.Playback); reason != "" {
			return NewValidationError("playback", reason)
		}
	}
	return nil
}

// playbackProblem returns why p is invalid, or "" if it is valid.
func playbackProblem(p DiagramPlayback) string {
	switch p.Direction {
	case DiagramPlaybackDirectionAsAuthored, DiagramPlaybackDirectionReversed:
	default:
		return "direction must be one of: as_authored, reversed"
	}
	if p.TempoBPM != nil && (*p.TempoBPM < MinTempoBPM || *p.TempoBPM > MaxTempoBPM) {
		return fmt.Sprintf("tempo_bpm must be between %d and %d", MinTempoBPM, MaxTempoBPM)
	}
	if p.VoiceID != nil && *p.VoiceID == "" {
		return "voice_id must not be empty"
	}
	return ""
}

// ValidateDiagramStackRef checks that stack has at least two entries and
// that each entry is itself a valid DiagramRef. Whether every entry shares
// the same instrument requires a repository round trip per entry, so is an
// application-layer concern.
func ValidateDiagramStackRef(stack DiagramStackRef) error {
	if len(stack.Stack) < 2 {
		return NewValidationError("stack", "must contain at least two diagram refs")
	}
	for _, ref := range stack.Stack {
		if err := ValidateDiagramRef(ref); err != nil {
			return err
		}
	}
	return nil
}

// FretCell is one fretboard cell a student can pick in a diagram exercise:
// a string (1 = highest-pitched) at a fret (0 = the open string).
type FretCell struct {
	String int
	Fret   int
}

// minAnswerFretSpan is the fewest frets an answer window spans.
const minAnswerFretSpan = 3

// FrettedAnswerCells returns every cell of a fretted diagram's answer
// window, fret by fret (lowest first) and string by string within a fret.
// The window is the one a student sees the diagram drawn in, computed over
// all its positions — hidden ones included — and its regions: one fret
// either side of them, at least three frets, never below the nut. It holds
// every string at each fret past the window's start, plus the open strings
// when the window starts at the nut.
func FrettedAnswerCells(positions []Position, regions []Region, stringCount int) []FretCell {
	var frets []int
	for _, p := range positions {
		if p.Fret != nil {
			frets = append(frets, *p.Fret)
		}
	}
	for _, r := range regions {
		if r.FretStart != nil {
			frets = append(frets, *r.FretStart)
		}
		if r.FretEnd != nil {
			frets = append(frets, *r.FretEnd)
		}
	}
	low, high := 0, minAnswerFretSpan
	if len(frets) > 0 {
		low = max(slices.Min(frets)-1, 0)
		high = low + max(slices.Max(frets)+1-low, minAnswerFretSpan)
	}
	first := low + 1
	if low == 0 {
		first = 0
	}
	cells := make([]FretCell, 0, (high-first+1)*stringCount)
	for fret := first; fret <= high; fret++ {
		for str := 1; str <= stringCount; str++ {
			cells = append(cells, FretCell{String: str, Fret: fret})
		}
	}
	return cells
}

// EmbeddedDiagramRefs returns every single diagram embedded anywhere in d,
// in document order. Embedded stacks are left out: a stack doesn't play.
func (d PromptDocument) EmbeddedDiagramRefs() []DiagramRef {
	var refs []DiagramRef
	var walk func(nodes []PromptNode)
	walk = func(nodes []PromptNode) {
		for _, node := range nodes {
			if node.Type == PromptNodeTypeDiagram && node.Attrs != nil && node.Attrs.DiagramRef != nil {
				refs = append(refs, *node.Attrs.DiagramRef)
			}
			walk(node.Content)
		}
	}
	walk(d.Content)
	return refs
}
