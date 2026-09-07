// Package health exposes the aggregation worker's liveness and readiness
// probes over HTTP. The worker is otherwise a pure Kafka consumer with no
// HTTP surface (ADR-011); this server is the minimal addition every PB-8a
// hosting model and the release-image smoke need (ADR-016).
package health

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/motifpath/aggregation-worker/internal/ports"
)

// readHeaderTimeout guards the probe listener against a slow-loris client.
const readHeaderTimeout = 5 * time.Second

// Check pairs a dependency name with its pinger. The name is the key the
// readiness probe reports it under in the checks map.
type Check struct {
	Name   string
	Pinger ports.Pinger
}

// statusBody is the JSON contract shared with the other services' probes:
// status is "ok" or "degraded"; checks maps each dependency to "ok" or
// "fail" and is omitted on the liveness probe.
type statusBody struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

// NewServer builds the probe HTTP server bound to addr.
func NewServer(addr string, checks []Check, logger *slog.Logger) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           Handler(checks, logger),
		ReadHeaderTimeout: readHeaderTimeout,
	}
}

// Handler routes GET /healthz (liveness) and GET /readyz (readiness). It is
// exported so tests can exercise it without binding a port.
func Handler(checks []Check, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, logger, http.StatusOK, statusBody{Status: "ok"})
	})

	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		results := make(map[string]string, len(checks))
		ready := true
		for _, c := range checks {
			if c.Pinger.Ping(r.Context()) != nil {
				results[c.Name] = "fail"
				ready = false
			} else {
				results[c.Name] = "ok"
			}
		}

		if ready {
			writeJSON(w, logger, http.StatusOK, statusBody{Status: "ok", Checks: results})
			return
		}
		writeJSON(w, logger, http.StatusServiceUnavailable, statusBody{Status: "degraded", Checks: results})
	})

	return mux
}

func writeJSON(w http.ResponseWriter, logger *slog.Logger, status int, body statusBody) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		logger.Error("failed to write health response", "error", err)
	}
}
