package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// ChordChangePair is two chord drills practised as a minute of changes, back
// and forth, one change on every beat (on bass, between their roots). A pair is
// one item for both directions. Pairs are reference data, installed only by
// migrations with fixed IDs.
type ChordChangePair struct {
	ent.Schema
}

func (ChordChangePair) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Immutable(),

		// from_chord_drill_id is the chord the item key names first.
		field.UUID("from_chord_drill_id", uuid.UUID{}).
			Immutable(),

		field.UUID("to_chord_drill_id", uuid.UUID{}).
			Immutable(),

		// ordinal is the pair's place in the catalog, from 1.
		field.Int("ordinal").
			Positive().
			Unique().
			Immutable(),
	}
}

func (ChordChangePair) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("from_chord_drill_id", "to_chord_drill_id").Unique(),
	}
}
