package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// CommercialOperationAttempt is one submission of a recorded operation: the
// first one, each same-intent replay and each refused reuse of its key.
type CommercialOperationAttempt struct {
	ent.Schema
}

func (CommercialOperationAttempt) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Immutable(),
		field.UUID("operation_id", uuid.UUID{}).Immutable(),
		field.String("fingerprint").NotEmpty().Immutable(),
		field.Enum("outcome").Values("applied", "failed", "denied", "duplicate", "uncertain").Immutable(),
		field.Time("at").Immutable(),
	}
}

func (CommercialOperationAttempt) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("operation", CommercialOperation.Type).
			Field("operation_id").
			Unique().
			Required().
			Immutable(),
	}
}
