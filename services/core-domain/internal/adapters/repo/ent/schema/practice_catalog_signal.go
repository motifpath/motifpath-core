package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// PracticeCatalogSignal is a signal, queued after a content change, that the
// practice subject catalog needs rebuilding. A rebuild takes every signal
// pending when it starts and removes them once it is stored.
type PracticeCatalogSignal struct {
	ent.Schema
}

func (PracticeCatalogSignal) Fields() []ent.Field {
	return []ent.Field{
		field.String("reason").
			NotEmpty(),

		field.Time("created_at"),
	}
}
