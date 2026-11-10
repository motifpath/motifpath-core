package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestParseEnvironment(t *testing.T) {
	for _, name := range []string{"local", "test", "staging", "production"} {
		t.Run("accepted: "+name, func(t *testing.T) {
			env, err := domain.ParseEnvironment(name)

			require.NoError(t, err)
			assert.Equal(t, name, env.String())
		})
	}

	for name, value := range map[string]string{
		"no value":                       "",
		"an unknown deployment":          "qa",
		"a known name in different case": "Production",
		"a known name with spaces":       " staging ",
	} {
		t.Run("rejected: "+name, func(t *testing.T) {
			_, err := domain.ParseEnvironment(value)

			require.ErrorIs(t, err, domain.ErrUnknownEnvironment)
		})
	}
}
