package domain_test

import (
	"encoding/json"
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
	tests := []struct {
		name       string
		diagramID  string
		layers     domain.DiagramLayers
		playback   *domain.DiagramRefPlayback
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
			name:      "playback with its own tempo, voice and looping",
			diagramID: "diagram-1",
			layers:    validDiagramLayers(),
			playback:  &domain.DiagramRefPlayback{Direction: domain.DiagramPlaybackDirectionReversed, TempoBPM: intPtr(60), VoiceID: strPtr("acoustic-guitar"), Loop: true},
		},
		{
			name:      "playback at the boundary tempos",
			diagramID: "diagram-1",
			layers:    validDiagramLayers(),
			playback:  &domain.DiagramRefPlayback{Direction: domain.DiagramPlaybackDirectionAsAuthored, TempoBPM: intPtr(20)},
		},
		{
			name:      "playback at the fastest tempo",
			diagramID: "diagram-1",
			layers:    validDiagramLayers(),
			playback:  &domain.DiagramRefPlayback{Direction: domain.DiagramPlaybackDirectionAsAuthored, TempoBPM: intPtr(300)},
		},
		{
			name:       "playback with invalid direction",
			diagramID:  "diagram-1",
			layers:     validDiagramLayers(),
			playback:   &domain.DiagramRefPlayback{Direction: "sideways"},
			wantField:  "playback",
			wantErrMsg: "direction",
		},
		{
			name:       "playback slower than 20 BPM",
			diagramID:  "diagram-1",
			layers:     validDiagramLayers(),
			playback:   &domain.DiagramRefPlayback{Direction: domain.DiagramPlaybackDirectionAsAuthored, TempoBPM: intPtr(19)},
			wantField:  "playback",
			wantErrMsg: "tempo_bpm",
		},
		{
			name:       "playback faster than 300 BPM",
			diagramID:  "diagram-1",
			layers:     validDiagramLayers(),
			playback:   &domain.DiagramRefPlayback{Direction: domain.DiagramPlaybackDirectionAsAuthored, TempoBPM: intPtr(301)},
			wantField:  "playback",
			wantErrMsg: "tempo_bpm",
		},
		{
			name:       "playback with an empty voice id",
			diagramID:  "diagram-1",
			layers:     validDiagramLayers(),
			playback:   &domain.DiagramRefPlayback{Direction: domain.DiagramPlaybackDirectionAsAuthored, VoiceID: strPtr("")},
			wantField:  "playback",
			wantErrMsg: "voice_id",
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

// TestDiagramPlayback_StoredBeforeTempoAndVoice pins that a playback saved
// before it had a tempo, voice and loop — when it carried step_ms instead —
// still loads: step_ms is dropped and the new fields read as unset.
func TestDiagramPlayback_StoredBeforeTempoAndVoice(t *testing.T) {
	var ref domain.DiagramRef

	err := json.Unmarshal([]byte(`{"diagram_id":"diagram-1","layers":{},"playback":{"direction":"reversed","step_ms":500}}`), &ref)

	require.NoError(t, err)
	require.NotNil(t, ref.Playback)
	assert.Equal(t, domain.DiagramRefPlayback{Direction: domain.DiagramPlaybackDirectionReversed}, *ref.Playback)
	require.NoError(t, domain.ValidateDiagramRef(ref))
}

// TestDiagramPlayback_WithoutDirection pins that a playback naming no
// direction plays as authored, wherever the ref arrives from — a usage's own
// field or a diagram embedded in a document.
func TestDiagramPlayback_WithoutDirection(t *testing.T) {
	var doc domain.PromptDocument

	err := json.Unmarshal([]byte(`{"type":"doc","content":[{"type":"diagram","attrs":{"diagramRef":{"diagram_id":"diagram-1","layers":{},"playback":{"tempo_bpm":60}}}}]}`), &doc)

	require.NoError(t, err)
	ref := doc.Content[0].Attrs.DiagramRef
	require.NotNil(t, ref)
	require.NotNil(t, ref.Playback)
	assert.Equal(t, domain.DiagramPlaybackDirectionAsAuthored, ref.Playback.Direction)
	require.NoError(t, domain.ValidateDiagramRef(*ref))
}

func TestDiagramRefPlayback_PlaybackChoice(t *testing.T) {
	t.Run("a usage saved before choosing a playback plays the default", func(t *testing.T) {
		var ref domain.DiagramRef

		err := json.Unmarshal([]byte(`{"diagram_id":"diagram-1","layers":{},"playback":{"direction":"as_authored"}}`), &ref)

		require.NoError(t, err)
		require.NotNil(t, ref.Playback)
		assert.Nil(t, ref.Playback.PlaybackID)
	})

	t.Run("a usage keeps the playback it chose", func(t *testing.T) {
		var ref domain.DiagramRef

		err := json.Unmarshal([]byte(`{"diagram_id":"diagram-1","layers":{},"playback":{"playback_id":"pb-arp"}}`), &ref)

		require.NoError(t, err)
		require.NotNil(t, ref.Playback.PlaybackID)
		assert.Equal(t, "pb-arp", *ref.Playback.PlaybackID)
	})
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
