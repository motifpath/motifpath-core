package application

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// WithProgressionKeyTours installs the progression key tours: each progression
// of the progression catalog played along with, key after key, on a fretboard
// layout. It reads the progressions and the chord catalog the harmony drills
// were installed with, so it does nothing without them.
func (s *PracticeSessionService) WithProgressionKeyTours(instruments ports.InstrumentRepository) *PracticeSessionService {
	if s.harmony == nil {
		return s
	}
	return s.WithDrillSources(tourSource{harmony: s.harmony, instruments: instruments})
}

// tourSource finds the tours the knowledge map classifies under a skill. They
// are played, so a session in the head finds none.
type tourSource struct {
	harmony     *harmonyDrills
	instruments ports.InstrumentRepository
}

func (src tourSource) composing(s *PracticeSessionService) DrillSource {
	return tourSource{harmony: s.harmony, instruments: newComposingInstruments(src.instruments)}
}

func (src tourSource) SkillItems(ctx context.Context, skillID, _ string, inHand bool, knowledge KnowledgeMap) ([]domain.DrillItem, error) {
	if !inHand {
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

// Item reads itemKey as a tour: its layout and its progression. The layout the
// chord catalog's voicings are on is the guitar one; any other is a bass, whose
// tours mark each chord's tones.
func (src tourSource) Item(ctx context.Context, itemKey string) (domain.DrillItem, bool, error) {
	rest, ok := strings.CutPrefix(itemKey, string(domain.PracticeItemKindProgressionKeys)+":")
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
	guitar, err := src.harmony.guitarLayoutID(ctx)
	if err != nil {
		return nil, false, err
	}
	return domain.ProgressionKeysItem{Layout: layout, Bass: layout.ID != guitar, Progression: progressions[i]}, true, nil
}
