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

func TestClient_ReturnsProfileFromUsersMe(t *testing.T) {
	var gotAuth, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user_id":"u-1","role":"admin","registered_at":"2026-08-01T00:00:00Z"}`))
	}))
	defer server.Close()

	client := coredomain.NewClient(server.URL, nil)
	profile, err := client.ResolveProfile(context.Background(), "token-abc")

	require.NoError(t, err)
	assert.Equal(t, ports.Profile{UserID: "u-1", Role: "admin"}, profile)
	assert.Equal(t, "Bearer token-abc", gotAuth)
	assert.Equal(t, "/users/me", gotPath)
}

func TestClient_TrimsTrailingSlashOnBaseURL(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"user_id":"u-1","role":"teacher"}`))
	}))
	defer server.Close()

	client := coredomain.NewClient(server.URL+"/", nil)
	_, err := client.ResolveProfile(context.Background(), "token-abc")

	require.NoError(t, err)
	assert.Equal(t, "/users/me", gotPath)
}

func TestClient_NotFoundMeansIdentityNotRegistered(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"no user record"}`))
	}))
	defer server.Close()

	client := coredomain.NewClient(server.URL, nil)
	_, err := client.ResolveProfile(context.Background(), "token-abc")

	require.ErrorIs(t, err, ports.ErrIdentityNotRegistered)
}

func TestClient_UnexpectedStatusMeansProfileUnavailable(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusInternalServerError, http.StatusBadGateway} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
			}))
			defer server.Close()

			client := coredomain.NewClient(server.URL, nil)
			_, err := client.ResolveProfile(context.Background(), "token-abc")

			require.ErrorIs(t, err, ports.ErrProfileUnavailable)
		})
	}
}

func TestClient_IncompleteProfileMeansProfileUnavailable(t *testing.T) {
	for _, body := range []string{`{"user_id":"u-1"}`, `{"role":"admin"}`, `{}`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()

			client := coredomain.NewClient(server.URL, nil)
			_, err := client.ResolveProfile(context.Background(), "token-abc")

			require.ErrorIs(t, err, ports.ErrProfileUnavailable)
		})
	}
}

func TestClient_MalformedJSONMeansProfileUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	client := coredomain.NewClient(server.URL, nil)
	_, err := client.ResolveProfile(context.Background(), "token-abc")

	require.ErrorIs(t, err, ports.ErrProfileUnavailable)
}

func TestClient_TransportErrorMeansProfileUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	serverURL := server.URL
	server.Close() // nothing is listening now

	client := coredomain.NewClient(serverURL, nil)
	_, err := client.ResolveProfile(context.Background(), "token-abc")

	require.ErrorIs(t, err, ports.ErrProfileUnavailable)
}

func TestClient_RespectsContextCancellation(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		_, _ = w.Write([]byte(`{"user_id":"u-1","role":"admin"}`))
	}))
	defer server.Close()
	defer close(release)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	client := coredomain.NewClient(server.URL, nil)
	_, err := client.ResolveProfile(ctx, "token-abc")

	require.ErrorIs(t, err, ports.ErrProfileUnavailable)
}
