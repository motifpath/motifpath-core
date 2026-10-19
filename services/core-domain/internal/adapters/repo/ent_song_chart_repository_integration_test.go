//go:build integration

package repo

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

// songChartDocument is "[G]Quando olhei a terra", with a comment, a label
// and a picked voicing, so every part of a document round-trips.
func songChartDocument(chordID string) domain.SongChartDocument {
	label, comment, voicing := "Verso 1", "Repeat twice", uuid.NewString()
	return domain.SongChartDocument{Sections: []domain.SongChartSection{{
		Kind: domain.SectionVerse, Label: &label,
		Lines: []domain.SongChartLine{
			{Runs: []domain.SongChartRun{
				{Text: "Quando olhei ", Anchor: &domain.ChordAnchor{ID: "a1", WrittenSymbol: "G", ChordDefinitionID: &chordID, ChordVoicingID: &voicing}},
				{Text: "a terra"},
			}},
			{Comment: &comment},
		},
	}}}
}

func newSongChart(title string, updatedAt time.Time) domain.SongChart {
	key, tempo := "G", 96
	return domain.SongChart{
		ID: uuid.NewString(), Status: domain.SongChartDraftStatus, CreatedBy: uuid.NewString(), CreatedAt: fixedAt,
		Draft: domain.SongChartDraft{
			Title: title, Artist: "Luiz Gonzaga", Language: "pt_BR", ConcertKey: &key, CapoFret: 2, TempoBPM: &tempo,
			TimeSignature: &domain.TimeSignature{Beats: 2, BeatValue: 4},
			Body:          songChartDocument(uuid.NewString()),
			Warnings:      []domain.AnchorWarning{{AnchorID: "a1", Position: domain.AnchorPosition{SectionIndex: 0, LineIndex: 0}, WrittenSymbol: "G", Kind: domain.AnchorBassNotInCatalog}},
			UpdatedBy:     uuid.NewString(), UpdatedAt: updatedAt,
		},
	}
}

func TestEntSongChartRepository(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	charts := NewEntSongChartRepository(client)

	t.Run("existing ids are those naming a chart, whatever its status", func(t *testing.T) {
		draft := newSongChart("Asa Branca", fixedAt)
		withdrawn := newSongChart("Amazing Grace", fixedAt)
		withdrawn.Status = domain.SongChartWithdrawn
		require.NoError(t, charts.Create(ctx, draft))
		require.NoError(t, charts.Create(ctx, withdrawn))

		got, err := charts.ExistingIDs(ctx, []string{draft.ID, withdrawn.ID, uuid.NewString(), "not-a-uuid"})

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{draft.ID, withdrawn.ID}, got)
	})

	t.Run("no ids name no chart", func(t *testing.T) {
		got, err := charts.ExistingIDs(ctx, nil)

		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("a chart round-trips with its whole draft", func(t *testing.T) {
		chart := newSongChart("Asa Branca", fixedAt)

		require.NoError(t, charts.Create(ctx, chart))
		got, err := charts.GetByID(ctx, chart.ID)

		require.NoError(t, err)
		assert.Equal(t, chart, got)
	})

	t.Run("a draft with only its required fields round-trips", func(t *testing.T) {
		chart := newSongChart("Minimal", fixedAt)
		chart.Draft.ConcertKey, chart.Draft.TempoBPM, chart.Draft.TimeSignature, chart.Draft.Warnings = nil, nil, nil, nil

		require.NoError(t, charts.Create(ctx, chart))
		got, err := charts.GetByID(ctx, chart.ID)

		require.NoError(t, err)
		assert.Equal(t, chart, got)
	})

	t.Run("an unknown chart is not found", func(t *testing.T) {
		for _, id := range []string{uuid.NewString(), "not-a-uuid"} {
			_, err := charts.GetByID(ctx, id)

			require.ErrorIs(t, err, domain.ErrNotFound)
		}
	})

	t.Run("save replaces the draft, rights confirmation and withdrawal", func(t *testing.T) {
		chart := newSongChart("Asa Branca", fixedAt)
		require.NoError(t, charts.Create(ctx, chart))
		chart.Draft.Title = "Asa Branca (Luiz Gonzaga)"
		chart.Draft.RightsConfirmation = &domain.RightsConfirmation{ConfirmedBy: uuid.NewString(), ConfirmedAt: fixedAt.Add(time.Hour)}
		chart.Draft.UpdatedAt = fixedAt.Add(time.Hour)
		chart.Status = domain.SongChartWithdrawn
		chart.Withdrawal = &domain.SongChartWithdrawal{WithdrawnBy: uuid.NewString(), WithdrawnAt: fixedAt.Add(2 * time.Hour), Reason: "Rights disputed"}

		require.NoError(t, charts.Save(ctx, chart))
		got, err := charts.GetByID(ctx, chart.ID)

		require.NoError(t, err)
		assert.Equal(t, chart, got)
	})

	t.Run("saving an unknown chart is not found", func(t *testing.T) {
		require.ErrorIs(t, charts.Save(ctx, newSongChart("Nope", fixedAt)), domain.ErrNotFound)
	})

	t.Run("publishing stores the revision and the chart's summary of it", func(t *testing.T) {
		chart := newSongChart("Asa Branca", fixedAt)
		require.NoError(t, charts.Create(ctx, chart))
		chart.Draft.RightsConfirmation = &domain.RightsConfirmation{ConfirmedBy: uuid.NewString(), ConfirmedAt: fixedAt}
		published, rev, err := chart.Publish(uuid.NewString(), fixedAt.Add(time.Hour))
		require.NoError(t, err)

		require.NoError(t, charts.Publish(ctx, published, rev))

		gotChart, err := charts.GetByID(ctx, chart.ID)
		require.NoError(t, err)
		assert.Equal(t, published, gotChart)
		gotRev, err := charts.GetRevision(ctx, chart.ID, 1)
		require.NoError(t, err)
		assert.Equal(t, rev, gotRev)
	})

	t.Run("revisions are listed newest first, and a missing one is not found", func(t *testing.T) {
		chart := newSongChart("Carinhoso", fixedAt)
		chart.Draft.RightsConfirmation = &domain.RightsConfirmation{ConfirmedBy: uuid.NewString(), ConfirmedAt: fixedAt}
		require.NoError(t, charts.Create(ctx, chart))
		first, rev1, err := chart.Publish(uuid.NewString(), fixedAt)
		require.NoError(t, err)
		require.NoError(t, charts.Publish(ctx, first, rev1))
		first.Draft.Title = "Carinhoso (corrected)"
		second, rev2, err := first.Publish(uuid.NewString(), fixedAt.Add(time.Hour))
		require.NoError(t, err)
		require.NoError(t, charts.Publish(ctx, second, rev2))

		revs, err := charts.ListRevisions(ctx, chart.ID)

		require.NoError(t, err)
		assert.Equal(t, []domain.SongChartRevision{rev2, rev1}, revs)
		_, err = charts.GetRevision(ctx, chart.ID, 3)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a revision number is never stored twice for a chart", func(t *testing.T) {
		chart := newSongChart("Duplicate", fixedAt)
		chart.Draft.RightsConfirmation = &domain.RightsConfirmation{ConfirmedBy: uuid.NewString(), ConfirmedAt: fixedAt}
		require.NoError(t, charts.Create(ctx, chart))
		published, rev, err := chart.Publish(uuid.NewString(), fixedAt)
		require.NoError(t, err)
		require.NoError(t, charts.Publish(ctx, published, rev))

		require.Error(t, charts.Publish(ctx, published, rev))
	})
}

func TestEntSongChartRepository_List(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	charts := NewEntSongChartRepository(client)

	older := newSongChart("Asa Branca", fixedAt)
	newer := newSongChart("Carinhoso", fixedAt.Add(time.Hour))
	published := newSongChart("Águas de Março", fixedAt.Add(2*time.Hour))
	published.Draft.Artist = "Tom Jobim"
	published.Draft.RightsConfirmation = &domain.RightsConfirmation{ConfirmedBy: uuid.NewString(), ConfirmedAt: fixedAt}
	for _, c := range []domain.SongChart{older, newer, published} {
		require.NoError(t, charts.Create(ctx, c))
	}
	publishedChart, rev, err := published.Publish(uuid.NewString(), fixedAt)
	require.NoError(t, err)
	require.NoError(t, charts.Publish(ctx, publishedChart, rev))

	ids := func(page domain.Page[domain.SongChart]) []string {
		out := make([]string, len(page.Items))
		for i, c := range page.Items {
			out[i] = c.ID
		}
		return out
	}

	t.Run("charts are listed most recently updated first, with the total", func(t *testing.T) {
		page, err := charts.List(ctx, domain.SongChartFilter{}, domain.PageRequest{Limit: 2})

		require.NoError(t, err)
		assert.Equal(t, []string{published.ID, newer.ID}, ids(page))
		assert.Equal(t, 3, page.Total)
		assert.Equal(t, 1, page.Items[0].PublishedRevision.Number, "a listed chart carries its revision summary")
	})

	t.Run("the status filter narrows the list", func(t *testing.T) {
		status := domain.SongChartDraftStatus

		page, err := charts.List(ctx, domain.SongChartFilter{Status: &status}, domain.PageRequest{Limit: 20})

		require.NoError(t, err)
		assert.Equal(t, []string{newer.ID, older.ID}, ids(page))
	})

	t.Run("a search matches title or artist, ignoring case and accents", func(t *testing.T) {
		for _, q := range []string{"aguas", "JOBIM"} {
			page, err := charts.List(ctx, domain.SongChartFilter{Q: q}, domain.PageRequest{Limit: 20})

			require.NoError(t, err)
			assert.Equal(t, []string{published.ID}, ids(page), q)
		}
	})

	t.Run("a search matches the published title and artist too, once the draft is retitled", func(t *testing.T) {
		retitled := publishedChart
		retitled.Draft.Title, retitled.Draft.Artist = "Retitled", "Someone else"
		require.NoError(t, charts.Save(ctx, retitled))

		for _, q := range []string{"aguas", "jobim", "retitled"} {
			page, err := charts.List(ctx, domain.SongChartFilter{Q: q}, domain.PageRequest{Limit: 20})

			require.NoError(t, err)
			assert.Equal(t, []string{published.ID}, ids(page), q)
		}
	})
}
