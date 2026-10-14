package domain

import "strings"

// GradeRejection is why a grader refused a response. A rejection stores no evidence.
type GradeRejection string

const (
	GradeRejectionResponseDoesNotFitItem GradeRejection = "response_does_not_fit_item"
	GradeRejectionUnknownReference       GradeRejection = "unknown_reference"
	GradeRejectionInvalidCell            GradeRejection = "invalid_cell"
	GradeRejectionUnknownOption          GradeRejection = "unknown_option"
	GradeRejectionMeasureMissing         GradeRejection = "measure_missing"
	GradeRejectionDegreeNotInShape       GradeRejection = "degree_not_in_shape"
	GradeRejectionInvalidChoices         GradeRejection = "invalid_choices"
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
// cell and its note, with the choices it was named among (fretboard cells), every
// option the student was shown (exercises), a shape's family and member (a shape
// named), the asked degree and every position of the shape that is it (a degree
// found), a generated question's right answer and options, a guide-tone take's
// bars, or every place of a note in a fret window.
type AnswerKey struct {
	String *int
	Fret   *int
	// NoteName is the cell's note, spelled with sharps; any spelling of its pitch
	// is right.
	NoteName    string
	Options     []AnswerOption
	ShapeFamily string
	Shape       string
	Interval    string
	Cells       []AnswerCell
	// Choices are the notes a cell was named among, as shown.
	Choices []string
	// FretWindowFrom is the lowest fret of the window a note was found everywhere in.
	FretWindowFrom *int
	// Way, MusicKey, Answer and Offered are a generated question's way, its key when
	// the item doesn't fix one, the right answer and the options as shown.
	Way      string
	MusicKey string
	Answer   string
	Offered  []string
	// Tone and Bars are a guide-tone take's asked tone and every bar, graded.
	Tone string
	Bars []AnswerBar
}

// AnswerBar is one bar of a guide-tone take: its chord, the asked tone's note
// (spelled with sharps), and whether it was hit.
type AnswerBar struct {
	Chord    string
	NoteName string
	Hit      bool
}

// AnswerCell is a string and fret where a right answer was.
type AnswerCell struct {
	String int
	Fret   int
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
// playback. A diagram that is a drill shape also has its layout instrument, its
// family and member, its family's members and its positions; ShapeFamily is
// empty for any other diagram.
type DiagramReference struct {
	ID                 string
	InstrumentIDs      []string
	TempoBPM           *int
	LayoutInstrumentID string
	ShapeFamily        string
	Shape              string
	FamilyMembers      []string
	Positions          []DiagramPosition
}

// DiagramPosition is one marker of a diagram: where it is, and the interval it
// is from the diagram's root.
type DiagramPosition struct {
	String   int
	Fret     int
	Interval string
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

// singleTemplate is the one drill template of a kind asked only one way, named
// after the drill rather than its response: two kinds may answer with the same
// response shape and still be different drills.
var singleTemplate = map[PracticeItemKind]string{
	PracticeItemKindFretboardNote: "find_every_instance",
	PracticeItemKindGuideTone:     "tap_the_third",
}

// WayAsked is the way an answer to an item of kind was asked, the second half of
// its drill template's key: a generated question's way, the kind's one template
// when it has only one, and the response type for every other answer
// (name_the_note, find_the_degree…).
func WayAsked(kind PracticeItemKind, r PracticeResponse) string {
	if r.Type == PracticeResponseGeneratedChoice {
		return r.Way
	}
	if template, ok := singleTemplate[kind]; ok {
		return template
	}
	return string(r.Type)
}

// kindOf is the kind an item key names, its prefix.
func kindOf(itemKey string) PracticeItemKind {
	kind, _, _ := strings.Cut(itemKey, ":")
	return PracticeItemKind(kind)
}

// TimedDrillTemplate is the timed drill template an answer to key practises:
// <kind>:<way asked> for a generated drill, or an exercise's type. A self-rated
// take is rated, not timed, so it practises none, and neither does an exercise
// missing from ref.
func TimedDrillTemplate(key PracticeItemKey, response PracticeResponse, ref PracticeReference) string {
	switch key.Kind {
	case PracticeItemKindFretboardCell, PracticeItemKindDiagramShape, PracticeItemKindDiatonicChord,
		PracticeItemKindChordFunction, PracticeItemKindChartProgression, PracticeItemKindHeardChord,
		PracticeItemKindHeardProgression, PracticeItemKindGuideTone, PracticeItemKindFretboardNote:
		return string(key.Kind) + ":" + WayAsked(key.Kind, response)
	case PracticeItemKindExercise:
		if e, ok := ref.Exercises[key.ExerciseID()]; ok {
			return e.DrillTemplateKey()
		}
	case PracticeItemKindPlayAlong, PracticeItemKindChordChange, PracticeItemKindChordVoicing,
		PracticeItemKindRootFifth, PracticeItemKindRootChange:
	}
	return ""
}

// ProgressionReference is a progression of the progression catalog: its key and
// its numerals, one bar each, in a major key.
type ProgressionReference struct {
	Key  string
	Bars []string
}

// PracticeReference is the reference data a grade runs against. Anything an item key
// points at that is missing here is unknown.
type PracticeReference struct {
	Diagrams     map[string]DiagramReference
	Exercises    map[string]ExerciseReference
	Instruments  map[string]InstrumentReference
	Progressions map[string]ProgressionReference
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
	PracticeItemKindPlayAlong:        selfRatingV1{},
	PracticeItemKindChordChange:      selfRatingV1{},
	PracticeItemKindChordVoicing:     selfRatingV1{},
	PracticeItemKindRootFifth:        selfRatingV1{},
	PracticeItemKindRootChange:       selfRatingV1{},
	PracticeItemKindExercise:         exerciseOptionV1{},
	PracticeItemKindFretboardCell:    fretboardCellV2{},
	PracticeItemKindDiagramShape:     diagramShapeV1{},
	PracticeItemKindDiatonicChord:    generatedChoiceV1{},
	PracticeItemKindChordFunction:    generatedChoiceV1{},
	PracticeItemKindChartProgression: generatedChoiceV1{},
	PracticeItemKindHeardChord:       generatedChoiceV1{},
	PracticeItemKindHeardProgression: generatedChoiceV1{},
	PracticeItemKindGuideTone:        timedTapsV1{},
	PracticeItemKindFretboardNote:    multiTapV1{},
}

// GraderFor returns the grader for an item kind, and false when the worker has no
// grader for that kind yet.
func GraderFor(kind PracticeItemKind) (Grader, bool) {
	g, ok := graderByKind[kind]
	return g, ok
}

// Graders lists each grader once: every registered one, and the earlier versions
// whose evidence is still kept.
func Graders() []Grader {
	return []Grader{selfRatingV1{}, exerciseOptionV1{}, fretboardCellV1{}, fretboardCellV2{}, diagramShapeV1{},
		generatedChoiceV1{}, timedTapsV1{}, multiTapV1{}}
}
