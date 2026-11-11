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

// generatedChoiceV2 grades as generatedChoiceV1 does, with minor keys, and with
// made-up wrong answers among a progression question's options: any progression
// of the asked length and mode is a possible answer.
type generatedChoiceV2 struct{}

func (generatedChoiceV2) ID() string { return "generated_choice.v2" }

func (generatedChoiceV2) Grade(key PracticeItemKey, r PracticeResponse, ref PracticeReference) GradeResult {
	return gradeGeneratedChoice(key, r, ref, true)
}

func (generatedChoiceV1) Grade(key PracticeItemKey, r PracticeResponse, ref PracticeReference) GradeResult {
	return gradeGeneratedChoice(key, r, ref, false)
}

// waysOf are the ways each kind is asked.
var waysOf = map[PracticeItemKind][]string{
	PracticeItemKindDiatonicChord:    {"numeral_to_chord", "chord_to_numeral"},
	PracticeItemKindChordFunction:    {"chord_to_function", "function_to_chord"},
	PracticeItemKindChartProgression: {"chart_to_numbers", "numbers_to_chart"},
	PracticeItemKindHeardChord:       {"cadence_then_chord", "see_then_name"},
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

// generatedQuestion is what a question asks: its key, and its numeral or
// progression with its mode.
type generatedQuestion struct {
	key            int
	numeral        string
	progressionKey string
	progression    []string
	mode           string
}

// answerSpace is what the options of one way can be: identify reads an option into
// a comparable value, and false when it is no possible answer to the way.
type answerSpace struct {
	identify func(option string) (string, bool)
	right    string
	answer   string
}

// gradeGeneratedChoice grades r under v1's rules, or v2's when v2 holds.
func gradeGeneratedChoice(key PracticeItemKey, r PracticeResponse, ref PracticeReference, v2 bool) GradeResult {
	ways, ok := waysOf[key.Kind]
	if !ok || r.Type != PracticeResponseGeneratedChoice || r.LatencyMs == nil || !slices.Contains(ways, r.Way) {
		return rejected(GradeRejectionResponseDoesNotFitItem)
	}
	if keyedByTheAnswer[key.Kind] != (r.MusicKey != "") {
		return rejected(GradeRejectionResponseDoesNotFitItem)
	}

	q, rejection := questionOf(key, r, ref, v2)
	if rejection != "" {
		return rejected(rejection)
	}
	space := answerSpaceOf(r.Way, q, ref, v2)

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
// in any key, the key sent with the answer. Under v2 a progression is asked in a
// key of its mode, minor keys included.
func questionOf(key PracticeItemKey, r PracticeResponse, ref PracticeReference, v2 bool) (generatedQuestion, GradeRejection) {
	musicKey := r.MusicKey
	var q generatedQuestion
	if progressionKey := key.ProgressionKey(); progressionKey != "" {
		p, ok := ref.Progressions[progressionKey]
		if !ok {
			return q, GradeRejectionUnknownReference
		}
		q.progressionKey, q.progression, q.mode = p.Key, p.Bars, modeOf(p)
		if v2 {
			pc, mode, ok := musicKeyOf(musicKey)
			if !ok || mode != q.mode {
				return q, GradeRejectionResponseDoesNotFitItem
			}
			q.key = pc
			return q, ""
		}
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
// Under v2 a progression question's options are any progression of its length
// and mode, made-up ones included.
func answerSpaceOf(way string, q generatedQuestion, ref PracticeReference, v2 bool) answerSpace {
	switch way {
	case "numeral_to_chord", "function_to_chord":
		right, _ := chordOf(q.key, q.numeral)
		return answerSpace{identify: chordOfTheKey(q.key), right: right.symbol(), answer: right.symbol()}
	case "chord_to_numeral", "cadence_then_chord", "see_then_name":
		return answerSpace{identify: oneOf(keyNumerals), right: q.numeral, answer: q.numeral}
	case "chord_to_function":
		fn := numeralFunction[q.numeral]
		return answerSpace{identify: oneOf([]string{"tonic", "subdominant", "dominant"}), right: fn, answer: fn}
	case "numbers_to_chart":
		chart, _ := chartOf(q.key, q.progression)
		right := chartSymbol(chart)
		identify := chartOfTheKey(q.key, ref)
		if v2 {
			identify = chartOfLength(len(q.progression))
		}
		return answerSpace{identify: identify, right: right, answer: right}
	}
	// chart_to_numbers and progression_by_number: the progression's key.
	identify := catalogProgression(ref)
	if v2 {
		identify = progressionOfShape(ref, len(q.progression), q.mode)
	}
	return answerSpace{identify: identify, right: q.progressionKey, answer: q.progressionKey}
}

// chartOfLength reads an option as a chart of bars chord symbols, whatever
// progression it is.
func chartOfLength(bars int) func(string) (string, bool) {
	return func(option string) (string, bool) {
		symbols := strings.Split(option, " ")
		if len(symbols) != bars {
			return "", false
		}
		chart := make([]chord, len(symbols))
		for i, s := range symbols {
			c, ok := parseChord(s, true)
			if !ok {
				return "", false
			}
			chart[i] = c
		}
		return chartSymbol(chart), true
	}
}

// progressionOfShape reads an option as a progression of bars bars in mode: a
// drill progression's key, or a made-up one's, its mode and ASCII numerals, such
// as major/I-ii-V.
func progressionOfShape(ref PracticeReference, bars int, mode string) func(string) (string, bool) {
	return func(option string) (string, bool) {
		if p, ok := ref.Progressions[option]; ok {
			return option, len(p.Bars) == bars && modeOf(p) == mode
		}
		optionMode, numerals, ok := strings.Cut(option, "/")
		if !ok || optionMode != mode {
			return "", false
		}
		parts := strings.Split(numerals, "-")
		if len(parts) != bars {
			return "", false
		}
		for _, part := range parts {
			if _, _, ok := readNumeral(asciiNumerals.Replace(part)); !ok {
				return "", false
			}
		}
		return option, true
	}
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
