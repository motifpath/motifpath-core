package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Progression is a common chord progression of the progression catalog, by
// number: one numeral per bar, in a key the drill picks. Progressions are
// reference data, installed only by migrations with fixed IDs.
type Progression struct {
	ent.Schema
}

func (Progression) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Immutable(),

		// key names the progression by its numerals, such as I-V-vi-IV.
		field.String("key").
			MaxLen(64).
			NotEmpty().
			Unique().
			Immutable(),

		// mode is the key's mode the numerals are in, such as major.
		field.String("mode").
			MaxLen(16).
			NotEmpty().
			Immutable(),

		// bars holds one numeral per bar, such as ["I", "V", "vi", "IV"].
		field.JSON("bars", []string{}).
			Immutable(),

		// node_ids are the knowledge nodes the progression practises.
		field.JSON("node_ids", []string{}).
			Immutable(),

		// confusable holds the keys of the progressions a learner mistakes
		// it for, each listing it back.
		field.JSON("confusable", []string{}).
			Immutable(),

		field.JSON("names", map[string]string{}).
			Immutable(),
	}
}
