package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
)

// errOperationNotBuilt marks an operation the API lists but this service
// doesn't serve yet. It answers 501 and must not touch any application
// service first, so the request changes and records nothing.
var errOperationNotBuilt = errors.New("this operation isn't available yet")

// NewStrictHandler adapts a Handler to the generated router. It keeps the
// generated defaults (400 for a request that can't be read, 500 for any other
// handler error) and answers 501 for an operation that isn't built yet.
func NewStrictHandler(h *Handler) generated.ServerInterface {
	return generated.NewStrictHandlerWithOptions(h, nil, generated.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		},
		ResponseErrorHandlerFunc: writeResponseError,
	})
}

func writeResponseError(w http.ResponseWriter, _ *http.Request, err error) {
	if errors.Is(err, errOperationNotBuilt) {
		writeNotBuilt(w)
		return
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func writeNotBuilt(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	// The status is already sent, so a failed write has nowhere to be reported.
	_ = json.NewEncoder(w).Encode(struct {
		Message string `json:"message"`
	}{Message: errOperationNotBuilt.Error()})
}
