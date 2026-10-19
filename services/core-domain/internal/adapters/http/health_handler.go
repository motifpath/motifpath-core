package http

import (
	"context"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/ports"
)

// Readiness check names. The probe reports one entry per dependency in the
// HealthStatus.checks map; a deployment platform routing on /readyz reads
// these to see which store is down.
const (
	checkLearningGraph   = "learning_graph"
	checkCompletionState = "completion_state"
)

// LivenessCheck always reports ok: it confirms the process is running, nothing
// more. It deliberately does not check downstream dependencies — that is what
// ReadinessCheck is for.
func (h *Handler) LivenessCheck(_ context.Context, _ generated.LivenessCheckRequestObject) (generated.LivenessCheckResponseObject, error) {
	return generated.LivenessCheck200JSONResponse{Status: generated.HealthStatusStatusOk}, nil
}

// ReadinessCheck pings the learning-graph store (Postgres) and the
// completion-state store (MongoDB) and reports per-dependency status. The
// service is ready only when both are reachable.
func (h *Handler) ReadinessCheck(ctx context.Context, _ generated.ReadinessCheckRequestObject) (generated.ReadinessCheckResponseObject, error) {
	checks := map[string]generated.HealthStatusChecks{
		checkLearningGraph:   pingResult(ctx, h.learningGraphPinger),
		checkCompletionState: pingResult(ctx, h.completionStatePinger),
	}

	for _, result := range checks {
		if result == generated.HealthStatusChecksFail {
			return generated.ReadinessCheck503JSONResponse{Status: generated.HealthStatusStatusDegraded, Checks: &checks}, nil
		}
	}
	return generated.ReadinessCheck200JSONResponse{Status: generated.HealthStatusStatusOk, Checks: &checks}, nil
}

func pingResult(ctx context.Context, p ports.Pinger) generated.HealthStatusChecks {
	if p.Ping(ctx) != nil {
		return generated.HealthStatusChecksFail
	}
	return generated.HealthStatusChecksOk
}
