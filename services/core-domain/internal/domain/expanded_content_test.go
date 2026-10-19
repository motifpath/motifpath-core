package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func validDiagramRefForExpandedContent() *domain.DiagramRef {
	return &domain.DiagramRef{DiagramID: "diagram-1", Layers: domain.DiagramLayers{Intervals: true}}
}

func TestNewExpandedContent_Diagram(t *testing.T) {
	tests := []struct {
		name            string
		diagramRef      *domain.DiagramRef
		diagramStackRef *domain.DiagramStackRef
		mediaURL        *string
		richContent     *domain.PromptDocument
		wantValid       bool
		wantField       string
	}{
		{
			name:       "a diagram item with a diagram_ref is valid",
			diagramRef: validDiagramRefForExpandedContent(),
			wantValid:  true,
		},
		{
			name: "a diagram item with a diagram_stack_ref is valid",
			diagramStackRef: &domain.DiagramStackRef{Stack: []domain.DiagramRef{
				*validDiagramRefForExpandedContent(),
				{DiagramID: "diagram-2", Layers: domain.DiagramLayers{Intervals: true}},
			}},
			wantValid: true,
		},
		{
			name:      "a diagram item with neither diagram_ref nor diagram_stack_ref is invalid",
			wantValid: false,
			wantField: "diagram_ref",
		},
		{
			name:            "a diagram item with both diagram_ref and diagram_stack_ref is invalid",
			diagramRef:      validDiagramRefForExpandedContent(),
			diagramStackRef: &domain.DiagramStackRef{Stack: []domain.DiagramRef{*validDiagramRefForExpandedContent()}},
			wantValid:       false,
			wantField:       "diagram_ref",
		},
		{
			name:       "a diagram item that also carries a media_url is invalid",
			diagramRef: validDiagramRefForExpandedContent(),
			mediaURL:   strPtr("https://cdn.example.com/img.png"),
			wantValid:  false,
			wantField:  "media_url",
		},
		{
			name:        "a diagram item that also carries rich_content is invalid",
			diagramRef:  validDiagramRefForExpandedContent(),
			richContent: &domain.PromptDocument{Type: "doc"},
			wantValid:   false,
			wantField:   "rich_content",
		},
		{
			name:       "a diagram item whose diagram_ref is structurally invalid is invalid",
			diagramRef: &domain.DiagramRef{Layers: domain.DiagramLayers{Intervals: true}},
			wantValid:  false,
			wantField:  "diagram_ref",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := domain.NewExpandedContent(
				"item-1", "node-1", domain.ContentTypeVideo, domain.ExpandedContentTypeDiagram,
				tt.mediaURL, tt.richContent, tt.diagramRef, tt.diagramStackRef,
				intPtr(150), intPtr(165), nil, nil, nil, time.Now(),
			)

			if tt.wantValid {
				require.NoError(t, err)
				return
			}
			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			found := false
			for _, f := range valErr.Fields {
				if f.Field == tt.wantField {
					found = true
				}
			}
			assert.True(t, found, "expected field %q among %+v", tt.wantField, valErr.Fields)
		})
	}

	t.Run("a non-diagram item that carries a diagram_ref is invalid", func(t *testing.T) {
		_, err := domain.NewExpandedContent(
			"item-1", "node-1", domain.ContentTypeVideo, domain.ExpandedContentTypeImage,
			strPtr("https://cdn.example.com/img.png"), nil, validDiagramRefForExpandedContent(), nil,
			intPtr(150), intPtr(165), nil, nil, nil, time.Now(),
		)

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, "diagram_ref", valErr.Fields[0].Field)
	})
}

