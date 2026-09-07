package http_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appHTTP "github.com/motifpath/core-domain/internal/adapters/http"
	"github.com/motifpath/core-domain/internal/adapters/http/generated"
)

// stubPinger is a ports.Pinger whose Ping result is fixed per test row.
type stubPinger struct{ err error }

func (p stubPinger) Ping(context.Context) error { return p.err }

var errUnreachable = errors.New("dependency unreachable")

// healthHandler builds a Handler with only the two pingers wired — the health
// probes never touch the application services, so nil is safe for those.
func healthHandler(pg, mongo error) *appHTTP.Handler {
	return appHTTP.NewHandler(nil, nil, nil, nil, nil, stubPinger{pg}, stubPinger{mongo})
}

func TestLivenessCheck_alwaysReportsOK(t *testing.T) {
	resp, err := healthHandler(errUnreachable, errUnreachable).
		LivenessCheck(context.Background(), generated.LivenessCheckRequestObject{})
	require.NoError(t, err)

	alive, ok := resp.(generated.LivenessCheck200JSONResponse)
	require.True(t, ok, "expected a 200 liveness response, got %#v", resp)
	assert.Equal(t, generated.HealthStatusStatusOk, alive.Status)
	assert.Nil(t, alive.Checks, "liveness carries no dependency checks")
}

func TestReadinessCheck(t *testing.T) {
	tests := []struct {
		name       string
		pgErr      error
		mongoErr   error
		wantReady  bool
		wantChecks map[string]generated.HealthStatusChecks
		wantStatus generated.HealthStatusStatus
	}{
		{
			name:       "ready when both stores are reachable",
			wantReady:  true,
			wantStatus: generated.HealthStatusStatusOk,
			wantChecks: map[string]generated.HealthStatusChecks{
				"learning_graph":   generated.HealthStatusChecksOk,
				"completion_state": generated.HealthStatusChecksOk,
			},
		},
		{
			name:       "not ready when the learning-graph store is unreachable",
			pgErr:      errUnreachable,
			wantReady:  false,
			wantStatus: generated.HealthStatusStatusDegraded,
			wantChecks: map[string]generated.HealthStatusChecks{
				"learning_graph":   generated.HealthStatusChecksFail,
				"completion_state": generated.HealthStatusChecksOk,
			},
		},
		{
			name:       "not ready when the completion-state store is unreachable",
			mongoErr:   errUnreachable,
			wantReady:  false,
			wantStatus: generated.HealthStatusStatusDegraded,
			wantChecks: map[string]generated.HealthStatusChecks{
				"learning_graph":   generated.HealthStatusChecksOk,
				"completion_state": generated.HealthStatusChecksFail,
			},
		},
		{
			name:       "not ready when both stores are unreachable",
			pgErr:      errUnreachable,
			mongoErr:   errUnreachable,
			wantReady:  false,
			wantStatus: generated.HealthStatusStatusDegraded,
			wantChecks: map[string]generated.HealthStatusChecks{
				"learning_graph":   generated.HealthStatusChecksFail,
				"completion_state": generated.HealthStatusChecksFail,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := healthHandler(tc.pgErr, tc.mongoErr).
				ReadinessCheck(context.Background(), generated.ReadinessCheckRequestObject{})
			require.NoError(t, err)

			var status generated.HealthStatusStatus
			var checks *map[string]generated.HealthStatusChecks
			switch r := resp.(type) {
			case generated.ReadinessCheck200JSONResponse:
				assert.True(t, tc.wantReady, "got 200 but expected not-ready")
				status, checks = r.Status, r.Checks
			case generated.ReadinessCheck503JSONResponse:
				assert.False(t, tc.wantReady, "got 503 but expected ready")
				status, checks = r.Status, r.Checks
			default:
				t.Fatalf("unexpected readiness response type %#v", resp)
			}

			assert.Equal(t, tc.wantStatus, status)
			require.NotNil(t, checks)
			assert.Equal(t, tc.wantChecks, *checks)
		})
	}
}
