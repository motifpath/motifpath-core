package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// instrumentKnowledge is seededKnowledgeNodeRepository plus a guitar-only
// skill "palm-muting" and a guitar-only concept "fretboard".
func instrumentKnowledge() *fakeKnowledgeNodeRepository {
	knowledge := seededKnowledgeNodeRepository()
	knowledge.put(domain.KnowledgeNode{ID: "palm-muting", Kind: domain.KnowledgeNodeKindSkill, Key: "palm-muting", InstrumentIDs: []string{"guitar"}})
	knowledge.put(domain.KnowledgeNode{ID: "fretboard", Kind: domain.KnowledgeNodeKindConcept, Key: "fretboard", InstrumentIDs: []string{"guitar"}})
	return knowledge
}

func newInstrumentExerciseService(challenges *fakeChallengeRepository, exercises *fakeExerciseRepository, nodes *fakeContentNodeRepository) *application.ExerciseService {
	return application.NewExerciseService(challenges, exercises, nodes, instrumentKnowledge(), newFakeDiagramRepository(), exerciseInstruments(), newFakeVoiceRepository(), exerciseUsers(), idSequence(), func() time.Time { return fixedCreatedAt }, noShuffle)
}

func instruments(ids ...string) *[]string { return &ids }

func nodeFor(id string, instrumentIDs ...string) domain.ContentNode {
	node := videoNode(id)
	node.InstrumentIDs = instrumentIDs
	return node
}

func TestExerciseService_InstrumentMatch(t *testing.T) {
	rules := []struct {
		name        string
		skillIDs    []string
		conceptIDs  []string
		instruments *[]string
		wantField   string // empty means the exercise is created
	}{
		{name: "a piano exercise may use an every-instrument skill", skillIDs: []string{"skill-1"}, conceptIDs: []string{"concept-1"}, instruments: instruments("piano")},
		{name: "a guitar-and-piano exercise may use a guitar skill", skillIDs: []string{"palm-muting"}, conceptIDs: []string{"concept-1"}, instruments: instruments("guitar", "piano")},
		{name: "a piano exercise cannot use a guitar skill", skillIDs: []string{"palm-muting"}, conceptIDs: []string{"concept-1"}, instruments: instruments("piano"), wantField: "skill_ids"},
		{name: "a piano exercise cannot use a guitar concept", skillIDs: []string{"skill-1"}, conceptIDs: []string{"fretboard"}, instruments: instruments("piano"), wantField: "concept_ids"},
		{name: "an every-instrument exercise cannot use a guitar skill", skillIDs: []string{"palm-muting"}, conceptIDs: []string{"concept-1"}, wantField: "skill_ids"},
	}
	for _, tt := range rules {
		t.Run(tt.name, func(t *testing.T) {
			svc := newInstrumentExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository(), newFakeContentNodeRepository())

			_, err := svc.CreateExercise(context.Background(), teacherCaller(), "Title", domain.NewPlainTextPrompt("Prompt"),
				domain.ExerciseTypeTextResponse, tt.skillIDs, tt.conceptIDs, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, tt.instruments)

			if tt.wantField == "" {
				require.NoError(t, err)
				return
			}
			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assertHasField(t, valErr, tt.wantField)
		})
	}

	t.Run("changing only an exercise's instruments re-checks its skills", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ExerciseType: domain.ExerciseTypeTextResponse, InstrumentIDs: []string{"guitar"}, ChallengeIDs: []string{}, ContentNodeIDs: []string{}})
		svc := newInstrumentExerciseService(newFakeChallengeRepository(), exercises, newFakeContentNodeRepository())

		_, err := svc.UpdateExercise(context.Background(), teacherCaller(), "exercise-1", "Muted chugs", domain.NewPlainTextPrompt("Prompt"),
			[]string{"palm-muting"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, instruments("piano"))

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assertHasField(t, valErr, "skill_ids")
	})

	t.Run("an update that omits instruments checks skills against the current ones", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ExerciseType: domain.ExerciseTypeTextResponse, InstrumentIDs: []string{"piano"}, ChallengeIDs: []string{}, ContentNodeIDs: []string{}})
		svc := newInstrumentExerciseService(newFakeChallengeRepository(), exercises, newFakeContentNodeRepository())

		_, err := svc.UpdateExercise(context.Background(), teacherCaller(), "exercise-1", "Muted chugs", domain.NewPlainTextPrompt("Prompt"),
			[]string{"palm-muting"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assertHasField(t, valErr, "skill_ids")
	})
}

func TestExerciseService_InstrumentFit(t *testing.T) {
	linkToChallenge := []struct {
		name     string
		node     []string
		exercise []string
		wantErr  error
	}{
		{name: "a guitar-and-piano node's challenge takes a guitar exercise", node: []string{"guitar", "piano"}, exercise: []string{"guitar"}},
		{name: "a piano node's challenge takes an every-instrument exercise", node: []string{"piano"}},
		{name: "a piano node's challenge refuses a guitar exercise", node: []string{"piano"}, exercise: []string{"guitar"}, wantErr: domain.ErrConflict},
		{name: "an every-instrument node's challenge refuses a guitar exercise", exercise: []string{"guitar"}, wantErr: domain.ErrConflict},
	}
	for _, tt := range linkToChallenge {
		t.Run(tt.name, func(t *testing.T) {
			nodes := newFakeContentNodeRepository()
			nodes.put(nodeFor("node-1", tt.node...))
			challenges := newFakeChallengeRepository()
			challenges.put(domain.Challenge{ID: "challenge-1", ContentNodeID: "node-1"})
			exercises := newFakeExerciseRepository()
			exercises.put(domain.Exercise{ID: "exercise-1", InstrumentIDs: tt.exercise, ChallengeIDs: []string{}, ContentNodeIDs: []string{}})
			svc := newInstrumentExerciseService(challenges, exercises, nodes)

			_, err := svc.LinkExerciseToChallenge(context.Background(), teacherCaller(), "challenge-1", "exercise-1")

			if tt.wantErr == nil {
				require.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}

	t.Run("a node refuses a path exercise for none of its instruments", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(nodeFor("node-1", "piano"))
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", InstrumentIDs: []string{"guitar"}, ChallengeIDs: []string{}, ContentNodeIDs: []string{}})
		svc := newInstrumentExerciseService(newFakeChallengeRepository(), exercises, nodes)

		_, err := svc.LinkExerciseToContentNode(context.Background(), teacherCaller(), "node-1", "exercise-1")

		assert.ErrorIs(t, err, domain.ErrConflict)
	})

	retarget := []struct {
		name          string
		viaChallenge  bool
		newInstrument []string
		wantErr       error
	}{
		{name: "changing a challenge exercise's instruments away from its node is refused", viaChallenge: true, newInstrument: []string{"guitar"}, wantErr: domain.ErrConflict},
		{name: "changing a path exercise's instruments away from its node is refused", newInstrument: []string{"guitar"}, wantErr: domain.ErrConflict},
		{name: "changing a linked exercise's instruments while it still suits its node succeeds", viaChallenge: true, newInstrument: []string{"guitar", "piano"}},
	}
	for _, tt := range retarget {
		t.Run(tt.name, func(t *testing.T) {
			nodes := newFakeContentNodeRepository()
			nodes.put(nodeFor("node-1", "piano"))
			challenges := newFakeChallengeRepository()
			challenges.put(domain.Challenge{ID: "challenge-1", ContentNodeID: "node-1"})
			exercise := domain.Exercise{ID: "exercise-1", ExerciseType: domain.ExerciseTypeTextResponse, InstrumentIDs: []string{"piano"}, ChallengeIDs: []string{}, ContentNodeIDs: []string{}}
			if tt.viaChallenge {
				exercise.ChallengeIDs = []string{"challenge-1"}
			} else {
				exercise.ContentNodeIDs = []string{"node-1"}
			}
			exercises := newFakeExerciseRepository()
			exercises.put(exercise)
			svc := newInstrumentExerciseService(challenges, exercises, nodes)

			_, err := svc.UpdateExercise(context.Background(), teacherCaller(), "exercise-1", "Title", domain.NewPlainTextPrompt("Prompt"),
				[]string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, &tt.newInstrument)

			if tt.wantErr == nil {
				require.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}
