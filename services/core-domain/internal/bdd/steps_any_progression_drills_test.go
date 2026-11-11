//go:build integration

package bdd

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/domain"
)

// Drills take any created progression of 2 to 16 bars, in either mode, and make
// up the wrong answers a question lacks.
func registerAnyProgressionDrillSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the wrong answers of "([^"]+)" are only "([^"]+)"$`, w.wrongAnswersAreOnly)
	sc.Step(`^"([^"]+)" lists no wrong answer$`, w.listsNoWrongAnswer)
	sc.Step(`^there is one for "([^"]+)"$`, w.heardItemListed)
	sc.Step(`^none is for "([^"]+)"$`, w.heardItemNotListed)
	sc.Step(`^it offers (two|four) progressions of (two|three|four|eight|twelve|sixteen) bars(?:, all (major|minor))?$`, w.offersProgressionsOfBars)
	sc.Step(`^every option but "([^"]+)" differs from it in exactly one bar$`, w.everyOtherOptionOneBarAway)
	sc.Step(`^every option is named by its numerals$`, w.everyOptionNamedByNumerals)
	sc.Step(`^it offers (two|four) charts of (two|three|four|eight) chords, one of them "([^"]+)"$`, w.offersChartsOfChords)
	sc.Step(`^every bar of the other option is one of (.+)$`, w.otherOptionBarsAmong)
	sc.Step(`^the progression "([^"]+)" is planned as (\w+) (\d+) times$`, w.progressionPlannedTimes)
	sc.Step(`^every option but "([^"]+)" and "([^"]+)" is a progression no drill has$`, w.otherOptionsAreMadeUp)
	sc.Step(`^its chart is "([^"]+)"$`, w.chartIs)
	sc.Step(`^the heard progression "([^"]+)" is planned as progression_by_number (\d+) times$`, w.heardProgressionPlannedTimes)
	sc.Step(`^every key it is planned in is a minor key$`, w.everyKeyIsMinor)
	sc.Step(`^every time it plays only its four bars$`, w.everyTimePlaysFourBars)
	sc.Step(`^the guide tone of "([^"]+)" is picked for "([^"]+)"'s session (\d+) times$`, w.guideToneOfPickedTimes)
}

var countWords = map[string]int{"two": 2, "three": 3, "four": 4, "eight": 8, "twelve": 12, "sixteen": 16}

func countWord(word string) int { return countWords[word] }

// asciiNumerals reads a created progression's key back into its numerals.
var asciiNumerals = strings.NewReplacer("b", "♭", "#", "♯", "o", "°", "h", "ø")

// optionShape is the mode and bars of a progression option: a drill
// progression's, or a made-up one's read from its key.
func (w *world) optionShape(value string) (string, []string, error) {
	drills, err := w.progressions.ListInDrills(context.Background())
	if err != nil {
		return "", nil, err
	}
	if i := slices.IndexFunc(drills, func(p domain.Progression) bool { return p.Key == value }); i >= 0 {
		return drills[i].Mode, drills[i].Bars, nil
	}
	mode, numerals, ok := strings.Cut(value, "/")
	if !ok {
		return "", nil, fmt.Errorf("%q is neither a drill progression nor a made-up one", value)
	}
	return mode, strings.Split(asciiNumerals.Replace(numerals), "-"), nil
}

func (w *world) wrongAnswersAreOnly(key, only string) error {
	p, err := w.listedProgression(key)
	if err != nil {
		return err
	}
	return expectChord(slices.Equal(p.Confusable, []string{only}), "%s lists the wrong answers %v", key, p.Confusable)
}

func (w *world) listsNoWrongAnswer(key string) error {
	p, err := w.listedProgression(key)
	if err != nil {
		return err
	}
	return expectChord(len(p.Confusable) == 0, "%s lists the wrong answers %v", key, p.Confusable)
}

func (w *world) heardItemListed(key string) error {
	return expectChord(slices.Contains(w.harmony().listed, domain.HeardProgressionItemKey(key)), "heard progressions %v", w.harmony().listed)
}

func (w *world) heardItemNotListed(key string) error {
	return expectChord(!slices.Contains(w.harmony().listed, domain.HeardProgressionItemKey(key)), "%s is heard", key)
}

func (w *world) offersProgressionsOfBars(count, bars, mode string) error {
	q, err := w.plannedQuestion()
	if err != nil {
		return err
	}
	if len(q.Options) != countWord(count) {
		return fmt.Errorf("it offers %d options", len(q.Options))
	}
	for _, o := range q.Options {
		optionMode, optionBars, err := w.optionShape(o.Value)
		if err != nil {
			return err
		}
		if len(optionBars) != countWord(bars) {
			return fmt.Errorf("%q has %d bars", o.Value, len(optionBars))
		}
		if mode != "" && optionMode != mode {
			return fmt.Errorf("%q is %s", o.Value, optionMode)
		}
	}
	return nil
}

func (w *world) everyOtherOptionOneBarAway(asked string) error {
	q, err := w.plannedQuestion()
	if err != nil {
		return err
	}
	_, askedBars, err := w.optionShape(asked)
	if err != nil {
		return err
	}
	for _, o := range q.Options {
		if o.Value == asked {
			continue
		}
		_, bars, err := w.optionShape(o.Value)
		if err != nil {
			return err
		}
		changed := 0
		for i := range bars {
			if bars[i] != askedBars[i] {
				changed++
			}
		}
		if changed != 1 {
			return fmt.Errorf("%q differs from %s in %d bars", o.Value, asked, changed)
		}
	}
	return nil
}

func (w *world) everyOptionNamedByNumerals() error {
	q, err := w.plannedQuestion()
	if err != nil {
		return err
	}
	for _, o := range q.Options {
		_, bars, err := w.optionShape(o.Value)
		if err != nil {
			return err
		}
		if want := strings.Join(bars, "–"); o.Names.Resolve("pt_BR") != want {
			return fmt.Errorf("%q is named %q, want %q", o.Value, o.Names.Resolve("pt_BR"), want)
		}
	}
	return nil
}

func (w *world) offersChartsOfChords(count, chords, chart string) error {
	q, err := w.plannedQuestion()
	if err != nil {
		return err
	}
	if len(q.Options) != countWord(count) {
		return fmt.Errorf("it offers %d charts", len(q.Options))
	}
	found := false
	for _, o := range q.Options {
		if len(o.Chart) != countWord(chords) {
			return fmt.Errorf("%q has %d chords", o.Value, len(o.Chart))
		}
		found = found || o.Value == chart
	}
	return expectChord(found, "no option is %q", chart)
}

func (w *world) otherOptionBarsAmong(list string) error {
	q, err := w.plannedQuestion()
	if err != nil {
		return err
	}
	allowed := strings.FieldsFunc(strings.ReplaceAll(list, " and ", ","), func(r rune) bool { return r == ',' || r == ' ' })
	for _, o := range q.Options {
		if o.Value == q.Answer {
			continue
		}
		_, bars, err := w.optionShape(o.Value)
		if err != nil {
			return err
		}
		for _, bar := range bars {
			if !slices.Contains(allowed, bar) {
				return fmt.Errorf("%q has the bar %s", o.Value, bar)
			}
		}
	}
	return nil
}

// progressionPlannedTimes plans the chart progression times, in any key, and
// keeps every option it offered.
func (w *world) progressionPlannedTimes(progression, way string, times int) error {
	item, err := w.drillChartItem(progression)
	if err != nil {
		return err
	}
	h := w.harmony()
	h.plannedOptions = nil
	for range times {
		planning := domain.DrillPlanning{Pick: rand.IntN, Reading: domain.GeneratedChordSymbols, Level: domain.KnowledgeLevelAccurate}
		if err := w.planHarmony(item, way, planning); err != nil {
			return err
		}
		for _, o := range h.question.Options {
			h.plannedOptions = append(h.plannedOptions, o.Value)
		}
	}
	return nil
}

func (w *world) otherOptionsAreMadeUp(asked, other string) error {
	drills, err := w.progressions.ListInDrills(context.Background())
	if err != nil {
		return err
	}
	for _, value := range w.harmony().plannedOptions {
		if value == asked || value == other {
			continue
		}
		if slices.ContainsFunc(drills, func(p domain.Progression) bool { return p.Key == value }) {
			return fmt.Errorf("%q is a drill progression", value)
		}
	}
	return nil
}

func (w *world) chartIs(chart string) error {
	q, err := w.plannedQuestion()
	if err != nil {
		return err
	}
	return expectChord(strings.Join(q.Chart, " ") == chart, "the chart is %v", q.Chart)
}

// heardProgressionPlannedTimes plans the heard progression times, in the keys
// the planner picks, keeping each key and how many bars it played.
func (w *world) heardProgressionPlannedTimes(progression string, times int) error {
	item, err := w.drillHeardProgression(progression)
	if err != nil {
		return err
	}
	h := w.harmony()
	w.guideToneKeys, h.playedBars = nil, nil
	for range times {
		if err := w.planHarmony(item, "progression_by_number", domain.DrillPlanning{Pick: rand.IntN, Level: domain.KnowledgeLevelLearning}); err != nil {
			return err
		}
		w.guideToneKeys = append(w.guideToneKeys, h.question.MusicKey)
		played := 0
		if h.question.Playback != nil {
			played = len(h.question.Playback.Bars)
		}
		h.playedBars = append(h.playedBars, played)
	}
	return nil
}

func (w *world) everyKeyIsMinor() error {
	for _, k := range w.guideToneKeys {
		if !slices.Contains(domain.MusicKeysOf(domain.ProgressionModeMinor), k) {
			return fmt.Errorf("%s is no minor key", k)
		}
	}
	return expectChord(len(w.guideToneKeys) > 0, "it was never planned")
}

func (w *world) everyTimePlaysFourBars() error {
	for _, played := range w.harmony().playedBars {
		if played != 4 {
			return fmt.Errorf("a time played %d bars", played)
		}
	}
	return expectChord(len(w.harmony().playedBars) > 0, "it was never planned")
}

// guideToneOfPickedTimes picks the guide tone a step already named, its
// progression's, times.
func (w *world) guideToneOfPickedTimes(progression, name string, times int) error {
	if !strings.HasSuffix(w.guideToneKey, ":"+progression) {
		return fmt.Errorf("no guide tone of %s was named", progression)
	}
	return w.guideTonePickedTimes(name, times)
}
