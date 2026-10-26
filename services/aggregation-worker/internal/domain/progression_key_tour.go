package domain

import (
	"math"
	"slices"
)

// A progression key tour plays one progression key after key, in
// ProgressionKeyOrder. It keeps one tempo, at effective speed: keys come first,
// then tempo.
const (
	// ProgressionKeysTargetTempoBPM is what every tour aims for, in quarters.
	ProgressionKeysTargetTempoBPM = 100
	// tourFirstTempoShare starts a tour never played at this share of the target,
	// rounded down to a tourTempoStepBPM step; tourMinTempoShare is the slowest a
	// struggle lowers it to.
	tourFirstTempoShare = 0.6
	tourMinTempoShare   = 0.6
	tourTempoStepBPM    = 5
)

// ProgressionKeyOrder is the tour's order, the cycle of fourths from C, spelled
// as the chord catalog spells roots.
var ProgressionKeyOrder = []string{"C", "F", "Bb", "Eb", "Ab", "Db", "F#", "B", "E", "A", "D", "G"}

// foldTour moves the tour by a take of it:
//   - a clean take at or above the tour tempo makes its key clean; when that was
//     the last key, the tour starts again with no key clean, one step faster,
//     never past the target by itself;
//   - a struggle at or below the tour tempo lowers it one step, never below the
//     floor, and keeps the clean keys. A struggle above it explores, as on any
//     self-rated item;
//   - an almost take changes nothing.
//
// A clean take also sets the note value the next session starts in.
func (f *ItemFold) foldTour(e PracticeEvidence, goal ItemGoal) {
	target := ProgressionKeysTargetTempoBPM
	if goal.TargetTempoBPM != nil {
		target = *goal.TargetTempoBPM
	}
	tempo := int(math.Floor(float64(target)*tourFirstTempoShare/tourTempoStepBPM)) * tourTempoStepBPM
	if f.TourTempoBPM != nil {
		tempo = *f.TourTempoBPM
	}
	speed := EffectiveSpeed(*e.TempoBPM, e.NoteValue)

	switch e.Rating {
	case SelfRatingClean:
		noteValue := NoteValueQuarters
		if e.NoteValue != nil {
			noteValue = *e.NoteValue
		}
		f.TourNoteValue = &noteValue
		if speed >= float64(tempo) && !slices.Contains(f.TourCleanKeys, e.MusicKey) {
			f.TourCleanKeys = inTourOrder(append(slices.Clone(f.TourCleanKeys), e.MusicKey))
		}
		if len(f.TourCleanKeys) == len(ProgressionKeyOrder) {
			tempo = max(tempo, min(tempo+tourTempoStepBPM, target))
			f.TourCleanKeys = nil
		}
	case SelfRatingStruggled:
		if speed <= float64(tempo) {
			floor := int(math.Ceil(float64(target) * tourMinTempoShare))
			tempo = max(floor, tempo-tourTempoStepBPM)
		}
	case SelfRatingAlmost:
	}
	f.TourTempoBPM = &tempo
}

// inTourOrder sorts keys by their place in the tour.
func inTourOrder(keys []string) []string {
	slices.SortFunc(keys, func(a, b string) int {
		return slices.Index(ProgressionKeyOrder, a) - slices.Index(ProgressionKeyOrder, b)
	})
	return keys
}
