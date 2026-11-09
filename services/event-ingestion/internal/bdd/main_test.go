//go:build integration

package bdd

import (
	"os"
	"testing"

	"github.com/cucumber/godog"
)

// featuresPath is relative to this package's directory, which is where `go test`
// sets the working directory — not the repo root the Makefile runs from.
//
// MOTIFPATH_SPECS_DIR overrides the default location of the motifpath-specs
// checkout — needed when running against a specs worktree, where the sibling
// checkout is not at that relative path.
var featuresPath = func() string {
	if dir := os.Getenv("MOTIFPATH_SPECS_DIR"); dir != "" {
		return dir + "/features/event-ingestion"
	}
	return "../../../../../motifpath-specs/features/event-ingestion"
}()

func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{featuresPath},
			TestingT: t,
			// Strict fails the suite on an undefined step instead of warning and
			// exiting 0, so a scenario merged to motifpath-specs without a step
			// definition here cannot pass CI silently. Scenarios not implemented
			// here yet are skipped by tag (bddTags). Mirrors core-domain's runner.
			Strict: true,
			Tags:   bddTags(t),
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}

// bddTags is the tag expression `make test:bdd` builds: it skips @wip
// (parked), @web (client behaviour, pinned by motifpath-web's own tests) and
// the scenarios of every ticket motifpath-specs tags that this repo doesn't
// implement yet, because motifpath-specs runs ahead of this repo by design.
// Running without it would run those scenarios, so the suite refuses.
func bddTags(t *testing.T) string {
	t.Helper()
	tags := os.Getenv("BDD_TAGS")
	if tags == "" {
		t.Fatal("BDD_TAGS is unset: run `make test:bdd`, which builds it from bdd/implemented-tickets.txt")
	}
	return tags
}

// InitializeScenario is called once per scenario by godog, so a fresh *world
// (and therefore fresh fakes) backs every scenario independently.
func InitializeScenario(sc *godog.ScenarioContext) {
	w := newWorld()
	registerIngestSteps(sc, w)
	registerHealthSteps(sc, w)
	registerAdminSteps(sc, w)
	registerPracticeSteps(sc, w)
	registerSongChartSteps(sc, w)
	registerReviewSteps(sc, w)
	registerEventLogSteps(sc, w)
}
