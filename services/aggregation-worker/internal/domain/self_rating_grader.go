package domain

import "slices"

// selfRatingV1 grades takes the student rates themselves: a play-along at a tempo,
// a progression in one key of its tour at a tempo, chord or root changes counted
// over a minute, or a chord voicing or a chord's root and fifth played cleanly,
// untimed. It checks the take fits the item and keeps the
// rating and its measure as given, except a minute of changes, whose rating follows
// from its count (golden/practice-graders/self_rating.v1.json).
type selfRatingV1 struct{}

func (selfRatingV1) ID() string { return "self_rating.v1" }

// selfRatedMeasure is the measure a self-rated kind takes.
type selfRatedMeasure int

const (
	measureNone selfRatedMeasure = iota
	measureTempo
	// measureChangeMinute is a minute of changes at a tempo, one change on every
	// beat: the tempo and the changes that rang clean.
	measureChangeMinute
)

// A minute of changes is clean when at least cleanChangeShare of its beats
// changed cleanly, and almost from almostChangeShare.
const (
	cleanChangeShare  = 0.9
	almostChangeShare = 0.6
)

var measureOf = map[PracticeItemKind]selfRatedMeasure{
	PracticeItemKindPlayAlong:       measureTempo,
	PracticeItemKindChordChange:     measureChangeMinute,
	PracticeItemKindRootChange:      measureChangeMinute,
	PracticeItemKindChordVoicing:    measureNone,
	PracticeItemKindRootFifth:       measureNone,
	PracticeItemKindProgressionKeys: measureTempo,
}

// playedInAKey are the self-rated kinds whose take names the key it was played
// in: the item holds every key, so the take says which one.
var playedInAKey = map[PracticeItemKind]bool{PracticeItemKindProgressionKeys: true}

// referencesExist reports whether the diagrams, the layout and the progression the
// key points at are in ref.
func referencesExist(key PracticeItemKey, ref PracticeReference) bool {
	if progression := key.ProgressionKey(); progression != "" {
		if _, ok := ref.Progressions[progression]; !ok {
			return false
		}
	}
	for _, id := range key.DiagramIDs() {
		if _, ok := ref.Diagrams[id]; !ok {
			return false
		}
	}
	if id := key.LayoutInstrumentID(); id != "" {
		if _, ok := ref.Instruments[id]; !ok {
			return false
		}
	}
	return true
}

// checkMeasure checks the kind's own measure is there, and no other. A note value
// belongs to a tempo only.
func checkMeasure(measure selfRatedMeasure, r PracticeResponse) GradeRejection {
	if r.NoteValue != nil && (measure != measureTempo || !r.NoteValue.Valid()) {
		return GradeRejectionResponseDoesNotFitItem
	}
	own, other := r.TempoBPM, r.ChangesPerMinute
	switch measure {
	case measureTempo:
	case measureChangeMinute:
		if r.TempoBPM == nil || r.ChangesPerMinute == nil {
			return GradeRejectionMeasureMissing
		}
		// One change on every beat: a minute can't hold more changes than beats.
		if *r.ChangesPerMinute > *r.TempoBPM {
			return GradeRejectionResponseDoesNotFitItem
		}
		return ""
	case measureNone:
		if own != nil || other != nil {
			return GradeRejectionResponseDoesNotFitItem
		}
		return ""
	}
	if own == nil {
		return GradeRejectionMeasureMissing
	}
	if other != nil {
		return GradeRejectionResponseDoesNotFitItem
	}
	return ""
}

func (selfRatingV1) Grade(key PracticeItemKey, r PracticeResponse, ref PracticeReference) GradeResult {
	measure, rated := measureOf[key.Kind]
	if r.Type != PracticeResponseSelfRating || !r.Rating.valid() || !rated {
		return rejected(GradeRejectionResponseDoesNotFitItem)
	}
	if playedInAKey[key.Kind] != (r.MusicKey != "") || (r.MusicKey != "" && !validMusicKey(r.MusicKey)) {
		return rejected(GradeRejectionResponseDoesNotFitItem)
	}
	if !referencesExist(key, ref) {
		return rejected(GradeRejectionUnknownReference)
	}

	if rejection := checkMeasure(measure, r); rejection != "" {
		return rejected(rejection)
	}
	rating := r.Rating
	if measure == measureChangeMinute {
		rating = changeMinuteRating(*r.ChangesPerMinute, *r.TempoBPM)
	}
	return GradeResult{Evidence: GradedEvidence{
		Source:           EvidenceSourceSelfAssessed,
		Rating:           rating,
		TempoBPM:         r.TempoBPM,
		NoteValue:        r.NoteValue,
		ChangesPerMinute: r.ChangesPerMinute,
		MusicKey:         r.MusicKey,
	}}
}

// changeMinuteRating rates a minute of changes at tempoBPM by the share of its
// beats that changed cleanly, whatever rating the student sent: the count and the
// rating can never disagree.
func changeMinuteRating(cleanChanges, tempoBPM int) SelfRating {
	share := float64(cleanChanges) / float64(tempoBPM)
	switch {
	case share >= cleanChangeShare:
		return SelfRatingClean
	case share >= almostChangeShare:
		return SelfRatingAlmost
	}
	return SelfRatingStruggled
}

// validMusicKey reports whether k is a major key spelled as the chord catalog
// spells roots.
func validMusicKey(k string) bool {
	return slices.Contains(catalogRoots[:], k)
}
