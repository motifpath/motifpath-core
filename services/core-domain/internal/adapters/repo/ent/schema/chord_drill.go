package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// ChordDrill is a chord practised with the instrument in hand: its open voicing
// fretted, strummed once and rated clean (on bass, its root and fifth). Chord
// drills are reference data, installed only by migrations with fixed IDs.
type ChordDrill struct {
	ent.Schema
}

func (ChordDrill) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Immutable(),

		// diagram_id is the chord catalog voicing diagram played.
		field.UUID("diagram_id", uuid.UUID{}).
			Unique().
			Immutable(),

		// chord is the voicing's chord symbol, such as Am: on bass, the chord
		// whose root and fifth are played.
		field.String("chord").
			MaxLen(16).
			NotEmpty().
			Immutable(),

		// ordinal is the drill's place in the catalog, from 1.
		field.Int("ordinal").
			Positive().
			Unique().
			Immutable(),
	}
}
