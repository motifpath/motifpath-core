package http

import (
	"net/http"

	"github.com/motifpath/core-domain/internal/domain"
)

// AcceptLanguageMiddleware normalizes the request's Accept-Language header
// (see domain.NormalizeAcceptLanguage) and attaches the candidate to the
// request context, for Handler.RegisterUser to use when resolving a new
// user's initial locale. See ADR-024.
func AcceptLanguageMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		candidate := domain.NormalizeAcceptLanguage(r.Header.Get("Accept-Language"))
		next.ServeHTTP(w, r.WithContext(WithAcceptLanguageCandidate(r.Context(), candidate)))
	})
}
