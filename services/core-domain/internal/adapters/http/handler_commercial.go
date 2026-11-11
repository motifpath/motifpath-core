package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// WithCommercial wires the commercial rollout and audit services. The
// service entry point always calls it; it is separate from NewHandler only to
// keep that constructor's long argument list from growing.
func (h *Handler) WithCommercial(rollout *application.CommercialRolloutService, audit *application.CommercialAuditService) *Handler {
	h.commercialRollout, h.commercialAudit = rollout, audit
	return h
}

// commercialFailure is how a commercial refusal or outage is answered. Any
// error it does not classify is returned as is, which the router answers 500.
type commercialFailure struct {
	status     int
	message    string
	validation generated.ValidationError
}

func classifyCommercialError(err error) (commercialFailure, bool) {
	var invalid *domain.ValidationError
	switch {
	case errors.As(err, &invalid):
		return commercialFailure{status: http.StatusBadRequest, validation: validationErrorResponse(invalid)}, true
	case errors.Is(err, domain.ErrForbidden):
		return commercialFailure{status: http.StatusForbidden, message: "only administrators may manage commercial activation"}, true
	case errors.Is(err, domain.ErrNotFound):
		return commercialFailure{status: http.StatusNotFound, message: err.Error()}, true
	case errors.Is(err, domain.ErrConflict):
		return commercialFailure{status: http.StatusConflict, message: conflictMessage(err)}, true
	case errors.Is(err, domain.ErrUnavailable):
		return commercialFailure{status: http.StatusServiceUnavailable, message: "commercial activation or its evidence is unavailable; nothing was changed, try again"}, true
	default:
		return commercialFailure{}, false
	}
}

func unavailable(message string) generated.UnavailableJSONResponse {
	return generated.UnavailableJSONResponse(generated.CommercialUnavailable{Message: message})
}

// ── Rollout ──────────────────────────────────────────────────────────────────

func (h *Handler) GetCommercialRollout(ctx context.Context, _ generated.GetCommercialRolloutRequestObject) (generated.GetCommercialRolloutResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.GetCommercialRollout401JSONResponse{UnauthenticatedJSONResponse: generated.UnauthenticatedJSONResponse(unauthorizedError())}, nil
	}
	rollout, err := h.commercialRollout.Get(ctx, caller)
	if err != nil {
		failure, known := classifyCommercialError(err)
		switch {
		case !known:
			return nil, err
		case failure.status == http.StatusForbidden:
			return generated.GetCommercialRollout403JSONResponse(forbiddenError(failure.message)), nil
		case failure.status == http.StatusServiceUnavailable:
			return generated.GetCommercialRollout503JSONResponse{UnavailableJSONResponse: unavailable(failure.message)}, nil
		}
		return nil, err
	}
	return generated.GetCommercialRollout200JSONResponse(commercialRolloutResponse(rollout)), nil
}

func (h *Handler) ChangeCommercialFlag(ctx context.Context, request generated.ChangeCommercialFlagRequestObject) (generated.ChangeCommercialFlagResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.ChangeCommercialFlag401JSONResponse{UnauthenticatedJSONResponse: generated.UnauthenticatedJSONResponse(unauthorizedError())}, nil
	}
	body := request.Body
	receipt, err := h.commercialRollout.ChangeFlag(ctx, caller, string(request.Flag), request.Params.IdempotencyKey.String(), application.FlagChangeRequest{
		ExpectedRevision: int64(body.ExpectedRevision), Enabled: body.Enabled, AllUsers: body.AllUsers,
		SubjectIDs: nonNilStringSlice(body.SubjectIds), Reason: body.Reason,
	})
	if err != nil {
		failure, known := classifyCommercialError(err)
		if !known {
			return nil, err
		}
		switch failure.status {
		case http.StatusBadRequest:
			return generated.ChangeCommercialFlag400JSONResponse{InvalidRequestJSONResponse: generated.InvalidRequestJSONResponse(failure.validation)}, nil
		case http.StatusForbidden:
			return generated.ChangeCommercialFlag403JSONResponse(forbiddenError(failure.message)), nil
		case http.StatusNotFound:
			return generated.ChangeCommercialFlag404JSONResponse{NotFoundJSONResponse: generated.NotFoundJSONResponse(notFoundError(failure.message))}, nil
		case http.StatusConflict:
			return generated.ChangeCommercialFlag409JSONResponse{ConflictJSONResponse: generated.ConflictJSONResponse(conflictError(failure.message))}, nil
		default:
			return generated.ChangeCommercialFlag503JSONResponse{UnavailableJSONResponse: unavailable(failure.message)}, nil
		}
	}
	return generated.ChangeCommercialFlag200JSONResponse(rolloutCommand(receipt)), nil
}

func (h *Handler) TurnOffAllCommercialFlags(ctx context.Context, request generated.TurnOffAllCommercialFlagsRequestObject) (generated.TurnOffAllCommercialFlagsResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.TurnOffAllCommercialFlags401JSONResponse{UnauthenticatedJSONResponse: generated.UnauthenticatedJSONResponse(unauthorizedError())}, nil
	}
	receipt, err := h.commercialRollout.KillSwitch(ctx, caller, request.Params.IdempotencyKey.String(), request.Body.Reason)
	if err != nil {
		failure, known := classifyCommercialError(err)
		if !known {
			return nil, err
		}
		switch failure.status {
		case http.StatusBadRequest:
			return generated.TurnOffAllCommercialFlags400JSONResponse{InvalidRequestJSONResponse: generated.InvalidRequestJSONResponse(failure.validation)}, nil
		case http.StatusForbidden:
			return generated.TurnOffAllCommercialFlags403JSONResponse(forbiddenError(failure.message)), nil
		case http.StatusConflict:
			return generated.TurnOffAllCommercialFlags409JSONResponse{ConflictJSONResponse: generated.ConflictJSONResponse(conflictError(failure.message))}, nil
		case http.StatusServiceUnavailable:
			return generated.TurnOffAllCommercialFlags503JSONResponse{UnavailableJSONResponse: unavailable(failure.message)}, nil
		}
		return nil, err
	}
	return generated.TurnOffAllCommercialFlags200JSONResponse(rolloutCommand(receipt)), nil
}

// ── Groups ───────────────────────────────────────────────────────────────────

func (h *Handler) ListCommercialGroups(ctx context.Context, _ generated.ListCommercialGroupsRequestObject) (generated.ListCommercialGroupsResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.ListCommercialGroups401JSONResponse{UnauthenticatedJSONResponse: generated.UnauthenticatedJSONResponse(unauthorizedError())}, nil
	}
	groups, err := h.commercialRollout.ListGroups(ctx, caller)
	if err != nil {
		failure, known := classifyCommercialError(err)
		switch {
		case !known:
			return nil, err
		case failure.status == http.StatusForbidden:
			return generated.ListCommercialGroups403JSONResponse(forbiddenError(failure.message)), nil
		case failure.status == http.StatusServiceUnavailable:
			return generated.ListCommercialGroups503JSONResponse{UnavailableJSONResponse: unavailable(failure.message)}, nil
		}
		return nil, err
	}
	response := generated.ListCommercialGroups200JSONResponse{Revision: int(groups.Revision), Groups: make([]generated.CommercialGroup, len(groups.Groups))}
	for i, group := range groups.Groups {
		response.Groups[i] = commercialGroupResponse(group)
	}
	return response, nil
}

func (h *Handler) SaveCommercialGroup(ctx context.Context, request generated.SaveCommercialGroupRequestObject) (generated.SaveCommercialGroupResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.SaveCommercialGroup401JSONResponse{UnauthenticatedJSONResponse: generated.UnauthenticatedJSONResponse(unauthorizedError())}, nil
	}
	receipt, err := h.commercialRollout.SaveGroup(ctx, caller, request.Slug, request.Params.IdempotencyKey.String(),
		application.GroupChangeRequest{Name: request.Body.Name, Reason: request.Body.Reason})
	if err != nil {
		failure, known := classifyCommercialError(err)
		if !known {
			return nil, err
		}
		switch failure.status {
		case http.StatusBadRequest:
			return generated.SaveCommercialGroup400JSONResponse{InvalidRequestJSONResponse: generated.InvalidRequestJSONResponse(failure.validation)}, nil
		case http.StatusForbidden:
			return generated.SaveCommercialGroup403JSONResponse(forbiddenError(failure.message)), nil
		case http.StatusConflict:
			return generated.SaveCommercialGroup409JSONResponse{ConflictJSONResponse: generated.ConflictJSONResponse(conflictError(failure.message))}, nil
		case http.StatusServiceUnavailable:
			return generated.SaveCommercialGroup503JSONResponse{UnavailableJSONResponse: unavailable(failure.message)}, nil
		}
		return nil, err
	}
	return generated.SaveCommercialGroup200JSONResponse(rolloutCommand(receipt)), nil
}

func (h *Handler) ChangeCommercialGroupMembers(ctx context.Context, request generated.ChangeCommercialGroupMembersRequestObject) (generated.ChangeCommercialGroupMembersResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.ChangeCommercialGroupMembers401JSONResponse{UnauthenticatedJSONResponse: generated.UnauthenticatedJSONResponse(unauthorizedError())}, nil
	}
	body := request.Body
	receipt, err := h.commercialRollout.ChangeMembers(ctx, caller, request.Slug, request.Params.IdempotencyKey.String(), application.MembershipChangeRequest{
		Add: uuidsToStrings(body.Add), Remove: uuidsToStrings(body.Remove), Reason: body.Reason,
	})
	if err != nil {
		failure, known := classifyCommercialError(err)
		if !known {
			return nil, err
		}
		switch failure.status {
		case http.StatusBadRequest:
			return generated.ChangeCommercialGroupMembers400JSONResponse{InvalidRequestJSONResponse: generated.InvalidRequestJSONResponse(failure.validation)}, nil
		case http.StatusForbidden:
			return generated.ChangeCommercialGroupMembers403JSONResponse(forbiddenError(failure.message)), nil
		case http.StatusNotFound:
			return generated.ChangeCommercialGroupMembers404JSONResponse{NotFoundJSONResponse: generated.NotFoundJSONResponse(notFoundError(failure.message))}, nil
		case http.StatusConflict:
			return generated.ChangeCommercialGroupMembers409JSONResponse{ConflictJSONResponse: generated.ConflictJSONResponse(conflictError(failure.message))}, nil
		default:
			return generated.ChangeCommercialGroupMembers503JSONResponse{UnavailableJSONResponse: unavailable(failure.message)}, nil
		}
	}
	return generated.ChangeCommercialGroupMembers200JSONResponse(rolloutCommand(receipt)), nil
}

// ── Audit ────────────────────────────────────────────────────────────────────

func (h *Handler) ListCommercialAuditEntries(ctx context.Context, request generated.ListCommercialAuditEntriesRequestObject) (generated.ListCommercialAuditEntriesResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.ListCommercialAuditEntries401JSONResponse{UnauthenticatedJSONResponse: generated.UnauthenticatedJSONResponse(unauthorizedError())}, nil
	}
	params := request.Params
	query := application.AuditQuery{}
	if params.OperationId != nil {
		query.OperationID = params.OperationId.String()
	}
	if params.ResourceKind != nil {
		query.ResourceKind = *params.ResourceKind
	}
	if params.ResourceId != nil {
		query.ResourceID = *params.ResourceId
	}
	if params.Cursor != nil {
		query.Cursor = *params.Cursor
	}
	if params.Limit != nil {
		query.Limit = *params.Limit
	}
	page, err := h.commercialAudit.List(ctx, caller, query)
	if err != nil {
		failure, known := classifyCommercialError(err)
		if !known {
			return nil, err
		}
		switch failure.status {
		case http.StatusBadRequest:
			return generated.ListCommercialAuditEntries400JSONResponse{InvalidRequestJSONResponse: generated.InvalidRequestJSONResponse(failure.validation)}, nil
		case http.StatusForbidden:
			return generated.ListCommercialAuditEntries403JSONResponse(forbiddenError("only administrators may read commercial evidence")), nil
		case http.StatusServiceUnavailable:
			return generated.ListCommercialAuditEntries503JSONResponse{UnavailableJSONResponse: unavailable("commercial evidence is unavailable; this is not an empty history")}, nil
		}
		return nil, err
	}
	response := generated.ListCommercialAuditEntries200JSONResponse{Items: make([]generated.AuditEntry, len(page.Items)), NextCursor: page.NextCursor}
	for i, entry := range page.Items {
		item, err := auditEntryResponse(entry)
		if err != nil {
			return nil, err
		}
		response.Items[i] = item
	}
	return response, nil
}

// ── Mapping ──────────────────────────────────────────────────────────────────

func commercialRolloutResponse(rollout application.CommercialRollout) generated.CommercialRollout {
	response := generated.CommercialRollout{Revision: int(rollout.Revision), Flags: make([]generated.FlagState, len(rollout.Flags))}
	for i, flag := range rollout.Flags {
		state := generated.FlagState{
			Flag: generated.FlagName(flag.Flag), Enabled: flag.Enabled, AllUsers: flag.AllUsers, Effective: flag.Effective,
			SubjectIds: nonNilStringSlice(flag.SubjectIDs), Prerequisites: make([]generated.FlagName, len(flag.Prerequisites)),
			UpdatedAt: flag.UpdatedAt,
		}
		for j, prerequisite := range flag.Prerequisites {
			state.Prerequisites[j] = generated.FlagName(prerequisite)
		}
		if flag.UpdatedBy != nil {
			by := mustUUID(*flag.UpdatedBy)
			state.UpdatedBy = &by
		}
		response.Flags[i] = state
	}
	return response
}

func rolloutCommand(receipt application.RolloutReceipt) generated.RolloutCommand {
	return generated.RolloutCommand{
		OperationId: mustUUID(receipt.OperationID), Status: generated.RolloutCommandStatus(receipt.Status),
		Revision: int(receipt.Revision), EvidenceEntryId: mustUUID(receipt.EvidenceEntryID),
	}
}

func commercialGroupResponse(group domain.CommercialGroup) generated.CommercialGroup {
	members := make([]uuid.UUID, len(group.MemberIDs))
	for i, member := range group.MemberIDs {
		members[i] = mustUUID(member)
	}
	return generated.CommercialGroup{
		Slug: group.Slug, Name: group.Name, MemberIds: members, UpdatedAt: group.UpdatedAt, UpdatedBy: mustUUID(group.UpdatedBy),
	}
}

func auditEntryResponse(entry domain.AuditEntry) (generated.AuditEntry, error) {
	before, err := commercialState(entry.BeforeState)
	if err != nil {
		return generated.AuditEntry{}, err
	}
	after, err := commercialState(entry.AfterState)
	if err != nil {
		return generated.AuditEntry{}, err
	}
	causes := make([]uuid.UUID, len(entry.CausedByEntryIDs))
	for i, cause := range entry.CausedByEntryIDs {
		causes[i] = mustUUID(cause)
	}
	key := entry.Operation
	return generated.AuditEntry{
		AuditEntryId: mustUUID(entry.AuditEntryID), OperationId: mustUUID(key.OperationID()), AttemptId: mustUUID(entry.AttemptID),
		ActorKind: generated.AuditEntryActorKind(key.ActorKind), ActorId: key.ActorID, Environment: key.Environment.String(),
		OccurredAt: entry.OccurredAt, RecordedAt: entry.RecordedAt, Action: key.Action,
		ResourceKind: key.ResourceKind, ResourceId: key.ResourceID, Reason: entry.Reason,
		Outcome: generated.AuditEntryOutcome(entry.Outcome), BeforeState: before, AfterState: after,
		PolicyRevision: entry.PolicyRevision, ConfigurationRevision: entry.ConfigurationRevision,
		CorrelationId: mustUUID(entry.CorrelationID), CausedByEntryIds: causes,
		ProviderReferences: nonNilStringSlice(entry.ProviderReferences),
	}, nil
}

// commercialState decodes stored evidence state; nil stays null, meaning no
// authoritative state was available.
func commercialState(raw json.RawMessage) (*generated.CommercialState, error) {
	if raw == nil {
		return nil, nil
	}
	var state generated.CommercialState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func nonNilStringSlice(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
