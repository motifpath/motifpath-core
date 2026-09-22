package domain

// DiagramPlaybackDirection is the order sequenced positions step through
// during playback.
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
	Intervals bool `json:"intervals"`
	// Subset, when non-nil, names the interval values to show; positions
	// with any other interval are hidden. Nil shows every position.
	Subset *[]string `json:"subset"`
	// ShapeOverlay, when non-nil, names a shape overlay style drawn around
	// the currently-visible positions. Nil shows no overlay.
	ShapeOverlay *string `json:"shape_overlay"`
}

// DiagramStyling holds author-chosen colors for one DiagramRef usage. A nil
// field uses motifpath-web's default color for that role. See DiagramLayers'
// doc comment for why this type carries json tags.
type DiagramStyling struct {
	RootColor     *string `json:"root_color"`
	IntervalColor *string `json:"interval_color"`
}

// DiagramPlayback is the sequenced playback config for a DiagramRef. It only
// affects positions with a non-nil SequenceIndex. See DiagramLayers' doc
// comment for why this type carries json tags.
type DiagramPlayback struct {
	Direction DiagramPlaybackDirection `json:"direction"`
	StepMs    int                      `json:"step_ms"`
}

// DiagramRef is one usage of a Diagram — its render config, never a stored
// variant of the diagram itself. The same Diagram can be pointed at by any
// number of DiagramRefs with different configs. See DiagramLayers' doc
// comment for why this type carries json tags.
type DiagramRef struct {
	DiagramID string `json:"diagram_id"`
	// RootOverride, when non-nil, transposes the diagram to this root note.
	// Nil uses the diagram's own authored root.
	RootOverride *string        `json:"root_override"`
	Layers       DiagramLayers  `json:"layers"`
	Styling      *DiagramStyling `json:"styling"`
	Playback     *DiagramPlayback `json:"playback"`
	// CorrectIntervals names which interval value(s) among the diagram's
	// currently-visible positions are correct answers. Meaningful, and
	// required, only when this ref is an Exercise's image_recognition
	// stimulus — ignored everywhere else this ref appears.
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
// playback config must name a valid direction and a positive step_ms.
func ValidateDiagramRef(ref DiagramRef) error {
	if ref.DiagramID == "" {
		return NewValidationError("diagram_id", "must not be empty")
	}
	if ref.Playback != nil {
		switch ref.Playback.Direction {
		case DiagramPlaybackDirectionAsAuthored, DiagramPlaybackDirectionReversed:
		default:
			return NewValidationError("playback", "direction must be one of: as_authored, reversed")
		}
		if ref.Playback.StepMs < 1 {
			return NewValidationError("playback", "step_ms must be at least 1")
		}
	}
	return nil
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
