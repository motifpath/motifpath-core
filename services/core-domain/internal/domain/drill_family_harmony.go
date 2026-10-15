package domain

import (
	"slices"
	"strings"
)

// The harmony drills: a numeral's chord in a key, its function, and a
// progression read as a chart. They are generated from the major keys and
// the progression catalog, and recalled in the head on any instrument.
const (
	PracticeItemKindDiatonicChord    PracticeItemKind = "diatonic_chord"
	PracticeItemKindChordFunction    PracticeItemKind = "chord_function"
	PracticeItemKindChartProgression PracticeItemKind = "chart_progression"
)

// HarmonyItemSeconds is how long a numeral question takes in a session.
const HarmonyItemSeconds = 8

// ChartItemSeconds is how long a chart question takes: a whole chart is
// read before answering.
const ChartItemSeconds = 12

// harmonyOptions is how many options a harmony question offers at most.
const harmonyOptions = 4

// GeneratedChordDrawing is how a harmony question draws its chords.
type GeneratedChordDrawing string

const (
	// GeneratedChordBoxes draws a chord box per chord, unnamed until
	// answered: the guitar reading.
	GeneratedChordBoxes GeneratedChordDrawing = "chord_boxes"
	// GeneratedChordSymbols writes each chord's symbol: the bass reading,
	// and every 12-bar chart.
	GeneratedChordSymbols GeneratedChordDrawing = "chord_symbols"
)

// ChordFunctionNames name each function in the student's language, with
// what it does, as a function question offers it.
var ChordFunctionNames = map[ChordFunction]LocalizedText{
	ChordFunctionTonic:       {"en": "Tonic — at rest", "pt_BR": "Tônica — repouso"},
	ChordFunctionSubdominant: {"en": "Subdominant — moves away", "pt_BR": "Subdominante — se afasta"},
	ChordFunctionDominant:    {"en": "Dominant — pulls home", "pt_BR": "Dominante — puxa para casa"},
}

// ChordBox is the voicing diagram a chord is drawn with.
type ChordBox struct {
	Chord     string
	DiagramID string
}

// GeneratedOption is an option of a harmony question: the value sent back
// as the answer, its name where the value is a key (a function or a
// progression), and its chart for a pick-the-chart question.
type GeneratedOption struct {
	Value string
	Names LocalizedText
	Chart []string
}

// QuestionProgression is the progression a pick-the-chart question names.
type QuestionProgression struct {
	Key   string
	Names LocalizedText
}

// PlannedGeneratedChoice is a harmony question as a session asks it: the
// way, the key, what it gives (a numeral, a chord, a function, a
// progression or a chart, by way), how its chords are drawn, the options in
// the order shown and the box of every chord drawn. Answer is the right
// option's value, kept to check the plan and never shown.
type PlannedGeneratedChoice struct {
	AskedWay    string
	MusicKey    string
	Numeral     string
	Chord       string
	Function    ChordFunction
	Progression *QuestionProgression
	Chart       []string
	DrawnAs     GeneratedChordDrawing
	Options     []GeneratedOption
	Voicings    []ChordBox
	Answer      string
	itemKind    PracticeItemKind
}

// Way is the way the question is asked.
func (q *PlannedGeneratedChoice) Way() string { return q.AskedWay }

// AttachTo puts the question on item.
func (q *PlannedGeneratedChoice) AttachTo(item *PracticeSessionItem) {
	item.Kind = q.itemKind
	item.GeneratedChoice = q
}

// Stimulus says the key and what the question gives, and offers its
// options by name; it draws no fretboard place.
func (q *PlannedGeneratedChoice) Stimulus() DrillStimulus {
	s := DrillStimulus{Prompt: []string{q.MusicKey}}
	switch {
	case q.Numeral != "":
		s.Prompt = append(s.Prompt, q.Numeral)
	case q.Chord != "":
		s.Prompt = append(s.Prompt, q.Chord)
	case q.Function != "":
		s.Prompt = append(s.Prompt, localizedValues(ChordFunctionNames[q.Function])...)
	case q.Progression != nil:
		s.Prompt = append(s.Prompt, localizedValues(q.Progression.Names)...)
	}
	s.Prompt = append(s.Prompt, q.Chart...)
	for _, o := range q.Options {
		labels := []string{o.Value}
		if o.Names != nil {
			labels = localizedValues(o.Names)
		}
		s.Options = append(s.Options, StimulusOption{Key: o.Value, Labels: labels})
		if o.Value == q.Answer {
			s.Answer = DrillAnswer{Options: []string{o.Value}, Labels: labels}
		}
	}
	return s
}

// DiatonicChordItem is a numeral's chord in a major key, asked as the
// chord for the numeral or the numeral for the chord.
type DiatonicChordItem struct {
	Key     string
	Numeral string
}

// ItemKey is diatonic_chord:<key>:<numeral>.
func (i DiatonicChordItem) ItemKey() string {
	return string(PracticeItemKindDiatonicChord) + ":" + i.Key + ":" + i.Numeral
}

// Kind is a diatonic chord's.
func (DiatonicChordItem) Kind() PracticeItemKind { return PracticeItemKindDiatonicChord }

// Ways picks the chord first, then names the numeral.
func (DiatonicChordItem) Ways() []string { return []string{"numeral_to_chord", "chord_to_numeral"} }

// Seconds is HarmonyItemSeconds.
func (DiatonicChordItem) Seconds() int { return HarmonyItemSeconds }

// SuitsSession holds in the head only.
func (DiatonicChordItem) SuitsSession(inHand bool) bool { return !inHand }

// Plan asks for the chord among the chords of the key on the nearest
// numerals, or for the numeral among the numerals nearest it.
func (i DiatonicChordItem) Plan(way string, planning DrillPlanning) PlannedDrill {
	numerals := nearestNumerals(i.Numeral, planning.Pick)
	q := &PlannedGeneratedChoice{AskedWay: way, MusicKey: i.Key, itemKind: PracticeItemKindDiatonicChord}
	if way == "chord_to_numeral" {
		q.Chord = NumeralChord(i.Key, i.Numeral)
		q.Answer = i.Numeral
		q.Options = valueOptions(numerals)
		q.draw([]string{q.Chord}, planning)
	} else {
		q.Numeral = i.Numeral
		q.Answer = NumeralChord(i.Key, i.Numeral)
		q.Options = valueOptions(ProgressionChart(numerals, i.Key))
		q.draw(optionValues(q.Options), planning)
	}
	q.Options = shuffled(q.Options, planning.Pick)
	return q
}

// ChordFunctionItem is a numeral's function in a major key, asked as the
// function of its chord or its chord for the function.
type ChordFunctionItem struct {
	Key     string
	Numeral string
}

// ItemKey is chord_function:<key>:<numeral>.
func (i ChordFunctionItem) ItemKey() string {
	return string(PracticeItemKindChordFunction) + ":" + i.Key + ":" + i.Numeral
}

// Kind is a chord function's.
func (ChordFunctionItem) Kind() PracticeItemKind { return PracticeItemKindChordFunction }

// Ways names the function first, then picks the chord.
func (ChordFunctionItem) Ways() []string { return []string{"chord_to_function", "function_to_chord"} }

// Seconds is HarmonyItemSeconds.
func (ChordFunctionItem) Seconds() int { return HarmonyItemSeconds }

// SuitsSession holds in the head only.
func (ChordFunctionItem) SuitsSession(inHand bool) bool { return !inHand }

// Plan gives the key's triad and offers the three functions, or names the
// function and offers its chord among three chords of other functions.
func (i ChordFunctionItem) Plan(way string, planning DrillPlanning) PlannedDrill {
	function := NumeralFunction(i.Numeral)
	q := &PlannedGeneratedChoice{AskedWay: way, MusicKey: i.Key, itemKind: PracticeItemKindChordFunction}
	if way == "function_to_chord" {
		q.Function = function
		q.Answer = NumeralChord(i.Key, i.Numeral)
		var others []string
		for _, n := range ChordNumerals {
			if NumeralFunction(n) != function {
				others = append(others, NumeralChord(i.Key, n))
			}
		}
		q.Options = valueOptions(append([]string{q.Answer}, pickSome(others, harmonyOptions-1, planning.Pick)...))
		q.draw(optionValues(q.Options), planning)
	} else {
		q.Chord = NumeralChord(i.Key, i.Numeral)
		q.Answer = string(function)
		for _, f := range ChordFunctions {
			q.Options = append(q.Options, GeneratedOption{Value: string(f), Names: ChordFunctionNames[f]})
		}
		q.draw([]string{q.Chord}, planning)
	}
	q.Options = shuffled(q.Options, planning.Pick)
	return q
}

// ChartProgressionItem is a progression of the catalog read as a chart, in
// a key picked each time it is asked. Catalog is every progression, the
// confusables offered as options among them.
type ChartProgressionItem struct {
	Progression Progression
	Catalog     []Progression
}

// ItemKey is chart_progression:<progression key>.
func (i ChartProgressionItem) ItemKey() string { return ChartProgressionItemKey(i.Progression.Key) }

// ChartProgressionItemKey is the chart progression item of the progression
// with progressionKey.
func ChartProgressionItemKey(progressionKey string) string {
	return string(PracticeItemKindChartProgression) + ":" + progressionKey
}

// Kind is a chart progression's.
func (ChartProgressionItem) Kind() PracticeItemKind { return PracticeItemKindChartProgression }

// Ways reads the chart first, then picks it.
func (ChartProgressionItem) Ways() []string { return []string{"chart_to_numbers", "numbers_to_chart"} }

// Seconds is ChartItemSeconds.
func (ChartProgressionItem) Seconds() int { return ChartItemSeconds }

// SuitsSession holds in the head only.
func (ChartProgressionItem) SuitsSession(inHand bool) bool { return !inHand }

// ConfusableWith lists the items of the progressions it is mistaken for.
func (i ChartProgressionItem) ConfusableWith() []string {
	keys := make([]string, len(i.Progression.Confusable))
	for n, key := range i.Progression.Confusable {
		keys[n] = ChartProgressionItemKey(key)
	}
	return keys
}

// Plan picks the key (an easy one while the item is new or being learnt),
// then shows the chart and offers the progression among its confusables,
// or names the progression and offers their charts.
func (i ChartProgressionItem) Plan(way string, planning DrillPlanning) PlannedDrill {
	keys := MusicKeys
	if planning.Level == KnowledgeLevelNew || planning.Level == KnowledgeLevelLearning || planning.Level == "" {
		keys = EasyChartKeys
	}
	key := keys[planning.Pick(len(keys))]
	offered := i.confusables(planning.Pick)
	q := &PlannedGeneratedChoice{AskedWay: way, MusicKey: key, itemKind: PracticeItemKindChartProgression}
	if way == "numbers_to_chart" {
		q.Progression = &QuestionProgression{Key: i.Progression.Key, Names: i.Progression.Names}
		var drawn []string
		for _, p := range offered {
			chart := ProgressionChart(p.Bars, key)
			q.Options = append(q.Options, GeneratedOption{Value: strings.Join(chart, " "), Chart: chart})
			drawn = append(drawn, chart...)
		}
		q.Answer = q.Options[0].Value
		q.draw(drawn, planning)
	} else {
		q.Chart = ProgressionChart(i.Progression.Bars, key)
		q.Answer = i.Progression.Key
		for _, p := range offered {
			q.Options = append(q.Options, GeneratedOption{Value: p.Key, Names: p.Names})
		}
		q.draw(q.Chart, planning)
	}
	if len(i.Progression.Bars) > 4 {
		q.DrawnAs, q.Voicings = GeneratedChordSymbols, nil
	}
	q.Options = shuffled(q.Options, planning.Pick)
	return q
}

// confusables is the progression, then its confusables, then theirs, as
// many as fit in the options, all of its length. A tie at the last ring
// that doesn't fit whole is broken by pick.
func (i ChartProgressionItem) confusables(pick func(int) int) []Progression {
	byKey := map[string]Progression{}
	for _, p := range i.Catalog {
		byKey[p.Key] = p
	}
	offered := []Progression{i.Progression}
	seen := map[string]bool{i.Progression.Key: true}
	ring := []Progression{i.Progression}
	for len(ring) > 0 && len(offered) < harmonyOptions {
		var next []Progression
		for _, p := range ring {
			for _, key := range p.Confusable {
				c, ok := byKey[key]
				if ok && !seen[key] && len(c.Bars) == len(i.Progression.Bars) {
					seen[key] = true
					next = append(next, c)
				}
			}
		}
		offered = append(offered, pickSome(next, harmonyOptions-len(offered), pick)...)
		ring = next
	}
	return offered
}

// draw sets how the question draws chords: boxes on a guitar reading when
// every chord has one, chord symbols otherwise.
func (q *PlannedGeneratedChoice) draw(chords []string, planning DrillPlanning) {
	q.DrawnAs, q.Voicings = GeneratedChordSymbols, nil
	if planning.Reading != GeneratedChordBoxes || planning.Voicing == nil {
		return
	}
	var boxes []ChordBox
	for _, chord := range chords {
		if slices.ContainsFunc(boxes, func(b ChordBox) bool { return b.Chord == chord }) {
			continue
		}
		id, ok := planning.Voicing(chord)
		if !ok {
			return
		}
		boxes = append(boxes, ChordBox{Chord: chord, DiagramID: id})
	}
	q.DrawnAs, q.Voicings = GeneratedChordBoxes, boxes
}

// nearestNumerals is numeral and the three numerals nearest it around the
// scale, one step away before two, the one below before the one above; a
// tie that doesn't fit whole is broken by pick.
func nearestNumerals(numeral string, pick func(int) int) []string {
	at := slices.Index(ChordNumerals, numeral)
	n := len(ChordNumerals)
	near := []string{numeral}
	for step := 1; len(near) < harmonyOptions && step <= n/2; step++ {
		ring := []string{ChordNumerals[(at-step+n)%n], ChordNumerals[(at+step)%n]}
		if ring[0] == ring[1] {
			ring = ring[:1]
		}
		near = append(near, pickSome(ring, harmonyOptions-len(near), pick)...)
	}
	return near
}

// pickSome is up to count of choices, in their order: all of them when they
// fit, else count picked by pick.
func pickSome[T any](choices []T, count int, pick func(int) int) []T {
	if len(choices) <= count {
		return choices
	}
	left := make([]int, len(choices))
	for i := range left {
		left[i] = i
	}
	picked := make([]int, 0, count)
	for range count {
		n := pick(len(left))
		picked = append(picked, left[n])
		left = slices.Delete(left, n, n+1)
	}
	slices.Sort(picked)
	out := make([]T, len(picked))
	for n, i := range picked {
		out[n] = choices[i]
	}
	return out
}

// shuffled is options in the order pick draws them; a pick always taking
// the first keeps their order.
func shuffled(options []GeneratedOption, pick func(int) int) []GeneratedOption {
	left := slices.Clone(options)
	out := make([]GeneratedOption, 0, len(options))
	for len(left) > 1 {
		i := pick(len(left))
		out = append(out, left[i])
		left = slices.Delete(left, i, i+1)
	}
	return append(out, left...)
}

func valueOptions(values []string) []GeneratedOption {
	options := make([]GeneratedOption, len(values))
	for i, v := range values {
		options[i] = GeneratedOption{Value: v}
	}
	return options
}

func optionValues(options []GeneratedOption) []string {
	values := make([]string, len(options))
	for i, o := range options {
		values[i] = o.Value
	}
	return values
}

// ParseDiatonicChordItemKey reads a diatonic_chord item key; false for any
// other key, or one whose key or numeral isn't a harmony item's.
func ParseDiatonicChordItemKey(itemKey string) (DiatonicChordItem, bool) {
	key, numeral, ok := parseNumeralItemKey(PracticeItemKindDiatonicChord, itemKey)
	return DiatonicChordItem{Key: key, Numeral: numeral}, ok
}

// ParseChordFunctionItemKey reads a chord_function item key; false for any
// other key, or one whose key or numeral isn't a harmony item's.
func ParseChordFunctionItemKey(itemKey string) (ChordFunctionItem, bool) {
	key, numeral, ok := parseNumeralItemKey(PracticeItemKindChordFunction, itemKey)
	return ChordFunctionItem{Key: key, Numeral: numeral}, ok
}

func parseNumeralItemKey(kind PracticeItemKind, itemKey string) (key, numeral string, ok bool) {
	rest, found := strings.CutPrefix(itemKey, string(kind)+":")
	if !found {
		return "", "", false
	}
	key, numeral, found = strings.Cut(rest, ":")
	if !found || !slices.Contains(MusicKeys, key) || !slices.Contains(HarmonyItemNumerals, numeral) {
		return "", "", false
	}
	return key, numeral, true
}
