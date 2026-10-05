//go:build integration

package bdd

import (
	"os"
	"testing"

	"github.com/cucumber/godog"
)

// specsDir is the motifpath-specs checkout, relative to this package's
// directory, which is where `go test` sets the working directory — not the repo
// root the Makefile runs from. MOTIFPATH_SPECS_DIR overrides it, for running
// against a specs worktree.
var specsDir = func() string {
	if dir := os.Getenv("MOTIFPATH_SPECS_DIR"); dir != "" {
		return dir
	}
	return "../../../../../motifpath-specs"
}()

func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeScenario,
		Options: &godog.Options{
			Format: "pretty",
			// Only the practice features whose steps run in this worker: grading,
			// knowledge state, and the record of sessions and learning activity. The
			// rest of features/practice is core-domain's.
			Paths: []string{
				specsDir + "/features/practice/grade-practice-response.feature",
				specsDir + "/features/practice/knowledge-state.feature",
				specsDir + "/features/practice/practice-sessions.feature",
			},
			TestingT: t,
			// Strict fails the suite on an undefined step instead of warning and
			// exiting 0. A scenario tagged @wip is not implemented here yet and is
			// skipped; remove the tag in motifpath-specs when it is. Mirrors the
			// other services' runners.
			Strict: true,
			Tags:   "~@wip",
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
	registerPracticeSteps(sc, w)
	registerActivitySteps(sc, w)
}
