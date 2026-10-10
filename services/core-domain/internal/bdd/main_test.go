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
				// Only the practice features whose steps run here; grading and
				// knowledge state run in the Aggregation Worker.
				featuresBase + "/practice/compose-practice-session.feature",
				featuresBase + "/practice/diagram-shapes.feature",
				featuresBase + "/practice/fretboard-cells.feature",
				featuresBase + "/practice/fretboard-map.feature",
				featuresBase + "/practice/node-levels.feature",
				featuresBase + "/practice/practice-summary.feature",
				featuresBase + "/practice/practice-overview.feature",
				featuresBase + "/practice/tap-check.feature",
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
			// @web scenarios are client behaviour, pinned by motifpath-web's own tests.
			Tags: "~@wip && ~@web",
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
	registerKnowledgeNodeSteps(sc, w)
	registerKnowledgeEdgeSteps(sc, w)
	registerClassificationInstrumentSteps(sc, w)
	registerExerciseInstrumentSteps(sc, w)
	registerInstrumentSteps(sc, w)
	registerDiagramSteps(sc, w)
	registerChordSteps(sc, w)
	registerSongChartSteps(sc, w)
	registerSongChartChordProSteps(sc, w)
	registerSongChartEmbedSteps(sc, w)
	registerDiagramPlaybackSteps(sc, w)
	registerVoiceSteps(sc, w)
	registerDiagramAnnotationSteps(sc, w)
	registerChallengeSteps(sc, w)
	registerExerciseSteps(sc, w)
	registerDiagramAnswerSteps(sc, w)
	registerMediaUploadSteps(sc, w)
	registerExpandedContentSteps(sc, w)
	registerLearningPathSteps(sc, w)
	registerPathPublishingSteps(sc, w)
	registerPathCatalogSteps(sc, w)
	registerPathEnrollmentSteps(sc, w)
	registerCourseSteps(sc, w)
	registerCourseReactivationSteps(sc, w)
	registerCourseLanguageSteps(sc, w)
	registerLearningPathLibrarySteps(sc, w)
	registerItemInstrumentSteps(sc, w)
	registerThumbnailSteps(sc, w)
	registerCourseEnrollmentSteps(sc, w)
	registerCurrentPathLifecycleSteps(sc, w)
	registerAssignStudentPathSteps(sc, w)
	registerStudentPathViewSteps(sc, w)
	registerContentNodeVersioningSteps(sc, w)
	registerHealthSteps(sc, w)
	registerListingSteps(sc, w)
	registerPracticeSessionSteps(sc, w)
	registerPracticeSummarySteps(sc, w)
	registerPracticeDaysSteps(sc, w)
	registerSongsPlayedSteps(sc, w)
	registerPracticeOverviewEffortSteps(sc, w)
	registerNodeLevelSteps(sc, w)
	registerFretboardCellSteps(sc, w)
	registerDiagramShapeSteps(sc, w)
	registerFretboardMapSteps(sc, w)
	registerTapCheckSteps(sc, w)
	registerFeltQuestionSteps(sc, w)
}
