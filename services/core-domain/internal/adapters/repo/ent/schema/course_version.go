package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// CourseVersion is an immutable, permanent snapshot of a course's title,
// summary, and level at the moment it was published. Its checkpoint
// identities live in the separate CourseVersionCheckpoint table (see
// course_version_checkpoint.go), joined by course_version_id — the same
// draft-table/version-table split Course/CourseCheckpoint already use.
// available_for_new_enrollments defaults true at publish time; nothing in
// this slice of the feature mutates it — a later course-retirement or
// superseding-version capability is its actual consumer.
type CourseVersion struct {
	ent.Schema
}

func (CourseVersion) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.UUID("course_id", uuid.UUID{}).
			Immutable(),

		field.Int("version_number").
			Immutable(),

		field.String("title_snapshot").
			Immutable(),

		field.String("summary_snapshot").
			Immutable(),

		field.Enum("level_snapshot").
			Values("beginner", "early_intermediate", "intermediate", "advanced", "expert").
			Immutable(),

		// language_snapshot is the course's language when this version was
		// published. The default only backfills versions published before
		// courses had a language, which were all written in English.
		field.String("language_snapshot").
			Default("en").
			Immutable(),

		// instrument_ids_snapshot is the instruments this version was for
		// when published; NULL (a version published before instruments were
		// recorded) or empty means every instrument.
		field.JSON("instrument_ids_snapshot", []string{}).
			Optional().
			Immutable(),

		// thumbnail_url_snapshot is the thumbnail when this version was
		// published; NULL = none.
		field.String("thumbnail_url_snapshot").
			Optional().
			Nillable().
			Immutable(),

		field.Bool("available_for_new_enrollments").
			Default(true),

		field.Time("published_at").
			Immutable().
			Default(time.Now),
	}
}

func (CourseVersion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("course_id", "version_number").Unique(),
	}
}
