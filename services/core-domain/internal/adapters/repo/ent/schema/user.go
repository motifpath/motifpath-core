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

		// LocaleID is the user's resolved locale preference. Required —
		// every user has one, defaulted in the application layer at
		// registration time rather than via a DB default, matching how role
		// is handled. Mutable afterwards via UpdateLocale.
		field.UUID("locale_id", uuid.UUID{}),

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
