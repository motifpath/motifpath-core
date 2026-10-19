package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestNewPageRequest(t *testing.T) {
	tests := []struct {
		name      string
		limit     *int
		offset    *int
		want      domain.PageRequest
		wantField string
	}{
		{name: "defaults apply when both are absent", want: domain.PageRequest{Limit: 20, Offset: 0}},
		{name: "explicit values are kept", limit: intPtr(50), offset: intPtr(100), want: domain.PageRequest{Limit: 50, Offset: 100}},
		{name: "the maximum limit is allowed", limit: intPtr(100), want: domain.PageRequest{Limit: 100, Offset: 0}},
		{name: "a limit of zero is rejected", limit: intPtr(0), wantField: "limit"},
		{name: "a limit above the maximum is rejected", limit: intPtr(101), wantField: "limit"},
		{name: "a negative offset is rejected", offset: intPtr(-1), wantField: "offset"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewPageRequest(tt.limit, tt.offset)

			if tt.wantField != "" {
				var valErr *domain.ValidationError
				require.ErrorAs(t, err, &valErr)
				require.Len(t, valErr.Fields, 1)
				assert.Equal(t, tt.wantField, valErr.Fields[0].Field)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
