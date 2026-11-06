package domain

import "time"

// Progression is a chord progression by number: one numeral per bar of a key
// in Mode, such as I, vi or V7, in a key the drill or chart using it picks.
// Confusable lists the keys of the progressions a learner mistakes it for. The
// catalog's are reference data, installed by migration; admins create the
// others, which reach the drills only when InDrills.
type Progression struct {
	ID         string
	Key        string
	Mode       string
	Bars       []string
	NodeIDs    []string
	Confusable []string
	Names      LocalizedText
	InDrills   bool
	// Nicknames are a created progression's optional nicknames per language.
	Nicknames map[string]string
	// CreatedBy and CreatedAt are nil on the catalog's progressions.
	CreatedBy *string
	CreatedAt *time.Time
}

// BorrowedBars are the bars, numbered from 0, whose numeral is outside the
// mode's diatonic chords; never nil.
func (p Progression) BorrowedBars() []int {
	borrowed := []int{}
	for i, bar := range p.Bars {
		if n, ok := ParseProgressionNumeral(bar, ProgressionMode(p.Mode)); ok && n.Borrowed {
			borrowed = append(borrowed, i)
		}
	}
	return borrowed
}
