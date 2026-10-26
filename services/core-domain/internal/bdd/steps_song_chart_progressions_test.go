//go:build integration

package bdd

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// A chart's sections are tagged with the progressions they play; a tag its
// section doesn't play is listed on the draft, with the section, and blocks
// publication.
func registerSongChartProgressionSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the chord catalog also has the chord "([^"]+)"$`, func(symbol string) error {
		return w.seedCatalogChord(symbol, openVoicingKey(symbol))
	})
	sc.Step(`^"([^"]+)" has a song chart "([^"]+)" in the concert key "([^"]+)" whose verse has the chords "([^"]+)"$`, func(author, title, key, chords string) error {
		return w.hasChartWithChords(author, title, &key, 0, false, chords)
	})
	sc.Step(`^"([^"]+)" has a song chart "([^"]+)" in the concert key "([^"]+)" whose verse has the chords "([^"]+)" and whose chorus has the chords "([^"]+)"$`, func(author, title, key, verse, chorus string) error {
		return w.hasChartWithChords(author, title, &key, 0, false, verse, chorus)
	})
	sc.Step(`^"([^"]+)" has a song chart "([^"]+)" in the concert key "([^"]+)", capo on fret (\d+), whose verse has the chords "([^"]+)"$`, func(author, title, key string, capo int, chords string) error {
		return w.hasChartWithChords(author, title, &key, capo, false, chords)
	})
	sc.Step(`^"([^"]+)" has a song chart "([^"]+)" with no concert key whose verse has the chords "([^"]+)"$`, func(author, title, chords string) error {
		return w.hasChartWithChords(author, title, nil, 0, false, chords)
	})
	sc.Step(`^"([^"]+)" has a song chart "([^"]+)" whose rights are confirmed, in the concert key "([^"]+)" whose verse has the chords "([^"]+)"$`, func(author, title, key, chords string) error {
		return w.hasChartWithChords(author, title, &key, 0, true, chords)
	})
	sc.Step(`^the verse of "([^"]+)" is tagged with the progression "([^"]+)"$`, func(title, progression string) error {
		if err := w.tagsVerse(songChartSeeder, title, progression); err != nil {
			return err
		}
		return w.mustSucceed("tagging " + title)
	})
	sc.Step(`^"([^"]+)" published "([^"]+)" as revision 1, its verse tagged with the progression "([^"]+)"$`, func(author, title, progression string) error {
		key := "G"
		if err := w.hasChartWithChords(author, title, &key, 0, true, "G C G D G"); err != nil {
			return err
		}
		if err := w.tagsVerse(author, title, progression); err != nil {
			return err
		}
		if err := w.mustSucceed("tagging " + title); err != nil {
			return err
		}
		return w.seedPublication(title)
	})

	sc.Step(`^"([^"]+)" tags the verse of "([^"]+)" with the progression "([^"]+)"$`, func(admin, title, progression string) error {
		return w.tagsVerse(admin, title, progression)
	})
	sc.Step(`^"([^"]+)" tags the verse of "([^"]+)" with the progressions "([^"]+)" and "([^"]+)"$`, func(admin, title, a, b string) error {
		return w.tagsVerse(admin, title, a, b)
	})
	sc.Step(`^"([^"]+)" removes the tag "([^"]+)" from the verse of the draft of "([^"]+)"$`, w.removesVerseTag)

	sc.Step(`^the verse's progressions are "([^"]+)"$`, func(a string) error { return w.verseProgressionsAre(a) })
	sc.Step(`^the verse's progressions are "([^"]+)" and "([^"]+)"$`, func(a, b string) error { return w.verseProgressionsAre(a, b) })
	sc.Step(`^the verse's progressions are empty$`, func() error { return w.verseProgressionsAre() })
	sc.Step(`^the draft lists no unplayed progression$`, func() error { return w.draftListsUnplayed() })
	sc.Step(`^the draft lists "([^"]+)" on the verse as unplayed$`, func(p string) error { return w.draftListsUnplayed(p) })
	sc.Step(`^the change is refused as invalid$`, w.requestRejectedInvalid)
	sc.Step(`^the refusal lists the unplayed progression "([^"]+)" on the verse$`, w.refusalListsUnplayed)
	sc.Step(`^revision 1 of "([^"]+)" still plays "([^"]+)" in its verse$`, w.revisionOnePlaysInVerse)
}

// chordsLine is a lyric line with one syllable under each chord: "[G]la [C]la".
func chordsLine(chords string) string {
	var line strings.Builder
	for _, c := range strings.Fields(chords) {
		fmt.Fprintf(&line, "[%s]la ", c)
	}
	return line.String()
}

// hasChartWithChords writes a chart with a section per entry of sections,
// the first a verse and the next a chorus, each a line of its chords.
func (w *world) hasChartWithChords(author, title string, concertKey *string, capo int, rightsConfirmed bool, sections ...string) error {
	var doc domain.SongChartDocument
	for i, chords := range sections {
		kind := domain.SectionVerse
		if i > 0 {
			kind = domain.SectionChorus
		}
		runs := chartLine(chordsLine(chords))
		for _, r := range runs {
			if r.Anchor != nil {
				// Anchor ids are unique within the document, not the line.
				r.Anchor.ID = fmt.Sprintf("s%d-%s", i, r.Anchor.ID)
			}
		}
		doc.Sections = append(doc.Sections, domain.SongChartSection{Kind: kind, Lines: []domain.SongChartLine{{Runs: runs}}})
	}
	input, err := draftInput(title, "en", doc, rightsConfirmed)
	if err != nil {
		return err
	}
	input.ConcertKey, input.CapoFret = concertKey, capo
	if err := w.createChart(w.adminCtx(author), title, input); err != nil {
		return err
	}
	return w.mustSucceed("setting up song chart " + title)
}

// verseIndex is the index of doc's first verse; with none, as after importing
// a line that names no section, its first section, where the verse was.
func verseIndex(doc domain.SongChartDocument) (int, error) {
	if len(doc.Sections) == 0 {
		return 0, fmt.Errorf("the chart has no section")
	}
	return max(0, slices.IndexFunc(doc.Sections, func(s domain.SongChartSection) bool { return s.Kind == domain.SectionVerse })), nil
}

func (w *world) tagsVerse(admin, title string, progressions ...string) error {
	var missing error
	err := w.changeDraft(admin, title, func(_ *generated.SongChartDraftInput, doc *domain.SongChartDocument) {
		i, err := verseIndex(*doc)
		if err != nil {
			missing = err
			return
		}
		doc.Sections[i].ProgressionKeys = progressions
	})
	return errors.Join(missing, err)
}

func (w *world) removesVerseTag(admin, progression, title string) error {
	var missing error
	err := w.changeDraft(admin, title, func(_ *generated.SongChartDraftInput, doc *domain.SongChartDocument) {
		i, err := verseIndex(*doc)
		if err != nil {
			missing = err
			return
		}
		doc.Sections[i].ProgressionKeys = slices.DeleteFunc(slices.Clone(doc.Sections[i].ProgressionKeys), func(p string) bool { return p == progression })
	})
	return errors.Join(missing, err)
}

// verseProgressionsAre reads the stored draft, so a refused change shows it
// unchanged.
func (w *world) verseProgressionsAre(want ...string) error {
	chart, err := w.getChart(w.charts().title)
	if err != nil {
		return err
	}
	doc, err := fromGeneratedDocument(chart.Draft.Body)
	if err != nil {
		return err
	}
	i, err := verseIndex(doc)
	if err != nil {
		return err
	}
	if got := doc.Sections[i].ProgressionKeys; len(got) != len(want) || (len(want) > 0 && !slices.Equal(got, want)) {
		return fmt.Errorf("expected the verse's progressions %v, got %v", want, got)
	}
	return nil
}

// onTheVerse are progressions as unplayed on the chart's first section, the
// verse.
func onTheVerse(progressions ...string) []generated.SongChartUnplayedProgression {
	out := []generated.SongChartUnplayedProgression{}
	for _, p := range progressions {
		out = append(out, generated.SongChartUnplayedProgression{SectionIndex: 0, ProgressionKey: p})
	}
	return out
}

func (w *world) draftListsUnplayed(want ...string) error {
	chart, err := w.getChart(w.charts().title)
	if err != nil {
		return err
	}
	if got := chart.Draft.UnplayedProgressions; !slices.Equal(got, onTheVerse(want...)) {
		return fmt.Errorf("expected the draft to list %v on the verse as unplayed, got %v", want, got)
	}
	return nil
}

func (w *world) refusalListsUnplayed(progression string) error {
	resp, err := w.notPublishable()
	if err != nil {
		return err
	}
	if !slices.Equal(resp.UnplayedProgressions, onTheVerse(progression)) {
		return fmt.Errorf("expected the refusal to list %q on the verse as unplayed, got %v", progression, resp.UnplayedProgressions)
	}
	return nil
}

func (w *world) revisionOnePlaysInVerse(title, progression string) error {
	revs, err := w.revisions(title)
	if err != nil {
		return err
	}
	for _, r := range revs {
		if r.RevisionNumber != 1 {
			continue
		}
		doc, err := fromGeneratedDocument(r.Body)
		if err != nil {
			return err
		}
		i, err := verseIndex(doc)
		if err != nil {
			return err
		}
		if !slices.Equal(doc.Sections[i].ProgressionKeys, []string{progression}) || !slices.Equal(r.ProgressionKeys, []string{progression}) {
			return fmt.Errorf("expected revision 1's verse tagged with %q, got %v (chart %v)", progression, doc.Sections[i].ProgressionKeys, r.ProgressionKeys)
		}
		return nil
	}
	return fmt.Errorf("%q has no revision 1", title)
}
