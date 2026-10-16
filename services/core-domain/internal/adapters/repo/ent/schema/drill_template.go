package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// DrillTemplate is a way of asking about practice items: what is shown, the
// response the student gives, the rung of the practice ladder it trains, the
// other template asking the same item, if any, whether the item also comes
// back later in the session, and whether the answer is timed. Fluent
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

		// rung is the rung of the practice ladder the template trains, such
		// as know or play_in_time.
		field.String("rung").
			MaxLen(20).
			NotEmpty().
			Immutable(),

		// second_way is the key of the other template asking the same kind
		// of item, so an item answered by a lucky guess is asked the other
		// way; null when the template has none.
		field.String("second_way").
			MaxLen(100).
			Optional().
			Nillable().
			Immutable(),

		// comes_back is whether an item asked this way comes back later in
		// the same session, asked this way again, besides any second way.
		field.Bool("comes_back").
			Default(false).
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
