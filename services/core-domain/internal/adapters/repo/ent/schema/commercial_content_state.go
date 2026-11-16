package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// CommercialContentState is an item's commercial revision and latest release.
// An item without a row has never been released and is at revision 1.
type CommercialContentState struct {
	ent.Schema
}

func (CommercialContentState) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.Enum("content_kind").Values("course", "path", "content_node", "exercise").Immutable(),
		field.UUID("content_id", uuid.UUID{}).Immutable(),
		field.Int("revision").Positive(),
		field.UUID("latest_release_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (CommercialContentState) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("content_kind", "content_id").Unique(),
	}
}
