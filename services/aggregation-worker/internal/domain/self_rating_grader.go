package domain

// selfRatingV1 grades takes the student rates themselves: a play-along at a tempo,
// chord or root changes counted over a minute, or a chord voicing or a chord's root
// and fifth played cleanly, untimed. It checks the take fits the item and keeps the
// rating and its measure as given (golden/practice-graders/self_rating.v1.json).
type selfRatingV1 struct{}

func (selfRatingV1) ID() string { return "self_rating.v1" }

// selfRatedMeasure is the measure a self-rated kind takes.
type selfRatedMeasure int

const (
	measureNone selfRatedMeasure = iota
	measureTempo
	measureChangesPerMinute
)

var measureOf = map[PracticeItemKind]selfRatedMeasure{
	PracticeItemKindPlayAlong:    measureTempo,
	PracticeItemKindChordChange:  measureChangesPerMinute,
	PracticeItemKindRootChange:   measureChangesPerMinute,
	PracticeItemKindChordVoicing: measureNone,
	PracticeItemKindRootFifth:    measureNone,
}

// referencesExist reports whether the diagrams and the layout the key points at
// are in ref.
func referencesExist(key PracticeItemKey, ref PracticeReference) bool {
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
	case measureChangesPerMinute:
		own, other = r.ChangesPerMinute, r.TempoBPM
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
	if !referencesExist(key, ref) {
		return rejected(GradeRejectionUnknownReference)
	}

	if rejection := checkMeasure(measure, r); rejection != "" {
		return rejected(rejection)
	}
	return GradeResult{Evidence: GradedEvidence{
		Source:           EvidenceSourceSelfAssessed,
		Rating:           r.Rating,
		TempoBPM:         r.TempoBPM,
		NoteValue:        r.NoteValue,
		ChangesPerMinute: r.ChangesPerMinute,
	}}
}
