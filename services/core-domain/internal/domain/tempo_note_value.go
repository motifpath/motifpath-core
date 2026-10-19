package domain

// TempoNoteValue is the note value a play-along is played at: a speed factor on
// the music as written, with the click staying on the beat. Quarters play it as
// written; eighths twice as fast over the same click.
type TempoNoteValue string

const (
	TempoNoteValueHalves        TempoNoteValue = "halves"
	TempoNoteValueQuarters      TempoNoteValue = "quarters"
	TempoNoteValueEighths       TempoNoteValue = "eighths"
	TempoNoteValueTriplets      TempoNoteValue = "triplets"
	TempoNoteValueSixteenths    TempoNoteValue = "sixteenths"
	TempoNoteValueQuintuplets   TempoNoteValue = "quintuplets"
	TempoNoteValueSextuplets    TempoNoteValue = "sextuplets"
	TempoNoteValueThirtySeconds TempoNoteValue = "thirty_seconds"
)

// tempoSpeedFactor is how many times faster than written each note value plays the music.
var tempoSpeedFactor = map[TempoNoteValue]float64{
	TempoNoteValueHalves:        0.5,
	TempoNoteValueQuarters:      1,
	TempoNoteValueEighths:       2,
	TempoNoteValueTriplets:      3,
	TempoNoteValueSixteenths:    4,
	TempoNoteValueQuintuplets:   5,
	TempoNoteValueSextuplets:    6,
	TempoNoteValueThirtySeconds: 8,
}

// factor is v's speed factor; an unknown note value plays as written.
func (v TempoNoteValue) factor() float64 {
	if f, ok := tempoSpeedFactor[v]; ok {
		return f
	}
	return 1
}

// EffectiveSpeed is how fast bpm in v plays the music, in quarter-note beats per
// minute: 80 BPM in eighths is 160. Tempos are compared at effective speed.
func (v TempoNoteValue) EffectiveSpeed(bpm int) float64 {
	return float64(bpm) * v.factor()
}

// tempoNoteValueOr is *v, or quarters when the tempo has no note value.
func tempoNoteValueOr(v *TempoNoteValue) TempoNoteValue {
	if v == nil {
		return TempoNoteValueQuarters
	}
	return *v
}
