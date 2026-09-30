package http

import (
	"context"
	"errors"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func (h *Handler) ListCatalogPaths(ctx context.Context, request generated.ListCatalogPathsRequestObject) (generated.ListCatalogPathsResponseObject, error) {
	if _, ok := h.resolveCaller(ctx); !ok {
		return generated.ListCatalogPaths401JSONResponse(unauthorizedError()), nil
	}

	page, err := domain.NewPageRequest(request.Params.Limit, request.Params.Offset)
	if err != nil {
		return listValidationFailure[generated.ListCatalogPathsResponseObject](err, func(e generated.ValidationError) generated.ListCatalogPathsResponseObject {
			return generated.ListCatalogPaths400JSONResponse(e)
		})
	}

	result, err := h.pathCatalog.ListCatalogPaths(ctx, catalogPathListFilter(request.Params), page)
	if err != nil {
		return nil, err
	}
	names, err := h.loadUserNames(ctx, learningPathUserIDs(result.Items...))
	if err != nil {
		return nil, err
	}
	entries := make([]generated.PathCatalogEntry, len(result.Items))
	for i, p := range result.Items {
		entries[i] = toPathCatalogEntry(p, names)
	}
	return generated.ListCatalogPaths200JSONResponse{
		Items: entries, Total: result.Total, Limit: page.Limit, Offset: page.Offset,
	}, nil
}

func (h *Handler) GetCatalogPath(ctx context.Context, request generated.GetCatalogPathRequestObject) (generated.GetCatalogPathResponseObject, error) {
	if _, ok := h.resolveCaller(ctx); !ok {
		return generated.GetCatalogPath401JSONResponse(unauthorizedError()), nil
	}

	path, err := h.pathCatalog.GetCatalogPath(ctx, request.LearningPathId.String())
	if errors.Is(err, domain.ErrNotFound) {
		return generated.GetCatalogPath404JSONResponse(notFoundError("no published learning path exists with the given id")), nil
	}
	if err != nil {
		return nil, err
	}
	names, err := h.loadUserNames(ctx, learningPathUserIDs(path))
	if err != nil {
		return nil, err
	}
	return generated.GetCatalogPath200JSONResponse(toPathDetail(path, names)), nil
}

func (h *Handler) ListCatalogPathCreators(ctx context.Context, request generated.ListCatalogPathCreatorsRequestObject) (generated.ListCatalogPathCreatorsResponseObject, error) {
	if _, ok := h.resolveCaller(ctx); !ok {
		return generated.ListCatalogPathCreators401JSONResponse(unauthorizedError()), nil
	}

	creators, err := h.pathCatalog.ListCatalogPathCreators(ctx, searchQuery(request.Params.Q))
	if err != nil {
		return nil, err
	}
	return generated.ListCatalogPathCreators200JSONResponse(toUserRefs(creators)), nil
}

// catalogPathListFilter maps GET /catalog/paths' query parameters onto the
// domain filter; the catalog itself pins the status to published.
func catalogPathListFilter(params generated.ListCatalogPathsParams) domain.LearningPathFilter {
	filter := domain.LearningPathFilter{Query: searchQuery(params.Q), InstrumentID: uuidPtrToString(params.InstrumentId)}
	if params.CreatedBy != nil {
		filter.CreatedBy = params.CreatedBy.String()
	}
	if params.Levels != nil {
		for _, l := range *params.Levels {
			filter.Levels = append(filter.Levels, domain.DifficultyLevel(l))
		}
	}
	if params.SkillIds != nil {
		filter.SkillIDs = uuidStrings(*params.SkillIds)
	}
	if params.ConceptIds != nil {
		filter.ConceptIDs = uuidStrings(*params.ConceptIds)
	}
	if params.Language != nil {
		filter.Language = *params.Language
	}
	return filter
}

// toPathCatalogEntry renders a published path as a catalog card. A published
// path always has a summary, language and level; the empty fallbacks only
// guard a path read mid-change.
func toPathCatalogEntry(p domain.LearningPath, names userNames) generated.PathCatalogEntry {
	return generated.PathCatalogEntry{
		LearningPathId: mustUUID(p.ID),
		Title:          p.Title,
		Summary:        deref(p.Summary),
		Level:          generated.PathCatalogEntryLevel(derefLevel(p.Level)),
		Language:       deref(p.Language),
		CreatedBy:      names.ref(p.TeacherID),
		InstrumentIds:  toUUIDs(p.InstrumentIDs),
		ThumbnailUrl:   p.ThumbnailURL,
		LessonCount:    len(p.Items),
	}
}

// toPathDetail is a published path's catalog card plus its title-only
// outline, never lesson content or content node ids.
func toPathDetail(p domain.LearningPath, names userNames) generated.PathDetail {
	entry := toPathCatalogEntry(p, names)
	items := make([]generated.CourseOutlineItem, len(p.Items))
	for i, item := range p.Items {
		items[i] = generated.CourseOutlineItem{Title: item.Title, SectionLabel: item.SectionLabel}
	}
	return generated.PathDetail{
		LearningPathId: entry.LearningPathId,
		Title:          entry.Title,
		Summary:        entry.Summary,
		Level:          generated.PathDetailLevel(entry.Level),
		Language:       entry.Language,
		CreatedBy:      entry.CreatedBy,
		InstrumentIds:  entry.InstrumentIds,
		ThumbnailUrl:   entry.ThumbnailUrl,
		LessonCount:    entry.LessonCount,
		Items:          items,
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefLevel(l *domain.DifficultyLevel) domain.DifficultyLevel {
	if l == nil {
		return ""
	}
	return *l
}
