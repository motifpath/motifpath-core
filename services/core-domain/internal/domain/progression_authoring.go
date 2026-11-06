package domain

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	// MaxProgressionBars is the most bars a created progression may have.
	MaxProgressionBars = 16
	// MaxProgressionNicknameLength bounds a nickname, in characters.
	MaxProgressionNicknameLength = 60
)

// ErrProgressionIsCatalog refuses a change to a catalog progression, whose
// names are reference data.
var ErrProgressionIsCatalog = errors.New("the progression is the catalog's")

// ProgressionExistsError refuses a progression whose mode and numerals another
// one already has, naming it so the caller can use it instead.
type ProgressionExistsError struct {
	Existing Progression
}

func (e *ProgressionExistsError) Error() string {
	return fmt.Sprintf("the progression %s already has these numerals", e.Existing.Key)
}

// ProgressionUseRevision says which of a chart's texts tags a progression.
type ProgressionUseRevision string

const (
	ProgressionUseDraft     ProgressionUseRevision = "draft"
	ProgressionUsePublished ProgressionUseRevision = "published"
)

// ProgressionUse is a song chart section tagged with a progression, in a chart's
// draft or latest published revision. Older published revisions never count.
type ProgressionUse struct {
	ProgressionKey string
	SongChartID    string
	Title          string
	Revision       ProgressionUseRevision
	SectionIndex   int
	SectionLabel   *string
}

// ProgressionNotRemovableReason says why a progression can't be removed.
type ProgressionNotRemovableReason string

const (
	ProgressionNotRemovableInUse    ProgressionNotRemovableReason = "in_use"
	ProgressionNotRemovableInDrills ProgressionNotRemovableReason = "in_drills"
)

// ProgressionNotRemovableError refuses a removal. Uses lists what to untag when
// the reason is in_use.
type ProgressionNotRemovableError struct {
	Reason ProgressionNotRemovableReason
	Uses   []ProgressionUse
}

func (e *ProgressionNotRemovableError) Error() string {
	return "the progression can't be removed: " + string(e.Reason)
}

// IsCatalog says a migration installed it, rather than an admin.
func (p Progression) IsCatalog() bool { return p.CreatedBy == nil }

// NewCreatedProgression reads what an admin typed as a progression that is for
// song charts only. Its key and names come from its numerals. languages are the
// codes names are written in.
func NewCreatedProgression(mode string, bars []string, nicknames map[string]string, languages []string) (Progression, error) {
	numerals, err := parseProgressionBars(mode, bars)
	if err != nil {
		return Progression{}, err
	}
	p := Progression{
		Key:        createdProgressionKey(mode, numerals),
		Mode:       mode,
		Bars:       numerals,
		NodeIDs:    []string{},
		Confusable: []string{},
	}
	return p.WithNicknames(nicknames, languages)
}

// WithNicknames is p with these nicknames, and the names they give. An empty map
// removes them.
func (p Progression) WithNicknames(nicknames map[string]string, languages []string) (Progression, error) {
	trimmed := make(map[string]string, len(nicknames))
	for code, nickname := range nicknames {
		field := "nicknames/" + code
		if !slices.Contains(languages, code) {
			return Progression{}, NewValidationError(field, "isn't a language")
		}
		nickname = strings.TrimSpace(nickname)
		if nickname == "" {
			return Progression{}, NewValidationError(field, "can't be blank")
		}
		if utf8.RuneCountInString(nickname) > MaxProgressionNicknameLength {
			return Progression{}, NewValidationError(field, fmt.Sprintf("can't be longer than %d characters", MaxProgressionNicknameLength))
		}
		trimmed[code] = nickname
	}
	numerals := strings.Join(p.Bars, "–")
	names := make(LocalizedText, len(languages))
	for _, code := range languages {
		names[code] = numerals
		if nickname, ok := trimmed[code]; ok {
			names[code] = numerals + " (" + nickname + ")"
		}
	}
	p.Names = names
	p.Nicknames = trimmed
	if len(trimmed) == 0 {
		p.Nicknames = nil
	}
	return p, nil
}

// parseProgressionBars reads each bar as a numeral in mode, in its stored form.
func parseProgressionBars(mode string, bars []string) ([]string, error) {
	if _, ok := diatonicChords[ProgressionMode(mode)]; !ok {
		return nil, NewValidationError("mode", "must be major or minor")
	}
	if len(bars) == 0 || len(bars) > MaxProgressionBars {
		return nil, NewValidationError("bars", fmt.Sprintf("must have 1 to %d bars", MaxProgressionBars))
	}
	numerals := make([]string, len(bars))
	for i, bar := range bars {
		numeral, ok := ParseProgressionNumeral(bar, ProgressionMode(mode))
		if !ok {
			return nil, NewValidationError(fmt.Sprintf("bars/%d", i), fmt.Sprintf("bar %d: %q isn't a chord", i+1, bar))
		}
		numerals[i] = numeral.Numeral
	}
	return numerals, nil
}

// createdProgressionKey is the mode and numerals in ASCII, so identical numerals
// always give the same key. It holds a "/", which no catalog key does.
func createdProgressionKey(mode string, numerals []string) string {
	ascii := strings.NewReplacer("♭", "b", "♯", "#", "°", "o", "ø", "h")
	return mode + "/" + ascii.Replace(strings.Join(numerals, "-"))
}
