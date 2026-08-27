//go:build integration

package bdd

import (
	"testing"

	"github.com/cucumber/godog"
)

// featuresPath is relative to this package's directory, which is where `go
// test` sets the working directory — not the repo root the Makefile runs
// from.
const featuresBase = "../../../../../motifpath-specs/features"

func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeScenario,
		Options: &godog.Options{
			Format: "pretty",
			Paths: []string{
				featuresBase + "/user-registration",
				featuresBase + "/content-management",
				featuresBase + "/learning-paths",
			},
			TestingT: t,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}

// InitializeScenario is called once per scenario by godog, so a fresh
// *world (and therefore fresh fakes) backs every scenario independently.
func InitializeScenario(sc *godog.ScenarioContext) {
	w := newWorld()
	registerCommonSteps(sc, w)
	registerUserRegistrationSteps(sc, w)
	registerContentNodeSteps(sc, w)
	registerChallengeSteps(sc, w)
	registerExerciseSteps(sc, w)
	registerExpandedContentSteps(sc, w)
	registerLearningPathSteps(sc, w)
	registerPathAssignmentSteps(sc, w)
	registerStudentPathViewSteps(sc, w)
}
