package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// FretboardCellRange is a practice drill catalog entry: the strings and frets
// of a fretboard layout whose generated cells serve a skill. Reference data,
// installed by migration with a fixed id and never edited.
type FretboardCellRange struct {
	ent.Schema
}

func (FretboardCellRange) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Immutable(),

		field.UUID("skill_id", uuid.UUID{}).
			Immutable(),

		// layout_instrument_id is the instrument whose fretboard the cells
		// are on; every instrument of the same geometry shares them.
		field.UUID("layout_instrument_id", uuid.UUID{}).
			Immutable(),

		// strings lists the range's strings, 1 being the highest-pitched, in
		// the order its cells are listed.
		field.Ints("strings").
			Immutable(),

		field.Int("from_fret").
			NonNegative().
			Immutable(),

		field.Int("to_fret").
			NonNegative().
			Immutable(),
	}
}

func (FretboardCellRange) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("skill", KnowledgeNode.Type).
			Unique().
			Required().
			Immutable().
			Field("skill_id"),
		edge.To("layout_instrument", Instrument.Type).
			Unique().
			Required().
			Immutable().
			Field("layout_instrument_id"),
	}
}

func (FretboardCellRange) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("skill_id", "layout_instrument_id").Unique(),
	}
}
