package http

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// WithCommercialContent sets the service that reads commercial content and
// corrects its ownership.
func (h *Handler) WithCommercialContent(content *application.CommercialContentService) *Handler {
	h.commercialContent = content
	return h
}

func (h *Handler) GetCommercialContent(ctx context.Context, request generated.GetCommercialContentRequestObject) (generated.GetCommercialContentResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.GetCommercialContent401JSONResponse{UnauthenticatedJSONResponse: generated.UnauthenticatedJSONResponse(unauthorizedError())}, nil
	}
	content, err := h.commercialContent.Get(ctx, caller, string(request.ContentKind), request.ContentId.String())
	if err != nil {
		failure, known := classifyCommercialError(err)
		if !known {
			return nil, err
		}
		switch failure.status {
		case http.StatusBadRequest:
			return generated.GetCommercialContent400JSONResponse{InvalidRequestJSONResponse: generated.InvalidRequestJSONResponse(failure.validation)}, nil
		case http.StatusNotFound:
			return generated.GetCommercialContent404JSONResponse{NotFoundJSONResponse: generated.NotFoundJSONResponse(notFoundError("commercial content not found"))}, nil
		case http.StatusServiceUnavailable:
			return generated.GetCommercialContent503JSONResponse{UnavailableJSONResponse: unavailable("commercial content is unavailable; try again")}, nil
		}
		return nil, err
	}
	response, err := commercialContentResponse(content)
	if err != nil {
		return nil, err
	}
	return generated.GetCommercialContent200JSONResponse(response), nil
}

func (h *Handler) CorrectCommercialOwnership(ctx context.Context, request generated.CorrectCommercialOwnershipRequestObject) (generated.CorrectCommercialOwnershipResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.CorrectCommercialOwnership401JSONResponse{UnauthenticatedJSONResponse: generated.UnauthenticatedJSONResponse(unauthorizedError())}, nil
	}
	body := request.Body
	receipt, err := h.commercialContent.CorrectOwnership(ctx, caller, string(request.ContentKind), request.ContentId.String(),
		request.Params.IdempotencyKey.String(), application.OwnershipCorrectionRequest{
			ExpectedRevision: body.ExpectedRevision, OwnerKind: string(body.OwnerKind), OwnerID: body.OwnerId.String(), Reason: body.Reason,
		})
	if err != nil {
		failure, known := classifyCommercialError(err)
		if !known {
			return nil, err
		}
		switch failure.status {
		case http.StatusBadRequest:
			return generated.CorrectCommercialOwnership400JSONResponse{InvalidRequestJSONResponse: generated.InvalidRequestJSONResponse(failure.validation)}, nil
		case http.StatusForbidden:
			return generated.CorrectCommercialOwnership403JSONResponse(forbiddenError("only an administrator corrects ownership, and only while content-ownership corrections are rolled out")), nil
		case http.StatusNotFound:
			return generated.CorrectCommercialOwnership404JSONResponse{NotFoundJSONResponse: generated.NotFoundJSONResponse(notFoundError("commercial content not found"))}, nil
		case http.StatusConflict:
			return generated.CorrectCommercialOwnership409JSONResponse{ConflictJSONResponse: generated.ConflictJSONResponse(conflictError(failure.message))}, nil
		default:
			return generated.CorrectCommercialOwnership503JSONResponse{UnavailableJSONResponse: unavailable(failure.message)}, nil
		}
	}
	response, err := commandResponse(receipt)
	if err != nil {
		return nil, err
	}
	return generated.CorrectCommercialOwnership200JSONResponse(response), nil
}

func commercialContentResponse(content application.CommercialContent) (generated.CommercialContent, error) {
	ownership, err := ownershipResponse(content.Ownership)
	if err != nil {
		return generated.CommercialContent{}, err
	}
	blockers := make([]generated.SaleBlocker, 0, len(content.SaleBlockers))
	for _, blocker := range content.SaleBlockers {
		item := generated.SaleBlocker{Code: generated.SaleBlockerCode(blocker.Code)}
		if blocker.Content != nil {
			ref, err := contentRefResponse(*blocker.Content)
			if err != nil {
				return generated.CommercialContent{}, err
			}
			item.Content = &ref
		}
		blockers = append(blockers, item)
	}
	var release *generated.Release
	if content.Release != nil {
		latest, err := releaseResponse(*content.Release)
		if err != nil {
			return generated.CommercialContent{}, err
		}
		release = &latest
	}
	return generated.CommercialContent{
		Ownership: ownership, Revision: content.Revision, SaleState: generated.CommercialContentSaleState(content.SaleState),
		Release: release, SaleBlockers: blockers,
	}, nil
}

func ownershipResponse(ownership domain.ContentOwnership) (generated.Ownership, error) {
	content, err := contentRefResponse(ownership.Content)
	if err != nil {
		return generated.Ownership{}, err
	}
	evidence, err := uuid.Parse(ownership.EvidenceEntryID)
	if err != nil {
		return generated.Ownership{}, err
	}
	ownerID, err := optionalUUID(ownership.OwnerID)
	if err != nil {
		return generated.Ownership{}, err
	}
	creatorID, err := optionalUUID(ownership.CreatorID)
	if err != nil {
		return generated.Ownership{}, err
	}
	return generated.Ownership{
		Content: content, OwnerKind: generated.OwnershipOwnerKind(ownership.OwnerKind), OwnerId: ownerID, CreatorId: creatorID,
		Source: generated.OwnershipSource(ownership.Source), EvidenceEntryId: evidence, Revision: ownership.Revision,
		CopiedFrom: nil,
	}, nil
}

func contentRefResponse(ref domain.ContentRef) (generated.ContentRef, error) {
	id, err := uuid.Parse(ref.ID)
	if err != nil {
		return generated.ContentRef{}, err
	}
	return generated.ContentRef{Kind: generated.ContentRefKind(ref.Kind), ContentId: id}, nil
}

func optionalUUID(value *string) (*uuid.UUID, error) {
	if value == nil {
		return nil, nil
	}
	id, err := uuid.Parse(*value)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func commandResponse(receipt application.CommandReceipt) (generated.Command, error) {
	operation, err := uuid.Parse(receipt.OperationID)
	if err != nil {
		return generated.Command{}, err
	}
	resource, err := uuid.Parse(receipt.ResourceID)
	if err != nil {
		return generated.Command{}, err
	}
	evidence, err := uuid.Parse(receipt.EvidenceEntryID)
	if err != nil {
		return generated.Command{}, err
	}
	return generated.Command{
		OperationId: operation, ResourceId: resource, Status: generated.CommandStatus(receipt.Status), EvidenceEntryId: evidence,
	}, nil
}
