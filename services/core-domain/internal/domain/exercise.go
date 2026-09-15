package domain

import "time"

// ExerciseType identifies the kind of practice interaction. Every type is
// checked the same way: option selection — the student's selected option
// ID(s) must match the option(s) marked correct.
type ExerciseType string

const (
	ExerciseTypeTextResponse     ExerciseType = "text_response"
	ExerciseTypeAudioRecognition ExerciseType = "audio_recognition"
	ExerciseTypeImageRecognition ExerciseType = "image_recognition"
	ExerciseTypeImageChoice      ExerciseType = "image_choice"
)

// OptionRegionShape is the rendered shape of an OptionRegion.
type OptionRegionShape string

const (
	OptionRegionShapeRectangle OptionRegionShape = "rectangle"
	OptionRegionShapeCircle    OptionRegionShape = "circle"
)

// OptionRegion is a region on an exercise's ImageURL, used by
// image_recognition options — selecting the region is selecting the option.
type OptionRegion struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
	Shape  OptionRegionShape
}

// Option is one selectable answer choice within an Exercise. Which of Label,
// ImageURL, or Region is populated depends on the parent exercise's
// ExerciseType.
type Option struct {
	ID        string
	IsCorrect bool
	Label     *string
	ImageURL  *string
	Region    *OptionRegion
}

// Exercise is a reusable, standalone practice item classified by skill tags
// and independent of any single challenge. It is checked by option
// selection: the student's selected option ID(s) must match the option(s)
// marked correct.
type Exercise struct {
	ID                       string
	Title                    string
	Prompt                   string
	ExerciseType             ExerciseType
	SkillTags                []string
	ImageURL                 *string
	AudioURL                 *string
	Options                  []Option
	EstimatedDurationSeconds *int
	// ChallengeIDs are the challenges this exercise is currently linked to.
	// Managed exclusively through LinkChallenge/UnlinkChallenge — never set
	// directly by NewExercise beyond the empty slice a brand-new exercise
	// starts with.
	ChallengeIDs []string
	// ContentNodeIDs are the content nodes this exercise is currently linked
	// to as a path exercise. Managed exclusively through
	// LinkContentNode/UnlinkContentNode, independent of ChallengeIDs.
	ContentNodeIDs []string
	CreatedAt      time.Time
}

// NewExercise validates and constructs a standalone Exercise, not yet linked
// to any challenge or content node. Whether it later gets linked to a
// challenge or node that exists is an application-layer concern.
func NewExercise(id, title, prompt string, exerciseType ExerciseType, skillTags []string, imageURL, audioURL *string, options []Option, estimatedDurationSeconds *int, createdAt time.Time) (Exercise, error) {
	var errs []FieldError

	if title == "" {
		errs = append(errs, FieldError{Field: "title", Reason: "must not be empty"})
	}
	if prompt == "" {
		errs = append(errs, FieldError{Field: "prompt", Reason: "must not be empty"})
	}
	errs = append(errs, validateExerciseType(exerciseType)...)
	errs = append(errs, validateSkillTags(skillTags)...)
	errs = append(errs, validateStimulusMedia(exerciseType, imageURL, audioURL)...)
	errs = append(errs, validateOptions(exerciseType, options)...)
	if estimatedDurationSeconds != nil && *estimatedDurationSeconds < 1 {
		errs = append(errs, FieldError{Field: "estimated_duration_seconds", Reason: "must be at least 1 when present"})
	}

	if len(errs) > 0 {
		return Exercise{}, &ValidationError{Fields: errs}
	}

	return Exercise{
		ID:                       id,
		Title:                    title,
		Prompt:                   prompt,
		ExerciseType:             exerciseType,
		SkillTags:                skillTags,
		ImageURL:                 imageURL,
		AudioURL:                 audioURL,
		Options:                  options,
		EstimatedDurationSeconds: estimatedDurationSeconds,
		ChallengeIDs:             []string{},
		ContentNodeIDs:           []string{},
		CreatedAt:                createdAt,
	}, nil
}

func validateExerciseType(exerciseType ExerciseType) []FieldError {
	switch exerciseType {
	case ExerciseTypeTextResponse, ExerciseTypeAudioRecognition, ExerciseTypeImageRecognition, ExerciseTypeImageChoice:
		return nil
	default:
		return []FieldError{{Field: "exercise_type", Reason: "must be one of text_response, audio_recognition, image_recognition, image_choice"}}
	}
}

func validateSkillTags(skillTags []string) []FieldError {
	for _, tag := range skillTags {
		if tag == "" {
			return []FieldError{{Field: "skill_tags", Reason: "each tag must be a non-empty string"}}
		}
	}
	return nil
}

// validateStimulusMedia checks the exercise-level media required by
// exerciseType — image_recognition needs a stimulus image, audio_recognition
// needs a stimulus audio clip. Other types carry no exercise-level media.
func validateStimulusMedia(exerciseType ExerciseType, imageURL, audioURL *string) []FieldError {
	var errs []FieldError
	if exerciseType == ExerciseTypeImageRecognition && (imageURL == nil || *imageURL == "") {
		errs = append(errs, FieldError{Field: "image_url", Reason: "required when exercise_type is image_recognition"})
	}
	if exerciseType == ExerciseTypeAudioRecognition && (audioURL == nil || *audioURL == "") {
		errs = append(errs, FieldError{Field: "audio_url", Reason: "required when exercise_type is audio_recognition"})
	}
	return errs
}

// validateOptions checks that at least one option exists, at least one is
// marked correct, and each option's shape matches exerciseType.
func validateOptions(exerciseType ExerciseType, options []Option) []FieldError {
	if len(options) == 0 {
		return []FieldError{{Field: "options", Reason: "must contain at least one option"}}
	}

	var errs []FieldError
	hasCorrect := false
	for _, opt := range options {
		if opt.IsCorrect {
			hasCorrect = true
		}
		if reason := optionShapeError(exerciseType, opt); reason != "" {
			errs = append(errs, FieldError{Field: "options", Reason: reason})
		}
	}
	if !hasCorrect {
		errs = append(errs, FieldError{Field: "options", Reason: "at least one option must be marked correct"})
	}
	return errs
}

// optionShapeError reports the reason opt's shape is invalid for
// exerciseType, or "" if it's valid. image_recognition options select a
// region on the exercise's image; image_choice options each carry their own
// image; text_response and audio_recognition options carry a text label.
func optionShapeError(exerciseType ExerciseType, opt Option) string {
	switch exerciseType {
	case ExerciseTypeImageRecognition:
		if opt.Region == nil {
			return "image_recognition options must carry a region"
		}
	case ExerciseTypeImageChoice:
		if opt.ImageURL == nil || *opt.ImageURL == "" {
			return "image_choice options must carry an image_url"
		}
	case ExerciseTypeTextResponse, ExerciseTypeAudioRecognition:
		if opt.Label == nil || *opt.Label == "" {
			return "text_response and audio_recognition options must carry a label"
		}
	}
	return ""
}
