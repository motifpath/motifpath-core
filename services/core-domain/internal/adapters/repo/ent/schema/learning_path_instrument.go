package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid"
)

// LearningPathInstrument is the join entity behind LearningPath's many-to-many "instruments"
// edge: the instruments a learning path is for. None at all means it suits every
// instrument.
type LearningPathInstrument struct {
	ent.Schema
}

func (LearningPathInstrument) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("learning_path_id", uuid.UUID{}).
			Immutable(),

		field.UUID("instrument_id", uuid.UUID{}).
			Immutable(),

		field.Time("linked_at").
			Immutable().
			Default(time.Now),
	}
}

func (LearningPathInstrument) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("learning_path", LearningPath.Type).
			Unique().
			Required().
			Immutable().
			Field("learning_path_id"),

		edge.To("instrument", Instrument.Type).
			Unique().
			Required().
			Immutable().
			Field("instrument_id"),
	}
}

func (LearningPathInstrument) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("learning_path_id", "instrument_id").Unique(),
	}
}
