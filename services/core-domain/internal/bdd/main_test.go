//go:build integration

package bdd

import (
	"os"
	"testing"

	"github.com/cucumber/godog"
)

// featuresPath is relative to this package's directory, which is where `go
// test` sets the working directory — not the repo root the Makefile runs
// from.
//
// MOTIFPATH_SPECS_DIR overrides the default location of the motifpath-specs
// checkout — needed when running from a git worktree, where the sibling
// checkout is not at that relative path.
var featuresBase = func() string {
	if dir := os.Getenv("MOTIFPATH_SPECS_DIR"); dir != "" {
		return dir + "/features"
	}
	return "../../../../../motifpath-specs/features"
}()

func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeScenario,
		Options: &godog.Options{
			Format: "pretty",
			Paths: []string{
				featuresBase + "/user-registration",
				featuresBase + "/content-management",
				featuresBase + "/learning-paths",
				featuresBase + "/core-domain",
			},
			TestingT: t,
			// Without this, godog reports an undefined step as a warning and
			// still exits 0 — so a scenario merged to motifpath-specs with no
			// step definition here passes CI silently. That is not
			// hypothetical: the three section_label scenarios from
			// motifpath-specs#24 sat undefined and green on dev until this
			// branch implemented them. motifpath-specs is the contract source
			// of truth and CI checks it out from its default branch, so an
			// unimplemented scenario must fail the build, not whisper.
			Strict: true,
			// Strict makes the suite fail on any scenario without step
			// definitions, and motifpath-specs runs ahead of this repo by
			// design (spec first). A scenario tagged @wip is a spec whose
			// implementation has not landed yet; excluding it keeps the build
			// green for unrelated work. Strict still applies to everything
			// untagged, so an untagged scenario cannot go undefined silently.
			// Remove the tag in motifpath-specs when the feature is implemented.
			Tags: "~@wip",
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
	registerDisplayNameSteps(sc, w)
	registerLocaleSteps(sc, w)
	registerContentNodeSteps(sc, w)
	registerSkillSteps(sc, w)
	registerInstrumentSteps(sc, w)
	registerDiagramSteps(sc, w)
	registerConceptSteps(sc, w)
	registerChallengeSteps(sc, w)
	registerExerciseSteps(sc, w)
	registerPracticeSessionSteps(sc, w)
	registerMediaUploadSteps(sc, w)
	registerExpandedContentSteps(sc, w)
	registerLearningPathSteps(sc, w)
	registerCourseSteps(sc, w)
	registerCourseReactivationSteps(sc, w)
	registerCourseLanguageSteps(sc, w)
	registerLearningPathLibrarySteps(sc, w)
	registerItemInstrumentSteps(sc, w)
	registerCourseEnrollmentSteps(sc, w)
	registerCurrentPathLifecycleSteps(sc, w)
	registerAssignStudentPathSteps(sc, w)
	registerStudentPathViewSteps(sc, w)
	registerContentNodeVersioningSteps(sc, w)
	registerHealthSteps(sc, w)
	registerListingSteps(sc, w)
}
