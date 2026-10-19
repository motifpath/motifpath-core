package domain

import (
	"fmt"
	"strings"
	"time"
)

// LearningPathItem is a single content node at a given 1-based position
// within a learning path. Title and ContentType are denormalised from the
// referenced ContentNode for display.
type LearningPathItem struct {
	Position      int
	ContentNodeID string
	Title         string
	ContentType   ContentType
	// SectionLabel optionally groups this item with its immediate
	// neighbours under a named section in the path view. Nil means the
	// item is ungrouped. It names a competency or skill area, never a time
	// period or schedule.
	SectionLabel *string
}

// NewLearningPathItem is one resolved item the caller wants in a new
// learning path: the content node it points at (already looked up by the
// application layer to verify existence and to denormalise Title/
// ContentType) plus its optional section label.
type NewLearningPathItem struct {
	Node         ContentNode
	SectionLabel *string
}

// LearningPathStatus says whether learners can find a path. A draft is seen
// only by teachers and admins; a published path is listed in the path
// catalog, where any user can enroll, and is the only kind staff can assign
// or a course can publish with.
type LearningPathStatus string

const (
	LearningPathStatusDraft     LearningPathStatus = "draft"
	LearningPathStatusPublished LearningPathStatus = "published"
)

// Valid reports whether s is a status a path can have.
func (s LearningPathStatus) Valid() bool {
	return s == LearningPathStatusDraft || s == LearningPathStatusPublished
}

// LearningPath is an ordered sequence of content nodes assigned to students
// as a structured curriculum.
type LearningPath struct {
	ID        string
	TeacherID string
	Title     string
	// Summary is the short description shown in the path catalog; nil
	// until an author gives the path one.
	Summary *string
	// Language is the Language.Code the path is written in; nil until an
	// author gives the path one. Never LanguageCodeAny.
	Language *string
	// Status is draft for every new path until an admin publishes it.
	// Saving a path never changes it.
	Status LearningPathStatus
	// Level is the level a learner should be at to follow the path. It is
	// nil only for a path created before levels were recorded, until it is
	// next saved.
	Level *DifficultyLevel
	// InstrumentIDs are the instruments the path is for; empty means every
	// instrument.
	InstrumentIDs []string
	// ThumbnailURL is the image shown for the path; nil means none.
	ThumbnailURL *string
	Items        []LearningPathItem
	CreatedAt    time.Time
	// UpdatedAt is when the path was created or last replaced.
	UpdatedAt time.Time
}

// LearningPathFields are the parts of a learning path its author writes, as
// given to NewLearningPath to create or replace one.
type LearningPathFields struct {
	Title string
	// Summary and Language may be nil while a path is a draft; publishing
	// requires both.
	Summary  *string
	Language *string
	Level    DifficultyLevel
	// InstrumentIDs are the instruments the path is for; empty means every
	// instrument.
	InstrumentIDs []string
	// ThumbnailURL is the image shown for the path; nil means none.
	ThumbnailURL *string
	Items        []NewLearningPathItem
}

// NewLearningPath validates title and items and assigns each item its
// 1-based position in the order given. Each item's Node must already be the
// resolved ContentNode — the application layer fetches them to verify
// existence (a content_node_id that doesn't exist is a 400 with field
// "content_node_id", not something this constructor can check on its own)
// and this constructor reuses that same lookup to denormalise Title/
// ContentType rather than requiring a second round-trip.
//
// Section labels are normalised here (see normaliseSectionLabel) so the
// stored path is the single source of truth about which items belong to the
// same section — consumers must not have to re-derive that by trimming.
//
// languages are the languages MotifPath offers: a language, when given,
// must be one of them and never LanguageCodeAny. A new path is always a
// draft; a replace carries the existing status over itself.
func NewLearningPath(id, teacherID string, fields LearningPathFields, languages []string, createdAt time.Time) (LearningPath, error) {
	title, pathItems := fields.Title, fields.Items
	var errs []FieldError

	if title == "" {
		errs = append(errs, FieldError{Field: "title", Reason: "must not be empty"})
	}
	if fields.Summary != nil && strings.TrimSpace(*fields.Summary) == "" {
		errs = append(errs, FieldError{Field: "summary", Reason: "must not be blank"})
	}
	if fields.Language != nil {
		if reason := courseLanguageProblem(*fields.Language, languages); reason != "" {
			errs = append(errs, FieldError{Field: "language", Reason: reason})
		}
	}
	if !fields.Level.Valid() {
		errs = append(errs, FieldError{Field: "level", Reason: "must be one of beginner, early_intermediate, intermediate, advanced, expert"})
	}
	if reason := instrumentIDsProblem(fields.InstrumentIDs); reason != "" {
		errs = append(errs, FieldError{Field: "instrument_ids", Reason: reason})
	}
	errs = append(errs, thumbnailProblems(fields.ThumbnailURL)...)
	if len(pathItems) == 0 {
		errs = append(errs, FieldError{Field: "items", Reason: "must contain at least one item"})
	}

	if len(errs) > 0 {
		return LearningPath{}, &ValidationError{Fields: errs}
	}

	items := make([]LearningPathItem, len(pathItems))
	for i, pathItem := range pathItems {
		items[i] = LearningPathItem{
			Position:      i + 1,
			ContentNodeID: pathItem.Node.ID,
			Title:         pathItem.Node.Title,
			ContentType:   pathItem.Node.ContentType,
			SectionLabel:  normaliseSectionLabel(pathItem.SectionLabel),
		}
	}

	level := fields.Level
	return LearningPath{
		ID:            id,
		TeacherID:     teacherID,
		Title:         title,
		Summary:       fields.Summary,
		Language:      fields.Language,
		Status:        LearningPathStatusDraft,
		Level:         &level,
		InstrumentIDs: fields.InstrumentIDs,
		ThumbnailURL:  fields.ThumbnailURL,
		Items:         items,
		CreatedAt:     createdAt,
		UpdatedAt:     createdAt,
	}, nil
}

// normaliseSectionLabel trims a section label and collapses a blank one to
// nil. Two items naming the same section must compare equal regardless of
// incidental whitespace, and a label that is empty or whitespace-only names
// no section at all — storing it as present-but-blank would render an empty
// heading downstream.
func normaliseSectionLabel(label *string) *string {
	if label == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*label)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// PublishRequirement names one thing a learning path lacks before it can be
// published.
type PublishRequirement string

const (
	PublishRequirementSummary            PublishRequirement = "summary"
	PublishRequirementLanguage           PublishRequirement = "language"
	PublishRequirementLevel              PublishRequirement = "level"
	PublishRequirementItems              PublishRequirement = "items"
	PublishRequirementUnpublishedContent PublishRequirement = "unpublished_content"
)

// LearningPathNotPublishableError refuses publishing a path, or changing a
// published one, because the result would be incomplete. It lists every
// missing requirement at once, so an author can fix them in one pass rather
// than discovering them one attempt at a time. It is a conflict with the
// path's state, not a malformed request.
type LearningPathNotPublishableError struct {
	Missing []PublishRequirement
	// UnpublishedContentNodeIDs are the items' content nodes that have never
	// been published, in item order. Set only when Missing includes
	// PublishRequirementUnpublishedContent.
	UnpublishedContentNodeIDs []string
}

func (e *LearningPathNotPublishableError) Error() string {
	return fmt.Sprintf("learning path is not publishable: missing %v", e.Missing)
}

func (e *LearningPathNotPublishableError) Unwrap() error {
	return ErrConflict
}

// PublishingProblems reports what stops p from being published, or nil when
// nothing does. unpublishedNodeIDs are the ids of p's items' content nodes
// that have never been published — the caller looks them up, since the path
// alone can't know.
func (p LearningPath) PublishingProblems(unpublishedNodeIDs []string) *LearningPathNotPublishableError {
	var missing []PublishRequirement
	if p.Summary == nil || strings.TrimSpace(*p.Summary) == "" {
		missing = append(missing, PublishRequirementSummary)
	}
	if p.Language == nil || *p.Language == "" {
		missing = append(missing, PublishRequirementLanguage)
	}
	if p.Level == nil {
		missing = append(missing, PublishRequirementLevel)
	}
	if len(p.Items) == 0 {
		missing = append(missing, PublishRequirementItems)
	}
	if len(unpublishedNodeIDs) > 0 {
		missing = append(missing, PublishRequirementUnpublishedContent)
	}
	if len(missing) == 0 {
		return nil
	}
	return &LearningPathNotPublishableError{Missing: missing, UnpublishedContentNodeIDs: unpublishedNodeIDs}
}
