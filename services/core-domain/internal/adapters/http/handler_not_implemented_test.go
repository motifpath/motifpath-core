package http_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appHTTP "github.com/motifpath/core-domain/internal/adapters/http"
	"github.com/motifpath/core-domain/internal/adapters/http/generated"
)

// Operations whose contract is published ahead of their implementation fail
// as not implemented, and never answer as if they had worked.
func TestOperationsNotImplementedYet(t *testing.T) {
	h := healthHandler(nil, nil)
	ctx := context.Background()
	tests := []struct {
		name string
		call func() error
	}{
		{"listPracticeSubjects", func() error {
			resp, err := h.ListPracticeSubjects(ctx, generated.ListPracticeSubjectsRequestObject{})
			assert.Nil(t, resp)
			return err
		}},
		{"getAlphaObservation", func() error {
			resp, err := h.GetAlphaObservation(ctx, generated.GetAlphaObservationRequestObject{})
			assert.Nil(t, resp)
			return err
		}},
		{"getAlphaStudentObservation", func() error {
			resp, err := h.GetAlphaStudentObservation(ctx, generated.GetAlphaStudentObservationRequestObject{})
			assert.Nil(t, resp)
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorIs(t, tt.call(), appHTTP.ErrNotImplemented)
		})
	}
}
