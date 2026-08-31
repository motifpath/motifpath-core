package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	appHTTP "github.com/motifpath/core-domain/internal/adapters/http"
)

const webOrigin = "http://localhost:5173"

func newCORSHandler() (http.Handler, *bool) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	return appHTTP.NewCORSMiddleware([]string{webOrigin})(next), &called
}

func TestCORSMiddleware(t *testing.T) {
	t.Run("answers a preflight from an allowed origin without invoking the handler", func(t *testing.T) {
		handler, called := newCORSHandler()
		req := httptest.NewRequest(http.MethodOptions, "/users/me", nil)
		req.Header.Set("Origin", webOrigin)
		req.Header.Set("Access-Control-Request-Method", http.MethodGet)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)
		assert.False(t, *called)
		assert.Equal(t, webOrigin, rec.Header().Get("Access-Control-Allow-Origin"))
		assert.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), "Authorization")
	})

	t.Run("adds CORS headers to an actual request from an allowed origin", func(t *testing.T) {
		handler, called := newCORSHandler()
		req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
		req.Header.Set("Origin", webOrigin)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.True(t, *called)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, webOrigin, rec.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "Origin", rec.Header().Get("Vary"))
	})

	t.Run("does not add CORS headers for a disallowed origin", func(t *testing.T) {
		handler, called := newCORSHandler()
		req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
		req.Header.Set("Origin", "https://evil.example")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.True(t, *called)
		assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("passes through a request with no Origin header", func(t *testing.T) {
		handler, called := newCORSHandler()
		req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.True(t, *called)
		assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("does not short-circuit an OPTIONS request from a disallowed origin", func(t *testing.T) {
		handler, called := newCORSHandler()
		req := httptest.NewRequest(http.MethodOptions, "/users/me", nil)
		req.Header.Set("Origin", "https://evil.example")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.True(t, *called)
	})
}

func TestParseAllowedOrigins(t *testing.T) {
	t.Run("splits and trims a comma-separated list", func(t *testing.T) {
		assert.Equal(t,
			[]string{"http://localhost:5173", "https://app.motifpath.io"},
			appHTTP.ParseAllowedOrigins("http://localhost:5173, https://app.motifpath.io"),
		)
	})

	t.Run("drops empty entries", func(t *testing.T) {
		assert.Equal(t, []string{"http://localhost:5173"}, appHTTP.ParseAllowedOrigins("http://localhost:5173,,"))
	})

	t.Run("returns an empty slice for a blank value", func(t *testing.T) {
		assert.Empty(t, appHTTP.ParseAllowedOrigins("  "))
	})
}
