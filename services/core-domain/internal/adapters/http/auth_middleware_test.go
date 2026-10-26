package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

// TestSessionCustomClaims_Name covers how the "name" claim is read from a
// session token payload. Decoding custom claims is part of verifying the
// token, so a claim that fails to decode would reject the whole token — a
// misconfigured name must only ever cost the name, never the login.
func TestSessionCustomClaims_Name(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    string
	}{
		{name: "a string name is read", payload: `{"name":"Ana Souza"}`, want: "Ana Souza"},
		{name: "a null name reads as no name", payload: `{"name":null}`, want: ""},
		{name: "a missing name reads as no name", payload: `{}`, want: ""},
		{name: "a number is ignored, not a decode failure", payload: `{"name":42}`, want: ""},
		{name: "an object is ignored, not a decode failure", payload: `{"name":{"first":"Ana"}}`, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var claims sessionCustomClaims

			err := json.Unmarshal([]byte(tt.payload), &claims)

			require.NoError(t, err)
			assert.Equal(t, tt.want, claims.Name)
		})
	}
}

// TestSessionCustomClaims_Email covers the "email" claim the same way: a
// misconfigured address must only ever cost the address, never the login,
// and must not disturb the name read beside it.
func TestSessionCustomClaims_Email(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		want     string
		wantName string
	}{
		{name: "a string email is read beside the name", payload: `{"name":"Ana Souza","email":"ana@example.com"}`, want: "ana@example.com", wantName: "Ana Souza"},
		{name: "a missing email reads as no email", payload: `{"name":"Ana Souza"}`, want: "", wantName: "Ana Souza"},
		{name: "a null email reads as no email", payload: `{"email":null}`, want: ""},
		{name: "a list is ignored, not a decode failure", payload: `{"name":"Ana Souza","email":["ana@example.com"]}`, want: "", wantName: "Ana Souza"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var claims sessionCustomClaims

			err := json.Unmarshal([]byte(tt.payload), &claims)

			require.NoError(t, err)
			assert.Equal(t, tt.want, claims.Email)
			assert.Equal(t, tt.wantName, claims.Name)
		})
	}
}

// TestClerkAuthMiddleware_PassesIdentityClaims follows a session token's
// payload, decoded the way the SDK decodes it, through ClerkAuthMiddleware
// to what handlers read from the request context. Registration rejects a
// request without an email, so losing the claim on this path would turn
// away every new user while every handler-level test still passes.
func TestClerkAuthMiddleware_PassesIdentityClaims(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    domain.IdentityClaims
	}{
		{name: "the name and the email reach the handler", payload: `{"name":"Ana Souza","email":"ana@example.com"}`, want: domain.IdentityClaims{Name: "Ana Souza", Email: "ana@example.com"}},
		{name: "a token without an email reaches the handler without one", payload: `{"name":"Ana Souza"}`, want: domain.IdentityClaims{Name: "Ana Souza"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var custom sessionCustomClaims
			require.NoError(t, json.Unmarshal([]byte(tt.payload), &custom))
			verified := &clerk.SessionClaims{RegisteredClaims: clerk.RegisteredClaims{Subject: "user_ana"}, Custom: &custom}

			var gotSubject string
			var gotClaims domain.IdentityClaims
			handler := ClerkAuthMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				gotSubject, _ = ClerkUserIDFromContext(r.Context())
				gotClaims = IdentityClaimsFromContext(r.Context())
			}))
			// The SDK has already verified the token and stored its claims;
			// without an Authorization header it passes the request on as is.
			request := httptest.NewRequest(http.MethodGet, "/users/me", nil).
				WithContext(clerk.ContextWithSessionClaims(context.Background(), verified))

			handler.ServeHTTP(httptest.NewRecorder(), request)

			assert.Equal(t, "user_ana", gotSubject)
			assert.Equal(t, tt.want, gotClaims)
		})
	}
}
