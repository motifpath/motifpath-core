//go:build integration

package bdd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/cucumber/godog"

	appHTTP "github.com/motifpath/event-ingestion/internal/adapters/http"
	"github.com/motifpath/event-ingestion/internal/adapters/http/generated"
	"github.com/motifpath/event-ingestion/internal/domain"
	"github.com/motifpath/event-ingestion/internal/ports"
)

func registerAdminSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^a lesson\.started event "([^"]+)" failed every delivery attempt and is now dead-lettered$`, w.eventIsDeadLettered)

	sc.Step(`^the caller is authenticated and the platform recognises them as an administrator$`, w.callerIsAdministrator)
	sc.Step(`^the caller is authenticated and the platform recognises them as a student$`, w.callerIsStudent)
	sc.Step(`^the caller is authenticated and their token asserts the administrator role$`, w.callerAuthenticatedWithAdminClaim)
	sc.Step(`^the platform recognises them as a teacher$`, w.platformRecognisesTeacher)
	sc.Step(`^the caller is authenticated but has never registered with the platform$`, w.callerAuthenticatedButUnregistered)
	sc.Step(`^the caller is authenticated$`, w.callerIsAuthenticated)
	sc.Step(`^the platform cannot currently establish the caller's role$`, w.roleCannotBeEstablished)
	sc.Step(`^the event stream is reachable again$`, func() error { return nil })
	sc.Step(`^event "([^"]+)" has already been delivered to the event stream$`, w.eventAlreadyDelivered)

	sc.Step(`^the caller asks to retry delivery of event "([^"]+)"$`, w.retryDelivery)
	sc.Step(`^an unauthenticated request asks to retry delivery of event "([^"]+)"$`, w.retryDelivery)
	sc.Step(`^the caller resolves event "([^"]+)" with the reason "([^"]+)"$`, w.resolveWithReason)
	sc.Step(`^the caller resolves event "([^"]+)" without a reason$`, w.resolveWithoutReason)

	sc.Step(`^the event is delivered to the event stream$`, w.eventWasDelivered)
	sc.Step(`^the delivery is confirmed as complete$`, w.deliveryConfirmedComplete)
	sc.Step(`^the entry is recorded as manually resolved$`, w.entryRecordedResolved)
	sc.Step(`^no delivery to the event stream is attempted$`, w.noDeliveryAttempted)
	sc.Step(`^no further delivery to the event stream is attempted$`, w.noDeliveryAttempted)
	sc.Step(`^the platform reports that no such event is awaiting delivery$`, w.reportedNotAwaitingDelivery)
	sc.Step(`^the remediation is refused because the caller is not an administrator$`, w.refusedNotAdministrator)
	sc.Step(`^the remediation is refused as temporarily unavailable$`, w.refusedTemporarilyUnavailable)
	sc.Step(`^the remediation is refused with an authentication error$`, w.refusedAuthError)
}

// ── Given ──────────────────────────────────────────────────────────────────

func (w *world) eventIsDeadLettered(eventName string) error {
	id := deterministicUUID("event", eventName).String()
	event := domain.LessonStartedEvent{
		TrackingEventBase: domain.TrackingEventBase{
			EventID:    id,
			EventType:  domain.EventTypeLessonStarted,
			StudentID:  studentID("dead-letter-owner"),
			SessionID:  fixedSessionID.String(),
			OccurredAt: fixedOccurredAt,
		},
		ContentContext: domain.ContentContext{ContentNodeID: deterministicUUID("node", "stuck-node").String()},
	}
	ctx := context.Background()
	if _, _, err := w.repo.Save(ctx, event); err != nil {
		return err
	}
	if err := w.outbox.Create(ctx, id); err != nil {
		return err
	}
	entry, _, err := w.outbox.Get(ctx, id)
	if err != nil {
		return err
	}
	entry.Status = domain.OutboxStatusDead
	return w.outbox.Update(ctx, entry)
}

func (w *world) callerIsAdministrator() error {
	w.hasBearerToken = true
	w.roleResolver.role = "admin"
	return nil
}

func (w *world) callerIsStudent() error {
	w.hasBearerToken = true
	w.roleResolver.role = "student"
	return nil
}

func (w *world) callerAuthenticatedWithAdminClaim() error {
	// The token's own claims are irrelevant to authorization (ADR-013); this
	// step only establishes that a valid token is present.
	w.hasBearerToken = true
	return nil
}

func (w *world) platformRecognisesTeacher() error {
	w.roleResolver.role = "teacher"
	return nil
}

func (w *world) callerAuthenticatedButUnregistered() error {
	w.hasBearerToken = true
	w.roleResolver.err = ports.ErrIdentityNotRegistered
	return nil
}

func (w *world) callerIsAuthenticated() error {
	w.hasBearerToken = true
	return nil
}

func (w *world) roleCannotBeEstablished() error {
	w.roleResolver.err = ports.ErrRoleUnavailable
	return nil
}

func (w *world) eventAlreadyDelivered(eventName string) error {
	id := deterministicUUID("event", eventName).String()
	return w.outbox.MarkPublished(context.Background(), id)
}

// ── When ───────────────────────────────────────────────────────────────────

func (w *world) adminCtx() context.Context {
	ctx := context.Background()
	if w.hasBearerToken {
		ctx = appHTTP.WithBearerToken(ctx, "bdd-caller-token")
	}
	return ctx
}

func (w *world) retryDelivery(eventName string) error {
	w.publishCountBeforeAdmin = len(w.publisher.sent)
	id := deterministicUUID("event", eventName)
	resp, err := w.handler.RetryPublishOutboxEntry(w.adminCtx(), generated.RetryPublishOutboxEntryRequestObject{EventId: id})
	if err != nil {
		w.adminErr = err
		return nil
	}
	rec := httptest.NewRecorder()
	if verr := resp.VisitRetryPublishOutboxEntryResponse(rec); verr != nil {
		return verr
	}
	w.adminStatusCode = rec.Code
	w.adminBody = rec.Body.Bytes()
	return nil
}

func (w *world) resolveWithReason(eventName, reason string) error {
	return w.resolve(eventName, &reason)
}

func (w *world) resolveWithoutReason(eventName string) error {
	return w.resolve(eventName, nil)
}

func (w *world) resolve(eventName string, reason *string) error {
	w.publishCountBeforeAdmin = len(w.publisher.sent)
	id := deterministicUUID("event", eventName)
	req := generated.ResolvePublishOutboxEntryRequestObject{EventId: id}
	if reason != nil {
		req.Body = &generated.ResolvePublishOutboxEntryJSONRequestBody{Reason: reason}
	}
	resp, err := w.handler.ResolvePublishOutboxEntry(w.adminCtx(), req)
	if err != nil {
		w.adminErr = err
		return nil
	}
	rec := httptest.NewRecorder()
	if verr := resp.VisitResolvePublishOutboxEntryResponse(rec); verr != nil {
		return verr
	}
	w.adminStatusCode = rec.Code
	w.adminBody = rec.Body.Bytes()
	return nil
}

// ── Then ───────────────────────────────────────────────────────────────────

func (w *world) adminOutcome() (int, error) {
	if w.adminErr != nil {
		return 0, fmt.Errorf("admin operation returned an unexpected error: %w", w.adminErr)
	}
	return w.adminStatusCode, nil
}

func (w *world) expectAdminStatus(want int) error {
	got, err := w.adminOutcome()
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("expected status %d, got %d (body: %s)", want, got, w.adminBody)
	}
	return nil
}

func (w *world) adminEntryStatus() (string, error) {
	got, err := w.adminOutcome()
	if err != nil {
		return "", err
	}
	if got != http.StatusOK {
		return "", fmt.Errorf("expected a 200 response, got %d (body: %s)", got, w.adminBody)
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(w.adminBody, &body); err != nil {
		return "", fmt.Errorf("decode outbox entry response: %w", err)
	}
	return body.Status, nil
}

func (w *world) eventWasDelivered() error {
	if len(w.publisher.sent) <= w.publishCountBeforeAdmin {
		return fmt.Errorf("expected the event to be published to the stream, but no publish occurred")
	}
	return nil
}

func (w *world) deliveryConfirmedComplete() error {
	status, err := w.adminEntryStatus()
	if err != nil {
		return err
	}
	if status != string(domain.OutboxStatusPublished) {
		return fmt.Errorf("expected entry status %q, got %q", domain.OutboxStatusPublished, status)
	}
	return nil
}

func (w *world) entryRecordedResolved() error {
	status, err := w.adminEntryStatus()
	if err != nil {
		return err
	}
	if status != string(domain.OutboxStatusResolvedManually) {
		return fmt.Errorf("expected entry status %q, got %q", domain.OutboxStatusResolvedManually, status)
	}
	return nil
}

func (w *world) noDeliveryAttempted() error {
	if len(w.publisher.sent) != w.publishCountBeforeAdmin {
		return fmt.Errorf("expected no publish to the event stream, but %d occurred", len(w.publisher.sent)-w.publishCountBeforeAdmin)
	}
	return nil
}

func (w *world) reportedNotAwaitingDelivery() error {
	return w.expectAdminStatus(http.StatusNotFound)
}

func (w *world) refusedNotAdministrator() error {
	return w.expectAdminStatus(http.StatusForbidden)
}

func (w *world) refusedTemporarilyUnavailable() error {
	return w.expectAdminStatus(http.StatusServiceUnavailable)
}

func (w *world) refusedAuthError() error {
	return w.expectAdminStatus(http.StatusUnauthorized)
}
