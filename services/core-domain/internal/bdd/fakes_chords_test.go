//go:build integration

package bdd

import (
	"context"
	"slices"

	"github.com/motifpath/core-domain/internal/domain"
)

// fakeChordCatalog holds the chords and voicings a scenario installs, the
// way the chord catalog's migrations do, and reads them as the repository
// does: active voicings only, best first.
type fakeChordCatalog struct {
	chords map[string]domain.ChordDefinition
	// order keeps the chords in installation order, so lookups are stable.
	order []string
}

func newFakeChordCatalog() *fakeChordCatalog {
	return &fakeChordCatalog{chords: map[string]domain.ChordDefinition{}}
}

func (f *fakeChordCatalog) put(c domain.ChordDefinition) {
	if _, ok := f.chords[c.ID]; !ok {
		f.order = append(f.order, c.ID)
	}
	f.chords[c.ID] = c
}

func (f *fakeChordCatalog) GetChord(_ context.Context, id string) (domain.ChordDefinition, error) {
	c, ok := f.chords[id]
	if !ok {
		return domain.ChordDefinition{}, domain.ErrNotFound
	}
	return withActiveVoicings(c), nil
}

func (f *fakeChordCatalog) FindChord(_ context.Context, rootPitchClass int, quality domain.ChordQuality, bassPitchClass *int) (domain.ChordDefinition, error) {
	for _, id := range f.order {
		c := f.chords[id]
		sameBass := (c.BassPitchClass == nil && bassPitchClass == nil) ||
			(c.BassPitchClass != nil && bassPitchClass != nil && *c.BassPitchClass == *bassPitchClass)
		if c.RootPitchClass == rootPitchClass && c.Quality == quality && sameBass {
			return withActiveVoicings(c), nil
		}
	}
	return domain.ChordDefinition{}, domain.ErrNotFound
}

func (f *fakeChordCatalog) GetChords(ctx context.Context, ids []string) (map[string]domain.ChordDefinition, error) {
	found := map[string]domain.ChordDefinition{}
	for _, id := range ids {
		if c, err := f.GetChord(ctx, id); err == nil {
			found[id] = c
		}
	}
	return found, nil
}

// GetVoicings reads voicings by id, withdrawn ones included.
func (f *fakeChordCatalog) GetVoicings(_ context.Context, ids []string) (map[string]domain.ChordVoicing, error) {
	found := map[string]domain.ChordVoicing{}
	for _, chordID := range f.order {
		for _, v := range f.chords[chordID].Voicings {
			if slices.Contains(ids, v.ID) {
				found[v.ID] = v
			}
		}
	}
	return found, nil
}

func withActiveVoicings(c domain.ChordDefinition) domain.ChordDefinition {
	active := slices.DeleteFunc(slices.Clone(c.Voicings), func(v domain.ChordVoicing) bool { return v.Status != domain.ChordVoicingActive })
	slices.SortFunc(active, func(a, b domain.ChordVoicing) int { return a.RecommendedRank - b.RecommendedRank })
	c.Voicings = active
	return c
}
