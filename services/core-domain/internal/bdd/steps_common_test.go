//go:build integration

package bdd

import (
	"fmt"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
)

// registerCommonSteps wires the Then-step wording that is identical across
// every feature file (the same phrases appear in user-registration,
// content-management, and learning-paths alike). Centralizing the response
// type switches here — rather than duplicating a near-identical assertion
// per operation per feature file — is what keeps 15 endpoints' worth of
// step definitions from turning into 15x the boilerplate.
func registerCommonSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the Core Domain Service is operational and ready to accept requests$`, func() error { return nil })
	sc.Step(`^no authentication token is provided$`, w.noAuthToken)

	sc.Step(`^the request is refused with a forbidden error$`, w.requestRefusedForbidden)
	sc.Step(`^the request is refused with a not-found error$`, w.requestRefusedNotFound)
	sc.Step(`^the request is refused with an authentication error$`, w.requestRefusedAuthError)
	sc.Step(`^the request is rejected as invalid$`, w.requestRejectedInvalid)
	sc.Step(`^the rejection identifies "([^"]+)" as the source of the error$`, w.rejectionIdentifiesField)

	sc.Step(`^three distinct exercise identifiers are returned$`, w.threeDistinctIdentifiersReturned)
	sc.Step(`^three distinct expanded content identifiers are returned$`, w.threeDistinctIdentifiersReturned)
}

func (w *world) threeDistinctIdentifiersReturned() error {
	if len(w.multiCreateIDs) != 3 {
		return fmt.Errorf("expected 3 created identifiers, got %d", len(w.multiCreateIDs))
	}
	seen := map[string]bool{}
	for _, id := range w.multiCreateIDs {
		if seen[id.String()] {
			return fmt.Errorf("identifier %s was returned more than once", id)
		}
		seen[id.String()] = true
	}
	return nil
}

func (w *world) noAuthToken() error {
	w.hasToken = false
	w.clerkSub = ""
	return nil
}

func (w *world) requestRefusedForbidden() error {
	switch w.lastResp.(type) {
	case generated.CreateContentNode403JSONResponse,
		generated.CreateChallenge403JSONResponse,
		generated.CreateExercise403JSONResponse,
		generated.CreateExpandedContent403JSONResponse,
		generated.CreateLearningPath403JSONResponse,
		generated.GetLearningPath403JSONResponse,
		generated.AssignLearningPath403JSONResponse,
		generated.GetMyPath403JSONResponse:
		return nil
	default:
		return fmt.Errorf("expected a 403 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
}

func (w *world) requestRefusedNotFound() error {
	switch w.lastResp.(type) {
	case generated.GetContentNode404JSONResponse,
		generated.CreateChallenge404JSONResponse,
		generated.GetChallenge404JSONResponse,
		generated.CreateExercise404JSONResponse,
		generated.GetExercise404JSONResponse,
		generated.CreateExpandedContent404JSONResponse,
		generated.ListExpandedContent404JSONResponse,
		generated.GetExpandedContent404JSONResponse,
		generated.GetLearningPath404JSONResponse,
		generated.AssignLearningPath404JSONResponse,
		generated.GetMyPath404JSONResponse,
		generated.GetMyProfile404JSONResponse:
		return nil
	default:
		return fmt.Errorf("expected a 404 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
}

func (w *world) requestRefusedAuthError() error {
	switch w.lastResp.(type) {
	case generated.RegisterUser401JSONResponse,
		generated.GetMyProfile401JSONResponse,
		generated.CreateContentNode401JSONResponse,
		generated.GetContentNode401JSONResponse,
		generated.CreateChallenge401JSONResponse,
		generated.GetChallenge401JSONResponse,
		generated.CreateExercise401JSONResponse,
		generated.GetExercise401JSONResponse,
		generated.CreateExpandedContent401JSONResponse,
		generated.ListExpandedContent401JSONResponse,
		generated.GetExpandedContent401JSONResponse,
		generated.CreateLearningPath401JSONResponse,
		generated.GetLearningPath401JSONResponse,
		generated.AssignLearningPath401JSONResponse,
		generated.GetMyPath401JSONResponse:
		return nil
	default:
		return fmt.Errorf("expected a 401 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
}

func (w *world) requestRejectedInvalid() error {
	_, err := w.validationErrors()
	return err
}

func (w *world) rejectionIdentifiesField(field string) error {
	errs, err := w.validationErrors()
	if err != nil {
		return err
	}
	for _, e := range errs {
		if e.Field == field {
			return nil
		}
	}
	return fmt.Errorf("no validation error identified field %q; got %+v", field, errs)
}

func (w *world) validationErrors() ([]struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}, error) {
	switch resp := w.lastResp.(type) {
	case generated.RegisterUser400JSONResponse:
		return resp.Errors, nil
	case generated.CreateContentNode400JSONResponse:
		return resp.Errors, nil
	case generated.CreateChallenge400JSONResponse:
		return resp.Errors, nil
	case generated.CreateExercise400JSONResponse:
		return resp.Errors, nil
	case generated.CreateExpandedContent400JSONResponse:
		return resp.Errors, nil
	case generated.CreateLearningPath400JSONResponse:
		return resp.Errors, nil
	case generated.AssignLearningPath400JSONResponse:
		return resp.Errors, nil
	default:
		return nil, fmt.Errorf("expected a 400 response with validation errors, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
}
