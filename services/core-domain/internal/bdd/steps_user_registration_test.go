//go:build integration

package bdd

import (
	"context"
	"fmt"

	"github.com/cucumber/godog"

	appHTTP "github.com/motifpath/core-domain/internal/adapters/http"
	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerUserRegistrationSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^a Clerk identity "([^"]+)" has not yet been registered$`, func(string) error { return nil })
	sc.Step(`^"([^"]+)" has already registered as a student$`, w.hasAlreadyRegisteredAsStudent)

	sc.Step(`^"([^"]+)" registers with role "([^"]+)"$`, w.registers)
	sc.Step(`^"([^"]+)" attempts to register again with role "([^"]+)"$`, w.registers)
	sc.Step(`^"([^"]+)" submits a registration request with role "([^"]+)"$`, w.registers)
	sc.Step(`^"([^"]+)" submits a registration request with the role field omitted$`, w.registersWithRoleOmitted)
	sc.Step(`^"([^"]+)" requests their own profile$`, w.requestsOwnProfile)
	sc.Step(`^an unauthenticated request attempts to register with role "([^"]+)"$`, w.unauthenticatedRegisters)
	sc.Step(`^an unauthenticated request attempts to retrieve a user profile$`, w.unauthenticatedGetProfile)

	sc.Step(`^a user record is created for "([^"]+)"$`, w.userRecordCreatedFor)
	sc.Step(`^the response includes a stable user_id and the role "([^"]+)"$`, w.responseIncludesUserIDAndRole)
	sc.Step(`^the response includes a registration timestamp$`, w.responseIncludesRegisteredAt)
	sc.Step(`^the response returns "([^"]+)"'s user_id, role "([^"]+)", and registration timestamp$`, w.profileResponseMatches)
	sc.Step(`^the request is refused with a conflict error$`, w.requestRefusedConflict)
}

func (w *world) hasAlreadyRegisteredAsStudent(name string) error {
	w.ensureRegistered(name, domain.RoleStudent)
	return nil
}

func (w *world) registers(name, role string) error {
	ctx := appHTTP.WithClerkUserID(context.Background(), clerkSub(name))
	resp, err := w.handler.RegisterUser(ctx, generated.RegisterUserRequestObject{
		Body: &generated.RegisterUserRequest{Role: generated.RegisterUserRequestRole(role)},
	})
	w.lastResp, w.lastErr = resp, err
	if created, ok := resp.(generated.RegisterUser201JSONResponse); ok {
		w.userMotifID[name] = created.UserId
	}
	return err
}

func (w *world) registersWithRoleOmitted(name string) error {
	return w.registers(name, "")
}

// requestsOwnProfile only authenticates — it deliberately does not
// register name. A valid Clerk token doesn't imply a MotifPath User record
// exists; whether GetMyProfile then succeeds or 404s depends entirely on
// whether an earlier Given step (e.g. "X has already registered as a
// student") registered this identity first. Both the "registered user
// retrieves their profile" and "profile before registering" scenarios use
// this exact same step wording — only their setup differs.
func (w *world) requestsOwnProfile(name string) error {
	w.hasToken = true
	w.clerkSub = clerkSub(name)
	resp, err := w.handler.GetMyProfile(w.ctx(), generated.GetMyProfileRequestObject{})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthenticatedRegisters(role string) error {
	w.noAuthToken() //nolint:errcheck // never errors
	resp, err := w.handler.RegisterUser(w.ctx(), generated.RegisterUserRequestObject{
		Body: &generated.RegisterUserRequest{Role: generated.RegisterUserRequestRole(role)},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthenticatedGetProfile() error {
	w.noAuthToken() //nolint:errcheck // never errors
	resp, err := w.handler.GetMyProfile(w.ctx(), generated.GetMyProfileRequestObject{})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) userRecordCreatedFor(name string) error {
	resp, ok := w.lastResp.(generated.RegisterUser201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.UserId != w.userMotifID[name] {
		return fmt.Errorf("expected user_id %s for %q, got %s", w.userMotifID[name], name, resp.UserId)
	}
	return nil
}

func (w *world) responseIncludesUserIDAndRole(role string) error {
	resp, ok := w.lastResp.(generated.RegisterUser201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.UserId.String() == "" {
		return fmt.Errorf("expected a non-empty user_id")
	}
	if string(resp.Role) != role {
		return fmt.Errorf("expected role %q, got %q", role, resp.Role)
	}
	return nil
}

func (w *world) responseIncludesRegisteredAt() error {
	resp, ok := w.lastResp.(generated.RegisterUser201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.RegisteredAt.IsZero() {
		return fmt.Errorf("expected a non-zero registered_at")
	}
	return nil
}

func (w *world) profileResponseMatches(name, role string) error {
	resp, ok := w.lastResp.(generated.GetMyProfile200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.UserId != w.userMotifID[name] {
		return fmt.Errorf("expected user_id %s for %q, got %s", w.userMotifID[name], name, resp.UserId)
	}
	if string(resp.Role) != role {
		return fmt.Errorf("expected role %q, got %q", role, resp.Role)
	}
	if resp.RegisteredAt.IsZero() {
		return fmt.Errorf("expected a non-zero registered_at")
	}
	return nil
}

func (w *world) requestRefusedConflict() error {
	if _, ok := w.lastResp.(generated.RegisterUser409JSONResponse); !ok {
		return fmt.Errorf("expected a 409 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}
