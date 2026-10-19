package domain

// Voice is a sampled sound (timbre) diagrams can be played with, such as
// acoustic guitar or piano. It is kept apart from Instrument, which is only
// the layout positions are drawn on, so several voices can play the same
// diagrams. Voices are provided by the platform, never authored by users.
type Voice struct {
	// ID is a stable slug, e.g. "acoustic-guitar".
	ID    string
	Names LocalizedText
	// Family is the instrument family whose diagrams this voice plays.
	Family InstrumentFamily
	// Pitches are the MIDI pitches that have a recorded sample, ascending.
	Pitches []int
	// Attribution is the credit the samples' license requires.
	Attribution string
}
