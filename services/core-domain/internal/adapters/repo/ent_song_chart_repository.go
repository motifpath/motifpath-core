package repo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/schema"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/songchart"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/songchartrevision"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntSongChartRepository persists song charts and their revisions via
// ent/Postgres. Documents are stored as the editor's ProseMirror JSON.
type EntSongChartRepository struct {
	client *ent.Client
}

func NewEntSongChartRepository(client *ent.Client) *EntSongChartRepository {
	return &EntSongChartRepository{client: client}
}

func (r *EntSongChartRepository) Create(ctx context.Context, chart domain.SongChart) error {
	ids, err := parseSongChartIDs(chart)
	if err != nil {
		return err
	}
	body, err := json.Marshal(chart.Draft.Body)
	if err != nil {
		return err
	}
	create := r.client.SongChart.Create().
		SetID(ids.chart).
		SetStatus(songchart.Status(chart.Status)).
		SetCreatedBy(ids.createdBy).
		SetCreatedAt(chart.CreatedAt).
		SetTitle(chart.Draft.Title).
		SetArtist(chart.Draft.Artist).
		SetLanguage(chart.Draft.Language).
		SetNillableConcertKey(chart.Draft.ConcertKey).
		SetCapoFret(chart.Draft.CapoFret).
		SetNillableTempoBpm(chart.Draft.TempoBPM).
		SetBody(string(body)).
		SetWarnings(toSchemaWarnings(chart.Draft.Warnings)).
		SetUpdatedBy(ids.updatedBy).
		SetUpdatedAt(chart.Draft.UpdatedAt).
		SetNillableRightsConfirmedBy(ids.rightsConfirmedBy).
		SetNillableRightsConfirmedAt(rightsConfirmedAt(chart.Draft.RightsConfirmation))
	if ts := toSchemaTimeSignature(chart.Draft.TimeSignature); ts != nil {
		create.SetTimeSignature(ts)
	}
	return create.Exec(ctx)
}

func (r *EntSongChartRepository) GetByID(ctx context.Context, id string) (domain.SongChart, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.SongChart{}, domain.ErrNotFound
	}
	row, err := r.client.SongChart.Get(ctx, parsed)
	if ent.IsNotFound(err) {
		return domain.SongChart{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.SongChart{}, err
	}
	return toDomainSongChart(row)
}

// List matches Q in Go, over the rows the status filter leaves — the same
// case- and accent-blind matching as domain.ContainsLoosely — and then
// pages in SQL over the matching ids, as the diagram library does.
func (r *EntSongChartRepository) List(ctx context.Context, filter domain.SongChartFilter, page domain.PageRequest) (domain.Page[domain.SongChart], error) {
	query := r.client.SongChart.Query()
	if filter.Status != nil {
		query = query.Where(songchart.StatusEQ(songchart.Status(*filter.Status)))
	}
	if filter.Q != "" {
		candidates, err := query.Clone().
			Select(songchart.FieldID, songchart.FieldTitle, songchart.FieldArtist, songchart.FieldPublishedTitle, songchart.FieldPublishedArtist).
			All(ctx)
		if err != nil {
			return domain.Page[domain.SongChart]{}, err
		}
		matched := []uuid.UUID{}
		for _, c := range candidates {
			for _, text := range []string{c.Title, c.Artist, deref(c.PublishedTitle), deref(c.PublishedArtist)} {
				if text != "" && domain.ContainsLoosely(text, filter.Q) {
					matched = append(matched, c.ID)
					break
				}
			}
		}
		query = query.Where(songchart.IDIn(matched...))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return domain.Page[domain.SongChart]{}, err
	}
	rows, err := query.Order(ent.Desc(songchart.FieldUpdatedAt), ent.Asc(songchart.FieldID)).
		Limit(page.Limit).Offset(page.Offset).All(ctx)
	if err != nil {
		return domain.Page[domain.SongChart]{}, err
	}
	items := make([]domain.SongChart, len(rows))
	for i, row := range rows {
		if items[i], err = toDomainSongChart(row); err != nil {
			return domain.Page[domain.SongChart]{}, err
		}
	}
	return domain.Page[domain.SongChart]{Items: items, Total: total}, nil
}

func (r *EntSongChartRepository) Save(ctx context.Context, chart domain.SongChart) error {
	return r.save(ctx, r.client, chart)
}

func (r *EntSongChartRepository) Publish(ctx context.Context, chart domain.SongChart, rev domain.SongChartRevision) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	if err := r.save(ctx, tx.Client(), chart); err != nil {
		return rollback(tx, err)
	}
	if err := createRevision(ctx, tx.Client(), rev); err != nil {
		return rollback(tx, err)
	}
	return tx.Commit()
}

// save replaces everything about a chart but its id and creation.
func (r *EntSongChartRepository) save(ctx context.Context, client *ent.Client, chart domain.SongChart) error {
	ids, err := parseSongChartIDs(chart)
	if err != nil {
		return err
	}
	body, err := json.Marshal(chart.Draft.Body)
	if err != nil {
		return err
	}
	update := client.SongChart.UpdateOneID(ids.chart).
		SetStatus(songchart.Status(chart.Status)).
		SetTitle(chart.Draft.Title).
		SetArtist(chart.Draft.Artist).
		SetLanguage(chart.Draft.Language).
		SetCapoFret(chart.Draft.CapoFret).
		SetBody(string(body)).
		SetWarnings(toSchemaWarnings(chart.Draft.Warnings)).
		SetUpdatedBy(ids.updatedBy).
		SetUpdatedAt(chart.Draft.UpdatedAt)
	setDraftOptionals(update, chart.Draft)
	if c := chart.Draft.RightsConfirmation; c != nil {
		update.SetRightsConfirmedBy(*ids.rightsConfirmedBy).SetRightsConfirmedAt(c.ConfirmedAt)
	} else {
		update.ClearRightsConfirmedBy().ClearRightsConfirmedAt()
	}
	if err := setPublishedSummary(update, chart.PublishedRevision); err != nil {
		return err
	}
	if err := setWithdrawal(update, chart.Withdrawal); err != nil {
		return err
	}
	err = update.Exec(ctx)
	if ent.IsNotFound(err) {
		return domain.ErrNotFound
	}
	return err
}

// setDraftOptionals sets the draft's optional fields, clearing those it
// leaves out.
func setDraftOptionals(update *ent.SongChartUpdateOne, d domain.SongChartDraft) {
	if d.ConcertKey != nil {
		update.SetConcertKey(*d.ConcertKey)
	} else {
		update.ClearConcertKey()
	}
	if d.TempoBPM != nil {
		update.SetTempoBpm(*d.TempoBPM)
	} else {
		update.ClearTempoBpm()
	}
	if ts := toSchemaTimeSignature(d.TimeSignature); ts != nil {
		update.SetTimeSignature(ts)
	} else {
		update.ClearTimeSignature()
	}
}

func setPublishedSummary(update *ent.SongChartUpdateOne, summary *domain.SongChartRevisionSummary) error {
	if summary == nil {
		update.ClearPublishedRevisionNumber().ClearPublishedTitle().ClearPublishedArtist().ClearPublishedLanguage().
			ClearPublishedConcertKey().ClearPublishedBy().ClearPublishedAt()
		return nil
	}
	by, err := uuid.Parse(summary.PublishedBy)
	if err != nil {
		return err
	}
	update.SetPublishedRevisionNumber(summary.Number).SetPublishedTitle(summary.Title).SetPublishedArtist(summary.Artist).
		SetPublishedLanguage(summary.Language).SetNillablePublishedConcertKey(summary.ConcertKey).
		SetPublishedBy(by).SetPublishedAt(summary.PublishedAt)
	if summary.ConcertKey == nil {
		update.ClearPublishedConcertKey()
	}
	return nil
}

func setWithdrawal(update *ent.SongChartUpdateOne, w *domain.SongChartWithdrawal) error {
	if w == nil {
		update.ClearWithdrawnBy().ClearWithdrawnAt().ClearWithdrawalReason()
		return nil
	}
	by, err := uuid.Parse(w.WithdrawnBy)
	if err != nil {
		return err
	}
	update.SetWithdrawnBy(by).SetWithdrawnAt(w.WithdrawnAt).SetWithdrawalReason(w.Reason)
	return nil
}

func createRevision(ctx context.Context, client *ent.Client, rev domain.SongChartRevision) error {
	chartID, err := uuid.Parse(rev.SongChartID)
	if err != nil {
		return err
	}
	confirmedBy, err := uuid.Parse(rev.RightsConfirmation.ConfirmedBy)
	if err != nil {
		return err
	}
	publishedBy, err := uuid.Parse(rev.PublishedBy)
	if err != nil {
		return err
	}
	body, err := json.Marshal(rev.Body)
	if err != nil {
		return err
	}
	create := client.SongChartRevision.Create().
		SetSongChartID(chartID).
		SetRevisionNumber(rev.Number).
		SetTitle(rev.Title).
		SetArtist(rev.Artist).
		SetLanguage(rev.Language).
		SetNillableConcertKey(rev.ConcertKey).
		SetCapoFret(rev.CapoFret).
		SetNillableTempoBpm(rev.TempoBPM).
		SetTuningFingerprint(rev.TuningFingerprint).
		SetBody(string(body)).
		SetRightsConfirmedBy(confirmedBy).
		SetRightsConfirmedAt(rev.RightsConfirmation.ConfirmedAt).
		SetPublishedBy(publishedBy).
		SetPublishedAt(rev.PublishedAt)
	if ts := toSchemaTimeSignature(rev.TimeSignature); ts != nil {
		create.SetTimeSignature(ts)
	}
	return create.Exec(ctx)
}

func (r *EntSongChartRepository) ListRevisions(ctx context.Context, chartID string) ([]domain.SongChartRevision, error) {
	parsed, err := uuid.Parse(chartID)
	if err != nil {
		return nil, nil
	}
	rows, err := r.client.SongChartRevision.Query().
		Where(songchartrevision.SongChartID(parsed)).
		Order(ent.Desc(songchartrevision.FieldRevisionNumber)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	revs := make([]domain.SongChartRevision, len(rows))
	for i, row := range rows {
		if revs[i], err = toDomainSongChartRevision(row); err != nil {
			return nil, err
		}
	}
	return revs, nil
}

func (r *EntSongChartRepository) GetRevision(ctx context.Context, chartID string, number int) (domain.SongChartRevision, error) {
	parsed, err := uuid.Parse(chartID)
	if err != nil {
		return domain.SongChartRevision{}, domain.ErrNotFound
	}
	row, err := r.client.SongChartRevision.Query().
		Where(songchartrevision.SongChartID(parsed), songchartrevision.RevisionNumber(number)).
		Only(ctx)
	if ent.IsNotFound(err) {
		return domain.SongChartRevision{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.SongChartRevision{}, err
	}
	return toDomainSongChartRevision(row)
}

// songChartIDs are a chart's ids, parsed for storage.
type songChartIDs struct {
	chart, createdBy, updatedBy uuid.UUID
	rightsConfirmedBy           *uuid.UUID
}

func parseSongChartIDs(chart domain.SongChart) (songChartIDs, error) {
	var ids songChartIDs
	var err error
	if ids.chart, err = uuid.Parse(chart.ID); err != nil {
		return ids, err
	}
	if ids.createdBy, err = uuid.Parse(chart.CreatedBy); err != nil {
		return ids, err
	}
	if ids.updatedBy, err = uuid.Parse(chart.Draft.UpdatedBy); err != nil {
		return ids, err
	}
	if c := chart.Draft.RightsConfirmation; c != nil {
		by, err := uuid.Parse(c.ConfirmedBy)
		if err != nil {
			return ids, err
		}
		ids.rightsConfirmedBy = &by
	}
	return ids, nil
}

func rightsConfirmedAt(c *domain.RightsConfirmation) *time.Time {
	if c == nil {
		return nil
	}
	return &c.ConfirmedAt
}

func toSchemaTimeSignature(ts *domain.TimeSignature) *schema.SongChartTimeSignature {
	if ts == nil {
		return nil
	}
	return &schema.SongChartTimeSignature{Beats: ts.Beats, BeatValue: ts.BeatValue}
}

func toDomainTimeSignature(ts *schema.SongChartTimeSignature) *domain.TimeSignature {
	if ts == nil {
		return nil
	}
	return &domain.TimeSignature{Beats: ts.Beats, BeatValue: ts.BeatValue}
}

func toSchemaWarnings(warnings []domain.AnchorWarning) []schema.SongChartWarning {
	out := make([]schema.SongChartWarning, len(warnings))
	for i, w := range warnings {
		out[i] = schema.SongChartWarning{
			AnchorID: w.AnchorID, SectionIndex: w.Position.SectionIndex, LineIndex: w.Position.LineIndex,
			WrittenSymbol: w.WrittenSymbol, Kind: string(w.Kind),
		}
	}
	return out
}

func toDomainWarnings(warnings []schema.SongChartWarning) []domain.AnchorWarning {
	if len(warnings) == 0 {
		return nil
	}
	out := make([]domain.AnchorWarning, len(warnings))
	for i, w := range warnings {
		out[i] = domain.AnchorWarning{
			AnchorID: w.AnchorID, Position: domain.AnchorPosition{SectionIndex: w.SectionIndex, LineIndex: w.LineIndex},
			WrittenSymbol: w.WrittenSymbol, Kind: domain.AnchorWarningKind(w.Kind),
		}
	}
	return out
}

func toDomainSongChart(row *ent.SongChart) (domain.SongChart, error) {
	body, err := domain.ParseSongChartDocument([]byte(row.Body))
	if err != nil {
		return domain.SongChart{}, err
	}
	chart := domain.SongChart{
		ID: row.ID.String(), Status: domain.SongChartStatus(row.Status),
		CreatedBy: row.CreatedBy.String(), CreatedAt: row.CreatedAt,
		Draft: domain.SongChartDraft{
			Title: row.Title, Artist: row.Artist, Language: row.Language, ConcertKey: row.ConcertKey,
			CapoFret: row.CapoFret, TempoBPM: row.TempoBpm, TimeSignature: toDomainTimeSignature(row.TimeSignature),
			Body: body, Warnings: toDomainWarnings(row.Warnings),
			UpdatedBy: row.UpdatedBy.String(), UpdatedAt: row.UpdatedAt,
		},
	}
	if row.RightsConfirmedBy != nil && row.RightsConfirmedAt != nil {
		chart.Draft.RightsConfirmation = &domain.RightsConfirmation{ConfirmedBy: row.RightsConfirmedBy.String(), ConfirmedAt: *row.RightsConfirmedAt}
	}
	if row.PublishedRevisionNumber != nil {
		chart.PublishedRevision = &domain.SongChartRevisionSummary{
			Number: *row.PublishedRevisionNumber, Title: deref(row.PublishedTitle), Artist: deref(row.PublishedArtist),
			Language: deref(row.PublishedLanguage), ConcertKey: row.PublishedConcertKey,
			PublishedBy: uuidString(row.PublishedBy), PublishedAt: derefTime(row.PublishedAt),
		}
	}
	if row.WithdrawnBy != nil {
		chart.Withdrawal = &domain.SongChartWithdrawal{
			WithdrawnBy: row.WithdrawnBy.String(), WithdrawnAt: derefTime(row.WithdrawnAt), Reason: deref(row.WithdrawalReason),
		}
	}
	return chart, nil
}

func toDomainSongChartRevision(row *ent.SongChartRevision) (domain.SongChartRevision, error) {
	body, err := domain.ParseSongChartDocument([]byte(row.Body))
	if err != nil {
		return domain.SongChartRevision{}, err
	}
	return domain.SongChartRevision{
		SongChartID: row.SongChartID.String(), Number: row.RevisionNumber,
		Title: row.Title, Artist: row.Artist, Language: row.Language, ConcertKey: row.ConcertKey,
		CapoFret: row.CapoFret, TempoBPM: row.TempoBpm, TimeSignature: toDomainTimeSignature(row.TimeSignature),
		TuningFingerprint: row.TuningFingerprint, Body: body,
		RightsConfirmation: domain.RightsConfirmation{ConfirmedBy: row.RightsConfirmedBy.String(), ConfirmedAt: row.RightsConfirmedAt},
		PublishedBy:        row.PublishedBy.String(), PublishedAt: row.PublishedAt,
	}, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func uuidString(u *uuid.UUID) string {
	if u == nil {
		return ""
	}
	return u.String()
}
