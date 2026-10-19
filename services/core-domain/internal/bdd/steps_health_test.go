//go:build integration

package bdd

import (
	"context"
	"fmt"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
)

func registerHealthSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the Core Domain Service process is running$`, func() error { return nil })

	sc.Step(`^the learning-graph store is (reachable|unreachable)$`, w.learningGraphStoreIs)
	sc.Step(`^the completion-state store is (reachable|unreachable)$`, w.completionStateStoreIs)
	sc.Step(`^the learning-graph store becomes reachable again$`, func() error {
		w.pgPinger.setReachable(true)
		return nil
	})

	sc.Step(`^the liveness probe is checked$`, w.checkLiveness)
	sc.Step(`^the readiness probe is checked$`, w.checkReadiness)
	sc.Step(`^the readiness probe reports the service as not ready$`, func() error {
		if err := w.checkReadiness(); err != nil {
			return err
		}
		return w.serviceReportsNotReady()
	})

	sc.Step(`^the service reports itself as alive$`, w.serviceReportsAlive)
	sc.Step(`^the service reports itself as ready$`, w.serviceReportsReady)
	sc.Step(`^the service reports itself as not ready$`, w.serviceReportsNotReady)
	sc.Step(`^all dependency checks report "([^"]+)"$`, w.allDependencyChecksReport)
	sc.Step(`^the "([^"]+)" dependency check reports "([^"]+)"$`, w.dependencyCheckReports)
}

func (w *world) learningGraphStoreIs(state string) error {
	w.pgPinger.setReachable(state == "reachable")
	return nil
}

func (w *world) completionStateStoreIs(state string) error {
	w.mongoPinger.setReachable(state == "reachable")
	return nil
}

func (w *world) checkLiveness() error {
	resp, err := w.handler.LivenessCheck(context.Background(), generated.LivenessCheckRequestObject{})
	if err != nil {
		return err
	}
	w.livenessResp = resp
	return nil
}

func (w *world) checkReadiness() error {
	resp, err := w.handler.ReadinessCheck(context.Background(), generated.ReadinessCheckRequestObject{})
	if err != nil {
		return err
	}
	w.readinessResp = resp
	return nil
}

func (w *world) serviceReportsAlive() error {
	resp, ok := w.livenessResp.(generated.LivenessCheck200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 liveness response, got %#v", w.livenessResp)
	}
	if resp.Status != generated.HealthStatusStatusOk {
		return fmt.Errorf("expected status ok, got %q", resp.Status)
	}
	return nil
}

func (w *world) serviceReportsReady() error {
	if _, ok := w.readinessResp.(generated.ReadinessCheck200JSONResponse); !ok {
		return fmt.Errorf("expected a 200 readiness response, got %#v", w.readinessResp)
	}
	return nil
}

func (w *world) serviceReportsNotReady() error {
	if _, ok := w.readinessResp.(generated.ReadinessCheck503JSONResponse); !ok {
		return fmt.Errorf("expected a 503 readiness response, got %#v", w.readinessResp)
	}
	return nil
}

func (w *world) readinessChecks() (map[string]generated.HealthStatusChecks, error) {
	switch resp := w.readinessResp.(type) {
	case generated.ReadinessCheck200JSONResponse:
		if resp.Checks == nil {
			return nil, fmt.Errorf("readiness response had no checks map")
		}
		return *resp.Checks, nil
	case generated.ReadinessCheck503JSONResponse:
		if resp.Checks == nil {
			return nil, fmt.Errorf("readiness response had no checks map")
		}
		return *resp.Checks, nil
	default:
		return nil, fmt.Errorf("unexpected readiness response type %#v", w.readinessResp)
	}
}

func (w *world) allDependencyChecksReport(want string) error {
	checks, err := w.readinessChecks()
	if err != nil {
		return err
	}
	if len(checks) == 0 {
		return fmt.Errorf("readiness response reported no dependency checks")
	}
	for name, status := range checks {
		if string(status) != want {
			return fmt.Errorf("dependency %q reported %q, want %q", name, status, want)
		}
	}
	return nil
}

func (w *world) dependencyCheckReports(name, want string) error {
	checks, err := w.readinessChecks()
	if err != nil {
		return err
	}
	got, ok := checks[name]
	if !ok {
		return fmt.Errorf("no check reported for dependency %q; got %+v", name, checks)
	}
	if string(got) != want {
		return fmt.Errorf("dependency %q reported %q, want %q", name, got, want)
	}
	return nil
}
