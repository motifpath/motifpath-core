package domain_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

// oneLineChart is a document with one verse holding "[G]Quando olhei a [C]terra".
const oneLineChart = `{"type":"doc","content":[{"type":"section","attrs":{"kind":"verse","label":null},"content":[
	{"type":"lyricLine","content":[
		{"type":"text","text":"Quando olhei a ","marks":[{"type":"chordAnchor","attrs":{"anchorId":"a1","writtenSymbol":"G","chordDefinitionId":null,"chordVoicingId":null}}]},
		{"type":"text","text":"terra","marks":[{"type":"chordAnchor","attrs":{"anchorId":"a2","writtenSymbol":"C","chordDefinitionId":null,"chordVoicingId":null}}]}
	]},
	{"type":"comment","content":[{"type":"text","text":"Repeat twice"}]}
]}]}`

func TestParseSongChartDocument(t *testing.T) {
	t.Run("a document is read with its sections, lines and anchors", func(t *testing.T) {
		doc, err := domain.ParseSongChartDocument([]byte(oneLineChart))

		require.NoError(t, err)
		require.Len(t, doc.Sections, 1)
		section := doc.Sections[0]
		assert.Equal(t, domain.SectionVerse, section.Kind)
		assert.Nil(t, section.Label)
		require.Len(t, section.Lines, 2)
		require.Len(t, section.Lines[0].Runs, 2)
		assert.Equal(t, "Quando olhei a ", section.Lines[0].Runs[0].Text)
		assert.Equal(t, "G", section.Lines[0].Runs[0].Anchor.WrittenSymbol)
		assert.Equal(t, "a2", section.Lines[0].Runs[1].Anchor.ID)
		require.NotNil(t, section.Lines[1].Comment)
		assert.Equal(t, "Repeat twice", *section.Lines[1].Comment)
	})

	t.Run("writing a read document gives back the same JSON", func(t *testing.T) {
		doc, err := domain.ParseSongChartDocument([]byte(oneLineChart))
		require.NoError(t, err)

		out, err := json.Marshal(doc)

		require.NoError(t, err)
		assert.JSONEq(t, oneLineChart, string(out))
	})

	t.Run("a run without an anchor is written without marks", func(t *testing.T) {
		raw := `{"type":"doc","content":[{"type":"section","attrs":{"kind":"other","label":"Intro"},"content":[{"type":"lyricLine","content":[{"type":"text","text":"La"}]}]}]}`
		doc, err := domain.ParseSongChartDocument([]byte(raw))
		require.NoError(t, err)

		out, err := json.Marshal(doc)

		require.NoError(t, err)
		assert.JSONEq(t, raw, string(out))
	})

	invalid := []struct {
		name      string
		raw       string
		wantField string
	}{
		{name: "no sections", raw: `{"type":"doc","content":[]}`, wantField: "body/content"},
		{name: "a root that isn't a doc", raw: `{"type":"paragraph","content":[]}`, wantField: "body/type"},
		{name: "a section with no lines", raw: `{"type":"doc","content":[{"type":"section","attrs":{"kind":"verse","label":null},"content":[]}]}`, wantField: "body/content/0/content"},
		{name: "an unknown section kind", raw: `{"type":"doc","content":[{"type":"section","attrs":{"kind":"coda","label":null},"content":[{"type":"comment","content":[{"type":"text","text":"x"}]}]}]}`, wantField: "body/content/0/attrs/kind"},
		{name: "a label over 100 characters", raw: `{"type":"doc","content":[{"type":"section","attrs":{"kind":"verse","label":"` + strings.Repeat("x", 101) + `"},"content":[{"type":"comment","content":[{"type":"text","text":"x"}]}]}]}`, wantField: "body/content/0/attrs/label"},
		{name: "a paragraph where a line belongs", raw: `{"type":"doc","content":[{"type":"section","attrs":{"kind":"verse","label":null},"content":[{"type":"paragraph","content":[{"type":"text","text":"x"}]}]}]}`, wantField: "body/content/0/content/0/type"},
		{name: "a lyric line with no text", raw: `{"type":"doc","content":[{"type":"section","attrs":{"kind":"verse","label":null},"content":[{"type":"lyricLine","content":[]}]}]}`, wantField: "body/content/0/content/0/content"},
		{name: "an empty text run", raw: `{"type":"doc","content":[{"type":"section","attrs":{"kind":"verse","label":null},"content":[{"type":"lyricLine","content":[{"type":"text","text":""}]}]}]}`, wantField: "body/content/0/content/0/content/0/text"},
		{name: "a mark other than a chord anchor", raw: `{"type":"doc","content":[{"type":"section","attrs":{"kind":"verse","label":null},"content":[{"type":"lyricLine","content":[{"type":"text","text":"x","marks":[{"type":"bold"}]}]}]}]}`, wantField: "body/content/0/content/0/content/0/marks/0/type"},
		{name: "two marks on one run", raw: `{"type":"doc","content":[{"type":"section","attrs":{"kind":"verse","label":null},"content":[{"type":"lyricLine","content":[{"type":"text","text":"x","marks":[` + anchorJSON("a1", "G") + `,` + anchorJSON("a2", "C") + `]}]}]}]}`, wantField: "body/content/0/content/0/content/0/marks"},
		{name: "a chord symbol over 32 characters", raw: `{"type":"doc","content":[{"type":"section","attrs":{"kind":"verse","label":null},"content":[{"type":"lyricLine","content":[{"type":"text","text":"x","marks":[` + anchorJSON("a1", strings.Repeat("C", 33)) + `]}]}]}]}`, wantField: "body/content/0/content/0/content/0/marks/0/attrs/writtenSymbol"},
		{name: "an empty anchor id", raw: `{"type":"doc","content":[{"type":"section","attrs":{"kind":"verse","label":null},"content":[{"type":"lyricLine","content":[{"type":"text","text":"x","marks":[` + anchorJSON("", "G") + `]}]}]}]}`, wantField: "body/content/0/content/0/content/0/marks/0/attrs/anchorId"},
		{name: "two anchors with the same id", raw: `{"type":"doc","content":[{"type":"section","attrs":{"kind":"verse","label":null},"content":[{"type":"lyricLine","content":[{"type":"text","text":"x","marks":[` + anchorJSON("a1", "G") + `]},{"type":"text","text":"y","marks":[` + anchorJSON("a1", "C") + `]}]}]}]}`, wantField: "body/content/0/content/0/content/1/marks/0/attrs/anchorId"},
		{name: "a comment with chords", raw: `{"type":"doc","content":[{"type":"section","attrs":{"kind":"verse","label":null},"content":[{"type":"comment","content":[{"type":"text","text":"x","marks":[` + anchorJSON("a1", "G") + `]}]}]}]}`, wantField: "body/content/0/content/0/content/0/marks"},
		{name: "JSON that isn't an object", raw: `[]`, wantField: "body"},
	}
	for _, tt := range invalid {
		t.Run("refuses "+tt.name, func(t *testing.T) {
			_, err := domain.ParseSongChartDocument([]byte(tt.raw))

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, tt.wantField, valErr.Fields[0].Field)
		})
	}
}

func anchorJSON(id, symbol string) string {
	return `{"type":"chordAnchor","attrs":{"anchorId":"` + id + `","writtenSymbol":"` + symbol + `","chordDefinitionId":null,"chordVoicingId":null}}`
}

func TestSongChartDocument_Anchors(t *testing.T) {
	doc, err := domain.ParseSongChartDocument([]byte(oneLineChart))
	require.NoError(t, err)

	anchors := doc.Anchors()

	require.Len(t, anchors, 2)
	assert.Equal(t, domain.AnchorPosition{SectionIndex: 0, LineIndex: 0}, anchors[1].Position)
	assert.Equal(t, "C", anchors[1].Anchor.WrittenSymbol)
}

func TestSongChartDocument_WithAnchors(t *testing.T) {
	doc, err := domain.ParseSongChartDocument([]byte(oneLineChart))
	require.NoError(t, err)
	chordID := "chord-g"

	changed := doc.WithAnchors(func(a domain.ChordAnchor) domain.ChordAnchor {
		if a.WrittenSymbol == "G" {
			a.ChordDefinitionID = &chordID
		}
		return a
	})

	assert.Equal(t, &chordID, changed.Anchors()[0].Anchor.ChordDefinitionID)
	assert.Nil(t, doc.Anchors()[0].Anchor.ChordDefinitionID, "the original document is unchanged")
}

func validDraft() domain.SongChartDraft {
	doc, _ := domain.ParseSongChartDocument([]byte(oneLineChart))
	key, tempo := "G", 96
	return domain.SongChartDraft{
		Title: "Asa Branca", Artist: "Luiz Gonzaga", Language: "pt_BR", ConcertKey: &key,
		CapoFret: 2, TempoBPM: &tempo, TimeSignature: &domain.TimeSignature{Beats: 2, BeatValue: 4}, Body: doc,
	}
}

func TestSongChartDraft_Validate(t *testing.T) {
	t.Run("a complete draft is valid", func(t *testing.T) {
		require.NoError(t, validDraft().Validate())
	})

	t.Run("a draft without key, tempo or meter is valid", func(t *testing.T) {
		d := validDraft()
		d.ConcertKey, d.TempoBPM, d.TimeSignature = nil, nil, nil

		require.NoError(t, d.Validate())
	})

	key := func(k string) *string { return &k }
	tempo := func(n int) *int { return &n }
	tests := []struct {
		name      string
		change    func(*domain.SongChartDraft)
		wantField string
	}{
		{name: "a blank title", change: func(d *domain.SongChartDraft) { d.Title = "  " }, wantField: "title"},
		{name: "a title over 200 characters", change: func(d *domain.SongChartDraft) { d.Title = strings.Repeat("x", 201) }, wantField: "title"},
		{name: "a blank artist", change: func(d *domain.SongChartDraft) { d.Artist = "" }, wantField: "artist"},
		{name: "no language", change: func(d *domain.SongChartDraft) { d.Language = "" }, wantField: "language"},
		{name: "the language any", change: func(d *domain.SongChartDraft) { d.Language = domain.LanguageCodeAny }, wantField: "language"},
		{name: "a key that isn't a key", change: func(d *domain.SongChartDraft) { d.ConcertKey = key("H") }, wantField: "concert_key"},
		{name: "a key with a quality other than minor", change: func(d *domain.SongChartDraft) { d.ConcertKey = key("G7") }, wantField: "concert_key"},
		{name: "a capo below the nut", change: func(d *domain.SongChartDraft) { d.CapoFret = -1 }, wantField: "capo_fret"},
		{name: "a capo past fret 12", change: func(d *domain.SongChartDraft) { d.CapoFret = 13 }, wantField: "capo_fret"},
		{name: "a tempo under 20", change: func(d *domain.SongChartDraft) { d.TempoBPM = tempo(19) }, wantField: "tempo_bpm"},
		{name: "a tempo over 300", change: func(d *domain.SongChartDraft) { d.TempoBPM = tempo(301) }, wantField: "tempo_bpm"},
		{name: "a meter of 4/3", change: func(d *domain.SongChartDraft) { d.TimeSignature = &domain.TimeSignature{Beats: 4, BeatValue: 3} }, wantField: "time_signature"},
	}
	for _, tt := range tests {
		t.Run("refuses "+tt.name, func(t *testing.T) {
			d := validDraft()
			tt.change(&d)

			var valErr *domain.ValidationError
			require.ErrorAs(t, d.Validate(), &valErr)
			assert.Equal(t, tt.wantField, valErr.Fields[0].Field)
		})
	}

	t.Run("accepts a minor key and accidentals", func(t *testing.T) {
		for _, k := range []string{"F#m", "Bb", "Ebm", "C#"} {
			d := validDraft()
			d.ConcertKey = key(k)
			assert.NoError(t, d.Validate(), k)
		}
	})
}

func TestAnchorWarning_BlocksPublication(t *testing.T) {
	for kind, blocks := range map[domain.AnchorWarningKind]bool{
		domain.AnchorUnparsedSymbol:     true,
		domain.AnchorUnsupportedQuality: true,
		domain.AnchorChordNotInCatalog:  true,
		domain.AnchorVoicingUnavailable: true,
		domain.AnchorBassNotInCatalog:   false,
	} {
		assert.Equal(t, blocks, domain.AnchorWarning{Kind: kind}.BlocksPublication(), kind)
	}
}

var (
	publishedAt = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	confirmedAt = time.Date(2026, 10, 7, 11, 0, 0, 0, time.UTC)
)

func confirmedChart() domain.SongChart {
	draft := validDraft()
	draft.RightsConfirmation = &domain.RightsConfirmation{ConfirmedBy: "ana", ConfirmedAt: confirmedAt}
	return domain.SongChart{ID: "chart-1", Status: domain.SongChartDraftStatus, Draft: draft, CreatedBy: "ana"}
}

func TestSongChart_Publish(t *testing.T) {
	t.Run("the first publication is revision 1, keeping the draft's rights confirmation", func(t *testing.T) {
		chart, rev, err := confirmedChart().Publish("rui", publishedAt)

		require.NoError(t, err)
		assert.Equal(t, domain.SongChartPublished, chart.Status)
		assert.Equal(t, 1, rev.Number)
		assert.Equal(t, &domain.SongChartRevisionSummary{Number: 1, Title: "Asa Branca", Language: "pt_BR", PublishedBy: "rui", PublishedAt: publishedAt}, chart.PublishedRevision)
		assert.Equal(t, "rui", rev.PublishedBy)
		assert.Equal(t, publishedAt, rev.PublishedAt)
		assert.Equal(t, domain.RightsConfirmation{ConfirmedBy: "ana", ConfirmedAt: confirmedAt}, rev.RightsConfirmation)
		assert.Equal(t, "Asa Branca", rev.Title)
		assert.Equal(t, domain.StandardGuitarTuningFingerprint, rev.TuningFingerprint)
	})

	t.Run("a later publication is the next revision", func(t *testing.T) {
		chart := confirmedChart()
		chart.Status, chart.PublishedRevision = domain.SongChartPublished, &domain.SongChartRevisionSummary{Number: 3}

		_, rev, err := chart.Publish("ana", publishedAt)

		require.NoError(t, err)
		assert.Equal(t, 4, rev.Number)
	})

	t.Run("publishing a withdrawn chart serves it again", func(t *testing.T) {
		chart := confirmedChart()
		chart.Status, chart.PublishedRevision = domain.SongChartWithdrawn, &domain.SongChartRevisionSummary{Number: 1}
		chart.Withdrawal = &domain.SongChartWithdrawal{WithdrawnBy: "ana", WithdrawnAt: confirmedAt, Reason: "x"}

		published, _, err := chart.Publish("ana", publishedAt)

		require.NoError(t, err)
		assert.Equal(t, domain.SongChartPublished, published.Status)
		assert.Nil(t, published.Withdrawal)
	})

	t.Run("a chart whose rights aren't confirmed isn't publishable", func(t *testing.T) {
		chart := confirmedChart()
		chart.Draft.RightsConfirmation = nil

		_, _, err := chart.Publish("ana", publishedAt)

		var notPublishable *domain.SongChartNotPublishableError
		require.ErrorAs(t, err, &notPublishable)
		assert.Equal(t, []domain.NotPublishableReason{domain.RightsNotConfirmed}, notPublishable.Reasons)
		assert.ErrorIs(t, err, domain.ErrConflict)
	})

	t.Run("blocking warnings are listed; a missing bass doesn't block", func(t *testing.T) {
		chart := confirmedChart()
		blocking := domain.AnchorWarning{AnchorID: "a1", WrittenSymbol: "H7", Kind: domain.AnchorUnparsedSymbol}
		chart.Draft.Warnings = []domain.AnchorWarning{blocking, {AnchorID: "a2", WrittenSymbol: "C/G", Kind: domain.AnchorBassNotInCatalog}}

		_, _, err := chart.Publish("ana", publishedAt)

		var notPublishable *domain.SongChartNotPublishableError
		require.ErrorAs(t, err, &notPublishable)
		assert.Equal(t, []domain.NotPublishableReason{domain.UnresolvedChords}, notPublishable.Reasons)
		assert.Equal(t, []domain.AnchorWarning{blocking}, notPublishable.AnchorWarnings)
	})

	t.Run("every reason is reported at once", func(t *testing.T) {
		chart := confirmedChart()
		chart.Draft.RightsConfirmation = nil
		chart.Draft.Warnings = []domain.AnchorWarning{{AnchorID: "a1", Kind: domain.AnchorChordNotInCatalog}}

		_, _, err := chart.Publish("ana", publishedAt)

		var notPublishable *domain.SongChartNotPublishableError
		require.ErrorAs(t, err, &notPublishable)
		assert.Equal(t, []domain.NotPublishableReason{domain.RightsNotConfirmed, domain.UnresolvedChords}, notPublishable.Reasons)
	})

	t.Run("a bass-only warning publishes", func(t *testing.T) {
		chart := confirmedChart()
		chart.Draft.Warnings = []domain.AnchorWarning{{AnchorID: "a1", Kind: domain.AnchorBassNotInCatalog}}

		_, _, err := chart.Publish("ana", publishedAt)

		require.NoError(t, err)
	})
}

func TestSongChart_Withdraw(t *testing.T) {
	t.Run("a published chart is withdrawn with who, when and why", func(t *testing.T) {
		chart := confirmedChart()
		chart.Status = domain.SongChartPublished

		withdrawn, err := chart.Withdraw("ana", publishedAt, "Rights disputed")

		require.NoError(t, err)
		assert.Equal(t, domain.SongChartWithdrawn, withdrawn.Status)
		assert.Equal(t, &domain.SongChartWithdrawal{WithdrawnBy: "ana", WithdrawnAt: publishedAt, Reason: "Rights disputed"}, withdrawn.Withdrawal)
	})

	t.Run("a chart that isn't published can't be withdrawn", func(t *testing.T) {
		for _, status := range []domain.SongChartStatus{domain.SongChartDraftStatus, domain.SongChartWithdrawn} {
			chart := confirmedChart()
			chart.Status = status

			_, err := chart.Withdraw("ana", publishedAt, "x")

			assert.ErrorIs(t, err, domain.ErrConflict, status)
		}
	})

	t.Run("a withdrawal needs a reason of at most 2000 characters", func(t *testing.T) {
		for _, reason := range []string{" ", strings.Repeat("x", 2001)} {
			chart := confirmedChart()
			chart.Status = domain.SongChartPublished

			_, err := chart.Withdraw("ana", publishedAt, reason)

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, "reason", valErr.Fields[0].Field)
		}
	})
}

func TestSongChartDraft_ConfirmRights(t *testing.T) {
	t.Run("confirming records who and when", func(t *testing.T) {
		d := validDraft()

		d.SetRightsConfirmed(true, "ana", confirmedAt)

		assert.Equal(t, &domain.RightsConfirmation{ConfirmedBy: "ana", ConfirmedAt: confirmedAt}, d.RightsConfirmation)
	})

	t.Run("confirming again keeps the first confirmation", func(t *testing.T) {
		d := validDraft()
		d.SetRightsConfirmed(true, "ana", confirmedAt)

		d.SetRightsConfirmed(true, "rui", publishedAt)

		assert.Equal(t, "ana", d.RightsConfirmation.ConfirmedBy)
	})

	t.Run("clearing removes the confirmation", func(t *testing.T) {
		d := validDraft()
		d.SetRightsConfirmed(true, "ana", confirmedAt)

		d.SetRightsConfirmed(false, "rui", publishedAt)

		assert.Nil(t, d.RightsConfirmation)
	})
}
