package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Diagram is a prebuilt, reusable set of positions on one instrument. It
// stores structured positions only, never a rendered image. Its skill and
// concept classification is many-to-many against the shared Skill/Concept
// trees, independent of ContentNode's and Exercise's own links to them.
type Diagram struct {
	ent.Schema
}

func (Diagram) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		// instrument_id can't change after creation: every position's
		// coordinate shape depends on the instrument's family.
		field.UUID("instrument_id", uuid.UUID{}).
			Immutable(),

		// names maps a language code to the diagram's name in that language:
		// every offered language for a basic diagram, at least one for a
		// custom one.
		field.JSON("names", map[string]string{}),

		// kind and created_by are fixed at creation: basic diagrams are
		// admin-curated templates, custom ones belong to created_by.
		field.Enum("kind").
			Values("basic", "custom").
			Immutable(),

		field.UUID("created_by", uuid.UUID{}).
			Immutable(),

		field.String("root_note").
			Optional().
			Nillable(),

		field.Enum("label_display").
			Values("interval", "note", "hidden").
			Default("interval"),

		// color is the general marker color as #RRGGBB; NULL = unrecorded.
		field.String("color").
			Optional().
			Nillable(),

		// mode with root_note names the key; NULL = no key.
		field.Enum("mode").
			Values("major", "minor", "dorian", "phrygian", "lydian", "mixolydian", "locrian").
			Optional().
			Nillable(),

		// playbacks are the ways the diagram sounds, in order, each with its
		// steps naming positions of this diagram by id; empty = the diagram
		// doesn't play.
		field.JSON("playbacks", []Playback{}).
			Default([]Playback{}),

		// default_playback_id names one of playbacks; NULL exactly when there
		// are none.
		field.String("default_playback_id").
			Optional().
			Nillable(),

		field.Time("created_at").
			Immutable().
			Default(time.Now),
	}
}

// Playback is one stored playback of a Diagram.
type Playback struct {
	ID            string            `json:"playback_id"`
	Names         map[string]string `json:"names"`
	TempoBPM      int               `json:"tempo_bpm"`
	TimeSignature TimeSignature     `json:"time_signature"`
	Steps         []SequenceStep    `json:"steps"`
}

// TimeSignature is a stored playback's meter.
type TimeSignature struct {
	Beats     int `json:"beats"`
	BeatValue int `json:"beat_value"`
}

// SequenceStep is one stored step of a playback.
type SequenceStep struct {
	PositionIDs []string  `json:"position_ids"`
	Value       NoteValue `json:"value"`
	Strum       string    `json:"strum"`
}

// NoteValue is a stored step length, as a fraction of a whole note.
type NoteValue struct {
	Num int `json:"num"`
	Den int `json:"den"`
}

func (Diagram) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("instrument", Instrument.Type).
			Unique().
			Required().
			Immutable().
			Field("instrument_id"),

		edge.To("compatible_instruments", Instrument.Type).
			Through("diagram_instruments", DiagramInstrument.Type),

		edge.To("positions", Position.Type),

		edge.To("regions", DiagramRegion.Type),

		edge.To("skills", KnowledgeNode.Type).
			Through("diagram_skills", DiagramSkill.Type),

		edge.To("concepts", KnowledgeNode.Type).
			Through("diagram_concepts", DiagramConcept.Type),
	}
}
