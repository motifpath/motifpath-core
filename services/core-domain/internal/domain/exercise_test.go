package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func diagramPromptNode(ref *domain.DiagramRef, stack *domain.DiagramStackRef) domain.PromptNode {
	return domain.PromptNode{
		Type: domain.PromptNodeTypeDiagram,
		Attrs: &domain.PromptNodeAttrs{
			DiagramRef:      ref,
			DiagramStackRef: stack,
		},
	}
}

func minimalExerciseArgs() (title string, exerciseType domain.ExerciseType, skillIDs, conceptIDs []string, options []domain.Option, languageCodes []string, createdAt time.Time) {
	label := "A major"
	return "Title", domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"},
		[]domain.Option{{ID: "opt-1", IsCorrect: true, Label: &label}}, []string{"en"}, time.Now()
}

func TestNewExercise_PromptWithInlineDiagram(t *testing.T) {
	title, exerciseType, skillIDs, conceptIDs, options, languageCodes, createdAt := minimalExerciseArgs()

	validRef := &domain.DiagramRef{DiagramID: "diagram-1", Layers: domain.DiagramLayers{Intervals: true}}
	validStack := &domain.DiagramStackRef{Stack: []domain.DiagramRef{
		{DiagramID: "diagram-1", Layers: domain.DiagramLayers{Intervals: true}},
		{DiagramID: "diagram-2", Layers: domain.DiagramLayers{Intervals: true}},
	}}

	tests := []struct {
		name      string
		node      domain.PromptNode
		wantValid bool
	}{
		{name: "diagram node with diagram_ref is valid", node: diagramPromptNode(validRef, nil), wantValid: true},
		{name: "diagram node with diagram_stack_ref is valid", node: diagramPromptNode(nil, validStack), wantValid: true},
		{name: "diagram node with neither ref is invalid", node: diagramPromptNode(nil, nil), wantValid: false},
		{name: "diagram node with both refs is invalid", node: diagramPromptNode(validRef, validStack), wantValid: false},
		{name: "diagram node with an invalid diagram_ref is invalid", node: diagramPromptNode(&domain.DiagramRef{Layers: domain.DiagramLayers{Intervals: true}}, nil), wantValid: false},
		{name: "diagram node with an invalid diagram_stack_ref is invalid", node: diagramPromptNode(nil, &domain.DiagramStackRef{Stack: []domain.DiagramRef{*validRef}}), wantValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := domain.PromptDocument{Type: "doc", Content: []domain.PromptNode{
				{Type: domain.PromptNodeTypeParagraph, Content: []domain.PromptNode{tt.node}},
			}}

			_, err := domain.NewExercise("ex-1", title, prompt, exerciseType, skillIDs, conceptIDs, nil, nil, nil, nil, options, nil, nil, languageCodes, createdAt)
			if tt.wantValid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				var valErr *domain.ValidationError
				require.ErrorAs(t, err, &valErr)
			}
		})
	}
}

func TestNewExercise_ImageRecognitionStimulus(t *testing.T) {
	imageURL := "https://cdn.example.com/img.png"
	diagramRef := &domain.DiagramRef{DiagramID: "diagram-1", Layers: domain.DiagramLayers{Intervals: true}, CorrectIntervals: &[]string{"1"}}
	diagramStackRef := &domain.DiagramStackRef{Stack: []domain.DiagramRef{
		{DiagramID: "diagram-1", Layers: domain.DiagramLayers{Intervals: true}, CorrectIntervals: &[]string{"1"}},
		{DiagramID: "diagram-2", Layers: domain.DiagramLayers{Intervals: true}, CorrectIntervals: &[]string{"1"}},
	}}
	derivedOptions := []domain.Option{
		{ID: "opt-1", IsCorrect: true, DiagramID: strPtr("diagram-1"), DiagramPositionID: strPtr("pos-1")},
	}
	regionOptions := imageRecognitionRegionOptions()

	tests := []struct {
		name            string
		imageURL        *string
		diagramRef      *domain.DiagramRef
		diagramStackRef *domain.DiagramStackRef
		options         []domain.Option
		wantValid       bool
	}{
		{name: "image_url alone is valid", imageURL: &imageURL, options: regionOptions, wantValid: true},
		{name: "diagram_ref alone is valid", diagramRef: diagramRef, options: derivedOptions, wantValid: true},
		{name: "diagram_stack_ref alone is valid", diagramStackRef: diagramStackRef, options: derivedOptions, wantValid: true},
		{name: "none of the three is invalid", options: regionOptions, wantValid: false},
		{name: "image_url and diagram_ref together is invalid", imageURL: &imageURL, diagramRef: diagramRef, options: regionOptions, wantValid: false},
		{name: "diagram_ref and diagram_stack_ref together is invalid", diagramRef: diagramRef, diagramStackRef: diagramStackRef, options: derivedOptions, wantValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := domain.NewPlainTextPrompt("prompt")
			exercise, err := domain.NewExercise("ex-1", "Title", prompt, domain.ExerciseTypeImageRecognition, []string{"skill-1"}, []string{"concept-1"}, tt.imageURL, nil, tt.diagramRef, tt.diagramStackRef, tt.options, nil, nil, []string{"en"}, time.Now())
			if tt.wantValid {
				require.NoError(t, err)
				assert.Equal(t, tt.diagramRef, exercise.DiagramRef)
				assert.Equal(t, tt.diagramStackRef, exercise.DiagramStackRef)
				return
			}
			require.Error(t, err)
			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
		})
	}
}

func TestNewExercise_ImageChoiceOptionDiagramRef(t *testing.T) {
	imageURL := "https://cdn.example.com/img.png"
	validRef := &domain.DiagramRef{DiagramID: "diagram-1", Layers: domain.DiagramLayers{Intervals: true}}
	invalidRef := &domain.DiagramRef{Layers: domain.DiagramLayers{Intervals: true}}

	tests := []struct {
		name      string
		option    domain.Option
		wantValid bool
	}{
		{name: "image_url alone is valid", option: domain.Option{ID: "opt-1", IsCorrect: true, ImageURL: &imageURL}, wantValid: true},
		{name: "diagram_ref alone is valid", option: domain.Option{ID: "opt-1", IsCorrect: true, DiagramRef: validRef}, wantValid: true},
		{name: "neither is invalid", option: domain.Option{ID: "opt-1", IsCorrect: true}, wantValid: false},
		{name: "both is invalid", option: domain.Option{ID: "opt-1", IsCorrect: true, ImageURL: &imageURL, DiagramRef: validRef}, wantValid: false},
		{name: "a structurally invalid diagram_ref is invalid", option: domain.Option{ID: "opt-1", IsCorrect: true, DiagramRef: invalidRef}, wantValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := domain.NewPlainTextPrompt("prompt")
			_, err := domain.NewExercise("ex-1", "Title", prompt, domain.ExerciseTypeImageChoice, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, []domain.Option{tt.option}, nil, nil, []string{"en"}, time.Now())
			if tt.wantValid {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
		})
	}
}

func imageRecognitionRegionOptions() []domain.Option {
	return []domain.Option{
		{ID: "opt-1", IsCorrect: true, Region: &domain.OptionRegion{X: 0.2, Y: 0.3, Width: 0.1, Height: 0.1, Shape: domain.OptionRegionShapeRectangle}},
	}
}
