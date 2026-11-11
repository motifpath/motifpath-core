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
			// knowledge state, timed answers against their fluent times, and the
			// record of sessions and learning activity. The rest of
			// features/practice is core-domain's.
			Paths: []string{
				specsDir + "/features/practice/grade-practice-response.feature",
				specsDir + "/features/practice/knowledge-state.feature",
				specsDir + "/features/practice/progression-key-tour.feature",
				specsDir + "/features/practice/practice-sessions.feature",
				specsDir + "/features/practice/timed-thresholds.feature",
			},
			TestingT: t,
			// Strict fails the suite on an undefined step instead of warning and
			// exiting 0; scenarios not implemented here yet are skipped by tag
			// (bddTags). Mirrors the other services' runners.
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
	registerPracticeSteps(sc, w)
	registerCellSteps(sc, w)
	registerEveryInstanceSteps(sc, w)
	registerExerciseSteps(sc, w)
	registerShapeSteps(sc, w)
	registerGeneratedChoiceSteps(sc, w)
	registerFeltSteps(sc, w)
	registerActivitySteps(sc, w)
	registerReviewSteps(sc, w)
	registerChordSteps(sc, w)
	registerSongSteps(sc, w)
	registerTourSteps(sc, w)
	registerGuideToneSteps(sc, w)
}
