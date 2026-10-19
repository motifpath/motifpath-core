//go:build integration

package bdd

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// hearWorld is what the ear drill steps share in a scenario: the heard item
// a step named, the student's level on it and the key it is heard in, for
// "it is planned for".
type hearWorld struct {
	item  domain.DrillItem
	level domain.KnowledgeLevel
	key   string
}

func (w *world) hear() *hearWorld {
	if w.hearDrills == nil {
		w.hearDrills = &hearWorld{}
	}
	return w.hearDrills
}

func registerHearDrillSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the progression "([^"]+)" is planned as progression_by_number$`, w.heardProgressionPlannedAnyKey)
	sc.Step(`^the heard progression "([^"]+)" is planned as progression_by_number in (\S+)$`, w.heardProgressionPlanned)
	sc.Step(`^the heard progression "([^"]+)" is planned as progression_by_number in (\S+) for a session that reads chords on "([^"]+)"$`, w.heardProgressionPlannedReading)
	sc.Step(`^the heard chord (\w+) is planned as see_then_name in (\S+) for a session that reads chords on "([^"]+)"$`, w.heardChordSeenPlanned)
	sc.Step(`^its options include "([^"]+)" and "([^"]+)"$`, w.optionsInclude)
	sc.Step(`^every option has four bars$`, w.everyOptionHasFourBars)

	sc.Step(`^"([^"]+)" has named the heard chord (\w+) by ear correctly twice and has never named it by sight$`, w.namedHeardChordTwice)
	sc.Step(`^the heard chord (\w+) is picked for "([^"]+)"'s session in the head$`, w.heardChordPickedInTheHead)
	sc.Step(`^the question names (\S+) and draws the box of "([^"]+)", unnamed$`, w.namesKeyAndDrawsBox)
	sc.Step(`^it offers numerals, one of them (\w+)$`, w.offersNumeralsWith)
	sc.Step(`^it plays nothing$`, w.playsNothing)

	sc.Step(`^"([^"]+)" is (learning|accurate|fluent) on the heard chord (\w+) in (\S+)$`, w.levelOnHeardChordInKey)
	sc.Step(`^"([^"]+)" is (learning) the heard chord (\w+) in (\S+)$`, w.levelOnHeardChordInKey)
	sc.Step(`^"([^"]+)" is (learning|accurate|fluent) on the heard chord (\w+)$`, w.levelOnHeardChord)
	sc.Step(`^"([^"]+)" is (learning|accurate|fluent) on the heard progression "([^"]+)"$`, w.levelOnHeardProgression)
	sc.Step(`^a cadence in (\S+) plays before the chord$`, w.cadencePlays)
	sc.Step(`^only the tonic chord of (\S+) plays, for one bar, before the chord$`, w.onlyTonicPlays)
	sc.Step(`^the bars play "([^"]+)", "([^"]+)", "([^"]+)", "([^"]+)" and then "([^"]+)"$`, w.barsPlayFive)
	sc.Step(`^the bars play "([^"]+)", "([^"]+)", "([^"]+)" and "([^"]+)"$`, w.barsPlayFour)
	sc.Step(`^its bars still play the chords "([^"]+)", "([^"]+)", "([^"]+)" and "([^"]+)"$`, w.barsPlayFour)
	sc.Step(`^the first four bars show I, IV, V and I, and the last shows no numeral$`, w.cadenceBarsNumbered)
	sc.Step(`^no bar shows a numeral$`, w.noBarNumbered)
	sc.Step(`^it offers (\w+) and the three numerals nearest it$`, w.offersNearestNumerals)
	sc.Step(`^it offers I, ii, iii, IV, V, vi and viio$`, w.offersEveryNumeral)
	sc.Step(`^its playback starts at (\d+) BPM$`, w.playbackStartsAt)
	sc.Step(`^its playback has a bass line$`, w.playbackHasBassLine)

	sc.Step(`^every heard progression item is listed$`, w.heardProgressionsListed)
	sc.Step(`^there is one for each 4-bar progression of the progression catalog$`, w.oneForEachFourBarProgression)
	sc.Step(`^none is for "([^"]+)" or "([^"]+)"$`, w.noneForProgressions)

	sc.Step(`^"([^"]+)" has new heard chords of (\S+) major$`, w.hasNewHeardChords)
	sc.Step(`^every heard chord in the session is asked again later in the same session$`, w.everyHeardChordAskedAgain)
	sc.Step(`^at least three other items come between the two times it is asked$`, w.threeItemsBetweenComebacks)
	sc.Step(`^the second time it is asked by ear, as (\w+)$`, w.secondTimeAskedAs)
	sc.Step(`^"([^"]+)" has new heard progressions "([^"]+)" and "([^"]+)", and other new heard progressions$`, w.hasNewHeardProgressions)
	sc.Step(`^"([^"]+)" and "([^"]+)" are asked with no other heard progression between them$`, w.heardProgressionsTogether)
}

// learnsHearing puts name on a path teaching the ear drills' nodes, and
// turns on the harmony and ear items.
func (w *world) learnsHearing(name string) string {
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	var skills []domain.KnowledgeNode
	for _, key := range []string{"hear-chord-quality", "hear-progressions"} {
		skills = append(skills, domain.KnowledgeNode{ID: w.skillIDFor(key).String()})
	}
	node := domain.ContentNode{
		ID: nodeID("ear-lesson").String(), Title: "ear-lesson", ContentType: domain.ContentTypeArticle,
		Classification: domain.Classification{Skills: skills},
	}
	w.nodes.put(node)
	w.studentPaths.put(domain.StudentPath{
		ID: deterministicUUID("student-path", name+"/ear").String(), StudentID: studentID,
		SourceTemplateID: pathID("ear-path").String(), AssignedAt: fixedNow,
		Items: []domain.StudentPathItemRecord{{Position: 1, ContentNodeID: node.ID}},
	})
	w.paths.put(domain.LearningPath{ID: pathID("ear-path").String()})
	w.practiceItems.harmony = w.harmonyClassified
	return studentID
}

// keyFirst picks key for the first choice a plan makes, the key, and at
// random after it.
func keyFirst(key string) (func(int) int, error) {
	at := slices.Index(domain.MusicKeys, key)
	if at < 0 {
		return nil, fmt.Errorf("no key %q", key)
	}
	first := true
	return func(n int) int {
		if first {
			first = false
			return at
		}
		return rand.IntN(n)
	}, nil
}

func catalogHeardProgression(progression string) (domain.HeardProgressionItem, error) {
	chart, err := catalogChartItem(progression)
	if err != nil {
		return domain.HeardProgressionItem{}, err
	}
	return domain.HeardProgressionItem{Progression: chart.Progression, Catalog: chart.Catalog}, nil
}

func (w *world) heardProgressionPlannedAnyKey(progression string) error {
	item, err := catalogHeardProgression(progression)
	if err != nil {
		return err
	}
	return w.planHarmony(item, "progression_by_number", domain.DrillPlanning{Pick: rand.IntN, Level: domain.KnowledgeLevelLearning})
}

func (w *world) heardProgressionPlanned(progression, key string) error {
	return w.heardProgressionPlannedReading(progression, key, "guitar")
}

func (w *world) heardProgressionPlannedReading(progression, key, instrument string) error {
	item, err := catalogHeardProgression(progression)
	if err != nil {
		return err
	}
	planning := w.readingOn(instrument)
	planning.Voicing = catalogChordBox
	if instrument == "electric-bass" {
		planning.BassInstrumentID = instrumentID(instrument).String()
	}
	if planning.Pick, err = keyFirst(key); err != nil {
		return err
	}
	return w.planHarmony(item, "progression_by_number", planning)
}

func (w *world) heardChordSeenPlanned(numeral, key, instrument string) error {
	planning := w.readingOn(instrument)
	var err error
	if planning.Pick, err = keyFirst(key); err != nil {
		return err
	}
	planning.Level = domain.KnowledgeLevelLearning
	return w.planHarmony(domain.HeardChordItem{Numeral: numeral}, "see_then_name", planning)
}

func (w *world) optionsInclude(a, b string) error {
	q, err := w.plannedQuestion()
	if err != nil {
		return err
	}
	for _, want := range []string{a, b} {
		if !slices.ContainsFunc(q.Options, func(o domain.GeneratedOption) bool { return o.Value == want }) {
			return fmt.Errorf("%q is not offered", want)
		}
	}
	return domain.CheckChallenge(w.harmony().stimulus)
}

func (w *world) everyOptionHasFourBars() error {
	q, err := w.plannedQuestion()
	if err != nil {
		return err
	}
	progressions, err := catalogProgressions{}.List(context.Background())
	if err != nil {
		return err
	}
	for _, o := range q.Options {
		i := slices.IndexFunc(progressions, func(p domain.Progression) bool { return p.Key == o.Value })
		if i < 0 || len(progressions[i].Bars) != 4 {
			return fmt.Errorf("option %q is not a 4-bar progression", o.Value)
		}
	}
	return nil
}

func (w *world) namedHeardChordTwice(name, numeral string) error {
	studentID := w.learnsHearing(name)
	due := fixedNow.AddDate(0, 0, -1)
	w.practiceStates.put(studentID, domain.PracticeItemState{
		ItemKey: domain.HeardChordItem{Numeral: numeral}.ItemKey(), Level: domain.KnowledgeLevelLearning,
		Counted: 2, Box: 1, DueAt: &due, RightByResponse: map[string]int{"cadence_then_chord": 2},
	})
	return nil
}

func (w *world) heardChordPickedInTheHead(numeral, _ string) error {
	w.harmony().itemKey = domain.HeardChordItem{Numeral: numeral}.ItemKey()
	w.lastShapeKey, w.lastCell = "", nil
	return w.composeSession(15, nil)
}

func (w *world) namesKeyAndDrawsBox(key, chord string) error {
	q, err := w.plannedQuestion()
	if err != nil {
		return err
	}
	if q.MusicKey != key || q.Chord != chord {
		return fmt.Errorf("the question names %s and gives %q", q.MusicKey, q.Chord)
	}
	box, err := catalogOpenVoicing(chord)
	if err != nil {
		return err
	}
	if q.DrawnAs != domain.GeneratedChordBoxes || !slices.Equal(q.Voicings, []domain.ChordBox{{Chord: chord, DiagramID: box}}) {
		return fmt.Errorf("it draws %s with %v", q.DrawnAs, q.Voicings)
	}
	return domain.CheckChallenge(w.harmony().stimulus)
}

func (w *world) offersNumeralsWith(numeral string) error {
	q, err := w.plannedQuestion()
	if err != nil {
		return err
	}
	for _, o := range q.Options {
		if !slices.Contains(domain.ChordNumerals, o.Value) {
			return fmt.Errorf("%q is not a numeral", o.Value)
		}
	}
	if !slices.ContainsFunc(q.Options, func(o domain.GeneratedOption) bool { return o.Value == numeral }) {
		return fmt.Errorf("%s is not offered", numeral)
	}
	return nil
}

func (w *world) playsNothing() error {
	q, err := w.plannedQuestion()
	if err != nil {
		return err
	}
	if q.Playback != nil {
		return fmt.Errorf("it plays %+v", *q.Playback)
	}
	return nil
}

// levelOnHeardChordInKey sets name's level on the heard chord, which is
// then planned in key.
func (w *world) levelOnHeardChordInKey(name, level, numeral, key string) error {
	w.learnsHearing(name)
	h := w.hear()
	h.item, h.level, h.key = domain.HeardChordItem{Numeral: numeral}, domain.KnowledgeLevel(level), key
	return nil
}

func (w *world) levelOnHeardChord(name, level, numeral string) error {
	return w.levelOnHeardChordInKey(name, level, numeral, "")
}

func (w *world) levelOnHeardProgression(name, level, progression string) error {
	w.learnsHearing(name)
	item, err := catalogHeardProgression(progression)
	if err != nil {
		return err
	}
	h := w.hear()
	h.item, h.level, h.key = item, domain.KnowledgeLevel(level), ""
	return nil
}

// planHeard plans the heard item a step named, by ear, at the student's
// level on it, in its key when a step named one.
func (w *world) planHeard() error {
	h := w.hear()
	planning := domain.DrillPlanning{Pick: rand.IntN, Level: h.level, Voicing: catalogChordBox}
	if h.key != "" {
		var err error
		if planning.Pick, err = keyFirst(h.key); err != nil {
			return err
		}
	}
	return w.planHarmony(h.item, h.item.Ways()[0], planning)
}

func (w *world) playback() (*domain.GeneratedPlayback, error) {
	q, err := w.plannedQuestion()
	if err != nil {
		return nil, err
	}
	if q.Playback == nil {
		return nil, fmt.Errorf("the question plays nothing")
	}
	return q.Playback, nil
}

func (w *world) barChords() ([]string, error) {
	p, err := w.playback()
	if err != nil {
		return nil, err
	}
	chords := make([]string, len(p.Bars))
	for i, b := range p.Bars {
		chords[i] = b.Chord
	}
	return chords, nil
}

func (w *world) barNumerals() ([]string, error) {
	p, err := w.playback()
	if err != nil {
		return nil, err
	}
	numerals := make([]string, len(p.Bars))
	for i, b := range p.Bars {
		numerals[i] = b.Numeral
	}
	return numerals, nil
}

func (w *world) barsAre(want ...string) error {
	chords, err := w.barChords()
	if err != nil {
		return err
	}
	if !slices.Equal(chords, want) {
		return fmt.Errorf("the bars play %v, want %v", chords, want)
	}
	return domain.CheckChallenge(w.harmony().stimulus)
}

func (w *world) barsPlayFive(a, b, c, d, e string) error { return w.barsAre(a, b, c, d, e) }

func (w *world) barsPlayFour(a, b, c, d string) error { return w.barsAre(a, b, c, d) }

func (w *world) cadencePlays(key string) error {
	chords, err := w.barChords()
	if err != nil {
		return err
	}
	cadence := domain.ProgressionChart([]string{"I", "IV", "V", "I"}, key)
	if len(chords) != len(cadence)+1 || !slices.Equal(chords[:len(cadence)], cadence) {
		return fmt.Errorf("the bars play %v, want the cadence %v then the chord", chords, cadence)
	}
	return nil
}

func (w *world) onlyTonicPlays(key string) error {
	chords, err := w.barChords()
	if err != nil {
		return err
	}
	if len(chords) != 2 || chords[0] != domain.NumeralChord(key, "I") {
		return fmt.Errorf("the bars play %v, want %s then the chord", chords, domain.NumeralChord(key, "I"))
	}
	return nil
}

func (w *world) cadenceBarsNumbered() error {
	numerals, err := w.barNumerals()
	if err != nil {
		return err
	}
	if want := []string{"I", "IV", "V", "I", ""}; !slices.Equal(numerals, want) {
		return fmt.Errorf("the bars show %q, want %q", numerals, want)
	}
	return nil
}

func (w *world) noBarNumbered() error {
	numerals, err := w.barNumerals()
	if err != nil {
		return err
	}
	for i, n := range numerals {
		if n != "" {
			return fmt.Errorf("bar %d shows %s", i+1, n)
		}
	}
	return nil
}

func (w *world) offeredValues() ([]string, error) {
	q, err := w.plannedQuestion()
	if err != nil {
		return nil, err
	}
	values := make([]string, len(q.Options))
	for i, o := range q.Options {
		values[i] = o.Value
	}
	return values, nil
}

// offersNearestNumerals checks the options are numeral and the three
// numerals nearest it around the scale: two one step away, and one of the
// two steps away.
func (w *world) offersNearestNumerals(numeral string) error {
	values, err := w.offeredValues()
	if err != nil {
		return err
	}
	at, n := slices.Index(domain.ChordNumerals, numeral), len(domain.ChordNumerals)
	step := func(by int) string { return domain.ChordNumerals[(at+by+n)%n] }
	ok := len(values) == 4 && slices.Contains(values, numeral) && slices.Contains(values, step(-1)) && slices.Contains(values, step(1)) &&
		(slices.Contains(values, step(-2)) || slices.Contains(values, step(2)))
	if !ok {
		return fmt.Errorf("it offers %v, want %s and the three numerals nearest it", values, numeral)
	}
	return nil
}

func (w *world) offersEveryNumeral() error {
	values, err := w.offeredValues()
	if err != nil {
		return err
	}
	if !slices.Equal(values, domain.ChordNumerals) {
		return fmt.Errorf("it offers %v, want %v", values, domain.ChordNumerals)
	}
	return nil
}

func (w *world) playbackStartsAt(bpm int) error {
	p, err := w.playback()
	if err != nil {
		return err
	}
	if p.TempoBPM != bpm {
		return fmt.Errorf("its playback starts at %d BPM, want %d", p.TempoBPM, bpm)
	}
	return nil
}

func (w *world) playbackHasBassLine() error {
	p, err := w.playback()
	if err != nil {
		return err
	}
	if want := instrumentID("electric-bass").String(); p.BassInstrumentID != want {
		return fmt.Errorf("its playback's bass line plays on %q, want the electric-bass %q", p.BassInstrumentID, want)
	}
	return nil
}

func (w *world) heardProgressionsListed() error {
	h := w.harmony()
	h.listed = nil
	for _, item := range w.harmonyClassified() {
		if strings.HasPrefix(item.ItemKey, string(domain.PracticeItemKindHeardProgression)+":") {
			h.listed = append(h.listed, item.ItemKey)
		}
	}
	return nil
}

func (w *world) oneForEachFourBarProgression() error {
	progressions, err := catalogProgressions{}.List(context.Background())
	if err != nil {
		return err
	}
	var want []string
	for _, p := range progressions {
		if len(p.Bars) == 4 {
			want = append(want, domain.HeardProgressionItemKey(p.Key))
		}
	}
	got := slices.Sorted(slices.Values(w.harmony().listed))
	slices.Sort(want)
	if !slices.Equal(got, want) {
		return fmt.Errorf("heard progressions %v, want %v", got, want)
	}
	return nil
}

func (w *world) noneForProgressions(a, b string) error {
	for _, p := range []string{a, b} {
		if slices.Contains(w.harmony().listed, domain.HeardProgressionItemKey(p)) {
			return fmt.Errorf("%s is listed", p)
		}
	}
	return nil
}

func (w *world) hasNewHeardChords(name, _ string) error {
	w.learnsHearing(name)
	return nil
}

func (w *world) hasNewHeardProgressions(name, _, _ string) error {
	w.learnsHearing(name)
	return nil
}

// heardKinds is the plan's items asked by ear or by sight about a heard
// chord or progression, in plan order.
func (w *world) heardItems(kind generated.PracticeItemKind) ([]generated.PracticeSessionItem, []generated.PracticeSessionItem, error) {
	plan, err := w.composedPlan()
	if err != nil {
		return nil, nil, err
	}
	var heard []generated.PracticeSessionItem
	for _, item := range plan.Items {
		if item.Kind == kind {
			heard = append(heard, item)
		}
	}
	if len(heard) == 0 {
		return nil, nil, fmt.Errorf("the session has no %s", kind)
	}
	return plan.Items, heard, nil
}

// askedTwice lists, for every heard chord of the plan, where it is asked.
func (w *world) heardChordPositions() (map[string][]int, []generated.PracticeSessionItem, error) {
	items, _, err := w.heardItems(generated.PracticeItemKindHeardChord)
	if err != nil {
		return nil, nil, err
	}
	positions := map[string][]int{}
	for i, item := range items {
		if item.Kind == generated.PracticeItemKindHeardChord {
			positions[item.ItemKey] = append(positions[item.ItemKey], i)
		}
	}
	return positions, items, nil
}

func (w *world) everyHeardChordAskedAgain() error {
	positions, _, err := w.heardChordPositions()
	if err != nil {
		return err
	}
	for key, at := range positions {
		if len(at) != 2 {
			return fmt.Errorf("%s is asked %d times, want twice", key, len(at))
		}
	}
	return nil
}

func (w *world) threeItemsBetweenComebacks() error {
	positions, _, err := w.heardChordPositions()
	if err != nil {
		return err
	}
	for key, at := range positions {
		if len(at) == 2 && at[1]-at[0]-1 < 3 {
			return fmt.Errorf("%s has %d items between its two times", key, at[1]-at[0]-1)
		}
	}
	return nil
}

func (w *world) secondTimeAskedAs(way string) error {
	positions, items, err := w.heardChordPositions()
	if err != nil {
		return err
	}
	for key, at := range positions {
		again := items[at[len(at)-1]].GeneratedChoice
		if again == nil || string(again.Way) != way || again.Playback == nil {
			return fmt.Errorf("%s is asked the second time as %v, want %s by ear", key, again, way)
		}
	}
	return nil
}

func (w *world) heardProgressionsTogether(a, b string) error {
	_, heard, err := w.heardItems(generated.PracticeItemKindHeardProgression)
	if err != nil {
		return err
	}
	ka, kb := domain.HeardProgressionItemKey(a), domain.HeardProgressionItemKey(b)
	first := slices.IndexFunc(heard, func(item generated.PracticeSessionItem) bool { return item.ItemKey == ka || item.ItemKey == kb })
	if first < 0 || first+1 >= len(heard) {
		return fmt.Errorf("%s and %s are not both in the session", a, b)
	}
	pair := []string{heard[first].ItemKey, heard[first+1].ItemKey}
	if !slices.Contains(pair, ka) || !slices.Contains(pair, kb) {
		return fmt.Errorf("the heard progressions after %s are %v", heard[first].ItemKey, pair)
	}
	return nil
}
