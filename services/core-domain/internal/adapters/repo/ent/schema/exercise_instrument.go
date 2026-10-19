package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid"
)

// ExerciseInstrument is the join entity behind Exercise's many-to-many "instruments"
// edge: the instruments an exercise is for. None at all means it suits every
// instrument.
type ExerciseInstrument struct {
	ent.Schema
}

func (ExerciseInstrument) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("exercise_id", uuid.UUID{}).
			Immutable(),

		field.UUID("instrument_id", uuid.UUID{}).
			Immutable(),

		field.Time("linked_at").
			Immutable().
			Default(time.Now),
	}
}

func (ExerciseInstrument) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("exercise", Exercise.Type).
			Unique().
			Required().
			Immutable().
			Field("exercise_id"),

		edge.To("instrument", Instrument.Type).
			Unique().
			Required().
			Immutable().
			Field("instrument_id"),
	}
}

func (ExerciseInstrument) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("exercise_id", "instrument_id").Unique(),
	}
}
