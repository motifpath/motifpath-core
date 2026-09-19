package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// ExpandedContent is an expositive item (image, GIF, or rich content)
// attached to a content node and shown to the student at a specific point
// during content consumption. Video nodes use
// trigger_at_seconds/hide_at_seconds; article nodes use
// trigger_at_paragraph/duration_ms — the XOR between the two field groups is
// enforced in the domain constructor, not here. image/gif carry media_url;
// rich_text carries rich_content instead — that XOR is likewise enforced in
// the domain layer.
type ExpandedContent struct {
	ent.Schema
}

func (ExpandedContent) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.UUID("content_node_id", uuid.UUID{}).
			Immutable(),

		field.Enum("content_type").
			Values("image", "gif", "rich_text"),

		field.String("media_url").Optional().Nillable(),

		// rich_content stores marshaled PromptDocument JSON as text, the
		// same pattern Exercise.prompt uses — present only when content_type
		// is rich_text.
		field.Text("rich_content").Optional().Nillable(),

		field.Int("trigger_at_seconds").Optional().Nillable(),
		field.Int("hide_at_seconds").Optional().Nillable(),
		field.Int("trigger_at_paragraph").Optional().Nillable(),
		field.Int("duration_ms").Optional().Nillable(),
		field.String("caption").Optional().Nillable(),

		field.Time("created_at").
			Immutable().
			Default(time.Now),
	}
}

func (ExpandedContent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("content_node_id"),
	}
}
