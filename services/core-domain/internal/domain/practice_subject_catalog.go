package domain

import "time"

// PracticeSubjectCatalog is what a list of practice subjects needs of the
// items classified for one scope that is the same for every student, built
// ahead of time so a list doesn't read every item. A scope is an instrument
// id, or empty for a student with no instruments.
type PracticeSubjectCatalog struct {
	Scope string
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

// PracticeCatalogSignalReason is why the practice subject catalog was asked
// to rebuild.
type PracticeCatalogSignalReason string

const (
	PracticeCatalogSignalDiagram    PracticeCatalogSignalReason = "diagram"
	PracticeCatalogSignalExercise   PracticeCatalogSignalReason = "exercise"
	PracticeCatalogSignalSongChart  PracticeCatalogSignalReason = "song_chart"
	PracticeCatalogSignalInstrument PracticeCatalogSignalReason = "instrument"
	// PracticeCatalogSignalStart is queued when the service starts, for
	// reference data installed by migrations and any signal lost in a crash.
	PracticeCatalogSignalStart PracticeCatalogSignalReason = "start"
	// PracticeCatalogSignalMissing is queued when a list finds a scope with no
	// catalog, or one of another format.
	PracticeCatalogSignalMissing PracticeCatalogSignalReason = "missing"
)
