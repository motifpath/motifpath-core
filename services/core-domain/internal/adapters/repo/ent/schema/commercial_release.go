package schema

import (
	"encoding/json"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// CommercialRelease is an immutable manifest of what an item's owner offered
// for sale at one moment. Rows are only ever appended: the item's next
// manifest is a new row with the next revision.
type CommercialRelease struct {
	ent.Schema
}

func (CommercialRelease) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Immutable(),
		field.Enum("content_kind").Values("course", "path", "content_node", "exercise").Immutable(),
		field.UUID("content_id", uuid.UUID{}).Immutable(),
		field.Int("revision").Positive().Immutable(),
		field.String("manifest_sha256").Immutable(),
		field.String("content_sha256").Immutable(),
		field.String("core_version_ref").Optional().Nillable().Immutable(),
		// ownership is the rights holder's ownership revision as released.
		field.JSON("ownership", json.RawMessage{}).Immutable(),
		// dependencies lists every transitive dependency at its release.
		field.JSON("dependencies", json.RawMessage{}).Immutable(),
		field.UUID("evidence_entry_id", uuid.UUID{}).Immutable(),
		field.Time("created_at").Immutable(),
	}
}

func (CommercialRelease) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("content_kind", "content_id", "revision").Unique(),
	}
}
