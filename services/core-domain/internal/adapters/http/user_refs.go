package http

import (
	"context"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// userNames holds the current display name of every user a response refers
// to, keyed by user_id. A handler loads it once per response, however many
// entities point at users, so a list page costs one name lookup, not one
// per row.
type userNames map[string]string

// ref renders id as the UserRef every response uses to point at a user.
func (n userNames) ref(id string) generated.UserRef {
	return generated.UserRef{UserId: mustUUID(id), DisplayName: n[id]}
}

// loadUserNames looks up the names of every user in ids. A reference to a
// user that doesn't exist is an error: a response never names a user it
// can't identify.
func (h *Handler) loadUserNames(ctx context.Context, ids []string) (userNames, error) {
	names, err := h.identity.DisplayNames(ctx, ids)
	if err != nil {
		return nil, err
	}
	return userNames(names), nil
}

func contentNodeUserIDs(nodes ...domain.ContentNode) []string {
	ids := make([]string, len(nodes))
	for i, n := range nodes {
		ids[i] = n.TeacherID
	}
	return ids
}

func learningPathUserIDs(paths ...domain.LearningPath) []string {
	ids := make([]string, len(paths))
	for i, p := range paths {
		ids[i] = p.TeacherID
	}
	return ids
}

func studentPathUserIDs(paths ...domain.StudentPath) []string {
	ids := make([]string, 0, 2*len(paths))
	for _, sp := range paths {
		ids = append(ids, sp.StudentID, sp.AssignedBy)
	}
	return ids
}

func courseEnrollmentUserIDs(enrollments ...domain.CourseEnrollment) []string {
	ids := make([]string, len(enrollments))
	for i, e := range enrollments {
		ids[i] = e.StudentID
	}
	return ids
}

func diagramUserIDs(diagrams ...domain.Diagram) []string {
	ids := make([]string, len(diagrams))
	for i, d := range diagrams {
		ids[i] = d.CreatedBy
	}
	return ids
}

func courseUserIDs(courses ...domain.Course) []string {
	ids := make([]string, len(courses))
	for i, c := range courses {
		ids[i] = c.CreatedBy
	}
	return ids
}
