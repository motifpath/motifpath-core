package schema

import (
	"encoding/json"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// ContentOwnership is one revision of an authored item's rights holder. Rows
// are only ever appended: the item's current ownership is its highest
// revision, and earlier revisions stay as its history.
type ContentOwnership struct {
	ent.Schema
}

func (ContentOwnership) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.Enum("content_kind").Values("course", "path", "content_node", "exercise").Immutable(),
		field.UUID("content_id", uuid.UUID{}).Immutable(),
		field.Int("revision").Positive().Immutable(),
		field.Enum("owner_kind").Values("platform", "teacher", "unresolved").Immutable(),
		field.UUID("owner_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("creator_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Enum("source").Values("native", "imported", "reviewed_legacy").Immutable(),
		field.UUID("evidence_entry_id", uuid.UUID{}).Unique().Immutable(),
		field.JSON("copied_from", json.RawMessage{}).Optional().Immutable(),
		field.Time("recorded_at").Default(time.Now).Immutable(),
	}
}

func (ContentOwnership) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("content_kind", "content_id", "revision").Unique(),
	}
}
