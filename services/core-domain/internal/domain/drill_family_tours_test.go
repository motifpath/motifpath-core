package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

var (
	popProgression   = domain.Progression{Key: "I-V-vi-IV", Bars: []string{"I", "V", "vi", "IV"}, Names: domain.LocalizedText{"en": "I–V–vi–IV (pop)"}}
	bluesProgression = domain.Progression{Key: "12-bar-blues", Bars: []string{"I7", "I7", "I7", "I7", "IV7", "IV7", "I7", "I7", "V7", "IV7", "I7", "V7"}}
	tourGuitar       = domain.Instrument{ID: "guitar", Tuning: []string{"E2", "A2", "D3", "G3", "B3", "E4"}}
	// everyChordHasABox finds a box for every chord, named after it.
	everyChordHasABox = func(chord string) (string, bool) { return "box-" + chord, true }
)

func plannedTour(t *testing.T, item domain.ProgressionKeysItem, state *domain.PracticeItemState) domain.PlannedProgressionKeys {
	t.Helper()
	planned := plannedItem(t, item, domain.DrillPlanning{Voicing: everyChordHasABox, State: state})
	require.NotNil(t, planned.ProgressionKeys)
	assert.Equal(t, domain.PracticeItemKindProgressionKeys, planned.Kind)
	return *planned.ProgressionKeys
}

func tourKey(t *testing.T, tour domain.PlannedProgressionKeys, key string) domain.PlannedTourKey {
	t.Helper()
	for _, k := range tour.Keys {
		if k.MusicKey == key {
			return k
		}
	}
	require.Failf(t, "no such key", "the tour has no key %s", key)
	return domain.PlannedTourKey{}
}

func TestAProgressionKeyTourIsPlayedInHandAtTheMoveRung(t *testing.T) {
	item := domain.ProgressionKeysItem{Layout: tourGuitar, Progression: popProgression}

	assert.Equal(t, "progression_keys:guitar:I-V-vi-IV", item.ItemKey())
	assert.Equal(t, domain.PracticeItemKindProgressionKeys, item.Kind())
	assert.True(t, item.SuitsSession(true))
	assert.False(t, item.SuitsSession(false), "a tour is played, never recalled in the head")

	var planned domain.PracticeSessionItem
	planned.Ask(item.Plan(item.Ways()[0], domain.DrillPlanning{Voicing: everyChordHasABox}))
	assert.Equal(t, domain.PracticeRungMove, domain.ItemRung(planned, nil))
	assert.Empty(t, planned.DrillTemplateKey(), "a rated tour practises no timed template")
	drill, most, capped := planned.SessionCap()
	assert.True(t, capped)
	assert.Equal(t, "progression_keys", drill)
	assert.Equal(t, 1, most, "at most one tour a session")
}

func TestATourIsSizedLikeAPlayAlongOverThreeTakes(t *testing.T) {
	// Each take: a bar of count-in and 16 bars (4 bars, 4 times through) at
	// 60 BPM in 4/4, then 10 seconds to rate it.
	assert.Equal(t, 3*(17*4+10), domain.ProgressionKeysItem{Layout: tourGuitar, Progression: popProgression}.Seconds())
	// A 12-bar blues plays its form once a take.
	assert.Equal(t, 3*(13*4+10), domain.ProgressionKeysItem{Layout: tourGuitar, Progression: bluesProgression}.Seconds())
}

func TestATourNeverPlayedStartsAtCAtSixtyPercentOfTheTarget(t *testing.T) {
	tour := plannedTour(t, domain.ProgressionKeysItem{Layout: tourGuitar, Progression: popProgression}, nil)

	assert.Equal(t, "guitar", tour.LayoutInstrumentID)
	assert.Equal(t, "I-V-vi-IV", tour.ProgressionKey)
	assert.Equal(t, popProgression.Names, tour.ProgressionNames)
	assert.Equal(t, 4, tour.TimesThrough)
	assert.Equal(t, 3, tour.Takes)
	assert.Equal(t, "C", tour.StartMusicKey)
	assert.Equal(t, 60, tour.TourTempoBPM)
	assert.Equal(t, domain.TempoNoteValueQuarters, tour.StartNoteValue)
	assert.Equal(t, 100, tour.TargetTempoBPM)
	assert.Equal(t, 60, tour.MinTempoBPM)
	var keys []string
	for _, k := range tour.Keys {
		keys = append(keys, k.MusicKey)
		assert.False(t, k.Clean, k.MusicKey)
	}
	assert.Equal(t, []string{"C", "F", "Bb", "Eb", "Ab", "Db", "F#", "B", "E", "A", "D", "G"}, keys)
}

func TestOnGuitarEachKeyHasItsChartAndAVoicingPerChord(t *testing.T) {
	tour := plannedTour(t, domain.ProgressionKeysItem{Layout: tourGuitar, Progression: popProgression}, nil)

	a := tourKey(t, tour, "A")
	assert.Equal(t, []string{"A", "E", "F#m", "D"}, a.Chart)
	assert.Equal(t, []domain.ChordBox{
		{Chord: "A", DiagramID: "box-A"}, {Chord: "E", DiagramID: "box-E"},
		{Chord: "F#m", DiagramID: "box-F#m"}, {Chord: "D", DiagramID: "box-D"},
	}, a.Voicings)
	assert.Empty(t, a.ChordTones, "no chord tones on guitar")
}

func TestAVoicingIsListedOncePerChord(t *testing.T) {
	tour := plannedTour(t, domain.ProgressionKeysItem{Layout: tourGuitar, Progression: bluesProgression}, nil)

	c := tourKey(t, tour, "C")
	assert.Len(t, c.Chart, 12)
	assert.Equal(t, []domain.ChordBox{{Chord: "C7", DiagramID: "box-C7"}, {Chord: "F7", DiagramID: "box-F7"}, {Chord: "G7", DiagramID: "box-G7"}}, c.Voicings)
	assert.Equal(t, 1, tour.TimesThrough)
}

// bassTour plans item for a student at level, every key's window picked as
// the pick-th of the windows the session may pick from.
func bassTour(t *testing.T, item domain.ProgressionKeysItem, level domain.KnowledgeLevel, pick func(n int) int) domain.PlannedProgressionKeys {
	t.Helper()
	planned := plannedItem(t, item, domain.DrillPlanning{Voicing: everyChordHasABox, Level: level, Pick: pick})
	require.NotNil(t, planned.ProgressionKeys)
	return *planned.ProgressionKeys
}

func tonesOf(key domain.PlannedTourKey) map[string]domain.BassChordTones {
	tones := map[string]domain.BassChordTones{}
	for _, c := range key.ChordTones {
		tones[c.Chord] = c
	}
	return tones
}

func TestOnBassEachChordHasItsTonesAboveAndBelowInTheKeysWindow(t *testing.T) {
	// A student new to the tour plays roots only, so every window fits:
	// the pick is the first fret itself.
	tour := bassTour(t, domain.ProgressionKeysItem{Layout: standardBass, Bass: true, Progression: popProgression}, domain.KnowledgeLevelNew, func(int) int { return 3 })

	g := tourKey(t, tour, "G")
	assert.Equal(t, []string{"G", "D", "Em", "C"}, g.Chart)
	assert.NotEmpty(t, g.Voicings, "the backing still sounds the chords")
	require.NotNil(t, g.WindowStartFret)
	assert.Equal(t, 3, *g.WindowStartFret)
	tones := tonesOf(g)
	assert.Len(t, tones, 4)
	// Frets 3 to 7: E on the A string, G and B above it, and the G and B an
	// octave lower on the E string.
	assert.Equal(t, domain.BassChordTones{
		Chord: "Em", Root: domain.Place{String: 3, Fret: 7},
		ThirdAbove: place(2, 5), FifthAbove: place(1, 4), ThirdBelow: place(4, 3), FifthBelow: place(4, 7),
	}, tones["Em"])
	// G on the E string's third fret: nothing below it is on the bass.
	assert.Equal(t, domain.BassChordTones{
		Chord: "G", Root: domain.Place{String: 4, Fret: 3}, ThirdAbove: place(4, 7), FifthAbove: place(3, 5),
	}, tones["G"])
}

func TestOnBassASeventhKeepsItsTriadsThirdAndFifth(t *testing.T) {
	tests := []struct {
		name         string
		numeral      string
		chord        string
		third, fifth *domain.Place
	}{
		// At the nut, strings count from 1 (G2) to 4 (E1); each tone just above the root.
		{name: "a minor seventh has a minor third", numeral: "ii7", chord: "Dm7", third: place(2, 3), fifth: place(1, 2)},
		{name: "a half-diminished seventh has a minor third and a flat fifth", numeral: "iiø7", chord: "Dm7b5", third: place(2, 3), fifth: place(1, 1)},
		{name: "a diminished seventh has a minor third and a flat fifth", numeral: "vii°7", chord: "Bdim7", third: place(2, 0), fifth: place(2, 3)},
		{name: "a major seventh has a major third", numeral: "♭VImaj7", chord: "Abmaj7", third: place(3, 3), fifth: place(2, 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := domain.ProgressionKeysItem{Layout: standardBass, Bass: true, Progression: domain.Progression{Key: "major/x", Bars: []string{tt.numeral}}}
			c := tourKey(t, bassTour(t, item, domain.KnowledgeLevelLearning, func(int) int { return 0 }), "C")
			require.Len(t, c.ChordTones, 1)
			tones := c.ChordTones[0]
			assert.Equal(t, tt.chord, tones.Chord)
			assert.Equal(t, tt.third, tones.ThirdAbove)
			assert.Equal(t, tt.fifth, tones.FifthAbove)
		})
	}
}

func TestOnBassTheTourPlaysAPatternByTheStudentsLevel(t *testing.T) {
	for _, tc := range []struct {
		level domain.KnowledgeLevel
		want  domain.BassPattern
	}{
		{"", domain.BassPatternRoot},
		{domain.KnowledgeLevelNew, domain.BassPatternRoot},
		{domain.KnowledgeLevelLearning, domain.BassPatternRoot},
		{domain.KnowledgeLevelAccurate, domain.BassPatternRootFifth},
		{domain.KnowledgeLevelFluent, domain.BassPatternRootThirdFifthThird},
		{domain.KnowledgeLevelRetained, domain.BassPatternRootThirdFifthSeventh},
	} {
		t.Run(string(tc.want)+" when "+string(tc.level), func(t *testing.T) {
			tour := bassTour(t, domain.ProgressionKeysItem{Layout: standardBass, Bass: true, Progression: bluesProgression}, tc.level, func(int) int { return 0 })
			assert.Equal(t, tc.want, tour.BassPattern)
		})
	}
}

func TestOnBassOnlyTheSeventhPatternMarksSevenths(t *testing.T) {
	item := domain.ProgressionKeysItem{Layout: standardBass, Bass: true, Progression: bluesProgression}
	for _, level := range []domain.KnowledgeLevel{domain.KnowledgeLevelNew, domain.KnowledgeLevelAccurate, domain.KnowledgeLevelFluent} {
		for _, k := range bassTour(t, item, level, func(int) int { return 0 }).Keys {
			for _, c := range k.ChordTones {
				assert.Nil(t, c.Seventh, "%s in %s at %s", c.Chord, k.MusicKey, level)
			}
		}
	}
}

func TestOnBassTheSeventhPatternPicksOnlyWindowsThatHoldEverySeventh(t *testing.T) {
	item := domain.ProgressionKeysItem{Layout: standardBass, Bass: true, Progression: bluesProgression}
	for name, pick := range map[string]func(n int) int{
		"the first window that fits": func(int) int { return 0 },
		"the last window that fits":  func(n int) int { return n - 1 },
	} {
		t.Run(name, func(t *testing.T) {
			tour := bassTour(t, item, domain.KnowledgeLevelRetained, pick)
			for _, k := range tour.Keys {
				require.NotNil(t, k.WindowStartFret)
				assert.True(t, *k.WindowStartFret >= 0 && *k.WindowStartFret <= 9, "window of %s", k.MusicKey)
				for _, c := range k.ChordTones {
					assert.NotNil(t, c.Seventh, "%s in %s from fret %d", c.Chord, k.MusicKey, *k.WindowStartFret)
				}
			}
			a7 := tonesOf(tourKey(t, tour, "A"))["A7"]
			require.NotNil(t, a7.Seventh)
			assert.Equal(t, 7, pitchClassAt(standardBass, *a7.Seventh), "A7's seventh is a G")
		})
	}
}

func TestOnBassTheSeventhPatternGivesATriadItsKeysSeventh(t *testing.T) {
	cases := []struct {
		name        string
		progression domain.Progression
		key         string
		// sevenths are each chord's seventh, as a pitch class.
		sevenths map[string]int
	}{
		{
			name:        "in major, the major seventh on I and IV and the minor seventh elsewhere",
			progression: popProgression, key: "G",
			sevenths: map[string]int{"G": 6, "D": 0, "Em": 2, "C": 11},
		},
		{
			name:        "in minor, natural minor's seventh",
			progression: domain.Progression{Key: "minor/i-bVI-bIII-bVII", Mode: "minor", Bars: []string{"i", "bVI", "bIII", "bVII"}},
			key:         "Am",
			sevenths:    map[string]int{"Am": 7, "F": 4, "C": 11, "G": 5},
		},
		{
			name:        "in minor, harmonic minor's on V and vii°, and ø7 on ii°",
			progression: domain.Progression{Key: "minor/iio-V-i-viio", Mode: "minor", Bars: []string{"iio", "V", "i", "viio"}},
			key:         "Am",
			sevenths:    map[string]int{"Bdim": 9, "E": 2, "Am": 7, "Abdim": 5},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := domain.ProgressionKeysItem{Layout: standardBass, Bass: true, Progression: tc.progression}
			tour := bassTour(t, item, domain.KnowledgeLevelRetained, func(int) int { return 0 })
			for _, k := range tour.Keys {
				for _, c := range k.ChordTones {
					assert.NotNil(t, c.Seventh, "%s in %s", c.Chord, k.MusicKey)
				}
			}
			tones := tonesOf(tourKey(t, tour, tc.key))
			require.Len(t, tones, len(tc.sevenths))
			for chord, want := range tc.sevenths {
				require.NotNil(t, tones[chord].Seventh, chord)
				assert.Equal(t, want, pitchClassAt(standardBass, *tones[chord].Seventh), "%s's seventh", chord)
			}
		})
	}
}

// pitchClassAt is the pitch class played at p on layout.
func pitchClassAt(layout domain.Instrument, p domain.Place) int {
	open, _ := domain.ParsePitch(layout.Tuning[len(layout.Tuning)-p.String])
	return (open + p.Fret) % 12
}

func TestOnGuitarATourHasNoBassPatternAndNoWindows(t *testing.T) {
	tour := bassTour(t, domain.ProgressionKeysItem{Layout: tourGuitar, Progression: popProgression}, domain.KnowledgeLevelRetained, func(int) int { return 0 })
	assert.Empty(t, tour.BassPattern)
	for _, k := range tour.Keys {
		assert.Nil(t, k.WindowStartFret, k.MusicKey)
		assert.Empty(t, k.ChordTones, k.MusicKey)
	}
}

func TestATourStartsWhereTheLastTakeLeftIt(t *testing.T) {
	eighths, triplets := domain.TempoNoteValueEighths, domain.TempoNoteValueTriplets
	cases := []struct {
		name      string
		tour      domain.KeyTourState
		startKey  string
		tempo     int
		noteValue domain.TempoNoteValue
		clean     []string
	}{
		{"at the first key not yet clean", domain.KeyTourState{TempoBPM: 70, CleanKeys: []string{"C", "F"}}, "Bb", 70, domain.TempoNoteValueQuarters, []string{"C", "F"}},
		{"skipping keys already clean", domain.KeyTourState{TempoBPM: 60, CleanKeys: []string{"C", "Bb"}}, "F", 60, domain.TempoNoteValueQuarters, []string{"C", "Bb"}},
		{"in the note value of the last clean take", domain.KeyTourState{TempoBPM: 70, NoteValue: &eighths}, "C", 35, domain.TempoNoteValueEighths, nil},
		{"rounding a tempo in the note value up, so playing it is as fast as the tour", domain.KeyTourState{TempoBPM: 70, NoteValue: &triplets}, "C", 24, domain.TempoNoteValueTriplets, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			state := c.tour
			tour := plannedTour(t, domain.ProgressionKeysItem{Layout: tourGuitar, Progression: popProgression}, &domain.PracticeItemState{KeyTour: &state})

			assert.Equal(t, c.startKey, tour.StartMusicKey)
			assert.Equal(t, c.tempo, tour.TourTempoBPM)
			assert.Equal(t, c.noteValue, tour.StartNoteValue)
			var clean []string
			for _, k := range tour.Keys {
				if k.Clean {
					clean = append(clean, k.MusicKey)
				}
			}
			assert.Equal(t, c.clean, clean)
		})
	}
}

func TestATakeRepeatsTheProgressionToFillSixteenBars(t *testing.T) {
	tests := []struct {
		bars int
		want int
	}{
		{bars: 2, want: 8},
		{bars: 3, want: 5},
		{bars: 4, want: 4},
		{bars: 8, want: 2},
		{bars: 12, want: 1},
		{bars: 16, want: 1},
	}
	for _, tt := range tests {
		bars := make([]string, tt.bars)
		for i := range bars {
			bars[i] = "I"
		}
		item := domain.ProgressionKeysItem{Layout: tourGuitar, Progression: domain.Progression{Key: "major/x", Mode: "major", Bars: bars}}
		assert.Equal(t, tt.want, plannedTour(t, item, nil).TimesThrough, "%d bars", tt.bars)
		assert.Equal(t, 3*((tt.bars*tt.want+1)*4+10), item.Seconds(), "%d bars, sized by its takes", tt.bars)
	}
}

func TestAMinorTourGoesThroughTheRelativeMinors(t *testing.T) {
	minor := domain.Progression{Key: "minor/i-bVII-bVI-V", Mode: "minor", Bars: []string{"i", "♭VII", "♭VI", "V"}}
	tour := plannedTour(t, domain.ProgressionKeysItem{Layout: tourGuitar, Progression: minor}, nil)

	assert.Equal(t, "Am", tour.StartMusicKey)
	var keys []string
	for _, k := range tour.Keys {
		keys = append(keys, k.MusicKey)
	}
	assert.Equal(t, []string{"Am", "Dm", "Gm", "Cm", "Fm", "Bbm", "Ebm", "Abm", "Dbm", "F#m", "Bm", "Em"}, keys)
	assert.Equal(t, []string{"Am", "G", "F", "E"}, tourKey(t, tour, "Am").Chart)

	t.Run("its next take is the next minor key not yet clean", func(t *testing.T) {
		tour := plannedTour(t, domain.ProgressionKeysItem{Layout: tourGuitar, Progression: minor}, &domain.PracticeItemState{KeyTour: &domain.KeyTourState{TempoBPM: 60, CleanKeys: []string{"Am"}}})
		assert.Equal(t, "Dm", tour.StartMusicKey)
	})
}
