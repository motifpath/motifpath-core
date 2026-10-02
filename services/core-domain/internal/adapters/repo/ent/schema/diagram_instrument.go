package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
	"time"
)

// DiagramInstrument links a diagram to an instrument that shares its
// coordinate geometry. The Diagram.instrument_id remains the immutable layout
// instrument used to validate positions and select the fallback playback voice.
type DiagramInstrument struct {
	ent.Schema
}

func (DiagramInstrument) Fields() []ent.Field {
	return []ent.Field{
		field.Time("linked_at").
			Immutable().
			Default(time.Now),
		field.UUID("diagram_id", uuid.UUID{}).Immutable(),
		field.UUID("instrument_id", uuid.UUID{}).Immutable(),
	}
}

func (DiagramInstrument) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("diagram", Diagram.Type).
			Unique().
			Required().
			Immutable().
			Field("diagram_id"),
		edge.To("instrument", Instrument.Type).
			Unique().
			Required().
			Immutable().
			Field("instrument_id"),
	}
}

func (DiagramInstrument) Indexes() []ent.Index {
	return []ent.Index{index.Fields("diagram_id", "instrument_id").Unique()}
}
