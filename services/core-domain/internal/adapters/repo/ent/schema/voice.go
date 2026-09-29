package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Voice is a sampled sound diagrams can be played with. Rows are provided
// with the platform by migrations, never written through the service.
type Voice struct {
	ent.Schema
}

func (Voice) Fields() []ent.Field {
	return []ent.Field{
		// id is a stable slug, e.g. "acoustic-guitar", also used in the
		// samples' storage path.
		field.String("id").
			Immutable(),

		// names maps a language code to the voice's name in that language.
		field.JSON("names", map[string]string{}),

		field.Enum("family").
			Values("fretted", "keyboard").
			Immutable(),

		// pitches are the MIDI pitches that have a recorded sample.
		field.JSON("pitches", []int{}),

		// attribution is the credit the samples' license requires.
		field.String("attribution"),
	}
}

func (Voice) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("instruments", Instrument.Type).
			Ref("default_voice"),
	}
}
