package domain

import (
	"fmt"
	"regexp"
)

// InstrumentFamily decides which coordinate shape every Diagram authored
// against an Instrument uses. Guitar and bass share the fretted shape;
// piano is keyboard. A genuinely new coordinate shape is a new family value,
// not a change to an existing one.
type InstrumentFamily string

const (
	InstrumentFamilyFretted  InstrumentFamily = "fretted"
	InstrumentFamilyKeyboard InstrumentFamily = "keyboard"
)

// Valid reports whether f is a family this service knows how to draw.
func (f InstrumentFamily) Valid() bool {
	return f == InstrumentFamilyFretted || f == InstrumentFamilyKeyboard
}

// KeyRange is the lowest and highest playable key of a keyboard instrument,
// by note name (e.g. "A0" to "C8").
type KeyRange struct {
	Lowest  string
	Highest string
}

// Instrument is something a Diagram can be authored against. Which of
// StringCount/Tuning (fretted) or KeyRange (keyboard) is populated follows
// Family; the other group is always empty.
type Instrument struct {
	ID string
	// Names is the instrument's name in every language MotifPath offers:
	// instruments are shared by every user, so every language is required.
	Names       LocalizedText
	Family      InstrumentFamily
	StringCount *int
	// Tuning is each string's open pitch in scientific pitch notation
	// (e.g. "E2"), lowest string first.
	Tuning   []string
	KeyRange *KeyRange
	// DefaultVoiceID is the Voice that plays this instrument's diagrams
	// unless a usage picks another; always a voice of Family.
	DefaultVoiceID string
	// Icon is the key of the picture a client draws for the instrument
	// (e.g. "electric_bass"). The set is open: a client draws an unknown key
	// as its family's generic icon, which is the key a new instrument starts
	// with ("fretted" or "keyboard").
	Icon string
}

// iconKey is the shape of an instrument icon key: lowercase words joined by
// underscores, starting with a letter.
var iconKey = regexp.MustCompile(`^[a-z][a-z_]*$`)

// MaxInstrumentNameLength is the longest an instrument's name may be, in
// characters, in any one language.
const MaxInstrumentNameLength = 200

// NewInstrument validates and constructs an Instrument, stopping at the
// first violated invariant. names must cover exactly languages — every
// language MotifPath offers (never LanguageCodeAny). A fretted instrument needs a positive
// stringCount and a tuning with exactly one pitch, octave included, per
// string, and no keyRange; a keyboard instrument needs a keyRange with both
// ends named, and neither stringCount nor tuning. defaultVoice must be a
// voice of family. Whether it exists needs a repository round trip, so that
// stays an application-layer concern.
func NewInstrument(id string, names map[string]string, languages []string, family InstrumentFamily, stringCount *int, tuning []string, keyRange *KeyRange, defaultVoice Voice) (Instrument, error) {
	localized, err := NewLocalizedText("names", names, MaxInstrumentNameLength, languages)
	if err != nil {
		return Instrument{}, err
	}

	switch family {
	case InstrumentFamilyFretted:
		if err := validateFrettedShape(stringCount, tuning, keyRange); err != nil {
			return Instrument{}, err
		}
	case InstrumentFamilyKeyboard:
		if err := validateKeyboardShape(stringCount, tuning, keyRange); err != nil {
			return Instrument{}, err
		}
	default:
		return Instrument{}, NewValidationError("family", "must be one of: fretted, keyboard")
	}

	return Instrument{
		ID:          id,
		Names:       localized,
		Family:      family,
		StringCount: stringCount,
		Tuning:      tuning,
		KeyRange:    keyRange,
		Icon:        string(family),
	}.WithDefaultVoice(defaultVoice)
}

// WithDefaultVoice returns a copy of i played by voice unless a usage picks
// another, or an error if voice is missing or plays another family.
func (i Instrument) WithDefaultVoice(voice Voice) (Instrument, error) {
	if voice.ID == "" {
		return Instrument{}, NewValidationError("default_voice_id", "is required")
	}
	if voice.Family != i.Family {
		return Instrument{}, NewValidationError("default_voice_id", "must be a voice of the instrument's family")
	}
	i.DefaultVoiceID = voice.ID
	return i, nil
}

// WithIcon returns a copy of i drawn with the picture icon names, or an
// error if icon is not a key.
func (i Instrument) WithIcon(icon string) (Instrument, error) {
	if !iconKey.MatchString(icon) {
		return Instrument{}, NewValidationError("icon", "must be lowercase words joined by underscores, such as electric_bass")
	}
	i.Icon = icon
	return i, nil
}

func validateFrettedShape(stringCount *int, tuning []string, keyRange *KeyRange) error {
	if stringCount == nil || *stringCount < 1 {
		return NewValidationError("string_count", "is required for a fretted instrument and must be at least 1")
	}
	if len(tuning) == 0 {
		return NewValidationError("tuning", "is required for a fretted instrument")
	}
	if len(tuning) != *stringCount {
		return NewValidationError("tuning", "must have exactly one entry per string")
	}
	for i, pitch := range tuning {
		if _, ok := ParsePitch(pitch); !ok {
			return NewValidationError("tuning", fmt.Sprintf("entry %d %q is not a pitch with its octave (e.g. E2)", i, pitch))
		}
	}
	if keyRange != nil {
		return NewValidationError("key_range", "must be absent for a fretted instrument")
	}
	return nil
}

func validateKeyboardShape(stringCount *int, tuning []string, keyRange *KeyRange) error {
	if stringCount != nil {
		return NewValidationError("string_count", "must be absent for a keyboard instrument")
	}
	if len(tuning) != 0 {
		return NewValidationError("tuning", "must be absent for a keyboard instrument")
	}
	if keyRange == nil || keyRange.Lowest == "" || keyRange.Highest == "" {
		return NewValidationError("key_range", "is required for a keyboard instrument, with both lowest and highest named")
	}
	return nil
}
