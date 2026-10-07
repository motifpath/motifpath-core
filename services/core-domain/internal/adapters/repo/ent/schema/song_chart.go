package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// SongChart is a song chart with its draft, the working copy admins edit.
// What learners read is its latest SongChartRevision; the published_*
// columns repeat that revision's summary so a list of charts needs no join.
type SongChart struct {
	ent.Schema
}

// SongChartTimeSignature is a chart's meter, as written.
type SongChartTimeSignature struct {
	Beats     int `json:"beats"`
	BeatValue int `json:"beat_value"`
}

// SongChartWarning is a chord anchor of the draft that didn't fully resolve
// when the draft was last saved.
type SongChartWarning struct {
	AnchorID      string `json:"anchor_id"`
	SectionIndex  int    `json:"section_index"`
	LineIndex     int    `json:"line_index"`
	WrittenSymbol string `json:"written_symbol"`
	Kind          string `json:"kind"`
}

func (SongChart) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Immutable(),

		field.Enum("status").
			Values("draft", "published", "withdrawn"),

		field.UUID("created_by", uuid.UUID{}).
			Immutable(),

		field.Time("created_at").
			Immutable(),

		// The draft.
		field.String("title").
			MaxLen(200).
			NotEmpty(),

		field.String("artist").
			MaxLen(200).
			NotEmpty(),

		field.String("language").
			NotEmpty(),

		field.String("concert_key").
			Optional().
			Nillable(),

		field.Int("capo_fret").
			Range(0, 12),

		field.Int("tempo_bpm").
			Optional().
			Nillable(),

		field.JSON("time_signature", &SongChartTimeSignature{}).
			Optional(),

		// rights_confirmed_by and rights_confirmed_at are null together: no
		// admin has confirmed the song's rights were checked.
		field.UUID("rights_confirmed_by", uuid.UUID{}).
			Optional().
			Nillable(),

		field.Time("rights_confirmed_at").
			Optional().
			Nillable(),

		// body is the draft's ProseMirror JSON.
		field.Text("body"),

		field.JSON("warnings", []SongChartWarning{}),

		field.UUID("updated_by", uuid.UUID{}),

		field.Time("updated_at"),

		// The latest published revision's summary; all null until the first
		// publication.
		field.Int("published_revision_number").
			Optional().
			Nillable(),

		field.String("published_title").
			Optional().
			Nillable(),

		field.String("published_language").
			Optional().
			Nillable(),

		field.UUID("published_by", uuid.UUID{}).
			Optional().
			Nillable(),

		field.Time("published_at").
			Optional().
			Nillable(),

		// The withdrawal; all null unless status is withdrawn.
		field.UUID("withdrawn_by", uuid.UUID{}).
			Optional().
			Nillable(),

		field.Time("withdrawn_at").
			Optional().
			Nillable(),

		field.Text("withdrawal_reason").
			Optional().
			Nillable(),
	}
}

func (SongChart) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("revisions", SongChartRevision.Type).
			Ref("song_chart"),
	}
}

func (SongChart) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("updated_at"),
	}
}
