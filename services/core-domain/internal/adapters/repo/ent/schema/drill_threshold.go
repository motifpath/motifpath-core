package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// DrillThreshold is one version of a timed template's fluent time: the time
// a fluent student spends knowing the answer, net of their tap time and of
// any audio heard once. A version is never edited; a new one starts at a
// later effective_from, and each answer is judged by the version in force
// when it was given.
type DrillThreshold struct {
	ent.Schema
}

func (DrillThreshold) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Immutable(),

		field.UUID("template_id", uuid.UUID{}).
			Immutable(),

		field.Int("version").
			Positive().
			Immutable(),

		field.Time("effective_from").
			Immutable(),

		field.Int("fluent_net_ms").
			Positive().
			Immutable(),

		// source is default (team-set), benchmark (twice the team's median
		// net time) or calibrated (from felt-rated sessions).
		field.Enum("source").
			Values("default", "benchmark", "calibrated").
			Immutable(),

		// sessions and students are the data behind a calibrated version;
		// 0 for the others.
		field.Int("sessions").
			NonNegative().
			Immutable(),

		field.Int("students").
			NonNegative().
			Immutable(),
	}
}

func (DrillThreshold) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("template", DrillTemplate.Type).
			Unique().
			Required().
			Immutable().
			Field("template_id"),
	}
}

func (DrillThreshold) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("template_id", "version").Unique(),
	}
}
