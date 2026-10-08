package http

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"
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

// uuidPtrFromStringPtr is mustUUID for an optional id.
func uuidPtrFromStringPtr(id *string) *uuid.UUID {
	if id == nil {
		return nil
	}
	parsed := mustUUID(*id)
	return &parsed
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
		DisplayName:  u.DisplayName,
		Locale:       toGeneratedLanguage(u.Locale),
		RegisteredAt: u.RegisteredAt,
	}
}

func toGeneratedKnowledgeNode(n domain.KnowledgeNode) generated.KnowledgeNode {
	node := generated.KnowledgeNode{
		NodeId:        mustUUID(n.ID),
		Kind:          generated.KnowledgeNodeKind(n.Kind),
		Key:           n.Key,
		Names:         generated.LocalizedNames(n.Names),
		Languages:     n.Names.Languages(),
		ParentId:      uuidPtrFromStringPtr(n.ParentID),
		InstrumentIds: toUUIDs(n.InstrumentIDs),
	}
	if n.Descriptions != nil {
		descriptions := generated.LocalizedDescription(n.Descriptions)
		node.Descriptions = &descriptions
	}
	return node
}

func toGeneratedKnowledgeNodes(nodes []domain.KnowledgeNode) []generated.KnowledgeNode {
	result := make([]generated.KnowledgeNode, len(nodes))
	for i, n := range nodes {
		result[i] = toGeneratedKnowledgeNode(n)
	}
	return result
}

// toUpdateKnowledgeNodeInput keeps the PATCH body's three states apart: a
// field left out is unchanged, an explicit null clears it.
func toUpdateKnowledgeNodeInput(body generated.UpdateKnowledgeNodeRequest) application.UpdateKnowledgeNodeInput {
	var input application.UpdateKnowledgeNodeInput
	if body.Names != nil {
		input.Names = *body.Names
	}
	if body.Descriptions.IsSpecified() {
		input.Descriptions.Set = true
		if !body.Descriptions.IsNull() {
			descriptions := map[string]string(body.Descriptions.MustGet())
			input.Descriptions.Value = &descriptions
		}
	}
	if body.ParentId.IsSpecified() {
		input.ParentID.Set = true
		if !body.ParentId.IsNull() {
			parentID := body.ParentId.MustGet().String()
			input.ParentID.Value = &parentID
		}
	}
	if body.InstrumentIds != nil {
		ids := uuidsToStrings(*body.InstrumentIds)
		input.InstrumentIDs = &ids
	}
	return input
}

func toGeneratedKnowledgeEdge(e domain.KnowledgeEdge) generated.KnowledgeEdge {
	edge := generated.KnowledgeEdge{
		EdgeId: mustUUID(e.ID),
		FromId: mustUUID(e.FromID),
		ToId:   mustUUID(e.ToID),
		Type:   generated.KnowledgeEdgeType(e.Type),
	}
	if e.Level != nil {
		level := generated.MasteryLevel(*e.Level)
		edge.Level = &level
	}
	return edge
}

func toGeneratedKnowledgeEdges(edges []domain.KnowledgeEdge) []generated.KnowledgeEdge {
	result := make([]generated.KnowledgeEdge, len(edges))
	for i, e := range edges {
		result[i] = toGeneratedKnowledgeEdge(e)
	}
	return result
}

func toContentNodeVersion(v domain.ContentNodeVersion) generated.ContentNodeVersion {
	result := generated.ContentNodeVersion{
		ContentNodeId: mustUUID(v.ContentNodeID),
		VersionNumber: v.VersionNumber,
		TitleSnapshot: v.Title,
		ClassificationSnapshot: generated.Classification{
			Skills:          toGeneratedKnowledgeNodes(v.Classification.Skills),
			Concepts:        toGeneratedKnowledgeNodes(v.Classification.Concepts),
			DifficultyLevel: generated.ClassificationDifficultyLevel(v.Classification.DifficultyLevel),
			ReviewState:     generated.ClassificationReviewState(v.Classification.ReviewState),
		},
		LanguagesSnapshot:     toGeneratedLanguages(v.Languages),
		InstrumentIdsSnapshot: toUUIDs(v.InstrumentIDsSnapshot),
		ThumbnailUrlSnapshot:  v.ThumbnailURLSnapshot,
		MediaUrlSnapshot:      v.MediaURL,
		PublishedAt:           v.PublishedAt,
	}
	if v.RichContent != nil {
		doc := toGeneratedPromptDocument(*v.RichContent)
		result.RichContentSnapshot = &doc
	}
	return result
}

func toContentNode(n domain.ContentNode, names userNames) generated.ContentNode {
	result := generated.ContentNode{
		ContentNodeId: mustUUID(n.ID),
		Teacher:       names.ref(n.TeacherID),
		Title:         n.Title,
		ContentType:   generated.ContentNodeContentType(n.ContentType),
		Classification: generated.Classification{
			Skills:          toGeneratedKnowledgeNodes(n.Classification.Skills),
			Concepts:        toGeneratedKnowledgeNodes(n.Classification.Concepts),
			DifficultyLevel: generated.ClassificationDifficultyLevel(n.Classification.DifficultyLevel),
			ReviewState:     generated.ClassificationReviewState(n.Classification.ReviewState),
		},
		MediaUrl:      n.MediaURL,
		Languages:     toGeneratedLanguages(n.Languages),
		InstrumentIds: toUUIDs(n.InstrumentIDs),
		ThumbnailUrl:  n.ThumbnailURL,
		CreatedAt:     n.CreatedAt,
	}
	if n.RichContent != nil {
		doc := toGeneratedPromptDocument(*n.RichContent)
		result.RichContent = &doc
	}
	return result
}

func toContentNodes(nodes []domain.ContentNode, names userNames) []generated.ContentNode {
	result := make([]generated.ContentNode, 0, len(nodes))
	for _, n := range nodes {
		result = append(result, toContentNode(n, names))
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

func toExercise(e domain.Exercise, names userNames) generated.Exercise {
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
		Skills:                   toGeneratedKnowledgeNodes(e.Skills),
		Concepts:                 toGeneratedKnowledgeNodes(e.Concepts),
		ImageUrl:                 e.ImageURL,
		AudioUrl:                 e.AudioURL,
		Options:                  options,
		ChallengeIds:             challengeIDs,
		ContentNodeIds:           contentNodeIDs,
		Languages:                toGeneratedLanguages(e.Languages),
		InstrumentIds:            toUUIDs(e.InstrumentIDs),
		EstimatedDurationSeconds: e.EstimatedDurationSeconds,
		RemediationTargets:       toRemediationTargets(e.RemediationTargets),
		CreatedAt:                e.CreatedAt,
		DiagramRef:               toGeneratedDiagramRefPtr(e.DiagramRef),
		DiagramStackRef:          toGeneratedDiagramStackRefPtr(e.DiagramStackRef),
	}
	if e.CreatedBy != "" {
		createdBy := names.ref(e.CreatedBy)
		exercise.CreatedBy = &createdBy
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

func toExercises(exercises []domain.Exercise, names userNames) []generated.Exercise {
	result := make([]generated.Exercise, len(exercises))
	for i, e := range exercises {
		result[i] = toExercise(e, names)
	}
	return result
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
	if o.FretCell != nil {
		option.FretCell = &struct {
			Fret   int `json:"fret"`
			String int `json:"string"`
		}{Fret: o.FretCell.Fret, String: o.FretCell.String}
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

func toLearningPath(p domain.LearningPath, names userNames) generated.LearningPath {
	items := make([]generated.LearningPathItem, len(p.Items))
	for i, item := range p.Items {
		items[i] = toLearningPathItem(item)
	}
	result := generated.LearningPath{
		LearningPathId: mustUUID(p.ID),
		Teacher:        names.ref(p.TeacherID),
		Title:          p.Title,
		Summary:        p.Summary,
		Language:       p.Language,
		Status:         generated.LearningPathStatus(p.Status),
		Items:          items,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
		InstrumentIds:  toUUIDs(p.InstrumentIDs),
		ThumbnailUrl:   p.ThumbnailURL,
	}
	if p.Level != nil {
		level := generated.LearningPathLevel(*p.Level)
		result.Level = &level
	}
	return result
}

// exerciseListFilter maps GET /exercises' query parameters onto the domain
// filter.
func exerciseListFilter(params generated.ListExercisesParams) domain.ExerciseFilter {
	filter := domain.ExerciseFilter{
		Query:     searchQuery(params.Q),
		SkillID:   uuidPtrToString(params.SkillId),
		ConceptID: uuidPtrToString(params.ConceptId),
		CreatedBy: uuidPtrToString(params.CreatedBy),
	}
	if params.ExerciseType != nil {
		filter.ExerciseType = domain.ExerciseType(*params.ExerciseType)
	}
	if params.Language != nil {
		filter.Language = *params.Language
	}
	if params.InstrumentId != nil {
		filter.InstrumentIDs = uuidsToStrings(*params.InstrumentId)
	}
	return filter
}

// learningPathListFilter maps GET /learning-paths' query parameters onto the
// domain filter.
func learningPathListFilter(params generated.ListLearningPathsParams) domain.LearningPathFilter {
	filter := domain.LearningPathFilter{Query: searchQuery(params.Q), InstrumentID: uuidPtrToString(params.InstrumentId)}
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
	if params.Sort != nil {
		filter.Sort = domain.LearningPathSort(*params.Sort)
	}
	if params.Language != nil {
		filter.Language = *params.Language
	}
	if params.Status != nil {
		filter.Status = domain.LearningPathStatus(*params.Status)
	}
	return filter
}

// toNotPublishableError is the 409 body listing everything that keeps a path
// from being published.
func toNotPublishableError(refusal *domain.LearningPathNotPublishableError, message string) generated.LearningPathNotPublishableError {
	missing := make([]generated.LearningPathNotPublishableErrorMissing, len(refusal.Missing))
	for i, m := range refusal.Missing {
		missing[i] = generated.LearningPathNotPublishableErrorMissing(m)
	}
	body := generated.LearningPathNotPublishableError{Message: message, Missing: missing}
	if len(refusal.UnpublishedContentNodeIDs) > 0 {
		ids := toUUIDs(refusal.UnpublishedContentNodeIDs)
		body.UnpublishedContentNodeIds = &ids
	}
	return body
}

func toLearningPaths(paths []domain.LearningPath, names userNames) []generated.LearningPath {
	result := make([]generated.LearningPath, 0, len(paths))
	for _, p := range paths {
		result = append(result, toLearningPath(p, names))
	}
	return result
}

// toStudentPath renders sp with the presentation it recorded when copied,
// its lesson count and completed, how many of those lessons its student
// has completed.
func toStudentPath(sp domain.StudentPath, names userNames, completed int) generated.StudentPath {
	result := generated.StudentPath{
		StudentPathId:            mustUUID(sp.ID),
		Student:                  names.ref(sp.StudentID),
		SourceTemplateId:         mustUUID(sp.SourceTemplateID),
		Title:                    sp.Title,
		AssignedBy:               names.ref(sp.AssignedBy),
		AssignedAt:               sp.AssignedAt,
		ArchivedAt:               sp.ArchivedAt,
		CourseCheckpointPosition: sp.CourseCheckpointPosition,
		Summary:                  sp.SummarySnapshot,
		ThumbnailUrl:             sp.ThumbnailURLSnapshot,
		LessonCount:              len(sp.Items),
		CompletedCount:           completed,
	}
	if sp.SourceCourseEnrollmentID != nil {
		id := mustUUID(*sp.SourceCourseEnrollmentID)
		result.SourceCourseEnrollmentId = &id
	}
	if sp.LevelSnapshot != nil {
		level := generated.StudentPathLevel(*sp.LevelSnapshot)
		result.Level = &level
	}
	if sp.CreatedBySnapshot != nil {
		creator := names.ref(*sp.CreatedBySnapshot)
		result.CreatedBy = &creator
	}
	return result
}

func toStudentPathItem(item domain.StudentPathItem) generated.StudentPathItem {
	result := generated.StudentPathItem{
		Position:             item.Position,
		ContentNodeId:        mustUUID(item.ContentNodeID),
		ContentNodeVersionId: mustUUID(item.ContentNodeVersionID),
		Title:                item.Title,
		ContentType:          generated.StudentPathItemContentType(item.ContentType),
		Status:               generated.StudentPathItemStatus(item.Status),
		SectionLabel:         item.SectionLabel,
	}
	if item.LockReason != nil {
		reason := generated.StudentPathItemLockReason(*item.LockReason)
		result.LockReason = &reason
	}
	if item.AvailableLanguages != nil {
		languages := toGeneratedLanguages(item.AvailableLanguages)
		result.AvailableLanguages = &languages
	}
	return result
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

func toCourseEnrollment(e domain.CourseEnrollment, presentation application.CourseEnrollmentPresentation, names userNames) generated.CourseEnrollment {
	result := generated.CourseEnrollment{
		CourseEnrollmentId:       mustUUID(e.ID),
		Student:                  names.ref(e.StudentID),
		CourseId:                 mustUUID(e.CourseID),
		CourseTitle:              e.CourseTitle,
		CourseSummary:            presentation.CourseSummary,
		CourseLevel:              generated.CourseEnrollmentCourseLevel(presentation.CourseLevel),
		CourseCreatedBy:          names.ref(presentation.CourseCreatedBy),
		CheckpointCount:          presentation.CheckpointCount,
		CourseThumbnailUrl:       e.CourseThumbnailURL,
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

func toCourseEnrollments(enrollments []domain.CourseEnrollment, presentations []application.CourseEnrollmentPresentation, names userNames) []generated.CourseEnrollment {
	result := make([]generated.CourseEnrollment, len(enrollments))
	for i, e := range enrollments {
		result[i] = toCourseEnrollment(e, presentations[i], names)
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
		InstrumentId:   mustUUID(i.ID),
		Names:          generated.LocalizedNames(i.Names),
		Languages:      i.Names.Languages(),
		Family:         generated.InstrumentFamily(i.Family),
		StringCount:    i.StringCount,
		DefaultVoiceId: i.DefaultVoiceID,
		Icon:           i.Icon,
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

// toDomainDiagramKind maps an optional create-request kind onto the domain;
// nil maps to the zero value, which domain.NewDiagram defaults to custom.
func toDomainDiagramKind(kind *generated.CreateDiagramRequestKind) domain.DiagramKind {
	if kind == nil {
		return ""
	}
	return domain.DiagramKind(*kind)
}

// diagramListFilter maps GET /diagrams' query parameters onto the domain
// filter. Role scoping (VisibleTo) is the application layer's to set.
// diagramListFilter maps the list parameters onto a DiagramListFilter. A
// name or root_note given but empty is refused here, since the filter can't
// tell an empty search from none.
func diagramListFilter(params generated.ListDiagramsParams) (domain.DiagramListFilter, error) {
	filter := domain.DiagramListFilter{
		InstrumentID: uuidPtrToString(params.InstrumentId),
		SkillID:      uuidPtrToString(params.SkillId),
		ConceptID:    uuidPtrToString(params.ConceptId),
		CreatedBy:    uuidPtrToString(params.CreatedBy),
	}
	if params.Kind != nil {
		filter.Kind = domain.DiagramKind(*params.Kind)
	}
	if params.Purpose != nil {
		filter.Purpose = domain.DiagramPurposeFilter(*params.Purpose)
	}
	if params.Language != nil {
		filter.Language = *params.Language
	}
	if params.Name != nil {
		if *params.Name == "" {
			return domain.DiagramListFilter{}, domain.NewValidationError("name", "must not be empty")
		}
		filter.Name = *params.Name
	}
	if params.RootNote != nil {
		if *params.RootNote == "" {
			return domain.DiagramListFilter{}, domain.NewValidationError("root_note", "must not be empty")
		}
		filter.RootNote = *params.RootNote
	}
	return filter, nil
}

func toGeneratedDiagram(d domain.Diagram, names userNames) generated.Diagram {
	instrumentIDs := d.InstrumentIDs
	if len(instrumentIDs) == 0 && d.InstrumentID != "" {
		instrumentIDs = []string{d.InstrumentID}
	}
	generatedInstrumentIDs := make([]openapi_types.UUID, len(instrumentIDs))
	for i, id := range instrumentIDs {
		generatedInstrumentIDs[i] = mustUUID(id)
	}
	positions := make([]generated.DiagramPosition, len(d.Positions))
	for i, p := range d.Positions {
		id := mustUUID(p.ID)
		shape := generated.DiagramPositionShape(p.Shape)
		positions[i] = generated.DiagramPosition{
			PositionId: &id,
			Interval:   generated.DiagramPositionInterval(p.Interval),
			NoteName:   p.NoteName,
			Shape:      &shape,
			Color:      p.Color,
			String:     p.String,
			Fret:       p.Fret,
			Key:        p.Key,
		}
		if p.CustomLabel != nil {
			label := generated.LocalizedMarkerLabel(p.CustomLabel)
			positions[i].CustomLabel = &label
		}
		if p.Note != nil {
			note := generated.LocalizedNote(p.Note)
			positions[i].Note = &note
		}
	}
	return generated.Diagram{
		DiagramId:     mustUUID(d.ID),
		InstrumentId:  mustUUID(d.InstrumentID),
		InstrumentIds: generatedInstrumentIDs,
		Names:         generated.LocalizedNames(d.Names),
		Languages:     d.Names.Languages(),
		Kind:          generated.DiagramKind(d.Kind),
		Purpose:       generated.DiagramPurpose(d.Purpose),
		CreatedBy:     names.ref(d.CreatedBy),
		RootNote:      d.RootNote,
		LabelDisplay:  generated.DiagramLabelDisplay(d.LabelDisplay),
		Color:         d.Color,
		Positions:     positions,
		Regions:       toGeneratedRegions(d.Regions),
		Classification: generated.DiagramClassification{
			Skills:   toGeneratedKnowledgeNodes(d.Skills),
			Concepts: toGeneratedKnowledgeNodes(d.Concepts),
		},
		Mode:              toGeneratedMode(d.Mode),
		Playbacks:         toGeneratedPlaybacks(d.Playbacks),
		DefaultPlaybackId: uuidPtrFromStringPtr(d.DefaultPlaybackID),
		CreatedAt:         d.CreatedAt,
	}
}

// toDiagramUpdate converts an update request's body: every field it leaves
// out keeps the diagram's current value.
func toDiagramUpdate(body *generated.UpdateDiagramRequest) application.DiagramUpdate {
	update := application.DiagramUpdate{
		RootNote: body.RootNote, Color: body.Color, Regions: toDomainRegions(body.Regions),
		Mode:              toUpdateNullable(body.Mode, func(m generated.DiagramMode) domain.DiagramMode { return domain.DiagramMode(m) }),
		Playbacks:         toDomainPlaybacks(body.Playbacks),
		DefaultPlaybackID: toUpdateNullable(body.DefaultPlaybackId, func(id openapi_types.UUID) string { return id.String() }),
	}
	if body.Names != nil {
		update.Names = *body.Names
	}
	if body.Positions != nil {
		update.Positions = toDomainPositions(*body.Positions)
	}
	if body.Classification != nil {
		update.SkillIDs = uuidsToStrings(body.Classification.SkillIds)
		update.ConceptIDs = uuidsToStrings(body.Classification.ConceptIds)
	}
	if body.LabelDisplay != nil {
		labelDisplay := domain.LabelDisplay(*body.LabelDisplay)
		update.LabelDisplay = &labelDisplay
	}
	if body.InstrumentIds != nil {
		update.InstrumentIDs = uuidsToStrings(*body.InstrumentIds)
	}
	return update
}

// toUpdateNullable converts a request field that may be left out, null or
// set, keeping which of the three it is.
func toUpdateNullable[T, D comparable](field nullable.Nullable[T], convert func(T) D) application.Nullable[D] {
	switch {
	case !field.IsSpecified():
		return application.Nullable[D]{}
	case field.IsNull():
		return application.Nullable[D]{Set: true}
	default:
		value := convert(field.MustGet())
		return application.Nullable[D]{Set: true, Value: &value}
	}
}

func toGeneratedMode(mode *domain.DiagramMode) *generated.DiagramMode {
	if mode == nil {
		return nil
	}
	m := generated.DiagramMode(*mode)
	return &m
}

func toDomainMode(mode *generated.DiagramMode) *domain.DiagramMode {
	if mode == nil {
		return nil
	}
	m := domain.DiagramMode(*mode)
	return &m
}

// toGeneratedPlaybacks renders playbacks, always as a list: a diagram that
// doesn't play has an empty list, never a null one.
func toGeneratedPlaybacks(playbacks []domain.DiagramPlayback) []generated.DiagramPlayback {
	out := make([]generated.DiagramPlayback, len(playbacks))
	for i, p := range playbacks {
		out[i] = generated.DiagramPlayback{
			PlaybackId:    mustUUID(p.ID),
			Names:         generated.LocalizedNames(p.Names),
			TempoBpm:      p.TempoBPM,
			TimeSignature: generated.TimeSignature{Beats: p.TimeSignature.Beats, BeatValue: generated.TimeSignatureBeatValue(p.TimeSignature.BeatValue)},
			Steps:         toGeneratedSequence(p.Steps),
		}
	}
	return out
}

// toDomainPlaybacks converts a request's playbacks, keeping omitted
// playbacks nil and an empty list empty — they mean different things on an
// update. A playback without an id is left for the service to assign one,
// and one without a time signature for the domain to default.
func toDomainPlaybacks(playbacks *[]generated.DiagramPlaybackInput) []domain.DiagramPlayback {
	if playbacks == nil {
		return nil
	}
	out := make([]domain.DiagramPlayback, len(*playbacks))
	for i, p := range *playbacks {
		out[i] = domain.DiagramPlayback{
			Names:         domain.LocalizedText(p.Names),
			TempoBPM:      p.TempoBpm,
			TimeSignature: toDomainTimeSignature(p.TimeSignature),
			Steps:         toDomainSequence(&p.Steps),
		}
		if p.PlaybackId != nil {
			out[i].ID = p.PlaybackId.String()
		}
	}
	return out
}

// toGeneratedSequence renders steps, always as a list.
func toGeneratedSequence(steps []domain.SequenceStep) []generated.SequenceStep {
	out := make([]generated.SequenceStep, len(steps))
	for i, s := range steps {
		ids := make([]uuid.UUID, len(s.PositionIDs))
		for j, id := range s.PositionIDs {
			ids[j] = mustUUID(id)
		}
		strum := generated.SequenceStepStrum(s.Strum)
		out[i] = generated.SequenceStep{PositionIds: ids, Value: generated.NoteValue{Num: s.Value.Num, Den: s.Value.Den}, Strum: &strum}
	}
	return out
}

// toDomainSequence converts a request's steps, keeping omitted steps nil and
// an empty list empty. A step without a strum is left for the domain to
// default.
func toDomainSequence(steps *[]generated.SequenceStep) []domain.SequenceStep {
	if steps == nil {
		return nil
	}
	out := make([]domain.SequenceStep, len(*steps))
	for i, s := range *steps {
		ids := make([]string, len(s.PositionIds))
		for j, id := range s.PositionIds {
			ids[j] = id.String()
		}
		out[i] = domain.SequenceStep{PositionIDs: ids, Value: domain.NoteValue{Num: s.Value.Num, Den: s.Value.Den}}
		if s.Strum != nil {
			out[i].Strum = domain.Strum(*s.Strum)
		}
	}
	return out
}

func toDomainTimeSignature(signature *generated.TimeSignature) domain.TimeSignature {
	if signature == nil {
		return domain.TimeSignature{}
	}
	return domain.TimeSignature{Beats: signature.Beats, BeatValue: int(signature.BeatValue)}
}

func toGeneratedVoices(voices []application.PlayableVoice) []generated.Voice {
	out := make([]generated.Voice, len(voices))
	for i, v := range voices {
		samples := make([]generated.VoiceSample, len(v.Samples))
		for j, sample := range v.Samples {
			samples[j] = generated.VoiceSample{Pitch: sample.Pitch, Url: sample.URL}
		}
		out[i] = generated.Voice{
			VoiceId:     v.Voice.ID,
			Names:       generated.LocalizedNames(v.Voice.Names),
			Languages:   v.Voice.Names.Languages(),
			Family:      generated.VoiceFamily(v.Voice.Family),
			Samples:     samples,
			Attribution: v.Voice.Attribution,
		}
	}
	return out
}

func toGeneratedDiagrams(diagrams []domain.Diagram, names userNames) []generated.Diagram {
	result := make([]generated.Diagram, len(diagrams))
	for i, d := range diagrams {
		result[i] = toGeneratedDiagram(d, names)
	}
	return result
}

func toDomainPositions(positions []generated.DiagramPosition) []domain.Position {
	result := make([]domain.Position, len(positions))
	for i, p := range positions {
		result[i] = domain.Position{
			Interval: string(p.Interval),
			NoteName: p.NoteName,
			String:   p.String,
			Fret:     p.Fret,
			Key:      p.Key,
			Color:    p.Color,
		}
		if p.Shape != nil {
			result[i].Shape = domain.PositionShape(*p.Shape)
		}
		if p.CustomLabel != nil {
			result[i].CustomLabel = domain.LocalizedText(*p.CustomLabel)
		}
		if p.Note != nil {
			result[i].Note = domain.LocalizedText(*p.Note)
		}
		if p.PositionId != nil {
			result[i].ID = p.PositionId.String()
		}
	}
	return result
}

// toGeneratedRegions converts regions to their wire shape — always a list,
// empty when there are none, since the response requires the field.
func toGeneratedRegions(regions []domain.Region) []generated.DiagramRegion {
	result := make([]generated.DiagramRegion, len(regions))
	for i, r := range regions {
		id := mustUUID(r.ID)
		result[i] = generated.DiagramRegion{
			RegionId:    &id,
			FretStart:   r.FretStart,
			FretEnd:     r.FretEnd,
			StringStart: r.StringStart,
			StringEnd:   r.StringEnd,
			KeyStart:    r.KeyStart,
			KeyEnd:      r.KeyEnd,
			Description: generated.LocalizedCaption(r.Description),
			Color:       r.Color,
		}
	}
	return result
}

// toDomainRegions converts a request's optional region list. An omitted
// list is nil and an empty one stays empty, so an update can tell "keep the
// regions" from "remove them all".
func toDomainRegions(regions *[]generated.DiagramRegion) []domain.Region {
	if regions == nil {
		return nil
	}
	result := make([]domain.Region, len(*regions))
	for i, r := range *regions {
		result[i] = domain.Region{
			FretStart:   r.FretStart,
			FretEnd:     r.FretEnd,
			StringStart: r.StringStart,
			StringEnd:   r.StringEnd,
			KeyStart:    r.KeyStart,
			KeyEnd:      r.KeyEnd,
			Description: domain.LocalizedText(r.Description),
			Color:       r.Color,
		}
		if r.RegionId != nil {
			result[i].ID = r.RegionId.String()
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
func toCourse(c domain.Course, latest *domain.CourseVersion, names userNames) generated.Course {
	result := generated.Course{
		CourseId:              mustUUID(c.ID),
		Title:                 c.Title,
		Summary:               c.Summary,
		Level:                 generated.CourseLevel(c.Level),
		Language:              c.Language,
		InstrumentIds:         toUUIDs(c.InstrumentIDs),
		ThumbnailUrl:          c.ThumbnailURL,
		Status:                generated.CourseStatus(c.Status),
		CreatedBy:             names.ref(c.CreatedBy),
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

// catalogEntryView is which list a CourseCatalogEntry is rendered for.
type catalogEntryView int

const (
	// learnerCatalogView is the published catalog every user browses: the
	// latest published version's text, never draft edits or authoring
	// annotations.
	learnerCatalogView catalogEntryView = iota
	// authoringListView is a teacher's or admin's course list: the live
	// draft, annotated with whether it has unpublished changes.
	authoringListView
)

// toCourseCatalogEntry renders c as a lightweight catalog entry for view —
// never a checkpoint's learning_path_id or other live-draft authoring
// detail, matching the OpenAPI CourseCatalogEntry contract. latest is c's
// latest published CourseVersion, or nil if the course has never been
// published; published_at and has_unpublished_changes are both derived
// from it.
func toCourseCatalogEntry(c domain.Course, view catalogEntryView, latest *domain.CourseVersion, names userNames) generated.CourseCatalogEntry {
	entry := generated.CourseCatalogEntry{
		CourseId:        mustUUID(c.ID),
		Title:           c.Title,
		Summary:         c.Summary,
		Level:           generated.CourseCatalogEntryLevel(c.Level),
		Language:        c.Language,
		InstrumentIds:   toUUIDs(c.InstrumentIDs),
		ThumbnailUrl:    c.ThumbnailURL,
		CreatedBy:       names.ref(c.CreatedBy),
		Status:          generated.CourseCatalogEntryStatus(c.Status),
		CheckpointCount: len(c.Checkpoints),
	}
	if latest != nil {
		publishedAt := latest.PublishedAt
		entry.PublishedAt = &publishedAt
		// A learner only ever sees what the latest published version says,
		// never the live draft's unpublished edits — the same text, level
		// and ordering their search and filters are evaluated against.
		if view == learnerCatalogView {
			entry.Title = latest.TitleSnapshot
			entry.Summary = latest.SummarySnapshot
			entry.Level = generated.CourseCatalogEntryLevel(latest.LevelSnapshot)
			entry.Language = latest.LanguageSnapshot
			entry.InstrumentIds = toUUIDs(latest.InstrumentIDsSnapshot)
			entry.ThumbnailUrl = latest.ThumbnailURLSnapshot
			entry.CheckpointCount = len(latest.Checkpoints)
		}
	}
	if view == authoringListView {
		hasUnpublishedChanges := domain.HasUnpublishedChanges(c, latest)
		entry.HasUnpublishedChanges = &hasUnpublishedChanges
	}
	return entry
}

// courseVersionLookup returns c's latest published CourseVersion, or nil if
// the course has never been published (domain.ErrNotFound). Any other
// error is returned unchanged as the second value.
type courseVersionLookup func(courseID string) (*domain.CourseVersion, error)

func courseLearningPathIDs(checkpoints []domain.CourseCheckpoint) []string {
	ids := make([]string, len(checkpoints))
	for i, checkpoint := range checkpoints {
		ids[i] = checkpoint.LearningPathID
	}
	return ids
}

func courseVersionLearningPathIDs(checkpoints []domain.CourseVersionCheckpoint) []string {
	ids := make([]string, len(checkpoints))
	for i, checkpoint := range checkpoints {
		ids[i] = checkpoint.LearningPathID
	}
	return ids
}

func toCourseCatalogEntries(courses []domain.Course, view catalogEntryView, latest courseVersionLookup, names userNames) ([]generated.CourseCatalogEntry, error) {
	result := make([]generated.CourseCatalogEntry, len(courses))
	for i, c := range courses {
		version, err := latest(c.ID)
		if err != nil {
			return nil, err
		}
		result[i] = toCourseCatalogEntry(c, view, version, names)
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
		LanguageSnapshot:           v.LanguageSnapshot,
		InstrumentIdsSnapshot:      toUUIDs(v.InstrumentIDsSnapshot),
		ThumbnailUrlSnapshot:       v.ThumbnailURLSnapshot,
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

func toCourseDetail(courseID string, view application.PublishedCourseView, names userNames) generated.CourseDetail {
	checkpoints := make([]generated.CourseOutlineCheckpoint, len(view.Checkpoints))
	for i, cp := range view.Checkpoints {
		checkpoints[i] = toCourseOutlineCheckpoint(cp)
	}
	publishedAt := view.PublishedAt
	return generated.CourseDetail{
		CourseId:        mustUUID(courseID),
		Title:           view.Title,
		Summary:         view.Summary,
		Level:           generated.CourseDetailLevel(view.Level),
		Language:        view.Language,
		CreatedBy:       names.ref(view.CreatedBy),
		InstrumentIds:   toUUIDs(view.InstrumentIDs),
		ThumbnailUrl:    view.ThumbnailURL,
		Status:          generated.CourseDetailStatus(view.Status),
		PublishedAt:     &publishedAt,
		Checkpoints:     checkpoints,
		CheckpointCount: view.CheckpointCount,
		LessonCount:     view.LessonCount,
	}
}

// courseListFilter translates GET /courses' query parameters into the
// domain filter. Role-dependent scoping is the application layer's job, not
// this mapping's.
func courseListFilter(params generated.ListCoursesParams) domain.CourseListFilter {
	filter := domain.CourseListFilter{Query: searchQuery(params.Q), InstrumentID: uuidPtrToString(params.InstrumentId)}
	if params.Status != nil {
		status := domain.CourseStatus(*params.Status)
		filter.Status = &status
	}
	if params.Language != nil {
		filter.Language = *params.Language
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

// catalogCourseListFilter maps GET /catalog/courses' parameters onto the
// same filter the authoring list uses; the catalog has no status parameter.
func catalogCourseListFilter(params generated.ListCatalogCoursesParams) domain.CourseListFilter {
	filter := domain.CourseListFilter{Query: searchQuery(params.Q), InstrumentID: uuidPtrToString(params.InstrumentId)}
	if params.Language != nil {
		filter.Language = *params.Language
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

// toUUIDs converts ids to their wire form, always a list (empty when there
// are none), since every instrument_ids field is required.
func toUUIDs(ids []string) []uuid.UUID {
	result := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		result[i] = mustUUID(id)
	}
	return result
}
