package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestHasMatchingLanguage(t *testing.T) {
	cases := []struct {
		name      string
		languages []domain.Language
		locale    string
		want      bool
	}{
		{"exact match", []domain.Language{{Code: "en"}, {Code: "pt_BR"}}, "pt_BR", true},
		{"no match", []domain.Language{{Code: "en"}}, "pt_BR", false},
		{"any always matches", []domain.Language{{Code: domain.LanguageCodeAny}}, "pt_BR", true},
		{"empty languages never match", []domain.Language{}, "en", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, domain.HasMatchingLanguage(tc.languages, tc.locale))
		})
	}
}
