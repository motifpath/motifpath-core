package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// KnowledgeNode is a skill or concept in the knowledge graph. parent_id
// makes each kind a strict tree (null for a root; the parent is always of
// the same kind, checked in the application layer). key is the stable,
// immutable handle code and reference-data migrations address a node by.
// Content links to nodes through the skill/concept join tables; typed
// links between nodes are KnowledgeEdges.
type KnowledgeNode struct {
	ent.Schema
}

func (KnowledgeNode) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.Enum("kind").
			Values("skill", "concept").
			Immutable(),

		field.String("key").
			Unique().
			Immutable(),

		// names maps a language code to the node's name in that language;
		// every language MotifPath offers is required on write.
		field.JSON("names", map[string]string{}),

		// descriptions is null when the node has no description.
		field.JSON("descriptions", map[string]string{}).
			Optional(),

		field.UUID("parent_id", uuid.UUID{}).
			Optional().
			Nillable(),
	}
}

func (KnowledgeNode) Edges() []ent.Edge {
	return []ent.Edge{
		// A node with children can't be deleted, so the parent link
		// restricts rather than cascades.
		edge.To("parent", KnowledgeNode.Type).
			Unique().
			Field("parent_id").
			From("children"),

		// Empty means the node is for every instrument.
		edge.To("instruments", Instrument.Type).
			Through("knowledge_node_instruments", KnowledgeNodeInstrument.Type),

		edge.From("outgoing_edges", KnowledgeEdge.Type).
			Ref("from"),

		edge.From("incoming_edges", KnowledgeEdge.Type).
			Ref("to"),

		edge.From("skill_content_nodes", ContentNode.Type).
			Ref("skills").
			Through("content_node_skills", ContentNodeSkill.Type),

		edge.From("concept_content_nodes", ContentNode.Type).
			Ref("concepts").
			Through("content_node_concepts", ContentNodeConcept.Type),

		edge.From("skill_exercises", Exercise.Type).
			Ref("skills").
			Through("exercise_skills", ExerciseSkill.Type),

		edge.From("concept_exercises", Exercise.Type).
			Ref("concepts").
			Through("exercise_concepts", ExerciseConcept.Type),

		edge.From("skill_diagrams", Diagram.Type).
			Ref("skills").
			Through("diagram_skills", DiagramSkill.Type),

		edge.From("concept_diagrams", Diagram.Type).
			Ref("concepts").
			Through("diagram_concepts", DiagramConcept.Type),
	}
}
