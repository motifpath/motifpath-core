package domain

import (
	"fmt"
	"slices"
	"strings"
)

// ChordDefinition is a chord of the chord catalog: what is played, apart
// from any fingering. Formula lists its tones as intervals above the root,
// starting with R; Omittable lists the ones a voicing may leave out. Bass
// and BassPitchClass are nil together, for a chord with no slash bass.
// Voicings, best first, are filled in only when a chord is read with them.
type ChordDefinition struct {
	ID              string
	CanonicalSymbol string
	Root            string
	RootPitchClass  int
	Quality         ChordQuality
	Formula         []string
	Omittable       []string
	Bass            *string
	BassPitchClass  *int
	Aliases         []string
	Voicings        []ChordVoicing
}

// ChordVoicingStatus says whether the catalog still offers a voicing. A
// withdrawn voicing stays for the content that embeds its diagram.
type ChordVoicingStatus string

const (
	ChordVoicingActive    ChordVoicingStatus = "active"
	ChordVoicingWithdrawn ChordVoicingStatus = "withdrawn"
)

// ChordVoicing is one playable fingering of a chord. Its positions and
// playbacks are those of its diagram, which it references and never copies.
type ChordVoicing struct {
	ID                string
	ChordDefinitionID string
	DiagramID         string
	InstrumentID      string
	// TuningFingerprint is the open-string pitches the voicing was checked
	// against, lowest string first, joined by "-".
	TuningFingerprint string
	LowestFret        int
	HighestFret       int
	Fingering         []VoicingFinger
	MutedStrings      []int
	OmittedIntervals  []string
	Difficulty        string
	TechniqueTags     []string
	ShapeFamily       *string
	IsMovable         bool
	RecommendedRank   int
	Status            ChordVoicingStatus
	// TemplateKey names the movable shape that generated the voicing; nil
	// for a hand-authored one.
	TemplateKey *string
}

// VoicingFinger is the fretting-hand finger on one fretted position.
type VoicingFinger struct {
	PositionID string
	Finger     string
}

// MaxVoicingFret is the highest fret a catalog voicing may use.
const MaxVoicingFret = 15

// intervalSemitones is each chord formula interval's distance above the
// root, in semitones within one octave.
var intervalSemitones = map[string]int{
	"R": 0, "b9": 1, "2": 2, "9": 2, "b3": 3, "#9": 3, "3": 4, "4": 5, "11": 5,
	"b5": 6, "5": 7, "#5": 8, "6": 9, "bb7": 9, "13": 9, "b7": 10, "7": 11,
}

// TuningFingerprint joins tuning's open-string pitches, lowest first, with "-".
func TuningFingerprint(tuning []string) string {
	return strings.Join(tuning, "-")
}

// ValidateChordVoicing checks that diagram, played on instrument, really
// plays chord as voicing describes it, so a wrong fingering never reaches a
// learner. The diagram must be a chord_voicing diagram on instrument, under
// the voicing's tuning. Its sounded pitch classes must be exactly the
// chord's formula less the voicing's declared omissions (doubled tones
// allowed; a slash chord's bass allowed too); the root is never omitted, and
// only the chord's omittable tones may be. A slash chord's lowest sounded
// string sounds its bass. No fret is above MaxVoicingFret, and a movable
// voicing sounds no open string.
func ValidateChordVoicing(chord ChordDefinition, diagram Diagram, instrument Instrument, voicing ChordVoicing) error {
	if diagram.Purpose != DiagramPurposeChordVoicing || diagram.ID != voicing.DiagramID {
		return NewValidationError("diagram_id", "must name the voicing's own chord_voicing diagram")
	}
	if diagram.InstrumentID != instrument.ID || voicing.InstrumentID != instrument.ID {
		return NewValidationError("instrument_id", "must be the diagram's layout instrument")
	}
	if voicing.TuningFingerprint != TuningFingerprint(instrument.Tuning) {
		return NewValidationError("tuning_fingerprint", "must be the instrument's tuning, "+TuningFingerprint(instrument.Tuning))
	}
	sounded, lowest, err := soundedPitchClasses(diagram, instrument, voicing.IsMovable)
	if err != nil {
		return err
	}
	if err := checkFormula(chord, sounded); err != nil {
		return err
	}
	if err := checkOmissions(chord, sounded, voicing.OmittedIntervals); err != nil {
		return err
	}
	if chord.BassPitchClass != nil && lowest != *chord.BassPitchClass {
		return NewValidationError("positions", fmt.Sprintf("the lowest sounded string must sound the bass of %s", chord.CanonicalSymbol))
	}
	return nil
}

// soundedPitchClasses returns the pitch class of every fretted position of
// diagram on instrument, and that of the lowest-pitched one.
func soundedPitchClasses(diagram Diagram, instrument Instrument, movable bool) (map[int]bool, int, error) {
	sounded := map[int]bool{}
	lowestMIDI, lowestPC := 1000, -1
	for _, p := range diagram.Positions {
		if p.String == nil || p.Fret == nil || *p.String < 1 || *p.String > len(instrument.Tuning) {
			return nil, 0, NewValidationError("positions", "every position must be a string and fret of the instrument")
		}
		fret := *p.Fret
		if fret < 0 || fret > MaxVoicingFret {
			return nil, 0, NewValidationError("positions", fmt.Sprintf("frets must be between 0 and %d", MaxVoicingFret))
		}
		if movable && fret == 0 {
			return nil, 0, NewValidationError("is_movable", "a movable voicing sounds no open string")
		}
		// String 1 is the highest-pitched, the last tuning entry.
		open, ok := ParsePitch(instrument.Tuning[len(instrument.Tuning)-*p.String])
		if !ok {
			return nil, 0, NewValidationError("tuning_fingerprint", "the instrument's tuning is not in scientific pitch notation")
		}
		midi := open + fret
		sounded[midi%12] = true
		if midi < lowestMIDI {
			lowestMIDI, lowestPC = midi, midi%12
		}
	}
	if len(sounded) == 0 {
		return nil, 0, NewValidationError("positions", "a voicing must sound at least one string")
	}
	return sounded, lowestPC, nil
}

// checkFormula refuses a sounded pitch class that is neither a tone of
// chord's formula nor its slash bass.
func checkFormula(chord ChordDefinition, sounded map[int]bool) error {
	allowed := map[int]bool{}
	for _, interval := range chord.Formula {
		allowed[(chord.RootPitchClass+intervalSemitones[interval])%12] = true
	}
	if chord.BassPitchClass != nil {
		allowed[*chord.BassPitchClass] = true
	}
	for pc := range sounded {
		if !allowed[pc] {
			return NewValidationError("positions", fmt.Sprintf("sounds a note that is not in %s", chord.CanonicalSymbol))
		}
	}
	return nil
}

// checkOmissions requires declared to be exactly the formula tones not
// sounded, each of them omittable and none of them the root.
func checkOmissions(chord ChordDefinition, sounded map[int]bool, declared []string) error {
	var missing []string
	for _, interval := range chord.Formula {
		if !sounded[(chord.RootPitchClass+intervalSemitones[interval])%12] {
			missing = append(missing, interval)
		}
	}
	for _, interval := range missing {
		if interval == "R" {
			return NewValidationError("omitted_intervals", "a voicing must sound the root")
		}
		if !slices.Contains(chord.Omittable, interval) {
			return NewValidationError("omitted_intervals", fmt.Sprintf("%s can't leave out its %s", chord.CanonicalSymbol, interval))
		}
	}
	sortedDeclared, sortedMissing := slices.Clone(declared), slices.Clone(missing)
	slices.Sort(sortedDeclared)
	slices.Sort(sortedMissing)
	if !slices.Equal(sortedDeclared, sortedMissing) {
		return NewValidationError("omitted_intervals", fmt.Sprintf("must list exactly the tones the voicing leaves out: %v", missing))
	}
	return nil
}
