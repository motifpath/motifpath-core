package domain

// selfRatingV1 grades takes the student rates themselves: a play-along at a tempo,
// or chord changes counted over a minute. It checks the take fits the item and
// keeps the rating and its measure as given (golden/practice-graders/self_rating.v1.json).
type selfRatingV1 struct{}

func (selfRatingV1) ID() string { return "self_rating.v1" }

func (selfRatingV1) Grade(key PracticeItemKey, r PracticeResponse, ref PracticeReference) GradeResult {
	if r.Type != PracticeResponseSelfRating || !r.Rating.valid() {
		return rejected(GradeRejectionResponseDoesNotFitItem)
	}
	diagramIDs := key.DiagramIDs()
	if len(diagramIDs) == 0 {
		return rejected(GradeRejectionResponseDoesNotFitItem)
	}
	for _, id := range diagramIDs {
		if _, ok := ref.Diagrams[id]; !ok {
			return rejected(GradeRejectionUnknownReference)
		}
	}

	// The kind's own measure must be there, and the other kind's must not.
	measure, other := r.TempoBPM, r.ChangesPerMinute
	if key.Kind == PracticeItemKindChordChange {
		measure, other = r.ChangesPerMinute, r.TempoBPM
	}
	if measure == nil {
		return rejected(GradeRejectionMeasureMissing)
	}
	if other != nil {
		return rejected(GradeRejectionResponseDoesNotFitItem)
	}

	return GradeResult{Evidence: GradedEvidence{
		Source:           EvidenceSourceSelfAssessed,
		Rating:           r.Rating,
		TempoBPM:         r.TempoBPM,
		ChangesPerMinute: r.ChangesPerMinute,
	}}
}
