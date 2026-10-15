package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// failingProgressions is a progression catalog that can't be read.
type failingProgressions struct{}

func (failingProgressions) List(context.Context) ([]domain.Progression, error) {
	return nil, errors.New("catalog unavailable")
}

func TestProgressionCatalogService_List(t *testing.T) {
	pop := domain.Progression{Key: "I-V-vi-IV", Bars: []string{"I", "V", "vi", "IV"}, Confusable: []string{"I-vi-IV-V"}}
	blues := domain.Progression{Key: "12-bar-blues", Bars: []string{"I7", "I7", "I7", "I7", "IV7", "IV7", "I7", "I7", "V7", "IV7", "I7", "V7"}}
	doowop := domain.Progression{Key: "I-vi-IV-V", Bars: []string{"I", "vi", "IV", "V"}, Confusable: []string{"I-V-vi-IV"}}

	tests := []struct {
		name    string
		catalog fakeProgressions
		want    []string
	}{
		{name: "every progression, ordered by key", catalog: fakeProgressions{pop, blues, doowop}, want: []string{"12-bar-blues", "I-V-vi-IV", "I-vi-IV-V"}},
		{name: "an empty catalog lists none", catalog: fakeProgressions{}, want: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := application.NewProgressionCatalogService(tt.catalog).List(context.Background())

			require.NoError(t, err)
			keys := make([]string, len(got))
			for i, p := range got {
				keys[i] = p.Key
			}
			assert.Equal(t, tt.want, keys)
		})
	}

	t.Run("a catalog that can't be read is an error", func(t *testing.T) {
		_, err := application.NewProgressionCatalogService(failingProgressions{}).List(context.Background())

		assert.Error(t, err)
	})
}
