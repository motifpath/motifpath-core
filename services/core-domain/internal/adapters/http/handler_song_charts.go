package http

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

const onlyAdminsAuthorSongCharts = "only admins author song charts"

func (h *Handler) CreateSongChart(ctx context.Context, request generated.CreateSongChartRequestObject) (generated.CreateSongChartResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.CreateSongChart401JSONResponse(unauthorizedError()), nil
	}
	input, err := toSongChartInput(*request.Body)
	if err == nil {
		var chart domain.SongChart
		chart, err = h.songChart.Create(ctx, caller, input)
		if err == nil {
			out, mapErr := h.toGeneratedSongChart(ctx, chart)
			return generated.CreateSongChart201JSONResponse(out), mapErr
		}
	}
	kind, valErr := classify(err)
	switch kind {
	case errKindValidation:
		return generated.CreateSongChart400JSONResponse(validationErrorResponse(valErr)), nil
	case errKindForbidden:
		return generated.CreateSongChart403JSONResponse(forbiddenError(onlyAdminsAuthorSongCharts)), nil
	case errKindNotFound, errKindOther:
	}
	return nil, err
}

func (h *Handler) ListSongCharts(ctx context.Context, request generated.ListSongChartsRequestObject) (generated.ListSongChartsResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.ListSongCharts401JSONResponse(unauthorizedError()), nil
	}
	params := request.Params
	page, err := domain.NewPageRequest(params.Limit, params.Offset)
	if err != nil {
		return listValidationFailure(err, func(v generated.ValidationError) generated.ListSongChartsResponseObject {
			return generated.ListSongCharts400JSONResponse(v)
		})
	}
	filter := domain.SongChartFilter{Q: searchQuery(params.Q)}
	if params.Status != nil {
		status := domain.SongChartStatus(*params.Status)
		filter.Status = &status
	}
	result, err := h.songChart.List(ctx, caller, filter, page)
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.ListSongCharts400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindForbidden:
			return generated.ListSongCharts403JSONResponse(forbiddenError(onlyAdminsAuthorSongCharts)), nil
		case errKindNotFound, errKindOther:
		}
		return nil, err
	}
	ids := make([]string, 0, len(result.Items))
	for _, c := range result.Items {
		ids = append(ids, c.Draft.UpdatedBy)
	}
	names, err := h.loadUserNames(ctx, ids)
	if err != nil {
		return nil, err
	}
	items := make([]generated.SongChartSummary, len(result.Items))
	for i, c := range result.Items {
		items[i] = toGeneratedSongChartSummary(c, names)
	}
	return generated.ListSongCharts200JSONResponse{Items: items, Total: result.Total, Limit: page.Limit, Offset: page.Offset}, nil
}

func (h *Handler) GetSongChart(ctx context.Context, request generated.GetSongChartRequestObject) (generated.GetSongChartResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.GetSongChart401JSONResponse(unauthorizedError()), nil
	}
	chart, err := h.songChart.Get(ctx, caller, request.SongChartId.String())
	if err != nil {
		kind, _ := classify(err)
		switch kind {
		case errKindForbidden:
			return generated.GetSongChart403JSONResponse(forbiddenError(onlyAdminsAuthorSongCharts)), nil
		case errKindNotFound:
			return generated.GetSongChart404JSONResponse(notFoundError("no song chart exists with the given song_chart_id")), nil
		case errKindValidation, errKindOther:
		}
		return nil, err
	}
	out, err := h.toGeneratedSongChart(ctx, chart)
	return generated.GetSongChart200JSONResponse(out), err
}

func (h *Handler) UpdateSongChartDraft(ctx context.Context, request generated.UpdateSongChartDraftRequestObject) (generated.UpdateSongChartDraftResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.UpdateSongChartDraft401JSONResponse(unauthorizedError()), nil
	}
	input, err := toSongChartInput(*request.Body)
	if err == nil {
		var chart domain.SongChart
		chart, err = h.songChart.UpdateDraft(ctx, caller, request.SongChartId.String(), input)
		if err == nil {
			out, mapErr := h.toGeneratedSongChart(ctx, chart)
			return generated.UpdateSongChartDraft200JSONResponse(out), mapErr
		}
	}
	kind, valErr := classify(err)
	switch kind {
	case errKindValidation:
		return generated.UpdateSongChartDraft400JSONResponse(validationErrorResponse(valErr)), nil
	case errKindForbidden:
		return generated.UpdateSongChartDraft403JSONResponse(forbiddenError(onlyAdminsAuthorSongCharts)), nil
	case errKindNotFound:
		return generated.UpdateSongChartDraft404JSONResponse(notFoundError("no song chart exists with the given song_chart_id")), nil
	case errKindOther:
	}
	return nil, err
}

func (h *Handler) PublishSongChart(ctx context.Context, request generated.PublishSongChartRequestObject) (generated.PublishSongChartResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.PublishSongChart401JSONResponse(unauthorizedError()), nil
	}
	rev, err := h.songChart.Publish(ctx, caller, request.SongChartId.String())
	var notPublishable *domain.SongChartNotPublishableError
	if errors.As(err, &notPublishable) {
		return generated.PublishSongChart409JSONResponse(toGeneratedNotPublishable(notPublishable)), nil
	}
	if err != nil {
		kind, _ := classify(err)
		switch kind {
		case errKindForbidden:
			return generated.PublishSongChart403JSONResponse(forbiddenError(onlyAdminsAuthorSongCharts)), nil
		case errKindNotFound:
			return generated.PublishSongChart404JSONResponse(notFoundError("no song chart exists with the given song_chart_id")), nil
		case errKindValidation, errKindOther:
		}
		return nil, err
	}
	names, err := h.loadUserNames(ctx, []string{rev.PublishedBy, rev.RightsConfirmation.ConfirmedBy})
	if err != nil {
		return nil, err
	}
	out, err := toGeneratedSongChartRevision(rev, names)
	return generated.PublishSongChart201JSONResponse(out), err
}

func (h *Handler) WithdrawSongChart(ctx context.Context, request generated.WithdrawSongChartRequestObject) (generated.WithdrawSongChartResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.WithdrawSongChart401JSONResponse(unauthorizedError()), nil
	}
	chart, err := h.songChart.Withdraw(ctx, caller, request.SongChartId.String(), request.Body.Reason)
	if errors.Is(err, domain.ErrConflict) {
		return generated.WithdrawSongChart409JSONResponse(conflictError(conflictMessage(err))), nil
	}
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.WithdrawSongChart400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindForbidden:
			return generated.WithdrawSongChart403JSONResponse(forbiddenError(onlyAdminsAuthorSongCharts)), nil
		case errKindNotFound:
			return generated.WithdrawSongChart404JSONResponse(notFoundError("no song chart exists with the given song_chart_id")), nil
		case errKindOther:
		}
		return nil, err
	}
	out, err := h.toGeneratedSongChart(ctx, chart)
	return generated.WithdrawSongChart200JSONResponse(out), err
}

func (h *Handler) ListSongChartRevisions(ctx context.Context, request generated.ListSongChartRevisionsRequestObject) (generated.ListSongChartRevisionsResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.ListSongChartRevisions401JSONResponse(unauthorizedError()), nil
	}
	revs, err := h.songChart.ListRevisions(ctx, caller, request.SongChartId.String())
	if err != nil {
		kind, _ := classify(err)
		switch kind {
		case errKindForbidden:
			return generated.ListSongChartRevisions403JSONResponse(forbiddenError(onlyAdminsAuthorSongCharts)), nil
		case errKindNotFound:
			return generated.ListSongChartRevisions404JSONResponse(notFoundError("no song chart exists with the given song_chart_id")), nil
		case errKindValidation, errKindOther:
		}
		return nil, err
	}
	ids := make([]string, 0, 2*len(revs))
	for _, r := range revs {
		ids = append(ids, r.PublishedBy, r.RightsConfirmation.ConfirmedBy)
	}
	names, err := h.loadUserNames(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(generated.ListSongChartRevisions200JSONResponse, len(revs))
	for i, r := range revs {
		if out[i], err = toGeneratedSongChartRevision(r, names); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (h *Handler) PreviewSongChart(ctx context.Context, request generated.PreviewSongChartRequestObject) (generated.PreviewSongChartResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.PreviewSongChart401JSONResponse(unauthorizedError()), nil
	}
	chart, err := h.songChart.Preview(ctx, caller, request.SongChartId.String())
	if err != nil {
		kind, _ := classify(err)
		switch kind {
		case errKindForbidden:
			return generated.PreviewSongChart403JSONResponse(forbiddenError(onlyAdminsAuthorSongCharts)), nil
		case errKindNotFound:
			return generated.PreviewSongChart404JSONResponse(notFoundError("no song chart exists with the given song_chart_id")), nil
		case errKindValidation, errKindOther:
		}
		return nil, err
	}
	out, err := h.toGeneratedLearnerSongChart(ctx, chart)
	return generated.PreviewSongChart200JSONResponse(out), err
}

func (h *Handler) GetPublishedSongChart(ctx context.Context, request generated.GetPublishedSongChartRequestObject) (generated.GetPublishedSongChartResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.GetPublishedSongChart401JSONResponse(unauthorizedError()), nil
	}
	chart, err := h.songChart.GetPublished(ctx, caller, request.SongChartId.String())
	if errors.Is(err, domain.ErrNotFound) {
		return generated.GetPublishedSongChart404JSONResponse(notFoundError("no published song chart exists with the given song_chart_id")), nil
	}
	if err != nil {
		return nil, err
	}
	out, err := h.toGeneratedLearnerSongChart(ctx, chart)
	return generated.GetPublishedSongChart200JSONResponse(out), err
}

func (h *Handler) ExportSongChartChordPro(ctx context.Context, request generated.ExportSongChartChordProRequestObject) (generated.ExportSongChartChordProResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.ExportSongChartChordPro401JSONResponse(unauthorizedError()), nil
	}
	text, err := h.songChart.ExportChordPro(ctx, caller, request.SongChartId.String())
	if err != nil {
		kind, _ := classify(err)
		switch kind {
		case errKindForbidden:
			return generated.ExportSongChartChordPro403JSONResponse(forbiddenError(onlyAdminsAuthorSongCharts)), nil
		case errKindNotFound:
			return generated.ExportSongChartChordPro404JSONResponse(notFoundError("no song chart exists with the given song_chart_id")), nil
		case errKindValidation, errKindOther:
		}
		return nil, err
	}
	return generated.ExportSongChartChordPro200TextResponse(text), nil
}

func (h *Handler) ImportSongChartChordPro(ctx context.Context, request generated.ImportSongChartChordProRequestObject) (generated.ImportSongChartChordProResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.ImportSongChartChordPro401JSONResponse(unauthorizedError()), nil
	}
	var text string
	if request.Body != nil {
		text = *request.Body
	}
	result, err := h.songChart.ImportChordPro(ctx, caller, request.SongChartId.String(), text)
	if err == nil {
		out, mapErr := h.toGeneratedSongChart(ctx, result.Chart)
		return generated.ImportSongChartChordPro200JSONResponse{SongChart: out, ImportWarnings: toGeneratedChordProWarnings(result.Warnings)}, mapErr
	}
	kind, valErr := classify(err)
	switch kind {
	case errKindValidation:
		return generated.ImportSongChartChordPro400JSONResponse(validationErrorResponse(valErr)), nil
	case errKindForbidden:
		return generated.ImportSongChartChordPro403JSONResponse(forbiddenError(onlyAdminsAuthorSongCharts)), nil
	case errKindNotFound:
		return generated.ImportSongChartChordPro404JSONResponse(notFoundError("no song chart exists with the given song_chart_id")), nil
	case errKindOther:
	}
	return nil, err
}

// ── Mapping ──────────────────────────────────────────────────────────────────

// toSongChartInput reads a draft from the request. The body is read through
// its JSON, so it is checked against the chart's own document schema.
func toSongChartInput(in generated.SongChartDraftInput) (application.SongChartInput, error) {
	body, err := fromGeneratedSongChartDocument(in.Body)
	if err != nil {
		return application.SongChartInput{}, err
	}
	var ts *domain.TimeSignature
	if in.TimeSignature != nil {
		ts = &domain.TimeSignature{Beats: in.TimeSignature.Beats, BeatValue: int(in.TimeSignature.BeatValue)}
	}
	return application.SongChartInput{
		Title: in.Title, Artist: in.Artist, Language: in.Language, ConcertKey: in.ConcertKey,
		CapoFret: in.CapoFret, TempoBPM: in.TempoBpm, TimeSignature: ts,
		RightsConfirmed: in.RightsConfirmed, Body: body,
	}, nil
}

func fromGeneratedSongChartDocument(doc generated.SongChartDocument) (domain.SongChartDocument, error) {
	raw, err := json.Marshal(doc)
	if err != nil {
		return domain.SongChartDocument{}, err
	}
	return domain.ParseSongChartDocument(raw)
}

func toGeneratedSongChartDocument(doc domain.SongChartDocument) (generated.SongChartDocument, error) {
	raw, err := json.Marshal(doc)
	if err != nil {
		return generated.SongChartDocument{}, err
	}
	var out generated.SongChartDocument
	return out, json.Unmarshal(raw, &out)
}

func toGeneratedTimeSignature(ts *domain.TimeSignature) *generated.TimeSignature {
	if ts == nil {
		return nil
	}
	return &generated.TimeSignature{Beats: ts.Beats, BeatValue: generated.TimeSignatureBeatValue(ts.BeatValue)}
}

func songChartUserIDs(chart domain.SongChart) []string {
	ids := []string{chart.CreatedBy, chart.Draft.UpdatedBy}
	if c := chart.Draft.RightsConfirmation; c != nil {
		ids = append(ids, c.ConfirmedBy)
	}
	if r := chart.PublishedRevision; r != nil {
		ids = append(ids, r.PublishedBy)
	}
	if w := chart.Withdrawal; w != nil {
		ids = append(ids, w.WithdrawnBy)
	}
	return ids
}

func (h *Handler) toGeneratedSongChart(ctx context.Context, chart domain.SongChart) (generated.SongChart, error) {
	names, err := h.loadUserNames(ctx, songChartUserIDs(chart))
	if err != nil {
		return generated.SongChart{}, err
	}
	d := chart.Draft
	body, err := toGeneratedSongChartDocument(d.Body)
	if err != nil {
		return generated.SongChart{}, err
	}
	out := generated.SongChart{
		SongChartId: mustUUID(chart.ID),
		Status:      generated.SongChartStatus(chart.Status),
		CreatedBy:   names.ref(chart.CreatedBy),
		CreatedAt:   chart.CreatedAt,
		Draft: generated.SongChartDraft{
			Title: d.Title, Artist: d.Artist, Language: d.Language, ConcertKey: d.ConcertKey,
			CapoFret: d.CapoFret, TempoBpm: d.TempoBPM, TimeSignature: toGeneratedTimeSignature(d.TimeSignature),
			RightsConfirmed: d.RightsConfirmation != nil, Body: body,
			Warnings:  toGeneratedAnchorWarnings(d.Warnings),
			UpdatedBy: names.ref(d.UpdatedBy), UpdatedAt: d.UpdatedAt,
		},
	}
	if c := d.RightsConfirmation; c != nil {
		out.Draft.RightsConfirmation = &generated.RightsConfirmation{ConfirmedBy: names.ref(c.ConfirmedBy), ConfirmedAt: c.ConfirmedAt}
	}
	if r := chart.PublishedRevision; r != nil {
		out.PublishedRevision = &generated.SongChartRevisionSummary{
			RevisionNumber: r.Number, Title: r.Title, Language: r.Language, PublishedBy: names.ref(r.PublishedBy), PublishedAt: r.PublishedAt,
		}
	}
	if w := chart.Withdrawal; w != nil {
		out.Withdrawal = &generated.SongChartWithdrawal{WithdrawnBy: names.ref(w.WithdrawnBy), WithdrawnAt: w.WithdrawnAt, Reason: w.Reason}
	}
	return out, nil
}

func toGeneratedSongChartSummary(c domain.SongChart, names userNames) generated.SongChartSummary {
	out := generated.SongChartSummary{
		SongChartId: mustUUID(c.ID), Title: c.Draft.Title, Artist: c.Draft.Artist, Language: c.Draft.Language,
		Status: generated.SongChartStatus(c.Status), RightsConfirmed: c.Draft.RightsConfirmation != nil,
		UpdatedBy: names.ref(c.Draft.UpdatedBy), UpdatedAt: c.Draft.UpdatedAt,
	}
	if c.PublishedRevision != nil {
		n := c.PublishedRevision.Number
		out.PublishedRevisionNumber = &n
	}
	return out
}

func toGeneratedAnchorWarnings(warnings []domain.AnchorWarning) []generated.SongChartAnchorWarning {
	out := make([]generated.SongChartAnchorWarning, len(warnings))
	for i, w := range warnings {
		out[i] = generated.SongChartAnchorWarning{
			AnchorId: w.AnchorID, SectionIndex: w.Position.SectionIndex, LineIndex: w.Position.LineIndex,
			WrittenSymbol: w.WrittenSymbol, Warning: generated.SongChartAnchorWarningWarning(w.Kind),
			BlocksPublication: w.BlocksPublication(),
		}
	}
	return out
}

func toGeneratedNotPublishable(e *domain.SongChartNotPublishableError) generated.SongChartNotPublishableError {
	reasons := make([]generated.SongChartNotPublishableErrorReasons, len(e.Reasons))
	for i, r := range e.Reasons {
		reasons[i] = generated.SongChartNotPublishableErrorReasons(r)
	}
	return generated.SongChartNotPublishableError{
		Message: "the song chart can't be published as it stands", Reasons: reasons,
		AnchorWarnings: toGeneratedAnchorWarnings(e.AnchorWarnings),
	}
}

func toGeneratedSongChartRevision(r domain.SongChartRevision, names userNames) (generated.SongChartRevision, error) {
	body, err := toGeneratedSongChartDocument(r.Body)
	if err != nil {
		return generated.SongChartRevision{}, err
	}
	return generated.SongChartRevision{
		SongChartId: mustUUID(r.SongChartID), RevisionNumber: r.Number,
		Title: r.Title, Artist: r.Artist, Language: r.Language, ConcertKey: r.ConcertKey,
		CapoFret: r.CapoFret, TempoBpm: r.TempoBPM, TimeSignature: toGeneratedTimeSignature(r.TimeSignature),
		TuningFingerprint: r.TuningFingerprint, Body: body,
		RightsConfirmation: generated.RightsConfirmation{ConfirmedBy: names.ref(r.RightsConfirmation.ConfirmedBy), ConfirmedAt: r.RightsConfirmation.ConfirmedAt},
		PublishedBy:        names.ref(r.PublishedBy), PublishedAt: r.PublishedAt,
	}, nil
}

func (h *Handler) toGeneratedLearnerSongChart(ctx context.Context, c application.LearnerSongChart) (generated.LearnerSongChart, error) {
	body, err := toGeneratedSongChartDocument(c.Body)
	if err != nil {
		return generated.LearnerSongChart{}, err
	}
	names, err := h.loadUserNames(ctx, diagramUserIDs(c.Diagrams...))
	if err != nil {
		return generated.LearnerSongChart{}, err
	}
	chords := make([]generated.ChordDefinition, len(c.Chords))
	for i, chord := range c.Chords {
		chords[i] = toGeneratedChord(chord)
	}
	return generated.LearnerSongChart{
		SongChartId: mustUUID(c.SongChartID), RevisionNumber: c.RevisionNumber,
		Title: c.Title, Artist: c.Artist, Language: c.Language, ConcertKey: c.ConcertKey,
		CapoFret: c.CapoFret, TempoBpm: c.TempoBPM, TimeSignature: toGeneratedTimeSignature(c.TimeSignature),
		TuningFingerprint: c.TuningFingerprint, Body: body,
		Chords: chords, Diagrams: toGeneratedDiagrams(c.Diagrams, names),
	}, nil
}

// toGeneratedChordProWarnings is never nil, so an import that read
// everything reports an empty list.
func toGeneratedChordProWarnings(warnings []domain.ChordProWarning) []generated.ChordProImportWarning {
	out := make([]generated.ChordProImportWarning, len(warnings))
	for i, w := range warnings {
		out[i] = generated.ChordProImportWarning{Line: w.Line, Kind: generated.ChordProImportWarningKind(w.Kind), Text: w.Text}
	}
	return out
}
