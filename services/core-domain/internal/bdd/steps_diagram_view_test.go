package bdd

import (
	"fmt"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
)

// registerDiagramViewSteps covers how a usage shows its diagram: on the
// fretboard, unless its author chose a chord box for a chord voicing.
func registerDiagramViewSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)" adds diagram "([^"]+)" to "([^"]+)" with trigger_at_seconds (\d+) and hide_at_seconds (\d+), shown as "([^"]+)"$`,
		func(_, diagramSlug, nodeSlug string, trigger, hide int, view string) error {
			return w.addsDiagramShownAs(diagramSlug, nodeSlug, trigger, hide, view)
		})
	sc.Step(`^"([^"]+)" adds the diagram of voicing "([^"]+)" to "([^"]+)" with trigger_at_seconds (\d+) and hide_at_seconds (\d+), shown as "([^"]+)"$`,
		func(_, key, nodeSlug string, trigger, hide int, view string) error {
			return w.addsDiagramShownAs(voicingSlug(key), nodeSlug, trigger, hide, view)
		})
	sc.Step(`^"([^"]+)" adds a stack of the diagram of voicing "([^"]+)", shown as "([^"]+)", and diagram "([^"]+)" to "([^"]+)"$`, w.addsStackWithChordBox)
	sc.Step(`^the item's diagram is shown as "([^"]+)"$`, w.itemDiagramShownAs)
}

func (w *world) addsDiagramShownAs(diagramSlug, nodeSlug string, trigger, hide int, view string) error {
	ref := diagramRefWithIntervalsLayer(diagramSlug)
	shown := generated.DiagramRefView(view)
	ref.View = &shown
	resp, err := w.handler.CreateExpandedContent(w.ctx(), generated.CreateExpandedContentRequestObject{
		ContentNodeId: nodeID(nodeSlug),
		Body: &generated.CreateExpandedContentRequest{
			ContentType:      generated.CreateExpandedContentRequestContentTypeDiagram,
			DiagramRef:       &ref,
			TriggerAtSeconds: &trigger, HideAtSeconds: &hide,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) addsStackWithChordBox(_, key, view, diagramSlug, nodeSlug string) error {
	first := diagramRefWithIntervalsLayer(voicingSlug(key))
	shown := generated.DiagramRefView(view)
	first.View = &shown
	stack := generated.DiagramStackRef{Stack: []generated.DiagramRef{first, diagramRefWithIntervalsLayer(diagramSlug)}}
	trigger, hide := 150, 165
	resp, err := w.handler.CreateExpandedContent(w.ctx(), generated.CreateExpandedContentRequestObject{
		ContentNodeId: nodeID(nodeSlug),
		Body: &generated.CreateExpandedContentRequest{
			ContentType:      generated.CreateExpandedContentRequestContentTypeDiagram,
			DiagramStackRef:  &stack,
			TriggerAtSeconds: &trigger, HideAtSeconds: &hide,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) itemDiagramShownAs(want string) error {
	resp, ok := w.lastResp.(generated.CreateExpandedContent201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a created item, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.DiagramRef == nil {
		return fmt.Errorf("expected the item to carry a diagram_ref")
	}
	got := string(generated.Fretboard)
	if resp.DiagramRef.View != nil {
		got = string(*resp.DiagramRef.View)
	}
	if got != want {
		return fmt.Errorf("expected the item's diagram shown as %q, got %q", want, got)
	}
	return nil
}
