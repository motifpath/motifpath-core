package domain

import "time"

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
	ID          string
	Title       string
	Summary     string
	Level       DifficultyLevel
	Status      CourseStatus
	CreatedBy   string
	CreatedAt   time.Time
	Checkpoints []CourseCheckpoint
}

// NewCourse validates title, summary, level, and checkpoints, and assigns
// each checkpoint its 1-based position in the order given. Each
// checkpoint's Path must already be the resolved LearningPath — the
// application layer fetches them to verify existence (a learning_path_id
// that doesn't exist is a 400 with field "learning_path_id", not something
// this constructor can check on its own) and this constructor reuses that
// same lookup to resolve EffectiveTitle rather than requiring a second
// round-trip. A newly created course always starts in CourseStatusDraft.
func NewCourse(id, createdBy, title, summary string, level DifficultyLevel, checkpoints []NewCourseCheckpoint, createdAt time.Time) (Course, error) {
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
		Status:      CourseStatusDraft,
		CreatedBy:   createdBy,
		CreatedAt:   createdAt,
		Checkpoints: built,
	}, nil
}
