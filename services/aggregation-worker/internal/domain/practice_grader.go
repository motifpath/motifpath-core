package domain

// GradeRejection is why a grader refused a response. A rejection stores no evidence.
type GradeRejection string

const (
	GradeRejectionResponseDoesNotFitItem GradeRejection = "response_does_not_fit_item"
	GradeRejectionUnknownReference       GradeRejection = "unknown_reference"
	GradeRejectionInvalidCell            GradeRejection = "invalid_cell"
	GradeRejectionUnknownOption          GradeRejection = "unknown_option"
	GradeRejectionMeasureMissing         GradeRejection = "measure_missing"
)

// EvidenceSource is where a piece of evidence comes from.
type EvidenceSource string

const (
	EvidenceSourceAutoGraded      EvidenceSource = "auto_graded"
	EvidenceSourceSelfAssessed    EvidenceSource = "self_assessed"
	EvidenceSourceTeacherReviewed EvidenceSource = "teacher_reviewed"
)

// GradedEvidence is what a grader concludes from a response: auto-graded evidence
// (Correct, LatencyMs) or self-assessed evidence (Rating and its one measure).
type GradedEvidence struct {
	Source           EvidenceSource
	Correct          *bool
	LatencyMs        *int
	Rating           SelfRating
	TempoBPM         *int
	ChangesPerMinute *int
}

// GradeResult is either evidence (Rejection empty) or a rejection.
type GradeResult struct {
	Evidence  GradedEvidence
	Rejection GradeRejection
}

func rejected(reason GradeRejection) GradeResult { return GradeResult{Rejection: reason} }

// DiagramReference is what a grader may know about a diagram, from core's
// practice_reference snapshot. TempoBPM is nil for a diagram without
// playback.
type DiagramReference struct {
	ID            string
	InstrumentIDs []string
	TempoBPM      *int
}

// PracticeReference is the reference data a grade runs against. Anything an item key
// points at that is missing here is unknown.
type PracticeReference struct {
	Diagrams map[string]DiagramReference
}

// Grader grades raw responses to one or more item kinds under one versioned rule
// set. It is pure: the same key, response and reference always grade the same.
type Grader interface {
	// ID is the grader and its version, such as self_rating.v1; evidence keeps it so
	// a rule change can regrade.
	ID() string
	Grade(key PracticeItemKey, response PracticeResponse, ref PracticeReference) GradeResult
}

// graderByKind is the registry: the grader for an item comes from its key's kind.
// Fretboard cells and authored exercises get their graders with their drills.
var graderByKind = map[PracticeItemKind]Grader{
	PracticeItemKindPlayAlong:   selfRatingV1{},
	PracticeItemKindChordChange: selfRatingV1{},
}

// GraderFor returns the grader for an item kind, and false when the worker has no
// grader for that kind yet.
func GraderFor(kind PracticeItemKind) (Grader, bool) {
	g, ok := graderByKind[kind]
	return g, ok
}

// Graders lists each registered grader once.
func Graders() []Grader {
	return []Grader{selfRatingV1{}}
}
