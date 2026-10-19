package domain

import (
	"fmt"
	"slices"
)

// PracticeRung is the rung of the practice ladder an item trains: what the
// student does with the thing practised, from recalling it to using it in
// music.
type PracticeRung string

const (
	// PracticeRungKnow recalls a fact: a note, a numeral, a shape.
	PracticeRungKnow PracticeRung = "know"
	// PracticeRungHear recognises it by ear.
	PracticeRungHear PracticeRung = "hear"
	// PracticeRungPlayClean plays it cleanly, untimed.
	PracticeRungPlayClean PracticeRung = "play_clean"
	// PracticeRungPlayInTime plays it at a tempo.
	PracticeRungPlayInTime PracticeRung = "play_in_time"
	// PracticeRungMove plays it in another key or place.
	PracticeRungMove PracticeRung = "move"
	// PracticeRungTarget aims at it while the music moves.
	PracticeRungTarget PracticeRung = "target"
	// PracticeRungUse plays it in a piece of music.
	PracticeRungUse PracticeRung = "use"
)

// PracticeRungs lists the ladder's rungs, lowest first.
var PracticeRungs = []PracticeRung{
	PracticeRungKnow, PracticeRungHear, PracticeRungPlayClean, PracticeRungPlayInTime,
	PracticeRungMove, PracticeRungTarget, PracticeRungUse,
}

// Valid reports whether r is a rung of the ladder.
func (r PracticeRung) Valid() bool {
	return slices.Contains(PracticeRungs, r)
}

// DrillTemplate is a way of asking about practice items: the kind of item it
// asks, the response the student gives, the rung it trains and, for a
// generated drill asked among options, its second way: the other template
// asking the same item, so a lucky guess meets the other way. Reference
// data, installed by migration.
type DrillTemplate struct {
	ID           string
	Key          string
	ItemKind     PracticeItemKind
	ResponseType string
	Rung         PracticeRung
	SecondWay    *string
	// ComesBack is whether an item asked this way is asked this way again
	// later in the same session, after at least three other items.
	ComesBack bool
	Timed     bool
	Names     LocalizedText
}

// CheckDrillTemplates reports, as ErrInvalidDrillCatalog, the first template
// that can't be installed with the others: one with no rung on the ladder,
// or whose second way is itself, no template's, or a template asking another
// kind of item.
func CheckDrillTemplates(templates []DrillTemplate) error {
	byKey := make(map[string]DrillTemplate, len(templates))
	for _, t := range templates {
		byKey[t.Key] = t
	}
	for _, t := range templates {
		if !t.Rung.Valid() {
			return fmt.Errorf("%w: template %s has no rung of the ladder (%q)", ErrInvalidDrillCatalog, t.Key, t.Rung)
		}
		if t.SecondWay == nil {
			continue
		}
		other, ok := byKey[*t.SecondWay]
		switch {
		case *t.SecondWay == t.Key:
			return fmt.Errorf("%w: template %s is its own second way", ErrInvalidDrillCatalog, t.Key)
		case !ok:
			return fmt.Errorf("%w: template %s names a second way no template has, %s", ErrInvalidDrillCatalog, t.Key, *t.SecondWay)
		case other.ItemKind != t.ItemKind:
			return fmt.Errorf("%w: template %s asks %s items, but its second way %s asks %s items", ErrInvalidDrillCatalog, t.Key, t.ItemKind, other.Key, other.ItemKind)
		}
	}
	return nil
}

// ItemRung is the rung item trains: its drill template's, from templates by
// key; a play-along's is playing in time. An item whose template isn't
// among templates is recall.
func ItemRung(item PracticeSessionItem, templates map[string]DrillTemplate) PracticeRung {
	if item.PlayAlong != nil {
		return PracticeRungPlayInTime
	}
	if rung, rated := selfRatedChordKinds[item.Kind]; rated {
		return rung
	}
	if t, ok := templates[item.DrillTemplateKey()]; ok {
		return t.Rung
	}
	return PracticeRungKnow
}
