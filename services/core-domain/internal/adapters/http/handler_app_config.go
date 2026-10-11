package http

import (
	"context"
	"errors"
	"fmt"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

func (h *Handler) ListAppConfig(ctx context.Context, _ generated.ListAppConfigRequestObject) (generated.ListAppConfigResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.ListAppConfig401JSONResponse(unauthorizedError()), nil
	}
	settings, err := h.appConfig.List(ctx, caller)
	if errors.Is(err, domain.ErrForbidden) {
		return generated.ListAppConfig403JSONResponse(forbiddenError("only admins may read settings")), nil
	}
	if err != nil {
		return nil, err
	}
	names, err := h.loadUserNames(ctx, settingUserIDs(settings...))
	if err != nil {
		return nil, err
	}
	summaries := make([]generated.AppConfigSummary, len(settings))
	for i, s := range settings {
		summaries[i] = generated.AppConfigSummary{Key: s.Key, IsSet: s.Entry != nil}
		if s.Entry != nil {
			ref := names.ref(s.Entry.UpdatedBy)
			summaries[i].UpdatedAt, summaries[i].UpdatedBy = &s.Entry.UpdatedAt, &ref
		}
	}
	return generated.ListAppConfig200JSONResponse(summaries), nil
}

func (h *Handler) GetAppConfig(ctx context.Context, request generated.GetAppConfigRequestObject) (generated.GetAppConfigResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.GetAppConfig401JSONResponse(unauthorizedError()), nil
	}
	setting, err := h.appConfig.Get(ctx, caller, request.Key)
	switch {
	case errors.Is(err, domain.ErrForbidden):
		return generated.GetAppConfig403JSONResponse(forbiddenError("only admins may read settings")), nil
	case errors.Is(err, domain.ErrNotFound):
		return generated.GetAppConfig404JSONResponse(notFoundError("no setting has this key")), nil
	case err != nil:
		return nil, err
	}
	entry, err := h.toAppConfigEntry(ctx, setting)
	if err != nil {
		return nil, err
	}
	return generated.GetAppConfig200JSONResponse(entry), nil
}

func (h *Handler) SetAppConfig(ctx context.Context, request generated.SetAppConfigRequestObject) (generated.SetAppConfigResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.SetAppConfig401JSONResponse(unauthorizedError()), nil
	}
	if request.Body == nil {
		return generated.SetAppConfig400JSONResponse(validationErrorResponse(domain.NewValidationError("value", "is required"))), nil
	}
	raw, err := request.Body.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("re-encoding the setting's value: %w", err)
	}
	setting, err := h.appConfig.Set(ctx, caller, request.Key, raw)
	var verr *domain.ValidationError
	switch {
	case errors.Is(err, domain.ErrForbidden):
		return generated.SetAppConfig403JSONResponse(forbiddenError("only admins may change settings")), nil
	case errors.Is(err, domain.ErrNotFound):
		return generated.SetAppConfig404JSONResponse(notFoundError("no setting has this key")), nil
	case errors.As(err, &verr):
		return generated.SetAppConfig400JSONResponse(validationErrorResponse(verr)), nil
	case err != nil:
		return nil, err
	}
	entry, err := h.toAppConfigEntry(ctx, setting)
	if err != nil {
		return nil, err
	}
	return generated.SetAppConfig200JSONResponse(entry), nil
}

func (h *Handler) toAppConfigEntry(ctx context.Context, setting application.AppConfigSetting) (generated.AppConfigEntry, error) {
	entry := generated.AppConfigEntry{Key: setting.Key}
	if setting.Entry == nil {
		return entry, nil
	}
	names, err := h.loadUserNames(ctx, settingUserIDs(setting))
	if err != nil {
		return generated.AppConfigEntry{}, err
	}
	var value generated.AppConfigValue
	if err := value.UnmarshalJSON(setting.Entry.Value); err != nil {
		return generated.AppConfigEntry{}, fmt.Errorf("stored setting %q is not JSON: %w", setting.Key, err)
	}
	ref := names.ref(setting.Entry.UpdatedBy)
	entry.Value, entry.UpdatedAt, entry.UpdatedBy = &value, &setting.Entry.UpdatedAt, &ref
	return entry, nil
}

func settingUserIDs(settings ...application.AppConfigSetting) []string {
	ids := make([]string, 0, len(settings))
	for _, s := range settings {
		if s.Entry != nil {
			ids = append(ids, s.Entry.UpdatedBy)
		}
	}
	return ids
}

func (h *Handler) ListStartingPoints(ctx context.Context, request generated.ListStartingPointsRequestObject) (generated.ListStartingPointsResponseObject, error) {
	if _, ok := h.resolveCaller(ctx); !ok {
		return generated.ListStartingPoints401JSONResponse(unauthorizedError()), nil
	}
	params := request.Params
	result, err := h.startingPoints.List(ctx, params.InstrumentId.String(), domain.StartingPointExperience(params.Experience), params.Language)
	var verr *domain.ValidationError
	if errors.As(err, &verr) {
		return generated.ListStartingPoints400JSONResponse(validationErrorResponse(verr)), nil
	}
	if err != nil {
		return nil, err
	}

	var paths []domain.LearningPath
	var courses []domain.Course
	for _, item := range result.Items {
		if item.Path != nil {
			paths = append(paths, *item.Path)
		}
		if item.Course != nil {
			courses = append(courses, *item.Course)
		}
	}
	pathNames, err := h.loadUserNames(ctx, learningPathUserIDs(paths...))
	if err != nil {
		return nil, err
	}
	courseEntries, err := h.catalogEntries(ctx, courses, learnerCatalogView)
	if err != nil {
		return nil, err
	}

	items := make([]generated.StartingPoint, len(result.Items))
	nextCourse := 0
	for i, item := range result.Items {
		switch item.Kind {
		case domain.StartingPointKindPath:
			entry := toPathCatalogEntry(*item.Path, pathNames)
			items[i] = generated.StartingPoint{Kind: generated.StartingPointKindPath, Path: &entry}
		case domain.StartingPointKindCourse:
			entry := courseEntries[nextCourse]
			nextCourse++
			items[i] = generated.StartingPoint{Kind: generated.StartingPointKindCourse, Course: &entry}
		}
	}
	return generated.ListStartingPoints200JSONResponse{Source: generated.StartingPointsSource(result.Source), Items: items}, nil
}
