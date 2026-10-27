package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// EmailSend records one engagement email the platform sent, so the same
// email never goes out twice. Its unique index is the guarantee: a send is
// claimed by inserting its row first, and only the insert that succeeds may
// send.
type EmailSend struct {
	ent.Schema
}

func (EmailSend) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		// Recipient is the user id of a student, or "concierge" for the
		// concierge summary, so it can't be a foreign key to users.
		field.String("recipient").
			NotEmpty().
			Immutable(),

		field.Enum("kind").
			Values("progress_report", "inactivity_reminder", "concierge_summary").
			Immutable(),

		// PeriodKey names what the email belongs to: an ISO week such as
		// 2026-W44, or an inactive spell's start and day threshold.
		field.String("period_key").
			NotEmpty().
			Immutable(),

		field.Time("sent_at").
			Immutable(),
	}
}

func (EmailSend) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("recipient", "kind", "period_key").Unique(),
	}
}
