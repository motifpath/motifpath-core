package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid"
)

// CourseInstrument is the join entity behind Course's many-to-many "instruments"
// edge: the instruments a course's live draft is for. None at all means it suits every
// instrument.
type CourseInstrument struct {
	ent.Schema
}

func (CourseInstrument) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("course_id", uuid.UUID{}).
			Immutable(),

		field.UUID("instrument_id", uuid.UUID{}).
			Immutable(),

		field.Time("linked_at").
			Immutable().
			Default(time.Now),
	}
}

func (CourseInstrument) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("course", Course.Type).
			Unique().
			Required().
			Immutable().
			Field("course_id"),

		edge.To("instrument", Instrument.Type).
			Unique().
			Required().
			Immutable().
			Field("instrument_id"),
	}
}

func (CourseInstrument) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("course_id", "instrument_id").Unique(),
	}
}
