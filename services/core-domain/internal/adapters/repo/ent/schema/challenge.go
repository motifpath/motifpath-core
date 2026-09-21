package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Challenge is the assessment unit for a content node: it groups exercises
// and carries the subject (a reference to exactly one Skill or Concept tree
// node — subject_skill_id/subject_concept_id are mutually exclusive, one of
// them always set) and pass threshold used by the rules-based recommendation
// engine, plus an optional, purely informational time threshold. Remediation
// targets are not modeled here — see Exercise's remediation_targets, which
// attaches remediation to the specific exercise a student struggled with
// rather than the whole challenge.
//
// subject_skill_id/subject_concept_id are stored as plain, unenforced UUID
// columns rather than ent edges — matching how content_node_id and
// teacher_id already reference other entities elsewhere in this schema
// package — since the membership rule that actually matters (the id must
// appear in the parent ContentNode's linked skill_ids/concept_ids) is a
// cross-entity invariant the domain/application layer enforces, not
// something a foreign key alone could express.
type Challenge struct {
	ent.Schema
}

func (Challenge) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.UUID("content_node_id", uuid.UUID{}).
			Immutable(),

		field.UUID("subject_skill_id", uuid.UUID{}).
			Optional().
			Nillable(),

		field.UUID("subject_concept_id", uuid.UUID{}).
			Optional().
			Nillable(),

		field.Int("pass_threshold"),

		// time_threshold_ms is the teacher's explicit override only — never
		// enforced, never affects scoring. When absent, the value shown to
		// a caller is computed in the application layer from linked
		// exercises' estimated durations, not stored here.
		field.Int("time_threshold_ms").
			Optional().
			Nillable(),

		field.Bool("shuffle_exercises").
			Default(false),

		field.Bool("shuffle_options").
			Default(false),

		field.Time("created_at").
			Immutable().
			Default(time.Now),
	}
}

func (Challenge) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("content_node_id"),
	}
}

func (Challenge) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("exercises", Exercise.Type).
			Ref("challenges").
			Through("challenge_exercises", ChallengeExercise.Type),
	}
}
