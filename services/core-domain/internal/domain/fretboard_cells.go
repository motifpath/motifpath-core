package domain

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
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

// FretboardCell is one string and fret of a layout instrument's fretboard.
type FretboardCell struct {
	LayoutInstrumentID string
	// String is the cell's string, 1 being the highest-pitched; Fret its
	// fret, 0 the open string.
	String int
	Fret   int
}

// ParseFretboardCellItemKey reads the cell a fretboard cell item key names.
// It reports false for any other item key, or one with no layout, a string
// below 1 or a negative fret.
func ParseFretboardCellItemKey(key string) (FretboardCell, bool) {
	rest, ok := strings.CutPrefix(key, string(PracticeItemKindFretboardCell)+":")
	if !ok {
		return FretboardCell{}, false
	}
	parts := strings.Split(rest, ":")
	if len(parts) != 3 || parts[0] == "" {
		return FretboardCell{}, false
	}
	str, err := strconv.Atoi(parts[1])
	if err != nil || str < 1 {
		return FretboardCell{}, false
	}
	fret, err := strconv.Atoi(parts[2])
	if err != nil || fret < 0 {
		return FretboardCell{}, false
	}
	return FretboardCell{LayoutInstrumentID: parts[0], String: str, Fret: fret}, true
}

// FretboardMapCell is a cell of a fretboard map with the student's shown
// level on it. Fading is true once its review is due.
type FretboardMapCell struct {
	FretboardCell
	ItemKey string
	Level   KnowledgeLevel
	Fading  bool
}

// FretboardMap is how well a student knows each cell of an instrument's
// fretboard. LayoutInstrumentID is the instrument whose layout the cells
// are on, nil when the instrument has none.
type FretboardMap struct {
	InstrumentID       string
	LayoutInstrumentID *string
	Cells              []FretboardMapCell
}

// NewFretboardMap is the map of instrumentID's fretboard: each fretboard
// cell among items once, by string then fret, at its shown level from
// states at now, new when never practised. Items of any other kind are left
// out.
func NewFretboardMap(instrumentID string, items []ClassifiedItem, states map[string]PracticeItemState, now time.Time) FretboardMap {
	m := FretboardMap{InstrumentID: instrumentID, Cells: []FretboardMapCell{}}
	seen := map[string]bool{}
	for _, item := range items {
		cell, ok := ParseFretboardCellItemKey(item.ItemKey)
		if !ok || seen[item.ItemKey] {
			continue
		}
		seen[item.ItemKey] = true
		mapped := FretboardMapCell{FretboardCell: cell, ItemKey: item.ItemKey, Level: KnowledgeLevelNew}
		if state, ok := states[item.ItemKey]; ok {
			mapped.Level = state.ShownLevel(now)
			mapped.Fading = state.ReviewDue(now)
		}
		m.Cells = append(m.Cells, mapped)
	}
	slices.SortFunc(m.Cells, func(a, b FretboardMapCell) int {
		return cmp.Or(cmp.Compare(a.String, b.String), cmp.Compare(a.Fret, b.Fret))
	})
	if len(m.Cells) > 0 {
		m.LayoutInstrumentID = &m.Cells[0].LayoutInstrumentID
	}
	return m
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
