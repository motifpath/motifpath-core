package domain

import (
	"slices"
	"time"
)

// PracticeSubjectCatalog is what a list of practice subjects needs of the
// items classified for one scope that is the same for every student, built
// ahead of time so a list doesn't read every item. A scope is an instrument
// id, or empty for a student with no instruments.
type PracticeSubjectCatalog struct {
	Scope string
	// ContentVersion is the content version the catalog was built from; one
	// built from an older version is not read.
	ContentVersion int64
	// FormatVersion is the version of how items were tallied when the catalog
	// was built; a catalog of another version is not read.
	FormatVersion int
	BuiltAt       time.Time
	Items         []PracticeSubjectCatalogItem
}

// PracticeSubjectCatalogItem is one classified item of a catalog: the nodes
// it is classified under, the sessions it suits and the rung it trains.
type PracticeSubjectCatalogItem struct {
	ItemKey string
	NodeIDs []string
	InHand  bool
	InHead  bool
	// Rung is the rung the item trains, when it has no Ways.
	Rung PracticeRung
	// Ways are a drill item's ways of being asked, in order, each with the
	// rung it trains.
	Ways []PracticeSubjectCatalogWay
	// Languages are the language codes an exercise is available in; empty
	// for any other item, which is shown in every locale.
	Languages []string
}

// PracticeSubjectCatalogWay is one way a drill item is asked, and the rung it
// trains.
type PracticeSubjectCatalogWay struct {
	Way  string
	Rung PracticeRung
}

// Suits reports whether the item suits a session with the instrument in hand
// (inHand) or in the head.
func (i PracticeSubjectCatalogItem) Suits(inHand bool) bool {
	if inHand {
		return i.InHand
	}
	return i.InHead
}

// OfferedIn reports whether the item can be offered to a student in locale:
// an exercise only when it is available in locale or in any language.
func (i PracticeSubjectCatalogItem) OfferedIn(locale string) bool {
	return len(i.Languages) == 0 || slices.ContainsFunc(i.Languages, func(code string) bool {
		return code == locale || code == LanguageCodeAny
	})
}

// RungFor is the rung the item trains for a student whose state on it is
// state: a drill item's is the rung of the way they would be asked next.
func (i PracticeSubjectCatalogItem) RungFor(state *PracticeItemState) PracticeRung {
	if len(i.Ways) == 0 {
		return i.Rung
	}
	ways := make([]string, len(i.Ways))
	for j, w := range i.Ways {
		ways[j] = w.Way
	}
	next := NextWay(ways, state)
	for _, w := range i.Ways {
		if w.Way == next {
			return w.Rung
		}
	}
	return i.Ways[0].Rung
}
