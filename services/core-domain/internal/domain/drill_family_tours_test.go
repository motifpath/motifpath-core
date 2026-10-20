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

func TestOnBassEachChordHasItsRootThirdAndFifth(t *testing.T) {
	tour := plannedTour(t, domain.ProgressionKeysItem{Layout: standardBass, Bass: true, Progression: popProgression}, nil)

	g := tourKey(t, tour, "G")
	assert.Equal(t, []string{"G", "D", "Em", "C"}, g.Chart)
	assert.NotEmpty(t, g.Voicings, "the backing still sounds the chords")
	tones := map[string]domain.ChordTonePlaces{}
	for _, c := range g.ChordTones {
		tones[c.Chord] = c
	}
	assert.Len(t, tones, 4)
	assert.Equal(t, domain.ChordTonePlaces{Chord: "Em", Root: domain.Place{String: 4, Fret: 0}, Third: domain.Place{String: 1, Fret: 0}, Fifth: domain.Place{String: 3, Fret: 2}}, tones["Em"])
	assert.Equal(t, domain.ChordTonePlaces{Chord: "G", Root: domain.Place{String: 1, Fret: 0}, Third: domain.Place{String: 3, Fret: 2}, Fifth: domain.Place{String: 2, Fret: 0}}, tones["G"])
}

func TestADominantSeventhKeepsItsRootThirdAndFifthOnBass(t *testing.T) {
	tour := plannedTour(t, domain.ProgressionKeysItem{Layout: standardBass, Bass: true, Progression: bluesProgression}, nil)

	e := tourKey(t, tour, "E")
	require.NotEmpty(t, e.ChordTones)
	// E7: E (open E string), G# (G string, 1st fret), B (A string, 2nd fret).
	assert.Equal(t, domain.ChordTonePlaces{Chord: "E7", Root: domain.Place{String: 4, Fret: 0}, Third: domain.Place{String: 1, Fret: 1}, Fifth: domain.Place{String: 3, Fret: 2}}, e.ChordTones[0])
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
