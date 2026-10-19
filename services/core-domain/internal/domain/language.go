package domain

import "strings"

// LanguageCodeAny marks content as language-agnostic (e.g. an image with no
// spoken or written words) rather than belonging to a specific language.
const LanguageCodeAny = "any"

// Language is a language MotifPath content or a user's locale preference can
// be tagged with.
type Language struct {
	Code string
	Name string
}

// HasMatchingLanguage reports whether languages contains locale or the
// language-agnostic LanguageCodeAny marker. Used to decide whether a
// ContentNode or Exercise is available to a student resolved to locale.
func HasMatchingLanguage(languages []Language, locale string) bool {
	for _, lang := range languages {
		if lang.Code == locale || lang.Code == LanguageCodeAny {
			return true
		}
	}
	return false
}

// languagesFromCodes builds placeholder Language values carrying only Code —
// used when constructing a domain entity from request-supplied codes before
// it has been persisted and read back with its Language rows (and their
// Name) joined in.
func languagesFromCodes(codes []string) []Language {
	result := make([]Language, len(codes))
	for i, code := range codes {
		result[i] = Language{Code: code}
	}
	return result
}

// NormalizeAcceptLanguage extracts the highest-priority language tag from an
// HTTP Accept-Language header value and normalizes it to a Language.Code
// candidate (e.g. "pt-BR" -> "pt_BR"). Returns "" if header is empty,
// unparseable, or a wildcard. The candidate is not guaranteed to name a known
// Language — the caller must still look it up.
func NormalizeAcceptLanguage(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}
	first := strings.SplitN(header, ",", 2)[0]
	tag := strings.TrimSpace(strings.SplitN(first, ";", 2)[0])
	if tag == "" || tag == "*" {
		return ""
	}
	return strings.ReplaceAll(tag, "-", "_")
}

// validateLanguageCodes checks that codes is non-empty — content must be
// explicitly tagged as either a specific language or language-agnostic
// (LanguageCodeAny), never left unclassified.
func validateLanguageCodes(field string, codes []string) []FieldError {
	if len(codes) == 0 {
		return []FieldError{{Field: field, Reason: "must contain at least one language code"}}
	}
	for _, code := range codes {
		if code == "" {
			return []FieldError{{Field: field, Reason: "each code must be a non-empty string"}}
		}
	}
	return nil
}
