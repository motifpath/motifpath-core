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
			// definition here cannot pass CI silently. motifpath-specs runs ahead
			// of this repo by design (spec first): a scenario tagged @wip is not
			// implemented yet and is skipped. Remove the tag in motifpath-specs
			// when the scenario is implemented. Mirrors core-domain's runner.
			Strict: true,
			// @web scenarios are client behaviour, pinned by motifpath-web's own tests.
			Tags: "~@wip && ~@web",
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
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
}
