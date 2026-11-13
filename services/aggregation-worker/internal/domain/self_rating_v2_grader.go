package domain

import "slices"

// selfRatingV2 grades self-rated takes as selfRatingV1 does, except a change and a
// chord's root and fifth: a change is ten cycles at a tempo, one change on every
// beat, rated by the share of its changes that rang clean, picked from four
// buttons; a root and fifth is played under the metronome and keeps its tempo. A
// count of changes over a minute is no longer a measure any kind takes.
type selfRatingV2 struct{}

func (selfRatingV2) ID() string { return "self_rating.v2" }

// measureChangeTake is a take of changes at a tempo: the tempo and the share of
// changes that rang clean.
const measureChangeTake selfRatedMeasure = measureChangeMinute + 1

var measureOfV2 = map[PracticeItemKind]selfRatedMeasure{
	PracticeItemKindPlayAlong:       measureTempo,
	PracticeItemKindChordChange:     measureChangeTake,
	PracticeItemKindRootChange:      measureChangeTake,
	PracticeItemKindChordVoicing:    measureNone,
	PracticeItemKindRootFifth:       measureTempo,
	PracticeItemKindProgressionKeys: measureTempo,
	PracticeItemKindSong:            measureNone,
}

// CleanShares are the shares of clean changes a student picks a take's from, in
// percent.
var CleanShares = []int{25, 50, 75, 100}

// noteValueTaken are the kinds whose take may name the note value it was played at.
var noteValueTaken = map[PracticeItemKind]bool{PracticeItemKindPlayAlong: true, PracticeItemKindProgressionKeys: true}

func (selfRatingV2) Grade(key PracticeItemKey, r PracticeResponse, ref PracticeReference) GradeResult {
	measure, rated := measureOfV2[key.Kind]
	if r.Type != PracticeResponseSelfRating || !r.Rating.valid() || !rated {
		return rejected(GradeRejectionResponseDoesNotFitItem)
	}
	if playedInAKey[key.Kind] != (r.MusicKey != "") || (r.MusicKey != "" && !validMusicKey(r.MusicKey)) {
		return rejected(GradeRejectionResponseDoesNotFitItem)
	}
	if !referencesExist(key, ref) {
		return rejected(GradeRejectionUnknownReference)
	}
	if rejection := checkMeasureV2(key.Kind, measure, r); rejection != "" {
		return rejected(rejection)
	}
	rating := r.Rating
	if measure == measureChangeTake {
		rating = ChangeShareRating(*r.CleanSharePercent)
	}
	return GradeResult{Evidence: GradedEvidence{
		Source:            EvidenceSourceSelfAssessed,
		Rating:            rating,
		TempoBPM:          r.TempoBPM,
		NoteValue:         r.NoteValue,
		CleanSharePercent: r.CleanSharePercent,
		MusicKey:          r.MusicKey,
	}}
}

// checkMeasureV2 checks the kind's own measure is there, and no other: a count of
// changes is refused on every kind, a share on any kind but a change, and a note
// value on any kind that doesn't keep one.
func checkMeasureV2(kind PracticeItemKind, measure selfRatedMeasure, r PracticeResponse) GradeRejection {
	if r.ChangesPerMinute != nil {
		return GradeRejectionResponseDoesNotFitItem
	}
	if r.NoteValue != nil && (!noteValueTaken[kind] || !r.NoteValue.Valid()) {
		return GradeRejectionResponseDoesNotFitItem
	}
	if r.CleanSharePercent != nil && (measure != measureChangeTake || !slices.Contains(CleanShares, *r.CleanSharePercent)) {
		return GradeRejectionResponseDoesNotFitItem
	}
	return checkOwnMeasure(measure, r)
}

// checkOwnMeasure checks the measure the kind takes is all there.
func checkOwnMeasure(measure selfRatedMeasure, r PracticeResponse) GradeRejection {
	switch measure {
	case measureNone:
		if r.TempoBPM != nil {
			return GradeRejectionResponseDoesNotFitItem
		}
	case measureTempo:
		if r.TempoBPM == nil {
			return GradeRejectionMeasureMissing
		}
	case measureChangeTake:
		if r.TempoBPM == nil || r.CleanSharePercent == nil {
			return GradeRejectionMeasureMissing
		}
	case measureChangeMinute:
		return GradeRejectionResponseDoesNotFitItem
	}
	return ""
}

// ChangeShareRating rates a take of changes by its share of clean changes,
// whatever rating the student sent: every change clean is clean, three in four
// almost, half or fewer struggled.
func ChangeShareRating(cleanSharePercent int) SelfRating {
	switch {
	case cleanSharePercent >= 100:
		return SelfRatingClean
	case cleanSharePercent >= 75:
		return SelfRatingAlmost
	}
	return SelfRatingStruggled
}
