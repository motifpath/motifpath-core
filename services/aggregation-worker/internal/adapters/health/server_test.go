package health_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/aggregation-worker/internal/adapters/health"
)

type stubPinger struct{ err error }

func (p stubPinger) Ping(context.Context) error { return p.err }

var errDown = errors.New("dependency unreachable")

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func checks(mongoErr, kafkaErr error) []health.Check {
	return []health.Check{
		{Name: "mongodb", Pinger: stubPinger{mongoErr}},
		{Name: "kafka_broker", Pinger: stubPinger{kafkaErr}},
	}
}

func do(t *testing.T, h http.Handler, path string) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return rec.Code, body
}

func TestLiveness_alwaysOK(t *testing.T) {
	h := health.Handler(checks(errDown, errDown), discardLogger())

	code, body := do(t, h, "/healthz")

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "ok", body["status"])
	assert.NotContains(t, body, "checks", "liveness carries no dependency checks")
}

func TestReadiness(t *testing.T) {
	tests := []struct {
		name       string
		mongoErr   error
		kafkaErr   error
		wantCode   int
		wantStatus string
		wantChecks map[string]any
	}{
		{
			name:       "ready when both dependencies are reachable",
			wantCode:   http.StatusOK,
			wantStatus: "ok",
			wantChecks: map[string]any{"mongodb": "ok", "kafka_broker": "ok"},
		},
		{
			name:       "not ready when MongoDB is unreachable",
			mongoErr:   errDown,
			wantCode:   http.StatusServiceUnavailable,
			wantStatus: "degraded",
			wantChecks: map[string]any{"mongodb": "fail", "kafka_broker": "ok"},
		},
		{
			name:       "not ready when the broker is unreachable",
			kafkaErr:   errDown,
			wantCode:   http.StatusServiceUnavailable,
			wantStatus: "degraded",
			wantChecks: map[string]any{"mongodb": "ok", "kafka_broker": "fail"},
		},
		{
			name:       "not ready when both are unreachable",
			mongoErr:   errDown,
			kafkaErr:   errDown,
			wantCode:   http.StatusServiceUnavailable,
			wantStatus: "degraded",
			wantChecks: map[string]any{"mongodb": "fail", "kafka_broker": "fail"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := health.Handler(checks(tc.mongoErr, tc.kafkaErr), discardLogger())

			code, body := do(t, h, "/readyz")

			assert.Equal(t, tc.wantCode, code)
			assert.Equal(t, tc.wantStatus, body["status"])
			assert.Equal(t, tc.wantChecks, body["checks"])
		})
	}
}

func TestNewServer_configuresAddrAndTimeout(t *testing.T) {
	srv := health.NewServer(":8082", nil, discardLogger())

	assert.Equal(t, ":8082", srv.Addr)
	assert.Positive(t, srv.ReadHeaderTimeout, "ReadHeaderTimeout guards against slow-loris")
}
