package schema

import (
	"encoding/json"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// PracticeSubjectCatalog is one scope's precomputed practice subject catalog:
// the items classified for an instrument, or for none when scope is empty,
// with what a subjects list tallies of each that is the same for every
// student. It is rebuilt whole after content changes, never edited in place.
type PracticeSubjectCatalog struct {
	ent.Schema
}

func (PracticeSubjectCatalog) Fields() []ent.Field {
	return []ent.Field{
		// scope is an instrument id, or empty for a student with no
		// instruments.
		field.String("scope").
			Unique(),

		// format_version is the version of how items were tallied when the
		// catalog was built; one of another version is not read.
		field.Int("format_version"),

		field.Time("built_at"),

		// items is the JSON list of the scope's catalog items.
		field.JSON("items", json.RawMessage{}),
	}
}
