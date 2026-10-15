package domain

// NoteValue is the note value a play-along take was played at: a speed factor on
// the music as written, with the click staying on the beat. A take with no note
// value was played as written, in quarters.
type NoteValue string

const (
	NoteValueHalves        NoteValue = "halves"
	NoteValueQuarters      NoteValue = "quarters"
	NoteValueEighths       NoteValue = "eighths"
	NoteValueTriplets      NoteValue = "triplets"
	NoteValueSixteenths    NoteValue = "sixteenths"
	NoteValueQuintuplets   NoteValue = "quintuplets"
	NoteValueSextuplets    NoteValue = "sextuplets"
	NoteValueThirtySeconds NoteValue = "thirty_seconds"
)

// speedFactor is how many times faster than written each note value plays the music.
var speedFactor = map[NoteValue]float64{
	NoteValueHalves:        0.5,
	NoteValueQuarters:      1,
	NoteValueEighths:       2,
	NoteValueTriplets:      3,
	NoteValueSixteenths:    4,
	NoteValueQuintuplets:   5,
	NoteValueSextuplets:    6,
	NoteValueThirtySeconds: 8,
}

// Valid reports whether v is one of the note values.
func (v NoteValue) Valid() bool {
	_, ok := speedFactor[v]
	return ok
}

// EffectiveSpeed is how fast a take at bpm in note value v plays the music, in
// quarter-note beats per minute: 80 BPM in eighths is 160. Tempos of one item are
// compared at effective speed, whatever note value each was played at.
func EffectiveSpeed(bpm int, v *NoteValue) float64 {
	if v == nil {
		return float64(bpm)
	}
	return float64(bpm) * speedFactor[*v]
}
