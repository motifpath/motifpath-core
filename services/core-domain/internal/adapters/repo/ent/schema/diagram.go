package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Diagram is a prebuilt, reusable set of positions on one instrument. It
// stores structured positions only, never a rendered image. Its skill and
// concept classification is many-to-many against the shared Skill/Concept
// trees, independent of ContentNode's and Exercise's own links to them.
type Diagram struct {
	ent.Schema
}

func (Diagram) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		// instrument_id can't change after creation: every position's
		// coordinate shape depends on the instrument's family.
		field.UUID("instrument_id", uuid.UUID{}).
			Immutable(),

		field.String("name"),

		field.String("root_note").
			Optional().
			Nillable(),

		field.Enum("label_display").
			Values("interval", "note", "hidden").
			Default("interval"),

		field.Time("created_at").
			Immutable().
			Default(time.Now),
	}
}

func (Diagram) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("instrument", Instrument.Type).
			Unique().
			Required().
			Immutable().
			Field("instrument_id"),

		edge.To("positions", Position.Type),

		edge.To("skills", Skill.Type).
			Through("diagram_skills", DiagramSkill.Type),

		edge.To("concepts", Concept.Type).
			Through("diagram_concepts", DiagramConcept.Type),
	}
}
