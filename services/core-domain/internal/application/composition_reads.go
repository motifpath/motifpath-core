package application

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// A composition, or a list of subjects, reads every item the student's
// knowledge maps hold, so what it reads is kept for it: the diagrams, the
// exercises, the instruments, the progression and chord drill catalogs and the
// guitar layout are each read once, however many items name them. A later composition reads them again,
// so it finds what changed since.

// composingDiagrams is the diagrams one composition reads, each once.
type composingDiagrams struct {
	ports.DiagramRepository
	mu sync.Mutex
	// read holds each diagram read, by id; nil for an id with no diagram.
	read map[string]*domain.Diagram
}

func newComposingDiagrams(diagrams ports.DiagramRepository) *composingDiagrams {
	return &composingDiagrams{DiagramRepository: diagrams, read: map[string]*domain.Diagram{}}
}

func (c *composingDiagrams) GetByID(ctx context.Context, id string) (domain.Diagram, error) {
	c.mu.Lock()
	d, ok := c.read[id]
	c.mu.Unlock()
	if !ok {
		got, err := c.DiagramRepository.GetByID(ctx, id)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return domain.Diagram{}, err
		}
		if err == nil {
			d = &got
		}
		c.mu.Lock()
		c.read[id] = d
		c.mu.Unlock()
	}
	if d == nil {
		return domain.Diagram{}, domain.ErrNotFound
	}
	return *d, nil
}

// readAll reads the diagrams the play-along and diagram shape items among
// items name, in one read, so none of them is read alone later.
func (c *composingDiagrams) readAll(ctx context.Context, items []domain.ClassifiedItem) error {
	ids := itemIDs(items, domain.PracticeItemKindPlayAlong, domain.PracticeItemKindDiagramShape)
	if len(ids) == 0 {
		return nil
	}
	found, err := c.GetByIDs(ctx, ids)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, id := range ids {
		if d, ok := found[id]; ok {
			c.read[id] = &d
		} else {
			c.read[id] = nil
		}
	}
	return nil
}

// composingExercises is the exercises one composition reads, each once.
type composingExercises struct {
	ports.ExerciseRepository
	mu sync.Mutex
	// read holds each exercise read, by id; nil for an id with no exercise.
	read map[string]*domain.Exercise
}

func newComposingExercises(exercises ports.ExerciseRepository) *composingExercises {
	return &composingExercises{ExerciseRepository: exercises, read: map[string]*domain.Exercise{}}
}

func (c *composingExercises) GetByID(ctx context.Context, id string) (domain.Exercise, error) {
	c.mu.Lock()
	e, ok := c.read[id]
	c.mu.Unlock()
	if !ok {
		got, err := c.ExerciseRepository.GetByID(ctx, id)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return domain.Exercise{}, err
		}
		if err == nil {
			e = &got
		}
		c.mu.Lock()
		c.read[id] = e
		c.mu.Unlock()
	}
	if e == nil {
		return domain.Exercise{}, domain.ErrNotFound
	}
	return *e, nil
}

// readAll reads the exercises the items among items name, in one read, so
// none of them is read alone later.
func (c *composingExercises) readAll(ctx context.Context, items []domain.ClassifiedItem) error {
	ids := itemIDs(items, domain.PracticeItemKindExercise)
	if len(ids) == 0 {
		return nil
	}
	found, err := c.GetByIDs(ctx, ids)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, id := range ids {
		if e, ok := found[id]; ok {
			c.read[id] = &e
		} else {
			c.read[id] = nil
		}
	}
	return nil
}

// itemIDs is the ids the items of kinds among items name, each once.
func itemIDs(items []domain.ClassifiedItem, kinds ...domain.PracticeItemKind) []string {
	seen := map[string]bool{}
	var ids []string
	for _, item := range items {
		for _, kind := range kinds {
			id, ok := strings.CutPrefix(item.ItemKey, string(kind)+":")
			if ok && !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids
}

// composingChordDrills is the chord drill catalog, read once for one
// composition.
type composingChordDrills struct {
	catalog ports.ChordDrillRepository
	once    sync.Once
	read    domain.ChordDrillCatalog
	err     error
}

func (c *composingChordDrills) Catalog(ctx context.Context) (domain.ChordDrillCatalog, error) {
	c.once.Do(func() { c.read, c.err = c.catalog.Catalog(ctx) })
	return c.read, c.err
}

// composingInstruments is the instruments one composition reads, each once.
type composingInstruments struct {
	ports.InstrumentRepository
	mu   sync.Mutex
	read map[string]instrumentRead
}

type instrumentRead struct {
	instrument domain.Instrument
	err        error
}

func newComposingInstruments(instruments ports.InstrumentRepository) *composingInstruments {
	return &composingInstruments{InstrumentRepository: instruments, read: map[string]instrumentRead{}}
}

// GetByID keeps an instrument, or its absence, but not another error.
func (c *composingInstruments) GetByID(ctx context.Context, id string) (domain.Instrument, error) {
	c.mu.Lock()
	r, ok := c.read[id]
	c.mu.Unlock()
	if ok {
		return r.instrument, r.err
	}
	instrument, err := c.InstrumentRepository.GetByID(ctx, id)
	if err == nil || errors.Is(err, domain.ErrNotFound) {
		c.mu.Lock()
		c.read[id] = instrumentRead{instrument: instrument, err: err}
		c.mu.Unlock()
	}
	return instrument, err
}

// composingProgressions is the progression catalog, read once for one
// composition.
type composingProgressions struct {
	progressions ports.ProgressionRepository
	once         sync.Once
	list         []domain.Progression
	err          error
}

func (c *composingProgressions) List(ctx context.Context) ([]domain.Progression, error) {
	c.once.Do(func() { c.list, c.err = c.progressions.List(ctx) })
	return c.list, c.err
}

// guitarLayoutRead is the guitar layout, read once for one composition.
type guitarLayoutRead struct {
	once sync.Once
	id   string
	err  error
}

// forComposition is h reading the progression catalog and the guitar layout
// once.
func (h *harmonyDrills) forComposition() *harmonyDrills {
	composing := *h
	composing.progressions = &composingProgressions{progressions: h.progressions}
	composing.guitar = &guitarLayoutRead{}
	return &composing
}
