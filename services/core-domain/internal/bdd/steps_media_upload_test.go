//go:build integration

package bdd

import (
	"fmt"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
)

func registerMediaUploadSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)" requests a media upload URL for purpose "([^"]+)" on exercise "([^"]+)" with content type "([^"]+)" and file name "([^"]+)"$`, w.requestsMediaUploadURLForExercise)
	sc.Step(`^"([^"]+)" requests a media upload URL for purpose "([^"]+)" with content type "([^"]+)" and file name "([^"]+)"$`, w.requestsMediaUploadURLForLibrary)
	sc.Step(`^"([^"]+)" requests a media upload URL for purpose "([^"]+)" with content type "([^"]+)" and file name "([^"]+)" and no exercise ID$`, w.requestsMediaUploadURLNoExerciseID)
	sc.Step(`^"([^"]+)" submits a media upload URL request with the content_type field omitted$`, w.submitsMediaUploadURLMissingContentType)
	sc.Step(`^"([^"]+)" requests a media upload URL for purpose "([^"]+)" on an exercise ID that does not exist$`, w.requestsMediaUploadURLForMissingExercise)
	sc.Step(`^"([^"]+)" attempts to request a media upload URL for purpose "([^"]+)"$`, w.attemptsRequestMediaUploadURL)
	sc.Step(`^an unauthenticated request attempts to request a media upload URL$`, w.unauthRequestsMediaUploadURL)

	sc.Step(`^a presigned upload URL is returned$`, w.presignedUploadURLReturned)
	sc.Step(`^the response includes the object's read URL and an expiry time$`, w.mediaUploadResponseComplete)
}

func (w *world) createMediaUploadURLRequest(purpose, exerciseSlug, contentType, fileName string) generated.CreateMediaUploadUrlRequestObject {
	req := &generated.CreateMediaUploadUrlRequest{
		Purpose:     generated.CreateMediaUploadUrlRequestPurpose(purpose),
		ContentType: generated.CreateMediaUploadUrlRequestContentType(contentType),
		FileName:    fileName,
	}
	if exerciseSlug != "" {
		id := exerciseID(exerciseSlug)
		req.ExerciseId = &id
	}
	return generated.CreateMediaUploadUrlRequestObject{Body: req}
}

func (w *world) requestsMediaUploadURLForExercise(name, purpose, exerciseSlug, contentType, fileName string) error {
	resp, err := w.handler.CreateMediaUploadUrl(w.ctx(), w.createMediaUploadURLRequest(purpose, exerciseSlug, contentType, fileName))
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) requestsMediaUploadURLForLibrary(name, purpose, contentType, fileName string) error {
	resp, err := w.handler.CreateMediaUploadUrl(w.ctx(), w.createMediaUploadURLRequest(purpose, "", contentType, fileName))
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) requestsMediaUploadURLNoExerciseID(name, purpose, contentType, fileName string) error {
	return w.requestsMediaUploadURLForLibrary(name, purpose, contentType, fileName)
}

func (w *world) submitsMediaUploadURLMissingContentType(name string) error {
	req := w.createMediaUploadURLRequest("library_asset", "", "", "diagram.png")
	resp, err := w.handler.CreateMediaUploadUrl(w.ctx(), req)
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) requestsMediaUploadURLForMissingExercise(name, purpose string) error {
	id := deterministicUUID("exercise", "does-not-exist")
	req := &generated.CreateMediaUploadUrlRequest{
		Purpose:     generated.CreateMediaUploadUrlRequestPurpose(purpose),
		ContentType: generated.CreateMediaUploadUrlRequestContentTypeImage,
		FileName:    "diagram.png",
		ExerciseId:  &id,
	}
	resp, err := w.handler.CreateMediaUploadUrl(w.ctx(), generated.CreateMediaUploadUrlRequestObject{Body: req})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) attemptsRequestMediaUploadURL(name, purpose string) error {
	resp, err := w.handler.CreateMediaUploadUrl(w.ctx(), w.createMediaUploadURLRequest(purpose, "", "image", "diagram.png"))
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthRequestsMediaUploadURL() error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.attemptsRequestMediaUploadURL("", "library_asset")
}

func (w *world) presignedUploadURLReturned() error {
	if _, ok := w.lastResp.(generated.CreateMediaUploadUrl201JSONResponse); !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}

func (w *world) mediaUploadResponseComplete() error {
	resp, ok := w.lastResp.(generated.CreateMediaUploadUrl201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.ObjectUrl == "" || resp.ExpiresAt.IsZero() {
		return fmt.Errorf("expected object_url and expires_at to be populated, got %+v", resp)
	}
	return nil
}
