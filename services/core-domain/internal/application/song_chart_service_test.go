package application_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// songChartFixture wires a SongChartService over a small chord catalog:
// G, C, D, D/F# and Em with active voicings, Am with none, and G's withdrawn
// voicing "g-withdrawn".
type songChartFixture struct {
	service  *application.SongChartService
	charts   *fakeSongChartRepository
	catalog  *fakeChordCatalogRepository
	diagrams *fakeDiagramRepository
	now      time.Time
}

func newSongChartFixture() *songChartFixture {
	fSharp, six := "F#", 6
	voicing := func(id, chord string, rank int, status domain.ChordVoicingStatus) domain.ChordVoicing {
		return domain.ChordVoicing{ID: id, ChordDefinitionID: chord, DiagramID: "diagram-" + id, RecommendedRank: rank, Status: status}
	}
	gOpen := voicing("g-open", "chord-g", 1, domain.ChordVoicingActive)
	gBarre := voicing("g-barre", "chord-g", 2, domain.ChordVoicingActive)
	gWithdrawn := voicing("g-withdrawn", "chord-g", 3, domain.ChordVoicingWithdrawn)
	cOpen := voicing("c-open", "chord-c", 1, domain.ChordVoicingActive)
	dOpen := voicing("d-open", "chord-d", 1, domain.ChordVoicingActive)
	dOverFSharp := voicing("d-over-f-sharp-open", "chord-d-over-f-sharp", 1, domain.ChordVoicingActive)
	emOpen := voicing("em-open", "chord-em", 1, domain.ChordVoicingActive)

	catalog := &fakeChordCatalogRepository{
		chords: []domain.ChordDefinition{
			{ID: "chord-g", CanonicalSymbol: "G", RootPitchClass: 7, Quality: domain.ChordQualityMajor, Voicings: []domain.ChordVoicing{gOpen, gBarre}},
			{ID: "chord-c", CanonicalSymbol: "C", RootPitchClass: 0, Quality: domain.ChordQualityMajor, Voicings: []domain.ChordVoicing{cOpen}},
			{ID: "chord-d", CanonicalSymbol: "D", RootPitchClass: 2, Quality: domain.ChordQualityMajor, Voicings: []domain.ChordVoicing{dOpen}},
			{ID: "chord-d-over-f-sharp", CanonicalSymbol: "D/F#", RootPitchClass: 2, Quality: domain.ChordQualityMajor, Bass: &fSharp, BassPitchClass: &six, Voicings: []domain.ChordVoicing{dOverFSharp}},
			{ID: "chord-em", CanonicalSymbol: "Em", RootPitchClass: 4, Quality: domain.ChordQualityMinor, Voicings: []domain.ChordVoicing{emOpen}},
			{ID: "chord-am", CanonicalSymbol: "Am", RootPitchClass: 9, Quality: domain.ChordQualityMinor},
		},
		voicings: map[string]domain.ChordVoicing{},
	}
	for _, v := range []domain.ChordVoicing{gOpen, gBarre, gWithdrawn, cOpen, dOpen, dOverFSharp, emOpen} {
		catalog.voicings[v.ID] = v
	}
	diagrams := newFakeDiagramRepository()
	for id := range catalog.voicings {
		diagrams.byID["diagram-"+id] = domain.Diagram{ID: "diagram-" + id}
	}

	f := &songChartFixture{
		charts:   newFakeSongChartRepository(),
		catalog:  catalog,
		diagrams: diagrams,
		now:      time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC),
	}
	ids := 0
	newID := func() string { ids++; return "chart-" + string(rune('0'+ids)) }
	f.service = application.NewSongChartService(f.charts, catalog, diagrams, newFakeLanguageRepository(), newID, func() time.Time { return f.now })
	return f
}

// run is a lyric run with a chord: anchor id, written symbol, text.
func run(id, symbol, text string) domain.SongChartRun {
	return domain.SongChartRun{Text: text, Anchor: &domain.ChordAnchor{ID: id, WrittenSymbol: symbol}}
}

func pickedRun(id, symbol, text, voicingID string) domain.SongChartRun {
	r := run(id, symbol, text)
	r.Anchor.ChordVoicingID = &voicingID
	return r
}

func chartBody(runs ...domain.SongChartRun) domain.SongChartDocument {
	return domain.SongChartDocument{Sections: []domain.SongChartSection{{Kind: domain.SectionVerse, Lines: []domain.SongChartLine{{Runs: runs}}}}}
}

func chartInput(body domain.SongChartDocument) application.SongChartInput {
	return application.SongChartInput{Title: "Asa Branca", Artist: "Luiz Gonzaga", Language: "pt_BR", CapoFret: 0, Body: body}
}

// asaBranca is "[G]Quando olhei a [C]terra ardendo", with confirmed rights.
func asaBranca() application.SongChartInput {
	in := chartInput(chartBody(run("a1", "G", "Quando olhei a "), run("a2", "C", "terra ardendo")))
	in.RightsConfirmed = true
	return in
}

func chordOf(t *testing.T, chart domain.SongChart, anchorID string) *string {
	t.Helper()
	for _, a := range chart.Draft.Body.Anchors() {
		if a.Anchor.ID == anchorID {
			return a.Anchor.ChordDefinitionID
		}
	}
	t.Fatalf("no anchor %q", anchorID)
	return nil
}

func ptr[T any](v T) *T { return &v }

func TestSongChartService_Create(t *testing.T) {
	ctx := context.Background()

	t.Run("an admin starts a draft whose anchors resolve to catalog chords", func(t *testing.T) {
		f := newSongChartFixture()

		chart, err := f.service.Create(ctx, adminCaller(), chartInput(chartBody(run("a1", "G", "Quando olhei a "), run("a2", "C", "terra"))))

		require.NoError(t, err)
		assert.Equal(t, domain.SongChartDraftStatus, chart.Status)
		assert.Nil(t, chart.PublishedRevision)
		assert.Equal(t, adminCaller().ID, chart.CreatedBy)
		assert.Equal(t, f.now, chart.CreatedAt)
		assert.Equal(t, adminCaller().ID, chart.Draft.UpdatedBy)
		assert.Equal(t, ptr("chord-g"), chordOf(t, chart, "a1"))
		assert.Equal(t, ptr("chord-c"), chordOf(t, chart, "a2"))
		assert.Empty(t, chart.Draft.Warnings)
		assert.Nil(t, chart.Draft.RightsConfirmation)

		stored, err := f.charts.GetByID(ctx, chart.ID)
		require.NoError(t, err)
		assert.Equal(t, chart, stored)
	})

	t.Run("a resolved chord sent by the client is replaced", func(t *testing.T) {
		f := newSongChartFixture()
		r := run("a1", "G", "La")
		r.Anchor.ChordDefinitionID = ptr("chord-c")

		chart, err := f.service.Create(ctx, adminCaller(), chartInput(chartBody(r)))

		require.NoError(t, err)
		assert.Equal(t, ptr("chord-g"), chordOf(t, chart, "a1"))
	})

	t.Run("confirming the rights records the caller and the time", func(t *testing.T) {
		f := newSongChartFixture()

		chart, err := f.service.Create(ctx, adminCaller(), asaBranca())

		require.NoError(t, err)
		assert.Equal(t, &domain.RightsConfirmation{ConfirmedBy: adminCaller().ID, ConfirmedAt: f.now}, chart.Draft.RightsConfirmation)
	})

	resolution := []struct {
		name      string
		symbol    string
		wantChord *string
		wantKind  domain.AnchorWarningKind
	}{
		{name: "another spelling resolves and keeps its spelling", symbol: "Gmaj", wantChord: ptr("chord-g")},
		{name: "a slash chord the catalog has resolves with its bass", symbol: "D/F#", wantChord: ptr("chord-d-over-f-sharp")},
		{name: "a slash chord the catalog lacks resolves without its bass", symbol: "C/G", wantChord: ptr("chord-c"), wantKind: domain.AnchorBassNotInCatalog},
		{name: "a no-chord marking resolves to nothing, without a warning", symbol: "N.C."},
		{name: "an unparsed symbol warns", symbol: "H7", wantKind: domain.AnchorUnparsedSymbol},
		{name: "an unsupported quality warns", symbol: "C7#11", wantKind: domain.AnchorUnsupportedQuality},
		{name: "a chord the catalog lacks warns", symbol: "C#7", wantKind: domain.AnchorChordNotInCatalog},
		{name: "a catalog chord with no voicing warns", symbol: "Am", wantKind: domain.AnchorChordNotInCatalog},
		{name: "a slash chord whose chord without bass the catalog lacks warns", symbol: "C#7/G", wantKind: domain.AnchorChordNotInCatalog},
	}
	for _, tt := range resolution {
		t.Run(tt.name, func(t *testing.T) {
			f := newSongChartFixture()

			chart, err := f.service.Create(ctx, adminCaller(), chartInput(chartBody(run("a1", "G", "Intro "), run("a2", tt.symbol, "La"))))

			require.NoError(t, err)
			assert.Equal(t, tt.wantChord, chordOf(t, chart, "a2"))
			assert.Equal(t, tt.symbol, chart.Draft.Body.Anchors()[1].Anchor.WrittenSymbol)
			if tt.wantKind == "" {
				assert.Empty(t, chart.Draft.Warnings)
				return
			}
			assert.Equal(t, []domain.AnchorWarning{{AnchorID: "a2", WrittenSymbol: tt.symbol, Kind: tt.wantKind}}, chart.Draft.Warnings)
		})
	}

	t.Run("a warning names the anchor's section and line", func(t *testing.T) {
		f := newSongChartFixture()
		body := domain.SongChartDocument{Sections: []domain.SongChartSection{
			{Kind: domain.SectionVerse, Lines: []domain.SongChartLine{{Runs: []domain.SongChartRun{run("a1", "G", "La")}}}},
			{Kind: domain.SectionChorus, Lines: []domain.SongChartLine{{Comment: ptr("Twice")}, {Runs: []domain.SongChartRun{run("a2", "H7", "Lo")}}}},
		}}

		chart, err := f.service.Create(ctx, adminCaller(), chartInput(body))

		require.NoError(t, err)
		require.Len(t, chart.Draft.Warnings, 1)
		assert.Equal(t, domain.AnchorPosition{SectionIndex: 1, LineIndex: 1}, chart.Draft.Warnings[0].Position)
	})

	t.Run("a picked voicing of the anchor's chord is kept", func(t *testing.T) {
		f := newSongChartFixture()

		chart, err := f.service.Create(ctx, adminCaller(), chartInput(chartBody(pickedRun("a1", "G", "La", "g-barre"))))

		require.NoError(t, err)
		assert.Equal(t, ptr("g-barre"), chart.Draft.Body.Anchors()[0].Anchor.ChordVoicingID)
		assert.Empty(t, chart.Draft.Warnings)
	})

	t.Run("a picked voicing that was withdrawn warns", func(t *testing.T) {
		f := newSongChartFixture()

		chart, err := f.service.Create(ctx, adminCaller(), chartInput(chartBody(pickedRun("a1", "G", "La", "g-withdrawn"))))

		require.NoError(t, err)
		assert.Equal(t, []domain.AnchorWarning{{AnchorID: "a1", WrittenSymbol: "G", Kind: domain.AnchorVoicingUnavailable}}, chart.Draft.Warnings)
	})

	t.Run("a withdrawn pick on a slash chord resolved without its bass blocks publication", func(t *testing.T) {
		f := newSongChartFixture()
		f.catalog.withdraw("c-open")
		f.catalog.voicings["c-barre"] = domain.ChordVoicing{ID: "c-barre", ChordDefinitionID: "chord-c", Status: domain.ChordVoicingActive}
		f.catalog.chords[1].Voicings = append(f.catalog.chords[1].Voicings, f.catalog.voicings["c-barre"])

		chart, err := f.service.Create(ctx, adminCaller(), chartInput(chartBody(pickedRun("a1", "C/G", "La", "c-open"))))

		require.NoError(t, err)
		assert.Equal(t, []domain.AnchorWarning{{AnchorID: "a1", WrittenSymbol: "C/G", Kind: domain.AnchorVoicingUnavailable}}, chart.Draft.Warnings)
	})

	refusedVoicings := []struct {
		name    string
		symbol  string
		voicing string
	}{
		{name: "a voicing of another chord", symbol: "G", voicing: "d-open"},
		{name: "a voicing that doesn't exist", symbol: "G", voicing: "nope"},
		{name: "a voicing on a symbol that doesn't resolve", symbol: "H7", voicing: "g-open"},
		{name: "a voicing on a no-chord marking", symbol: "N.C.", voicing: "g-open"},
	}
	for _, tt := range refusedVoicings {
		t.Run("refuses "+tt.name, func(t *testing.T) {
			f := newSongChartFixture()

			_, err := f.service.Create(ctx, adminCaller(), chartInput(chartBody(run("a0", "C", "Intro "), pickedRun("a1", tt.symbol, "La", tt.voicing))))

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, "body/content/0/content/0/content/1/marks/0/attrs/chordVoicingId", valErr.Fields[0].Field)
		})
	}

	t.Run("refuses a language that doesn't exist", func(t *testing.T) {
		in := asaBranca()
		in.Language = "xx"

		_, err := newSongChartFixture().service.Create(ctx, adminCaller(), in)

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, "language", valErr.Fields[0].Field)
	})

	t.Run("refuses an invalid draft field", func(t *testing.T) {
		in := asaBranca()
		in.CapoFret = 13

		_, err := newSongChartFixture().service.Create(ctx, adminCaller(), in)

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, "capo_fret", valErr.Fields[0].Field)
	})

	t.Run("teachers and students cannot author song charts", func(t *testing.T) {
		for _, caller := range []domain.User{teacherCaller(), studentCaller()} {
			_, err := newSongChartFixture().service.Create(ctx, caller, asaBranca())

			assert.ErrorIs(t, err, domain.ErrForbidden, caller.Role)
		}
	})
}

func TestSongChartService_UpdateDraft(t *testing.T) {
	ctx := context.Background()

	t.Run("an admin replaces the draft, resolving its anchors again", func(t *testing.T) {
		f := newSongChartFixture()
		chart, err := f.service.Create(ctx, adminCaller(), asaBranca())
		require.NoError(t, err)
		f.now = f.now.Add(time.Hour)
		rui := domain.User{ID: "admin-2", Role: domain.RoleAdmin}
		in := asaBranca()
		in.Title = "Asa Branca (Luiz Gonzaga)"
		in.Body = chartBody(run("a1", "Em", "Quando"))

		updated, err := f.service.UpdateDraft(ctx, rui, chart.ID, in)

		require.NoError(t, err)
		assert.Equal(t, "Asa Branca (Luiz Gonzaga)", updated.Draft.Title)
		assert.Equal(t, ptr("chord-em"), chordOf(t, updated, "a1"))
		assert.Equal(t, "admin-2", updated.Draft.UpdatedBy)
		assert.Equal(t, f.now, updated.Draft.UpdatedAt)
		assert.Equal(t, adminCaller().ID, updated.Draft.RightsConfirmation.ConfirmedBy, "the first confirmation stays")
		stored, _ := f.charts.GetByID(ctx, chart.ID)
		assert.Equal(t, updated, stored)
	})

	t.Run("clearing the rights confirmation removes it", func(t *testing.T) {
		f := newSongChartFixture()
		chart, _ := f.service.Create(ctx, adminCaller(), asaBranca())
		in := asaBranca()
		in.RightsConfirmed = false

		updated, err := f.service.UpdateDraft(ctx, adminCaller(), chart.ID, in)

		require.NoError(t, err)
		assert.Nil(t, updated.Draft.RightsConfirmation)
	})

	t.Run("editing a published chart's draft doesn't change its revision", func(t *testing.T) {
		f := newSongChartFixture()
		chart, _ := f.service.Create(ctx, adminCaller(), asaBranca())
		_, err := f.service.Publish(ctx, adminCaller(), chart.ID)
		require.NoError(t, err)
		in := asaBranca()
		in.Title = "Changed"

		_, err = f.service.UpdateDraft(ctx, adminCaller(), chart.ID, in)

		require.NoError(t, err)
		rev, err := f.service.GetPublished(ctx, studentCaller(), chart.ID)
		require.NoError(t, err)
		assert.Equal(t, "Asa Branca", rev.Title)
	})

	t.Run("an unknown chart is not found", func(t *testing.T) {
		_, err := newSongChartFixture().service.UpdateDraft(ctx, adminCaller(), "nope", asaBranca())

		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a teacher cannot edit a draft", func(t *testing.T) {
		f := newSongChartFixture()
		chart, _ := f.service.Create(ctx, adminCaller(), asaBranca())

		_, err := f.service.UpdateDraft(ctx, teacherCaller(), chart.ID, asaBranca())

		require.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestSongChartService_Publish(t *testing.T) {
	ctx := context.Background()

	t.Run("an admin publishes revision 1, which learners read", func(t *testing.T) {
		f := newSongChartFixture()
		chart, _ := f.service.Create(ctx, adminCaller(), asaBranca())
		rui := domain.User{ID: "admin-2", Role: domain.RoleAdmin}

		rev, err := f.service.Publish(ctx, rui, chart.ID)

		require.NoError(t, err)
		assert.Equal(t, 1, rev.Number)
		assert.Equal(t, "admin-2", rev.PublishedBy)
		assert.Equal(t, adminCaller().ID, rev.RightsConfirmation.ConfirmedBy)
		stored, _ := f.charts.GetByID(ctx, chart.ID)
		assert.Equal(t, domain.SongChartPublished, stored.Status)
		assert.Equal(t, 1, stored.PublishedRevision.Number)
		revs, _ := f.charts.ListRevisions(ctx, chart.ID)
		assert.Equal(t, []domain.SongChartRevision{rev}, revs)
	})

	t.Run("a draft whose rights aren't confirmed isn't published", func(t *testing.T) {
		f := newSongChartFixture()
		in := asaBranca()
		in.RightsConfirmed = false
		chart, _ := f.service.Create(ctx, adminCaller(), in)

		_, err := f.service.Publish(ctx, adminCaller(), chart.ID)

		var refusal *domain.SongChartNotPublishableError
		require.ErrorAs(t, err, &refusal)
		assert.Equal(t, []domain.NotPublishableReason{domain.RightsNotConfirmed}, refusal.Reasons)
		stored, _ := f.charts.GetByID(ctx, chart.ID)
		assert.Equal(t, domain.SongChartDraftStatus, stored.Status)
	})

	t.Run("a draft with chords that block publication lists them", func(t *testing.T) {
		f := newSongChartFixture()
		in := asaBranca()
		in.Body = chartBody(run("a1", "H7", "La "), run("a2", "C#7", "Lo"))
		chart, _ := f.service.Create(ctx, adminCaller(), in)

		_, err := f.service.Publish(ctx, adminCaller(), chart.ID)

		var refusal *domain.SongChartNotPublishableError
		require.ErrorAs(t, err, &refusal)
		assert.Equal(t, []domain.NotPublishableReason{domain.UnresolvedChords}, refusal.Reasons)
		require.Len(t, refusal.AnchorWarnings, 2)
		assert.Equal(t, "C#7", refusal.AnchorWarnings[1].WrittenSymbol)
	})

	t.Run("chords are checked again at publication", func(t *testing.T) {
		f := newSongChartFixture()
		in := asaBranca()
		in.Body = chartBody(pickedRun("a1", "G", "La", "g-barre"))
		chart, _ := f.service.Create(ctx, adminCaller(), in)
		f.catalog.withdraw("g-barre")

		_, err := f.service.Publish(ctx, adminCaller(), chart.ID)

		var refusal *domain.SongChartNotPublishableError
		require.ErrorAs(t, err, &refusal)
		assert.Equal(t, domain.AnchorVoicingUnavailable, refusal.AnchorWarnings[0].Kind)
	})

	t.Run("a pick of a chord's only voicing, withdrawn since, blocks publication", func(t *testing.T) {
		f := newSongChartFixture()
		in := asaBranca()
		in.Body = chartBody(pickedRun("a1", "C", "La", "c-open"))
		chart, _ := f.service.Create(ctx, adminCaller(), in)
		f.catalog.withdraw("c-open")

		_, err := f.service.Publish(ctx, adminCaller(), chart.ID)

		var refusal *domain.SongChartNotPublishableError
		require.ErrorAs(t, err, &refusal)
		assert.Equal(t, []domain.AnchorWarning{{AnchorID: "a1", WrittenSymbol: "C", Kind: domain.AnchorVoicingUnavailable}}, refusal.AnchorWarnings)
	})

	t.Run("a slash chord missing from the catalog publishes", func(t *testing.T) {
		f := newSongChartFixture()
		in := asaBranca()
		in.Body = chartBody(run("a1", "C/G", "La"))
		chart, _ := f.service.Create(ctx, adminCaller(), in)

		_, err := f.service.Publish(ctx, adminCaller(), chart.ID)

		require.NoError(t, err)
	})

	t.Run("a correction is published as the next revision, leaving the first unchanged", func(t *testing.T) {
		f := newSongChartFixture()
		chart, _ := f.service.Create(ctx, adminCaller(), asaBranca())
		first, _ := f.service.Publish(ctx, adminCaller(), chart.ID)
		in := asaBranca()
		in.Body = chartBody(run("a1", "G", "Quando olhei a "), run("a2", "Em", "terra ardendo"))
		_, err := f.service.UpdateDraft(ctx, adminCaller(), chart.ID, in)
		require.NoError(t, err)

		second, err := f.service.Publish(ctx, adminCaller(), chart.ID)

		require.NoError(t, err)
		assert.Equal(t, 2, second.Number)
		revs, _ := f.charts.ListRevisions(ctx, chart.ID)
		require.Len(t, revs, 2)
		assert.Equal(t, first, revs[1])
	})

	t.Run("an unknown chart is not found", func(t *testing.T) {
		_, err := newSongChartFixture().service.Publish(ctx, adminCaller(), "nope")

		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a teacher cannot publish", func(t *testing.T) {
		f := newSongChartFixture()
		chart, _ := f.service.Create(ctx, adminCaller(), asaBranca())

		_, err := f.service.Publish(ctx, teacherCaller(), chart.ID)

		require.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestSongChartService_Withdraw(t *testing.T) {
	ctx := context.Background()

	t.Run("a withdrawn chart can't be read, and its revisions stay", func(t *testing.T) {
		f := newSongChartFixture()
		chart, _ := f.service.Create(ctx, adminCaller(), asaBranca())
		_, _ = f.service.Publish(ctx, adminCaller(), chart.ID)

		withdrawn, err := f.service.Withdraw(ctx, adminCaller(), chart.ID, "Rights disputed")

		require.NoError(t, err)
		assert.Equal(t, domain.SongChartWithdrawn, withdrawn.Status)
		assert.Equal(t, "Rights disputed", withdrawn.Withdrawal.Reason)
		_, err = f.service.GetPublished(ctx, studentCaller(), chart.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)
		revs, _ := f.service.ListRevisions(ctx, adminCaller(), chart.ID)
		assert.Len(t, revs, 1)
	})

	t.Run("a chart that isn't published can't be withdrawn", func(t *testing.T) {
		f := newSongChartFixture()
		chart, _ := f.service.Create(ctx, adminCaller(), asaBranca())

		_, err := f.service.Withdraw(ctx, adminCaller(), chart.ID, "Not ready")

		require.ErrorIs(t, err, domain.ErrConflict)
	})

	t.Run("a teacher cannot withdraw", func(t *testing.T) {
		_, err := newSongChartFixture().service.Withdraw(ctx, teacherCaller(), "nope", "x")

		require.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestSongChartService_ListRevisions(t *testing.T) {
	ctx := context.Background()

	t.Run("a chart never published has no revisions", func(t *testing.T) {
		f := newSongChartFixture()
		chart, _ := f.service.Create(ctx, adminCaller(), asaBranca())

		revs, err := f.service.ListRevisions(ctx, adminCaller(), chart.ID)

		require.NoError(t, err)
		assert.Empty(t, revs)
	})

	t.Run("an unknown chart is not found", func(t *testing.T) {
		_, err := newSongChartFixture().service.ListRevisions(ctx, adminCaller(), "nope")

		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestSongChartService_List(t *testing.T) {
	ctx := context.Background()

	t.Run("an admin lists published charts only", func(t *testing.T) {
		f := newSongChartFixture()
		published, _ := f.service.Create(ctx, adminCaller(), asaBranca())
		_, _ = f.service.Publish(ctx, adminCaller(), published.ID)
		_, _ = f.service.Create(ctx, adminCaller(), asaBranca())
		status := domain.SongChartPublished

		page, err := f.service.List(ctx, adminCaller(), domain.SongChartFilter{Status: &status}, domain.PageRequest{Limit: 20})

		require.NoError(t, err)
		require.Len(t, page.Items, 1)
		assert.Equal(t, published.ID, page.Items[0].ID)
		assert.Equal(t, 1, page.Total)
	})

	t.Run("a status the API doesn't know is a validation error, not an empty list", func(t *testing.T) {
		unknown := domain.SongChartStatus("bogus")

		_, err := newSongChartFixture().service.List(ctx, adminCaller(), domain.SongChartFilter{Status: &unknown}, domain.PageRequest{Limit: 20})

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, "status", valErr.Fields[0].Field)
	})

	t.Run("a teacher cannot list song charts", func(t *testing.T) {
		_, err := newSongChartFixture().service.List(ctx, teacherCaller(), domain.SongChartFilter{}, domain.PageRequest{Limit: 20})

		require.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestSongChartService_Get(t *testing.T) {
	ctx := context.Background()

	t.Run("an admin reads a chart with its draft", func(t *testing.T) {
		f := newSongChartFixture()
		chart, _ := f.service.Create(ctx, adminCaller(), asaBranca())

		got, err := f.service.Get(ctx, adminCaller(), chart.ID)

		require.NoError(t, err)
		assert.Equal(t, chart, got)
	})

	t.Run("a student cannot read a chart's draft", func(t *testing.T) {
		f := newSongChartFixture()
		chart, _ := f.service.Create(ctx, adminCaller(), asaBranca())

		_, err := f.service.Get(ctx, studentCaller(), chart.ID)

		require.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func voicingIDs(c domain.ChordDefinition) []string {
	ids := make([]string, len(c.Voicings))
	for i, v := range c.Voicings {
		ids[i] = v.ID
	}
	return ids
}

func TestSongChartService_GetPublished(t *testing.T) {
	ctx := context.Background()

	publishedChart := func(t *testing.T, f *songChartFixture, in application.SongChartInput) domain.SongChart {
		t.Helper()
		chart, err := f.service.Create(ctx, adminCaller(), in)
		require.NoError(t, err)
		_, err = f.service.Publish(ctx, adminCaller(), chart.ID)
		require.NoError(t, err)
		return chart
	}

	t.Run("a student reads the published revision with its chords, voicings and diagrams", func(t *testing.T) {
		f := newSongChartFixture()
		in := asaBranca()
		in.Body = chartBody(run("a1", "G", "Quando "), run("a2", "C", "olhei "), run("a3", "G", "a terra"))
		chart := publishedChart(t, f, in)

		got, err := f.service.GetPublished(ctx, studentCaller(), chart.ID)

		require.NoError(t, err)
		assert.Equal(t, ptr(1), got.RevisionNumber)
		assert.Equal(t, "Asa Branca", got.Title)
		assert.Equal(t, domain.StandardGuitarTuningFingerprint, got.TuningFingerprint)
		require.Len(t, got.Chords, 2, "each chord once, in document order")
		assert.Equal(t, "chord-g", got.Chords[0].ID)
		assert.Equal(t, []string{"g-open", "g-barre"}, voicingIDs(got.Chords[0]))
		assert.Equal(t, "chord-c", got.Chords[1].ID)
		diagramIDs := make([]string, len(got.Diagrams))
		for i, d := range got.Diagrams {
			diagramIDs[i] = d.ID
		}
		assert.Equal(t, []string{"diagram-g-open", "diagram-g-barre", "diagram-c-open"}, diagramIDs)
	})

	t.Run("a picked voicing that was withdrawn after publication is included last", func(t *testing.T) {
		f := newSongChartFixture()
		in := asaBranca()
		in.Body = chartBody(pickedRun("a1", "G", "La", "g-barre"))
		chart := publishedChart(t, f, in)
		f.catalog.withdraw("g-barre")

		got, err := f.service.GetPublished(ctx, studentCaller(), chart.ID)

		require.NoError(t, err)
		assert.Equal(t, []string{"g-open", "g-barre"}, voicingIDs(got.Chords[0]))
	})

	t.Run("a chart never published is not found", func(t *testing.T) {
		f := newSongChartFixture()
		chart, _ := f.service.Create(ctx, adminCaller(), asaBranca())

		_, err := f.service.GetPublished(ctx, studentCaller(), chart.ID)

		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a chart that doesn't exist is not found", func(t *testing.T) {
		_, err := newSongChartFixture().service.GetPublished(ctx, studentCaller(), "nope")

		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestSongChartService_Preview(t *testing.T) {
	ctx := context.Background()

	t.Run("an admin previews the draft, with no revision number", func(t *testing.T) {
		f := newSongChartFixture()
		in := asaBranca()
		in.Body = chartBody(run("a1", "G", "La "), run("a2", "H7", "Lo"))
		chart, _ := f.service.Create(ctx, adminCaller(), in)

		got, err := f.service.Preview(ctx, adminCaller(), chart.ID)

		require.NoError(t, err)
		assert.Nil(t, got.RevisionNumber)
		require.Len(t, got.Chords, 1, "an anchor that doesn't resolve has no chord")
		assert.Equal(t, "chord-g", got.Chords[0].ID)
	})

	t.Run("a student cannot preview", func(t *testing.T) {
		f := newSongChartFixture()
		chart, _ := f.service.Create(ctx, adminCaller(), asaBranca())

		_, err := f.service.Preview(ctx, studentCaller(), chart.ID)

		require.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestSongChartService_WithdrawNeedsAReason(t *testing.T) {
	f := newSongChartFixture()
	chart, _ := f.service.Create(context.Background(), adminCaller(), asaBranca())
	_, _ = f.service.Publish(context.Background(), adminCaller(), chart.ID)

	_, err := f.service.Withdraw(context.Background(), adminCaller(), chart.ID, strings.Repeat(" ", 3))

	var valErr *domain.ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "reason", valErr.Fields[0].Field)
}
