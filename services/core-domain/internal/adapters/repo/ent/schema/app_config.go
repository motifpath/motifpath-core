package schema

import (
	"encoding/json"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// AppConfig is one of the app's settings: an editorial or operational value the team changes at
// runtime, stored as the JSON of its key's own type. Only keys the service registers are ever
// written, and each value is checked before it is.
type AppConfig struct {
	ent.Schema
}

func (AppConfig) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			StorageKey("key").
			NotEmpty().
			Immutable(),

		field.JSON("value", json.RawMessage{}),

		field.Time("updated_at"),

		// updated_by is the user who last changed the setting.
		field.UUID("updated_by", uuid.UUID{}),
	}
}
