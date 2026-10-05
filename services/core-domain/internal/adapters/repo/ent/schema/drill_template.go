package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// DrillTemplate is a way of asking about practice items: what is shown, the
// response the student gives, and whether the answer is timed. Fluent
// times are kept per template, never per item. Templates are reference
// data, installed only by migrations with fixed IDs.
type DrillTemplate struct {
	ent.Schema
}

func (DrillTemplate) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Immutable(),

		// key is the template's stable name, such as exercise:text_response
		// or fretboard_cell:name_the_note.
		field.String("key").
			MaxLen(100).
			NotEmpty().
			Unique().
			Immutable(),

		field.String("item_kind").
			MaxLen(50).
			NotEmpty().
			Immutable(),

		field.String("response_type").
			MaxLen(50).
			NotEmpty().
			Immutable(),

		field.Bool("timed").
			Immutable(),

		field.JSON("names", map[string]string{}),
	}
}

func (DrillTemplate) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("thresholds", DrillThreshold.Type).
			Ref("template"),
	}
}
