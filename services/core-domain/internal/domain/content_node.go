package domain

import (
	"net/url"
	"time"
)

// ContentType is the media format of a ContentNode.
type ContentType string

const (
	ContentTypeVideo   ContentType = "video"
	ContentTypeArticle ContentType = "article"
)

// DifficultyLevel is one of a ContentNode's classification dimensions,
// ordered beginner < early_intermediate < intermediate < advanced < expert.
type DifficultyLevel string

const (
	DifficultyLevelBeginner          DifficultyLevel = "beginner"
	DifficultyLevelEarlyIntermediate DifficultyLevel = "early_intermediate"
	DifficultyLevelIntermediate      DifficultyLevel = "intermediate"
	DifficultyLevelAdvanced          DifficultyLevel = "advanced"
	DifficultyLevelExpert            DifficultyLevel = "expert"
)

// Valid reports whether l is one of the five difficulty levels.
func (l DifficultyLevel) Valid() bool {
	switch l {
	case DifficultyLevelBeginner, DifficultyLevelEarlyIntermediate, DifficultyLevelIntermediate, DifficultyLevelAdvanced, DifficultyLevelExpert:
		return true
	}
	return false
}

// ReviewState tracks whether an admin has confirmed a ContentNode's
// classification as ground truth.
type ReviewState string

const (
	ReviewStatePending    ReviewState = "pending"
	ReviewStateConfirmed  ReviewState = "confirmed"
	ReviewStateOverridden ReviewState = "overridden"
)

// Classification is the minimum semantic layer required for gap detection
// and the rules-based recommendation engine to function: the Skill(s) and
// Concept(s) a ContentNode teaches (each a reference into the shared
// Skill/Concept tree, requiring at least one of each), its difficulty, and
// whether an admin has confirmed it.
//
// Skills/Concepts carry only ID until this ContentNode is read back from
// the repository with its Skill/Concept rows joined in — the same
// construct-then-refetch convention Languages already follows.
type Classification struct {
	Skills          []Skill
	Concepts        []Concept
	DifficultyLevel DifficultyLevel
	ReviewState     ReviewState
}

// SkillIDs returns the ids of c.Skills, in order.
func (c Classification) SkillIDs() []string {
	ids := make([]string, len(c.Skills))
	for i, s := range c.Skills {
		ids[i] = s.ID
	}
	return ids
}

// ConceptIDs returns the ids of c.Concepts, in order.
func (c Classification) ConceptIDs() []string {
	ids := make([]string, len(c.Concepts))
	for i, cn := range c.Concepts {
		ids[i] = cn.ID
	}
	return ids
}

// ContentNode is the base unit of a class — a video or article published by
// a teacher.
type ContentNode struct {
	ID             string
	TeacherID      string
	Title          string
	ContentType    ContentType
	Classification Classification
	// MediaURL is the video students watch. Set only for video nodes; nil on
	// article nodes, and on video nodes created before this field existed.
	MediaURL *string
	// RichContent is the article's body. Set only for article nodes.
	RichContent *PromptDocument
	// Languages are the languages this content node is available in, or a
	// single LanguageCodeAny entry for language-agnostic content. Never
	// empty — a node must be explicitly tagged. Name is populated only once
	// this ContentNode has been read back from the repository with its
	// Language rows joined in.
	Languages []Language
	CreatedAt time.Time
}

// NewContentNode validates and constructs a ContentNode. ReviewState is
// always forced to pending, regardless of any caller-supplied value — only
// an admin confirms a classification, never the teacher who created it.
// skillIDs/conceptIDs carry only the request-supplied ids until this node is
// read back from the repository with its Skill/Concept rows joined in — see
// validateContentNodeClassification for the shared classification rules.
// Whether each id actually references an existing Skill/Concept is an
// application-layer concern, requiring a repository round trip this
// constructor can't perform.
func NewContentNode(id, teacherID, title string, contentType ContentType, skillIDs, conceptIDs []string, difficulty DifficultyLevel, languageCodes []string, mediaURL *string, richContent *PromptDocument, createdAt time.Time) (ContentNode, error) {
	errs := validateContentNodeClassification(title, skillIDs, conceptIDs, difficulty)
	errs = append(errs, validateLanguageCodes("language_codes", languageCodes)...)

	switch contentType {
	case ContentTypeVideo, ContentTypeArticle:
	default:
		errs = append(errs, FieldError{Field: "content_type", Reason: "must be video or article"})
	}
	errs = append(errs, validateContentNodeBody(contentType, mediaURL, richContent)...)

	if len(errs) > 0 {
		return ContentNode{}, &ValidationError{Fields: errs}
	}

	return ContentNode{
		ID:          id,
		TeacherID:   teacherID,
		Title:       title,
		ContentType: contentType,
		Classification: Classification{
			Skills:          skillsFromIDs(skillIDs),
			Concepts:        conceptsFromIDs(conceptIDs),
			DifficultyLevel: difficulty,
			ReviewState:     ReviewStatePending,
		},
		MediaURL:    mediaURL,
		RichContent: richContent,
		Languages:   languagesFromCodes(languageCodes),
		CreatedAt:   createdAt,
	}, nil
}

// Update validates and returns a copy of n with its title, classification,
// and languages replaced. ContentType and Classification.ReviewState carry
// over unchanged — content_type cannot change after creation since it
// determines which ExpandedContent trigger fields are valid for items
// already attached to this node, and an edit does not reset or require
// re-confirming an admin's prior review.
func (n ContentNode) Update(title string, skillIDs, conceptIDs []string, difficulty DifficultyLevel, languageCodes []string, mediaURL *string, richContent *PromptDocument) (ContentNode, error) {
	errs := validateContentNodeClassification(title, skillIDs, conceptIDs, difficulty)
	errs = append(errs, validateLanguageCodes("language_codes", languageCodes)...)
	errs = append(errs, validateContentNodeBody(n.ContentType, mediaURL, richContent)...)
	if len(errs) > 0 {
		return ContentNode{}, &ValidationError{Fields: errs}
	}

	updated := n
	updated.Title = title
	updated.Classification.Skills = skillsFromIDs(skillIDs)
	updated.Classification.Concepts = conceptsFromIDs(conceptIDs)
	updated.Classification.DifficultyLevel = difficulty
	updated.MediaURL = mediaURL
	updated.RichContent = richContent
	updated.Languages = languagesFromCodes(languageCodes)
	return updated, nil
}

// validateContentNodeBody checks the media_url/rich_content pair shared by
// creation and update: a video requires media_url and forbids rich_content;
// an article requires rich_content and forbids media_url. An unrecognised
// content type is reported separately by the caller, so it adds nothing here.
func validateContentNodeBody(contentType ContentType, mediaURL *string, richContent *PromptDocument) []FieldError {
	var errs []FieldError

	switch contentType {
	case ContentTypeVideo:
		switch {
		case mediaURL == nil || *mediaURL == "":
			errs = append(errs, FieldError{Field: "media_url", Reason: "is required when content_type is video"})
		case !isHTTPURL(*mediaURL):
			errs = append(errs, FieldError{Field: "media_url", Reason: "must be an absolute http or https URL"})
		}
		if richContent != nil {
			errs = append(errs, FieldError{Field: "rich_content", Reason: "must be absent when content_type is video"})
		}
	case ContentTypeArticle:
		if mediaURL != nil {
			errs = append(errs, FieldError{Field: "media_url", Reason: "must be absent when content_type is article"})
		}
		if richContent == nil {
			errs = append(errs, FieldError{Field: "rich_content", Reason: "is required when content_type is article"})
		} else {
			for _, docErr := range validatePromptDocument(*richContent, true) {
				errs = append(errs, FieldError{Field: "rich_content", Reason: docErr.Reason})
			}
		}
	}

	return errs
}

// isHTTPURL reports whether raw is an absolute http or https URL with a host.
// Restricting the scheme matters because students' players load this value:
// a bare "format: uri" check would also admit javascript: and ftp: values.
func isHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// skillsFromIDs builds placeholder Skill entries (ID only, no Name/ParentID)
// from a request's skill_ids — mirrors languagesFromCodes' role for
// Languages. The full Skill objects are populated only once the owning
// ContentNode/Exercise is read back from the repository with its Skill rows
// joined in.
func skillsFromIDs(ids []string) []Skill {
	if len(ids) == 0 {
		return nil
	}
	skills := make([]Skill, len(ids))
	for i, id := range ids {
		skills[i] = Skill{ID: id}
	}
	return skills
}

// conceptsFromIDs is skillsFromIDs' counterpart for concept_ids.
func conceptsFromIDs(ids []string) []Concept {
	if len(ids) == 0 {
		return nil
	}
	concepts := make([]Concept, len(ids))
	for i, id := range ids {
		concepts[i] = Concept{ID: id}
	}
	return concepts
}

// validateContentNodeClassification checks the fields shared by creation and
// update — everything except content_type itself, which only creation
// validates (it cannot be changed after creation).
//
// When skillIDs, conceptIDs, and difficulty are all empty/zero-valued, the
// failure is reported against the single field "classification" rather than
// three separate sub-fields: oapi-codegen decodes a JSON body with the
// classification key omitted into the same zero-value struct as one with
// classification present but empty, so this is the only signal available to
// tell "the whole object was left out" apart from "one field inside it was
// invalid" — which matters because the two are reported under different
// field names in the merged Gherkin scenarios (register-user-style "whole
// object omitted" -> "classification"; a single bad value -> its own field).
func validateContentNodeClassification(title string, skillIDs, conceptIDs []string, difficulty DifficultyLevel) []FieldError {
	var errs []FieldError

	if title == "" {
		errs = append(errs, FieldError{Field: "title", Reason: "must not be empty"})
	}

	difficultyValid := false
	switch difficulty {
	case DifficultyLevelBeginner, DifficultyLevelEarlyIntermediate, DifficultyLevelIntermediate, DifficultyLevelAdvanced, DifficultyLevelExpert:
		difficultyValid = true
	}

	switch {
	case len(skillIDs) == 0 && len(conceptIDs) == 0 && difficulty == "":
		errs = append(errs, FieldError{Field: "classification", Reason: "must not be empty"})
	default:
		if len(skillIDs) == 0 {
			errs = append(errs, FieldError{Field: "skill_ids", Reason: "must not be empty"})
		}
		if len(conceptIDs) == 0 {
			errs = append(errs, FieldError{Field: "concept_ids", Reason: "must not be empty"})
		}
		if !difficultyValid {
			errs = append(errs, FieldError{Field: "difficulty_level", Reason: "must be beginner, early_intermediate, intermediate, advanced, or expert"})
		}
	}

	return errs
}
