package schema

import (
	"encoding/json"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// CommercialOperation is the durable record of one logical commercial
// operation. Its unique scope is the guarantee that a caller's key produces
// one effect: concurrent submissions race on the insert, and the loser reads
// the winner's record. Records never expire.
type CommercialOperation struct {
	ent.Schema
}

func (CommercialOperation) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.String("environment").NotEmpty().Immutable(),
		field.Enum("actor_kind").Values("user", "service", "provider").Immutable(),
		field.String("actor_id").NotEmpty().Immutable(),
		field.String("action").NotEmpty().Immutable(),
		field.String("resource_kind").NotEmpty().Immutable(),
		field.String("resource_id").NotEmpty().Immutable(),
		field.UUID("caller_key", uuid.UUID{}).Immutable(),
		// fingerprint is the SHA-256 of the canonical intent first sent.
		field.String("fingerprint").NotEmpty().Immutable(),
		// result is the response the operation produced, returned unchanged
		// to every same-intent replay.
		field.JSON("result", json.RawMessage{}).Immutable(),
		field.Int64("authorized_revision").Min(0).Immutable(),
		field.Time("created_at").Immutable(),
	}
}

func (CommercialOperation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("environment", "actor_kind", "actor_id", "action", "resource_kind", "resource_id", "caller_key").
			Unique(),
	}
}
