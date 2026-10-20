//go:build integration

package bdd

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cucumber/godog"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// tourSkill is the node every progression key tour is classified under here.
const tourSkill = "common-progressions"

func registerTourSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)"'s path skill "common-progressions" has new progression key tours on guitar$`, w.pathSkillHasTours)
	sc.Step(`^"([^"]+)" has new progression key tours on guitar$`, w.pathSkillHasTours)
	sc.Step(`^"([^"]+)" has 10 new progression key tours on guitar, and nothing else$`, w.hasOnlyTours)
	sc.Step(`^"([^"]+)" has never played the progression key tour of "([^"]+)" on (guitar|electric bass)$`, w.neverPlayedTour)
	sc.Step(`^"([^"]+)" has clean takes of the progression key tour of "([^"]+)" on guitar$`, w.hasCleanTourTakes)
	sc.Step(`^the progression key tour is picked for "([^"]+)"'s session with "(guitar|electric-bass)" in hand$`, w.tourPicked)

	sc.Step(`^the session includes a progression key tour of "common-progressions" with the reason new$`, w.sessionIncludesNewTour)
	sc.Step(`^the progression key tour is at the move rung$`, w.tourIsAtTheMoveRung)
	sc.Step(`^the session has exactly one progression key tour$`, w.exactlyOneTour)
	sc.Step(`^it plays (\d+) times? through in each of at most (\d+) takes$`, w.tourPlaysTimesThrough)
	sc.Step(`^it is estimated at (\d+) takes of a bar of count-in and (\d+) bars at (\d+) BPM, each followed by (\d+) seconds to rate it$`, w.tourEstimatedAt)
	sc.Step(`^its keys are ((?:[A-G][b#]? ?)+), in that order$`, w.tourKeysAre)
	sc.Step(`^the key of ([A-G][b#]?) has the chart "([^"]+)" and a voicing for each of its chords$`, w.tourKeyHasChart)
	sc.Step(`^no key has chord tones$`, w.noKeyHasChordTones)
	sc.Step(`^in the key of ([A-G][b#]?), the chord "([^"]+)" has its root on string (\d+) at fret (\d+), its third on string (\d+) at fret (\d+) and its fifth on string (\d+) at fret (\d+)$`, w.tourChordTonesAt)
	sc.Step(`^no item in the session is a progression key tour$`, w.noTours)
	sc.Step(`^no progression key tour has the reason warm_up or application$`, w.noTourWarmUpOrEnding)
}

// tourKeys are every progression of the catalog toured on instrument's layout.
func tourKeys(instrument string) ([]string, error) {
	progressions, err := catalogProgressions{}.List(context.Background())
	if err != nil {
		return nil, err
	}
	keys := make([]string, len(progressions))
	for i, p := range progressions {
		keys[i] = domain.ProgressionKeysItemKey(instrumentID(instrument).String(), p.Key)
	}
	return keys, nil
}

func (w *world) pathSkillHasTours(name string) error {
	keys, err := tourKeys("guitar")
	if err != nil {
		return err
	}
	w.onChordPath(name, tourSkill)
	w.classifyChords("guitar", tourSkill, keys...)
	return nil
}

// hasOnlyTours leaves the path's play-alongs out, so only the tours can be
// played.
func (w *world) hasOnlyTours(name string) error {
	w.diagrams.mu.Lock()
	delete(w.diagrams.byID, diagramID("guitar-lick").String())
	w.diagrams.mu.Unlock()
	return w.pathSkillHasTours(name)
}

// neverPlayedTour puts the one tour on name's path; a bass is tuned as a
// four-string bass, whose places the chord tones are computed on.
func (w *world) neverPlayedTour(name, progression, instrument string) error {
	instrument = strings.ReplaceAll(instrument, " ", "-")
	if instrument == "electric-bass" {
		bass, err := w.ensureInstrumentSeeded(instrument)
		if err != nil {
			return err
		}
		four := 4
		bass.StringCount, bass.Tuning = &four, []string{"E1", "A1", "D2", "G2"}
		w.instruments.put(bass)
	}
	w.onChordPath(name, tourSkill)
	w.tourItemKey = domain.ProgressionKeysItemKey(instrumentID(instrument).String(), progression)
	w.classifyChords(instrument, tourSkill, w.tourItemKey)
	return nil
}

func (w *world) hasCleanTourTakes(name, progression string) error {
	if err := w.neverPlayedTour(name, progression, "guitar"); err != nil {
		return err
	}
	w.knownItems(name, 10, w.tourItemKey)
	return nil
}

// tourPicked composes name's session with the tour due, so it is picked
// whatever else is new.
func (w *world) tourPicked(name, instrument string) error {
	w.dueItems(name, w.tourItemKey)
	w.authenticateAs(name, domain.RoleStudent)
	return w.composesSession(name, 30, instrument)
}

func (w *world) plannedTours() ([]generated.PracticeSessionItem, error) {
	return w.planItemsOfKind(generated.PracticeItemKindProgressionKeys)
}

func (w *world) theTour() (*generated.PlannedProgressionKeys, error) {
	tours, err := w.plannedTours()
	if err != nil {
		return nil, err
	}
	for _, item := range tours {
		if item.ItemKey == w.tourItemKey && item.ProgressionKeys != nil {
			return item.ProgressionKeys, nil
		}
	}
	return nil, fmt.Errorf("the session has no tour %s, of %d tours", w.tourItemKey, len(tours))
}

func (w *world) sessionIncludesNewTour() error {
	tours, err := w.plannedTours()
	if err != nil {
		return err
	}
	if len(tours) == 0 || tours[0].Reason != generated.PracticePickReasonNew {
		return fmt.Errorf("the session has no tour with the reason new, of %d", len(tours))
	}
	return nil
}

func (w *world) tourIsAtTheMoveRung() error {
	return w.everyChordOfKind(generated.PracticeItemKindProgressionKeys, func(item generated.PracticeSessionItem) error {
		return expectChord(item.Rung == generated.Move, "rung %s", item.Rung)
	})()
}

func (w *world) exactlyOneTour() error {
	tours, err := w.plannedTours()
	if err != nil {
		return err
	}
	return expectChord(len(tours) == 1, "the session has %d tours, want 1", len(tours))
}

func (w *world) tourPlaysTimesThrough(times, takes int) error {
	tour, err := w.theTour()
	if err != nil {
		return err
	}
	return expectChord(tour.TimesThrough == times && tour.Takes == takes, "the tour plays %d times through over %d takes", tour.TimesThrough, tour.Takes)
}

func (w *world) tourEstimatedAt(takes, bars, bpm, rateSeconds int) error {
	want := takes * ((bars+1)*4*60/bpm + rateSeconds)
	return w.everyChordEstimatedAt(generated.PracticeItemKindProgressionKeys)(want)
}

func (w *world) tourKeysAre(keys string) error {
	tour, err := w.theTour()
	if err != nil {
		return err
	}
	var got []string
	for _, k := range tour.Keys {
		got = append(got, string(k.MusicKey))
	}
	return expectChord(slices.Equal(got, strings.Fields(keys)), "the tour's keys are %v", got)
}

func (w *world) plannedTourKey(key string) (*generatedTourKey, error) {
	tour, err := w.theTour()
	if err != nil {
		return nil, err
	}
	for i, k := range tour.Keys {
		if string(k.MusicKey) == key {
			return &tour.Keys[i], nil
		}
	}
	return nil, fmt.Errorf("the tour has no key %s", key)
}

func (w *world) tourKeyHasChart(key, chart string) error {
	k, err := w.plannedTourKey(key)
	if err != nil {
		return err
	}
	if !slices.Equal(k.Chart, strings.Fields(chart)) {
		return fmt.Errorf("the chart in %s is %v, want %s", key, k.Chart, chart)
	}
	for _, chord := range k.Chart {
		if !slices.ContainsFunc(k.Voicings, func(v generatedTourVoicing) bool { return v.Chord == chord }) {
			return fmt.Errorf("%s in %s has no voicing", chord, key)
		}
	}
	return nil
}

func (w *world) noKeyHasChordTones() error {
	tour, err := w.theTour()
	if err != nil {
		return err
	}
	for _, k := range tour.Keys {
		if len(k.ChordTones) != 0 {
			return fmt.Errorf("the key of %s has chord tones", k.MusicKey)
		}
	}
	return nil
}

func (w *world) tourChordTonesAt(key, chord string, rootString, rootFret, thirdString, thirdFret, fifthString, fifthFret int) error {
	k, err := w.plannedTourKey(key)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(k.ChordTones, func(c generatedChordTones) bool { return c.Chord == chord })
	if i < 0 {
		return fmt.Errorf("%s in %s has no chord tones", chord, key)
	}
	got := k.ChordTones[i]
	want := generatedChordTones{
		Chord: chord, Root: generated.FretPlace{String: rootString, Fret: rootFret},
		Third: generated.FretPlace{String: thirdString, Fret: thirdFret}, Fifth: generated.FretPlace{String: fifthString, Fret: fifthFret},
	}
	return expectChord(got == want, "%s in %s is at %+v", chord, key, got)
}

func (w *world) noTours() error {
	tours, err := w.plannedTours()
	if err != nil {
		return err
	}
	return expectChord(len(tours) == 0, "the session has %d tours", len(tours))
}

func (w *world) noTourWarmUpOrEnding() error {
	tours, err := w.plannedTours()
	if err != nil {
		return err
	}
	for _, item := range tours {
		if item.Reason == generated.PracticePickReasonWarmUp || item.Reason == generated.PracticePickReasonApplication {
			return fmt.Errorf("%s has the reason %s", item.ItemKey, item.Reason)
		}
	}
	return nil
}

// The generated tour key, voicing and chord tone types are anonymous; these
// aliases name them.
type (
	generatedTourKey = struct {
		Chart      []string               `json:"chart"`
		ChordTones []generatedChordTones  `json:"chord_tones"`
		Clean      bool                   `json:"clean"`
		MusicKey   generated.MusicKey     `json:"music_key"`
		Voicings   []generatedTourVoicing `json:"voicings"`
	}
	generatedTourVoicing = struct {
		Chord     string             `json:"chord"`
		DiagramId openapi_types.UUID `json:"diagram_id"`
	}
	generatedChordTones = struct {
		Chord string              `json:"chord"`
		Fifth generated.FretPlace `json:"fifth"`
		Root  generated.FretPlace `json:"root"`
		Third generated.FretPlace `json:"third"`
	}
)
