package domain

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

// LocalizedText is one piece of user-facing text written in several
// languages, keyed by Language.Code — e.g. {"en": "Guitar", "pt_BR":
// "Violão"}. LanguageCodeAny is never a key: text is always words in some
// language. Clients show the viewer's language, falling back to "en", then
// to any language present.
type LocalizedText map[string]string

// Languages returns the codes text is written in, sorted.
func (t LocalizedText) Languages() []string {
	codes := make([]string, 0, len(t))
	for code := range t {
		codes = append(codes, code)
	}
	slices.Sort(codes)
	return codes
}

// NewLocalizedText validates text against languages, the exact set of
// language codes it must cover — no more and no fewer — and returns it with
// every value trimmed. Each value must be non-blank and at most maxLength
// characters. Every problem is reported against field.
func NewLocalizedText(field string, text map[string]string, maxLength int, languages []string) (LocalizedText, error) {
	if len(text) == 0 {
		return nil, NewValidationError(field, "must contain text in at least one language")
	}
	result := make(LocalizedText, len(text))
	// Walked in sorted order, so identical input always reports the same
	// problem first.
	for _, code := range LocalizedText(text).Languages() {
		value := text[code]
		if code == LanguageCodeAny {
			return nil, NewValidationError(field, fmt.Sprintf("must not use %q as a language", LanguageCodeAny))
		}
		if !slices.Contains(languages, code) {
			return nil, NewValidationError(field, fmt.Sprintf("has text in %q, which is not one of the required languages", code))
		}
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return nil, NewValidationError(field, fmt.Sprintf("must not be blank in %q", code))
		}
		if utf8.RuneCountInString(trimmed) > maxLength {
			return nil, NewValidationError(field, fmt.Sprintf("must be at most %d characters in %q", maxLength, code))
		}
		result[code] = trimmed
	}
	for _, code := range languages {
		if _, ok := result[code]; !ok {
			return nil, NewValidationError(field, fmt.Sprintf("is missing text in %q", code))
		}
	}
	return result, nil
}
