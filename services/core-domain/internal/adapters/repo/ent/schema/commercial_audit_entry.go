package schema

import (
	"encoding/json"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// CommercialAuditEntry is one immutable piece of commercial evidence. The
// database refuses any update or delete of these rows, so evidence can only be
// appended. The auto-increment id is the stable append order a traversal
// pages through; recorded_at never decides order.
type CommercialAuditEntry struct {
	ent.Schema
}

func (CommercialAuditEntry) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			StorageKey("order_key").
			Immutable(),
		field.UUID("audit_entry_id", uuid.UUID{}).
			Unique().
			Immutable(),
		field.String("environment").NotEmpty().Immutable(),
		field.UUID("operation_id", uuid.UUID{}).Immutable(),
		field.UUID("attempt_id", uuid.UUID{}).Immutable(),
		field.Enum("actor_kind").Values("user", "service", "provider").Immutable(),
		field.String("actor_id").NotEmpty().Immutable(),
		field.String("action").NotEmpty().Immutable(),
		field.String("resource_kind").NotEmpty().MaxLen(100).Immutable(),
		field.String("resource_id").NotEmpty().MaxLen(255).Immutable(),
		field.Time("occurred_at").Immutable(),
		field.Time("recorded_at").Immutable(),
		field.Text("reason").NotEmpty().Immutable(),
		field.Enum("outcome").Values("applied", "failed", "denied", "duplicate", "uncertain").Immutable(),
		// A null state means no authoritative state was available, which is
		// not the same as an empty one.
		field.JSON("before_state", json.RawMessage{}).Optional().Immutable(),
		field.JSON("after_state", json.RawMessage{}).Optional().Immutable(),
		field.String("policy_revision").NotEmpty().Immutable(),
		field.String("configuration_revision").NotEmpty().Immutable(),
		field.UUID("correlation_id", uuid.UUID{}).Immutable(),
		field.Strings("caused_by_entry_ids").Immutable(),
		field.Strings("provider_references").Immutable(),
	}
}

func (CommercialAuditEntry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("environment", "operation_id"),
		index.Fields("environment", "resource_kind", "resource_id", "id"),
	}
}
