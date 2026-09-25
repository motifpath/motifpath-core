package domain

import (
	"fmt"
	"slices"
	"time"
)

// CourseStatus tracks a Course's lifecycle: draft (never published),
// published (has at least one CourseVersion), or retired (removed from the
// catalog for new enrollment only — existing enrollments are unaffected).
type CourseStatus string

const (
	CourseStatusDraft     CourseStatus = "draft"
	CourseStatusPublished CourseStatus = "published"
	CourseStatusRetired   CourseStatus = "retired"
)

// CourseCheckpoint is one stage of a course's journey at a given 1-based
// position, pointing at a LearningPath template. Title is an optional
// override; EffectiveTitle is what is actually shown to a caller — the
// override if one was set, otherwise the learning path's own title.
type CourseCheckpoint struct {
	Position       int
	LearningPathID string
	Title          *string
	EffectiveTitle string
}

// NewCourseCheckpoint is one checkpoint the caller wants in a new or
// replaced course: the learning path template it points at (already
// resolved by the application layer to verify existence and to read the
// template's own title) plus its optional title override.
type NewCourseCheckpoint struct {
	Path  LearningPath
	Title *string
}

// Course is an ordered sequence of learning-path checkpoints students
// progress through, with an explicit draft/publish split. This
// representation is always the live, currently-being-authored draft —
// publishing a snapshot of it into an immutable CourseVersion is a
// separate operation this type does not perform.
type Course struct {
	ID      string
	Title   string
	Summary string
	Level   DifficultyLevel
	// Language is the Language.Code the course is written in: a course is
	// not localized, so its title, summary and checkpoint titles all read
	// in this one language.
	Language    string
	Status      CourseStatus
	CreatedBy   string
	CreatedAt   time.Time
	Checkpoints []CourseCheckpoint
}

// CourseFields are the parts of a course its author writes, as given to
// NewCourse to create or replace a course draft.
type CourseFields struct {
	Title       string
	Summary     string
	Level       DifficultyLevel
	Language    string
	Checkpoints []NewCourseCheckpoint
}

// NewCourse validates title, summary, level, language, and checkpoints, and assigns
// each checkpoint its 1-based position in the order given. Each
// checkpoint's Path must already be the resolved LearningPath — the
// application layer fetches them to verify existence (a learning_path_id
// that doesn't exist is a 400 with field "learning_path_id", not something
// this constructor can check on its own) and this constructor reuses that
// same lookup to resolve EffectiveTitle rather than requiring a second
// round-trip. A newly created course always starts in CourseStatusDraft.
// languages are the languages MotifPath offers; the course's language must
// be one of them, and never LanguageCodeAny, since a course always has
// written words.
func NewCourse(id, createdBy string, fields CourseFields, languages []string, createdAt time.Time) (Course, error) {
	title, summary, level, checkpoints := fields.Title, fields.Summary, fields.Level, fields.Checkpoints
	var errs []FieldError

	if title == "" {
		errs = append(errs, FieldError{Field: "title", Reason: "must not be empty"})
	}
	if summary == "" {
		errs = append(errs, FieldError{Field: "summary", Reason: "must not be empty"})
	}

	switch level {
	case DifficultyLevelBeginner, DifficultyLevelEarlyIntermediate, DifficultyLevelIntermediate, DifficultyLevelAdvanced, DifficultyLevelExpert:
	default:
		errs = append(errs, FieldError{Field: "level", Reason: "must be one of beginner, early_intermediate, intermediate, advanced, expert"})
	}

	if reason := courseLanguageProblem(fields.Language, languages); reason != "" {
		errs = append(errs, FieldError{Field: "language", Reason: reason})
	}

	if len(checkpoints) == 0 {
		errs = append(errs, FieldError{Field: "checkpoints", Reason: "must contain at least one checkpoint"})
	}

	if len(errs) > 0 {
		return Course{}, &ValidationError{Fields: errs}
	}

	built := make([]CourseCheckpoint, len(checkpoints))
	for i, checkpoint := range checkpoints {
		effectiveTitle := checkpoint.Path.Title
		if checkpoint.Title != nil && *checkpoint.Title != "" {
			effectiveTitle = *checkpoint.Title
		}
		built[i] = CourseCheckpoint{
			Position:       i + 1,
			LearningPathID: checkpoint.Path.ID,
			Title:          checkpoint.Title,
			EffectiveTitle: effectiveTitle,
		}
	}

	return Course{
		ID:          id,
		Title:       title,
		Summary:     summary,
		Level:       level,
		Language:    fields.Language,
		Status:      CourseStatusDraft,
		CreatedBy:   createdBy,
		CreatedAt:   createdAt,
		Checkpoints: built,
	}, nil
}

// courseLanguageProblem returns why language can't be a course's language
// among the offered languages, or "" if it can.
func courseLanguageProblem(language string, languages []string) string {
	switch {
	case language == "":
		return "must not be empty"
	case language == LanguageCodeAny:
		return fmt.Sprintf("must be a language, not %q", LanguageCodeAny)
	case !slices.Contains(languages, language):
		return "must be one of the languages MotifPath offers"
	}
	return ""
}
