package domain

const (
	// DefaultPageLimit is the page size applied when a list request gives none.
	DefaultPageLimit = 20
	// MaxPageLimit is the largest page size a list request may ask for.
	MaxPageLimit = 100
)

// PageRequest is a validated window into a list's filtered results: skip
// Offset matching items, return at most Limit.
type PageRequest struct {
	Limit  int
	Offset int
}

// NewPageRequest applies the default limit and offset to whichever is nil
// and rejects a limit outside 1..MaxPageLimit or a negative offset.
func NewPageRequest(limit, offset *int) (PageRequest, error) {
	req := PageRequest{Limit: DefaultPageLimit}
	if limit != nil {
		req.Limit = *limit
	}
	if offset != nil {
		req.Offset = *offset
	}

	if req.Limit < 1 || req.Limit > MaxPageLimit {
		return PageRequest{}, NewValidationError("limit", "must be between 1 and 100")
	}
	if req.Offset < 0 {
		return PageRequest{}, NewValidationError("offset", "must not be negative")
	}
	return req, nil
}

// Page is one window of a filtered list plus the size of the whole
// filtered set, so callers can render page controls.
type Page[T any] struct {
	Items []T
	// Total counts every item matching the request's filters across all
	// pages, not just the ones in Items.
	Total int
}

// ContentNodeFilter narrows a content node listing. A zero-valued field
// means "no filter" on that dimension. SkillID/ConceptID match a node whose
// linked ids contain that exact id — not its ancestors or descendants. Query
// is a case-insensitive substring match against the node's title.
type ContentNodeFilter struct {
	ContentType ContentType
	SkillID     string
	ConceptID   string
	Difficulty  DifficultyLevel
	Query       string
}

// ExerciseFilter narrows an exercise listing. A zero-valued field means "no
// filter" on that dimension.
type ExerciseFilter struct {
	SkillID      string
	ExerciseType ExerciseType
}

// LearningPathFilter narrows a learning path listing. Query is a
// case-insensitive substring match against the path's title.
type LearningPathFilter struct {
	Query string
}

// CourseListFilter narrows a course catalog listing. A zero-valued field
// means "no filter" on that dimension, and every set field must match. Query
// is a case-insensitive substring match against title or summary; Levels,
// SkillIDs and ConceptIDs are any-of within themselves. A course matches
// SkillIDs/ConceptIDs when some checkpoint's learning path template contains
// a content node classified with any of the given skills and — if both are
// given — any of the given concepts.
//
// PublishedView selects what the text, level and classification predicates
// (and the result's ordering) are evaluated against: the course's latest
// published version when true, its live draft when false. Status filters the
// live row either way.
type CourseListFilter struct {
	Status        *CourseStatus
	Query         string
	Levels        []DifficultyLevel
	CreatedBy     string
	SkillIDs      []string
	ConceptIDs    []string
	PublishedView bool
}
