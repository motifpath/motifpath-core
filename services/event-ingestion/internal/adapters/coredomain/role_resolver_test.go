package coredomain_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/event-ingestion/internal/adapters/coredomain"
	"github.com/motifpath/event-ingestion/internal/ports"
)

func TestRoleResolver_ReturnsRoleFromProfile(t *testing.T) {
	var gotAuth, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user_id":"u-1","role":"admin","registered_at":"2026-08-01T00:00:00Z"}`))
	}))
	defer server.Close()

	resolver := coredomain.NewRoleResolver(server.URL, nil)
	role, err := resolver.ResolveRole(context.Background(), "token-abc")

	require.NoError(t, err)
	assert.Equal(t, "admin", role)
	assert.Equal(t, "Bearer token-abc", gotAuth)
	assert.Equal(t, "/users/me", gotPath)
}

func TestRoleResolver_TrimsTrailingSlashOnBaseURL(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"role":"teacher"}`))
	}))
	defer server.Close()

	resolver := coredomain.NewRoleResolver(server.URL+"/", nil)
	_, err := resolver.ResolveRole(context.Background(), "token-abc")

	require.NoError(t, err)
	assert.Equal(t, "/users/me", gotPath)
}

func TestRoleResolver_NotFoundMeansIdentityNotRegistered(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"no user record"}`))
	}))
	defer server.Close()

	resolver := coredomain.NewRoleResolver(server.URL, nil)
	_, err := resolver.ResolveRole(context.Background(), "token-abc")

	require.ErrorIs(t, err, ports.ErrIdentityNotRegistered)
}

func TestRoleResolver_UnexpectedStatusMeansRoleUnavailable(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusInternalServerError, http.StatusBadGateway} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
			}))
			defer server.Close()

			resolver := coredomain.NewRoleResolver(server.URL, nil)
			_, err := resolver.ResolveRole(context.Background(), "token-abc")

			require.ErrorIs(t, err, ports.ErrRoleUnavailable)
		})
	}
}

func TestRoleResolver_MissingRoleInResponseMeansRoleUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"user_id":"u-1"}`))
	}))
	defer server.Close()

	resolver := coredomain.NewRoleResolver(server.URL, nil)
	_, err := resolver.ResolveRole(context.Background(), "token-abc")

	require.ErrorIs(t, err, ports.ErrRoleUnavailable)
}

func TestRoleResolver_MalformedJSONMeansRoleUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	resolver := coredomain.NewRoleResolver(server.URL, nil)
	_, err := resolver.ResolveRole(context.Background(), "token-abc")

	require.ErrorIs(t, err, ports.ErrRoleUnavailable)
}

func TestRoleResolver_TransportErrorMeansRoleUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	serverURL := server.URL
	server.Close() // nothing is listening now

	resolver := coredomain.NewRoleResolver(serverURL, nil)
	_, err := resolver.ResolveRole(context.Background(), "token-abc")

	require.ErrorIs(t, err, ports.ErrRoleUnavailable)
}

func TestRoleResolver_RespectsContextCancellation(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		_, _ = w.Write([]byte(`{"role":"admin"}`))
	}))
	defer server.Close()
	defer close(release)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	resolver := coredomain.NewRoleResolver(server.URL, nil)
	_, err := resolver.ResolveRole(ctx, "token-abc")

	require.ErrorIs(t, err, ports.ErrRoleUnavailable)
}
