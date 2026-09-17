package http

import (
	"encoding/json"

	"github.com/google/uuid"

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

func toUserProfile(u domain.User) generated.UserProfile {
	return generated.UserProfile{
		UserId:       mustUUID(u.ID),
		Role:         generated.UserProfileRole(u.Role),
		RegisteredAt: u.RegisteredAt,
	}
}

func toContentNode(n domain.ContentNode) generated.ContentNode {
	return generated.ContentNode{
		ContentNodeId: mustUUID(n.ID),
		TeacherId:     mustUUID(n.TeacherID),
		Title:         n.Title,
		ContentType:   generated.ContentNodeContentType(n.ContentType),
		Classification: generated.Classification{
			Skill:           n.Classification.Skill,
			Concept:         n.Classification.Concept,
			DifficultyLevel: generated.ClassificationDifficultyLevel(n.Classification.DifficultyLevel),
			ReviewState:     generated.ClassificationReviewState(n.Classification.ReviewState),
		},
		CreatedAt: n.CreatedAt,
	}
}

func toChallenge(c domain.Challenge) generated.Challenge {
	challenge := generated.Challenge{
		ChallengeId:      mustUUID(c.ID),
		ContentNodeId:    mustUUID(c.ContentNodeID),
		SubjectTag:       c.SubjectTag,
		PassThreshold:    c.PassThreshold,
		ShuffleExercises: c.ShuffleExercises,
		ShuffleOptions:   c.ShuffleOptions,
		CreatedAt:        c.CreatedAt,
	}
	if c.RemediationTargetContentNodeID != nil {
		target := mustUUID(*c.RemediationTargetContentNodeID)
		challenge.RemediationTargetContentNodeId = &target
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
		ImageUrl:                 e.ImageURL,
		AudioUrl:                 e.AudioURL,
		Options:                  options,
		ChallengeIds:             challengeIDs,
		ContentNodeIds:           contentNodeIDs,
		EstimatedDurationSeconds: e.EstimatedDurationSeconds,
		CreatedAt:                e.CreatedAt,
	}
	if len(e.SkillTags) > 0 {
		exercise.SkillTags = &e.SkillTags
	}
	return exercise
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
		SkillTag:          session.SkillTag,
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
	return generated.ExpandedContent{
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
	}
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

func toPathAssignment(a domain.PathAssignment) generated.PathAssignment {
	return generated.PathAssignment{
		AssignmentId:   mustUUID(a.ID),
		StudentId:      mustUUID(a.StudentID),
		LearningPathId: mustUUID(a.LearningPathID),
		AssignedBy:     mustUUID(a.AssignedBy),
		AssignedAt:     a.AssignedAt,
	}
}

func toStudentPathItem(item domain.StudentPathItem) generated.StudentPathItem {
	return generated.StudentPathItem{
		Position:      item.Position,
		ContentNodeId: mustUUID(item.ContentNodeID),
		Title:         item.Title,
		ContentType:   generated.StudentPathItemContentType(item.ContentType),
		Status:        generated.StudentPathItemStatus(item.Status),
		SectionLabel:  item.SectionLabel,
	}
}

func toStudentPathView(v application.StudentPathView) generated.StudentPathView {
	items := make([]generated.StudentPathItem, len(v.Items))
	for i, item := range v.Items {
		items[i] = toStudentPathItem(item)
	}
	return generated.StudentPathView{
		AssignmentId:    mustUUID(v.AssignmentID),
		LearningPathId:  mustUUID(v.LearningPathID),
		Title:           v.Title,
		CurrentPosition: v.CurrentPosition,
		Items:           items,
	}
}
