package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func validDiagramLayers() domain.DiagramLayers {
	return domain.DiagramLayers{Intervals: true}
}

func TestNewDiagramRef(t *testing.T) {
	stepMs := 500

	tests := []struct {
		name       string
		diagramID  string
		layers     domain.DiagramLayers
		playback   *domain.DiagramPlayback
		wantField  string
		wantErrMsg string
	}{
		{
			name:      "valid minimal ref",
			diagramID: "diagram-1",
			layers:    validDiagramLayers(),
		},
		{
			name:      "missing diagram_id",
			diagramID: "",
			layers:    validDiagramLayers(),
			wantField: "diagram_id",
		},
		{
			name:       "playback with invalid direction",
			diagramID:  "diagram-1",
			layers:     validDiagramLayers(),
			playback:   &domain.DiagramPlayback{Direction: "sideways", StepMs: stepMs},
			wantField:  "playback",
			wantErrMsg: "direction",
		},
		{
			name:       "playback with non-positive step_ms",
			diagramID:  "diagram-1",
			layers:     validDiagramLayers(),
			playback:   &domain.DiagramPlayback{Direction: domain.DiagramPlaybackDirectionAsAuthored, StepMs: 0},
			wantField:  "playback",
			wantErrMsg: "step_ms",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := domain.DiagramRef{DiagramID: tt.diagramID, Layers: tt.layers, Playback: tt.playback}
			err := domain.ValidateDiagramRef(ref)
			if tt.wantField == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			require.Len(t, valErr.Fields, 1)
			assert.Equal(t, tt.wantField, valErr.Fields[0].Field)
			if tt.wantErrMsg != "" {
				assert.Contains(t, valErr.Fields[0].Reason, tt.wantErrMsg)
			}
		})
	}
}

func TestNewDiagramStackRef(t *testing.T) {
	validRef := func(id string) domain.DiagramRef {
		return domain.DiagramRef{DiagramID: id, Layers: validDiagramLayers()}
	}

	tests := []struct {
		name      string
		stack     []domain.DiagramRef
		wantField string
	}{
		{
			name:  "valid stack of two",
			stack: []domain.DiagramRef{validRef("diagram-1"), validRef("diagram-2")},
		},
		{
			name:      "single entry is rejected",
			stack:     []domain.DiagramRef{validRef("diagram-1")},
			wantField: "stack",
		},
		{
			name:      "empty stack is rejected",
			stack:     []domain.DiagramRef{},
			wantField: "stack",
		},
		{
			name:      "an invalid entry surfaces its own error",
			stack:     []domain.DiagramRef{validRef("diagram-1"), validRef("")},
			wantField: "diagram_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateDiagramStackRef(domain.DiagramStackRef{Stack: tt.stack})
			if tt.wantField == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			require.Len(t, valErr.Fields, 1)
			assert.Equal(t, tt.wantField, valErr.Fields[0].Field)
		})
	}
}
