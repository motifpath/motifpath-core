package application

import (
	"context"
	"slices"
	"strings"

	"github.com/motifpath/core-domain/internal/domain"
)

// DrillSource finds the items of one generated drill family. A session
// composes every family's items alike, so a new family is a new source, and
// never a change to the composer.
type DrillSource interface {
	// SkillItems lists the family's items classified under skillID that
	// suit a session with instrumentID in hand (inHand) or in the head;
	// knowledge is the student's knowledge map for instrumentID.
	SkillItems(ctx context.Context, skillID, instrumentID string, inHand bool, knowledge KnowledgeMap) ([]domain.DrillItem, error)
	// Item loads the item itemKey names. It reports false for an item of
	// another family, and for one that no longer exists.
	Item(ctx context.Context, itemKey string) (domain.DrillItem, bool, error)
}

// fretboardCellSource finds fretboard cells: generated from the drill
// catalog's cell ranges, they are the knowledge map's items.
type fretboardCellSource struct{}

func (fretboardCellSource) SkillItems(_ context.Context, skillID, _ string, _ bool, knowledge KnowledgeMap) ([]domain.DrillItem, error) {
	var cells []domain.DrillItem
	for _, item := range knowledge.Items {
		if !slices.Contains(item.NodeIDs, skillID) {
			continue
		}
		if cell, ok := domain.ParseFretboardCellItemKey(item.ItemKey); ok {
			cells = append(cells, cell)
		}
	}
	return cells, nil
}

func (fretboardCellSource) Item(_ context.Context, itemKey string) (domain.DrillItem, bool, error) {
	cell, ok := domain.ParseFretboardCellItemKey(itemKey)
	return cell, ok, nil
}

// diagramShapeSource finds diagram shapes: the catalog diagrams the drill
// catalog makes shapes, recalled in the head only.
type diagramShapeSource struct {
	service *PracticeSessionService
}

func (src diagramShapeSource) composing(s *PracticeSessionService) DrillSource {
	return diagramShapeSource{service: s}
}

func (src diagramShapeSource) SkillItems(ctx context.Context, skillID, instrumentID string, inHand bool, _ KnowledgeMap) ([]domain.DrillItem, error) {
	if inHand || instrumentID == "" {
		return nil, nil
	}
	diagrams, err := src.service.listDiagrams(ctx, domain.DiagramListFilter{SkillIDs: []string{skillID}, InstrumentID: instrumentID, Kind: domain.DiagramKindBasic, Purpose: domain.DiagramPurposeFilterGeneral})
	if err != nil {
		return nil, err
	}
	var shapes []domain.DrillItem
	for _, d := range diagrams {
		if d.Shape != nil {
			shapes = append(shapes, domain.DiagramShapeItem{Diagram: d})
		}
	}
	return shapes, nil
}

func (src diagramShapeSource) Item(ctx context.Context, itemKey string) (domain.DrillItem, bool, error) {
	id, ok := strings.CutPrefix(itemKey, string(domain.PracticeItemKindDiagramShape)+":")
	if !ok {
		return nil, false, nil
	}
	d, found, err := src.service.diagram(ctx, id)
	if err != nil || !found || d.Shape == nil {
		return nil, false, err
	}
	return domain.DiagramShapeItem{Diagram: d}, true, nil
}
