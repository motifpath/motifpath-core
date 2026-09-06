package http

import (
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
		ChallengeId:   mustUUID(c.ID),
		ContentNodeId: mustUUID(c.ContentNodeID),
		SubjectTag:    c.SubjectTag,
		PassThreshold: c.PassThreshold,
		CreatedAt:     c.CreatedAt,
	}
	if c.RemediationTargetContentNodeID != nil {
		target := mustUUID(*c.RemediationTargetContentNodeID)
		challenge.RemediationTargetContentNodeId = &target
	}
	return challenge
}

func toExercise(e domain.Exercise) generated.Exercise {
	return generated.Exercise{
		ExerciseId:   mustUUID(e.ID),
		ChallengeId:  mustUUID(e.ChallengeID),
		ExerciseType: generated.ExerciseExerciseType(e.ExerciseType),
		Prompt:       e.Prompt,
		CreatedAt:    e.CreatedAt,
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

// optionalLabel maps an empty section label to nil so the field is omitted
// from the JSON response rather than serialised as "".
func optionalLabel(label string) *string {
	if label == "" {
		return nil
	}
	return &label
}

func toLearningPathItem(item domain.LearningPathItem) generated.LearningPathItem {
	return generated.LearningPathItem{
		Position:      item.Position,
		ContentNodeId: mustUUID(item.ContentNodeID),
		Title:         item.Title,
		ContentType:   generated.LearningPathItemContentType(item.ContentType),
		SectionLabel:  optionalLabel(item.SectionLabel),
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
		SectionLabel:  optionalLabel(item.SectionLabel),
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
