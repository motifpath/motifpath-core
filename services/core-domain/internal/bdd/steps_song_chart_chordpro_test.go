//go:build integration

package bdd

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerSongChartChordProSteps(sc *godog.ScenarioContext, w *world) {
	// Given
	sc.Step(`^"([^"]+)" has a song chart "([^"]+)" in "([^"]+)" with a capo on fret (\d+)$`, w.hasSongChartWithCapo)
	sc.Step(`^the rights of "([^"]+)" are confirmed by "([^"]+)"$`, func(title, admin string) error {
		if err := w.setsRightsConfirmed(admin, title, true); err != nil {
			return err
		}
		return w.mustSucceed("confirming the rights of " + title)
	})
	sc.Step(`^the draft of "([^"]+)" is titled "([^"]+)" by "([^"]+)", with one verse "([^"]+)"$`, w.draftTitledWithVerse)

	// When
	sc.Step(`^"([^"]+)" imports into "([^"]+)" the ChordPro:$`, func(admin, title string, text *godog.DocString) error {
		return w.importsChordPro(admin, title, text.Content)
	})
	sc.Step(`^"([^"]+)" imports into "([^"]+)" ChordPro that has no capo directive$`, func(admin, title string) error {
		return w.importsChordPro(admin, title, "{title: Asa Branca}\n{artist: Luiz Gonzaga}\n[G]Quando olhei a terra ardendo\n")
	})
	sc.Step(`^"([^"]+)" imports into "([^"]+)" a line "([^"]+)"$`, func(admin, title, line string) error {
		return w.importsChordPro(admin, title, line+"\n")
	})
	sc.Step(`^"([^"]+)" imports into "([^"]+)" ChordPro whose line 2 is "([^"]+)"$`, func(admin, title, line string) error {
		return w.importsChordPro(admin, title, "{title: Asa Branca}\n"+line+"\n[G]Quando olhei a terra ardendo\n")
	})
	sc.Step(`^"([^"]+)" imports into "([^"]+)" ChordPro that has only directives$`, func(admin, title string) error {
		return w.importsChordPro(admin, title, "{title: Asa Branca}\n{artist: Luiz Gonzaga}\n{capo: 2}\n")
	})
	sc.Step(`^"([^"]+)" exports "([^"]+)" as ChordPro$`, w.exportsChordPro)

	// Then
	sc.Step(`^the draft is titled "([^"]+)" by "([^"]+)"$`, w.draftTitledBy)
	sc.Step(`^the draft has one verse with the line "([^"]+)"$`, w.draftHasOneVerseLine)
	sc.Step(`^the import reported no warnings$`, func() error { return w.importReported(nil) })
	sc.Step(`^the import reported the warning "([^"]+)" on line (\d+)$`, func(kind string, line int) error {
		return w.importReported([]generated.ChordProImportWarning{{Line: line, Kind: generated.ChordProImportWarningKind(kind)}})
	})
	sc.Step(`^the draft still has a capo on fret (\d+)$`, w.draftHasCapo)
	sc.Step(`^the draft's language is still "([^"]+)"$`, w.draftLanguageIs)
	sc.Step(`^the anchors resolve to the catalog chords "([^"]+)" and "([^"]+)"$`, w.anchorsResolveTo)
	sc.Step(`^the import is refused as invalid$`, w.requestRejectedInvalid)
	sc.Step(`^the draft is unchanged$`, w.draftUnchanged)
	sc.Step(`^the draft's rights are still confirmed by "([^"]+)"$`, w.rightsConfirmedBy)
	sc.Step(`^the ChordPro is:$`, func(want *godog.DocString) error { return w.chordProIs(want.Content + "\n") })
}

// ── Given ────────────────────────────────────────────────────────────────────

func (w *world) hasSongChartWithCapo(author, title, language string, capo int) error {
	input, err := draftInput(title, language, chartDocument(defaultChartLine), false)
	if err != nil {
		return err
	}
	input.CapoFret = capo
	if err := w.createChart(w.adminCtx(author), title, input); err != nil {
		return err
	}
	return w.mustSucceed("setting up song chart " + title)
}

func (w *world) draftTitledWithVerse(title, newTitle, artist, line string) error {
	if err := w.changeDraft(songChartSeeder, title, func(input *generated.SongChartDraftInput, doc *domain.SongChartDocument) {
		input.Title, input.Artist = newTitle, artist
		*doc = chartDocument(line)
	}); err != nil {
		return err
	}
	return w.mustSucceed("rewriting " + title)
}

// ── When ─────────────────────────────────────────────────────────────────────

// importsChordPro imports text into the chart, remembering the chart as it
// was so a refused import can be shown to leave it unchanged.
func (w *world) importsChordPro(admin, title, text string) error {
	id, err := w.chartID(title)
	if err != nil {
		return err
	}
	before, err := w.getChart(title)
	if err != nil {
		return err
	}
	w.charts().before = &before
	w.charts().title = title
	w.lastResp, w.lastErr = w.handler.ImportSongChartChordPro(w.identityCtx(admin), generated.ImportSongChartChordProRequestObject{SongChartId: id, Body: &text})
	return w.lastErr
}

func (w *world) exportsChordPro(caller, title string) error {
	id, err := w.chartID(title)
	if err != nil {
		return err
	}
	w.charts().title = title
	w.lastResp, w.lastErr = w.handler.ExportSongChartChordPro(w.identityCtx(caller), generated.ExportSongChartChordProRequestObject{SongChartId: id})
	return w.lastErr
}

// ── Then ─────────────────────────────────────────────────────────────────────

func (w *world) lastImport() (generated.SongChartChordProImport, error) {
	if resp, ok := w.lastResp.(generated.ImportSongChartChordPro200JSONResponse); ok {
		return generated.SongChartChordProImport(resp), nil
	}
	return generated.SongChartChordProImport{}, fmt.Errorf("expected an import, got %#v (err=%v)", w.lastResp, w.lastErr)
}

func (w *world) draftTitledBy(title, artist string) error {
	chart, err := w.lastChart()
	if err != nil {
		return err
	}
	if chart.Draft.Title != title || chart.Draft.Artist != artist {
		return fmt.Errorf("expected %q by %q, got %q by %q", title, artist, chart.Draft.Title, chart.Draft.Artist)
	}
	return nil
}

func (w *world) draftHasOneVerseLine(want string) error {
	chart, err := w.lastChart()
	if err != nil {
		return err
	}
	doc, err := fromGeneratedDocument(chart.Draft.Body)
	if err != nil {
		return err
	}
	if len(doc.Sections) != 1 || doc.Sections[0].Kind != domain.SectionVerse || len(doc.Sections[0].Lines) != 1 {
		return fmt.Errorf("expected one verse of one line, got %+v", doc)
	}
	var text strings.Builder
	for _, run := range doc.Sections[0].Lines[0].Runs {
		text.WriteString(run.Text)
	}
	if text.String() != want {
		return fmt.Errorf("expected the line %q, got %q", want, text.String())
	}
	return nil
}

// importReported checks the import's warnings by kind and line; their
// text is pinned by the golden cases.
func (w *world) importReported(want []generated.ChordProImportWarning) error {
	imported, err := w.lastImport()
	if err != nil {
		return err
	}
	got := imported.ImportWarnings
	if len(got) != len(want) {
		return fmt.Errorf("expected the warnings %+v, got %+v", want, got)
	}
	for i := range want {
		if got[i].Kind != want[i].Kind || got[i].Line != want[i].Line {
			return fmt.Errorf("expected the warnings %+v, got %+v", want, got)
		}
	}
	return nil
}

func (w *world) draftHasCapo(fret int) error {
	chart, err := w.lastChart()
	if err != nil {
		return err
	}
	if chart.Draft.CapoFret != fret {
		return fmt.Errorf("expected a capo on fret %d, got %d", fret, chart.Draft.CapoFret)
	}
	return nil
}

func (w *world) draftLanguageIs(language string) error {
	chart, err := w.lastChart()
	if err != nil {
		return err
	}
	if chart.Draft.Language != language {
		return fmt.Errorf("expected the language %q, got %q", language, chart.Draft.Language)
	}
	return nil
}

func (w *world) anchorsResolveTo(first, second string) error {
	chart, err := w.lastChart()
	if err != nil {
		return err
	}
	doc, err := fromGeneratedDocument(chart.Draft.Body)
	if err != nil {
		return err
	}
	anchors := doc.Anchors()
	if len(anchors) != 2 {
		return fmt.Errorf("expected two anchors, got %+v", anchors)
	}
	for i, symbol := range []string{first, second} {
		if err := resolvesTo(anchors[i].Anchor, &symbol); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) draftUnchanged() error {
	before := w.charts().before
	if before == nil {
		return fmt.Errorf("no chart was remembered before the import")
	}
	after, err := w.getChart(w.charts().title)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(*before, after) {
		return fmt.Errorf("expected the draft unchanged:\nbefore %+v\nafter  %+v", *before, after)
	}
	return nil
}

func (w *world) chordProIs(want string) error {
	got, ok := w.lastResp.(generated.ExportSongChartChordPro200TextResponse)
	if !ok {
		return fmt.Errorf("expected ChordPro text, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if string(got) != want {
		return fmt.Errorf("expected the ChordPro\n%s\ngot\n%s", want, got)
	}
	return nil
}
