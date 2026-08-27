package http

import (
	"errors"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func unauthorizedError() generated.UnauthorizedError {
	return generated.UnauthorizedError{Message: "missing or invalid bearer token"}
}

func forbiddenError(message string) generated.ForbiddenError {
	return generated.ForbiddenError{Message: message}
}

func notFoundError(message string) generated.NotFoundError {
	return generated.NotFoundError{Message: message}
}

func conflictError(message string) generated.ConflictError {
	return generated.ConflictError{Message: message}
}

func validationErrorResponse(err *domain.ValidationError) generated.ValidationError {
	entries := make([]struct {
		Field  string `json:"field"`
		Reason string `json:"reason"`
	}, len(err.Fields))
	for i, f := range err.Fields {
		entries[i] = struct {
			Field  string `json:"field"`
			Reason string `json:"reason"`
		}{Field: f.Field, Reason: f.Reason}
	}
	return generated.ValidationError{Message: "request failed validation", Errors: entries}
}

type errKind int

const (
	errKindOther errKind = iota
	errKindValidation
	errKindNotFound
	errKindForbidden
)

// classify maps a domain error to the HTTP-relevant category the caller
// needs in order to pick the right generated response type for its
// operation. errKindOther means "not a domain sentinel handled specially
// here" — the caller propagates it as a 500 by returning (nil, err).
func classify(err error) (errKind, *domain.ValidationError) {
	var valErr *domain.ValidationError
	switch {
	case errors.As(err, &valErr):
		return errKindValidation, valErr
	case errors.Is(err, domain.ErrNotFound):
		return errKindNotFound, nil
	case errors.Is(err, domain.ErrForbidden):
		return errKindForbidden, nil
	default:
		return errKindOther, nil
	}
}
