package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// User is a registered MotifPath identity — student, teacher, or admin.
type User struct {
	ent.Schema
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		// ClerkUserID maps this record back to the Clerk identity (JWT sub
		// claim) that registered it. Registration must be idempotent per
		// Clerk identity — POST /users returns 409 on a second call from the
		// same identity — so this is the field that detects the duplicate
		// and the field GetMyProfile looks up by.
		field.String("clerk_user_id").
			Unique().
			Immutable(),

		field.Enum("role").
			Values("student", "teacher", "admin").
			Immutable(),

		// DisplayName is the user's full name as held by Clerk, refreshed
		// from the session token whenever it changes. Required: no user may
		// exist without a name. Its 200-character cap is enforced (in
		// characters, not bytes) by the domain before it gets here, so no
		// MaxLen validator — ent's counts bytes and would reject a long
		// non-ASCII name the domain accepted.
		field.String("display_name").
			NotEmpty(),

		// LocaleID is the user's resolved locale preference. Required —
		// every user has one, defaulted in the application layer at
		// registration time rather than via a DB default, matching how role
		// is handled. Mutable afterwards through a profile update.
		field.UUID("locale_id", uuid.UUID{}),

		// Email is the user's address as held by the identity provider,
		// refreshed from the session token whenever it changes. Nullable:
		// users registered before addresses were stored have none until
		// their next signed-in request.
		field.String("email").
			Optional().
			Nillable(),

		// TimeZone is the IANA time zone the user's client last reported,
		// or null when none ever was.
		field.String("time_zone").
			Optional().
			Nillable(),

		// One switch per kind of email a student can get. Both start on.
		field.Bool("email_progress_report").
			Default(true),
		field.Bool("email_reminders").
			Default(true),

		field.Time("registered_at").
			Immutable().
			Default(time.Now),
	}
}

func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("locale", Language.Type).
			Unique().
			Required().
			Field("locale_id"),
	}
}
