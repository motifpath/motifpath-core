package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Progression is a chord progression by number: one numeral per bar, in a key
// a drill or a chart picks. The catalog's are reference data, installed by
// migrations with fixed IDs. Admins create the others, with random IDs, and
// only those in_drills reach the drills.
type Progression struct {
	ent.Schema
}

func (Progression) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Immutable(),

		// key names the progression by its numerals: I-V-vi-IV in the catalog,
		// major/I-bVII-IV-I when created. Sixteen of the longest numeral and
		// their separators take 149 characters.
		field.String("key").
			MaxLen(160).
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

		// node_ids are the knowledge nodes the progression practises. They are
		// set when it joins the drills.
		field.JSON("node_ids", []string{}),

		// confusable holds the keys of the progressions a learner mistakes
		// it for. They are set when it joins the drills.
		field.JSON("confusable", []string{}),

		// names follow the numerals and, on a created progression, its nicknames.
		field.JSON("names", map[string]string{}),

		// in_drills says the drills use it. Joining them is one-way.
		field.Bool("in_drills").
			Default(false),

		// nicknames are a created progression's optional nicknames per language.
		field.JSON("nicknames", map[string]string{}).
			Optional(),

		// created_by and created_at are empty on the catalog's progressions.
		field.UUID("created_by", uuid.UUID{}).
			Optional().
			Nillable().
			Immutable(),

		field.Time("created_at").
			Optional().
			Nillable().
			Immutable(),
	}
}

func (Progression) Indexes() []ent.Index {
	return []ent.Index{
		// One progression per mode and numerals.
		index.Fields("mode", "bars").Unique(),
	}
}
