package domain

import (
	"slices"
	"strings"
)

// generatedChoiceV1 grades generated questions asked among options the server
// planned: a numeral's chord or a chord's numeral in a key, a chord's function, a
// progression read as a chart, a chord or a progression heard. It computes the right
// answer from the key, the numeral or the progression, checks that every option is
// a possible answer and that the right one and the pick are among them, and keeps
// the right answer and the options in the answer key.
type generatedChoiceV1 struct{}

func (generatedChoiceV1) ID() string { return "generated_choice.v1" }

// waysOf are the ways each kind is asked.
var waysOf = map[PracticeItemKind][]string{
	PracticeItemKindDiatonicChord:    {"numeral_to_chord", "chord_to_numeral"},
	PracticeItemKindChordFunction:    {"chord_to_function", "function_to_chord"},
	PracticeItemKindChartProgression: {"chart_to_numbers", "numbers_to_chart"},
	PracticeItemKindHeardChord:       {"cadence_then_chord"},
	PracticeItemKindHeardProgression: {"progression_by_number"},
}

// WaysOf are the ways a generated question of kind is asked, none for any other kind.
func WaysOf(kind PracticeItemKind) []string { return waysOf[kind] }

// keyedByTheAnswer are the kinds asked in any key: the key comes with the answer.
var keyedByTheAnswer = map[PracticeItemKind]bool{
	PracticeItemKindChartProgression: true,
	PracticeItemKindHeardChord:       true,
	PracticeItemKindHeardProgression: true,
}

// generatedQuestion is what a question asks: its key, and its numeral or progression.
type generatedQuestion struct {
	key            int
	numeral        string
	progressionKey string
	progression    []string
}

// answerSpace is what the options of one way can be: identify reads an option into
// a comparable value, and false when it is no possible answer to the way.
type answerSpace struct {
	identify func(option string) (string, bool)
	right    string
	answer   string
}

func (generatedChoiceV1) Grade(key PracticeItemKey, r PracticeResponse, ref PracticeReference) GradeResult {
	ways, ok := waysOf[key.Kind]
	if !ok || r.Type != PracticeResponseGeneratedChoice || r.LatencyMs == nil || !slices.Contains(ways, r.Way) {
		return rejected(GradeRejectionResponseDoesNotFitItem)
	}
	if keyedByTheAnswer[key.Kind] != (r.MusicKey != "") {
		return rejected(GradeRejectionResponseDoesNotFitItem)
	}

	q, rejection := questionOf(key, r, ref)
	if rejection != "" {
		return rejected(rejection)
	}
	space := answerSpaceOf(r.Way, q, ref)

	picked, valid := checkOptions(r, space)
	if !valid {
		return rejected(GradeRejectionInvalidChoices)
	}
	if r.Way == "function_to_chord" && !oneChordWithTheFunction(r.Options, q) {
		return rejected(GradeRejectionInvalidChoices)
	}

	correct := picked == space.right
	answerKey := &AnswerKey{Way: r.Way, Answer: space.answer, Offered: r.Options}
	if keyedByTheAnswer[key.Kind] {
		answerKey.MusicKey = r.MusicKey
	}
	return GradeResult{Evidence: GradedEvidence{
		Source:    EvidenceSourceAutoGraded,
		Correct:   &correct,
		LatencyMs: r.LatencyMs,
		AudioMs:   r.AudioMs,
		AnswerKey: answerKey,
	}}
}

// questionOf reads what the question asks from the item key and, for an item asked
// in any key, the key sent with the answer.
func questionOf(key PracticeItemKey, r PracticeResponse, ref PracticeReference) (generatedQuestion, GradeRejection) {
	musicKey := r.MusicKey
	var q generatedQuestion
	if progressionKey := key.ProgressionKey(); progressionKey != "" {
		p, ok := ref.Progressions[progressionKey]
		if !ok {
			return q, GradeRejectionUnknownReference
		}
		q.progressionKey, q.progression = p.Key, p.Bars
	} else if keyedByTheAnswer[key.Kind] {
		q.numeral = key.parts[0]
	} else {
		musicKey, q.numeral = key.parts[0], key.parts[1]
	}
	pc, ok := pitchClass(musicKey)
	if !ok {
		return q, GradeRejectionResponseDoesNotFitItem
	}
	q.key = pc
	return q, ""
}

// answerSpaceOf is what the options of way can be, and which of them is right.
func answerSpaceOf(way string, q generatedQuestion, ref PracticeReference) answerSpace {
	switch way {
	case "numeral_to_chord", "function_to_chord":
		right, _ := chordOf(q.key, q.numeral)
		return answerSpace{identify: chordOfTheKey(q.key), right: right.symbol(), answer: right.symbol()}
	case "chord_to_numeral", "cadence_then_chord":
		return answerSpace{identify: oneOf(keyNumerals), right: q.numeral, answer: q.numeral}
	case "chord_to_function":
		fn := numeralFunction[q.numeral]
		return answerSpace{identify: oneOf([]string{"tonic", "subdominant", "dominant"}), right: fn, answer: fn}
	case "numbers_to_chart":
		chart, _ := chartOf(q.key, q.progression)
		right := chartSymbol(chart)
		return answerSpace{identify: chartOfTheKey(q.key, ref), right: right, answer: right}
	}
	// chart_to_numbers and progression_by_number: the progression's key.
	return answerSpace{identify: catalogProgression(ref), right: q.progressionKey, answer: q.progressionKey}
}

// checkOptions reports the pick, read as the way reads it, and whether the options
// are different possible answers that include the right one and the pick.
func checkOptions(r PracticeResponse, space answerSpace) (string, bool) {
	seen := map[string]bool{}
	var picked string
	pickedAmong := false
	for _, option := range r.Options {
		value, ok := space.identify(option)
		if !ok || seen[value] {
			return "", false
		}
		seen[value] = true
		if option == r.Choice {
			picked, pickedAmong = value, true
		}
	}
	return picked, pickedAmong && seen[space.right]
}

// oneChordWithTheFunction reports whether the asked chord is the only option with
// its function.
func oneChordWithTheFunction(options []string, q generatedQuestion) bool {
	asked := numeralFunction[q.numeral]
	count := 0
	for _, option := range options {
		c, _ := parseChord(option, false)
		for _, numeral := range keyNumerals {
			if k, _ := chordOf(q.key, numeral); k == c && numeralFunction[numeral] == asked {
				count++
			}
		}
	}
	return count == 1
}

// chordOfTheKey reads an option as one of the key's triads, by its catalog symbol.
func chordOfTheKey(key int) func(string) (string, bool) {
	return func(option string) (string, bool) {
		c, ok := parseChord(option, false)
		if !ok {
			return "", false
		}
		for _, numeral := range keyNumerals {
			if k, _ := chordOf(key, numeral); k == c {
				return c.symbol(), true
			}
		}
		return "", false
	}
}

// chartOfTheKey reads an option as the chart, in the key, of a progression in ref.
func chartOfTheKey(key int, ref PracticeReference) func(string) (string, bool) {
	charts := map[string]bool{}
	for _, p := range ref.Progressions {
		if chart, ok := chartOf(key, p.Bars); ok {
			charts[chartSymbol(chart)] = true
		}
	}
	return func(option string) (string, bool) {
		symbols := strings.Split(option, " ")
		chart := make([]chord, len(symbols))
		for i, s := range symbols {
			c, ok := parseChord(s, true)
			if !ok {
				return "", false
			}
			chart[i] = c
		}
		value := chartSymbol(chart)
		return value, charts[value]
	}
}

// catalogProgression reads an option as a progression key in ref.
func catalogProgression(ref PracticeReference) func(string) (string, bool) {
	return func(option string) (string, bool) {
		_, ok := ref.Progressions[option]
		return option, ok
	}
}

// oneOf reads an option as one of values, spelled exactly.
func oneOf(values []string) func(string) (string, bool) {
	return func(option string) (string, bool) {
		return option, slices.Contains(values, option)
	}
}
