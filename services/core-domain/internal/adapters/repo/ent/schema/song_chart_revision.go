package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// SongChartRevision is a published revision of a song chart: what learners
// read. A revision is never changed once stored; a correction is the next
// revision.
type SongChartRevision struct {
	ent.Schema
}

func (SongChartRevision) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.UUID("song_chart_id", uuid.UUID{}).
			Immutable(),

		field.Int("revision_number").
			Positive().
			Immutable(),

		field.String("title").
			Immutable(),

		field.String("artist").
			Immutable(),

		field.String("language").
			Immutable(),

		field.String("concert_key").
			Optional().
			Nillable().
			Immutable(),

		field.Int("capo_fret").
			Immutable(),

		field.Int("tempo_bpm").
			Optional().
			Nillable().
			Immutable(),

		field.JSON("time_signature", &SongChartTimeSignature{}).
			Optional().
			Immutable(),

		field.String("tuning_fingerprint").
			Immutable(),

		// body is the revision's ProseMirror JSON.
		field.Text("body").
			Immutable(),

		field.UUID("rights_confirmed_by", uuid.UUID{}).
			Immutable(),

		field.Time("rights_confirmed_at").
			Immutable(),

		field.UUID("published_by", uuid.UUID{}).
			Immutable(),

		field.Time("published_at").
			Immutable(),
	}
}

func (SongChartRevision) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("song_chart", SongChart.Type).
			Field("song_chart_id").
			Unique().
			Required().
			Immutable(),
	}
}

func (SongChartRevision) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("song_chart_id", "revision_number").
			Unique(),
	}
}
