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

func TestNewLocalizedText_ReportsTheSameProblemEveryTime(t *testing.T) {
	// Two problems at once: a blank English value and an unoffered French one.
	// Map iteration order is random, so without a fixed order the reason would
	// change between identical requests.
	text := map[string]string{"en": "  ", "fr": "Guitare", "pt_BR": "Violão"}

	_, first := domain.NewLocalizedText("names", text, 200, []string{"en", "pt_BR"})
	var firstErr *domain.ValidationError
	require.ErrorAs(t, first, &firstErr)
	for range 50 {
		_, err := domain.NewLocalizedText("names", text, 200, []string{"en", "pt_BR"})
		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		require.Equal(t, firstErr.Fields[0].Reason, valErr.Fields[0].Reason)
	}
	assert.Equal(t, `must not be blank in "en"`, firstErr.Fields[0].Reason)
}

func TestLocalizedText_Resolve(t *testing.T) {
	names := domain.LocalizedText{"en": "Guitar", "pt_BR": "Violão"}

	assert.Equal(t, "Violão", names.Resolve("pt_BR"), "the viewer's language first")
	assert.Equal(t, "Guitar", domain.LocalizedText{"en": "Guitar"}.Resolve("pt_BR"), "then English")
	assert.Equal(t, "Guitarra", domain.LocalizedText{"pt_BR": "Violão", "es": "Guitarra"}.Resolve("fr"), "then the alphabetically first language")
	assert.Equal(t, "", domain.LocalizedText{}.Resolve("en"), "empty only when there is no text")
}
