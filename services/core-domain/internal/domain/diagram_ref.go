package domain

// DiagramPlaybackDirection is the order sequenced positions step through
// during playback.
type DiagramPlaybackDirection string

const (
	DiagramPlaybackDirectionAsAuthored DiagramPlaybackDirection = "as_authored"
	DiagramPlaybackDirectionReversed   DiagramPlaybackDirection = "reversed"
)

// DiagramLayers decides which optional layers a DiagramRef shows, decorating
// the referenced Diagram's base positions.
type DiagramLayers struct {
	Intervals bool
	// Subset, when non-nil, names the interval values to show; positions
	// with any other interval are hidden. Nil shows every position.
	Subset *[]string
	// ShapeOverlay, when non-nil, names a shape overlay style drawn around
	// the currently-visible positions. Nil shows no overlay.
	ShapeOverlay *string
}

// DiagramStyling holds author-chosen colors for one DiagramRef usage. A nil
// field uses motifpath-web's default color for that role.
type DiagramStyling struct {
	RootColor     *string
	IntervalColor *string
}

// DiagramPlayback is the sequenced playback config for a DiagramRef. It only
// affects positions with a non-nil SequenceIndex.
type DiagramPlayback struct {
	Direction DiagramPlaybackDirection
	StepMs    int
}

// DiagramRef is one usage of a Diagram — its render config, never a stored
// variant of the diagram itself. The same Diagram can be pointed at by any
// number of DiagramRefs with different configs.
type DiagramRef struct {
	DiagramID string
	// RootOverride, when non-nil, transposes the diagram to this root note.
	// Nil uses the diagram's own authored root.
	RootOverride *string
	Layers       DiagramLayers
	Styling      *DiagramStyling
	Playback     *DiagramPlayback
	// CorrectIntervals names which interval value(s) among the diagram's
	// currently-visible positions are correct answers. Meaningful, and
	// required, only when this ref is an Exercise's image_recognition
	// stimulus — ignored everywhere else this ref appears.
	CorrectIntervals *[]string
}

// DiagramStackRef composites two or more DiagramRefs into one view. Every
// entry must reference a Diagram on the same instrument — that requires a
// repository round trip, so it is an application-layer concern, not checked
// here.
type DiagramStackRef struct {
	Stack []DiagramRef
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
