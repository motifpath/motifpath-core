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
	AudioMs          *int
	Rating           SelfRating
	TempoBPM         *int
	ChangesPerMinute *int
	// AnswerKey is what a right answer was, kept with auto-graded evidence so a
	// disputed answer can be checked from the evidence alone; nil when the grader
	// had nothing to keep.
	AnswerKey *AnswerKey
}

// AnswerKey is what a right answer was when an answer was graded: the asked
// cell and its note (fretboard cells), or every option the student was shown
// (exercises).
type AnswerKey struct {
	String *int
	Fret   *int
	// NoteName is the cell's note, spelled with sharps; any spelling of its pitch
	// is right.
	NoteName string
	Options  []AnswerOption
}

// AnswerOption is an exercise option as the student was shown it. Shown is the
// option as core keeps it in the reference snapshot, passed through untouched so
// the evidence keeps exactly what was on screen.
type AnswerOption struct {
	OptionID  string
	IsCorrect bool
	Shown     []byte
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

// ExerciseReference is what a grader may know about an authored exercise, from
// core's practice_reference snapshot: its type, every option and the correct ones.
type ExerciseReference struct {
	ID               string
	ExerciseType     string
	OptionIDs        []string
	CorrectOptionIDs []string
	InstrumentIDs    []string
	// Options is every option as it is shown, empty for a reference kept before
	// core began keeping them.
	Options []AnswerOption
}

// InstrumentReference is what the fretboard grader may know about an
// instrument: the tuning that gives each cell its pitch, lowest string first.
type InstrumentReference struct {
	ID     string
	Tuning []string
}

// DrillTemplateKey is the drill template the exercise's answers are timed under:
// one per exercise type, so a family of exercises shares one fluent time.
func (e ExerciseReference) DrillTemplateKey() string {
	return string(PracticeItemKindExercise) + ":" + e.ExerciseType
}

// TimedDrillTemplate is the timed drill template an answer to key practises:
// a fretboard cell's way of being asked, or an exercise's type. A play-along
// or a chord change is rated, not timed, so it practises none, and neither
// does an exercise missing from ref.
func TimedDrillTemplate(key PracticeItemKey, response PracticeResponse, ref PracticeReference) string {
	switch key.Kind {
	case PracticeItemKindFretboardCell:
		return string(PracticeItemKindFretboardCell) + ":" + string(response.Type)
	case PracticeItemKindExercise:
		if e, ok := ref.Exercises[key.ExerciseID()]; ok {
			return e.DrillTemplateKey()
		}
	case PracticeItemKindPlayAlong, PracticeItemKindChordChange:
	}
	return ""
}

// PracticeReference is the reference data a grade runs against. Anything an item key
// points at that is missing here is unknown.
type PracticeReference struct {
	Diagrams    map[string]DiagramReference
	Exercises   map[string]ExerciseReference
	Instruments map[string]InstrumentReference
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
var graderByKind = map[PracticeItemKind]Grader{
	PracticeItemKindPlayAlong:     selfRatingV1{},
	PracticeItemKindChordChange:   selfRatingV1{},
	PracticeItemKindExercise:      exerciseOptionV1{},
	PracticeItemKindFretboardCell: fretboardCellV1{},
}

// GraderFor returns the grader for an item kind, and false when the worker has no
// grader for that kind yet.
func GraderFor(kind PracticeItemKind) (Grader, bool) {
	g, ok := graderByKind[kind]
	return g, ok
}

// Graders lists each registered grader once.
func Graders() []Grader {
	return []Grader{selfRatingV1{}, exerciseOptionV1{}, fretboardCellV1{}}
}
