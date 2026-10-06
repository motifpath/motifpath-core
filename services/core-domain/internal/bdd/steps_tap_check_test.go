//go:build integration

package bdd

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/domain"
)

// fakeTapChecks is an in-memory ports.TapCheckReader holding each
// student's newest tap check.
type fakeTapChecks struct {
	mu   sync.Mutex
	last map[string]time.Time
}

func newFakeTapChecks() *fakeTapChecks {
	return &fakeTapChecks{last: map[string]time.Time{}}
}

func (f *fakeTapChecks) LastTapCheck(_ context.Context, studentID string) (time.Time, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	at, ok := f.last[studentID]
	return at, ok, nil
}

func registerTapCheckSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)" is enrolled in a path with the skill "([^"]+)"$`, w.enrolledInPathWithSkill)
	sc.Step(`^"([^"]+)" has never done a tap check$`, w.neverDidTapCheck)
	sc.Step(`^"([^"]+)" did a tap check (\d+) days ago$`, w.didTapCheckDaysAgo)
	sc.Step(`^"([^"]+)"'s next session will practise only play-alongs and exercises$`, w.nextSessionHasNoCells)

	sc.Step(`^the plan asks for a tap check$`, func() error { return w.planAsksForTapCheck(true) })
	sc.Step(`^the plan doesn't ask for a tap check$`, func() error { return w.planAsksForTapCheck(false) })
}

// enrolledInPathWithSkill gives name a guitar path teaching skill, with
// guitar cells on its root strings.
func (w *world) enrolledInPathWithSkill(name, skill string) error {
	w.addPathSkill(name, skill)
	w.paths.put(domain.LearningPath{
		ID:            pathID("practice-path-" + skill).String(),
		InstrumentIDs: []string{instrumentID("guitar").String()},
	})
	w.tapCheckSkill = skill
	w.putCells("guitar", skill, 12, 6, 5)
	return nil
}

func (w *world) neverDidTapCheck(name string) error {
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	if _, found, _ := w.tapChecks.LastTapCheck(context.Background(), studentID); found {
		return fmt.Errorf("expected %q to have no tap check", name)
	}
	return nil
}

func (w *world) didTapCheckDaysAgo(name string, days int) error {
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	w.tapChecks.mu.Lock()
	defer w.tapChecks.mu.Unlock()
	w.tapChecks.last[studentID] = fixedNow.AddDate(0, 0, -days)
	return nil
}

// nextSessionHasNoCells drops the guitar cells and gives the path's skill a
// play-along and exercises on guitar instead.
func (w *world) nextSessionHasNoCells(string) error {
	delete(w.practiceItems.cells, instrumentID("guitar").String())
	w.putGuitarExercises(w.tapCheckSkill, w.tapCheckSkill, 4)
	w.putPlayAlongOn("tap-check-lick", "guitar", w.tapCheckSkill, 100)
	return nil
}

func (w *world) planAsksForTapCheck(want bool) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	if plan.TapCheckDue != want {
		return fmt.Errorf("expected tap_check_due %t, got %t (%d items)", want, plan.TapCheckDue, len(plan.Items))
	}
	return nil
}
