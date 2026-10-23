//go:build integration

package bdd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// A chart is tagged with the progressions it plays; a tag it doesn't play is
// listed on the draft and blocks publication.
func registerSongChartProgressionSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the chord catalog also has the chord "([^"]+)"$`, func(symbol string) error {
		return w.seedCatalogChord(symbol, openVoicingKey(symbol))
	})
	sc.Step(`^"([^"]+)" has a song chart "([^"]+)" in the concert key "([^"]+)" with the chords "([^"]+)"$`, func(author, title, key, chords string) error {
		return w.hasChartWithChords(author, title, &key, 0, chords, false)
	})
	sc.Step(`^"([^"]+)" has a song chart "([^"]+)" in the concert key "([^"]+)", capo on fret (\d+), with the chords "([^"]+)"$`, func(author, title, key string, capo int, chords string) error {
		return w.hasChartWithChords(author, title, &key, capo, chords, false)
	})
	sc.Step(`^"([^"]+)" has a song chart "([^"]+)" with no concert key and the chords "([^"]+)"$`, func(author, title, chords string) error {
		return w.hasChartWithChords(author, title, nil, 0, chords, false)
	})
	sc.Step(`^"([^"]+)" has a song chart "([^"]+)" whose rights are confirmed, in the concert key "([^"]+)" with the chords "([^"]+)"$`, func(author, title, key, chords string) error {
		return w.hasChartWithChords(author, title, &key, 0, chords, true)
	})
	sc.Step(`^"([^"]+)" is tagged with the progression "([^"]+)"$`, func(title, progression string) error {
		if err := w.tagsChart(songChartSeeder, title, progression); err != nil {
			return err
		}
		return w.mustSucceed("tagging " + title)
	})
	sc.Step(`^"([^"]+)" published "([^"]+)" as revision 1, tagged with the progression "([^"]+)"$`, func(author, title, progression string) error {
		key := "G"
		if err := w.hasChartWithChords(author, title, &key, 0, "G C G D G", true); err != nil {
			return err
		}
		if err := w.tagsChart(author, title, progression); err != nil {
			return err
		}
		if err := w.mustSucceed("tagging " + title); err != nil {
			return err
		}
		return w.seedPublication(title)
	})

	sc.Step(`^"([^"]+)" tags "([^"]+)" with the progression "([^"]+)"$`, func(admin, title, progression string) error {
		return w.tagsChart(admin, title, progression)
	})
	sc.Step(`^"([^"]+)" tags "([^"]+)" with the progressions "([^"]+)" and "([^"]+)"$`, func(admin, title, a, b string) error {
		return w.tagsChart(admin, title, a, b)
	})
	sc.Step(`^"([^"]+)" removes the tag "([^"]+)" from the draft of "([^"]+)"$`, w.removesTag)

	sc.Step(`^the draft's progressions are "([^"]+)"$`, func(a string) error { return w.draftProgressionsAre(a) })
	sc.Step(`^the draft's progressions are "([^"]+)" and "([^"]+)"$`, func(a, b string) error { return w.draftProgressionsAre(a, b) })
	sc.Step(`^the draft's progressions are empty$`, func() error { return w.draftProgressionsAre() })
	sc.Step(`^the draft lists no unplayed progression$`, func() error { return w.draftListsUnplayed() })
	sc.Step(`^the draft lists "([^"]+)" as unplayed$`, func(p string) error { return w.draftListsUnplayed(p) })
	sc.Step(`^the change is refused as invalid$`, w.requestRejectedInvalid)
	sc.Step(`^the refusal lists the unplayed progression "([^"]+)"$`, w.refusalListsUnplayed)
	sc.Step(`^revision 1 of "([^"]+)" is still tagged with "([^"]+)"$`, w.revisionOneTaggedWith)
}

// chordsLine is a lyric line with one syllable under each chord: "[G]la [C]la".
func chordsLine(chords string) string {
	var line strings.Builder
	for _, c := range strings.Fields(chords) {
		fmt.Fprintf(&line, "[%s]la ", c)
	}
	return line.String()
}

func (w *world) hasChartWithChords(author, title string, concertKey *string, capo int, chords string, rightsConfirmed bool) error {
	input, err := draftInput(title, "en", chartDocument(chordsLine(chords)), rightsConfirmed)
	if err != nil {
		return err
	}
	input.ConcertKey, input.CapoFret = concertKey, capo
	if err := w.createChart(w.adminCtx(author), title, input); err != nil {
		return err
	}
	return w.mustSucceed("setting up song chart " + title)
}

func (w *world) tagsChart(admin, title string, progressions ...string) error {
	return w.changeDraft(admin, title, func(input *generated.SongChartDraftInput, _ *domain.SongChartDocument) {
		input.ProgressionKeys = &progressions
	})
}

func (w *world) removesTag(admin, progression, title string) error {
	return w.changeDraft(admin, title, func(input *generated.SongChartDraftInput, _ *domain.SongChartDocument) {
		if input.ProgressionKeys != nil {
			kept := slices.DeleteFunc(slices.Clone(*input.ProgressionKeys), func(p string) bool { return p == progression })
			input.ProgressionKeys = &kept
		}
	})
}

// draftProgressionsAre reads the stored draft, so a refused change shows it
// unchanged.
func (w *world) draftProgressionsAre(want ...string) error {
	chart, err := w.getChart(w.charts().title)
	if err != nil {
		return err
	}
	var got []string
	if chart.Draft.ProgressionKeys != nil {
		got = *chart.Draft.ProgressionKeys
	}
	if len(got) != len(want) || (len(want) > 0 && !slices.Equal(got, want)) {
		return fmt.Errorf("expected the draft's progressions %v, got %v", want, got)
	}
	return nil
}

func (w *world) draftListsUnplayed(want ...string) error {
	chart, err := w.getChart(w.charts().title)
	if err != nil {
		return err
	}
	if got := chart.Draft.UnplayedProgressionKeys; len(got) != len(want) || (len(want) > 0 && !slices.Equal(got, want)) {
		return fmt.Errorf("expected the draft to list %v as unplayed, got %v", want, got)
	}
	return nil
}

func (w *world) refusalListsUnplayed(progression string) error {
	resp, err := w.notPublishable()
	if err != nil {
		return err
	}
	if !slices.Equal(resp.UnplayedProgressionKeys, []string{progression}) {
		return fmt.Errorf("expected the refusal to list %q as unplayed, got %v", progression, resp.UnplayedProgressionKeys)
	}
	return nil
}

func (w *world) revisionOneTaggedWith(title, progression string) error {
	revs, err := w.revisions(title)
	if err != nil {
		return err
	}
	for _, r := range revs {
		if r.RevisionNumber == 1 {
			if !slices.Equal(r.ProgressionKeys, []string{progression}) {
				return fmt.Errorf("expected revision 1 tagged with %q, got %v", progression, r.ProgressionKeys)
			}
			return nil
		}
	}
	return fmt.Errorf("%q has no revision 1", title)
}
