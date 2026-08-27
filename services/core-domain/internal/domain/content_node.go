package domain

import "time"

// ContentType is the media format of a ContentNode.
type ContentType string

const (
	ContentTypeVideo   ContentType = "video"
	ContentTypeArticle ContentType = "article"
)

// DifficultyLevel is one of a ContentNode's three classification dimensions.
type DifficultyLevel string

const (
	DifficultyLevelBeginner     DifficultyLevel = "beginner"
	DifficultyLevelIntermediate DifficultyLevel = "intermediate"
	DifficultyLevelAdvanced     DifficultyLevel = "advanced"
)

// ReviewState tracks whether an admin has confirmed a ContentNode's
// classification as ground truth.
type ReviewState string

const (
	ReviewStatePending    ReviewState = "pending"
	ReviewStateConfirmed  ReviewState = "confirmed"
	ReviewStateOverridden ReviewState = "overridden"
)

// Classification is the minimum semantic layer required for gap detection
// and the rules-based recommendation engine to function: the skill and
// concept a ContentNode teaches, its difficulty, and whether an admin has
// confirmed it.
type Classification struct {
	Skill           string
	Concept         string
	DifficultyLevel DifficultyLevel
	ReviewState     ReviewState
}

// ContentNode is the base unit of a class — a video or article published by
// a teacher.
type ContentNode struct {
	ID             string
	TeacherID      string
	Title          string
	ContentType    ContentType
	Classification Classification
	CreatedAt      time.Time
}

// NewContentNode validates and constructs a ContentNode. ReviewState is
// always forced to pending, regardless of any caller-supplied value — only
// an admin confirms a classification, never the teacher who created it.
//
// When skill, concept, and difficulty are all zero-valued, the failure is
// reported against the single field "classification" rather than three
// separate sub-fields: oapi-codegen decodes a JSON body with the
// classification key omitted into the same zero-value struct as one with
// classification present but empty, so this is the only signal available to
// tell "the whole object was left out" apart from "one field inside it was
// invalid" — which matters because the two are reported under different
// field names in the merged Gherkin scenarios (register-user-style "whole
// object omitted" -> "classification"; a single bad value -> its own field).
func NewContentNode(id, teacherID, title string, contentType ContentType, skill, concept string, difficulty DifficultyLevel, createdAt time.Time) (ContentNode, error) {
	var errs []FieldError

	if title == "" {
		errs = append(errs, FieldError{Field: "title", Reason: "must not be empty"})
	}

	switch contentType {
	case ContentTypeVideo, ContentTypeArticle:
	default:
		errs = append(errs, FieldError{Field: "content_type", Reason: "must be video or article"})
	}

	difficultyValid := false
	switch difficulty {
	case DifficultyLevelBeginner, DifficultyLevelIntermediate, DifficultyLevelAdvanced:
		difficultyValid = true
	}

	switch {
	case skill == "" && concept == "" && difficulty == "":
		errs = append(errs, FieldError{Field: "classification", Reason: "must not be empty"})
	default:
		if skill == "" {
			errs = append(errs, FieldError{Field: "skill", Reason: "must not be empty"})
		}
		if concept == "" {
			errs = append(errs, FieldError{Field: "concept", Reason: "must not be empty"})
		}
		if !difficultyValid {
			errs = append(errs, FieldError{Field: "difficulty_level", Reason: "must be beginner, intermediate, or advanced"})
		}
	}

	if len(errs) > 0 {
		return ContentNode{}, &ValidationError{Fields: errs}
	}

	return ContentNode{
		ID:          id,
		TeacherID:   teacherID,
		Title:       title,
		ContentType: contentType,
		Classification: Classification{
			Skill:           skill,
			Concept:         concept,
			DifficultyLevel: difficulty,
			ReviewState:     ReviewStatePending,
		},
		CreatedAt: createdAt,
	}, nil
}
