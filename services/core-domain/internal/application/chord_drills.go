package application

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// WithChordDrills installs the chord drills: single chords and minutes of
// changes between two chords on guitar, and on bass a chord's root and fifth
// and changes between two roots, all played with the instrument in hand.
func (s *PracticeSessionService) WithChordDrills(catalog ports.ChordDrillRepository, instruments ports.InstrumentRepository) *PracticeSessionService {
	return s.WithDrillSources(chordSource{catalog: catalog, instruments: instruments, now: s.now})
}

// chordSource finds the chord drill items the knowledge map classifies under
// a skill. They are played, so a session in the head finds none.
type chordSource struct {
	catalog     ports.ChordDrillRepository
	instruments ports.InstrumentRepository
	now         func() time.Time
}

func (src chordSource) SkillItems(ctx context.Context, skillID, _ string, inHand bool, knowledge KnowledgeMap) ([]domain.DrillItem, error) {
	if !inHand {
		return nil, nil
	}
	var items []domain.DrillItem
	var catalog *domain.ChordDrillCatalog
	for _, classified := range knowledge.Items {
		if !slices.Contains(classified.NodeIDs, skillID) {
			continue
		}
		item, ok, err := src.item(ctx, classified.ItemKey, &catalog)
		if err != nil {
			return nil, err
		}
		if ok {
			items = append(items, item)
		}
	}
	return items, nil
}

func (src chordSource) Item(ctx context.Context, itemKey string) (domain.DrillItem, bool, error) {
	var catalog *domain.ChordDrillCatalog
	return src.item(ctx, itemKey, &catalog)
}

// item reads itemKey as a chord drill item, loading the chord drill catalog
// into catalog the first time it is needed. A guitar item names catalog
// voicings; a bass item names a layout and chord symbols. A change is found
// only while a change target is in force.
func (src chordSource) item(ctx context.Context, itemKey string, catalog **domain.ChordDrillCatalog) (domain.DrillItem, bool, error) {
	kind, rest, ok := strings.Cut(itemKey, ":")
	if !ok {
		return nil, false, nil
	}
	read, ok := chordItemReaders[domain.PracticeItemKind(kind)]
	parts := strings.Split(rest, ":")
	if !ok || len(parts) != read.parts {
		return nil, false, nil
	}
	loaded, err := src.load(ctx, catalog)
	if err != nil {
		return nil, false, err
	}
	return read.read(ctx, src, loaded, parts)
}

// chordItemReader reads the parts of one chord kind's item key, after its
// kind.
type chordItemReader struct {
	parts int
	read  func(ctx context.Context, src chordSource, catalog domain.ChordDrillCatalog, parts []string) (domain.DrillItem, bool, error)
}

var chordItemReaders = map[domain.PracticeItemKind]chordItemReader{
	domain.PracticeItemKindChordVoicing: {parts: 1, read: func(_ context.Context, _ chordSource, catalog domain.ChordDrillCatalog, parts []string) (domain.DrillItem, bool, error) {
		drill, found := chordDrillOf(catalog, parts[0])
		return domain.ChordVoicingItem{Drill: drill}, found, nil
	}},
	domain.PracticeItemKindChordChange: {parts: 2, read: func(_ context.Context, src chordSource, catalog domain.ChordDrillCatalog, parts []string) (domain.DrillItem, bool, error) {
		from, fromFound := chordDrillOf(catalog, parts[0])
		to, toFound := chordDrillOf(catalog, parts[1])
		target, inForce := src.targetInForce(catalog)
		return domain.ChordChangeItem{From: from, To: to, TargetChangesPerMinute: target}, fromFound && toFound && inForce, nil
	}},
	domain.PracticeItemKindRootFifth: {parts: 2, read: func(ctx context.Context, src chordSource, _ domain.ChordDrillCatalog, parts []string) (domain.DrillItem, bool, error) {
		layout, found, err := src.layout(ctx, parts[0])
		if !found || err != nil {
			return nil, false, err
		}
		item, err := domain.NewRootFifthItem(layout, parts[1])
		return item, err == nil, nil
	}},
	domain.PracticeItemKindRootChange: {parts: 3, read: func(ctx context.Context, src chordSource, catalog domain.ChordDrillCatalog, parts []string) (domain.DrillItem, bool, error) {
		target, inForce := src.targetInForce(catalog)
		layout, found, err := src.layout(ctx, parts[0])
		if !found || !inForce || err != nil {
			return nil, false, err
		}
		item, err := domain.NewRootChangeItem(layout, parts[1], parts[2], target)
		return item, err == nil, nil
	}},
}

// load reads the chord drill catalog into catalog the first time.
func (src chordSource) load(ctx context.Context, catalog **domain.ChordDrillCatalog) (domain.ChordDrillCatalog, error) {
	if *catalog == nil {
		loaded, err := src.catalog.Catalog(ctx)
		if err != nil {
			return domain.ChordDrillCatalog{}, err
		}
		*catalog = &loaded
	}
	return **catalog, nil
}

// targetInForce is the change target version in force now; false before the
// first.
func (src chordSource) targetInForce(catalog domain.ChordDrillCatalog) (int, bool) {
	now := src.now()
	var inForce *domain.ChangeTarget
	for i, t := range catalog.Targets {
		if !t.EffectiveFrom.After(now) && (inForce == nil || t.EffectiveFrom.After(inForce.EffectiveFrom)) {
			inForce = &catalog.Targets[i]
		}
	}
	if inForce == nil {
		return 0, false
	}
	return inForce.ChangesPerMinute, true
}

// layout reads the bass layout instrument id names; false when there is none.
func (src chordSource) layout(ctx context.Context, id string) (domain.Instrument, bool, error) {
	layout, err := src.instruments.GetByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Instrument{}, false, nil
	}
	return layout, err == nil, err
}

// chordDrillOf is the catalog's chord drill playing diagramID; false when
// none does.
func chordDrillOf(catalog domain.ChordDrillCatalog, diagramID string) (domain.ChordDrill, bool) {
	i := slices.IndexFunc(catalog.Drills, func(d domain.ChordDrill) bool { return d.DiagramID == diagramID })
	if i < 0 {
		return domain.ChordDrill{}, false
	}
	return catalog.Drills[i], true
}
