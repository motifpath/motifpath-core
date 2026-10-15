package domain_test

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

// firstPick always takes the first of equally fitting choices, so a plan is
// predictable: options stay in the order they were chosen.
func firstPick(int) int { return 0 }

// picks takes the given choices in turn, then the first.
func picks(choices ...int) func(int) int {
	return func(n int) int {
		if len(choices) == 0 {
			return 0
		}
		next := choices[0]
		choices = choices[1:]
		return next % n
	}
}

// boxOf names a voicing diagram for every chord.
func boxOf(chord string) (string, bool) { return "box-" + chord, true }

var catalogProgressions = []domain.Progression{
	{Key: "I-V-vi-IV", Bars: []string{"I", "V", "vi", "IV"}, Confusable: []string{"I-vi-IV-V", "vi-IV-I-V"}, Names: domain.LocalizedText{"en": "I–V–vi–IV (pop)"}},
	{Key: "vi-IV-I-V", Bars: []string{"vi", "IV", "I", "V"}, Confusable: []string{"I-V-vi-IV"}, Names: domain.LocalizedText{"en": "vi–IV–I–V"}},
	{Key: "I-vi-IV-V", Bars: []string{"I", "vi", "IV", "V"}, Confusable: []string{"I-V-vi-IV", "I-vi-ii-V"}, Names: domain.LocalizedText{"en": "I–vi–IV–V (doo-wop)"}},
	{Key: "I-vi-ii-V", Bars: []string{"I", "vi", "ii", "V"}, Confusable: []string{"I-vi-IV-V"}, Names: domain.LocalizedText{"en": "I–vi–ii–V (turnaround)"}},
	{Key: "ii-V-I", Bars: []string{"ii", "V", "I", "I"}, Confusable: []string{"IV-V-I"}, Names: domain.LocalizedText{"en": "ii–V–I"}},
	{Key: "IV-V-I", Bars: []string{"IV", "V", "I", "I"}, Confusable: []string{"ii-V-I"}, Names: domain.LocalizedText{"en": "IV–V–I"}},
	{
		Key: "12-bar-blues", Bars: []string{"I7", "I7", "I7", "I7", "IV7", "IV7", "I7", "I7", "V7", "IV7", "I7", "V7"},
		Confusable: []string{"12-bar-blues-quick-change"}, Names: domain.LocalizedText{"en": "12-bar blues, slow change"},
	},
	{
		Key: "12-bar-blues-quick-change", Bars: []string{"I7", "IV7", "I7", "I7", "IV7", "IV7", "I7", "I7", "V7", "IV7", "I7", "V7"},
		Confusable: []string{"12-bar-blues"}, Names: domain.LocalizedText{"en": "12-bar blues, quick change"},
	},
}

func progression(key string) domain.Progression {
	i := slices.IndexFunc(catalogProgressions, func(p domain.Progression) bool { return p.Key == key })
	return catalogProgressions[i]
}

func plannedChoice(t *testing.T, item domain.DrillItem, way string, planning domain.DrillPlanning) (*domain.PlannedGeneratedChoice, domain.DrillStimulus) {
	t.Helper()
	planned := item.Plan(way, planning)
	require.Equal(t, way, planned.Way())
	var session domain.PracticeSessionItem
	planned.AttachTo(&session)
	require.Equal(t, item.Kind(), session.Kind)
	require.NotNil(t, session.GeneratedChoice)
	return session.GeneratedChoice, planned.Stimulus()
}

func optionValues(q *domain.PlannedGeneratedChoice) []string {
	values := make([]string, len(q.Options))
	for i, o := range q.Options {
		values[i] = o.Value
	}
	return values
}

func chordsOfKey(key string) []string {
	chords := make([]string, len(domain.ChordNumerals))
	for i, n := range domain.ChordNumerals {
		chords[i] = domain.NumeralChord(key, n)
	}
	return chords
}

func TestDiatonicChordItem_AsADrillItem(t *testing.T) {
	vi := domain.DiatonicChordItem{Key: "G", Numeral: "vi"}
	symbols := domain.DrillPlanning{Pick: firstPick, Reading: domain.GeneratedChordSymbols}

	t.Run("is a numeral of a key recalled in the head, both ways", func(t *testing.T) {
		assert.Equal(t, "diatonic_chord:G:vi", vi.ItemKey())
		assert.Equal(t, domain.PracticeItemKindDiatonicChord, vi.Kind())
		assert.Equal(t, []string{"numeral_to_chord", "chord_to_numeral"}, vi.Ways())
		assert.True(t, vi.SuitsSession(false))
		assert.False(t, vi.SuitsSession(true))
		assert.Positive(t, vi.Seconds())
	})

	t.Run("its key round-trips", func(t *testing.T) {
		item, ok := domain.ParseDiatonicChordItemKey("diatonic_chord:F#:viio")
		require.True(t, ok)
		assert.Equal(t, domain.DiatonicChordItem{Key: "F#", Numeral: "viio"}, item)
		_, ok = domain.ParseDiatonicChordItemKey("chord_function:G:vi")
		assert.False(t, ok)
	})

	t.Run("numeral to chord names the key and numeral, and offers four chords of the key", func(t *testing.T) {
		q, s := plannedChoice(t, vi, "numeral_to_chord", symbols)

		assert.Equal(t, "G", q.MusicKey)
		assert.Equal(t, "vi", q.Numeral)
		assert.Equal(t, "Em", q.Answer)
		assert.Equal(t, []string{"Em", "D", "F#dim", "C"}, optionValues(q), "the answer, then the numerals one step away (V, viio), then two (IV before I)")
		assert.Subset(t, chordsOfKey("G"), optionValues(q))
		assert.Equal(t, []string{"G", "vi"}, s.Prompt)
		require.NoError(t, domain.CheckChallenge(s))
	})

	t.Run("on a bass reading, chords are symbols with no box", func(t *testing.T) {
		q, _ := plannedChoice(t, vi, "numeral_to_chord", symbols)

		assert.Equal(t, domain.GeneratedChordSymbols, q.DrawnAs)
		assert.Empty(t, q.Voicings)
	})

	t.Run("on a guitar reading, every chord drawn gets its box", func(t *testing.T) {
		q, _ := plannedChoice(t, vi, "numeral_to_chord", domain.DrillPlanning{Pick: firstPick, Reading: domain.GeneratedChordBoxes, Voicing: boxOf})

		assert.Equal(t, domain.GeneratedChordBoxes, q.DrawnAs)
		assert.Equal(t, []domain.ChordBox{
			{Chord: "Em", DiagramID: "box-Em"}, {Chord: "D", DiagramID: "box-D"},
			{Chord: "F#dim", DiagramID: "box-F#dim"}, {Chord: "C", DiagramID: "box-C"},
		}, q.Voicings)
	})

	t.Run("a chord with no box falls back to symbols for the whole question", func(t *testing.T) {
		noDim := func(chord string) (string, bool) { return "box-" + chord, !strings.HasSuffix(chord, "dim") }
		q, _ := plannedChoice(t, vi, "numeral_to_chord", domain.DrillPlanning{Pick: firstPick, Reading: domain.GeneratedChordBoxes, Voicing: noDim})

		assert.Equal(t, domain.GeneratedChordSymbols, q.DrawnAs)
		assert.Empty(t, q.Voicings)
	})

	t.Run("chord to numeral gives the chord and offers the numerals nearest the answer", func(t *testing.T) {
		q, s := plannedChoice(t, vi, "chord_to_numeral", symbols)

		assert.Equal(t, "Em", q.Chord)
		assert.Empty(t, q.Numeral)
		assert.Equal(t, "vi", q.Answer)
		assert.Equal(t, []string{"vi", "V", "viio", "IV"}, optionValues(q))
		assert.Equal(t, []string{"G", "Em"}, s.Prompt)
		require.NoError(t, domain.CheckChallenge(s))
	})

	t.Run("options are shuffled by the planning's pick", func(t *testing.T) {
		q, _ := plannedChoice(t, vi, "chord_to_numeral", domain.DrillPlanning{Pick: picks(0, 0, 0, 3), Reading: domain.GeneratedChordSymbols})

		assert.ElementsMatch(t, []string{"vi", "V", "viio", "IV"}, optionValues(q))
		assert.NotEqual(t, []string{"vi", "V", "viio", "IV"}, optionValues(q))
	})
}

func TestChordFunctionItem_AsADrillItem(t *testing.T) {
	v := domain.ChordFunctionItem{Key: "G", Numeral: "V"}
	symbols := domain.DrillPlanning{Pick: firstPick, Reading: domain.GeneratedChordSymbols}

	t.Run("is a numeral's function recalled in the head, both ways", func(t *testing.T) {
		assert.Equal(t, "chord_function:G:V", v.ItemKey())
		assert.Equal(t, domain.PracticeItemKindChordFunction, v.Kind())
		assert.Equal(t, []string{"chord_to_function", "function_to_chord"}, v.Ways())
		assert.True(t, v.SuitsSession(false))
		assert.False(t, v.SuitsSession(true))
		item, ok := domain.ParseChordFunctionItemKey("chord_function:Bb:ii")
		require.True(t, ok)
		assert.Equal(t, domain.ChordFunctionItem{Key: "Bb", Numeral: "ii"}, item)
	})

	t.Run("chord to function gives the key's triad and offers the three functions by name", func(t *testing.T) {
		q, s := plannedChoice(t, v, "chord_to_function", symbols)

		assert.Equal(t, "D", q.Chord, "the triad, never D7")
		assert.Equal(t, "dominant", q.Answer)
		assert.ElementsMatch(t, []string{"tonic", "subdominant", "dominant"}, optionValues(q))
		for _, o := range q.Options {
			assert.NotEmpty(t, o.Names["en"])
			assert.NotEmpty(t, o.Names["pt_BR"])
		}
		require.NoError(t, domain.CheckChallenge(s))
	})

	t.Run("function to chord offers exactly one chord with the asked function", func(t *testing.T) {
		q, s := plannedChoice(t, v, "function_to_chord", symbols)

		assert.Equal(t, domain.ChordFunctionDominant, q.Function)
		assert.Equal(t, "D", q.Answer)
		assert.Len(t, q.Options, 4)
		assert.Subset(t, chordsOfKey("G"), optionValues(q))
		assert.NotContains(t, optionValues(q), "F#dim", "viio is dominant too")
		require.NoError(t, domain.CheckChallenge(s))
	})
}

func TestChartProgressionItem_AsADrillItem(t *testing.T) {
	pop := domain.ChartProgressionItem{Progression: progression("I-V-vi-IV"), Catalog: catalogProgressions}
	boxes := domain.DrillPlanning{Pick: firstPick, Reading: domain.GeneratedChordBoxes, Voicing: boxOf, Level: domain.KnowledgeLevelLearning}

	t.Run("is a progression read as a chart in the head, both ways, confused with its neighbours", func(t *testing.T) {
		assert.Equal(t, "chart_progression:I-V-vi-IV", pop.ItemKey())
		assert.Equal(t, domain.PracticeItemKindChartProgression, pop.Kind())
		assert.Equal(t, []string{"chart_to_numbers", "numbers_to_chart"}, pop.Ways())
		assert.Equal(t, []string{"chart_progression:I-vi-IV-V", "chart_progression:vi-IV-I-V"}, pop.ConfusableWith())
		assert.True(t, pop.SuitsSession(false))
		assert.False(t, pop.SuitsSession(true))
	})

	t.Run("read the chart shows the chart in the key and offers the progression and its confusables", func(t *testing.T) {
		q, s := plannedChoice(t, pop, "chart_to_numbers", boxes)

		assert.Equal(t, "C", q.MusicKey)
		assert.Equal(t, []string{"C", "G", "Am", "F"}, q.Chart)
		assert.Equal(t, "I-V-vi-IV", q.Answer)
		assert.Equal(t, []string{"I-V-vi-IV", "I-vi-IV-V", "vi-IV-I-V", "I-vi-ii-V"}, optionValues(q))
		assert.Equal(t, "I–vi–IV–V (doo-wop)", q.Options[1].Names["en"])
		assert.Equal(t, []domain.ChordBox{
			{Chord: "C", DiagramID: "box-C"}, {Chord: "G", DiagramID: "box-G"}, {Chord: "Am", DiagramID: "box-Am"}, {Chord: "F", DiagramID: "box-F"},
		}, q.Voicings)
		require.NoError(t, domain.CheckChallenge(s))
	})

	t.Run("pick the chart names the progression and offers the confusables' charts in the key", func(t *testing.T) {
		q, s := plannedChoice(t, pop, "numbers_to_chart", domain.DrillPlanning{Pick: picks(1), Reading: domain.GeneratedChordSymbols, Level: domain.KnowledgeLevelLearning})

		assert.Equal(t, "G", q.MusicKey)
		require.NotNil(t, q.Progression)
		assert.Equal(t, "I-V-vi-IV", q.Progression.Key)
		assert.ElementsMatch(t, []string{"G D Em C", "G Em C D", "Em C G D", "G Em Am D"}, optionValues(q))
		for _, o := range q.Options {
			assert.Equal(t, strings.Fields(o.Value), o.Chart)
		}
		assert.Equal(t, "G D Em C", q.Answer)
		require.NoError(t, domain.CheckChallenge(s))
	})

	t.Run("while learning it, a chart is asked in an easy key", func(t *testing.T) {
		for range 50 {
			q, _ := plannedChoice(t, pop, "chart_to_numbers", domain.DrillPlanning{Pick: rand.IntN, Reading: domain.GeneratedChordSymbols, Level: domain.KnowledgeLevelLearning})
			assert.Contains(t, domain.EasyChartKeys, q.MusicKey)
		}
	})

	t.Run("once accurate, any key may be asked", func(t *testing.T) {
		q, _ := plannedChoice(t, pop, "chart_to_numbers", domain.DrillPlanning{Pick: picks(11), Reading: domain.GeneratedChordSymbols, Level: domain.KnowledgeLevelAccurate})
		assert.Equal(t, "B", q.MusicKey)
	})

	t.Run("a 12-bar chart is read as symbols and offers the two 12-bar forms", func(t *testing.T) {
		blues := domain.ChartProgressionItem{Progression: progression("12-bar-blues"), Catalog: catalogProgressions}
		q, s := plannedChoice(t, blues, "chart_to_numbers", boxes)

		assert.Equal(t, domain.GeneratedChordSymbols, q.DrawnAs)
		assert.Empty(t, q.Voicings)
		assert.Len(t, q.Chart, 12)
		assert.ElementsMatch(t, []string{"12-bar-blues", "12-bar-blues-quick-change"}, optionValues(q))
		require.NoError(t, domain.CheckChallenge(s))
	})
}

// Every harmony item, asked every way, at every level and in every reading,
// challenges the student and is a question the grader accepts: distinct
// options, each a possible answer, the right one among them, and for
// function to chord no other chord with the asked function.
func TestHarmonyDrills_EveryPlanChallenges(t *testing.T) {
	var items []domain.DrillItem
	for _, key := range domain.MusicKeys {
		for _, numeral := range domain.HarmonyItemNumerals {
			items = append(items, domain.DiatonicChordItem{Key: key, Numeral: numeral}, domain.ChordFunctionItem{Key: key, Numeral: numeral})
		}
	}
	for _, p := range catalogProgressions {
		items = append(items, domain.ChartProgressionItem{Progression: p, Catalog: catalogProgressions})
	}
	levels := []domain.KnowledgeLevel{domain.KnowledgeLevelNew, domain.KnowledgeLevelLearning, domain.KnowledgeLevelAccurate, domain.KnowledgeLevelFluent}
	readings := []domain.GeneratedChordDrawing{domain.GeneratedChordBoxes, domain.GeneratedChordSymbols}

	for _, item := range items {
		for _, way := range item.Ways() {
			for _, level := range levels {
				for _, reading := range readings {
					for range 5 {
						planning := domain.DrillPlanning{Pick: rand.IntN, Reading: reading, Voicing: boxOf, Level: level}
						q, s := plannedChoice(t, item, way, planning)
						name := item.ItemKey() + " " + way

						require.NoError(t, domain.CheckChallenge(s), name)
						values := optionValues(q)
						assert.Len(t, slices.Compact(slices.Sorted(slices.Values(values))), len(values), name+": distinct options")
						assert.Contains(t, values, q.Answer, name)
						assert.LessOrEqual(t, len(values), 4, name)
						assert.GreaterOrEqual(t, len(values), 2, name)
						assert.Contains(t, domain.MusicKeys, q.MusicKey, name)
						if way == "function_to_chord" {
							for _, v := range values {
								if v != q.Answer {
									assert.NotEqual(t, q.Function, functionOfChord(q.MusicKey, v), name+": one chord with the function")
								}
							}
						}
						if way == "numeral_to_chord" || way == "function_to_chord" {
							assert.Subset(t, chordsOfKey(q.MusicKey), values, name+": chords of the key")
						}
					}
				}
			}
		}
	}
}

func functionOfChord(key, chord string) domain.ChordFunction {
	for _, n := range domain.ChordNumerals {
		if domain.NumeralChord(key, n) == chord {
			return domain.NumeralFunction(n)
		}
	}
	return ""
}
