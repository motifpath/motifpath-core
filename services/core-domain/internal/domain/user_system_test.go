package domain

import (
	"testing"
	"time"
)

func TestNewUserRejectsReservedSystemIdentity(t *testing.T) {
	_, err := NewUser("user-id", "system:catalog", RoleStudent, "Catalog", Language{Code: "en"}, time.Now())
	if err == nil {
		t.Fatal("expected a reserved system identity to be rejected")
	}
}

func TestNewUserAllowsClerkIdentity(t *testing.T) {
	_, err := NewUser("user-id", "user_real_clerk_identity", RoleStudent, "Gilson", Language{Code: "pt_BR"}, time.Now())
	if err != nil {
		t.Fatalf("expected a Clerk identity to be accepted: %v", err)
	}
}
