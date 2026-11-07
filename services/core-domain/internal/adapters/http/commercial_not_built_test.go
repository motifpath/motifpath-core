package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appHTTP "github.com/motifpath/core-domain/internal/adapters/http"
	"github.com/motifpath/core-domain/internal/adapters/http/generated"
)

// commercialRouter serves the real generated routes over a Handler with no
// application services wired: an operation that answers without touching one
// can't have changed or recorded anything.
func commercialRouter() http.Handler {
	handler := appHTTP.NewHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	return generated.HandlerWithOptions(appHTTP.NewStrictHandler(handler), generated.ChiServerOptions{})
}

func TestCommercialOperationsNotYetBuilt_answer501WithAMessage(t *testing.T) {
	const anyID = "0b6f3c1e-8d7a-4e52-9a3f-2c4d5e6f7a8b"
	tests := []struct {
		name    string
		method  string
		path    string
		headers map[string]string
		body    string
	}{
		{name: "a learner lists their subscriptions", method: http.MethodGet, path: "/users/me/subscriptions"},
		{name: "a learner lists their holdings", method: http.MethodGet, path: "/users/me/commercial-holdings"},
		{name: "a caller reads a refund request", method: http.MethodGet, path: "/refund-requests/" + anyID},
		{
			name:    "Stripe delivers a payment event",
			method:  http.MethodPost,
			path:    "/payments/stripe/webhook",
			headers: map[string]string{"Stripe-Signature": "t=1,v1=abc"},
			body:    `{}`,
		},
		{
			name:    "an admin turns every commercial flag off",
			method:  http.MethodPost,
			path:    "/admin/commercial-rollout/kill-switch",
			headers: map[string]string{"Idempotency-Key": anyID},
			body:    `{}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()

			commercialRouter().ServeHTTP(rec, req)

			require.Equal(t, http.StatusNotImplemented, rec.Code, rec.Body.String())
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
			var body struct {
				Message string `json:"message"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.NotEmpty(t, body.Message)
		})
	}
}

func TestCommercialOperationsNotYetBuilt_stillRefuseAnUnreadableRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/refund-requests/not-a-uuid", nil)
	rec := httptest.NewRecorder()

	commercialRouter().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
