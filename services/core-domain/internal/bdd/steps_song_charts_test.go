//go:build integration

package bdd

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// defaultChartLine is the line a chart is written with when a scenario
// doesn't give one.
const defaultChartLine = "[G]Quando olhei a [C]terra ardendo"

// songChartSeeder is the admin who sets charts up in Given steps that name
// no author; publishing isn't tied to who wrote a chart, so who does it
// doesn't matter to those scenarios.
const songChartSeeder = "song-chart-seeder"

// songChartReader is the learner the "learners can read" steps read as.
const songChartReader = "song-chart-learner"

// songChartWorld is what the song chart scenarios remember between steps.
type songChartWorld struct {
	idByTitle map[string]string
	// title is the chart the scenario is about; anchor is the anchor the
	// last step wrote.
	title  string
	anchor string
}

func registerSongChartSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the chord catalog has the chords ((?:"[^"]+"(?:, | and )?)+), each with an active voicing$`, w.catalogHasChordsWithOneVoicing)

	// Setting charts up.
	sc.Step(`^"([^"]+)" has a song chart "([^"]+)"$`, func(author, title string) error { return w.hasSongChart(author, title, false, nil) })
	sc.Step(`^"([^"]+)" has a song chart "([^"]+)" whose rights are confirmed$`, func(author, title string) error { return w.hasSongChart(author, title, true, nil) })
	sc.Step(`^"([^"]+)" has a song chart "([^"]+)" whose rights are not confirmed$`, func(author, title string) error { return w.hasSongChart(author, title, false, nil) })
	sc.Step(`^"([^"]+)" has a song chart "([^"]+)" whose rights are confirmed, with voicing "([^"]+)" picked for an anchor$`, func(author, title, voicing string) error {
		return w.hasSongChart(author, title, true, map[string]string{"G": voicing})
	})
	sc.Step(`^the chart has chord anchors written "([^"]+)" and "([^"]+)"$`, func(a, b string) error {
		return w.rewriteChartLine(fmt.Sprintf("[%s]La [%s]lo", a, b))
	})
	sc.Step(`^the chart has a chord anchor written "([^"]+)"$`, func(symbol string) error { return w.rewriteChartLine(fmt.Sprintf("[%s]La", symbol)) })
	sc.Step(`^the song chart "([^"]+)" is published at revision 1$`, func(title string) error { return w.publishedChart(title, "pt_BR", defaultChartLine) })
	sc.Step(`^the song chart "([^"]+)" in "([^"]+)" is published at revision 1 with the line "([^"]+)"$`, w.publishedChart)
	sc.Step(`^the song chart "([^"]+)" was published at revision 1 and then withdrawn$`, func(title string) error {
		if err := w.publishedChart(title, "pt_BR", defaultChartLine); err != nil {
			return err
		}
		return w.seedWithdrawal(title)
	})
	sc.Step(`^"([^"]+)" has been withdrawn$`, w.seedWithdrawal)
	sc.Step(`^"([^"]+)" was withdrawn and then published again at revision 2$`, func(title string) error {
		if err := w.seedWithdrawal(title); err != nil {
			return err
		}
		return w.seedPublication(title)
	})
	sc.Step(`^the song chart "([^"]+)" has never been published$`, func(title string) error { return w.hasSongChart(songChartSeeder, title, true, nil) })
	sc.Step(`^the anchor written "([^"]+)" in "([^"]+)" picks the voicing "([^"]+)"$`, w.anchorPicksVoicing)
	sc.Step(`^"([^"]+)" has changed the chord on "([^"]+)" in the draft of "([^"]+)" from "([^"]+)" to "([^"]+)"$`, func(author, word, title, _, to string) error {
		return w.changeChordOnWord(author, title, word, to)
	})

	// Authoring.
	sc.Step(`^"([^"]+)" creates a song chart "([^"]+)" in "([^"]+)" with the line "([^"]+)"$`, func(author, title, language, line string) error {
		return w.createsSongChart(author, title, language, line, nil)
	})
	sc.Step(`^"([^"]+)" creates a song chart with a chord anchor written "([^"]+)"$`, func(author, symbol string) error {
		return w.createsSongChart(author, "Asa Branca", "pt_BR", fmt.Sprintf("[%s]La", symbol), nil)
	})
	sc.Step(`^"([^"]+)" creates a song chart with a chord anchor written "([^"]+)" that claims the catalog chord "([^"]+)"$`, func(author, symbol, claimed string) error {
		claim := chordID(claimed).String()
		return w.createsSongChart(author, "Asa Branca", "pt_BR", fmt.Sprintf("[%s]La", symbol), func(a *domain.ChordAnchor) { a.ChordDefinitionID = &claim })
	})
	sc.Step(`^"([^"]+)" tries to create a song chart "([^"]+)"$`, func(author, title string) error {
		return w.createsSongChart(author, title, "pt_BR", defaultChartLine, nil)
	})
	sc.Step(`^"([^"]+)" changes a chord anchor of "([^"]+)" to "([^"]+)"$`, w.changesFirstAnchor)
	sc.Step(`^"([^"]+)" picks a voicing of "([^"]+)" for an anchor written "([^"]+)" in "([^"]+)"$`, w.picksVoicingOfOtherChord)
	sc.Step(`^"([^"]+)" changes the title of the draft of "([^"]+)" to "([^"]+)"$`, w.changesChartTitle)
	sc.Step(`^"([^"]+)" confirms that the rights of "([^"]+)" were checked$`, func(admin, title string) error { return w.setsRightsConfirmed(admin, title, true) })
	sc.Step(`^"([^"]+)" clears the rights confirmation of "([^"]+)"$`, func(admin, title string) error { return w.setsRightsConfirmed(admin, title, false) })
	sc.Step(`^"([^"]+)" publishes "([^"]+)"$`, w.publishesChart)
	sc.Step(`^"([^"]+)" withdraws "([^"]+)" because "([^"]+)"$`, w.withdrawsChart)
	sc.Step(`^"([^"]+)" lists the revisions of "([^"]+)"$`, w.listsChartRevisions)
	sc.Step(`^"([^"]+)" lists song charts$`, func(admin string) error { return w.listsSongCharts(admin, nil) })
	sc.Step(`^"([^"]+)" lists song charts that are published$`, func(admin string) error {
		status := generated.SongChartStatusPublished
		return w.listsSongCharts(admin, &status)
	})
	sc.Step(`^"([^"]+)" previews "([^"]+)"$`, w.previewsChart)

	// Reading.
	sc.Step(`^"([^"]+)" reads the song chart "([^"]+)"$`, w.readsChart)
	sc.Step(`^an unauthenticated request reads the song chart "([^"]+)"$`, w.readsChartUnauthenticated)

	// Outcomes of authoring.
	sc.Step(`^the chart is a draft that has never been published$`, w.chartIsUnpublishedDraft)
	sc.Step(`^the chart is still a draft that has never been published$`, w.chartIsUnpublishedDraft)
	sc.Step(`^the anchor on "([^"]+)" has the written symbol "([^"]+)" and resolves to the catalog chord "([^"]+)"$`, w.anchorOnWordResolves)
	sc.Step(`^the anchor resolves to the catalog chord "([^"]+)"$`, func(symbol string) error { return w.lastAnchorResolvesTo(&symbol) })
	sc.Step(`^the anchor resolves to no chord$`, func() error { return w.lastAnchorResolvesTo(nil) })
	sc.Step(`^the anchor's written symbol is still "([^"]+)"$`, w.lastAnchorWrittenAs)
	sc.Step(`^the anchor keeps the written symbol "([^"]+)" and resolves to no chord$`, func(symbol string) error {
		if err := w.lastAnchorWrittenAs(symbol); err != nil {
			return err
		}
		return w.lastAnchorResolvesTo(nil)
	})
	sc.Step(`^the draft is saved$`, w.draftSaved)
	sc.Step(`^the draft has the warning "([^"]+)" on that anchor, which blocks publication$`, func(kind string) error { return w.draftHasWarning(kind, true) })
	sc.Step(`^the draft has the warning "([^"]+)" on that anchor, which doesn't block publication$`, func(kind string) error { return w.draftHasWarning(kind, false) })
	sc.Step(`^the draft has no warnings$`, w.draftHasNoWarnings)
	sc.Step(`^the draft is refused as invalid$`, w.requestRejectedInvalid)
	sc.Step(`^the rejection identifies the anchor's chordVoicingId as the source of the error$`, w.rejectionIdentifiesVoicingField)
	sc.Step(`^the request is refused because only admins author song charts$`, w.refusedAsNotAdmin)
	sc.Step(`^the draft's rights are confirmed by "([^"]+)", with the time of confirming$`, w.rightsConfirmedBy)
	sc.Step(`^the draft's rights are not confirmed$`, w.rightsNotConfirmed)

	// Outcomes of publishing and withdrawing.
	sc.Step(`^the chart is published at revision (\d+)$`, func(n int) error { return w.chartPublishedAt(n, "") })
	sc.Step(`^the chart is published at revision (\d+), published by "([^"]+)"$`, w.chartPublishedAt)
	sc.Step(`^revision 1 keeps the rights confirmation by "([^"]+)"$`, w.revisionOneConfirmedBy)
	sc.Step(`^learners can read "([^"]+)"$`, func(title string) error { return w.learnersCanRead(title, true) })
	sc.Step(`^learners can't read "([^"]+)"$`, func(title string) error { return w.learnersCanRead(title, false) })
	sc.Step(`^publishing is refused as not publishable, because "([^"]+)"$`, w.publishingRefusedBecause)
	sc.Step(`^the refusal lists the anchors written "([^"]+)" and "([^"]+)"$`, w.refusalListsSymbols)
	sc.Step(`^the refusal lists the anchor with the warning "([^"]+)"$`, w.refusalListsWarning)
	sc.Step(`^revision 1 still has the chord "([^"]+)" on "([^"]+)"$`, w.revisionOneHasChordOn)
	sc.Step(`^the chart's revisions are listed as 2 then 1$`, w.revisionsListedTwoThenOne)
	sc.Step(`^learners still read "([^"]+)" at revision (\d+)$`, func(title string, n int) error { return w.learnersReadRevision(title, n, "") })
	sc.Step(`^learners still read "([^"]+)" at revision (\d+), titled "([^"]+)"$`, w.learnersReadRevision)
	sc.Step(`^no revisions are listed$`, w.noRevisionsListed)
	sc.Step(`^the chart is withdrawn by "([^"]+)" because "([^"]+)"$`, w.chartWithdrawnBy)
	sc.Step(`^revision 1 is still listed$`, w.revisionOneStillListed)
	sc.Step(`^the request is refused because the chart is not published$`, w.refusedAsNotPublished)
	sc.Step(`^the list holds "([^"]+)" only$`, w.chartListHoldsOnly)
	sc.Step(`^the preview holds the draft's lyrics and chords, with no revision number$`, w.previewHoldsDraft)
	sc.Step(`^it includes the catalog chords "([^"]+)" and "([^"]+)" with their voicings and diagrams$`, w.learnerChartIncludesChords)

	// Outcomes of reading.
	sc.Step(`^"([^"]+)" gets revision (\d+) of "([^"]+)", with the line "([^"]+)"$`, w.getsRevisionWithLine)
	sc.Step(`^"([^"]+)" gets revision (\d+) of "([^"]+)"$`, func(reader string, n int, title string) error {
		return w.getsRevisionWithLine(reader, n, title, "")
	})
	sc.Step(`^the chord anchors are written "([^"]+)" on "([^"]+)" and "([^"]+)" on "([^"]+)"$`, w.chordAnchorsWrittenOn)
	sc.Step(`^the chords included are "([^"]+)" with voicings "([^"]+)" then "([^"]+)", and "([^"]+)" with voicing "([^"]+)"$`, func(first, v1, v2, second, v3 string) error {
		return w.chordsIncluded([]string{first, second}, map[string][]string{first: {v1, v2}, second: {v3}})
	})
	sc.Step(`^the chord "([^"]+)" is included with voicings "([^"]+)" then "([^"]+)"$`, func(symbol, v1, v2 string) error {
		return w.chordIncludedWithVoicings(symbol, v1, v2)
	})
	sc.Step(`^the diagram of every included voicing is included$`, w.everyVoicingDiagramIncluded)
	sc.Step(`^the chart "([^"]+)" gets holds no rights confirmation and no author or publisher$`, w.learnerChartHoldsNoAuthorship)
	sc.Step(`^the chart is not found$`, w.chartNotFound)
}

// ── Building documents ───────────────────────────────────────────────────────

// chartLine reads a ChordPro-style line, "[G]Quando olhei a [C]terra", into
// lyric runs whose anchors are numbered a1, a2, … in order. It is only a
// fixture shorthand, not the ChordPro import.
func chartLine(line string) []domain.SongChartRun {
	var runs []domain.SongChartRun
	n := 0
	for len(line) > 0 {
		open := strings.Index(line, "[")
		if open != 0 {
			text := line
			if open > 0 {
				text = line[:open]
			}
			runs = append(runs, domain.SongChartRun{Text: text})
			if open < 0 {
				break
			}
			line = line[open:]
			continue
		}
		end := strings.Index(line, "]")
		symbol := line[1:end]
		line = line[end+1:]
		next := strings.Index(line, "[")
		text := line
		if next >= 0 {
			text = line[:next]
		}
		line = line[len(text):]
		if text == "" {
			text = " "
		}
		n++
		runs = append(runs, domain.SongChartRun{Text: text, Anchor: &domain.ChordAnchor{ID: fmt.Sprintf("a%d", n), WrittenSymbol: symbol}})
	}
	return runs
}

func chartDocument(line string) domain.SongChartDocument {
	return domain.SongChartDocument{Sections: []domain.SongChartSection{{Kind: domain.SectionVerse, Lines: []domain.SongChartLine{{Runs: chartLine(line)}}}}}
}

func toGeneratedDocument(doc domain.SongChartDocument) (generated.SongChartDocument, error) {
	raw, err := json.Marshal(doc)
	if err != nil {
		return generated.SongChartDocument{}, err
	}
	var out generated.SongChartDocument
	return out, json.Unmarshal(raw, &out)
}

func fromGeneratedDocument(doc generated.SongChartDocument) (domain.SongChartDocument, error) {
	raw, err := json.Marshal(doc)
	if err != nil {
		return domain.SongChartDocument{}, err
	}
	return domain.ParseSongChartDocument(raw)
}

// draftInput is the draft a chart is written with: title, language, a body
// and the rights confirmation, the rest left unset.
func draftInput(title, language string, doc domain.SongChartDocument, rightsConfirmed bool) (generated.SongChartDraftInput, error) {
	body, err := toGeneratedDocument(doc)
	return generated.SongChartDraftInput{Title: title, Artist: "Luiz Gonzaga", Language: language, Body: body, RightsConfirmed: rightsConfirmed}, err
}

// ── Talking to the API ───────────────────────────────────────────────────────

func (w *world) charts() *songChartWorld {
	if w.songCharts == nil {
		w.songCharts = &songChartWorld{idByTitle: map[string]string{}}
	}
	return w.songCharts
}

func (w *world) chartID(title string) (uuid.UUID, error) {
	id, ok := w.charts().idByTitle[title]
	if !ok {
		return uuid.UUID{}, fmt.Errorf("no song chart %q in this scenario", title)
	}
	return uuid.MustParse(id), nil
}

// adminCtx is the context of a request made by admin, registering them.
func (w *world) adminCtx(admin string) context.Context {
	w.ensureRegistered(admin, domain.RoleAdmin)
	return w.identityCtx(admin)
}

// getChart reads a chart as the seeding admin, outside the scenario's own
// requests.
func (w *world) getChart(title string) (generated.SongChart, error) {
	id, err := w.chartID(title)
	if err != nil {
		return generated.SongChart{}, err
	}
	resp, err := w.handler.GetSongChart(w.adminCtx(songChartSeeder), generated.GetSongChartRequestObject{SongChartId: id})
	if err != nil {
		return generated.SongChart{}, err
	}
	chart, ok := resp.(generated.GetSongChart200JSONResponse)
	if !ok {
		return generated.SongChart{}, fmt.Errorf("reading %q: %#v", title, resp)
	}
	return generated.SongChart(chart), nil
}

// inputOf is the chart's current draft as an input to change.
func (w *world) inputOf(title string) (generated.SongChartDraftInput, error) {
	chart, err := w.getChart(title)
	if err != nil {
		return generated.SongChartDraftInput{}, err
	}
	d := chart.Draft
	return generated.SongChartDraftInput{
		Title: d.Title, Artist: d.Artist, Language: d.Language, ConcertKey: d.ConcertKey, CapoFret: d.CapoFret,
		TempoBpm: d.TempoBpm, TimeSignature: d.TimeSignature, RightsConfirmed: d.RightsConfirmation != nil, Body: d.Body,
	}, nil
}

func (w *world) createChart(ctx context.Context, title string, input generated.SongChartDraftInput) error {
	resp, err := w.handler.CreateSongChart(ctx, generated.CreateSongChartRequestObject{Body: &input})
	w.lastResp, w.lastErr = resp, err
	if err != nil {
		return err
	}
	if created, ok := resp.(generated.CreateSongChart201JSONResponse); ok {
		w.charts().idByTitle[title] = created.SongChartId.String()
	}
	w.charts().title = title
	return nil
}

func (w *world) updateChart(ctx context.Context, title string, input generated.SongChartDraftInput) error {
	id, err := w.chartID(title)
	if err != nil {
		return err
	}
	resp, err := w.handler.UpdateSongChartDraft(ctx, generated.UpdateSongChartDraftRequestObject{SongChartId: id, Body: &input})
	w.lastResp, w.lastErr = resp, err
	w.charts().title = title
	return err
}

// changeDraft applies change to the chart's current draft and saves it as
// admin.
func (w *world) changeDraft(admin, title string, change func(*generated.SongChartDraftInput, *domain.SongChartDocument)) error {
	input, err := w.inputOf(title)
	if err != nil {
		return err
	}
	doc, err := fromGeneratedDocument(input.Body)
	if err != nil {
		return err
	}
	change(&input, &doc)
	if input.Body, err = toGeneratedDocument(doc); err != nil {
		return err
	}
	return w.updateChart(w.adminCtx(admin), title, input)
}

// mustSucceed fails a Given step whose request didn't do what it set up.
func (w *world) mustSucceed(what string) error {
	switch w.lastResp.(type) {
	case generated.CreateSongChart201JSONResponse, generated.UpdateSongChartDraft200JSONResponse,
		generated.PublishSongChart201JSONResponse, generated.WithdrawSongChart200JSONResponse:
		return nil
	}
	return fmt.Errorf("%s: %#v (err=%v)", what, w.lastResp, w.lastErr)
}

// ── Given ────────────────────────────────────────────────────────────────────

// openVoicingKey names the one voicing a chord seeded without a table gets:
// "g-open", "d-over-f#-open".
func openVoicingKey(symbol string) string {
	return strings.ToLower(strings.ReplaceAll(symbol, "/", "-over-")) + "-open"
}

func (w *world) catalogHasChordsWithOneVoicing(list string) error {
	for _, match := range quotedName.FindAllStringSubmatch(list, -1) {
		symbol := match[1]
		if err := w.seedCatalogChord(symbol, openVoicingKey(symbol)); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) hasSongChart(author, title string, rightsConfirmed bool, picks map[string]string) error {
	doc := chartDocument(defaultChartLine).WithAnchors(func(a domain.ChordAnchor) domain.ChordAnchor {
		if key, ok := picks[a.WrittenSymbol]; ok {
			id := voicingID(key).String()
			a.ChordVoicingID = &id
		}
		return a
	})
	input, err := draftInput(title, "pt_BR", doc, rightsConfirmed)
	if err != nil {
		return err
	}
	if err := w.createChart(w.adminCtx(author), title, input); err != nil {
		return err
	}
	return w.mustSucceed("setting up song chart " + title)
}

func (w *world) rewriteChartLine(line string) error {
	title := w.charts().title
	if err := w.changeDraft(songChartSeeder, title, func(_ *generated.SongChartDraftInput, doc *domain.SongChartDocument) {
		*doc = chartDocument(line)
	}); err != nil {
		return err
	}
	return w.mustSucceed("rewriting " + title)
}

func (w *world) publishedChart(title, language, line string) error {
	input, err := draftInput(title, language, chartDocument(line), true)
	if err != nil {
		return err
	}
	if err := w.createChart(w.adminCtx(songChartSeeder), title, input); err != nil {
		return err
	}
	if err := w.mustSucceed("creating " + title); err != nil {
		return err
	}
	return w.seedPublication(title)
}

func (w *world) seedPublication(title string) error {
	if err := w.publishesChartAs(songChartSeeder, title); err != nil {
		return err
	}
	return w.mustSucceed("publishing " + title)
}

func (w *world) seedWithdrawal(title string) error {
	if err := w.withdrawsChart(songChartSeeder, title, "Set up by the scenario"); err != nil {
		return err
	}
	return w.mustSucceed("withdrawing " + title)
}

func (w *world) anchorPicksVoicing(symbol, title, voicing string) error {
	id := voicingID(voicing).String()
	if err := w.changeDraft(songChartSeeder, title, func(_ *generated.SongChartDraftInput, doc *domain.SongChartDocument) {
		*doc = doc.WithAnchors(func(a domain.ChordAnchor) domain.ChordAnchor {
			if a.WrittenSymbol == symbol {
				a.ChordVoicingID = &id
			}
			return a
		})
	}); err != nil {
		return err
	}
	return w.seedPublication(title)
}

func (w *world) changeChordOnWord(author, title, word, to string) error {
	if err := w.changeDraft(author, title, func(_ *generated.SongChartDraftInput, doc *domain.SongChartDocument) {
		*doc = withChordOn(*doc, word, to)
	}); err != nil {
		return err
	}
	return w.mustSucceed("changing " + title)
}

// withChordOn rewrites the anchor on the run starting with word.
func withChordOn(doc domain.SongChartDocument, word, symbol string) domain.SongChartDocument {
	for si, section := range doc.Sections {
		for li, line := range section.Lines {
			for ri, run := range line.Runs {
				if run.Anchor != nil && strings.HasPrefix(run.Text, word) {
					anchor := *run.Anchor
					anchor.WrittenSymbol, anchor.ChordVoicingID = symbol, nil
					doc.Sections[si].Lines[li].Runs[ri].Anchor = &anchor
				}
			}
		}
	}
	return doc
}

// ── When ─────────────────────────────────────────────────────────────────────

func (w *world) createsSongChart(author, title, language, line string, change func(*domain.ChordAnchor)) error {
	doc := chartDocument(line)
	if change != nil {
		doc = doc.WithAnchors(func(a domain.ChordAnchor) domain.ChordAnchor { change(&a); return a })
	}
	input, err := draftInput(title, language, doc, false)
	if err != nil {
		return err
	}
	w.charts().anchor = "a1"
	return w.createChart(w.identityCtx(author), title, input)
}

func (w *world) changesFirstAnchor(admin, title, symbol string) error {
	w.charts().anchor = "a1"
	return w.changeDraft(admin, title, func(_ *generated.SongChartDraftInput, doc *domain.SongChartDocument) {
		*doc = doc.WithAnchors(func(a domain.ChordAnchor) domain.ChordAnchor {
			if a.ID == "a1" {
				a.WrittenSymbol, a.ChordVoicingID = symbol, nil
			}
			return a
		})
	})
}

func (w *world) picksVoicingOfOtherChord(admin, chord, symbol, title string) error {
	id := voicingID(openVoicingKey(chord)).String()
	return w.changeDraft(admin, title, func(_ *generated.SongChartDraftInput, doc *domain.SongChartDocument) {
		*doc = doc.WithAnchors(func(a domain.ChordAnchor) domain.ChordAnchor {
			if a.WrittenSymbol == symbol {
				a.ChordVoicingID = &id
			}
			return a
		})
	})
}

func (w *world) changesChartTitle(admin, title, newTitle string) error {
	return w.changeDraft(admin, title, func(input *generated.SongChartDraftInput, _ *domain.SongChartDocument) {
		input.Title = newTitle
	})
}

func (w *world) setsRightsConfirmed(admin, title string, confirmed bool) error {
	return w.changeDraft(admin, title, func(input *generated.SongChartDraftInput, _ *domain.SongChartDocument) {
		input.RightsConfirmed = confirmed
	})
}

func (w *world) publishesChart(admin, title string) error {
	return w.publishesChartAs(admin, title)
}

func (w *world) publishesChartAs(caller, title string) error {
	id, err := w.chartID(title)
	if err != nil {
		return err
	}
	ctx := w.identityCtx(caller)
	if caller == songChartSeeder {
		ctx = w.adminCtx(caller)
	}
	resp, err := w.handler.PublishSongChart(ctx, generated.PublishSongChartRequestObject{SongChartId: id})
	w.lastResp, w.lastErr = resp, err
	w.charts().title = title
	return err
}

func (w *world) withdrawsChart(admin, title, reason string) error {
	id, err := w.chartID(title)
	if err != nil {
		return err
	}
	resp, err := w.handler.WithdrawSongChart(w.adminCtx(admin), generated.WithdrawSongChartRequestObject{SongChartId: id, Body: &generated.WithdrawSongChartRequest{Reason: reason}})
	w.lastResp, w.lastErr = resp, err
	w.charts().title = title
	return err
}

func (w *world) listsChartRevisions(admin, title string) error {
	id, err := w.chartID(title)
	if err != nil {
		return err
	}
	resp, err := w.handler.ListSongChartRevisions(w.identityCtx(admin), generated.ListSongChartRevisionsRequestObject{SongChartId: id})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) listsSongCharts(caller string, status *generated.SongChartStatus) error {
	resp, err := w.handler.ListSongCharts(w.identityCtx(caller), generated.ListSongChartsRequestObject{Params: generated.ListSongChartsParams{Status: status}})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) previewsChart(admin, title string) error {
	id, err := w.chartID(title)
	if err != nil {
		return err
	}
	resp, err := w.handler.PreviewSongChart(w.identityCtx(admin), generated.PreviewSongChartRequestObject{SongChartId: id})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) readsChart(reader, title string) error {
	id, err := w.chartID(title)
	if err != nil {
		return err
	}
	resp, err := w.handler.GetPublishedSongChart(w.identityCtx(reader), generated.GetPublishedSongChartRequestObject{SongChartId: id})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) readsChartUnauthenticated(title string) error {
	id, err := w.chartID(title)
	if err != nil {
		return err
	}
	resp, err := w.handler.GetPublishedSongChart(context.Background(), generated.GetPublishedSongChartRequestObject{SongChartId: id})
	w.lastResp, w.lastErr = resp, err
	return err
}

// ── Then: authoring ──────────────────────────────────────────────────────────

// lastChart is the chart the last authoring request returned.
func (w *world) lastChart() (generated.SongChart, error) {
	switch resp := w.lastResp.(type) {
	case generated.CreateSongChart201JSONResponse:
		return generated.SongChart(resp), nil
	case generated.UpdateSongChartDraft200JSONResponse:
		return generated.SongChart(resp), nil
	case generated.WithdrawSongChart200JSONResponse:
		return generated.SongChart(resp), nil
	}
	return generated.SongChart{}, fmt.Errorf("expected a song chart, got %#v (err=%v)", w.lastResp, w.lastErr)
}

func (w *world) chartIsUnpublishedDraft() error {
	chart, err := w.getChart(w.charts().title)
	if err != nil {
		return err
	}
	if chart.Status != generated.SongChartStatusDraft || chart.PublishedRevision != nil {
		return fmt.Errorf("expected a never-published draft, got status %q, revision %+v", chart.Status, chart.PublishedRevision)
	}
	return nil
}

// anchorOf finds an anchor of the last returned chart's draft.
func (w *world) anchorOf(match func(domain.SongChartRun) bool) (domain.ChordAnchor, error) {
	chart, err := w.lastChart()
	if err != nil {
		return domain.ChordAnchor{}, err
	}
	doc, err := fromGeneratedDocument(chart.Draft.Body)
	if err != nil {
		return domain.ChordAnchor{}, err
	}
	for _, section := range doc.Sections {
		for _, line := range section.Lines {
			for _, run := range line.Runs {
				if run.Anchor != nil && match(run) {
					return *run.Anchor, nil
				}
			}
		}
	}
	return domain.ChordAnchor{}, fmt.Errorf("no such anchor in %+v", doc)
}

func (w *world) lastAnchor() (domain.ChordAnchor, error) {
	id := w.charts().anchor
	return w.anchorOf(func(r domain.SongChartRun) bool { return r.Anchor.ID == id })
}

func (w *world) anchorOnWordResolves(text, symbol, chord string) error {
	anchor, err := w.anchorOf(func(r domain.SongChartRun) bool { return r.Text == text })
	if err != nil {
		return err
	}
	if anchor.WrittenSymbol != symbol {
		return fmt.Errorf("expected %q written on %q, got %q", symbol, text, anchor.WrittenSymbol)
	}
	return resolvesTo(anchor, &chord)
}

func resolvesTo(anchor domain.ChordAnchor, symbol *string) error {
	switch {
	case symbol == nil && anchor.ChordDefinitionID != nil:
		return fmt.Errorf("expected %q to resolve to no chord, got %s", anchor.WrittenSymbol, *anchor.ChordDefinitionID)
	case symbol != nil && (anchor.ChordDefinitionID == nil || *anchor.ChordDefinitionID != chordID(*symbol).String()):
		return fmt.Errorf("expected %q to resolve to the catalog chord %q, got %v", anchor.WrittenSymbol, *symbol, anchor.ChordDefinitionID)
	}
	return nil
}

func (w *world) lastAnchorResolvesTo(symbol *string) error {
	anchor, err := w.lastAnchor()
	if err != nil {
		return err
	}
	return resolvesTo(anchor, symbol)
}

func (w *world) lastAnchorWrittenAs(symbol string) error {
	anchor, err := w.lastAnchor()
	if err != nil {
		return err
	}
	if anchor.WrittenSymbol != symbol {
		return fmt.Errorf("expected the written symbol %q, got %q", symbol, anchor.WrittenSymbol)
	}
	return nil
}

func (w *world) draftSaved() error {
	_, err := w.lastChart()
	return err
}

func (w *world) draftHasWarning(kind string, blocks bool) error {
	chart, err := w.lastChart()
	if err != nil {
		return err
	}
	for _, warning := range chart.Draft.Warnings {
		if warning.AnchorId == w.charts().anchor {
			if string(warning.Warning) != kind || warning.BlocksPublication != blocks {
				return fmt.Errorf("expected the warning %q (blocks: %v), got %+v", kind, blocks, warning)
			}
			return nil
		}
	}
	return fmt.Errorf("expected a warning on anchor %s, got %+v", w.charts().anchor, chart.Draft.Warnings)
}

func (w *world) draftHasNoWarnings() error {
	chart, err := w.lastChart()
	if err != nil {
		return err
	}
	if len(chart.Draft.Warnings) > 0 {
		return fmt.Errorf("expected no warnings, got %+v", chart.Draft.Warnings)
	}
	return nil
}

func (w *world) rejectionIdentifiesVoicingField() error {
	resp, ok := w.lastResp.(generated.UpdateSongChartDraft400JSONResponse)
	if !ok {
		return fmt.Errorf("expected the draft refused as invalid, got %#v", w.lastResp)
	}
	for _, e := range resp.Errors {
		if strings.HasSuffix(e.Field, "/chordVoicingId") {
			return nil
		}
	}
	return fmt.Errorf("expected the anchor's chordVoicingId to be named, got %+v", resp.Errors)
}

func (w *world) refusedAsNotAdmin() error {
	switch w.lastResp.(type) {
	case generated.CreateSongChart403JSONResponse, generated.PublishSongChart403JSONResponse, generated.ListSongCharts403JSONResponse:
		return nil
	}
	return fmt.Errorf("expected a refusal because only admins author song charts, got %#v (err=%v)", w.lastResp, w.lastErr)
}

func (w *world) rightsConfirmedBy(admin string) error {
	chart, err := w.lastChart()
	if err != nil {
		return err
	}
	c := chart.Draft.RightsConfirmation
	if c == nil || c.ConfirmedBy.UserId != w.ensureRegistered(admin, domain.RoleAdmin) || !c.ConfirmedAt.Equal(fixedNow) {
		return fmt.Errorf("expected the rights confirmed by %q at %v, got %+v", admin, fixedNow, c)
	}
	return nil
}

func (w *world) rightsNotConfirmed() error {
	chart, err := w.lastChart()
	if err != nil {
		return err
	}
	if chart.Draft.RightsConfirmation != nil || chart.Draft.RightsConfirmed {
		return fmt.Errorf("expected the rights not confirmed, got %+v", chart.Draft.RightsConfirmation)
	}
	return nil
}

// ── Then: publishing ─────────────────────────────────────────────────────────

func (w *world) chartPublishedAt(n int, publisher string) error {
	rev, ok := w.lastResp.(generated.PublishSongChart201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a published revision, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if rev.RevisionNumber != n {
		return fmt.Errorf("expected revision %d, got %d", n, rev.RevisionNumber)
	}
	if publisher != "" && rev.PublishedBy.UserId != w.ensureRegistered(publisher, domain.RoleAdmin) {
		return fmt.Errorf("expected revision %d published by %q, got %+v", n, publisher, rev.PublishedBy)
	}
	chart, err := w.getChart(w.charts().title)
	if err != nil {
		return err
	}
	if chart.Status != generated.SongChartStatusPublished || chart.PublishedRevision == nil || chart.PublishedRevision.RevisionNumber != n {
		return fmt.Errorf("expected the chart published at revision %d, got %q %+v", n, chart.Status, chart.PublishedRevision)
	}
	return nil
}

func (w *world) revisions(title string) ([]generated.SongChartRevision, error) {
	id, err := w.chartID(title)
	if err != nil {
		return nil, err
	}
	resp, err := w.handler.ListSongChartRevisions(w.adminCtx(songChartSeeder), generated.ListSongChartRevisionsRequestObject{SongChartId: id})
	if err != nil {
		return nil, err
	}
	revs, ok := resp.(generated.ListSongChartRevisions200JSONResponse)
	if !ok {
		return nil, fmt.Errorf("listing the revisions of %q: %#v", title, resp)
	}
	return revs, nil
}

func (w *world) revisionOneConfirmedBy(admin string) error {
	revs, err := w.revisions(w.charts().title)
	if err != nil {
		return err
	}
	first := revs[len(revs)-1]
	if first.RevisionNumber != 1 || first.RightsConfirmation.ConfirmedBy.UserId != w.ensureRegistered(admin, domain.RoleAdmin) {
		return fmt.Errorf("expected revision 1 confirmed by %q, got %+v", admin, first.RightsConfirmation)
	}
	return nil
}

// readAsLearner reads a chart as a student, outside the scenario's own
// requests.
func (w *world) readAsLearner(title string) (generated.GetPublishedSongChartResponseObject, error) {
	id, err := w.chartID(title)
	if err != nil {
		return nil, err
	}
	w.ensureRegistered(songChartReader, domain.RoleStudent)
	return w.handler.GetPublishedSongChart(w.identityCtx(songChartReader), generated.GetPublishedSongChartRequestObject{SongChartId: id})
}

func (w *world) learnersCanRead(title string, can bool) error {
	resp, err := w.readAsLearner(title)
	if err != nil {
		return err
	}
	_, read := resp.(generated.GetPublishedSongChart200JSONResponse)
	if read != can {
		return fmt.Errorf("expected learners able to read %q: %v, got %#v", title, can, resp)
	}
	return nil
}

func (w *world) learnersReadRevision(title string, n int, wantTitle string) error {
	resp, err := w.readAsLearner(title)
	if err != nil {
		return err
	}
	chart, ok := resp.(generated.GetPublishedSongChart200JSONResponse)
	if !ok || chart.RevisionNumber == nil || *chart.RevisionNumber != n {
		return fmt.Errorf("expected learners to read revision %d of %q, got %#v", n, title, resp)
	}
	if wantTitle != "" && chart.Title != wantTitle {
		return fmt.Errorf("expected learners to read the title %q, got %q", wantTitle, chart.Title)
	}
	return nil
}

func (w *world) notPublishable() (generated.PublishSongChart409JSONResponse, error) {
	resp, ok := w.lastResp.(generated.PublishSongChart409JSONResponse)
	if !ok {
		return resp, fmt.Errorf("expected publishing refused as not publishable, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return resp, nil
}

func (w *world) publishingRefusedBecause(reason string) error {
	resp, err := w.notPublishable()
	if err != nil {
		return err
	}
	for _, r := range resp.Reasons {
		if string(r) == reason {
			return nil
		}
	}
	return fmt.Errorf("expected the reason %q, got %v", reason, resp.Reasons)
}

func (w *world) refusalListsSymbols(a, b string) error {
	resp, err := w.notPublishable()
	if err != nil {
		return err
	}
	var symbols []string
	for _, warning := range resp.AnchorWarnings {
		symbols = append(symbols, warning.WrittenSymbol)
	}
	if !slices.Equal(symbols, []string{a, b}) {
		return fmt.Errorf("expected the anchors %q and %q listed, got %v", a, b, symbols)
	}
	return nil
}

func (w *world) refusalListsWarning(kind string) error {
	resp, err := w.notPublishable()
	if err != nil {
		return err
	}
	for _, warning := range resp.AnchorWarnings {
		if string(warning.Warning) == kind {
			return nil
		}
	}
	return fmt.Errorf("expected an anchor with the warning %q, got %+v", kind, resp.AnchorWarnings)
}

func (w *world) revisionOneHasChordOn(symbol, word string) error {
	revs, err := w.revisions(w.charts().title)
	if err != nil {
		return err
	}
	doc, err := fromGeneratedDocument(revs[len(revs)-1].Body)
	if err != nil {
		return err
	}
	for _, a := range doc.Anchors() {
		run := doc.Sections[a.Position.SectionIndex].Lines[a.Position.LineIndex]
		for _, r := range run.Runs {
			if r.Anchor != nil && r.Anchor.ID == a.Anchor.ID && strings.HasPrefix(r.Text, word) {
				if a.Anchor.WrittenSymbol != symbol {
					return fmt.Errorf("expected revision 1 to have %q on %q, got %q", symbol, word, a.Anchor.WrittenSymbol)
				}
				return nil
			}
		}
	}
	return fmt.Errorf("no chord on %q in revision 1", word)
}

func (w *world) revisionsListedTwoThenOne() error {
	revs, err := w.revisions(w.charts().title)
	if err != nil {
		return err
	}
	if len(revs) != 2 || revs[0].RevisionNumber != 2 || revs[1].RevisionNumber != 1 {
		return fmt.Errorf("expected revisions 2 then 1, got %d revisions", len(revs))
	}
	return nil
}

func (w *world) noRevisionsListed() error {
	revs, ok := w.lastResp.(generated.ListSongChartRevisions200JSONResponse)
	if !ok || len(revs) != 0 {
		return fmt.Errorf("expected no revisions, got %#v", w.lastResp)
	}
	return nil
}

func (w *world) chartWithdrawnBy(admin, reason string) error {
	chart, err := w.lastChart()
	if err != nil {
		return err
	}
	wd := chart.Withdrawal
	if chart.Status != generated.SongChartStatusWithdrawn || wd == nil || wd.WithdrawnBy.UserId != w.ensureRegistered(admin, domain.RoleAdmin) || wd.Reason != reason {
		return fmt.Errorf("expected the chart withdrawn by %q because %q, got %q %+v", admin, reason, chart.Status, wd)
	}
	return nil
}

func (w *world) revisionOneStillListed() error {
	revs, err := w.revisions(w.charts().title)
	if err != nil {
		return err
	}
	if len(revs) != 1 || revs[0].RevisionNumber != 1 {
		return fmt.Errorf("expected revision 1 still listed, got %d revisions", len(revs))
	}
	return nil
}

func (w *world) refusedAsNotPublished() error {
	if _, ok := w.lastResp.(generated.WithdrawSongChart409JSONResponse); !ok {
		return fmt.Errorf("expected a refusal because the chart is not published, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}

func (w *world) chartListHoldsOnly(title string) error {
	resp, ok := w.lastResp.(generated.ListSongCharts200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a list of song charts, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if len(resp.Items) != 1 || resp.Items[0].Title != title || resp.Total != 1 {
		return fmt.Errorf("expected only %q, got %+v", title, resp.Items)
	}
	return nil
}

// ── Then: reading ────────────────────────────────────────────────────────────

func (w *world) learnerChart() (generated.LearnerSongChart, error) {
	switch resp := w.lastResp.(type) {
	case generated.GetPublishedSongChart200JSONResponse:
		return generated.LearnerSongChart(resp), nil
	case generated.PreviewSongChart200JSONResponse:
		return generated.LearnerSongChart(resp), nil
	}
	return generated.LearnerSongChart{}, fmt.Errorf("expected a song chart to read, got %#v (err=%v)", w.lastResp, w.lastErr)
}

func (w *world) previewHoldsDraft() error {
	chart, err := w.learnerChart()
	if err != nil {
		return err
	}
	if _, ok := w.lastResp.(generated.PreviewSongChart200JSONResponse); !ok || chart.RevisionNumber != nil {
		return fmt.Errorf("expected a preview with no revision number, got %#v", w.lastResp)
	}
	draft, err := w.getChart(w.charts().title)
	if err != nil {
		return err
	}
	got, _ := json.Marshal(chart.Body)
	want, _ := json.Marshal(draft.Draft.Body)
	if string(got) != string(want) {
		return fmt.Errorf("expected the preview to hold the draft's body\n got %s\nwant %s", got, want)
	}
	return nil
}

func (w *world) includedChord(symbol string) (generated.ChordDefinition, error) {
	chart, err := w.learnerChart()
	if err != nil {
		return generated.ChordDefinition{}, err
	}
	for _, c := range chart.Chords {
		if c.ChordDefinitionId == chordID(symbol) {
			return c, nil
		}
	}
	return generated.ChordDefinition{}, fmt.Errorf("expected the chord %q included, got %d chords", symbol, len(chart.Chords))
}

func (w *world) learnerChartIncludesChords(a, b string) error {
	for _, symbol := range []string{a, b} {
		if _, err := w.includedChord(symbol); err != nil {
			return err
		}
	}
	return w.everyVoicingDiagramIncluded()
}

func (w *world) getsRevisionWithLine(_ string, n int, title, line string) error {
	chart, err := w.learnerChart()
	if err != nil {
		return err
	}
	if chart.RevisionNumber == nil || *chart.RevisionNumber != n || chart.Title != title {
		return fmt.Errorf("expected revision %d of %q, got revision %v of %q", n, title, chart.RevisionNumber, chart.Title)
	}
	if line == "" {
		return nil
	}
	doc, err := fromGeneratedDocument(chart.Body)
	if err != nil {
		return err
	}
	var text string
	for _, r := range doc.Sections[0].Lines[0].Runs {
		text += r.Text
	}
	if text != line {
		return fmt.Errorf("expected the line %q, got %q", line, text)
	}
	return nil
}

func (w *world) chordAnchorsWrittenOn(symbolA, textA, symbolB, textB string) error {
	chart, err := w.learnerChart()
	if err != nil {
		return err
	}
	doc, err := fromGeneratedDocument(chart.Body)
	if err != nil {
		return err
	}
	runs := doc.Sections[0].Lines[0].Runs
	want := []struct{ symbol, text string }{{symbolA, textA}, {symbolB, textB}}
	for i, wnt := range want {
		if i >= len(runs) || runs[i].Anchor == nil || runs[i].Anchor.WrittenSymbol != wnt.symbol || runs[i].Text != wnt.text {
			return fmt.Errorf("expected %q on %q, got %+v", wnt.symbol, wnt.text, runs)
		}
	}
	return nil
}

func voicingKeysOf(c generated.ChordDefinition) []string {
	ids := make([]string, len(c.Voicings))
	for i, v := range c.Voicings {
		ids[i] = v.ChordVoicingId.String()
	}
	return ids
}

func voicingIDsFor(keys ...string) []string {
	ids := make([]string, len(keys))
	for i, k := range keys {
		ids[i] = voicingID(k).String()
	}
	return ids
}

func (w *world) chordsIncluded(order []string, voicings map[string][]string) error {
	chart, err := w.learnerChart()
	if err != nil {
		return err
	}
	if len(chart.Chords) != len(order) {
		return fmt.Errorf("expected %d chords, got %d", len(order), len(chart.Chords))
	}
	for i, symbol := range order {
		c := chart.Chords[i]
		if c.ChordDefinitionId != chordID(symbol) {
			return fmt.Errorf("expected chord %d to be %q, got %s", i+1, symbol, c.CanonicalSymbol)
		}
		if got := voicingKeysOf(c); !slices.Equal(got, voicingIDsFor(voicings[symbol]...)) {
			return fmt.Errorf("expected %q's voicings %v, got %v", symbol, voicings[symbol], got)
		}
	}
	return nil
}

func (w *world) chordIncludedWithVoicings(symbol string, keys ...string) error {
	c, err := w.includedChord(symbol)
	if err != nil {
		return err
	}
	if got := voicingKeysOf(c); !slices.Equal(got, voicingIDsFor(keys...)) {
		return fmt.Errorf("expected %q's voicings %v, got %v", symbol, keys, got)
	}
	return nil
}

func (w *world) everyVoicingDiagramIncluded() error {
	chart, err := w.learnerChart()
	if err != nil {
		return err
	}
	diagrams := map[uuid.UUID]bool{}
	for _, d := range chart.Diagrams {
		diagrams[d.DiagramId] = true
	}
	for _, c := range chart.Chords {
		for _, v := range c.Voicings {
			if !diagrams[v.DiagramId] {
				return fmt.Errorf("expected the diagram of voicing %s included", v.ChordVoicingId)
			}
		}
	}
	return nil
}

func (w *world) learnerChartHoldsNoAuthorship(string) error {
	chart, err := w.learnerChart()
	if err != nil {
		return err
	}
	raw, err := json.Marshal(chart)
	if err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	for _, key := range []string{"rights_confirmation", "rights_confirmed", "created_by", "published_by", "updated_by"} {
		if _, ok := fields[key]; ok {
			return fmt.Errorf("expected the learner's chart to hold no %q", key)
		}
	}
	return nil
}

func (w *world) chartNotFound() error {
	if _, ok := w.lastResp.(generated.GetPublishedSongChart404JSONResponse); !ok {
		return fmt.Errorf("expected the chart not found, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}
