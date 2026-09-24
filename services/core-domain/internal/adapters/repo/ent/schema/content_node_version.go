package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// ContentNodeVersion is an immutable, permanent snapshot of a content
// node's title, content type, media, and rich content at the moment it was
// published. published_by/published_at follow the same actor+timestamp
// audit pattern already used by LearningPath (teacher_id/created_at) and
// PathAssignment's predecessor (assigned_by/assigned_at) elsewhere in this
// schema.
type ContentNodeVersion struct {
	ent.Schema
}

func (ContentNodeVersion) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.UUID("content_node_id", uuid.UUID{}).
			Immutable(),

		field.Int("version_number").
			Immutable(),

		field.String("title").
			Immutable(),

		field.Enum("content_type").
			Values("video", "article").
			Immutable(),

		field.String("media_url").
			Optional().
			Nillable().
			Immutable(),

		field.Text("rich_content").
			Optional().
			Nillable().
			Immutable(),

		// classification_snapshot and languages_snapshot hold JSON text. Both
		// are nil on versions published before they were persisted.
		field.Text("classification_snapshot").
			Optional().
			Nillable().
			Immutable(),

		field.Text("languages_snapshot").
			Optional().
			Nillable().
			Immutable(),

		field.UUID("published_by", uuid.UUID{}).
			Immutable(),

		field.Time("published_at").
			Immutable().
			Default(time.Now),
	}
}

func (ContentNodeVersion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("content_node_id", "version_number").Unique(),
	}
}
