package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// CommercialActivation is the environment's single activation row. Its
// revision counts applied activation and group changes, and the row is the
// lock that fences new intents behind a committed change: intents read it
// FOR SHARE, changes FOR UPDATE. The migration inserts the row at revision 0.
type CommercialActivation struct {
	ent.Schema
}

func (CommercialActivation) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").Immutable(),
		field.Int64("revision").Min(0),
	}
}
