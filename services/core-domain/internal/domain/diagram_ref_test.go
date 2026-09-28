package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func labelPtr(l domain.DiagramLabel) *domain.DiagramLabel { return &l }

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
			name:      "every label mode is valid",
			diagramID: "diagram-1",
			layers:    domain.DiagramLayers{Label: labelPtr(domain.DiagramLabelCustom)},
		},
		{
			name:       "an unknown label mode is invalid",
			diagramID:  "diagram-1",
			layers:     domain.DiagramLayers{Label: labelPtr("shouty")},
			wantField:  "layers",
			wantErrMsg: "label",
		},
		{
			name:      "hidden positions need no check here",
			diagramID: "diagram-1",
			layers:    domain.DiagramLayers{HiddenPositionIDs: &[]string{"p-1"}},
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

func TestFrettedAnswerCells(t *testing.T) {
	pos := func(str, fret int) domain.Position { return domain.Position{String: &str, Fret: &fret} }
	band := func(from, to int) domain.Region { return domain.Region{FretStart: &from, FretEnd: &to} }
	frets := func(cells []domain.FretCell) []int {
		seen := map[int]bool{}
		out := []int{}
		for _, c := range cells {
			if !seen[c.Fret] {
				seen[c.Fret] = true
				out = append(out, c.Fret)
			}
		}
		return out
	}

	tests := []struct {
		name      string
		positions []domain.Position
		regions   []domain.Region
		wantFrets []int
	}{
		{name: "one fret either side of the positions", positions: []domain.Position{pos(6, 5), pos(4, 7), pos(6, 8)}, wantFrets: []int{5, 6, 7, 8, 9}},
		{name: "at least three frets", positions: []domain.Position{pos(3, 7)}, wantFrets: []int{7, 8, 9}},
		{name: "a window reaching the nut adds the open strings", positions: []domain.Position{pos(6, 0), pos(5, 2), pos(3, 1)}, wantFrets: []int{0, 1, 2, 3}},
		{name: "a first-fret shape reaches the nut too", positions: []domain.Position{pos(2, 1), pos(1, 3)}, wantFrets: []int{0, 1, 2, 3, 4}},
		{name: "regions widen the window", positions: []domain.Position{pos(6, 5)}, regions: []domain.Region{band(5, 9)}, wantFrets: []int{5, 6, 7, 8, 9, 10}},
		{name: "no positions or regions is the first three frets and the open strings", wantFrets: []int{0, 1, 2, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cells := domain.FrettedAnswerCells(tt.positions, tt.regions, 6)

			assert.Equal(t, tt.wantFrets, frets(cells))
			assert.Len(t, cells, 6*len(tt.wantFrets))
		})
	}

	t.Run("cells run string by string within each fret, lowest fret first", func(t *testing.T) {
		cells := domain.FrettedAnswerCells([]domain.Position{pos(1, 7)}, nil, 2)

		assert.Equal(t, []domain.FretCell{
			{String: 1, Fret: 7}, {String: 2, Fret: 7},
			{String: 1, Fret: 8}, {String: 2, Fret: 8},
			{String: 1, Fret: 9}, {String: 2, Fret: 9},
		}, cells)
	})
}
