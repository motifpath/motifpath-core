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
	return newLocalizedText(field, text, maxLength, languages, true)
}

// NewLocalizedTextFrom is NewLocalizedText for text that needs only some of
// the allowed languages: at least one, each of them one of allowed.
func NewLocalizedTextFrom(field string, text map[string]string, maxLength int, allowed []string) (LocalizedText, error) {
	return newLocalizedText(field, text, maxLength, allowed, false)
}

func newLocalizedText(field string, text map[string]string, maxLength int, languages []string, requireAll bool) (LocalizedText, error) {
	result, problem := localizedTextProblem(text, maxLength, languages, requireAll)
	if problem != "" {
		return nil, NewValidationError(field, problem)
	}
	return result, nil
}

// localizedTextProblem returns text trimmed, or why it breaks
// newLocalizedText's rules — for a caller that reports the problem against
// a field of its own.
func localizedTextProblem(text map[string]string, maxLength int, languages []string, requireAll bool) (LocalizedText, string) {
	if len(text) == 0 {
		return nil, "must contain text in at least one language"
	}
	result := make(LocalizedText, len(text))
	// Walked in sorted order, so identical input always reports the same
	// problem first.
	for _, code := range LocalizedText(text).Languages() {
		trimmed, problem := localizedValue(code, text[code], maxLength, languages)
		if problem != "" {
			return nil, problem
		}
		result[code] = trimmed
	}
	if requireAll {
		for _, code := range languages {
			if _, ok := result[code]; !ok {
				return nil, fmt.Sprintf("is missing text in %q", code)
			}
		}
	}
	return result, ""
}

// localizedValue returns value trimmed, or why it can't be the text in
// language code.
func localizedValue(code, value string, maxLength int, languages []string) (trimmed, problem string) {
	if code == LanguageCodeAny {
		return "", fmt.Sprintf("must not use %q as a language", LanguageCodeAny)
	}
	if !slices.Contains(languages, code) {
		return "", fmt.Sprintf("has text in %q, which is not one of the allowed languages", code)
	}
	trimmed = strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Sprintf("must not be blank in %q", code)
	}
	if utf8.RuneCountInString(trimmed) > maxLength {
		return "", fmt.Sprintf("must be at most %d characters in %q", maxLength, code)
	}
	return trimmed, ""
}

// Resolve returns the text to show a viewer reading languageCode: the text in
// that language, else the English text, else the text in the alphabetically
// first language — never blank while any text exists.
func (t LocalizedText) Resolve(languageCode string) string {
	if text, ok := t[languageCode]; ok {
		return text
	}
	if text, ok := t["en"]; ok {
		return text
	}
	if codes := t.Languages(); len(codes) > 0 {
		return t[codes[0]]
	}
	return ""
}
