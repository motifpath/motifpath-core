package http

import (
	"encoding/json"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// mustUUID parses an id produced by this service's own newID generator
// (uuid.NewString, per the application layer's constructor injection).
// It is always valid — a parse failure here means persisted data was
// corrupted, not a request-handling error, so panicking is the right
// failure mode rather than threading another error path through every
// mapper.
func mustUUID(id string) uuid.UUID {
	return uuid.MustParse(id)
}

func toGeneratedLanguage(l domain.Language) generated.Language {
	return generated.Language{Code: l.Code, Name: l.Name}
}

func toGeneratedLanguages(languages []domain.Language) []generated.Language {
	result := make([]generated.Language, len(languages))
	for i, l := range languages {
		result[i] = toGeneratedLanguage(l)
	}
	return result
}

func toUserProfile(u domain.User) generated.UserProfile {
	return generated.UserProfile{
		UserId:       mustUUID(u.ID),
		Role:         generated.UserProfileRole(u.Role),
		Locale:       toGeneratedLanguage(u.Locale),
		RegisteredAt: u.RegisteredAt,
	}
}

func toGeneratedSkill(s domain.Skill) generated.Skill {
	skill := generated.Skill{SkillId: mustUUID(s.ID), Name: s.Name}
	if s.ParentID != nil {
		id := mustUUID(*s.ParentID)
		skill.ParentId = &id
	}
	return skill
}

func toGeneratedSkills(skills []domain.Skill) []generated.Skill {
	result := make([]generated.Skill, len(skills))
	for i, s := range skills {
		result[i] = toGeneratedSkill(s)
	}
	return result
}

func toGeneratedConcept(c domain.Concept) generated.Concept {
	concept := generated.Concept{ConceptId: mustUUID(c.ID), Name: c.Name}
	if c.ParentID != nil {
		id := mustUUID(*c.ParentID)
		concept.ParentId = &id
	}
	return concept
}

func toGeneratedConcepts(concepts []domain.Concept) []generated.Concept {
	result := make([]generated.Concept, len(concepts))
	for i, c := range concepts {
		result[i] = toGeneratedConcept(c)
	}
	return result
}

func toContentNodeVersion(v domain.ContentNodeVersion) generated.ContentNodeVersion {
	result := generated.ContentNodeVersion{
		ContentNodeId: mustUUID(v.ContentNodeID),
		VersionNumber: v.VersionNumber,
		TitleSnapshot: v.Title,
		ClassificationSnapshot: generated.Classification{
			Skills:          toGeneratedSkills(v.Classification.Skills),
			Concepts:        toGeneratedConcepts(v.Classification.Concepts),
			DifficultyLevel: generated.ClassificationDifficultyLevel(v.Classification.DifficultyLevel),
			ReviewState:     generated.ClassificationReviewState(v.Classification.ReviewState),
		},
		LanguagesSnapshot: toGeneratedLanguages(v.Languages),
		MediaUrlSnapshot:  v.MediaURL,
		PublishedAt:       v.PublishedAt,
	}
	if v.RichContent != nil {
		doc := toGeneratedPromptDocument(*v.RichContent)
		result.RichContentSnapshot = &doc
	}
	return result
}

func toContentNode(n domain.ContentNode) generated.ContentNode {
	result := generated.ContentNode{
		ContentNodeId: mustUUID(n.ID),
		TeacherId:     mustUUID(n.TeacherID),
		Title:         n.Title,
		ContentType:   generated.ContentNodeContentType(n.ContentType),
		Classification: generated.Classification{
			Skills:          toGeneratedSkills(n.Classification.Skills),
			Concepts:        toGeneratedConcepts(n.Classification.Concepts),
			DifficultyLevel: generated.ClassificationDifficultyLevel(n.Classification.DifficultyLevel),
			ReviewState:     generated.ClassificationReviewState(n.Classification.ReviewState),
		},
		MediaUrl:  n.MediaURL,
		Languages: toGeneratedLanguages(n.Languages),
		CreatedAt: n.CreatedAt,
	}
	if n.RichContent != nil {
		doc := toGeneratedPromptDocument(*n.RichContent)
		result.RichContent = &doc
	}
	return result
}

func toContentNodes(nodes []domain.ContentNode) []generated.ContentNode {
	result := make([]generated.ContentNode, 0, len(nodes))
	for _, n := range nodes {
		result = append(result, toContentNode(n))
	}
	return result
}

func toChallenge(c domain.Challenge) generated.Challenge {
	challenge := generated.Challenge{
		ChallengeId:      mustUUID(c.ID),
		ContentNodeId:    mustUUID(c.ContentNodeID),
		PassThreshold:    c.PassThreshold,
		TimeThresholdMs:  c.TimeThresholdMS,
		ShuffleExercises: c.ShuffleExercises,
		ShuffleOptions:   c.ShuffleOptions,
		CreatedAt:        c.CreatedAt,
	}
	if c.SubjectSkillID != nil {
		id := mustUUID(*c.SubjectSkillID)
		challenge.SubjectSkillId = &id
	}
	if c.SubjectConceptID != nil {
		id := mustUUID(*c.SubjectConceptID)
		challenge.SubjectConceptId = &id
	}
	return challenge
}

func toChallenges(challenges []domain.Challenge) []generated.Challenge {
	result := make([]generated.Challenge, len(challenges))
	for i, c := range challenges {
		result[i] = toChallenge(c)
	}
	return result
}

// toGeneratedPromptDocument converts a domain.PromptDocument to its wire
// shape via a JSON round trip. generated.PromptNode.Attrs is a generic map
// and domain.PromptNode.Attrs is a typed struct sharing the same JSON field
// names, so encoding/json bridges the two without either side reading the
// map's values directly. Marshaling a value of this shape and unmarshaling
// it into the structurally compatible target cannot fail; a failure here
// means one of the two types drifted out of sync with the other, which
// panicking surfaces immediately rather than silently.
func toGeneratedPromptDocument(prompt domain.PromptDocument) generated.PromptDocument {
	data, err := json.Marshal(prompt)
	if err != nil {
		panic(err)
	}
	var out generated.PromptDocument
	if err := json.Unmarshal(data, &out); err != nil {
		panic(err)
	}
	return out
}

// toDomainPromptDocument converts a generated.PromptDocument, as already
// decoded from a request body into its generated Go shape, to its domain
// shape — the reverse of toGeneratedPromptDocument, with the same
// unreachable-failure reasoning.
func toDomainPromptDocument(prompt generated.PromptDocument) domain.PromptDocument {
	data, err := json.Marshal(prompt)
	if err != nil {
		panic(err)
	}
	var out domain.PromptDocument
	if err := json.Unmarshal(data, &out); err != nil {
		panic(err)
	}
	return out
}

// toDomainPromptDocumentPtr converts an optional generated.PromptDocument
// pointer to its optional domain shape, preserving nil.
func toDomainPromptDocumentPtr(prompt *generated.PromptDocument) *domain.PromptDocument {
	if prompt == nil {
		return nil
	}
	doc := toDomainPromptDocument(*prompt)
	return &doc
}

func toExercise(e domain.Exercise) generated.Exercise {
	challengeIDs := make([]uuid.UUID, len(e.ChallengeIDs))
	for i, id := range e.ChallengeIDs {
		challengeIDs[i] = mustUUID(id)
	}
	contentNodeIDs := make([]uuid.UUID, len(e.ContentNodeIDs))
	for i, id := range e.ContentNodeIDs {
		contentNodeIDs[i] = mustUUID(id)
	}
	options := make([]generated.Option, len(e.Options))
	for i, opt := range e.Options {
		options[i] = toOption(opt)
	}

	exercise := generated.Exercise{
		ExerciseId:               mustUUID(e.ID),
		Title:                    e.Title,
		Prompt:                   toGeneratedPromptDocument(e.Prompt),
		ExerciseType:             generated.ExerciseExerciseType(e.ExerciseType),
		Skills:                   toGeneratedSkills(e.Skills),
		Concepts:                 toGeneratedConcepts(e.Concepts),
		ImageUrl:                 e.ImageURL,
		AudioUrl:                 e.AudioURL,
		Options:                  options,
		ChallengeIds:             challengeIDs,
		ContentNodeIds:           contentNodeIDs,
		Languages:                toGeneratedLanguages(e.Languages),
		EstimatedDurationSeconds: e.EstimatedDurationSeconds,
		RemediationTargets:       toRemediationTargets(e.RemediationTargets),
		CreatedAt:                e.CreatedAt,
		DiagramRef:               toGeneratedDiagramRefPtr(e.DiagramRef),
		DiagramStackRef:          toGeneratedDiagramStackRefPtr(e.DiagramStackRef),
	}
	return exercise
}

func toRemediationTargets(targets []domain.RemediationTarget) []generated.RemediationTarget {
	result := make([]generated.RemediationTarget, len(targets))
	for i, t := range targets {
		target := generated.RemediationTarget{Caption: t.Caption}
		if t.ContentNodeID != nil {
			id := mustUUID(*t.ContentNodeID)
			target.ContentNodeId = &id
		}
		if t.RichContent != nil {
			doc := toGeneratedPromptDocument(*t.RichContent)
			target.RichContent = &doc
		}
		result[i] = target
	}
	return result
}

func toDomainRemediationTargets(targets []generated.RemediationTarget) []domain.RemediationTarget {
	result := make([]domain.RemediationTarget, len(targets))
	for i, t := range targets {
		target := domain.RemediationTarget{Caption: t.Caption}
		if t.ContentNodeId != nil {
			id := t.ContentNodeId.String()
			target.ContentNodeID = &id
		}
		if t.RichContent != nil {
			doc := toDomainPromptDocument(*t.RichContent)
			target.RichContent = &doc
		}
		result[i] = target
	}
	return result
}

func toExercises(exercises []domain.Exercise) []generated.Exercise {
	result := make([]generated.Exercise, len(exercises))
	for i, e := range exercises {
		result[i] = toExercise(e)
	}
	return result
}

func toPracticeSession(session application.PracticeSession) generated.PracticeSession {
	return generated.PracticeSession{
		PracticeSessionId: mustUUID(session.ID),
		SkillId:           mustUUID(session.SkillID),
		Exercises:         toExercises(session.Exercises),
	}
}

func toOption(o domain.Option) generated.Option {
	option := generated.Option{
		OptionId:  mustUUID(o.ID),
		IsCorrect: o.IsCorrect,
		Label:     o.Label,
		ImageUrl:  o.ImageURL,
		AudioUrl:  o.AudioURL,
	}
	if o.Region != nil {
		option.Region = &generated.OptionRegion{
			X:      float32(o.Region.X),
			Y:      float32(o.Region.Y),
			Width:  float32(o.Region.Width),
			Height: float32(o.Region.Height),
			Shape:  generated.OptionRegionShape(o.Region.Shape),
		}
	}
	option.DiagramRef = toGeneratedDiagramRefPtr(o.DiagramRef)
	if o.DiagramID != nil {
		id := mustUUID(*o.DiagramID)
		option.DiagramId = &id
	}
	if o.DiagramPositionID != nil {
		id := mustUUID(*o.DiagramPositionID)
		option.DiagramPositionId = &id
	}
	return option
}

func toDomainOptions(options []generated.Option) []domain.Option {
	result := make([]domain.Option, len(options))
	for i, opt := range options {
		result[i] = domain.Option{
			ID:        opt.OptionId.String(),
			IsCorrect: opt.IsCorrect,
			Label:     opt.Label,
			ImageURL:  opt.ImageUrl,
			AudioURL:  opt.AudioUrl,
		}
		if opt.Region != nil {
			result[i].Region = &domain.OptionRegion{
				X:      float64(opt.Region.X),
				Y:      float64(opt.Region.Y),
				Width:  float64(opt.Region.Width),
				Height: float64(opt.Region.Height),
				Shape:  domain.OptionRegionShape(opt.Region.Shape),
			}
		}
		result[i].DiagramRef = toDomainDiagramRefPtr(opt.DiagramRef)
	}
	return result
}

func toMediaUploadURL(u domain.MediaUploadURL) generated.MediaUploadUrl {
	return generated.MediaUploadUrl{
		UploadUrl: u.UploadURL,
		ObjectUrl: u.ObjectURL,
		ExpiresAt: u.ExpiresAt,
	}
}

func toExpandedContent(item domain.ExpandedContent) generated.ExpandedContent {
	result := generated.ExpandedContent{
		ExpandedContentId:  mustUUID(item.ID),
		ContentNodeId:      mustUUID(item.ContentNodeID),
		ContentType:        generated.ExpandedContentContentType(item.ContentType),
		MediaUrl:           item.MediaURL,
		TriggerAtSeconds:   item.TriggerAtSeconds,
		HideAtSeconds:      item.HideAtSeconds,
		TriggerAtParagraph: item.TriggerAtParagraph,
		DurationMs:         item.DurationMS,
		Caption:            item.Caption,
		CreatedAt:          item.CreatedAt,
		DiagramRef:         toGeneratedDiagramRefPtr(item.DiagramRef),
		DiagramStackRef:    toGeneratedDiagramStackRefPtr(item.DiagramStackRef),
	}
	if item.RichContent != nil {
		doc := toGeneratedPromptDocument(*item.RichContent)
		result.RichContent = &doc
	}
	return result
}

func toLearningPathItem(item domain.LearningPathItem) generated.LearningPathItem {
	return generated.LearningPathItem{
		Position:      item.Position,
		ContentNodeId: mustUUID(item.ContentNodeID),
		Title:         item.Title,
		ContentType:   generated.LearningPathItemContentType(item.ContentType),
		SectionLabel:  item.SectionLabel,
	}
}

func toLearningPath(p domain.LearningPath) generated.LearningPath {
	items := make([]generated.LearningPathItem, len(p.Items))
	for i, item := range p.Items {
		items[i] = toLearningPathItem(item)
	}
	return generated.LearningPath{
		LearningPathId: mustUUID(p.ID),
		TeacherId:      mustUUID(p.TeacherID),
		Title:          p.Title,
		Items:          items,
		CreatedAt:      p.CreatedAt,
	}
}

func toLearningPaths(paths []domain.LearningPath) []generated.LearningPath {
	result := make([]generated.LearningPath, 0, len(paths))
	for _, p := range paths {
		result = append(result, toLearningPath(p))
	}
	return result
}

func toStudentPath(sp domain.StudentPath) generated.StudentPath {
	result := generated.StudentPath{
		StudentPathId:            mustUUID(sp.ID),
		StudentId:                mustUUID(sp.StudentID),
		SourceTemplateId:         mustUUID(sp.SourceTemplateID),
		Title:                    sp.Title,
		AssignedBy:               mustUUID(sp.AssignedBy),
		AssignedAt:               sp.AssignedAt,
		ArchivedAt:               sp.ArchivedAt,
		CourseCheckpointPosition: sp.CourseCheckpointPosition,
	}
	if sp.SourceCourseEnrollmentID != nil {
		id := mustUUID(*sp.SourceCourseEnrollmentID)
		result.SourceCourseEnrollmentId = &id
	}
	return result
}

func toStudentPathItem(item domain.StudentPathItem) generated.StudentPathItem {
	return generated.StudentPathItem{
		Position:             item.Position,
		ContentNodeId:        mustUUID(item.ContentNodeID),
		ContentNodeVersionId: mustUUID(item.ContentNodeVersionID),
		Title:                item.Title,
		ContentType:          generated.StudentPathItemContentType(item.ContentType),
		Status:               generated.StudentPathItemStatus(item.Status),
		SectionLabel:         item.SectionLabel,
	}
}

func toStudentPathView(v application.StudentPathView) generated.StudentPathView {
	items := make([]generated.StudentPathItem, len(v.Items))
	for i, item := range v.Items {
		items[i] = toStudentPathItem(item)
	}
	view := generated.StudentPathView{
		StudentPathId:            mustUUID(v.StudentPathID),
		SourceTemplateId:         mustUUID(v.SourceTemplateID),
		Title:                    v.Title,
		CurrentPosition:          v.CurrentPosition,
		Items:                    items,
		CourseCheckpointPosition: v.CourseCheckpointPosition,
		CourseCompleted:          v.CourseCompleted,
	}
	if v.CourseEnrollmentID != nil {
		id := mustUUID(*v.CourseEnrollmentID)
		view.CourseEnrollmentId = &id
	}
	return view
}

func toCourseEnrollment(e domain.CourseEnrollment) generated.CourseEnrollment {
	result := generated.CourseEnrollment{
		CourseEnrollmentId:       mustUUID(e.ID),
		StudentId:                mustUUID(e.StudentID),
		CourseId:                 mustUUID(e.CourseID),
		CourseTitle:              e.CourseTitle,
		CourseVersionNumber:      e.CourseVersionNumber,
		Status:                   generated.CourseEnrollmentStatus(e.Status),
		ActiveCheckpointPosition: e.ActiveCheckpointPosition,
		EnrolledAt:               e.EnrolledAt,
	}
	if e.ActiveCheckpointStudentPathID != nil {
		id := mustUUID(*e.ActiveCheckpointStudentPathID)
		result.ActiveCheckpointStudentPathId = &id
	}
	return result
}

func toCourseEnrollments(enrollments []domain.CourseEnrollment) []generated.CourseEnrollment {
	result := make([]generated.CourseEnrollment, len(enrollments))
	for i, e := range enrollments {
		result[i] = toCourseEnrollment(e)
	}
	return result
}

// derefOptions returns the options a request carried, or nil when the field
// was omitted. Omission is legitimate only for a diagram-driven exercise; the
// application layer decides whether it is acceptable for the exercise type.
func derefOptions(options *[]generated.Option) []generated.Option {
	if options == nil {
		return nil
	}
	return *options
}

// uuidPtrToString converts an optional query-parameter UUID to a string id,
// with "" meaning "no filter".
func uuidPtrToString(id *openapi_types.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

// toGeneratedDiagramRef converts a domain.DiagramRef to its wire shape via a
// JSON round trip, the same bridging technique toGeneratedPromptDocument
// uses: both sides share JSON field names (domain.DiagramRef is tagged to
// match), so encoding/json bridges the two without a field-by-field
// constructor. A failure here means the two shapes drifted out of sync,
// which panicking surfaces immediately.
func toGeneratedDiagramRef(ref domain.DiagramRef) generated.DiagramRef {
	data, err := json.Marshal(ref)
	if err != nil {
		panic(err)
	}
	var out generated.DiagramRef
	if err := json.Unmarshal(data, &out); err != nil {
		panic(err)
	}
	return out
}

func toGeneratedDiagramRefPtr(ref *domain.DiagramRef) *generated.DiagramRef {
	if ref == nil {
		return nil
	}
	out := toGeneratedDiagramRef(*ref)
	return &out
}

func toGeneratedDiagramStackRef(stack domain.DiagramStackRef) generated.DiagramStackRef {
	refs := make([]generated.DiagramRef, len(stack.Stack))
	for i, ref := range stack.Stack {
		refs[i] = toGeneratedDiagramRef(ref)
	}
	return generated.DiagramStackRef{Stack: refs}
}

func toGeneratedDiagramStackRefPtr(stack *domain.DiagramStackRef) *generated.DiagramStackRef {
	if stack == nil {
		return nil
	}
	out := toGeneratedDiagramStackRef(*stack)
	return &out
}

// toDomainDiagramRef converts a generated.DiagramRef, as already decoded
// from a request body, to its domain shape — the reverse of
// toGeneratedDiagramRef, with the same unreachable-failure reasoning.
func toDomainDiagramRef(ref generated.DiagramRef) domain.DiagramRef {
	data, err := json.Marshal(ref)
	if err != nil {
		panic(err)
	}
	var out domain.DiagramRef
	if err := json.Unmarshal(data, &out); err != nil {
		panic(err)
	}
	return out
}

func toDomainDiagramRefPtr(ref *generated.DiagramRef) *domain.DiagramRef {
	if ref == nil {
		return nil
	}
	out := toDomainDiagramRef(*ref)
	return &out
}

func toDomainDiagramStackRef(stack generated.DiagramStackRef) domain.DiagramStackRef {
	refs := make([]domain.DiagramRef, len(stack.Stack))
	for i, ref := range stack.Stack {
		refs[i] = toDomainDiagramRef(ref)
	}
	return domain.DiagramStackRef{Stack: refs}
}

func toDomainDiagramStackRefPtr(stack *generated.DiagramStackRef) *domain.DiagramStackRef {
	if stack == nil {
		return nil
	}
	out := toDomainDiagramStackRef(*stack)
	return &out
}

func toGeneratedInstrument(i domain.Instrument) generated.Instrument {
	instrument := generated.Instrument{
		InstrumentId: mustUUID(i.ID),
		Name:         i.Name,
		Family:       generated.InstrumentFamily(i.Family),
		StringCount:  i.StringCount,
	}
	if len(i.Tuning) > 0 {
		tuning := i.Tuning
		instrument.Tuning = &tuning
	}
	if i.KeyRange != nil {
		instrument.KeyRange = &struct {
			Highest string `json:"highest"`
			Lowest  string `json:"lowest"`
		}{Highest: i.KeyRange.Highest, Lowest: i.KeyRange.Lowest}
	}
	return instrument
}

func toGeneratedInstruments(instruments []domain.Instrument) []generated.Instrument {
	result := make([]generated.Instrument, len(instruments))
	for i, instrument := range instruments {
		result[i] = toGeneratedInstrument(instrument)
	}
	return result
}

func toGeneratedDiagram(d domain.Diagram) generated.Diagram {
	positions := make([]generated.DiagramPosition, len(d.Positions))
	for i, p := range d.Positions {
		id := mustUUID(p.ID)
		shape := generated.DiagramPositionShape(p.Shape)
		positions[i] = generated.DiagramPosition{
			PositionId:    &id,
			Interval:      p.Interval,
			NoteName:      p.NoteName,
			Shape:         &shape,
			Color:         p.Color,
			SequenceIndex: p.SequenceIndex,
			String:        p.String,
			Fret:          p.Fret,
			Key:           p.Key,
		}
	}
	return generated.Diagram{
		DiagramId:    mustUUID(d.ID),
		InstrumentId: mustUUID(d.InstrumentID),
		Name:         d.Name,
		RootNote:     d.RootNote,
		LabelDisplay: generated.DiagramLabelDisplay(d.LabelDisplay),
		Color:        d.Color,
		Positions:    positions,
		Classification: generated.DiagramClassification{
			Skills:   toGeneratedSkills(d.Skills),
			Concepts: toGeneratedConcepts(d.Concepts),
		},
		CreatedAt: d.CreatedAt,
	}
}

func toGeneratedDiagrams(diagrams []domain.Diagram) []generated.Diagram {
	result := make([]generated.Diagram, len(diagrams))
	for i, d := range diagrams {
		result[i] = toGeneratedDiagram(d)
	}
	return result
}

func toDomainPositions(positions []generated.DiagramPosition) []domain.Position {
	result := make([]domain.Position, len(positions))
	for i, p := range positions {
		result[i] = domain.Position{
			Interval:      p.Interval,
			NoteName:      p.NoteName,
			SequenceIndex: p.SequenceIndex,
			String:        p.String,
			Fret:          p.Fret,
			Key:           p.Key,
			Color:         p.Color,
		}
		if p.Shape != nil {
			result[i].Shape = domain.PositionShape(*p.Shape)
		}
		if p.PositionId != nil {
			result[i].ID = p.PositionId.String()
		}
	}
	return result
}

// toDomainLabelDisplay converts an optional generated label_display enum
// pointer to its domain form, defaulting to the zero value (which
// domain.NewDiagram itself normalizes to LabelDisplayInterval) when absent.
func toDomainLabelDisplay[T ~string](labelDisplay *T) domain.LabelDisplay {
	if labelDisplay == nil {
		return ""
	}
	return domain.LabelDisplay(*labelDisplay)
}

// isStaff reports whether role is teacher or admin — the HTTP layer's own
// copy of the teacher-or-admin check, used only to decide response shape
// (e.g. whether to include has_unpublished_changes on a catalog entry).
// The actual authorization gate for an operation is enforced in the
// application layer, not here.
func isStaff(role domain.Role) bool {
	return role == domain.RoleTeacher || role == domain.RoleAdmin
}

func toCourseCheckpoint(cp domain.CourseCheckpoint) generated.CourseCheckpoint {
	return generated.CourseCheckpoint{
		Position:       cp.Position,
		LearningPathId: mustUUID(cp.LearningPathID),
		Title:          cp.Title,
		EffectiveTitle: cp.EffectiveTitle,
	}
}

func toCourseCheckpoints(checkpoints []domain.CourseCheckpoint) []generated.CourseCheckpoint {
	result := make([]generated.CourseCheckpoint, len(checkpoints))
	for i, cp := range checkpoints {
		result[i] = toCourseCheckpoint(cp)
	}
	return result
}

// toCourse renders c as its live, currently-being-authored draft — the
// teacher/admin-only representation returned by CreateCourse/GetCourse/
// ReplaceCourse. latest is c's latest published CourseVersion, or nil if
// the course has never been published; latest_published_version and
// has_unpublished_changes are both derived from it via
// domain.HasUnpublishedChanges, matching the OpenAPI contract's "or
// nothing has been published yet" case.
func toCourse(c domain.Course, latest *domain.CourseVersion) generated.Course {
	result := generated.Course{
		CourseId:              mustUUID(c.ID),
		Title:                 c.Title,
		Summary:               c.Summary,
		Level:                 generated.CourseLevel(c.Level),
		Status:                generated.CourseStatus(c.Status),
		CreatedBy:             mustUUID(c.CreatedBy),
		CreatedAt:             c.CreatedAt,
		HasUnpublishedChanges: domain.HasUnpublishedChanges(c, latest),
		Checkpoints:           toCourseCheckpoints(c.Checkpoints),
	}
	if latest != nil {
		versionNumber := latest.VersionNumber
		result.LatestPublishedVersion = &versionNumber
	}
	return result
}

// toCourseCatalogEntry renders c as a lightweight catalog entry for the
// given caller — never a checkpoint's learning_path_id or other live-draft
// authoring detail, matching the OpenAPI CourseCatalogEntry contract.
// has_unpublished_changes is included only for teachers/admins; a student
// never receives it. latest is c's latest published CourseVersion, or nil
// if the course has never been published; published_at and
// has_unpublished_changes are both derived from it.
func toCourseCatalogEntry(c domain.Course, caller domain.User, latest *domain.CourseVersion) generated.CourseCatalogEntry {
	entry := generated.CourseCatalogEntry{
		CourseId:  mustUUID(c.ID),
		Title:     c.Title,
		Summary:   c.Summary,
		Level:     generated.CourseCatalogEntryLevel(c.Level),
		CreatedBy: mustUUID(c.CreatedBy),
		Status:    generated.CourseCatalogEntryStatus(c.Status),
	}
	if latest != nil {
		publishedAt := latest.PublishedAt
		entry.PublishedAt = &publishedAt
		// A student only ever sees what the latest published version says,
		// never the live draft's unpublished edits — the same text, level
		// and ordering their search and filters are evaluated against.
		if !isStaff(caller.Role) {
			entry.Title = latest.TitleSnapshot
			entry.Summary = latest.SummarySnapshot
			entry.Level = generated.CourseCatalogEntryLevel(latest.LevelSnapshot)
		}
	}
	if isStaff(caller.Role) {
		hasUnpublishedChanges := domain.HasUnpublishedChanges(c, latest)
		entry.HasUnpublishedChanges = &hasUnpublishedChanges
	}
	return entry
}

// courseVersionLookup returns c's latest published CourseVersion, or nil if
// the course has never been published (domain.ErrNotFound). Any other
// error is returned unchanged as the second value.
type courseVersionLookup func(courseID string) (*domain.CourseVersion, error)

func toCourseCatalogEntries(courses []domain.Course, caller domain.User, latest courseVersionLookup) ([]generated.CourseCatalogEntry, error) {
	result := make([]generated.CourseCatalogEntry, len(courses))
	for i, c := range courses {
		version, err := latest(c.ID)
		if err != nil {
			return nil, err
		}
		result[i] = toCourseCatalogEntry(c, caller, version)
	}
	return result, nil
}

func toGeneratedCourseVersion(v domain.CourseVersion) generated.CourseVersion {
	return generated.CourseVersion{
		CourseId:                   mustUUID(v.CourseID),
		VersionNumber:              v.VersionNumber,
		TitleSnapshot:              v.TitleSnapshot,
		SummarySnapshot:            v.SummarySnapshot,
		LevelSnapshot:              generated.CourseVersionLevelSnapshot(v.LevelSnapshot),
		PublishedAt:                v.PublishedAt,
		AvailableForNewEnrollments: v.AvailableForNewEnrollments,
	}
}

func toCourseOutlineItem(item application.CourseOutlineItem) generated.CourseOutlineItem {
	return generated.CourseOutlineItem{Title: item.Title, SectionLabel: item.SectionLabel}
}

func toCourseOutlineCheckpoint(cp application.CourseOutlineCheckpoint) generated.CourseOutlineCheckpoint {
	items := make([]generated.CourseOutlineItem, len(cp.Items))
	for i, item := range cp.Items {
		items[i] = toCourseOutlineItem(item)
	}
	return generated.CourseOutlineCheckpoint{Position: cp.Position, Title: cp.Title, Items: items}
}

func toCourseDetail(courseID string, view application.PublishedCourseView) generated.CourseDetail {
	checkpoints := make([]generated.CourseOutlineCheckpoint, len(view.Checkpoints))
	for i, cp := range view.Checkpoints {
		checkpoints[i] = toCourseOutlineCheckpoint(cp)
	}
	publishedAt := view.PublishedAt
	return generated.CourseDetail{
		CourseId:    mustUUID(courseID),
		Title:       view.Title,
		Summary:     view.Summary,
		Level:       generated.CourseDetailLevel(view.Level),
		Status:      generated.CourseDetailStatus(view.Status),
		PublishedAt: &publishedAt,
		Checkpoints: checkpoints,
	}
}

// courseListFilter translates GET /courses' query parameters into the
// domain filter. Role-dependent scoping is the application layer's job, not
// this mapping's.
func courseListFilter(params generated.ListCoursesParams) domain.CourseListFilter {
	filter := domain.CourseListFilter{Query: searchQuery(params.Q)}
	if params.Status != nil {
		status := domain.CourseStatus(*params.Status)
		filter.Status = &status
	}
	if params.CreatedBy != nil {
		filter.CreatedBy = params.CreatedBy.String()
	}
	if params.Levels != nil {
		for _, l := range *params.Levels {
			filter.Levels = append(filter.Levels, domain.DifficultyLevel(l))
		}
	}
	if params.SkillIds != nil {
		filter.SkillIDs = uuidStrings(*params.SkillIds)
	}
	if params.ConceptIds != nil {
		filter.ConceptIDs = uuidStrings(*params.ConceptIds)
	}
	return filter
}

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}
