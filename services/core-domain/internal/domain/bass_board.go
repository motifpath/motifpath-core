package domain

const (
	// bassWindowFrets is how many frets a bass window spans from its first,
	// an open string counting as fret 0; bassWindowStarts is how many first
	// frets a session picks from, 0 up.
	bassWindowFrets  = 5
	bassWindowStarts = 10

	perfectFourth = 5
	perfectFifth  = 7
)

// bassWindow is the frets a bass drill marks its places within: from start to
// start + bassWindowFrets - 1.
type bassWindow struct {
	start int
}

func (w bassWindow) holds(fret int) bool {
	return fret >= w.start && fret < w.start+bassWindowFrets
}

// pickBassWindow is the window a session picks, its first fret one of
// bassWindowStarts; at the nut without a pick.
func pickBassWindow(pick func(n int) int) bassWindow {
	if pick == nil {
		return bassWindow{}
	}
	return bassWindow{start: pick(bassWindowStarts)}
}

// bassBoard is a bass layout's pitches inside one window.
type bassBoard struct {
	// opens is each string's open pitch, lowest string first, which is the
	// highest string number.
	opens  []int
	window bassWindow
}

func newBassBoard(layout Instrument, window bassWindow) (bassBoard, error) {
	opens := make([]int, len(layout.Tuning))
	for i, open := range layout.Tuning {
		midi, ok := ParsePitch(open)
		if !ok {
			return bassBoard{}, NewValidationError("tuning", "the layout's tuning is not in scientific pitch notation")
		}
		opens[i] = midi
	}
	return bassBoard{opens: opens, window: window}, nil
}

// optionalPlace is where pitch is played in the window, on the lowest-pitched
// string that reaches it there; nil when none does.
func (b bassBoard) optionalPlace(pitch int) *Place {
	for i, open := range b.opens {
		if fret := pitch - open; b.window.holds(fret) {
			return &Place{String: len(b.opens) - i, Fret: fret}
		}
	}
	return nil
}

// placeOf is where pitch is played in the window: see optionalPlace. Callers
// pass a pitch they found held there.
func (b bassBoard) placeOf(pitch int) Place {
	if p := b.optionalPlace(pitch); p != nil {
		return *p
	}
	return Place{}
}

func (b bassBoard) holds(pitch int) bool { return b.optionalPlace(pitch) != nil }

// root is the lowest pitch of pitchClass in the window, which is on the
// lowest string holding it there, with every interval of above held above it
// too; false when none is.
func (b bassBoard) root(pitchClass int, above []int) (int, bool) {
	if len(b.opens) == 0 {
		return 0, false
	}
	lowest := b.opens[0] + b.window.start
	for pitch := lowest; pitch < lowest+12*len(b.opens); pitch++ {
		if (pitch-pitchClass)%12 != 0 || !b.holds(pitch) {
			continue
		}
		fits := true
		for _, interval := range above {
			fits = fits && b.holds(pitch+interval)
		}
		if fits {
			return pitch, true
		}
	}
	return 0, false
}
