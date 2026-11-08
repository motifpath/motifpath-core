package domain

import (
	"slices"
	"strings"
)

// modeTriads are the triads of each mode a made-up wrong answer falls back
// on, as progressions store their numerals.
var modeTriads = map[ProgressionMode][]string{
	ProgressionModeMajor: {"I", "ii", "iii", "IV", "V", "vi", "vii°"},
	ProgressionModeMinor: {"i", "ii°", "♭III", "iv", "v", "♭VI", "♭VII"},
}

// madeUpWrongAnswers are up to count progressions p is easily mistaken for,
// made up because too few drill progressions share its length and mode: p with
// one bar changed, first to a chord p already plays in another bar, so the
// option keeps p's style, then to a triad of p's mode. Each is picked by pick
// and has a chart of its own, none of offered's and none of any of drills' of
// p's length, so it is never another progression's answer. Its name and an
// offered one's never hold one another as words, so neither says the other. It
// is the progression it would be if an admin created it: its key and its name
// are its numerals.
func madeUpWrongAnswers(p Progression, drills, offered []Progression, count int, pick func(int) int) []Progression {
	charts := map[string]bool{}
	var names [][]string
	for _, o := range offered {
		charts[comparableChart(o)] = true
		for _, name := range o.Names {
			names = append(names, words(name))
		}
	}
	for _, d := range drills {
		if len(d.Bars) == len(p.Bars) {
			charts[comparableChart(d)] = true
		}
	}
	mode := ProgressionMode(p.Mode)
	if _, ok := modeTriads[mode]; !ok {
		mode = ProgressionModeMajor
	}
	var made []Progression
	for _, chords := range [][]string{distinct(p.Bars), modeTriads[mode]} {
		candidates := barChanges(p.Bars, chords)
		for len(made) < count && len(candidates) > 0 {
			i := pick(len(candidates))
			bars := candidates[i]
			candidates = slices.Delete(candidates, i, i+1)
			candidate := Progression{Mode: string(mode), Bars: bars}
			chart := comparableChart(candidate)
			name := strings.Join(bars, "–")
			if charts[chart] || saysOrIsSaid(words(name), names) {
				continue
			}
			charts[chart] = true
			names = append(names, words(name))
			candidate.Key = createdProgressionKey(string(mode), bars)
			candidate.Names = LocalizedText{"en": name}
			made = append(made, candidate)
		}
	}
	return made
}

// barChanges are bars with one bar changed to one of chords, bar by bar, every
// chord the bar doesn't already hold.
func barChanges(bars, chords []string) [][]string {
	var changes [][]string
	for i, bar := range bars {
		for _, chord := range chords {
			if chord == bar {
				continue
			}
			changed := slices.Clone(bars)
			changed[i] = chord
			changes = append(changes, changed)
		}
	}
	return changes
}

// saysOrIsSaid reports whether name holds one of names as words, or one of them
// holds it.
func saysOrIsSaid(name []string, names [][]string) bool {
	for _, other := range names {
		if containsRun(name, other) || containsRun(other, name) {
			return true
		}
	}
	return false
}
