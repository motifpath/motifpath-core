package http

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
