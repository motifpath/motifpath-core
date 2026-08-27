package http

import (
	"context"
	"errors"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// Handler implements generated.StrictServerInterface — one method per
// OpenAPI operation, each translating between generated wire types and the
// application layer.
type Handler struct {
	identity   *application.IdentityService
	content    *application.ContentService
	challenge  *application.ChallengeService
	path       *application.LearningPathService
	assignment *application.PathAssignmentService
}

var _ generated.StrictServerInterface = (*Handler)(nil)

func NewHandler(
	identity *application.IdentityService,
	content *application.ContentService,
	challenge *application.ChallengeService,
	path *application.LearningPathService,
	assignment *application.PathAssignmentService,
) *Handler {
	return &Handler{identity: identity, content: content, challenge: challenge, path: path, assignment: assignment}
}

// resolveCaller returns the authenticated caller's User record, or ok=false
// if the request must be rejected with 401 — either because no Clerk
// identity is present in the context (missing/invalid bearer token) or
// because that identity has never registered via POST /users. The two
// cases are deliberately indistinguishable to the client: a valid-but-
// unregistered Clerk session isn't a recognised MotifPath caller for any
// endpoint except registration and profile retrieval, which resolve the
// identity themselves and report 404 instead (see RegisterUser,
// GetMyProfile below).
func (h *Handler) resolveCaller(ctx context.Context) (domain.User, bool) {
	clerkUserID, ok := ClerkUserIDFromContext(ctx)
	if !ok {
		return domain.User{}, false
	}
	user, err := h.identity.GetProfile(ctx, clerkUserID)
	if err != nil {
		return domain.User{}, false
	}
	return user, true
}

func (h *Handler) RegisterUser(ctx context.Context, request generated.RegisterUserRequestObject) (generated.RegisterUserResponseObject, error) {
	clerkUserID, ok := ClerkUserIDFromContext(ctx)
	if !ok {
		return generated.RegisterUser401JSONResponse(unauthorizedError()), nil
	}

	user, err := h.identity.RegisterUser(ctx, clerkUserID, domain.Role(request.Body.Role))
	if err != nil {
		kind, valErr := classify(err)
		switch {
		case kind == errKindValidation:
			return generated.RegisterUser400JSONResponse(validationErrorResponse(valErr)), nil
		case errors.Is(err, domain.ErrAlreadyExists):
			return generated.RegisterUser409JSONResponse(conflictError("a user record already exists for this Clerk identity")), nil
		default:
			return nil, err
		}
	}

	return generated.RegisterUser201JSONResponse(toUserProfile(user)), nil
}

func (h *Handler) GetMyProfile(ctx context.Context, _ generated.GetMyProfileRequestObject) (generated.GetMyProfileResponseObject, error) {
	clerkUserID, ok := ClerkUserIDFromContext(ctx)
	if !ok {
		return generated.GetMyProfile401JSONResponse(unauthorizedError()), nil
	}

	user, err := h.identity.GetProfile(ctx, clerkUserID)
	if err != nil {
		if kind, _ := classify(err); kind == errKindNotFound {
			return generated.GetMyProfile404JSONResponse(notFoundError("no user record exists for this Clerk identity")), nil
		}
		return nil, err
	}

	return generated.GetMyProfile200JSONResponse(toUserProfile(user)), nil
}

func (h *Handler) CreateContentNode(ctx context.Context, request generated.CreateContentNodeRequestObject) (generated.CreateContentNodeResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.CreateContentNode401JSONResponse(unauthorizedError()), nil
	}

	body := request.Body
	node, err := h.content.CreateContentNode(ctx, caller, body.Title,
		domain.ContentType(body.ContentType),
		body.Classification.Skill, body.Classification.Concept,
		domain.DifficultyLevel(body.Classification.DifficultyLevel))
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.CreateContentNode400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindForbidden:
			return generated.CreateContentNode403JSONResponse(forbiddenError("only teachers and admins may create content nodes")), nil
		case errKindNotFound, errKindOther:
			return nil, err
		}
	}

	return generated.CreateContentNode201JSONResponse(toContentNode(node)), nil
}

func (h *Handler) GetContentNode(ctx context.Context, request generated.GetContentNodeRequestObject) (generated.GetContentNodeResponseObject, error) {
	if _, ok := h.resolveCaller(ctx); !ok {
		return generated.GetContentNode401JSONResponse(unauthorizedError()), nil
	}

	node, err := h.content.GetContentNode(ctx, request.ContentNodeId.String())
	if err != nil {
		if kind, _ := classify(err); kind == errKindNotFound {
			return generated.GetContentNode404JSONResponse(notFoundError("no content node exists with the given content_node_id")), nil
		}
		return nil, err
	}

	return generated.GetContentNode200JSONResponse(toContentNode(node)), nil
}

func (h *Handler) CreateChallenge(ctx context.Context, request generated.CreateChallengeRequestObject) (generated.CreateChallengeResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.CreateChallenge401JSONResponse(unauthorizedError()), nil
	}

	var remediationTarget *string
	if request.Body.RemediationTargetContentNodeId != nil {
		s := request.Body.RemediationTargetContentNodeId.String()
		remediationTarget = &s
	}

	challenge, err := h.challenge.CreateChallenge(ctx, caller, request.ContentNodeId.String(),
		request.Body.SubjectTag, request.Body.PassThreshold, remediationTarget)
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.CreateChallenge400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindForbidden:
			return generated.CreateChallenge403JSONResponse(forbiddenError("only teachers and admins may create challenges")), nil
		case errKindNotFound:
			return generated.CreateChallenge404JSONResponse(notFoundError("no content node exists with the given content_node_id")), nil
		case errKindOther:
			return nil, err
		}
	}

	return generated.CreateChallenge201JSONResponse(toChallenge(challenge)), nil
}

func (h *Handler) GetChallenge(ctx context.Context, request generated.GetChallengeRequestObject) (generated.GetChallengeResponseObject, error) {
	if _, ok := h.resolveCaller(ctx); !ok {
		return generated.GetChallenge401JSONResponse(unauthorizedError()), nil
	}

	challenge, err := h.challenge.GetChallenge(ctx, request.ChallengeId.String())
	if err != nil {
		if kind, _ := classify(err); kind == errKindNotFound {
			return generated.GetChallenge404JSONResponse(notFoundError("no challenge exists with the given id")), nil
		}
		return nil, err
	}

	return generated.GetChallenge200JSONResponse(toChallenge(challenge)), nil
}

func (h *Handler) CreateExercise(ctx context.Context, request generated.CreateExerciseRequestObject) (generated.CreateExerciseResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.CreateExercise401JSONResponse(unauthorizedError()), nil
	}

	exercise, err := h.challenge.CreateExercise(ctx, caller, request.ChallengeId.String(),
		domain.ExerciseType(request.Body.ExerciseType), request.Body.Prompt)
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.CreateExercise400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindForbidden:
			return generated.CreateExercise403JSONResponse(forbiddenError("only teachers and admins may create exercises")), nil
		case errKindNotFound:
			return generated.CreateExercise404JSONResponse(notFoundError("no challenge exists with the given challenge_id")), nil
		case errKindOther:
			return nil, err
		}
	}

	return generated.CreateExercise201JSONResponse(toExercise(exercise)), nil
}

func (h *Handler) GetExercise(ctx context.Context, request generated.GetExerciseRequestObject) (generated.GetExerciseResponseObject, error) {
	if _, ok := h.resolveCaller(ctx); !ok {
		return generated.GetExercise401JSONResponse(unauthorizedError()), nil
	}

	exercise, err := h.challenge.GetExercise(ctx, request.ExerciseId.String())
	if err != nil {
		if kind, _ := classify(err); kind == errKindNotFound {
			return generated.GetExercise404JSONResponse(notFoundError("no exercise exists with the given id")), nil
		}
		return nil, err
	}

	return generated.GetExercise200JSONResponse(toExercise(exercise)), nil
}

func (h *Handler) CreateExpandedContent(ctx context.Context, request generated.CreateExpandedContentRequestObject) (generated.CreateExpandedContentResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.CreateExpandedContent401JSONResponse(unauthorizedError()), nil
	}

	body := request.Body
	item, err := h.content.CreateExpandedContent(ctx, caller, request.ContentNodeId.String(),
		domain.ExpandedContentType(body.ContentType), body.MediaUrl,
		body.TriggerAtSeconds, body.HideAtSeconds, body.TriggerAtParagraph, body.DurationMs, body.Caption)
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.CreateExpandedContent400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindForbidden:
			return generated.CreateExpandedContent403JSONResponse(forbiddenError("only teachers and admins may add expanded content")), nil
		case errKindNotFound:
			return generated.CreateExpandedContent404JSONResponse(notFoundError("no content node exists with the given content_node_id")), nil
		case errKindOther:
			return nil, err
		}
	}

	return generated.CreateExpandedContent201JSONResponse(toExpandedContent(item)), nil
}

func (h *Handler) ListExpandedContent(ctx context.Context, request generated.ListExpandedContentRequestObject) (generated.ListExpandedContentResponseObject, error) {
	if _, ok := h.resolveCaller(ctx); !ok {
		return generated.ListExpandedContent401JSONResponse(unauthorizedError()), nil
	}

	items, err := h.content.ListExpandedContent(ctx, request.ContentNodeId.String())
	if err != nil {
		if kind, _ := classify(err); kind == errKindNotFound {
			return generated.ListExpandedContent404JSONResponse(notFoundError("no content node exists with the given content_node_id")), nil
		}
		return nil, err
	}

	generatedItems := make([]generated.ExpandedContent, len(items))
	for i, item := range items {
		generatedItems[i] = toExpandedContent(item)
	}
	return generated.ListExpandedContent200JSONResponse{Items: generatedItems, Total: len(generatedItems)}, nil
}

func (h *Handler) GetExpandedContent(ctx context.Context, request generated.GetExpandedContentRequestObject) (generated.GetExpandedContentResponseObject, error) {
	if _, ok := h.resolveCaller(ctx); !ok {
		return generated.GetExpandedContent401JSONResponse(unauthorizedError()), nil
	}

	item, err := h.content.GetExpandedContent(ctx, request.ExpandedContentId.String())
	if err != nil {
		if kind, _ := classify(err); kind == errKindNotFound {
			return generated.GetExpandedContent404JSONResponse(notFoundError("no expanded content item exists with the given id")), nil
		}
		return nil, err
	}

	return generated.GetExpandedContent200JSONResponse(toExpandedContent(item)), nil
}

func (h *Handler) CreateLearningPath(ctx context.Context, request generated.CreateLearningPathRequestObject) (generated.CreateLearningPathResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.CreateLearningPath401JSONResponse(unauthorizedError()), nil
	}

	contentNodeIDs := make([]string, len(request.Body.Items))
	for i, item := range request.Body.Items {
		contentNodeIDs[i] = item.ContentNodeId.String()
	}

	path, err := h.path.CreateLearningPath(ctx, caller, request.Body.Title, contentNodeIDs)
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.CreateLearningPath400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindForbidden:
			return generated.CreateLearningPath403JSONResponse(forbiddenError("only teachers and admins may create learning paths")), nil
		case errKindNotFound, errKindOther:
			return nil, err
		}
	}

	return generated.CreateLearningPath201JSONResponse(toLearningPath(path)), nil
}

func (h *Handler) GetLearningPath(ctx context.Context, request generated.GetLearningPathRequestObject) (generated.GetLearningPathResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.GetLearningPath401JSONResponse(unauthorizedError()), nil
	}

	path, err := h.path.GetLearningPath(ctx, caller, request.LearningPathId.String())
	if err != nil {
		kind, _ := classify(err)
		switch kind {
		case errKindForbidden:
			return generated.GetLearningPath403JSONResponse(forbiddenError("students may not retrieve learning paths directly")), nil
		case errKindNotFound:
			return generated.GetLearningPath404JSONResponse(notFoundError("no learning path exists with the given id")), nil
		case errKindValidation, errKindOther:
			return nil, err
		}
	}

	return generated.GetLearningPath200JSONResponse(toLearningPath(path)), nil
}

func (h *Handler) AssignLearningPath(ctx context.Context, request generated.AssignLearningPathRequestObject) (generated.AssignLearningPathResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.AssignLearningPath401JSONResponse(unauthorizedError()), nil
	}

	assignment, err := h.assignment.AssignLearningPath(ctx, caller, request.StudentId.String(), request.Body.LearningPathId.String())
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.AssignLearningPath400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindForbidden:
			return generated.AssignLearningPath403JSONResponse(forbiddenError("only teachers and admins may assign learning paths")), nil
		case errKindNotFound:
			return generated.AssignLearningPath404JSONResponse(notFoundError("the student or learning path does not exist")), nil
		case errKindOther:
			return nil, err
		}
	}

	return generated.AssignLearningPath201JSONResponse(toPathAssignment(assignment)), nil
}

func (h *Handler) GetMyPath(ctx context.Context, _ generated.GetMyPathRequestObject) (generated.GetMyPathResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.GetMyPath401JSONResponse(unauthorizedError()), nil
	}

	view, err := h.assignment.GetMyPath(ctx, caller)
	if err != nil {
		kind, _ := classify(err)
		switch kind {
		case errKindForbidden:
			return generated.GetMyPath403JSONResponse(forbiddenError("only students may access this endpoint")), nil
		case errKindNotFound:
			return generated.GetMyPath404JSONResponse(notFoundError("the authenticated student has no active path assignment")), nil
		case errKindValidation, errKindOther:
			return nil, err
		}
	}

	return generated.GetMyPath200JSONResponse(toStudentPathView(view)), nil
}
