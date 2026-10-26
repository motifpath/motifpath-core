//go:build integration

package bdd

import (
	"fmt"

	"github.com/cucumber/godog"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

// A song is a song chart played whole on a layout and rated once, with no
// measure.
func registerSongSteps(sc *godog.ScenarioContext, w *world) {
	// ── Grading ────────────────────────────────────────────────────────────────
	sc.Step(`^"([^"]*)" rates the song "([^"]*)" on guitar as "([^"]*)"$`, w.ratesTheSong)
	sc.Step(`^"([^"]*)" rates the song "([^"]*)" on guitar as "([^"]*)" at (\d+) BPM$`, w.ratesTheSongAtATempo)
	sc.Step(`^"([^"]*)" rates the song of a song chart that doesn't exist on guitar as "([^"]*)"$`, w.ratesASongOfAMissingChart)
	sc.Step(`^"([^"]*)" has self-assessed evidence for that song rated "([^"]*)", with no tempo$`, w.hasEvidenceForTheSong)
	sc.Step(`^"([^"]*)" has no evidence for that song$`, w.hasNoEvidenceForTheSong)

	// ── Knowledge state ────────────────────────────────────────────────────────
	sc.Step(`^"([^"]*)" practises the song "([^"]*)" on guitar$`, w.practisesTheSong)
	sc.Step(`^"([^"]*)" rates it "([^"]*)" on (\d+) different days$`, w.ratesTheSongOnDifferentDays)
	sc.Step(`^"([^"]*)"'s level for the song is "([^"]*)"$`, w.levelForTheSongIs)
}

// songOnGuitar is the song of chart on the scenario's guitar. The layout exists in
// the reference snapshot from the moment a scenario names it, and so does the
// chart unless the scenario says it doesn't.
func (w *world) songOnGuitar(chart string, exists bool) string {
	layout := stableUUID("instrument", "guitar")
	if _, ok := w.reference.instruments[layout]; !ok {
		w.reference.instruments[layout] = domain.InstrumentReference{ID: layout, Tuning: standardGuitar}
	}
	id := stableUUID("song_chart", chart)
	if exists {
		w.reference.songCharts[id] = domain.SongChartReference{ID: id}
	}
	w.songKey = "song:" + layout + ":" + id
	return w.songKey
}

func (w *world) rateSong(student, itemKey, r string, tempo *int) error {
	rt, err := rating(r)
	if err != nil {
		return err
	}
	return w.answer(student, itemKey, domain.PracticeResponse{Type: domain.PracticeResponseSelfRating, Rating: rt, TempoBPM: tempo})
}

func (w *world) ratesTheSong(student, chart, r string) error {
	return w.rateSong(student, w.songOnGuitar(chart, true), r, nil)
}

func (w *world) ratesTheSongAtATempo(student, chart, r string, bpm int) error {
	return w.rateSong(student, w.songOnGuitar(chart, true), r, &bpm)
}

func (w *world) ratesASongOfAMissingChart(student, r string) error {
	return w.rateSong(student, w.songOnGuitar("a chart that doesn't exist", false), r, nil)
}

func (w *world) hasEvidenceForTheSong(student, r string) error {
	evidence, err := w.evidenceFor(student, w.songKey)
	if err != nil {
		return err
	}
	if len(evidence) != 1 {
		return fmt.Errorf("want one piece of evidence, got %d", len(evidence))
	}
	e := evidence[0]
	if e.Source != domain.EvidenceSourceSelfAssessed || string(e.Rating) != r || e.TempoBPM != nil {
		return fmt.Errorf("evidence is %s rated %q at %v BPM, want self_assessed rated %q with no tempo", e.Source, e.Rating, e.TempoBPM, r)
	}
	return nil
}

func (w *world) hasNoEvidenceForTheSong(student string) error {
	evidence, err := w.evidenceFor(student, w.songKey)
	if err != nil {
		return err
	}
	if len(evidence) != 0 {
		return fmt.Errorf("want no evidence, got %d", len(evidence))
	}
	return nil
}

func (w *world) practisesTheSong(_, chart string) error {
	w.songOnGuitar(chart, true)
	return nil
}

func (w *world) ratesTheSongOnDifferentDays(student, r string, days int) error {
	for range days {
		if err := w.rateSong(student, w.songKey, r, nil); err != nil {
			return err
		}
		w.clock = w.clock.AddDate(0, 0, 1)
	}
	return nil
}

func (w *world) levelForTheSongIs(student, level string) error {
	return w.levelIs(student, w.songKey, level)
}
