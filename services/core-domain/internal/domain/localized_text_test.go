package domain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestNewLocalizedText(t *testing.T) {
	offered := []string{"en", "pt_BR"}

	t.Run("text in exactly the required languages is kept, trimmed", func(t *testing.T) {
		got, err := domain.NewLocalizedText("names", map[string]string{"en": " Guitar ", "pt_BR": "Violão"}, 200, offered)

		require.NoError(t, err)
		assert.Equal(t, domain.LocalizedText{"en": "Guitar", "pt_BR": "Violão"}, got)
		assert.Equal(t, []string{"en", "pt_BR"}, got.Languages())
	})

	tests := []struct {
		name string
		text map[string]string
		max  int
	}{
		{name: "no text at all", text: map[string]string{}, max: 200},
		{name: "a required language missing", text: map[string]string{"en": "Guitar"}, max: 200},
		{name: "a language nobody offers", text: map[string]string{"en": "Guitar", "pt_BR": "Violão", "fr": "Guitare"}, max: 200},
		{name: `the "any" marker as a language`, text: map[string]string{"en": "Guitar", "pt_BR": "Violão", "any": "Guitar"}, max: 200},
		{name: "a blank value", text: map[string]string{"en": "Guitar", "pt_BR": "   "}, max: 200},
		{name: "a value over the limit", text: map[string]string{"en": "Guitar", "pt_BR": strings.Repeat("á", 3)}, max: 2},
	}
	for _, tt := range tests {
		t.Run("rejected: "+tt.name, func(t *testing.T) {
			_, err := domain.NewLocalizedText("names", tt.text, tt.max, offered)

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, "names", valErr.Fields[0].Field)
		})
	}

	t.Run("the limit counts characters, not bytes", func(t *testing.T) {
		_, err := domain.NewLocalizedText("names", map[string]string{"en": "ab", "pt_BR": "áé"}, 2, offered)

		require.NoError(t, err)
	})
}
