package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// ChangeTarget is one version of the clean changes a minute that count as fully
// fluent on a chord or root change, the same for every pair and instrument. A
// version is never edited; a new one starts at a later effective_from, and each
// minute is judged by the version in force when it was played.
type ChangeTarget struct {
	ent.Schema
}

func (ChangeTarget) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Immutable(),

		field.Int("version").
			Positive().
			Unique().
			Immutable(),

		field.Time("effective_from").
			Immutable(),

		field.Int("changes_per_minute").
			Positive().
			Immutable(),

		// source is default (team-set), benchmark or calibrated, as for fluent
		// times.
		field.Enum("source").
			Values("default", "benchmark", "calibrated").
			Immutable(),
	}
}
