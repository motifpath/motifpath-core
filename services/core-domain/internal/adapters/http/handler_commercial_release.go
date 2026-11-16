package http

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

func (h *Handler) PublishCommercialRelease(ctx context.Context, request generated.PublishCommercialReleaseRequestObject) (generated.PublishCommercialReleaseResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.PublishCommercialRelease401JSONResponse{UnauthenticatedJSONResponse: generated.UnauthenticatedJSONResponse(unauthorizedError())}, nil
	}
	release, err := h.commercialContent.PublishRelease(ctx, caller, string(request.ContentKind), request.ContentId.String(),
		request.Params.IdempotencyKey.String(), application.PublishReleaseRequest{
			ExpectedRevision: request.Body.ExpectedRevision, Reason: request.Body.Reason,
		})
	if err != nil {
		failure, known := classifyCommercialError(err)
		if !known {
			return nil, err
		}
		switch failure.status {
		case http.StatusBadRequest:
			return generated.PublishCommercialRelease400JSONResponse{InvalidRequestJSONResponse: generated.InvalidRequestJSONResponse(failure.validation)}, nil
		case http.StatusForbidden:
			return generated.PublishCommercialRelease403JSONResponse(forbiddenError("only the item's owner or an administrator publishes its release, and only while content versions are rolled out")), nil
		case http.StatusNotFound:
			return generated.PublishCommercialRelease404JSONResponse{NotFoundJSONResponse: generated.NotFoundJSONResponse(notFoundError("commercial content not found"))}, nil
		case http.StatusConflict:
			return generated.PublishCommercialRelease409JSONResponse{ConflictJSONResponse: generated.ConflictJSONResponse(conflictError(failure.message))}, nil
		default:
			return generated.PublishCommercialRelease503JSONResponse{UnavailableJSONResponse: unavailable(failure.message)}, nil
		}
	}
	response, err := releaseResponse(release)
	if err != nil {
		return nil, err
	}
	return generated.PublishCommercialRelease200JSONResponse(response), nil
}

func (h *Handler) GetCommercialRelease(ctx context.Context, request generated.GetCommercialReleaseRequestObject) (generated.GetCommercialReleaseResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.GetCommercialRelease401JSONResponse{UnauthenticatedJSONResponse: generated.UnauthenticatedJSONResponse(unauthorizedError())}, nil
	}
	release, err := h.commercialContent.GetRelease(ctx, caller, request.ReleaseId.String())
	if err != nil {
		failure, known := classifyCommercialError(err)
		if !known {
			return nil, err
		}
		switch failure.status {
		case http.StatusBadRequest:
			return generated.GetCommercialRelease400JSONResponse{InvalidRequestJSONResponse: generated.InvalidRequestJSONResponse(failure.validation)}, nil
		case http.StatusNotFound:
			return generated.GetCommercialRelease404JSONResponse{NotFoundJSONResponse: generated.NotFoundJSONResponse(notFoundError("commercial release not found"))}, nil
		case http.StatusServiceUnavailable:
			return generated.GetCommercialRelease503JSONResponse{UnavailableJSONResponse: unavailable("commercial releases are unavailable; try again")}, nil
		}
		return nil, err
	}
	response, err := releaseResponse(release)
	if err != nil {
		return nil, err
	}
	return generated.GetCommercialRelease200JSONResponse(response), nil
}

func releaseResponse(release domain.CommercialRelease) (generated.Release, error) {
	id, err := uuid.Parse(release.ReleaseID)
	if err != nil {
		return generated.Release{}, err
	}
	content, err := contentRefResponse(release.Content)
	if err != nil {
		return generated.Release{}, err
	}
	evidence, err := uuid.Parse(release.EvidenceEntryID)
	if err != nil {
		return generated.Release{}, err
	}
	ownership, err := ownershipResponse(release.Ownership)
	if err != nil {
		return generated.Release{}, err
	}
	dependencies := make([]generated.Dependency, 0, len(release.Dependencies))
	dependencyIDs := make([]uuid.UUID, 0, len(release.Dependencies))
	for _, d := range release.Dependencies {
		dependency, err := dependencyResponse(d)
		if err != nil {
			return generated.Release{}, err
		}
		dependencies = append(dependencies, dependency)
		dependencyIDs = append(dependencyIDs, dependency.ReleaseId)
	}
	return generated.Release{
		ReleaseId: id, Content: content, Revision: release.Revision, ManifestSha256: release.ManifestSHA256,
		ContentSha256: release.ContentSHA256, CoreVersionRef: release.CoreVersionRef, CreatedAt: release.CreatedAt,
		EvidenceEntryId: evidence, Ownership: ownership, Dependencies: dependencies, DependencyReleaseIds: dependencyIDs,
	}, nil
}

func dependencyResponse(d domain.ReleaseDependency) (generated.Dependency, error) {
	content, err := contentRefResponse(d.Content)
	if err != nil {
		return generated.Dependency{}, err
	}
	releaseID, err := uuid.Parse(d.ReleaseID)
	if err != nil {
		return generated.Dependency{}, err
	}
	evidence, err := uuid.Parse(d.EvidenceEntryID)
	if err != nil {
		return generated.Dependency{}, err
	}
	return generated.Dependency{
		Content: content, ContentSha256: d.ContentSHA256, ReleaseId: releaseID,
		Permission: generated.DependencyPermission(d.Permission), EvidenceEntryId: evidence,
	}, nil
}
