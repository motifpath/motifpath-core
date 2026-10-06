package domain

import (
	"errors"
	"fmt"
	"slices"
)

// ErrInvalidDrillCatalog reports a practice drill catalog entry that can't be
// installed, such as fretboard cells on a string the instrument doesn't have.
var ErrInvalidDrillCatalog = errors.New("invalid practice drill catalog")

// FretboardCellItemKey is the item key of a fretboard cell: the layout
// instrument whose fretboard it is on, its string (1 being the
// highest-pitched) and its fret (0 the open string). Instruments sharing a
// layout share the cell, so a student knows one fretboard, not one per
// instrument.
func FretboardCellItemKey(layoutInstrumentID string, str, fret int) string {
	return fmt.Sprintf("%s:%s:%d:%d", PracticeItemKindFretboardCell, layoutInstrumentID, str, fret)
}

// FretboardCellRange is a practice drill catalog entry: the strings and the
// frets of a fretboard layout whose cells serve a skill. Its cells are
// generated, never authored: every string and fret in it is a practice item.
type FretboardCellRange struct {
	ID                 string
	SkillID            string
	LayoutInstrumentID string
	// Strings lists the range's strings, 1 being the highest-pitched, in the
	// order its cells are listed.
	Strings  []int
	FromFret int
	ToFret   int
}

// Items returns the range's cells as practice items classified under its
// skill, string by string in the order listed, then by fret.
func (r FretboardCellRange) Items() []ClassifiedItem {
	items := make([]ClassifiedItem, 0, len(r.Strings)*(r.ToFret-r.FromFret+1))
	for _, str := range r.Strings {
		for fret := r.FromFret; fret <= r.ToFret; fret++ {
			items = append(items, ClassifiedItem{
				ItemKey: FretboardCellItemKey(r.LayoutInstrumentID, str, fret),
				NodeIDs: []string{r.SkillID},
			})
		}
	}
	return items
}

// CheckFits reports, as ErrInvalidDrillCatalog, why the range can't be
// installed on layout for skill: it names another layout, a string layout
// doesn't have, frets out of order, or a skill that isn't for layout. A
// skill for every instrument suits any layout.
func (r FretboardCellRange) CheckFits(layout Instrument, skill KnowledgeNode) error {
	if r.LayoutInstrumentID != layout.ID {
		return fmt.Errorf("%w: the cells of skill %s belong to layout %s, not %s", ErrInvalidDrillCatalog, r.SkillID, r.LayoutInstrumentID, layout.ID)
	}
	if len(skill.InstrumentIDs) > 0 && !slices.Contains(skill.InstrumentIDs, layout.ID) {
		return fmt.Errorf("%w: skill %s is not for layout %s", ErrInvalidDrillCatalog, skill.ID, layout.ID)
	}
	if r.FromFret < 0 || r.ToFret < r.FromFret {
		return fmt.Errorf("%w: layout %s frets %d to %d are out of order", ErrInvalidDrillCatalog, layout.ID, r.FromFret, r.ToFret)
	}
	for _, str := range r.Strings {
		if layout.StringCount == nil || str < 1 || str > *layout.StringCount {
			return fmt.Errorf("%w: layout %s has no string %d", ErrInvalidDrillCatalog, layout.ID, str)
		}
	}
	return nil
}
