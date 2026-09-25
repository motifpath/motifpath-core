package domain

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
	Tuning      []string
	KeyRange    *KeyRange
}

// MaxInstrumentNameLength is the longest an instrument's name may be, in
// characters, in any one language.
const MaxInstrumentNameLength = 200

// NewInstrument validates and constructs an Instrument, stopping at the
// first violated invariant. names must cover exactly languages — every
// language MotifPath offers (never LanguageCodeAny). A fretted instrument needs a positive
// stringCount and a tuning with exactly one entry per string, and no
// keyRange; a keyboard instrument needs a keyRange with both ends named, and
// neither stringCount nor tuning.
func NewInstrument(id string, names map[string]string, languages []string, family InstrumentFamily, stringCount *int, tuning []string, keyRange *KeyRange) (Instrument, error) {
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
	}, nil
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
