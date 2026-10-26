package application

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// WithGuideTones installs the guide tones: each progression of the progression
// catalog tapped in time on a fretboard layout. It reads the progressions and
// the chord catalog the harmony drills were installed with, so it does nothing
// without them.
func (s *PracticeSessionService) WithGuideTones(instruments ports.InstrumentRepository) *PracticeSessionService {
	if s.harmony == nil {
		return s
	}
	return s.WithDrillSources(guideToneSource{harmony: s.harmony, instruments: instruments})
}

// guideToneSource finds the guide tones the knowledge map classifies under a
// skill. They are tapped on the screen, so an in-hand session finds none.
type guideToneSource struct {
	harmony     *harmonyDrills
	instruments ports.InstrumentRepository
}

func (src guideToneSource) composing(s *PracticeSessionService) DrillSource {
	return guideToneSource{harmony: s.harmony, instruments: newComposingInstruments(src.instruments)}
}

func (src guideToneSource) SkillItems(ctx context.Context, skillID, _ string, inHand bool, knowledge KnowledgeMap) ([]domain.DrillItem, error) {
	if inHand {
		return nil, nil
	}
	var items []domain.DrillItem
	for _, classified := range knowledge.Items {
		if !slices.Contains(classified.NodeIDs, skillID) {
			continue
		}
		item, ok, err := src.Item(ctx, classified.ItemKey)
		if err != nil {
			return nil, err
		}
		if ok {
			items = append(items, item)
		}
	}
	return items, nil
}

// Item reads itemKey as a guide tone: its layout and its progression.
func (src guideToneSource) Item(ctx context.Context, itemKey string) (domain.DrillItem, bool, error) {
	rest, ok := strings.CutPrefix(itemKey, string(domain.PracticeItemKindGuideTone)+":")
	if !ok {
		return nil, false, nil
	}
	layoutID, progressionKey, ok := strings.Cut(rest, ":")
	if !ok {
		return nil, false, nil
	}
	progressions, err := src.harmony.progressions.List(ctx)
	if err != nil {
		return nil, false, err
	}
	i := slices.IndexFunc(progressions, func(p domain.Progression) bool { return p.Key == progressionKey })
	if i < 0 {
		return nil, false, nil
	}
	layout, err := src.instruments.GetByID(ctx, layoutID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return domain.GuideToneItem{Layout: layout, Progression: progressions[i]}, true, nil
}
