package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// PracticeContentVersion is the single counter of content changes that the
// practice subject catalog is built from: each change raises it, and a
// catalog built from an older version is out of date. Its one row has id 1
// and is created by the first raise.
type PracticeContentVersion struct {
	ent.Schema
}

func (PracticeContentVersion) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("version"),
	}
}
