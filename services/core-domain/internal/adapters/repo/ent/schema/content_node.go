package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// ContentNode is the base unit of a class — a video or article published by
// a teacher. Classification's skill/concept dimensions are many-to-many
// relations against the shared Skill/Concept trees; difficulty_level and
// review_state remain scalar fields on the node itself.
type ContentNode struct {
	ent.Schema
}

func (ContentNode) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.UUID("teacher_id", uuid.UUID{}).
			Immutable(),

		field.String("title"),

		field.Enum("content_type").
			Values("video", "article").
			Immutable(),

		// media_url is the video students watch; rich_content stores the
		// article's marshaled PromptDocument JSON as text, the same encoding
		// ExpandedContent uses. Exactly one is populated per content_type —
		// that XOR is enforced in the domain layer, not here. Both are
		// nullable so video nodes created before media_url existed still load.
		field.String("media_url").Optional().Nillable(),

		field.Text("rich_content").Optional().Nillable(),

		field.Enum("difficulty_level").
			Values("beginner", "early_intermediate", "intermediate", "advanced", "expert"),

		// ReviewState always starts pending on creation regardless of any
		// value supplied by the caller — enforced in the domain constructor,
		// not here.
		field.Enum("review_state").
			Values("pending", "confirmed", "overridden").
			Default("pending"),

		// thumbnail_url is the image shown for it in lists and cards; NULL =
		// none.
		field.String("thumbnail_url").
			Optional().
			Nillable(),

		field.Time("created_at").
			Immutable().
			Default(time.Now),

		// updated_at is when the node was created or last edited; authoring
		// lists sort by it.
		field.Time("updated_at").
			Default(time.Now),

		// latest_published_version is the version_number last published;
		// NULL for a node never published.
		field.Int("latest_published_version").
			Optional().
			Nillable(),

		// has_unpublished_changes is true when the node differs from its
		// latest published version, or nothing has been published yet.
		field.Bool("has_unpublished_changes").
			Default(true),
	}
}

func (ContentNode) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("path_exercises", Exercise.Type).
			Ref("content_nodes").
			Through("content_node_exercises", ContentNodeExercise.Type),

		edge.To("languages", Language.Type).
			Through("content_node_languages", ContentNodeLanguage.Type),

		edge.To("skills", KnowledgeNode.Type).
			Through("content_node_skills", ContentNodeSkill.Type),

		edge.To("concepts", KnowledgeNode.Type).
			Through("content_node_concepts", ContentNodeConcept.Type),

		edge.To("instruments", Instrument.Type).
			Through("content_node_instruments", ContentNodeInstrument.Type),
	}
}
