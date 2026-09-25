package http

import (
	"context"
	"errors"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// uuidsToStrings converts a slice of parsed request UUIDs to the plain
// string ids the application layer works in.
func uuidsToStrings(ids []openapi_types.UUID) []string {
	result := make([]string, len(ids))
	for i, id := range ids {
		result[i] = id.String()
	}
	return result
}

// uuidPtrToStringPtr converts an optional request UUID to an optional
// string id, preserving nil.
func uuidPtrToStringPtr(id *openapi_types.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}

// Handler implements generated.StrictServerInterface — one method per
// OpenAPI operation, each translating between generated wire types and the
// application layer.
type Handler struct {
	identity         *application.IdentityService
	content          *application.ContentService
	challenge        *application.ChallengeService
	exercise         *application.ExerciseService
	skill            *application.SkillService
	concept          *application.ConceptService
	media            *application.MediaService
	path             *application.LearningPathService
	studentPath      *application.StudentPathService
	course           *application.CourseService
	courseEnrollment *application.CourseEnrollmentService
	instrument       *application.InstrumentService
	diagram          *application.DiagramService

	// pingers back the readiness probe only; the health probes never touch
	// the application services above.
	learningGraphPinger   ports.Pinger
	completionStatePinger ports.Pinger
}

var _ generated.StrictServerInterface = (*Handler)(nil)

func NewHandler(
	identity *application.IdentityService,
	content *application.ContentService,
	challenge *application.ChallengeService,
	exercise *application.ExerciseService,
	skill *application.SkillService,
	concept *application.ConceptService,
	media *application.MediaService,
	path *application.LearningPathService,
	studentPath *application.StudentPathService,
	course *application.CourseService,
	courseEnrollment *application.CourseEnrollmentService,
	instrument *application.InstrumentService,
	diagram *application.DiagramService,
	learningGraphPinger ports.Pinger,
	completionStatePinger ports.Pinger,
) *Handler {
	return &Handler{
		identity:              identity,
		content:               content,
		challenge:             challenge,
		exercise:              exercise,
		skill:                 skill,
		concept:               concept,
		media:                 media,
		path:                  path,
		studentPath:           studentPath,
		course:                course,
		courseEnrollment:      courseEnrollment,
		instrument:            instrument,
		diagram:               diagram,
		learningGraphPinger:   learningGraphPinger,
		completionStatePinger: completionStatePinger,
	}
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
	user, err := h.identity.ResolveCaller(ctx, clerkUserID, NameClaimFromContext(ctx))
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

	acceptLanguageCandidate := AcceptLanguageCandidateFromContext(ctx)
	user, err := h.identity.RegisterUser(ctx, clerkUserID, domain.Role(request.Body.Role), acceptLanguageCandidate, NameClaimFromContext(ctx))
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

	user, err := h.identity.ResolveCaller(ctx, clerkUserID, NameClaimFromContext(ctx))
	if err != nil {
		if kind, _ := classify(err); kind == errKindNotFound {
			return generated.GetMyProfile404JSONResponse(notFoundError("no user record exists for this Clerk identity")), nil
		}
		return nil, err
	}

	return generated.GetMyProfile200JSONResponse(toUserProfile(user)), nil
}

func (h *Handler) UpdateMyLocale(ctx context.Context, request generated.UpdateMyLocaleRequestObject) (generated.UpdateMyLocaleResponseObject, error) {
	clerkUserID, ok := ClerkUserIDFromContext(ctx)
	if !ok {
		return generated.UpdateMyLocale401JSONResponse(unauthorizedError()), nil
	}

	user, err := h.identity.UpdateLocale(ctx, clerkUserID, request.Body.Locale)
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.UpdateMyLocale400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindNotFound:
			return generated.UpdateMyLocale404JSONResponse(notFoundError("no user record exists for this Clerk identity")), nil
		case errKindForbidden, errKindOther:
			return nil, err
		}
	}

	return generated.UpdateMyLocale200JSONResponse(toUserProfile(user)), nil
}

func (h *Handler) CreateContentNode(ctx context.Context, request generated.CreateContentNodeRequestObject) (generated.CreateContentNodeResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.CreateContentNode401JSONResponse(unauthorizedError()), nil
	}

	body := request.Body
	node, err := h.content.CreateContentNode(ctx, caller, body.Title,
		domain.ContentType(body.ContentType),
		uuidsToStrings(body.Classification.SkillIds), uuidsToStrings(body.Classification.ConceptIds),
		domain.DifficultyLevel(body.Classification.DifficultyLevel),
		body.LanguageCodes, body.MediaUrl, toDomainPromptDocumentPtr(body.RichContent))
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

	names, err := h.loadUserNames(ctx, contentNodeUserIDs(node))
	if err != nil {
		return nil, err
	}
	return generated.CreateContentNode201JSONResponse(toContentNode(node, names)), nil
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

	names, err := h.loadUserNames(ctx, contentNodeUserIDs(node))
	if err != nil {
		return nil, err
	}
	return generated.GetContentNode200JSONResponse(toContentNode(node, names)), nil
}

func (h *Handler) ListContentNodes(ctx context.Context, request generated.ListContentNodesRequestObject) (generated.ListContentNodesResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.ListContentNodes401JSONResponse(unauthorizedError()), nil
	}

	var contentType domain.ContentType
	if request.Params.ContentType != nil {
		contentType = domain.ContentType(*request.Params.ContentType)
	}
	var skillID string
	if request.Params.SkillId != nil {
		skillID = request.Params.SkillId.String()
	}
	var conceptID string
	if request.Params.ConceptId != nil {
		conceptID = request.Params.ConceptId.String()
	}
	var difficulty domain.DifficultyLevel
	if request.Params.DifficultyLevel != nil {
		difficulty = domain.DifficultyLevel(*request.Params.DifficultyLevel)
	}

	page, err := domain.NewPageRequest(request.Params.Limit, request.Params.Offset)
	if err != nil {
		return listValidationFailure[generated.ListContentNodesResponseObject](err, func(e generated.ValidationError) generated.ListContentNodesResponseObject {
			return generated.ListContentNodes400JSONResponse(e)
		})
	}
	filter := domain.ContentNodeFilter{
		ContentType: contentType,
		SkillID:     skillID,
		ConceptID:   conceptID,
		Difficulty:  difficulty,
		Query:       searchQuery(request.Params.Q),
	}

	result, err := h.content.ListContentNodes(ctx, caller, filter, page)
	if err != nil {
		if kind, _ := classify(err); kind == errKindForbidden {
			return generated.ListContentNodes403JSONResponse(forbiddenError("only teachers and admins may list content nodes")), nil
		}
		return nil, err
	}

	names, err := h.loadUserNames(ctx, contentNodeUserIDs(result.Items...))
	if err != nil {
		return nil, err
	}
	return generated.ListContentNodes200JSONResponse{
		Items: toContentNodes(result.Items, names), Total: result.Total, Limit: page.Limit, Offset: page.Offset,
	}, nil
}

func (h *Handler) UpdateContentNode(ctx context.Context, request generated.UpdateContentNodeRequestObject) (generated.UpdateContentNodeResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.UpdateContentNode401JSONResponse(unauthorizedError()), nil
	}

	body := request.Body
	node, err := h.content.UpdateContentNode(ctx, caller, request.ContentNodeId.String(), body.Title,
		uuidsToStrings(body.Classification.SkillIds), uuidsToStrings(body.Classification.ConceptIds),
		domain.DifficultyLevel(body.Classification.DifficultyLevel), body.LanguageCodes,
		body.MediaUrl, toDomainPromptDocumentPtr(body.RichContent))
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.UpdateContentNode400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindForbidden:
			return generated.UpdateContentNode403JSONResponse(forbiddenError("only the creating teacher or an admin may update this content node")), nil
		case errKindNotFound:
			return generated.UpdateContentNode404JSONResponse(notFoundError("no content node exists with the given content_node_id")), nil
		case errKindOther:
			return nil, err
		}
	}

	names, err := h.loadUserNames(ctx, contentNodeUserIDs(node))
	if err != nil {
		return nil, err
	}
	return generated.UpdateContentNode200JSONResponse(toContentNode(node, names)), nil
}

func (h *Handler) CreateChallenge(ctx context.Context, request generated.CreateChallengeRequestObject) (generated.CreateChallengeResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.CreateChallenge401JSONResponse(unauthorizedError()), nil
	}

	var shuffleExercises, shuffleOptions bool
	if request.Body.ShuffleExercises != nil {
		shuffleExercises = *request.Body.ShuffleExercises
	}
	if request.Body.ShuffleOptions != nil {
		shuffleOptions = *request.Body.ShuffleOptions
	}

	challenge, err := h.challenge.CreateChallenge(ctx, caller, request.ContentNodeId.String(),
		uuidPtrToStringPtr(request.Body.SubjectSkillId), uuidPtrToStringPtr(request.Body.SubjectConceptId),
		request.Body.PassThreshold, request.Body.TimeThresholdMs, shuffleExercises, shuffleOptions)
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

func (h *Handler) UpdateChallenge(ctx context.Context, request generated.UpdateChallengeRequestObject) (generated.UpdateChallengeResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.UpdateChallenge401JSONResponse(unauthorizedError()), nil
	}

	var shuffleExercises, shuffleOptions bool
	if request.Body.ShuffleExercises != nil {
		shuffleExercises = *request.Body.ShuffleExercises
	}
	if request.Body.ShuffleOptions != nil {
		shuffleOptions = *request.Body.ShuffleOptions
	}

	challenge, err := h.challenge.UpdateChallenge(ctx, caller, request.ChallengeId.String(),
		uuidPtrToStringPtr(request.Body.SubjectSkillId), uuidPtrToStringPtr(request.Body.SubjectConceptId),
		request.Body.PassThreshold, request.Body.TimeThresholdMs, shuffleExercises, shuffleOptions)
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.UpdateChallenge400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindForbidden:
			return generated.UpdateChallenge403JSONResponse(forbiddenError("only the creating teacher or an admin may update this challenge")), nil
		case errKindNotFound:
			return generated.UpdateChallenge404JSONResponse(notFoundError("no challenge exists with the given challenge_id")), nil
		case errKindOther:
			return nil, err
		}
	}

	return generated.UpdateChallenge200JSONResponse(toChallenge(challenge)), nil
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

	body := request.Body
	var remediationTargets []generated.RemediationTarget
	if body.RemediationTargets != nil {
		remediationTargets = *body.RemediationTargets
	}
	exercise, err := h.exercise.CreateExercise(ctx, caller, body.Title, toDomainPromptDocument(body.Prompt),
		domain.ExerciseType(body.ExerciseType), uuidsToStrings(body.SkillIds), uuidsToStrings(body.ConceptIds),
		body.ImageUrl, body.AudioUrl, toDomainDiagramRefPtr(body.DiagramRef), toDomainDiagramStackRefPtr(body.DiagramStackRef),
		toDomainOptions(derefOptions(body.Options)), body.EstimatedDurationSeconds,
		toDomainRemediationTargets(remediationTargets), body.LanguageCodes)
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.CreateExercise400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindForbidden:
			return generated.CreateExercise403JSONResponse(forbiddenError("only teachers and admins may create exercises")), nil
		case errKindNotFound, errKindOther:
			return nil, err
		}
	}

	return generated.CreateExercise201JSONResponse(toExercise(exercise)), nil
}

func (h *Handler) GetExercise(ctx context.Context, request generated.GetExerciseRequestObject) (generated.GetExerciseResponseObject, error) {
	if _, ok := h.resolveCaller(ctx); !ok {
		return generated.GetExercise401JSONResponse(unauthorizedError()), nil
	}

	exercise, err := h.exercise.GetExercise(ctx, request.ExerciseId.String())
	if err != nil {
		if kind, _ := classify(err); kind == errKindNotFound {
			return generated.GetExercise404JSONResponse(notFoundError("no exercise exists with the given id")), nil
		}
		return nil, err
	}

	return generated.GetExercise200JSONResponse(toExercise(exercise)), nil
}

func (h *Handler) ListExercises(ctx context.Context, request generated.ListExercisesRequestObject) (generated.ListExercisesResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.ListExercises401JSONResponse(unauthorizedError()), nil
	}

	var skillID string
	if request.Params.SkillId != nil {
		skillID = request.Params.SkillId.String()
	}
	var exerciseType domain.ExerciseType
	if request.Params.ExerciseType != nil {
		exerciseType = domain.ExerciseType(*request.Params.ExerciseType)
	}

	page, err := domain.NewPageRequest(request.Params.Limit, request.Params.Offset)
	if err != nil {
		return listValidationFailure[generated.ListExercisesResponseObject](err, func(e generated.ValidationError) generated.ListExercisesResponseObject {
			return generated.ListExercises400JSONResponse(e)
		})
	}

	result, err := h.exercise.ListExercises(ctx, caller, domain.ExerciseFilter{SkillID: skillID, ExerciseType: exerciseType}, page)
	if err != nil {
		if kind, _ := classify(err); kind == errKindForbidden {
			return generated.ListExercises403JSONResponse(forbiddenError("only teachers and admins may list exercises")), nil
		}
		return nil, err
	}

	return generated.ListExercises200JSONResponse{
		Items: toExercises(result.Items), Total: result.Total, Limit: page.Limit, Offset: page.Offset,
	}, nil
}

func (h *Handler) UpdateExercise(ctx context.Context, request generated.UpdateExerciseRequestObject) (generated.UpdateExerciseResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.UpdateExercise401JSONResponse(unauthorizedError()), nil
	}

	body := request.Body
	var remediationTargets []generated.RemediationTarget
	if body.RemediationTargets != nil {
		remediationTargets = *body.RemediationTargets
	}
	exercise, err := h.exercise.UpdateExercise(ctx, caller, request.ExerciseId.String(), body.Title, toDomainPromptDocument(body.Prompt),
		uuidsToStrings(body.SkillIds), uuidsToStrings(body.ConceptIds),
		body.ImageUrl, body.AudioUrl, toDomainDiagramRefPtr(body.DiagramRef), toDomainDiagramStackRefPtr(body.DiagramStackRef),
		toDomainOptions(derefOptions(body.Options)), body.EstimatedDurationSeconds,
		toDomainRemediationTargets(remediationTargets), body.LanguageCodes)
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.UpdateExercise400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindForbidden:
			return generated.UpdateExercise403JSONResponse(forbiddenError("only teachers and admins may update exercises")), nil
		case errKindNotFound:
			return generated.UpdateExercise404JSONResponse(notFoundError("no exercise exists with the given exercise_id")), nil
		case errKindOther:
			return nil, err
		}
	}

	return generated.UpdateExercise200JSONResponse(toExercise(exercise)), nil
}

func (h *Handler) LinkExerciseToChallenge(ctx context.Context, request generated.LinkExerciseToChallengeRequestObject) (generated.LinkExerciseToChallengeResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.LinkExerciseToChallenge401JSONResponse(unauthorizedError()), nil
	}

	exercise, err := h.exercise.LinkExerciseToChallenge(ctx, caller, request.ChallengeId.String(), request.ExerciseId.String())
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrForbidden):
			return generated.LinkExerciseToChallenge403JSONResponse(forbiddenError("only teachers and admins may link exercises to challenges")), nil
		case errors.Is(err, domain.ErrNotFound):
			return generated.LinkExerciseToChallenge404JSONResponse(notFoundError("no challenge exists with challenge_id, or no exercise exists with exercise_id")), nil
		case errors.Is(err, domain.ErrAlreadyExists):
			return generated.LinkExerciseToChallenge409JSONResponse(conflictError("the exercise is already linked to this challenge")), nil
		default:
			return nil, err
		}
	}

	return generated.LinkExerciseToChallenge201JSONResponse(toExercise(exercise)), nil
}

func (h *Handler) UnlinkExerciseFromChallenge(ctx context.Context, request generated.UnlinkExerciseFromChallengeRequestObject) (generated.UnlinkExerciseFromChallengeResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.UnlinkExerciseFromChallenge401JSONResponse(unauthorizedError()), nil
	}

	err := h.exercise.UnlinkExerciseFromChallenge(ctx, caller, request.ChallengeId.String(), request.ExerciseId.String())
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrForbidden):
			return generated.UnlinkExerciseFromChallenge403JSONResponse(forbiddenError("only teachers and admins may unlink exercises from challenges")), nil
		case errors.Is(err, domain.ErrNotFound):
			return generated.UnlinkExerciseFromChallenge404JSONResponse(notFoundError("no challenge exists with challenge_id, no exercise exists with exercise_id, or the exercise is not currently linked to this challenge")), nil
		default:
			return nil, err
		}
	}

	return generated.UnlinkExerciseFromChallenge204Response{}, nil
}

func (h *Handler) CreateMediaUploadUrl(ctx context.Context, request generated.CreateMediaUploadUrlRequestObject) (generated.CreateMediaUploadUrlResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.CreateMediaUploadUrl401JSONResponse(unauthorizedError()), nil
	}

	body := request.Body
	var exerciseID *string
	if body.ExerciseId != nil {
		s := body.ExerciseId.String()
		exerciseID = &s
	}
	url, err := h.media.CreateUploadURL(ctx, caller, domain.MediaUploadPurpose(body.Purpose), exerciseID,
		domain.MediaContentType(body.ContentType), body.FileName)
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.CreateMediaUploadUrl400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindForbidden:
			return generated.CreateMediaUploadUrl403JSONResponse(forbiddenError("only teachers and admins may request an upload URL")), nil
		case errKindNotFound:
			return generated.CreateMediaUploadUrl404JSONResponse(notFoundError("purpose is exercise_asset but no exercise exists with the given exercise_id")), nil
		case errKindOther:
			return nil, err
		}
	}

	return generated.CreateMediaUploadUrl201JSONResponse(toMediaUploadURL(url)), nil
}

func (h *Handler) CreateExpandedContent(ctx context.Context, request generated.CreateExpandedContentRequestObject) (generated.CreateExpandedContentResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.CreateExpandedContent401JSONResponse(unauthorizedError()), nil
	}

	body := request.Body
	item, err := h.content.CreateExpandedContent(ctx, caller, request.ContentNodeId.String(),
		domain.ExpandedContentType(body.ContentType), body.MediaUrl, toDomainPromptDocumentPtr(body.RichContent),
		toDomainDiagramRefPtr(body.DiagramRef), toDomainDiagramStackRefPtr(body.DiagramStackRef),
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

func (h *Handler) UpdateExpandedContent(ctx context.Context, request generated.UpdateExpandedContentRequestObject) (generated.UpdateExpandedContentResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.UpdateExpandedContent401JSONResponse(unauthorizedError()), nil
	}

	body := request.Body
	item, err := h.content.UpdateExpandedContent(ctx, caller, request.ExpandedContentId.String(),
		domain.ExpandedContentType(body.ContentType), body.MediaUrl, toDomainPromptDocumentPtr(body.RichContent),
		toDomainDiagramRefPtr(body.DiagramRef), toDomainDiagramStackRefPtr(body.DiagramStackRef),
		body.TriggerAtSeconds, body.HideAtSeconds, body.TriggerAtParagraph, body.DurationMs, body.Caption)
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.UpdateExpandedContent400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindForbidden:
			return generated.UpdateExpandedContent403JSONResponse(forbiddenError("only the creating teacher or an admin may update this expanded content item")), nil
		case errKindNotFound:
			return generated.UpdateExpandedContent404JSONResponse(notFoundError("no expanded content item exists with the given id")), nil
		case errKindOther:
			return nil, err
		}
	}

	return generated.UpdateExpandedContent200JSONResponse(toExpandedContent(item)), nil
}

func (h *Handler) DeleteExpandedContent(ctx context.Context, request generated.DeleteExpandedContentRequestObject) (generated.DeleteExpandedContentResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.DeleteExpandedContent401JSONResponse(unauthorizedError()), nil
	}

	err := h.content.DeleteExpandedContent(ctx, caller, request.ExpandedContentId.String())
	if err != nil {
		kind, _ := classify(err)
		switch kind {
		case errKindForbidden:
			return generated.DeleteExpandedContent403JSONResponse(forbiddenError("only the creating teacher or an admin may delete this expanded content item")), nil
		case errKindNotFound:
			return generated.DeleteExpandedContent404JSONResponse(notFoundError("no expanded content item exists with the given id")), nil
		case errKindValidation, errKindOther:
			return nil, err
		}
	}

	return generated.DeleteExpandedContent204Response{}, nil
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

	pathItems := make([]application.PathItemInput, len(request.Body.Items))
	for i, item := range request.Body.Items {
		pathItems[i] = application.PathItemInput{
			ContentNodeID: item.ContentNodeId.String(),
			SectionLabel:  item.SectionLabel,
		}
	}

	path, err := h.path.CreateLearningPath(ctx, caller, request.Body.Title, pathItems)
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

	names, err := h.loadUserNames(ctx, learningPathUserIDs(path))
	if err != nil {
		return nil, err
	}
	return generated.CreateLearningPath201JSONResponse(toLearningPath(path, names)), nil
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

	names, err := h.loadUserNames(ctx, learningPathUserIDs(path))
	if err != nil {
		return nil, err
	}
	return generated.GetLearningPath200JSONResponse(toLearningPath(path, names)), nil
}

func (h *Handler) ListLearningPaths(ctx context.Context, request generated.ListLearningPathsRequestObject) (generated.ListLearningPathsResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.ListLearningPaths401JSONResponse(unauthorizedError()), nil
	}

	page, err := domain.NewPageRequest(request.Params.Limit, request.Params.Offset)
	if err != nil {
		return listValidationFailure[generated.ListLearningPathsResponseObject](err, func(e generated.ValidationError) generated.ListLearningPathsResponseObject {
			return generated.ListLearningPaths400JSONResponse(e)
		})
	}

	result, err := h.path.ListLearningPaths(ctx, caller, domain.LearningPathFilter{Query: searchQuery(request.Params.Q)}, page)
	if err != nil {
		if kind, _ := classify(err); kind == errKindForbidden {
			return generated.ListLearningPaths403JSONResponse(forbiddenError("students may not list learning paths directly")), nil
		}
		return nil, err
	}

	names, err := h.loadUserNames(ctx, learningPathUserIDs(result.Items...))
	if err != nil {
		return nil, err
	}
	return generated.ListLearningPaths200JSONResponse{
		Items: toLearningPaths(result.Items, names), Total: result.Total, Limit: page.Limit, Offset: page.Offset,
	}, nil
}

func (h *Handler) ReplaceLearningPath(ctx context.Context, request generated.ReplaceLearningPathRequestObject) (generated.ReplaceLearningPathResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.ReplaceLearningPath401JSONResponse(unauthorizedError()), nil
	}

	pathItems := make([]application.PathItemInput, len(request.Body.Items))
	for i, item := range request.Body.Items {
		pathItems[i] = application.PathItemInput{
			ContentNodeID: item.ContentNodeId.String(),
			SectionLabel:  item.SectionLabel,
		}
	}

	path, err := h.path.ReplaceLearningPath(ctx, caller, request.LearningPathId.String(), request.Body.Title, pathItems)
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.ReplaceLearningPath400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindForbidden:
			return generated.ReplaceLearningPath403JSONResponse(forbiddenError("only the creating teacher or an admin may replace this learning path")), nil
		case errKindNotFound:
			return generated.ReplaceLearningPath404JSONResponse(notFoundError("no learning path exists with the given id")), nil
		case errKindOther:
			return nil, err
		}
	}

	names, err := h.loadUserNames(ctx, learningPathUserIDs(path))
	if err != nil {
		return nil, err
	}
	return generated.ReplaceLearningPath200JSONResponse(toLearningPath(path, names)), nil
}

func (h *Handler) DeleteLearningPath(ctx context.Context, request generated.DeleteLearningPathRequestObject) (generated.DeleteLearningPathResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.DeleteLearningPath401JSONResponse(unauthorizedError()), nil
	}

	err := h.path.DeleteLearningPath(ctx, caller, request.LearningPathId.String())
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrForbidden):
			return generated.DeleteLearningPath403JSONResponse(forbiddenError("only the creating teacher or an admin may delete this learning path")), nil
		case errors.Is(err, domain.ErrNotFound):
			return generated.DeleteLearningPath404JSONResponse(notFoundError("no learning path exists with the given id")), nil
		case errors.Is(err, domain.ErrConflict):
			return generated.DeleteLearningPath409JSONResponse(conflictError("this learning path is referenced by a checkpoint of at least one published course version")), nil
		default:
			return nil, err
		}
	}

	return generated.DeleteLearningPath204Response{}, nil
}

func (h *Handler) AssignLearningPath(ctx context.Context, request generated.AssignLearningPathRequestObject) (generated.AssignLearningPathResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.AssignLearningPath401JSONResponse(unauthorizedError()), nil
	}

	sp, err := h.studentPath.AssignLearningPath(ctx, caller, request.StudentId.String(), request.Body.LearningPathId.String())
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.AssignLearningPath400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindForbidden:
			return generated.AssignLearningPath403JSONResponse(forbiddenError("only teachers and admins may assign learning paths")), nil
		case errKindNotFound:
			return generated.AssignLearningPath404JSONResponse(notFoundError("the student_id or learning_path_id does not exist, the student's role is not student, or one of the path's content nodes has never been published")), nil
		case errKindOther:
			return nil, err
		}
	}

	names, err := h.loadUserNames(ctx, studentPathUserIDs(sp))
	if err != nil {
		return nil, err
	}
	return generated.AssignLearningPath201JSONResponse(toStudentPath(sp, names)), nil
}

func (h *Handler) GetMyPath(ctx context.Context, _ generated.GetMyPathRequestObject) (generated.GetMyPathResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.GetMyPath401JSONResponse(unauthorizedError()), nil
	}

	view, err := h.studentPath.GetMyPath(ctx, caller)
	if err != nil {
		kind, _ := classify(err)
		switch kind {
		case errKindNotFound:
			return generated.GetMyPath404JSONResponse(notFoundError("the authenticated caller has no current path set")), nil
		case errKindForbidden, errKindValidation, errKindOther:
			return nil, err
		}
	}

	return generated.GetMyPath200JSONResponse(toStudentPathView(view)), nil
}

func (h *Handler) ArchiveStandaloneStudentPath(ctx context.Context, request generated.ArchiveStandaloneStudentPathRequestObject) (generated.ArchiveStandaloneStudentPathResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.ArchiveStandaloneStudentPath401JSONResponse(unauthorizedError()), nil
	}

	sp, err := h.studentPath.ArchiveStandaloneStudentPath(ctx, caller, request.StudentPathId.String())
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			return generated.ArchiveStandaloneStudentPath404JSONResponse(notFoundError("no non-archived standalone student path exists with this id for the caller")), nil
		case errors.Is(err, domain.ErrConflict):
			return generated.ArchiveStandaloneStudentPath409JSONResponse(conflictError("this is the caller's only current course or path; switch to another one first")), nil
		default:
			return nil, err
		}
	}

	names, err := h.loadUserNames(ctx, studentPathUserIDs(sp))
	if err != nil {
		return nil, err
	}
	return generated.ArchiveStandaloneStudentPath200JSONResponse(toStudentPath(sp, names)), nil
}

func (h *Handler) PublishContentNode(ctx context.Context, request generated.PublishContentNodeRequestObject) (generated.PublishContentNodeResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.PublishContentNode401JSONResponse(unauthorizedError()), nil
	}

	version, err := h.content.PublishContentNode(ctx, caller, request.ContentNodeId.String())
	if err != nil {
		kind, _ := classify(err)
		switch kind {
		case errKindForbidden:
			return generated.PublishContentNode403JSONResponse(forbiddenError("only the creating teacher or an admin may publish this content node")), nil
		case errKindNotFound:
			return generated.PublishContentNode404JSONResponse(notFoundError("no content node exists with the given id")), nil
		case errKindValidation, errKindOther:
			return nil, err
		}
	}

	return generated.PublishContentNode201JSONResponse(toContentNodeVersion(version)), nil
}

func (h *Handler) ListContentNodeVersions(ctx context.Context, request generated.ListContentNodeVersionsRequestObject) (generated.ListContentNodeVersionsResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.ListContentNodeVersions401JSONResponse(unauthorizedError()), nil
	}

	versions, err := h.content.ListContentNodeVersions(ctx, caller, request.ContentNodeId.String())
	if err != nil {
		kind, _ := classify(err)
		switch kind {
		case errKindForbidden:
			return generated.ListContentNodeVersions403JSONResponse(forbiddenError("only the creating teacher or an admin may view this content node's version history")), nil
		case errKindNotFound:
			return generated.ListContentNodeVersions404JSONResponse(notFoundError("no content node exists with the given id")), nil
		case errKindValidation, errKindOther:
			return nil, err
		}
	}

	items := make([]generated.ContentNodeVersion, len(versions))
	for i, v := range versions {
		items[i] = toContentNodeVersion(v)
	}
	return generated.ListContentNodeVersions200JSONResponse(items), nil
}

func (h *Handler) ListMyStandalonePaths(ctx context.Context, _ generated.ListMyStandalonePathsRequestObject) (generated.ListMyStandalonePathsResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.ListMyStandalonePaths401JSONResponse(unauthorizedError()), nil
	}

	paths, err := h.studentPath.ListMyStandalonePaths(ctx, caller)
	if err != nil {
		if kind, _ := classify(err); kind == errKindForbidden {
			return generated.ListMyStandalonePaths403JSONResponse(forbiddenError("only students hold student paths")), nil
		}
		return nil, err
	}

	names, err := h.loadUserNames(ctx, studentPathUserIDs(paths...))
	if err != nil {
		return nil, err
	}
	items := make([]generated.StudentPath, len(paths))
	for i, sp := range paths {
		items[i] = toStudentPath(sp, names)
	}
	return generated.ListMyStandalonePaths200JSONResponse(items), nil
}

func (h *Handler) ListCourses(ctx context.Context, request generated.ListCoursesRequestObject) (generated.ListCoursesResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.ListCourses401JSONResponse(unauthorizedError()), nil
	}

	page, err := domain.NewPageRequest(request.Params.Limit, request.Params.Offset)
	if err != nil {
		return listValidationFailure[generated.ListCoursesResponseObject](err, func(e generated.ValidationError) generated.ListCoursesResponseObject {
			return generated.ListCourses400JSONResponse(e)
		})
	}

	result, err := h.course.ListCourses(ctx, caller, courseListFilter(request.Params), page)
	if err != nil {
		if kind, _ := classify(err); kind == errKindForbidden {
			return generated.ListCourses403JSONResponse(forbiddenError("a teacher may only list courses they created")), nil
		}
		return nil, err
	}

	courseIDs := make([]string, len(result.Items))
	for i, c := range result.Items {
		courseIDs[i] = c.ID
	}
	latestByCourse, err := h.course.LatestVersions(ctx, courseIDs)
	if err != nil {
		return nil, err
	}

	names, err := h.loadUserNames(ctx, courseUserIDs(result.Items...))
	if err != nil {
		return nil, err
	}
	entries, err := toCourseCatalogEntries(result.Items, caller, func(courseID string) (*domain.CourseVersion, error) {
		if v, ok := latestByCourse[courseID]; ok {
			return &v, nil
		}
		return nil, nil
	}, names)
	if err != nil {
		return nil, err
	}

	return generated.ListCourses200JSONResponse{
		Items: entries, Total: result.Total, Limit: page.Limit, Offset: page.Offset,
	}, nil
}

// ListCourseCreators returns the creators of the courses visible to the
// caller, ordered by display name, then user id.
func (h *Handler) ListCourseCreators(ctx context.Context, _ generated.ListCourseCreatorsRequestObject) (generated.ListCourseCreatorsResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.ListCourseCreators401JSONResponse(unauthorizedError()), nil
	}

	ids, err := h.course.ListCourseCreatorIDs(ctx, caller)
	if err != nil {
		return nil, err
	}
	names, err := h.loadUserNames(ctx, ids)
	if err != nil {
		return nil, err
	}
	return generated.ListCourseCreators200JSONResponse(names.sortedRefs(ids)), nil
}

// latestCourseVersion returns course's latest published CourseVersion, or
// nil if it has never been published — the courseVersionLookup shape
// toCourseCatalogEntries and the toCourse/toCourseCatalogEntry mappers
// need to compute has_unpublished_changes and latest_published_version.
func (h *Handler) latestCourseVersion(ctx context.Context, courseID string) (*domain.CourseVersion, error) {
	version, err := h.course.LatestVersion(ctx, courseID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &version, nil
}

func (h *Handler) CreateCourse(ctx context.Context, request generated.CreateCourseRequestObject) (generated.CreateCourseResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.CreateCourse401JSONResponse(unauthorizedError()), nil
	}

	checkpoints := make([]application.CheckpointInput, len(request.Body.Checkpoints))
	for i, cp := range request.Body.Checkpoints {
		checkpoints[i] = application.CheckpointInput{
			LearningPathID: cp.LearningPathId.String(),
			Title:          cp.Title,
		}
	}

	course, err := h.course.CreateCourse(ctx, caller, request.Body.Title, request.Body.Summary,
		domain.DifficultyLevel(request.Body.Level), checkpoints)
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.CreateCourse400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindForbidden:
			return generated.CreateCourse403JSONResponse(forbiddenError("only teachers and admins may create courses")), nil
		case errKindNotFound, errKindOther:
			return nil, err
		}
	}

	names, err := h.loadUserNames(ctx, courseUserIDs(course))
	if err != nil {
		return nil, err
	}
	return generated.CreateCourse201JSONResponse(toCourse(course, nil, names)), nil
}

func (h *Handler) GetCourse(ctx context.Context, request generated.GetCourseRequestObject) (generated.GetCourseResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.GetCourse401JSONResponse(unauthorizedError()), nil
	}

	course, err := h.course.GetCourse(ctx, caller, request.CourseId.String())
	if err != nil {
		kind, _ := classify(err)
		switch kind {
		case errKindForbidden:
			return generated.GetCourse403JSONResponse(forbiddenError("only teachers and admins may view a course's live state")), nil
		case errKindNotFound:
			return generated.GetCourse404JSONResponse(notFoundError("no course exists with the given id")), nil
		case errKindValidation, errKindOther:
			return nil, err
		}
	}

	latest, err := h.latestCourseVersion(ctx, course.ID)
	if err != nil {
		return nil, err
	}

	names, err := h.loadUserNames(ctx, courseUserIDs(course))
	if err != nil {
		return nil, err
	}
	return generated.GetCourse200JSONResponse(toCourse(course, latest, names)), nil
}

func (h *Handler) ReplaceCourse(ctx context.Context, request generated.ReplaceCourseRequestObject) (generated.ReplaceCourseResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.ReplaceCourse401JSONResponse(unauthorizedError()), nil
	}

	checkpoints := make([]application.CheckpointInput, len(request.Body.Checkpoints))
	for i, cp := range request.Body.Checkpoints {
		checkpoints[i] = application.CheckpointInput{
			LearningPathID: cp.LearningPathId.String(),
			Title:          cp.Title,
		}
	}

	course, err := h.course.ReplaceCourse(ctx, caller, request.CourseId.String(), request.Body.Title, request.Body.Summary,
		domain.DifficultyLevel(request.Body.Level), checkpoints)
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.ReplaceCourse400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindForbidden:
			return generated.ReplaceCourse403JSONResponse(forbiddenError("only the creating teacher or an admin may replace this course")), nil
		case errKindNotFound:
			return generated.ReplaceCourse404JSONResponse(notFoundError("no course exists with the given id")), nil
		case errKindOther:
			return nil, err
		}
	}

	latest, err := h.latestCourseVersion(ctx, course.ID)
	if err != nil {
		return nil, err
	}

	names, err := h.loadUserNames(ctx, courseUserIDs(course))
	if err != nil {
		return nil, err
	}
	return generated.ReplaceCourse200JSONResponse(toCourse(course, latest, names)), nil
}

func (h *Handler) PublishCourse(ctx context.Context, request generated.PublishCourseRequestObject) (generated.PublishCourseResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.PublishCourse401JSONResponse(unauthorizedError()), nil
	}

	version, err := h.course.PublishCourse(ctx, caller, request.CourseId.String())
	if err != nil {
		kind, _ := classify(err)
		switch kind {
		case errKindForbidden:
			return generated.PublishCourse403JSONResponse(forbiddenError("only admins may publish a course")), nil
		case errKindNotFound:
			return generated.PublishCourse404JSONResponse(notFoundError("no course exists with the given id")), nil
		case errKindValidation, errKindOther:
			return nil, err
		}
	}

	return generated.PublishCourse201JSONResponse(toGeneratedCourseVersion(version)), nil
}

func (h *Handler) GetPublishedCourse(ctx context.Context, request generated.GetPublishedCourseRequestObject) (generated.GetPublishedCourseResponseObject, error) {
	if _, ok := h.resolveCaller(ctx); !ok {
		return generated.GetPublishedCourse401JSONResponse(unauthorizedError()), nil
	}

	courseID := request.CourseId.String()
	view, err := h.course.GetPublishedCourse(ctx, courseID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return generated.GetPublishedCourse404JSONResponse(notFoundError("no course exists with the given id, or it has never been published")), nil
		}
		return nil, err
	}

	return generated.GetPublishedCourse200JSONResponse(toCourseDetail(courseID, view)), nil
}

func (h *Handler) RetireCourse(ctx context.Context, request generated.RetireCourseRequestObject) (generated.RetireCourseResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.RetireCourse401JSONResponse(unauthorizedError()), nil
	}

	course, err := h.course.RetireCourse(ctx, caller, request.CourseId.String())
	if err != nil {
		kind, _ := classify(err)
		switch kind {
		case errKindForbidden:
			return generated.RetireCourse403JSONResponse(forbiddenError("only admins may retire a course")), nil
		case errKindNotFound:
			return generated.RetireCourse404JSONResponse(notFoundError("no course exists with the given id")), nil
		case errKindValidation, errKindOther:
			return nil, err
		}
	}

	latest, err := h.latestCourseVersion(ctx, course.ID)
	if err != nil {
		return nil, err
	}

	names, err := h.loadUserNames(ctx, courseUserIDs(course))
	if err != nil {
		return nil, err
	}
	return generated.RetireCourse200JSONResponse(toCourse(course, latest, names)), nil
}

func (h *Handler) ListMyCourseEnrollments(ctx context.Context, _ generated.ListMyCourseEnrollmentsRequestObject) (generated.ListMyCourseEnrollmentsResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.ListMyCourseEnrollments401JSONResponse(unauthorizedError()), nil
	}

	enrollments, err := h.courseEnrollment.ListMyCourseEnrollments(ctx, caller)
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			return generated.ListMyCourseEnrollments403JSONResponse(forbiddenError("only students hold course enrollments")), nil
		}
		return nil, err
	}

	names, err := h.loadUserNames(ctx, courseEnrollmentUserIDs(enrollments...))
	if err != nil {
		return nil, err
	}
	return generated.ListMyCourseEnrollments200JSONResponse(toCourseEnrollments(enrollments, names)), nil
}

func (h *Handler) CreateCourseEnrollment(ctx context.Context, request generated.CreateCourseEnrollmentRequestObject) (generated.CreateCourseEnrollmentResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.CreateCourseEnrollment401JSONResponse(unauthorizedError()), nil
	}

	enrollment, err := h.courseEnrollment.CreateCourseEnrollment(ctx, caller, request.Body.CourseId.String())
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrForbidden):
			return generated.CreateCourseEnrollment403JSONResponse(forbiddenError("only students may self-enroll")), nil
		case errors.Is(err, domain.ErrNotFound):
			return generated.CreateCourseEnrollment404JSONResponse(notFoundError("the course_id does not exist, the course is draft or retired, or its latest published version is not currently available for new enrollments")), nil
		case errors.Is(err, domain.ErrConflict):
			return generated.CreateCourseEnrollment409JSONResponse(conflictError("the student already has an active course enrollment for this course")), nil
		default:
			var valErr *domain.ValidationError
			if errors.As(err, &valErr) {
				return generated.CreateCourseEnrollment400JSONResponse(validationErrorResponse(valErr)), nil
			}
			return nil, err
		}
	}

	names, err := h.loadUserNames(ctx, courseEnrollmentUserIDs(enrollment))
	if err != nil {
		return nil, err
	}
	return generated.CreateCourseEnrollment201JSONResponse(toCourseEnrollment(enrollment, names)), nil
}

func (h *Handler) AbandonCourseEnrollment(ctx context.Context, request generated.AbandonCourseEnrollmentRequestObject) (generated.AbandonCourseEnrollmentResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.AbandonCourseEnrollment401JSONResponse(unauthorizedError()), nil
	}

	enrollment, err := h.courseEnrollment.AbandonCourseEnrollment(ctx, caller, request.CourseEnrollmentId.String())
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrForbidden):
			return generated.AbandonCourseEnrollment403JSONResponse(forbiddenError("only students hold course enrollments")), nil
		case errors.Is(err, domain.ErrNotFound):
			return generated.AbandonCourseEnrollment404JSONResponse(notFoundError("no active enrollment with this id exists for the caller")), nil
		case errors.Is(err, domain.ErrConflict):
			return generated.AbandonCourseEnrollment409JSONResponse(conflictError("this is the caller's only current course or path with something else eligible to become current; switch to it first")), nil
		default:
			return nil, err
		}
	}

	names, err := h.loadUserNames(ctx, courseEnrollmentUserIDs(enrollment))
	if err != nil {
		return nil, err
	}
	return generated.AbandonCourseEnrollment200JSONResponse(toCourseEnrollment(enrollment, names)), nil
}

func (h *Handler) SetCurrentPath(ctx context.Context, request generated.SetCurrentPathRequestObject) (generated.SetCurrentPathResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.SetCurrentPath401JSONResponse(unauthorizedError()), nil
	}

	view, err := h.studentPath.SetCurrentPath(ctx, caller, application.SetCurrentPathInput{
		CourseEnrollmentID: uuidPtrToStringPtr(request.Body.CourseEnrollmentId),
		StudentPathID:      uuidPtrToStringPtr(request.Body.StudentPathId),
	})
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.SetCurrentPath400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindForbidden:
			return generated.SetCurrentPath403JSONResponse(forbiddenError("only students hold a current course or path")), nil
		case errKindNotFound:
			return generated.SetCurrentPath404JSONResponse(notFoundError("the referenced course enrollment or student path does not exist or does not belong to the caller")), nil
		case errKindOther:
			return nil, err
		}
	}

	return generated.SetCurrentPath200JSONResponse(toStudentPathView(view)), nil
}

func (h *Handler) ListContentNodeChallenges(ctx context.Context, request generated.ListContentNodeChallengesRequestObject) (generated.ListContentNodeChallengesResponseObject, error) {
	if _, ok := h.resolveCaller(ctx); !ok {
		return generated.ListContentNodeChallenges401JSONResponse(unauthorizedError()), nil
	}

	challenges, err := h.challenge.ListChallengesForContentNode(ctx, request.ContentNodeId.String())
	if err != nil {
		if kind, _ := classify(err); kind == errKindNotFound {
			return generated.ListContentNodeChallenges404JSONResponse(notFoundError("no content node exists with the given content_node_id")), nil
		}
		return nil, err
	}

	return generated.ListContentNodeChallenges200JSONResponse(toChallenges(challenges)), nil
}

func (h *Handler) ListChallengeExercises(ctx context.Context, request generated.ListChallengeExercisesRequestObject) (generated.ListChallengeExercisesResponseObject, error) {
	if _, ok := h.resolveCaller(ctx); !ok {
		return generated.ListChallengeExercises401JSONResponse(unauthorizedError()), nil
	}

	exercises, err := h.exercise.ListExercisesForChallenge(ctx, request.ChallengeId.String())
	if err != nil {
		if kind, _ := classify(err); kind == errKindNotFound {
			return generated.ListChallengeExercises404JSONResponse(notFoundError("no challenge exists with the given challenge_id")), nil
		}
		return nil, err
	}

	return generated.ListChallengeExercises200JSONResponse(toExercises(exercises)), nil
}

func (h *Handler) ListContentNodePathExercises(ctx context.Context, request generated.ListContentNodePathExercisesRequestObject) (generated.ListContentNodePathExercisesResponseObject, error) {
	if _, ok := h.resolveCaller(ctx); !ok {
		return generated.ListContentNodePathExercises401JSONResponse(unauthorizedError()), nil
	}

	exercises, err := h.exercise.ListPathExercisesForContentNode(ctx, request.ContentNodeId.String())
	if err != nil {
		if kind, _ := classify(err); kind == errKindNotFound {
			return generated.ListContentNodePathExercises404JSONResponse(notFoundError("no content node exists with the given content_node_id")), nil
		}
		return nil, err
	}

	return generated.ListContentNodePathExercises200JSONResponse(toExercises(exercises)), nil
}

func (h *Handler) LinkExerciseToContentNode(ctx context.Context, request generated.LinkExerciseToContentNodeRequestObject) (generated.LinkExerciseToContentNodeResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.LinkExerciseToContentNode401JSONResponse(unauthorizedError()), nil
	}

	exercise, err := h.exercise.LinkExerciseToContentNode(ctx, caller, request.ContentNodeId.String(), request.ExerciseId.String())
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrForbidden):
			return generated.LinkExerciseToContentNode403JSONResponse(forbiddenError("only teachers and admins may link exercises to content nodes")), nil
		case errors.Is(err, domain.ErrNotFound):
			return generated.LinkExerciseToContentNode404JSONResponse(notFoundError("no content node exists with content_node_id, or no exercise exists with exercise_id")), nil
		case errors.Is(err, domain.ErrAlreadyExists):
			return generated.LinkExerciseToContentNode409JSONResponse(conflictError("the exercise is already linked to this content node")), nil
		default:
			return nil, err
		}
	}

	return generated.LinkExerciseToContentNode201JSONResponse(toExercise(exercise)), nil
}

func (h *Handler) UnlinkExerciseFromContentNode(ctx context.Context, request generated.UnlinkExerciseFromContentNodeRequestObject) (generated.UnlinkExerciseFromContentNodeResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.UnlinkExerciseFromContentNode401JSONResponse(unauthorizedError()), nil
	}

	err := h.exercise.UnlinkExerciseFromContentNode(ctx, caller, request.ContentNodeId.String(), request.ExerciseId.String())
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrForbidden):
			return generated.UnlinkExerciseFromContentNode403JSONResponse(forbiddenError("only teachers and admins may unlink exercises from content nodes")), nil
		case errors.Is(err, domain.ErrNotFound):
			return generated.UnlinkExerciseFromContentNode404JSONResponse(notFoundError("no content node exists with content_node_id, no exercise exists with exercise_id, or the exercise is not currently linked to this content node")), nil
		default:
			return nil, err
		}
	}

	return generated.UnlinkExerciseFromContentNode204Response{}, nil
}

func (h *Handler) StartPracticeSession(ctx context.Context, request generated.StartPracticeSessionRequestObject) (generated.StartPracticeSessionResponseObject, error) {
	if _, ok := h.resolveCaller(ctx); !ok {
		return generated.StartPracticeSession401JSONResponse(unauthorizedError()), nil
	}

	count := 10
	if request.Params.Count != nil {
		count = *request.Params.Count
	}

	// SkillId is a required, non-pointer field: the generated chi router
	// rejects a request that omits skill_id before this handler ever runs,
	// but request objects built directly (as this package's own tests do)
	// bypass that layer and land here with the zero uuid.UUID — treated the
	// same as "" so ExerciseService's own validation still catches it.
	var skillID string
	if request.Params.SkillId != (openapi_types.UUID{}) {
		skillID = request.Params.SkillId.String()
	}

	session, err := h.exercise.StartPracticeSession(ctx, skillID, count)
	if err != nil {
		if kind, valErr := classify(err); kind == errKindValidation {
			return generated.StartPracticeSession400JSONResponse(validationErrorResponse(valErr)), nil
		}
		return nil, err
	}

	return generated.StartPracticeSession200JSONResponse(toPracticeSession(session)), nil
}

func (h *Handler) ListSkills(ctx context.Context, _ generated.ListSkillsRequestObject) (generated.ListSkillsResponseObject, error) {
	if _, ok := h.resolveCaller(ctx); !ok {
		return generated.ListSkills401JSONResponse(unauthorizedError()), nil
	}

	skills, err := h.skill.ListSkills(ctx)
	if err != nil {
		return nil, err
	}

	return generated.ListSkills200JSONResponse(toGeneratedSkills(skills)), nil
}

func (h *Handler) CreateSkill(ctx context.Context, request generated.CreateSkillRequestObject) (generated.CreateSkillResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.CreateSkill401JSONResponse(unauthorizedError()), nil
	}

	skill, err := h.skill.CreateSkill(ctx, caller, request.Body.Name, uuidPtrToStringPtr(request.Body.ParentId))
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.CreateSkill400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindForbidden:
			return generated.CreateSkill403JSONResponse(forbiddenError("only teachers and admins may create a skill")), nil
		case errKindNotFound, errKindOther:
			return nil, err
		}
	}

	return generated.CreateSkill201JSONResponse(toGeneratedSkill(skill)), nil
}

func (h *Handler) ListConcepts(ctx context.Context, _ generated.ListConceptsRequestObject) (generated.ListConceptsResponseObject, error) {
	if _, ok := h.resolveCaller(ctx); !ok {
		return generated.ListConcepts401JSONResponse(unauthorizedError()), nil
	}

	concepts, err := h.concept.ListConcepts(ctx)
	if err != nil {
		return nil, err
	}

	return generated.ListConcepts200JSONResponse(toGeneratedConcepts(concepts)), nil
}

func (h *Handler) CreateConcept(ctx context.Context, request generated.CreateConceptRequestObject) (generated.CreateConceptResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.CreateConcept401JSONResponse(unauthorizedError()), nil
	}

	concept, err := h.concept.CreateConcept(ctx, caller, request.Body.Name, uuidPtrToStringPtr(request.Body.ParentId))
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.CreateConcept400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindForbidden:
			return generated.CreateConcept403JSONResponse(forbiddenError("only teachers and admins may create a concept")), nil
		case errKindNotFound, errKindOther:
			return nil, err
		}
	}

	return generated.CreateConcept201JSONResponse(toGeneratedConcept(concept)), nil
}

func (h *Handler) ListInstruments(ctx context.Context, _ generated.ListInstrumentsRequestObject) (generated.ListInstrumentsResponseObject, error) {
	if _, ok := h.resolveCaller(ctx); !ok {
		return generated.ListInstruments401JSONResponse(unauthorizedError()), nil
	}

	instruments, err := h.instrument.ListInstruments(ctx)
	if err != nil {
		return nil, err
	}

	return generated.ListInstruments200JSONResponse(toGeneratedInstruments(instruments)), nil
}

func (h *Handler) CreateInstrument(ctx context.Context, request generated.CreateInstrumentRequestObject) (generated.CreateInstrumentResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.CreateInstrument401JSONResponse(unauthorizedError()), nil
	}

	body := request.Body
	var tuning []string
	if body.Tuning != nil {
		tuning = *body.Tuning
	}
	var keyRange *domain.KeyRange
	if body.KeyRange != nil {
		keyRange = &domain.KeyRange{Lowest: body.KeyRange.Lowest, Highest: body.KeyRange.Highest}
	}
	instrument, err := h.instrument.CreateInstrument(ctx, caller, body.Names, domain.InstrumentFamily(body.Family), body.StringCount, tuning, keyRange)
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.CreateInstrument400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindForbidden:
			return generated.CreateInstrument403JSONResponse(forbiddenError("only teachers and admins may create an instrument")), nil
		case errKindNotFound, errKindOther:
			return nil, err
		}
	}

	return generated.CreateInstrument201JSONResponse(toGeneratedInstrument(instrument)), nil
}

func (h *Handler) UpdateInstrument(ctx context.Context, request generated.UpdateInstrumentRequestObject) (generated.UpdateInstrumentResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.UpdateInstrument401JSONResponse(unauthorizedError()), nil
	}

	instrument, err := h.instrument.UpdateInstrumentNames(ctx, caller, request.InstrumentId.String(), request.Body.Names)
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.UpdateInstrument400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindForbidden:
			return generated.UpdateInstrument403JSONResponse(forbiddenError("only admins may update an instrument")), nil
		case errKindNotFound:
			return generated.UpdateInstrument404JSONResponse(notFoundError("no instrument exists with the given instrument_id")), nil
		case errKindOther:
			return nil, err
		}
	}

	return generated.UpdateInstrument200JSONResponse(toGeneratedInstrument(instrument)), nil
}

func (h *Handler) ListDiagrams(ctx context.Context, request generated.ListDiagramsRequestObject) (generated.ListDiagramsResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.ListDiagrams401JSONResponse(unauthorizedError()), nil
	}

	page, err := domain.NewPageRequest(request.Params.Limit, request.Params.Offset)
	if err != nil {
		return listValidationFailure[generated.ListDiagramsResponseObject](err, func(e generated.ValidationError) generated.ListDiagramsResponseObject {
			return generated.ListDiagrams400JSONResponse(e)
		})
	}

	result, err := h.diagram.ListDiagrams(ctx, caller, diagramListFilter(request.Params), page)
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.ListDiagrams400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindForbidden:
			return generated.ListDiagrams403JSONResponse(forbiddenError("students may not list diagrams, and a teacher may only filter by their own created_by")), nil
		case errKindNotFound, errKindOther:
			return nil, err
		}
	}

	names, err := h.loadUserNames(ctx, diagramUserIDs(result.Items...))
	if err != nil {
		return nil, err
	}
	return generated.ListDiagrams200JSONResponse{
		Items: toGeneratedDiagrams(result.Items, names), Total: result.Total, Limit: page.Limit, Offset: page.Offset,
	}, nil
}

func (h *Handler) CreateDiagram(ctx context.Context, request generated.CreateDiagramRequestObject) (generated.CreateDiagramResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.CreateDiagram401JSONResponse(unauthorizedError()), nil
	}

	body := request.Body
	diagram, err := h.diagram.CreateDiagram(ctx, caller, body.InstrumentId.String(), body.Names, toDomainPositions(body.Positions),
		uuidsToStrings(body.Classification.SkillIds), uuidsToStrings(body.Classification.ConceptIds),
		domain.DiagramOptions{RootNote: body.RootNote, LabelDisplay: toDomainLabelDisplay(body.LabelDisplay), Color: body.Color, Kind: toDomainDiagramKind(body.Kind)})
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.CreateDiagram400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindForbidden:
			return generated.CreateDiagram403JSONResponse(forbiddenError("only teachers and admins may create a diagram, and only admins may create a basic one")), nil
		case errKindNotFound, errKindOther:
			return nil, err
		}
	}

	names, err := h.loadUserNames(ctx, diagramUserIDs(diagram))
	if err != nil {
		return nil, err
	}
	return generated.CreateDiagram201JSONResponse(toGeneratedDiagram(diagram, names)), nil
}

func (h *Handler) GetDiagram(ctx context.Context, request generated.GetDiagramRequestObject) (generated.GetDiagramResponseObject, error) {
	if _, ok := h.resolveCaller(ctx); !ok {
		return generated.GetDiagram401JSONResponse(unauthorizedError()), nil
	}

	diagram, err := h.diagram.GetDiagram(ctx, request.DiagramId.String())
	if err != nil {
		if kind, _ := classify(err); kind == errKindNotFound {
			return generated.GetDiagram404JSONResponse(notFoundError("no diagram exists with the given diagram_id")), nil
		}
		return nil, err
	}

	names, err := h.loadUserNames(ctx, diagramUserIDs(diagram))
	if err != nil {
		return nil, err
	}
	return generated.GetDiagram200JSONResponse(toGeneratedDiagram(diagram, names)), nil
}

func (h *Handler) UpdateDiagram(ctx context.Context, request generated.UpdateDiagramRequestObject) (generated.UpdateDiagramResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.UpdateDiagram401JSONResponse(unauthorizedError()), nil
	}

	body := request.Body
	update := application.DiagramUpdate{RootNote: body.RootNote, Color: body.Color}
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

	diagram, err := h.diagram.UpdateDiagram(ctx, caller, request.DiagramId.String(), update)
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.UpdateDiagram400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindForbidden:
			return generated.UpdateDiagram403JSONResponse(forbiddenError("only admins may update a basic diagram, and only its creator or an admin may update a custom one")), nil
		case errKindNotFound:
			return generated.UpdateDiagram404JSONResponse(notFoundError("no diagram exists with the given diagram_id")), nil
		case errKindOther:
			return nil, err
		}
	}

	names, err := h.loadUserNames(ctx, diagramUserIDs(diagram))
	if err != nil {
		return nil, err
	}
	return generated.UpdateDiagram200JSONResponse(toGeneratedDiagram(diagram, names)), nil
}
