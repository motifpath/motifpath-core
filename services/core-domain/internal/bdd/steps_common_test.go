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
	sc.Step(`^the request is refused with a conflict error$`, w.requestRefusedConflict)
	sc.Step(`^the request is refused with an authentication error$`, w.requestRefusedAuthError)
	sc.Step(`^the request is rejected as invalid$`, w.requestRejectedInvalid)
	sc.Step(`^the rejection identifies "([^"]+)" as the source of the error$`, w.rejectionIdentifiesField)

	sc.Step(`^three distinct exercise identifiers are returned$`, w.threeDistinctIdentifiersReturned)
	sc.Step(`^three distinct expanded content identifiers are returned$`, w.threeDistinctIdentifiersReturned)

	// Shared across the challenges and exercises features: both list a
	// resource by parent ID and assert membership/emptiness the same way.
	sc.Step(`^the response includes "([^"]+)"$`, w.responseIncludes)
	sc.Step(`^the response includes "([^"]+)" and "([^"]+)"$`, w.responseIncludesTwo)
	sc.Step(`^the response does not include "([^"]+)"$`, w.responseDoesNotInclude)
	sc.Step(`^the response is an empty list$`, w.responseIsEmptyList)
}

func (w *world) responseIncludes(slug string) error {
	switch resp := w.lastResp.(type) {
	case generated.ListContentNodeChallenges200JSONResponse:
		want := challengeID(slug)
		for _, c := range resp {
			if c.ChallengeId == want {
				return nil
			}
		}
		return fmt.Errorf("expected challenges to include %s, got %+v", want, resp)
	case generated.ListChallengeExercises200JSONResponse:
		want := exerciseID(slug)
		for _, e := range resp {
			if e.ExerciseId == want {
				return nil
			}
		}
		return fmt.Errorf("expected exercises to include %s, got %+v", want, resp)
	case generated.ListExercises200JSONResponse:
		want := exerciseID(slug)
		for _, e := range resp.Items {
			if e.ExerciseId == want {
				return nil
			}
		}
		return fmt.Errorf("expected exercises to include %s, got %+v", want, resp)
	case generated.ListContentNodes200JSONResponse:
		want := nodeID(slug)
		for _, n := range resp.Items {
			if n.ContentNodeId == want {
				return nil
			}
		}
		return fmt.Errorf("expected content nodes to include %s, got %+v", want, resp)
	case generated.ListLearningPaths200JSONResponse:
		want := pathID(slug)
		for _, p := range resp.Items {
			if p.LearningPathId == want {
				return nil
			}
		}
		return fmt.Errorf("expected learning paths to include %s, got %+v", want, resp)
	case generated.ListDiagrams200JSONResponse:
		want := diagramID(slug)
		for _, d := range resp.Items {
			if d.DiagramId == want {
				return nil
			}
		}
		return fmt.Errorf("expected diagrams to include %s, got %+v", want, resp)
	case generated.ListMyStandalonePaths200JSONResponse:
		want := pathID(slug)
		for _, sp := range resp {
			if sp.SourceTemplateId == want {
				return nil
			}
		}
		return fmt.Errorf("expected student paths to include %q, got %+v", slug, resp)
	case generated.ListCourses200JSONResponse:
		for _, c := range resp.Items {
			if w.courseMatchesSlug(c, slug) {
				return nil
			}
		}
		return fmt.Errorf("expected courses to include %q, got %+v", slug, resp.Items)
	default:
		return fmt.Errorf("expected a list response, got %#v", w.lastResp)
	}
}

func (w *world) responseIncludesTwo(slugA, slugB string) error {
	if err := w.responseIncludes(slugA); err != nil {
		return err
	}
	return w.responseIncludes(slugB)
}

func (w *world) responseDoesNotInclude(slug string) error {
	switch resp := w.lastResp.(type) {
	case generated.ListExercises200JSONResponse:
		want := exerciseID(slug)
		for _, e := range resp.Items {
			if e.ExerciseId == want {
				return fmt.Errorf("expected exercises not to include %s, got %+v", want, resp)
			}
		}
		return nil
	case generated.ListContentNodes200JSONResponse:
		want := nodeID(slug)
		for _, n := range resp.Items {
			if n.ContentNodeId == want {
				return fmt.Errorf("expected content nodes not to include %s, got %+v", want, resp)
			}
		}
		return nil
	case generated.ListDiagrams200JSONResponse:
		want := diagramID(slug)
		for _, d := range resp.Items {
			if d.DiagramId == want {
				return fmt.Errorf("expected diagrams not to include %s, got %+v", want, resp)
			}
		}
		return nil
	case generated.ListLearningPaths200JSONResponse:
		want := pathID(slug)
		for _, p := range resp.Items {
			if p.LearningPathId == want {
				return fmt.Errorf("expected learning paths not to include %q, got %+v", slug, resp.Items)
			}
		}
		return nil
	case generated.ListMyStandalonePaths200JSONResponse:
		want := pathID(slug)
		for _, sp := range resp {
			if sp.SourceTemplateId == want {
				return fmt.Errorf("expected student paths not to include %q, got %+v", slug, resp)
			}
		}
		return nil
	case generated.ListCourses200JSONResponse:
		for _, c := range resp.Items {
			if w.courseMatchesSlug(c, slug) {
				return fmt.Errorf("expected courses not to include %q, got %+v", slug, resp.Items)
			}
		}
		return nil
	default:
		return fmt.Errorf("expected a list response, got %#v", w.lastResp)
	}
}

func (w *world) responseIsEmptyList() error {
	switch resp := w.lastResp.(type) {
	case generated.ListContentNodeChallenges200JSONResponse:
		if len(resp) != 0 {
			return fmt.Errorf("expected an empty list, got %d challenges", len(resp))
		}
	case generated.ListChallengeExercises200JSONResponse:
		if len(resp) != 0 {
			return fmt.Errorf("expected an empty list, got %d exercises", len(resp))
		}
	case generated.ListContentNodePathExercises200JSONResponse:
		if len(resp) != 0 {
			return fmt.Errorf("expected an empty list, got %d path exercises", len(resp))
		}
	case generated.ListExercises200JSONResponse:
		if len(resp.Items) != 0 {
			return fmt.Errorf("expected an empty list, got %d exercises", len(resp.Items))
		}
	case generated.ListContentNodes200JSONResponse:
		if len(resp.Items) != 0 {
			return fmt.Errorf("expected an empty list, got %d content nodes", len(resp.Items))
		}
	case generated.ListLearningPaths200JSONResponse:
		if len(resp.Items) != 0 {
			return fmt.Errorf("expected an empty list, got %d learning paths", len(resp.Items))
		}
	case generated.ListSkills200JSONResponse:
		if len(resp) != 0 {
			return fmt.Errorf("expected an empty list, got %d skills", len(resp))
		}
	case generated.ListConcepts200JSONResponse:
		if len(resp) != 0 {
			return fmt.Errorf("expected an empty list, got %d concepts", len(resp))
		}
	case generated.ListInstruments200JSONResponse:
		if len(resp) != 0 {
			return fmt.Errorf("expected an empty list, got %d instruments", len(resp))
		}
	case generated.ListDiagrams200JSONResponse:
		if len(resp.Items) != 0 {
			return fmt.Errorf("expected an empty list, got %d diagrams", len(resp.Items))
		}
	case generated.ListCourses200JSONResponse:
		if len(resp.Items) != 0 {
			return fmt.Errorf("expected an empty list, got %d courses", len(resp.Items))
		}
	case generated.ListMyStandalonePaths200JSONResponse:
		if len(resp) != 0 {
			return fmt.Errorf("expected an empty list, got %d student paths", len(resp))
		}
	case generated.ListContentNodeVersions200JSONResponse:
		if len(resp) != 0 {
			return fmt.Errorf("expected an empty list, got %d versions", len(resp))
		}
	default:
		return fmt.Errorf("expected a list response, got %#v", w.lastResp)
	}
	return nil
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
	w.persona = ""
	return nil
}

func (w *world) requestRefusedForbidden() error {
	switch w.lastResp.(type) {
	case generated.CreateContentNode403JSONResponse,
		generated.CreateChallenge403JSONResponse,
		generated.CreateExercise403JSONResponse,
		generated.ListExercises403JSONResponse,
		generated.UpdateExercise403JSONResponse,
		generated.LinkExerciseToChallenge403JSONResponse,
		generated.UnlinkExerciseFromChallenge403JSONResponse,
		generated.LinkExerciseToContentNode403JSONResponse,
		generated.UnlinkExerciseFromContentNode403JSONResponse,
		generated.CreateMediaUploadUrl403JSONResponse,
		generated.CreateExpandedContent403JSONResponse,
		generated.CreateLearningPath403JSONResponse,
		generated.GetLearningPath403JSONResponse,
		generated.AssignLearningPath403JSONResponse,
		generated.ListContentNodes403JSONResponse,
		generated.ListContentNodeVersions403JSONResponse,
		generated.ListCourses403JSONResponse,
		generated.ListDiagrams403JSONResponse,
		generated.UpdateInstrument403JSONResponse,
		generated.UpdateContentNode403JSONResponse,
		generated.UpdateChallenge403JSONResponse,
		generated.UpdateExpandedContent403JSONResponse,
		generated.DeleteExpandedContent403JSONResponse,
		generated.ListLearningPaths403JSONResponse,
		generated.ReplaceLearningPath403JSONResponse,
		generated.DeleteLearningPath403JSONResponse,
		generated.CreateSkill403JSONResponse,
		generated.CreateConcept403JSONResponse,
		generated.CreateInstrument403JSONResponse,
		generated.CreateDiagram403JSONResponse,
		generated.UpdateDiagram403JSONResponse,
		generated.PublishContentNode403JSONResponse,
		generated.CreateCourse403JSONResponse,
		generated.GetCourse403JSONResponse,
		generated.ReplaceCourse403JSONResponse,
		generated.PublishCourse403JSONResponse,
		generated.RetireCourse403JSONResponse,
		generated.ReactivateCourse403JSONResponse,
		generated.ListCourseCreators403JSONResponse:
		return nil
	default:
		return fmt.Errorf("expected a 403 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
}

func (w *world) requestRefusedNotFound() error {
	switch w.lastResp.(type) {
	case generated.GetContentNode404JSONResponse,
		generated.UpdateInstrument404JSONResponse,
		generated.ListContentNodeVersions404JSONResponse,
		generated.CreateChallenge404JSONResponse,
		generated.GetChallenge404JSONResponse,
		generated.ListContentNodeChallenges404JSONResponse,
		generated.GetExercise404JSONResponse,
		generated.UpdateExercise404JSONResponse,
		generated.LinkExerciseToChallenge404JSONResponse,
		generated.UnlinkExerciseFromChallenge404JSONResponse,
		generated.ListChallengeExercises404JSONResponse,
		generated.ListContentNodePathExercises404JSONResponse,
		generated.LinkExerciseToContentNode404JSONResponse,
		generated.UnlinkExerciseFromContentNode404JSONResponse,
		generated.CreateMediaUploadUrl404JSONResponse,
		generated.CreateExpandedContent404JSONResponse,
		generated.ListExpandedContent404JSONResponse,
		generated.GetExpandedContent404JSONResponse,
		generated.GetLearningPath404JSONResponse,
		generated.AssignLearningPath404JSONResponse,
		generated.GetMyPath404JSONResponse,
		generated.GetMyProfile404JSONResponse,
		generated.UpdateContentNode404JSONResponse,
		generated.UpdateChallenge404JSONResponse,
		generated.UpdateExpandedContent404JSONResponse,
		generated.DeleteExpandedContent404JSONResponse,
		generated.ReplaceLearningPath404JSONResponse,
		generated.DeleteLearningPath404JSONResponse,
		generated.UpdateMyLocale404JSONResponse,
		generated.GetDiagram404JSONResponse,
		generated.UpdateDiagram404JSONResponse,
		generated.PublishContentNode404JSONResponse,
		generated.ArchiveStandaloneStudentPath404JSONResponse,
		generated.GetCourse404JSONResponse,
		generated.GetPublishedCourse404JSONResponse,
		generated.ReplaceCourse404JSONResponse,
		generated.PublishCourse404JSONResponse,
		generated.RetireCourse404JSONResponse,
		generated.ReactivateCourse404JSONResponse,
		generated.CreateCourseEnrollment404JSONResponse,
		generated.AbandonCourseEnrollment404JSONResponse,
		generated.SetCurrentPath404JSONResponse:
		return nil
	default:
		return fmt.Errorf("expected a 404 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
}

func (w *world) requestRefusedConflict() error {
	switch w.lastResp.(type) {
	case generated.RegisterUser409JSONResponse,
		generated.LinkExerciseToChallenge409JSONResponse,
		generated.LinkExerciseToContentNode409JSONResponse,
		generated.ArchiveStandaloneStudentPath409JSONResponse,
		generated.DeleteLearningPath409JSONResponse,
		generated.CreateCourseEnrollment409JSONResponse,
		generated.AbandonCourseEnrollment409JSONResponse:
		return nil
	default:
		return fmt.Errorf("expected a 409 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
}

func (w *world) requestRefusedAuthError() error {
	switch w.lastResp.(type) {
	case generated.RegisterUser401JSONResponse,
		generated.ListContentNodeVersions401JSONResponse,
		generated.ListCourseCreators401JSONResponse,
		generated.ListCatalogCreators401JSONResponse,
		generated.ListCatalogCourses401JSONResponse,
		generated.ListMyStandalonePaths401JSONResponse,
		generated.GetMyProfile401JSONResponse,
		generated.CreateContentNode401JSONResponse,
		generated.GetContentNode401JSONResponse,
		generated.CreateChallenge401JSONResponse,
		generated.GetChallenge401JSONResponse,
		generated.ListContentNodeChallenges401JSONResponse,
		generated.CreateExercise401JSONResponse,
		generated.GetExercise401JSONResponse,
		generated.ListExercises401JSONResponse,
		generated.UpdateExercise401JSONResponse,
		generated.LinkExerciseToChallenge401JSONResponse,
		generated.UnlinkExerciseFromChallenge401JSONResponse,
		generated.ListChallengeExercises401JSONResponse,
		generated.ListContentNodePathExercises401JSONResponse,
		generated.LinkExerciseToContentNode401JSONResponse,
		generated.UnlinkExerciseFromContentNode401JSONResponse,
		generated.StartPracticeSession401JSONResponse,
		generated.CreateMediaUploadUrl401JSONResponse,
		generated.CreateExpandedContent401JSONResponse,
		generated.ListExpandedContent401JSONResponse,
		generated.GetExpandedContent401JSONResponse,
		generated.CreateLearningPath401JSONResponse,
		generated.GetLearningPath401JSONResponse,
		generated.AssignLearningPath401JSONResponse,
		generated.GetMyPath401JSONResponse,
		generated.ListContentNodes401JSONResponse,
		generated.UpdateContentNode401JSONResponse,
		generated.UpdateChallenge401JSONResponse,
		generated.UpdateExpandedContent401JSONResponse,
		generated.DeleteExpandedContent401JSONResponse,
		generated.ListLearningPaths401JSONResponse,
		generated.ReplaceLearningPath401JSONResponse,
		generated.DeleteLearningPath401JSONResponse,
		generated.UpdateMyLocale401JSONResponse,
		generated.ListSkills401JSONResponse,
		generated.CreateSkill401JSONResponse,
		generated.ListConcepts401JSONResponse,
		generated.CreateConcept401JSONResponse,
		generated.ListInstruments401JSONResponse,
		generated.CreateInstrument401JSONResponse,
		generated.UpdateInstrument401JSONResponse,
		generated.ListDiagrams401JSONResponse,
		generated.CreateDiagram401JSONResponse,
		generated.GetDiagram401JSONResponse,
		generated.UpdateDiagram401JSONResponse,
		generated.PublishContentNode401JSONResponse,
		generated.ArchiveStandaloneStudentPath401JSONResponse,
		generated.CreateCourse401JSONResponse,
		generated.GetCourse401JSONResponse,
		generated.ReplaceCourse401JSONResponse,
		generated.PublishCourse401JSONResponse,
		generated.RetireCourse401JSONResponse,
		generated.GetPublishedCourse401JSONResponse,
		generated.ListCourses401JSONResponse,
		generated.CreateCourseEnrollment401JSONResponse,
		generated.ListMyCourseEnrollments401JSONResponse,
		generated.AbandonCourseEnrollment401JSONResponse,
		generated.SetCurrentPath401JSONResponse:
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
	case generated.StartPracticeSession400JSONResponse:
		return resp.Errors, nil
	case generated.CreateContentNode400JSONResponse:
		return resp.Errors, nil
	case generated.CreateChallenge400JSONResponse:
		return resp.Errors, nil
	case generated.CreateExercise400JSONResponse:
		return resp.Errors, nil
	case generated.UpdateExercise400JSONResponse:
		return resp.Errors, nil
	case generated.CreateExpandedContent400JSONResponse:
		return resp.Errors, nil
	case generated.CreateLearningPath400JSONResponse:
		return resp.Errors, nil
	case generated.CreateMediaUploadUrl400JSONResponse:
		return resp.Errors, nil
	case generated.AssignLearningPath400JSONResponse:
		return resp.Errors, nil
	case generated.UpdateContentNode400JSONResponse:
		return resp.Errors, nil
	case generated.UpdateChallenge400JSONResponse:
		return resp.Errors, nil
	case generated.UpdateExpandedContent400JSONResponse:
		return resp.Errors, nil
	case generated.ReplaceLearningPath400JSONResponse:
		return resp.Errors, nil
	case generated.UpdateMyLocale400JSONResponse:
		return resp.Errors, nil
	case generated.CreateSkill400JSONResponse:
		return resp.Errors, nil
	case generated.CreateConcept400JSONResponse:
		return resp.Errors, nil
	case generated.CreateInstrument400JSONResponse:
		return resp.Errors, nil
	case generated.CreateDiagram400JSONResponse:
		return resp.Errors, nil
	case generated.UpdateDiagram400JSONResponse:
		return resp.Errors, nil
	case generated.ListContentNodes400JSONResponse:
		return resp.Errors, nil
	case generated.ListExercises400JSONResponse:
		return resp.Errors, nil
	case generated.ListLearningPaths400JSONResponse:
		return resp.Errors, nil
	case generated.ListCourses400JSONResponse:
		return resp.Errors, nil
	case generated.ListCatalogCourses400JSONResponse:
		return resp.Errors, nil
	case generated.ListDiagrams400JSONResponse:
		return resp.Errors, nil
	case generated.UpdateInstrument400JSONResponse:
		return resp.Errors, nil
	case generated.CreateCourse400JSONResponse:
		return resp.Errors, nil
	case generated.ReplaceCourse400JSONResponse:
		return resp.Errors, nil
	case generated.ReactivateCourse400JSONResponse:
		return resp.Errors, nil
	case generated.CreateCourseEnrollment400JSONResponse:
		return resp.Errors, nil
	case generated.SetCurrentPath400JSONResponse:
		return resp.Errors, nil
	default:
		return nil, fmt.Errorf("expected a 400 response with validation errors, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
}
