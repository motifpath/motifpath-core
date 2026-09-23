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
	ExerciseTypeAudioSelection   ExerciseType = "audio_selection"
)

// PromptDocument is a structured rich-text document for an exercise's
// prompt, produced by the exercise-prompt rich-text editor and persisted
// exactly as the editor produces it.
type PromptDocument struct {
	Type    string       `json:"type"`
	Content []PromptNode `json:"content"`
}

// NewPlainTextPrompt builds a PromptDocument holding text as a single,
// unformatted paragraph — the minimal valid document. An empty text
// produces an empty paragraph (no text node) — ProseMirror rejects
// zero-length text nodes as invalid.
func NewPlainTextPrompt(text string) PromptDocument {
	content := []PromptNode{}
	if text != "" {
		content = []PromptNode{{Type: PromptNodeTypeText, Text: text}}
	}
	return PromptDocument{
		Type: "doc",
		Content: []PromptNode{
			{Type: PromptNodeTypeParagraph, Content: content},
		},
	}
}

// PromptNodeType identifies the kind of a PromptNode. Only the node types
// the exercise-prompt editor can actually produce are valid here.
type PromptNodeType string

const (
	PromptNodeTypeHeading     PromptNodeType = "heading"
	PromptNodeTypeParagraph   PromptNodeType = "paragraph"
	PromptNodeTypeText        PromptNodeType = "text"
	PromptNodeTypeBulletList  PromptNodeType = "bulletList"
	PromptNodeTypeOrderedList PromptNodeType = "orderedList"
	PromptNodeTypeListItem    PromptNodeType = "listItem"
	PromptNodeTypeTable       PromptNodeType = "table"
	PromptNodeTypeTableRow    PromptNodeType = "tableRow"
	PromptNodeTypeTableHeader PromptNodeType = "tableHeader"
	PromptNodeTypeTableCell   PromptNodeType = "tableCell"
	PromptNodeTypeImage       PromptNodeType = "image"
	// PromptNodeTypeAudio and PromptNodeTypeVideo are available to rich_text
	// ExpandedContent and Exercise.RemediationTargets' rich content — the
	// exercise-prompt authoring toolbar does not offer them, so they never
	// appear in a PromptDocument used as an exercise's own prompt, but this
	// shared type permits them for the surfaces that do.
	PromptNodeTypeAudio PromptNodeType = "audio"
	PromptNodeTypeVideo PromptNodeType = "video"
	// PromptNodeTypeDiagram embeds a prebuilt diagram inline, via its
	// PromptNodeAttrs.DiagramRef or DiagramStackRef. Available to every
	// surface that produces a PromptDocument, including an exercise's own
	// prompt — a diagram is not a content type of its own, it is a resource
	// embedded into content that already has a format.
	PromptNodeTypeDiagram PromptNodeType = "diagram"
)

// PromptNodeAttrs holds the type-specific attributes a PromptNode may
// carry. Which fields are set depends on the node's Type; the rest stay
// nil.
type PromptNodeAttrs struct {
	Level           *int    `json:"level,omitempty"`
	TextAlign       *string `json:"textAlign,omitempty"`
	Src             *string `json:"src,omitempty"`
	Alt             *string `json:"alt,omitempty"`
	Colspan         *int    `json:"colspan,omitempty"`
	Rowspan         *int    `json:"rowspan,omitempty"`
	BackgroundColor *string `json:"backgroundColor,omitempty"`
	BorderColor     *string `json:"borderColor,omitempty"`
	// DiagramRef and DiagramStackRef carry a diagram PromptNode's embedded
	// diagram — exactly one is set, never both.
	DiagramRef      *DiagramRef      `json:"diagramRef,omitempty"`
	DiagramStackRef *DiagramStackRef `json:"diagramStackRef,omitempty"`
}

// PromptNode is a single node in a PromptDocument's tree. Container node
// types nest further nodes under Content; Text is a leaf carrying the
// literal string, with any inline formatting under Marks.
type PromptNode struct {
	Type    PromptNodeType   `json:"type"`
	Attrs   *PromptNodeAttrs `json:"attrs,omitempty"`
	Content []PromptNode     `json:"content,omitempty"`
	Text    string           `json:"text,omitempty"`
	Marks   []PromptMark     `json:"marks,omitempty"`
}

// PromptMarkType identifies the kind of an inline PromptMark. Only the mark
// types the exercise-prompt editor can actually produce are valid here.
type PromptMarkType string

const (
	PromptMarkTypeBold      PromptMarkType = "bold"
	PromptMarkTypeItalic    PromptMarkType = "italic"
	PromptMarkTypeStrike    PromptMarkType = "strike"
	PromptMarkTypeHighlight PromptMarkType = "highlight"
	PromptMarkTypeLink      PromptMarkType = "link"
	// PromptMarkTypeTextStyle carries a chosen font color, background
	// color, or both, via its Attrs.Color and Attrs.BackgroundColor.
	PromptMarkTypeTextStyle PromptMarkType = "textStyle"
)

// PromptMarkAttrs holds the type-specific attributes a PromptMark may
// carry. Which fields are set depends on the mark's Type; the rest stay
// nil.
type PromptMarkAttrs struct {
	Href            *string `json:"href,omitempty"`
	Color           *string `json:"color,omitempty"`
	BackgroundColor *string `json:"backgroundColor,omitempty"`
}

// PromptMark is an inline formatting mark applied to a PromptNode.
type PromptMark struct {
	Type  PromptMarkType   `json:"type"`
	Attrs *PromptMarkAttrs `json:"attrs,omitempty"`
}

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
// ImageURL, AudioURL, or Region is populated depends on the parent exercise's
// ExerciseType.
type Option struct {
	ID        string
	IsCorrect bool
	Label     *string
	ImageURL  *string
	AudioURL  *string
	Region    *OptionRegion
	// DiagramRef renders this option's own thumbnail from a prebuilt
	// diagram, replacing ImageURL — only meaningful for image_choice
	// options.
	DiagramRef *DiagramRef
	// DiagramID and DiagramPositionID are server-derived and read-only:
	// present only on an option that came from an image_recognition
	// exercise's DiagramRef/DiagramStackRef stimulus, identifying which
	// Diagram and Position this option represents. Never set by request
	// construction — the application layer derives them.
	DiagramID         *string
	DiagramPositionID *string
}

// RemediationTarget is one piece of content recommended to a student who
// answers a specific exercise incorrectly. Exactly one of ContentNodeID or
// RichContent must be set — a target is either a reference to an existing
// content node, or inline-authored content (e.g. a specific external video
// or article, with a caption explaining why it's suggested) using the same
// rich-content model as an exercise prompt.
type RemediationTarget struct {
	ContentNodeID *string
	RichContent   *PromptDocument
	Caption       *string
}

// Exercise is a reusable, standalone practice item classified by Skill/
// Concept tree references — the same shared trees ContentNode classifies
// against — and independent of any single challenge. It is checked by
// option selection: the student's selected option ID(s) must match the
// option(s) marked correct.
type Exercise struct {
	ID           string
	Title        string
	Prompt       PromptDocument
	ExerciseType ExerciseType
	// Skills/Concepts carry only ID until this Exercise is read back from
	// the repository with its Skill/Concept rows joined in — the same
	// construct-then-refetch convention Languages already follows.
	Skills   []Skill
	Concepts []Concept
	ImageURL *string
	AudioURL *string
	// DiagramRef and DiagramStackRef, when set, replace ImageURL as an
	// image_recognition exercise's stimulus — the diagram's own positions
	// (after layers.subset filtering) become this exercise's options
	// automatically. Meaningless for any other ExerciseType.
	DiagramRef               *DiagramRef
	DiagramStackRef          *DiagramStackRef
	Options                  []Option
	EstimatedDurationSeconds *int
	// RemediationTargets is the ordered content recommended to a student who
	// answers this exercise incorrectly. May be empty.
	RemediationTargets []RemediationTarget
	// ChallengeIDs are the challenges this exercise is currently linked to.
	// Managed exclusively through LinkChallenge/UnlinkChallenge — never set
	// directly by NewExercise beyond the empty slice a brand-new exercise
	// starts with.
	ChallengeIDs []string
	// ContentNodeIDs are the content nodes this exercise is currently linked
	// to as a path exercise. Managed exclusively through
	// LinkContentNode/UnlinkContentNode, independent of ChallengeIDs.
	ContentNodeIDs []string
	// Languages are the languages this exercise is available in, or a single
	// LanguageCodeAny entry for language-agnostic content. Independent of
	// any ContentNode's languages — an exercise reused across nodes has no
	// single parent to inherit a language from. Name is populated only once
	// this Exercise has been read back from the repository with its
	// Language rows joined in.
	Languages []Language
	CreatedAt time.Time
}

// NewExercise validates and constructs a standalone Exercise, not yet linked
// to any challenge or content node. Whether it later gets linked to a
// challenge or node that exists is an application-layer concern — as is
// whether each remediationTargets[i].ContentNodeID refers to a content node
// that actually exists, which requires a repository round-trip this
// constructor can't perform.
func NewExercise(id, title string, prompt PromptDocument, exerciseType ExerciseType, skillIDs, conceptIDs []string, imageURL, audioURL *string, diagramRef *DiagramRef, diagramStackRef *DiagramStackRef, options []Option, estimatedDurationSeconds *int, remediationTargets []RemediationTarget, languageCodes []string, createdAt time.Time) (Exercise, error) {
	errs := validateExerciseType(exerciseType)
	errs = append(errs, validateExerciseContent(title, prompt, exerciseType, skillIDs, conceptIDs, imageURL, audioURL, diagramRef, diagramStackRef, options, estimatedDurationSeconds, remediationTargets)...)
	errs = append(errs, validateLanguageCodes("language_codes", languageCodes)...)
	if len(errs) > 0 {
		return Exercise{}, &ValidationError{Fields: errs}
	}

	return Exercise{
		ID:                       id,
		Title:                    title,
		Prompt:                   prompt,
		ExerciseType:             exerciseType,
		Skills:                   skillsFromIDs(skillIDs),
		Concepts:                 conceptsFromIDs(conceptIDs),
		ImageURL:                 imageURL,
		AudioURL:                 audioURL,
		DiagramRef:               diagramRef,
		DiagramStackRef:          diagramStackRef,
		Options:                  options,
		EstimatedDurationSeconds: estimatedDurationSeconds,
		RemediationTargets:       remediationTargets,
		ChallengeIDs:             []string{},
		ContentNodeIDs:           []string{},
		Languages:                languagesFromCodes(languageCodes),
		CreatedAt:                createdAt,
	}, nil
}

// Update validates and returns a copy of e with its authored content
// replaced: title, prompt, skill tags, stimulus media, options, estimated
// duration, and remediation targets. ID, ExerciseType, ChallengeIDs,
// ContentNodeIDs, and CreatedAt carry over unchanged — exercise_type cannot
// change after creation since it determines the option shape (region vs.
// text vs. image), and links are managed exclusively through the exercise's
// Link/Unlink operations, not through an update.
func (e Exercise) Update(title string, prompt PromptDocument, skillIDs, conceptIDs []string, imageURL, audioURL *string, diagramRef *DiagramRef, diagramStackRef *DiagramStackRef, options []Option, estimatedDurationSeconds *int, remediationTargets []RemediationTarget, languageCodes []string) (Exercise, error) {
	errs := validateExerciseContent(title, prompt, e.ExerciseType, skillIDs, conceptIDs, imageURL, audioURL, diagramRef, diagramStackRef, options, estimatedDurationSeconds, remediationTargets)
	errs = append(errs, validateLanguageCodes("language_codes", languageCodes)...)
	if len(errs) > 0 {
		return Exercise{}, &ValidationError{Fields: errs}
	}

	updated := e
	updated.Title = title
	updated.Prompt = prompt
	updated.Skills = skillsFromIDs(skillIDs)
	updated.Concepts = conceptsFromIDs(conceptIDs)
	updated.ImageURL = imageURL
	updated.AudioURL = audioURL
	updated.DiagramRef = diagramRef
	updated.DiagramStackRef = diagramStackRef
	updated.Options = options
	updated.EstimatedDurationSeconds = estimatedDurationSeconds
	updated.RemediationTargets = remediationTargets
	updated.Languages = languagesFromCodes(languageCodes)
	return updated, nil
}

// validateExerciseContent checks the fields shared by creation and update —
// everything except exercise_type itself, which only creation sets and only
// creation validates.
func validateExerciseContent(title string, prompt PromptDocument, exerciseType ExerciseType, skillIDs, conceptIDs []string, imageURL, audioURL *string, diagramRef *DiagramRef, diagramStackRef *DiagramStackRef, options []Option, estimatedDurationSeconds *int, remediationTargets []RemediationTarget) []FieldError {
	var errs []FieldError

	if title == "" {
		errs = append(errs, FieldError{Field: "title", Reason: "must not be empty"})
	}
	errs = append(errs, validatePromptDocument(prompt, false)...)
	if len(skillIDs) == 0 {
		errs = append(errs, FieldError{Field: "skill_ids", Reason: "must not be empty"})
	}
	if len(conceptIDs) == 0 {
		errs = append(errs, FieldError{Field: "concept_ids", Reason: "must not be empty"})
	}
	errs = append(errs, validateStimulusMedia(exerciseType, imageURL, audioURL, diagramRef, diagramStackRef)...)
	errs = append(errs, validateOptions(exerciseType, diagramRef, diagramStackRef, options)...)
	if estimatedDurationSeconds != nil && *estimatedDurationSeconds < 1 {
		errs = append(errs, FieldError{Field: "estimated_duration_seconds", Reason: "must be at least 1 when present"})
	}
	errs = append(errs, validateRemediationTargets(remediationTargets)...)

	return errs
}

// validateRemediationTargets checks that each target carries exactly one of
// ContentNodeID or RichContent, and that any RichContent is a well-formed
// document. Whether a ContentNodeID refers to a content node that actually
// exists is an application-layer concern.
func validateRemediationTargets(targets []RemediationTarget) []FieldError {
	var errs []FieldError
	for _, target := range targets {
		hasNode := target.ContentNodeID != nil && *target.ContentNodeID != ""
		hasRich := target.RichContent != nil
		switch {
		case hasNode == hasRich:
			errs = append(errs, FieldError{Field: "remediation_targets", Reason: "each target must carry exactly one of content_node_id or rich_content"})
		case hasRich:
			for _, docErr := range validatePromptDocument(*target.RichContent, true) {
				errs = append(errs, FieldError{Field: "remediation_targets", Reason: docErr.Reason})
			}
		}
	}
	return errs
}

// validatePromptDocument checks that prompt is a well-formed document
// (a "doc" root with at least one block node) using only the node and mark
// types the given surface's editor can actually produce. allowMediaNodes
// permits PromptNodeTypeAudio/Video — offered by the rich_text
// ExpandedContent and remediation-target editors, but never by the
// exercise-prompt authoring toolbar, so an exercise's own Prompt must
// reject them even though this validation function is shared across all
// three surfaces.
func validatePromptDocument(prompt PromptDocument, allowMediaNodes bool) []FieldError {
	if prompt.Type != "doc" {
		return []FieldError{{Field: "prompt", Reason: "must be a structured document with type \"doc\""}}
	}
	if len(prompt.Content) == 0 {
		return []FieldError{{Field: "prompt", Reason: "must not be empty"}}
	}
	for _, node := range prompt.Content {
		if reason := promptNodeError(node, allowMediaNodes); reason != "" {
			return []FieldError{{Field: "prompt", Reason: reason}}
		}
	}
	return nil
}

// promptNodeError reports the reason node (and, recursively, its content
// and marks) is invalid, or "" if it's valid.
func promptNodeError(node PromptNode, allowMediaNodes bool) string {
	switch node.Type {
	case PromptNodeTypeHeading, PromptNodeTypeParagraph, PromptNodeTypeText,
		PromptNodeTypeBulletList, PromptNodeTypeOrderedList, PromptNodeTypeListItem,
		PromptNodeTypeTable, PromptNodeTypeTableRow, PromptNodeTypeTableHeader, PromptNodeTypeTableCell,
		PromptNodeTypeImage:
	case PromptNodeTypeAudio, PromptNodeTypeVideo:
		if !allowMediaNodes {
			return "contains an unsupported node type \"" + string(node.Type) + "\""
		}
	case PromptNodeTypeDiagram:
		if reason := diagramPromptNodeError(node.Attrs); reason != "" {
			return reason
		}
	default:
		return "contains an unsupported node type \"" + string(node.Type) + "\""
	}

	for _, child := range node.Content {
		if reason := promptNodeError(child, allowMediaNodes); reason != "" {
			return reason
		}
	}
	for _, mark := range node.Marks {
		if reason := promptMarkError(mark); reason != "" {
			return reason
		}
	}
	return ""
}

// diagramPromptNodeError reports the reason a diagram PromptNode's attrs are
// invalid, or "" if they're valid: exactly one of DiagramRef/DiagramStackRef
// must be set, and it must itself be structurally valid.
func diagramPromptNodeError(attrs *PromptNodeAttrs) string {
	hasRef := attrs != nil && attrs.DiagramRef != nil
	hasStack := attrs != nil && attrs.DiagramStackRef != nil
	switch {
	case hasRef == hasStack:
		return "a diagram node must carry exactly one of diagram_ref or diagram_stack_ref"
	case hasRef:
		if err := ValidateDiagramRef(*attrs.DiagramRef); err != nil {
			return "diagram_ref: " + err.Error()
		}
	case hasStack:
		if err := ValidateDiagramStackRef(*attrs.DiagramStackRef); err != nil {
			return "diagram_stack_ref: " + err.Error()
		}
	}
	return ""
}

// promptMarkError reports the reason mark is invalid, or "" if it's valid.
func promptMarkError(mark PromptMark) string {
	switch mark.Type {
	case PromptMarkTypeBold, PromptMarkTypeItalic, PromptMarkTypeStrike, PromptMarkTypeHighlight, PromptMarkTypeLink, PromptMarkTypeTextStyle:
		return ""
	default:
		return "contains an unsupported mark type \"" + string(mark.Type) + "\""
	}
}

func validateExerciseType(exerciseType ExerciseType) []FieldError {
	switch exerciseType {
	case ExerciseTypeTextResponse, ExerciseTypeAudioRecognition, ExerciseTypeImageRecognition, ExerciseTypeImageChoice, ExerciseTypeAudioSelection:
		return nil
	default:
		return []FieldError{{Field: "exercise_type", Reason: "must be one of text_response, audio_recognition, image_recognition, image_choice, audio_selection"}}
	}
}

// validateStimulusMedia checks the exercise-level stimulus required by
// exerciseType. image_recognition needs exactly one of image_url,
// diagram_ref, or diagram_stack_ref; a diagram stimulus must carry
// correct_intervals (or, for a stack, each entry must) since that is what
// makes its positions gradable, and must itself be structurally valid.
// audio_recognition needs a stimulus audio clip. Other types carry no
// exercise-level stimulus.
func validateStimulusMedia(exerciseType ExerciseType, imageURL, audioURL *string, diagramRef *DiagramRef, diagramStackRef *DiagramStackRef) []FieldError {
	var errs []FieldError

	if exerciseType == ExerciseTypeImageRecognition {
		hasImage := imageURL != nil && *imageURL != ""
		hasRef := diagramRef != nil
		hasStack := diagramStackRef != nil
		switch {
		case countTrue(hasImage, hasRef, hasStack) == 0:
			errs = append(errs, FieldError{Field: "image_url", Reason: "one of image_url, diagram_ref, or diagram_stack_ref is required when exercise_type is image_recognition"})
		case countTrue(hasImage, hasRef, hasStack) > 1:
			errs = append(errs, FieldError{Field: "image_url", Reason: "at most one of image_url, diagram_ref, or diagram_stack_ref may be set"})
		case hasRef:
			errs = append(errs, validateDiagramStimulusRef(*diagramRef)...)
		case hasStack:
			if err := ValidateDiagramStackRef(*diagramStackRef); err != nil {
				errs = append(errs, FieldError{Field: "diagram_stack_ref", Reason: err.Error()})
			}
			for _, ref := range diagramStackRef.Stack {
				errs = append(errs, validateDiagramStimulusRef(ref)...)
			}
		}
	} else if diagramRef != nil || diagramStackRef != nil {
		errs = append(errs, FieldError{Field: "diagram_ref", Reason: "only meaningful when exercise_type is image_recognition"})
	}

	if exerciseType == ExerciseTypeAudioRecognition && (audioURL == nil || *audioURL == "") {
		errs = append(errs, FieldError{Field: "audio_url", Reason: "required when exercise_type is audio_recognition"})
	}
	return errs
}

// validateDiagramStimulusRef checks that ref is structurally valid and
// carries correct_intervals — required only when a DiagramRef is an
// Exercise's image_recognition stimulus, unlike its other usages.
func validateDiagramStimulusRef(ref DiagramRef) []FieldError {
	var errs []FieldError
	if err := ValidateDiagramRef(ref); err != nil {
		errs = append(errs, FieldError{Field: "diagram_ref", Reason: err.Error()})
	}
	if ref.CorrectIntervals == nil {
		errs = append(errs, FieldError{Field: "diagram_ref", Reason: "correct_intervals is required when used as an image_recognition stimulus"})
	}
	return errs
}

// countTrue returns how many of bs are true.
func countTrue(bs ...bool) int {
	n := 0
	for _, b := range bs {
		if b {
			n++
		}
	}
	return n
}

// validateOptions checks that at least one option exists, at least one is
// marked correct, and each option's shape matches exerciseType.
func validateOptions(exerciseType ExerciseType, diagramRef *DiagramRef, diagramStackRef *DiagramStackRef, options []Option) []FieldError {
	if len(options) == 0 {
		return []FieldError{{Field: "options", Reason: "must contain at least one option"}}
	}

	diagramDriven := exerciseType == ExerciseTypeImageRecognition && (diagramRef != nil || diagramStackRef != nil)

	var errs []FieldError
	hasCorrect := false
	for _, opt := range options {
		if opt.IsCorrect {
			hasCorrect = true
		}
		if reason := optionShapeError(exerciseType, diagramDriven, opt); reason != "" {
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
// region on the exercise's image, or — when diagramDriven — are entirely
// server-derived (diagram_id/diagram_position_id) from the exercise's
// diagram stimulus; image_choice options each carry their own image or
// diagram_ref; audio_selection options each carry their own audio clip;
// text_response and audio_recognition options carry a text label.
func optionShapeError(exerciseType ExerciseType, diagramDriven bool, opt Option) string {
	switch exerciseType {
	case ExerciseTypeImageRecognition:
		return imageRecognitionOptionShapeError(diagramDriven, opt)
	case ExerciseTypeImageChoice:
		return imageChoiceOptionShapeError(opt)
	case ExerciseTypeAudioSelection:
		if opt.AudioURL == nil || *opt.AudioURL == "" {
			return "audio_selection options must carry an audio_url"
		}
	case ExerciseTypeTextResponse, ExerciseTypeAudioRecognition:
		if opt.Label == nil || *opt.Label == "" {
			return "text_response and audio_recognition options must carry a label"
		}
	}
	return ""
}

// imageRecognitionOptionShapeError reports the reason an image_recognition
// option's shape is invalid, or "" if it's valid: a region on the
// exercise's image, or — when diagramDriven — server-derived diagram_id/
// diagram_position_id from the exercise's diagram stimulus.
func imageRecognitionOptionShapeError(diagramDriven bool, opt Option) string {
	if diagramDriven {
		if opt.DiagramID == nil || *opt.DiagramID == "" || opt.DiagramPositionID == nil || *opt.DiagramPositionID == "" {
			return "diagram-driven image_recognition options must carry diagram_id and diagram_position_id"
		}
		return ""
	}
	if opt.Region == nil {
		return "image_recognition options must carry a region"
	}
	return ""
}

// imageChoiceOptionShapeError reports the reason an image_choice option's
// shape is invalid, or "" if it's valid: exactly one of an image_url or a
// structurally valid diagram_ref.
func imageChoiceOptionShapeError(opt Option) string {
	hasImage := opt.ImageURL != nil && *opt.ImageURL != ""
	hasDiagram := opt.DiagramRef != nil
	switch {
	case !hasImage && !hasDiagram:
		return "image_choice options must carry an image_url or a diagram_ref"
	case hasImage && hasDiagram:
		return "image_choice options must not carry both an image_url and a diagram_ref"
	case hasDiagram:
		if err := ValidateDiagramRef(*opt.DiagramRef); err != nil {
			return "diagram_ref: " + err.Error()
		}
	}
	return ""
}
