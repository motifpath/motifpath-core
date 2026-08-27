package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Exercise is a pre-defined practice item within a challenge. For MVP, all
// exercises are fretboard region interactions with a binary correct/
// incorrect outcome.
type Exercise struct {
	ent.Schema
}

func (Exercise) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.UUID("challenge_id", uuid.UUID{}).
			Immutable(),

		field.Enum("exercise_type").
			Values("fretboard_region").
			Immutable(),

		field.Text("prompt"),

		field.Time("created_at").
			Immutable().
			Default(time.Now),
	}
}

func (Exercise) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("challenge_id"),
	}
}
