package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// ChordDefinition is a chord of the chord catalog: root, quality and an
// optional slash bass, apart from any fingering. Chords are reference data,
// installed only by migrations with fixed IDs.
type ChordDefinition struct {
	ent.Schema
}

func (ChordDefinition) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Immutable(),

		field.String("canonical_symbol").
			MaxLen(32).
			NotEmpty().
			Unique().
			Immutable(),

		field.String("root").
			MaxLen(2).
			NotEmpty().
			Immutable(),

		field.Int("root_pitch_class").
			Range(0, 11).
			Immutable(),

		field.String("quality").
			MaxLen(32).
			NotEmpty().
			Immutable(),

		// formula and omittable are interval codes, formula starting with R.
		field.JSON("formula", []string{}).
			Immutable(),

		field.JSON("omittable", []string{}).
			Immutable(),

		field.String("bass").
			MaxLen(2).
			Optional().
			Nillable().
			Immutable(),

		field.Int("bass_pitch_class").
			Range(0, 11).
			Optional().
			Nillable().
			Immutable(),

		field.JSON("aliases", []string{}),
	}
}

func (ChordDefinition) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("voicings", ChordVoicing.Type).
			Ref("chord_definition"),
	}
}

// A chord is found by what it sounds like, so pitch classes, not spellings,
// identify it: at most one chord per root, quality and bass. Postgres treats
// a null bass as distinct from every other, so for chords without a slash
// bass the catalog build is what keeps that true.
func (ChordDefinition) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("root_pitch_class", "quality", "bass_pitch_class").
			Unique(),
	}
}
