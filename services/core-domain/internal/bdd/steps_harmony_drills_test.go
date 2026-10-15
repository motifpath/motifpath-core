//go:build integration

package bdd

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/cucumber/godog"
	"gopkg.in/yaml.v3"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// chordCatalogFile is the generated chord catalog the chord catalog's
// migrations install.
const chordCatalogFile = "../../../../catalog/chord-voicings-v1/catalog.json"

// harmonyWorld is what the harmony drill steps share in a scenario: the
// question a step planned and how it was drawn.
type harmonyWorld struct {
	question  *domain.PlannedGeneratedChoice
	stimulus  domain.DrillStimulus
	itemKey   string
	listed    []string
	listedKey string
}

func (w *world) harmony() *harmonyWorld {
	if w.harmonyDrills == nil {
		w.harmonyDrills = &harmonyWorld{}
	}
	return w.harmonyDrills
}

func registerHarmonyDrillSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the diatonic chord (\w+) in (\S+) is planned as (\w+)$`, w.diatonicChordPlanned)
	sc.Step(`^the diatonic chord (\w+) in (\S+) is planned as (\w+) for a session that reads chords on "([^"]+)"$`, w.diatonicChordPlannedReading)
	sc.Step(`^the chord function (\w+) in (\S+) is planned as (\w+)$`, w.chordFunctionPlanned)
	sc.Step(`^the progression "([^"]+)" is planned as (\w+) in (\S+)$`, w.progressionPlanned)
	sc.Step(`^the progression "([^"]+)" is planned as (\w+) in (\S+) for a session that reads chords on "([^"]+)"$`, w.progressionPlannedReading)

	sc.Step(`^it offers four chords, all of them chords of (\S+) major$`, w.offersFourChordsOfKey)
	sc.Step(`^it offers four chords of (\S+) major$`, w.offersFourChordsOfKey)
	sc.Step(`^it offers four chord symbols, all of them chords of (\S+) major$`, w.offersFourChordSymbolsOfKey)
	sc.Step(`^one of them is (\S+) (major|minor)$`, w.oneOptionIsChord)
	sc.Step(`^the question names the key and the numeral, and no chord$`, w.namesKeyAndNumeral)
	sc.Step(`^no chord is drawn as a chord box$`, w.noChordBox)
	sc.Step(`^every option is drawn as a chord box with no chord name$`, w.everyOptionABox)
	sc.Step(`^the box of "([^"]+)" is the chord catalog's open voicing of (\S+)$`, w.boxIsOpenVoicing)
	sc.Step(`^"([^"]+)" is the only one of them that is (tonic|subdominant|dominant) in (\S+)$`, w.onlyOptionWithFunction)
	sc.Step(`^the chord it gives is "([^"]+)"$`, w.chordItGives)
	sc.Step(`^it offers tonic, subdominant and dominant$`, w.offersTheFunctions)
	sc.Step(`^its options are the charts "([^"]+)", "([^"]+)", "([^"]+)" and "([^"]+)"$`, w.optionsAreCharts)
	sc.Step(`^the question names "([^"]+)" and (\S+), and no chart$`, w.namesProgressionAndKey)
	sc.Step(`^its chart is drawn as chord symbols in three lines of four bars$`, w.twelveBarChartAsSymbols)
	sc.Step(`^it offers "([^"]+)" and "([^"]+)"$`, w.offersTwo)

	sc.Step(`^"([^"]+)" has started (\d+) sessions in the head so far$`, w.hasStartedHeadSessions)
	sc.Step(`^"([^"]+)" composes a session in the head with diatonic chords in it$`, w.composesHarmonySession)
	sc.Step(`^"([^"]+)" composes a session in the head whose new items include diatonic chords$`, w.composesShortHarmonySession)
	sc.Step(`^every diatonic chord in the session is drawn as chord (boxes|symbols)$`, w.everyDiatonicChordDrawnAs)
	sc.Step(`^"([^"]+)" has never practised a diatonic chord$`, w.neverPractisedDiatonicChords)
	sc.Step(`^every new diatonic chord in the session is in (\S+) major$`, w.newDiatonicChordsInKey)
	sc.Step(`^"([^"]+)" has picked the chord for (\w+) in (\S+) correctly twice and has never named its numeral$`, w.pickedChordTwice)
	sc.Step(`^the diatonic chord (\w+) in (\S+) is picked for "([^"]+)"'s session in the head$`, w.diatonicChordPickedInTheHead)
	sc.Step(`^"([^"]+)" is learning the chart progression "([^"]+)"$`, w.learningChartProgression)
	sc.Step(`^it is asked in one of (.+)$`, w.askedInOneOf)
	sc.Step(`^it is planned for "([^"]+)"$`, w.harmonyPlannedFor)
	sc.Step(`^every diatonic chord and chord function item of (\S+) major is listed$`, w.harmonyItemsListed)
	sc.Step(`^there is one for each of (.+)$`, w.oneForEachNumeral)
	sc.Step(`^none is for (\w+)$`, w.noneForNumeral)
}

// planHarmony plans item as way, keeping the question and its stimulus.
func (w *world) planHarmony(item domain.DrillItem, way string, planning domain.DrillPlanning) error {
	if !slices.Contains(item.Ways(), way) {
		return fmt.Errorf("%s is not asked as %s", item.ItemKey(), way)
	}
	planned := item.Plan(way, planning)
	var session domain.PracticeSessionItem
	planned.AttachTo(&session)
	h := w.harmony()
	h.question, h.stimulus, h.itemKey = session.GeneratedChoice, planned.Stimulus(), item.ItemKey()
	return nil
}

// readingOn is how a session reading chords on instrument plans them.
func (w *world) readingOn(instrument string) domain.DrillPlanning {
	if instrument == "electric-bass" {
		return domain.DrillPlanning{Pick: rand.IntN, Reading: domain.GeneratedChordSymbols}
	}
	return domain.DrillPlanning{Pick: rand.IntN, Reading: domain.GeneratedChordBoxes, Voicing: catalogChordBox}
}

func (w *world) diatonicChordPlanned(numeral, key, way string) error {
	return w.planHarmony(domain.DiatonicChordItem{Key: key, Numeral: numeral}, way, domain.DrillPlanning{Pick: rand.IntN})
}

func (w *world) diatonicChordPlannedReading(numeral, key, way, instrument string) error {
	return w.planHarmony(domain.DiatonicChordItem{Key: key, Numeral: numeral}, way, w.readingOn(instrument))
}

func (w *world) chordFunctionPlanned(numeral, key, way string) error {
	return w.planHarmony(domain.ChordFunctionItem{Key: key, Numeral: numeral}, way, domain.DrillPlanning{Pick: rand.IntN})
}

func (w *world) progressionPlanned(progression, way, key string) error {
	return w.progressionPlannedReading(progression, way, key, "electric-bass")
}

// progressionPlannedReading plans the chart progression in key: an
// accurate student's question may be asked in any key, and its first pick
// is the key.
func (w *world) progressionPlannedReading(progression, way, key, instrument string) error {
	item, err := catalogChartItem(progression)
	if err != nil {
		return err
	}
	planning := w.readingOn(instrument)
	keyIndex := slices.Index(domain.MusicKeys, key)
	if keyIndex < 0 {
		return fmt.Errorf("no key %q", key)
	}
	first := true
	planning.Pick = func(n int) int {
		if first {
			first = false
			return keyIndex
		}
		return rand.IntN(n)
	}
	planning.Level = domain.KnowledgeLevelAccurate
	return w.planHarmony(item, way, planning)
}

func (w *world) plannedQuestion() (*domain.PlannedGeneratedChoice, error) {
	q := w.harmony().question
	if q == nil {
		return nil, fmt.Errorf("no harmony question was planned")
	}
	return q, nil
}

func (w *world) offersFourChordsOfKey(key string) error {
	q, err := w.plannedQuestion()
	if err != nil {
		return err
	}
	if len(q.Options) != 4 {
		return fmt.Errorf("it offers %d options, want 4", len(q.Options))
	}
	chords := chordsOfKey(key)
	for _, o := range q.Options {
		if !slices.Contains(chords, o.Value) {
			return fmt.Errorf("%q is not a chord of %s major %v", o.Value, key, chords)
		}
	}
	return nil
}

func (w *world) offersFourChordSymbolsOfKey(key string) error {
	if err := w.offersFourChordsOfKey(key); err != nil {
		return err
	}
	return w.noChordBox()
}

func (w *world) oneOptionIsChord(root, quality string) error {
	q, err := w.plannedQuestion()
	if err != nil {
		return err
	}
	chord := root
	if quality == "minor" {
		chord += "m"
	}
	if !slices.ContainsFunc(q.Options, func(o domain.GeneratedOption) bool { return o.Value == chord }) {
		return fmt.Errorf("%s is not offered", chord)
	}
	return nil
}

func (w *world) namesKeyAndNumeral() error {
	q, err := w.plannedQuestion()
	if err != nil {
		return err
	}
	if q.Numeral == "" || q.Chord != "" {
		return fmt.Errorf("the question gives numeral %q and chord %q", q.Numeral, q.Chord)
	}
	if !slices.Equal(w.harmony().stimulus.Prompt, []string{q.MusicKey, q.Numeral}) {
		return fmt.Errorf("the prompt says %v", w.harmony().stimulus.Prompt)
	}
	return domain.CheckChallenge(w.harmony().stimulus)
}

func (w *world) noChordBox() error {
	q, err := w.plannedQuestion()
	if err != nil {
		return err
	}
	if q.DrawnAs != domain.GeneratedChordSymbols || len(q.Voicings) > 0 {
		return fmt.Errorf("it draws %s with %d boxes", q.DrawnAs, len(q.Voicings))
	}
	return nil
}

func (w *world) everyOptionABox() error {
	q, err := w.plannedQuestion()
	if err != nil {
		return err
	}
	if q.DrawnAs != domain.GeneratedChordBoxes {
		return fmt.Errorf("it draws %s", q.DrawnAs)
	}
	for _, o := range q.Options {
		if o.Names != nil {
			return fmt.Errorf("option %q is named", o.Value)
		}
		if !slices.ContainsFunc(q.Voicings, func(b domain.ChordBox) bool { return b.Chord == o.Value }) {
			return fmt.Errorf("option %q has no box", o.Value)
		}
	}
	return nil
}

func (w *world) boxIsOpenVoicing(chord, of string) error {
	q, err := w.plannedQuestion()
	if err != nil {
		return err
	}
	open, err := catalogOpenVoicing(of)
	if err != nil {
		return err
	}
	for _, b := range q.Voicings {
		if b.Chord == chord {
			if b.DiagramID != open {
				return fmt.Errorf("%s is drawn with %s, want the open voicing %s", chord, b.DiagramID, open)
			}
			return nil
		}
	}
	return fmt.Errorf("%s is not drawn", chord)
}

func (w *world) onlyOptionWithFunction(chord, function, key string) error {
	q, err := w.plannedQuestion()
	if err != nil {
		return err
	}
	for _, o := range q.Options {
		has := functionOfChord(key, o.Value) == domain.ChordFunction(function)
		if has != (o.Value == chord) {
			return fmt.Errorf("option %q: %s in %s is %v", o.Value, function, key, has)
		}
	}
	if q.Answer != chord {
		return fmt.Errorf("the answer is %q, want %q", q.Answer, chord)
	}
	return domain.CheckChallenge(w.harmony().stimulus)
}

func (w *world) chordItGives(chord string) error {
	q, err := w.plannedQuestion()
	if err != nil {
		return err
	}
	if q.Chord != chord {
		return fmt.Errorf("it gives %q, want %q", q.Chord, chord)
	}
	return nil
}

func (w *world) offersTheFunctions() error {
	q, err := w.plannedQuestion()
	if err != nil {
		return err
	}
	var values []string
	for _, o := range q.Options {
		values = append(values, o.Value)
	}
	slices.Sort(values)
	if !slices.Equal(values, []string{"dominant", "subdominant", "tonic"}) {
		return fmt.Errorf("it offers %v", values)
	}
	return domain.CheckChallenge(w.harmony().stimulus)
}

func (w *world) optionsAreCharts(a, b, c, d string) error {
	q, err := w.plannedQuestion()
	if err != nil {
		return err
	}
	var values []string
	for _, o := range q.Options {
		values = append(values, o.Value)
	}
	want := []string{a, b, c, d}
	slices.Sort(values)
	slices.Sort(want)
	if !slices.Equal(values, want) {
		return fmt.Errorf("its options are %v, want %v", values, want)
	}
	return domain.CheckChallenge(w.harmony().stimulus)
}

func (w *world) namesProgressionAndKey(progression, key string) error {
	q, err := w.plannedQuestion()
	if err != nil {
		return err
	}
	if q.Progression == nil || q.Progression.Key != progression || q.MusicKey != key || len(q.Chart) > 0 {
		return fmt.Errorf("the question names %+v in %s with chart %v", q.Progression, q.MusicKey, q.Chart)
	}
	return nil
}

func (w *world) twelveBarChartAsSymbols() error {
	q, err := w.plannedQuestion()
	if err != nil {
		return err
	}
	if len(q.Chart) != 12 {
		return fmt.Errorf("the chart has %d bars", len(q.Chart))
	}
	return w.noChordBox()
}

func (w *world) offersTwo(a, b string) error {
	q, err := w.plannedQuestion()
	if err != nil {
		return err
	}
	var values []string
	for _, o := range q.Options {
		values = append(values, o.Value)
	}
	slices.Sort(values)
	want := []string{a, b}
	slices.Sort(want)
	if !slices.Equal(values, want) {
		return fmt.Errorf("it offers %v, want %v", values, want)
	}
	return domain.CheckChallenge(w.harmony().stimulus)
}

// learnsHarmony puts name on a path teaching the harmony drills' nodes,
// and turns on their items.
func (w *world) learnsHarmony(name string) {
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	var skills []domain.KnowledgeNode
	for _, key := range []string{"diatonic-chords", "roman-numerals", "chord-function", "read-chord-charts"} {
		skills = append(skills, domain.KnowledgeNode{ID: w.skillIDFor(key).String()})
	}
	node := domain.ContentNode{
		ID: nodeID("harmony-lesson").String(), Title: "harmony-lesson", ContentType: domain.ContentTypeArticle,
		Classification: domain.Classification{Skills: skills},
	}
	w.nodes.put(node)
	w.studentPaths.put(domain.StudentPath{
		ID: deterministicUUID("student-path", name+"/harmony").String(), StudentID: studentID,
		SourceTemplateID: pathID("harmony-path").String(), AssignedAt: fixedNow,
		Items: []domain.StudentPathItemRecord{{Position: 1, ContentNodeID: node.ID}},
	})
	w.paths.put(domain.LearningPath{ID: pathID("harmony-path").String()})
	w.practiceItems.harmony = w.harmonyClassified
}

// harmonyClassified classifies every harmony item under the world's skills.
func (w *world) harmonyClassified() []domain.ClassifiedItem {
	nodes := func(kind domain.PracticeItemKind) []string {
		var ids []string
		for _, key := range domain.HarmonyNodeKeys[kind] {
			ids = append(ids, w.skillIDFor(key).String())
		}
		return ids
	}
	var items []domain.ClassifiedItem
	for _, key := range domain.MusicKeys {
		for _, numeral := range domain.HarmonyItemNumerals {
			items = append(items,
				domain.ClassifiedItem{ItemKey: domain.DiatonicChordItem{Key: key, Numeral: numeral}.ItemKey(), NodeIDs: nodes(domain.PracticeItemKindDiatonicChord)},
				domain.ClassifiedItem{ItemKey: domain.ChordFunctionItem{Key: key, Numeral: numeral}.ItemKey(), NodeIDs: nodes(domain.PracticeItemKindChordFunction)})
		}
	}
	progressions, err := catalogProgressions{}.List(context.Background())
	if err == nil {
		for _, p := range progressions {
			items = append(items, domain.ClassifiedItem{ItemKey: domain.ChartProgressionItemKey(p.Key), NodeIDs: nodes(domain.PracticeItemKindChartProgression)})
		}
	}
	return items
}

func (w *world) composesHarmonySession(name string) error {
	w.learnsHarmony(name)
	return w.composeSession(15, nil)
}

// composesShortHarmonySession composes a 5-minute session in the head,
// whose new share holds fewer chords than one key has.
func (w *world) composesShortHarmonySession(name string) error {
	w.learnsHarmony(name)
	return w.composeSession(5, nil)
}

func (w *world) plannedHarmony(kind generated.PracticeItemKind) ([]generated.PracticeSessionItem, error) {
	plan, err := w.composedPlan()
	if err != nil {
		return nil, err
	}
	var items []generated.PracticeSessionItem
	for _, item := range plan.Items {
		if item.Kind == kind {
			if item.GeneratedChoice == nil {
				return nil, fmt.Errorf("%s has no generated question", item.ItemKey)
			}
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("the session has no %s", kind)
	}
	return items, nil
}

func (w *world) everyDiatonicChordDrawnAs(reading string) error {
	items, err := w.plannedHarmony(generated.PracticeItemKindDiatonicChord)
	if err != nil {
		return err
	}
	want := generated.ChordBoxes
	if reading == "symbols" {
		want = generated.ChordSymbols
	}
	for _, item := range items {
		if item.GeneratedChoice.DrawnAs != want {
			return fmt.Errorf("%s is drawn as %s, want %s", item.ItemKey, item.GeneratedChoice.DrawnAs, want)
		}
	}
	return nil
}

func (w *world) neverPractisedDiatonicChords(string) error { return nil }

func (w *world) newDiatonicChordsInKey(key string) error {
	items, err := w.plannedHarmony(generated.PracticeItemKindDiatonicChord)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.Reason == generated.PracticePickReasonNew && !strings.HasPrefix(item.ItemKey, "diatonic_chord:"+key+":") {
			return fmt.Errorf("new item %s is not in %s major", item.ItemKey, key)
		}
	}
	return nil
}

func (w *world) pickedChordTwice(name, numeral, key string) error {
	w.learnsHarmony(name)
	due := fixedNow.AddDate(0, 0, -1)
	w.practiceStates.put(w.ensureRegistered(name, domain.RoleStudent).String(), domain.PracticeItemState{
		ItemKey: domain.DiatonicChordItem{Key: key, Numeral: numeral}.ItemKey(), Level: domain.KnowledgeLevelLearning,
		Counted: 2, Box: 1, DueAt: &due, RightByResponse: map[string]int{"numeral_to_chord": 2},
	})
	return nil
}

func (w *world) diatonicChordPickedInTheHead(numeral, key, name string) error {
	w.harmony().itemKey = domain.DiatonicChordItem{Key: key, Numeral: numeral}.ItemKey()
	w.lastShapeKey, w.lastCell = "", nil
	return w.composeSession(15, nil)
}

func (w *world) learningChartProgression(name, progression string) error {
	w.learnsHarmony(name)
	due := fixedNow.AddDate(0, 0, -1)
	key := domain.ChartProgressionItemKey(progression)
	w.practiceStates.put(w.ensureRegistered(name, domain.RoleStudent).String(), domain.PracticeItemState{
		ItemKey: key, Level: domain.KnowledgeLevelLearning, Counted: 2, Box: 1, DueAt: &due,
	})
	w.harmony().itemKey = key
	return nil
}

// harmonyItemPlanned composes a session in the head and finds the item a
// step named in it.
func (w *world) harmonyItemPlanned() (generated.PracticeSessionItem, error) {
	key := w.harmony().itemKey
	if w.lastResp == nil {
		if err := w.composeSession(15, nil); err != nil {
			return generated.PracticeSessionItem{}, err
		}
	}
	plan, err := w.composedPlan()
	if err != nil {
		return generated.PracticeSessionItem{}, err
	}
	for _, item := range plan.Items {
		if item.ItemKey == key && item.GeneratedChoice != nil {
			return item, nil
		}
	}
	return generated.PracticeSessionItem{}, fmt.Errorf("expected %s in the session", key)
}

// harmonyPlannedFor composes a session in the head in which the item a
// step named is planned.
func (w *world) harmonyPlannedFor(string) error {
	return w.composeSession(15, nil)
}

func (w *world) askedInOneOf(list string) error {
	item, err := w.harmonyItemPlanned()
	if err != nil {
		return err
	}
	keys := strings.FieldsFunc(strings.ReplaceAll(list, " and ", ","), func(r rune) bool { return r == ',' || r == ' ' })
	if !slices.Contains(keys, string(item.GeneratedChoice.MusicKey)) {
		return fmt.Errorf("it is asked in %s, want one of %v", item.GeneratedChoice.MusicKey, keys)
	}
	return nil
}

// harmonyIsAskedAs is "it is asked as" for a harmony item a step picked.
func (w *world) harmonyIsAskedAs(way string) error {
	item, err := w.harmonyItemPlanned()
	if err != nil {
		return err
	}
	if string(item.GeneratedChoice.Way) != way {
		return fmt.Errorf("%s is asked as %s, want %s", item.ItemKey, item.GeneratedChoice.Way, way)
	}
	return nil
}

func (w *world) harmonyItemsListed(key string) error {
	h := w.harmony()
	h.listed, h.listedKey = nil, key
	for _, item := range w.harmonyClassified() {
		if strings.HasPrefix(item.ItemKey, "diatonic_chord:"+key+":") || strings.HasPrefix(item.ItemKey, "chord_function:"+key+":") {
			h.listed = append(h.listed, item.ItemKey)
		}
	}
	return nil
}

func (w *world) oneForEachNumeral(list string) error {
	h := w.harmony()
	numerals := strings.FieldsFunc(strings.ReplaceAll(list, " and ", ","), func(r rune) bool { return r == ',' || r == ' ' })
	for _, prefix := range []string{"diatonic_chord:", "chord_function:"} {
		for _, n := range numerals {
			if !slices.Contains(h.listed, prefix+h.listedKey+":"+n) {
				return fmt.Errorf("no %s%s:%s item", prefix, h.listedKey, n)
			}
		}
	}
	if len(h.listed) != 2*len(numerals) {
		return fmt.Errorf("%d items listed, want %d", len(h.listed), 2*len(numerals))
	}
	return nil
}

func (w *world) noneForNumeral(numeral string) error {
	for _, key := range w.harmony().listed {
		if strings.HasSuffix(key, ":"+numeral) {
			return fmt.Errorf("%s is listed", key)
		}
	}
	return nil
}

func chordsOfKey(key string) []string {
	chords := make([]string, len(domain.ChordNumerals))
	for i, n := range domain.ChordNumerals {
		chords[i] = domain.NumeralChord(key, n)
	}
	return chords
}

func functionOfChord(key, chord string) domain.ChordFunction {
	for _, n := range domain.ChordNumerals {
		if domain.NumeralChord(key, n) == chord {
			return domain.NumeralFunction(n)
		}
	}
	return ""
}

// planEveryHarmonyItem plans every harmony item in every way, level and
// reading, checking each question.
func (w *world) planEveryHarmonyItem() error {
	progressions, err := catalogProgressions{}.List(context.Background())
	if err != nil {
		return err
	}
	var items []domain.DrillItem
	for _, key := range domain.MusicKeys {
		for _, numeral := range domain.HarmonyItemNumerals {
			items = append(items, domain.DiatonicChordItem{Key: key, Numeral: numeral}, domain.ChordFunctionItem{Key: key, Numeral: numeral})
		}
	}
	for _, p := range progressions {
		items = append(items, domain.ChartProgressionItem{Progression: p, Catalog: progressions})
	}
	for _, item := range items {
		for _, way := range item.Ways() {
			for _, level := range []domain.KnowledgeLevel{domain.KnowledgeLevelLearning, domain.KnowledgeLevelFluent} {
				for _, instrument := range []string{"guitar", "electric-bass"} {
					planning := w.readingOn(instrument)
					planning.Level = level
					w.checkPlanned(item, way, planning)
				}
			}
		}
	}
	return nil
}

// catalogProgressions is a ports.ProgressionRepository over motifpath-specs'
// progression catalog.
type catalogProgressions struct{}

type progressionCatalogFile struct {
	Progressions []struct {
		Key        string            `yaml:"key"`
		Bars       []string          `yaml:"bars"`
		Nodes      []string          `yaml:"nodes"`
		Confusable []string          `yaml:"confusable"`
		Names      map[string]string `yaml:"names"`
		ID         string            `yaml:"id"`
	} `yaml:"progressions"`
}

func (catalogProgressions) List(context.Context) ([]domain.Progression, error) {
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(featuresBase), "catalogs", "progressions.yaml"))
	if err != nil {
		return nil, err
	}
	var file progressionCatalogFile
	if err := yaml.Unmarshal(raw, &file); err != nil {
		return nil, err
	}
	progressions := make([]domain.Progression, len(file.Progressions))
	for i, p := range file.Progressions {
		progressions[i] = domain.Progression{ID: p.ID, Key: p.Key, Mode: "major", Bars: p.Bars, Confusable: p.Confusable, Names: domain.LocalizedText(p.Names)}
	}
	return progressions, nil
}

func catalogChartItem(progression string) (domain.ChartProgressionItem, error) {
	progressions, err := catalogProgressions{}.List(context.Background())
	if err != nil {
		return domain.ChartProgressionItem{}, err
	}
	i := slices.IndexFunc(progressions, func(p domain.Progression) bool { return p.Key == progression })
	if i < 0 {
		return domain.ChartProgressionItem{}, fmt.Errorf("no progression %q in the catalog", progression)
	}
	return domain.ChartProgressionItem{Progression: progressions[i], Catalog: progressions}, nil
}

// generatedChordCatalog is the generated chord catalog, read once.
type generatedChordCatalog struct {
	Chords []struct {
		ID             string `json:"chord_definition_id"`
		Symbol         string `json:"canonical_symbol"`
		Quality        string `json:"quality"`
		RootPitchClass int    `json:"root_pitch_class"`
		Bass           *int   `json:"bass_pitch_class"`
	} `json:"chords"`
	Voicings []struct {
		ChordID     string  `json:"chord_definition_id"`
		DiagramID   string  `json:"diagram_id"`
		Difficulty  string  `json:"difficulty"`
		LowestFret  int     `json:"lowest_fret"`
		TemplateKey *string `json:"template_key"`
		Rank        int     `json:"recommended_rank"`
	} `json:"voicings"`
}

var (
	chordCatalogOnce sync.Once
	chordCatalogByID map[string]domain.ChordDefinition
	chordCatalogErr  error
)

func loadChordCatalog() (map[string]domain.ChordDefinition, error) {
	chordCatalogOnce.Do(func() {
		raw, err := os.ReadFile(chordCatalogFile)
		if err != nil {
			chordCatalogErr = err
			return
		}
		var file generatedChordCatalog
		if err := json.Unmarshal(raw, &file); err != nil {
			chordCatalogErr = err
			return
		}
		chordCatalogByID = map[string]domain.ChordDefinition{}
		for _, c := range file.Chords {
			if c.Bass != nil {
				continue
			}
			chordCatalogByID[c.ID] = domain.ChordDefinition{ID: c.ID, CanonicalSymbol: c.Symbol, RootPitchClass: c.RootPitchClass, Quality: domain.ChordQuality(c.Quality)}
		}
		slices.SortFunc(file.Voicings, func(a, b struct {
			ChordID     string  `json:"chord_definition_id"`
			DiagramID   string  `json:"diagram_id"`
			Difficulty  string  `json:"difficulty"`
			LowestFret  int     `json:"lowest_fret"`
			TemplateKey *string `json:"template_key"`
			Rank        int     `json:"recommended_rank"`
		}) int {
			return a.Rank - b.Rank
		})
		for _, v := range file.Voicings {
			c, ok := chordCatalogByID[v.ChordID]
			if !ok {
				continue
			}
			c.Voicings = append(c.Voicings, domain.ChordVoicing{
				ChordDefinitionID: v.ChordID, DiagramID: v.DiagramID, InstrumentID: instrumentID("guitar").String(),
				Difficulty: v.Difficulty, LowestFret: v.LowestFret, TemplateKey: v.TemplateKey, Status: domain.ChordVoicingActive,
			})
			chordCatalogByID[v.ChordID] = c
		}
	})
	return chordCatalogByID, chordCatalogErr
}

// catalogChordBox is the box the generated chord catalog draws chord with.
func catalogChordBox(chord string) (string, bool) {
	c, ok := catalogChord(chord)
	if !ok {
		return "", false
	}
	v, ok := domain.PreferredChordBox(c.Voicings)
	return v.DiagramID, ok
}

func catalogChord(symbol string) (domain.ChordDefinition, bool) {
	chords, err := loadChordCatalog()
	if err != nil {
		return domain.ChordDefinition{}, false
	}
	reading := domain.ParseChordSymbol(symbol)
	if reading.Parsed == nil {
		return domain.ChordDefinition{}, false
	}
	for _, c := range chords {
		if c.RootPitchClass == reading.Parsed.RootPitchClass && c.Quality == reading.Parsed.Quality {
			return c, true
		}
	}
	return domain.ChordDefinition{}, false
}

func catalogOpenVoicing(symbol string) (string, error) {
	c, ok := catalogChord(symbol)
	if !ok {
		return "", fmt.Errorf("no chord %q in the chord catalog", symbol)
	}
	for _, v := range c.Voicings {
		if v.TemplateKey == nil {
			return v.DiagramID, nil
		}
	}
	return "", fmt.Errorf("%s has no open voicing", symbol)
}

// catalogChordRepository is a ports.ChordCatalogRepository over the
// generated chord catalog, enough for the harmony drills to find boxes.
type catalogChordRepository struct{ *fakeChordCatalog }

func (catalogChordRepository) FindChord(_ context.Context, rootPitchClass int, quality domain.ChordQuality, bassPitchClass *int) (domain.ChordDefinition, error) {
	chords, err := loadChordCatalog()
	if err != nil {
		return domain.ChordDefinition{}, err
	}
	if bassPitchClass != nil {
		return domain.ChordDefinition{}, domain.ErrNotFound
	}
	for _, c := range chords {
		if c.RootPitchClass == rootPitchClass && c.Quality == quality {
			return c, nil
		}
	}
	return domain.ChordDefinition{}, domain.ErrNotFound
}

// harmonyLayouts is a ports.FretboardLayoutReader: guitars play on the
// guitar layout, the bass on its own.
type harmonyLayouts struct{}

func (harmonyLayouts) FretboardLayouts(_ context.Context, id string) ([]domain.Instrument, error) {
	switch id {
	case instrumentID("guitar").String(), instrumentID("electric-guitar").String():
		return []domain.Instrument{{ID: instrumentID("guitar").String()}}, nil
	case instrumentID("electric-bass").String():
		return []domain.Instrument{{ID: instrumentID("electric-bass").String()}}, nil
	}
	return nil, nil
}
