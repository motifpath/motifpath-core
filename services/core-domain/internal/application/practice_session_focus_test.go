package application_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// childSkill puts a skill part_of parentID on the knowledge map.
func (f *practiceFixture) childSkill(id, parentID string) {
	f.skill(parentID)
	f.knowledgeNodes.put(domain.KnowledgeNode{ID: id, Kind: domain.KnowledgeNodeKindSkill, Key: id, ParentID: &parentID})
}

// exercisesOf puts count exercises of exerciseType taking 30 seconds each
// under skillID for every instrument, ids prefix-00, prefix-01, ..., and
// returns their item keys.
func (f *practiceFixture) exercisesOf(prefix, skillID string, exerciseType domain.ExerciseType, count int) []string {
	f.skill(skillID)
	keys := make([]string, count)
	seconds := 30
	for i := range count {
		id := fmt.Sprintf("%s-%02d", prefix, i)
		require.NoError(f.t, f.exercises.Create(context.Background(), domain.Exercise{
			ID: id, Title: id, ExerciseType: exerciseType,
			Skills: []domain.KnowledgeNode{{ID: skillID}}, EstimatedDurationSeconds: &seconds,
		}))
		keys[i] = domain.ExerciseItemKey(id)
	}
	return keys
}

// dueSince makes keys due since days ago.
func (f *practiceFixture) dueSince(days int, keys ...string) {
	for _, k := range keys {
		f.stateOf(k, domain.PracticeItemState{Level: domain.KnowledgeLevelLearning, Counted: 2, Box: 1, DueAt: f.daysAgo(days)})
	}
}

// accurate makes keys accurate, coming due in 5 days.
func (f *practiceFixture) accurate(keys ...string) {
	for _, k := range keys {
		f.stateOf(k, domain.PracticeItemState{Level: domain.KnowledgeLevelAccurate, Counted: 3, Box: 2, DueAt: f.inDays(5)})
	}
}

// playClean installs image_choice exercises as the play_clean rung, so a
// test has three rungs of exercises: know (text_response), hear
// (audio_recognition) and play_clean (image_choice).
func (f *practiceFixture) playClean() {
	f.templates.extra = append(f.templates.extra, domain.DrillTemplate{Key: "exercise:image_choice", ItemKind: domain.PracticeItemKindExercise, Rung: domain.PracticeRungPlayClean, Timed: true})
}

func (f *practiceFixture) choose(t *testing.T, instrumentID *string, nodeID string, minutes int) domain.PracticeSessionPlan {
	t.Helper()
	plan, err := f.svc.ComposePlan(context.Background(), studentCaller(), instrumentID, &nodeID, minutes)
	require.NoError(t, err)
	return plan
}

func focusIDs(plan domain.PracticeSessionPlan) []string {
	ids := make([]string, len(plan.FocusNodes))
	for i, n := range plan.FocusNodes {
		ids[i] = n.ID
	}
	return ids
}

// reviewBlock lists the plan's due items on none of its focus nodes.
func reviewBlock(plan domain.PracticeSessionPlan) []domain.PracticeSessionItem {
	focus := focusIDs(plan)
	var block []domain.PracticeSessionItem
	for _, item := range plan.Items {
		if item.Reason == domain.PracticePickDue && (item.NodeID == nil || !slices.Contains(focus, *item.NodeID)) {
			block = append(block, item)
		}
	}
	return block
}

func guitar() *string {
	id := practiceGuitar
	return &id
}

func TestPracticeSessionService_ComposePlan_TodaysMixFocus(t *testing.T) {
	// subjectsAndOthers puts the student on skills a, b and c with 10 due
	// items each, and on skills d, e and f with 4 each, due since 1 to 12
	// days ago, f's most overdue.
	subjectsAndOthers := func(t *testing.T) (*practiceFixture, []string) {
		f := newPracticeFixture(t)
		f.onPath("skill-a", "skill-b", "skill-c", "skill-d", "skill-e", "skill-f")
		for _, s := range []string{"skill-a", "skill-b", "skill-c"} {
			f.due(f.exercisesOf(s, s, domain.ExerciseTypeTextResponse, 10)...)
		}
		var others []string
		for i, s := range []string{"skill-d", "skill-e", "skill-f"} {
			for j, k := range f.exercisesOf(s, s, domain.ExerciseTypeTextResponse, 4) {
				f.dueSince(1+i*4+j, k)
				others = append(others, k)
			}
		}
		return f, others
	}

	t.Run("names at most three subjects, and every other item is the warm-up or the review block", func(t *testing.T) {
		f, _ := subjectsAndOthers(t)

		plan := f.compose(t, practiceGuitar, 20)

		assert.Equal(t, domain.PracticeFocusSuggested, plan.FocusSource)
		assert.Equal(t, []string{"skill-a", "skill-b", "skill-c"}, focusIDs(plan))
		for _, item := range plan.Items {
			if item.Reason == domain.PracticePickWarmUp || item.Reason == domain.PracticePickDue {
				continue
			}
			require.NotNil(t, item.NodeID, item.ItemKey)
			assert.Contains(t, focusIDs(plan), *item.NodeID, item.ItemKey)
		}
	})

	t.Run("the main subject is the node with the most due and weak time", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-b", "skill-a")
		f.due(f.exercisesOf("b", "skill-b", domain.ExerciseTypeTextResponse, 4)...)
		f.due(f.exercisesOf("a-due", "skill-a", domain.ExerciseTypeTextResponse, 4)...)
		f.weak(f.exercisesOf("a-weak", "skill-a", domain.ExerciseTypeTextResponse, 4)...)

		plan := f.compose(t, practiceGuitar, 20)

		assert.Equal(t, []string{"skill-a", "skill-b"}, focusIDs(plan))
	})

	t.Run("due items of other nodes come in a review block of the 5 most overdue", func(t *testing.T) {
		f, others := subjectsAndOthers(t)

		plan := f.compose(t, practiceGuitar, 30)

		var block []string
		for _, item := range reviewBlock(plan) {
			block = append(block, item.ItemKey)
		}
		assert.ElementsMatch(t, others[7:], block, "the 5 most overdue of the 12")
	})

	t.Run("the review block never takes more than a fifth of the focus time", func(t *testing.T) {
		f, _ := subjectsAndOthers(t)

		plan := f.compose(t, practiceGuitar, 5)

		seconds := 0
		for _, item := range reviewBlock(plan) {
			seconds += item.EstimatedSeconds
		}
		assert.Equal(t, 60, seconds, "a fifth of 5 minutes")
	})

	t.Run("the review block comes right after the warm-up", func(t *testing.T) {
		f, _ := subjectsAndOthers(t)
		f.playAlong("warm", "Warm", practiceGuitar, "skill-a", 100)
		f.state("warm", domain.PracticeItemState{Level: domain.KnowledgeLevelFluent, Counted: 6, Box: 4, DueAt: f.inDays(5), BestCleanBPM: intPtr(100)})

		plan := f.compose(t, practiceGuitar, 20)

		block := reviewBlock(plan)
		require.NotEmpty(t, block)
		require.Greater(t, len(plan.Items), len(block)+1)
		assert.Equal(t, domain.PracticePickWarmUp, plan.Items[0].Reason)
		assert.Equal(t, block, plan.Items[1:1+len(block)])
	})

	t.Run("with few nodes due or weak, the nodes of new items complete the subjects in path order", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-due", "skill-x", "skill-y", "skill-z")
		f.due(f.exercisesOf("due", "skill-due", domain.ExerciseTypeTextResponse, 2)...)
		for _, s := range []string{"skill-x", "skill-y", "skill-z"} {
			f.exercisesOf(s, s, domain.ExerciseTypeTextResponse, 4)
		}

		plan := f.compose(t, practiceGuitar, 20)

		assert.Equal(t, []string{"skill-due", "skill-x", "skill-y"}, focusIDs(plan))
		for _, item := range plan.Items {
			assert.NotEqual(t, "skill-z", *item.NodeID, item.ItemKey)
		}
	})

	t.Run("a caught-up session's subjects are the nodes it reviews ahead, then those it stretches to", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.known(3, f.exerciseBatch("known", "skill-1", 4, 30)...)
		f.applies("skill-1", "skill-ready")
		f.exerciseBatch("stretch", "skill-ready", 40, 30)

		plan := f.compose(t, practiceGuitar, 10)

		assert.Equal(t, []string{"skill-1", "skill-ready"}, focusIDs(plan))
	})

	t.Run("with no play-along on any of its subjects, there is no application ending", func(t *testing.T) {
		f, _ := subjectsAndOthers(t)
		f.playAlong("riff", "Riff", practiceGuitar, "skill-d", 100)

		plan := f.compose(t, practiceGuitar, 10)

		for _, item := range plan.Items {
			assert.NotEqual(t, domain.PracticePickApplication, item.Reason, item.ItemKey)
		}
	})

	t.Run("the ending plays a subject that other items practise, not one with only that play-along", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-lick", "skill-tones")
		f.playAlong("lick", "Lick", practiceGuitar, "skill-lick", 100)
		f.exercisesOf("tones", "skill-tones", domain.ExerciseTypeTextResponse, 4)
		f.playAlong("riff", "Riff", practiceGuitar, "skill-tones", 100)

		plan := f.compose(t, practiceGuitar, 10)

		last := plan.Items[len(plan.Items)-1]
		assert.Equal(t, "play_along:riff", last.ItemKey)
		assert.Equal(t, domain.PracticePickApplication, last.Reason)
	})

	t.Run("the application ending plays the main subject first", func(t *testing.T) {
		f, _ := subjectsAndOthers(t)
		f.playAlong("riff-c", "Riff C", practiceGuitar, "skill-c", 100)
		f.playAlong("riff-a", "Riff A", practiceGuitar, "skill-a", 100)

		plan := f.compose(t, practiceGuitar, 10)

		last := plan.Items[len(plan.Items)-1]
		assert.Equal(t, domain.PracticePickApplication, last.Reason)
		assert.Equal(t, "play_along:riff-a", last.ItemKey)
		require.NotNil(t, last.NodeID)
		assert.Equal(t, "skill-a", *last.NodeID)
	})
}

func TestPracticeSessionService_ComposePlan_SubjectRungs(t *testing.T) {
	// diatonic puts the student on skill-diatonic with 5 exercises at each
	// of play_clean, hear and know, play_clean listed first.
	diatonic := func(t *testing.T) (f *practiceFixture, know, hear, clean []string) {
		f = newPracticeFixture(t)
		f.playClean()
		clean = f.exercisesOf("a-clean", "skill-diatonic", domain.ExerciseTypeImageChoice, 5)
		hear = f.exercisesOf("b-hear", "skill-diatonic", domain.ExerciseTypeAudioRecognition, 5)
		know = f.exercisesOf("c-know", "skill-diatonic", domain.ExerciseTypeTextResponse, 5)
		f.onPath("skill-diatonic")
		return f, know, hear, clean
	}
	newRungs := func(plan domain.PracticeSessionPlan) []domain.PracticeRung {
		var rungs []domain.PracticeRung
		for _, item := range plan.Items {
			if item.Reason == domain.PracticePickNew || item.Reason == domain.PracticePickStretch {
				rungs = append(rungs, item.Rung)
			}
		}
		return rungs
	}

	t.Run("a student new to a subject gets new items from its first two rungs", func(t *testing.T) {
		f, _, _, _ := diatonic(t)

		plan := f.compose(t, practiceGuitar, 10)

		rungs := newRungs(plan)
		require.NotEmpty(t, rungs)
		assert.NotContains(t, rungs, domain.PracticeRungPlayClean)
	})

	t.Run("once a rung is accurate, new items climb to the next two rungs", func(t *testing.T) {
		f, know, _, _ := diatonic(t)
		f.accurate(know...)

		plan := f.compose(t, practiceGuitar, 10)

		rungs := newRungs(plan)
		require.NotEmpty(t, rungs)
		assert.NotContains(t, rungs, domain.PracticeRungKnow)
		assert.Contains(t, rungs, domain.PracticeRungPlayClean)
	})

	t.Run("due items are reviewed whatever their rung", func(t *testing.T) {
		f, know, hear, _ := diatonic(t)
		f.accurate(know[3:]...)
		f.accurate(hear...)
		f.due(know[:3]...)

		plan := f.compose(t, practiceGuitar, 10)

		for _, k := range know[:3] {
			i := slices.IndexFunc(plan.Items, func(item domain.PracticeSessionItem) bool { return item.ItemKey == k })
			require.GreaterOrEqual(t, i, 0, k)
			assert.Equal(t, domain.PracticePickDue, plan.Items[i].Reason)
		}
	})
}

// conceptOf classifies diagramID under conceptID too.
func (f *practiceFixture) conceptOf(diagramID, conceptID string) {
	d, err := f.diagrams.GetByID(context.Background(), diagramID)
	require.NoError(f.t, err)
	d.Concepts = append(d.Concepts, domain.KnowledgeNode{ID: conceptID})
	require.NoError(f.t, f.diagrams.Update(context.Background(), d))
}

func TestPracticeSessionService_ComposePlan_ChosenSubject(t *testing.T) {
	// progressions puts the skill progressions, with its part_of child
	// blues-form, off the student's paths, each with new exercises, and the
	// student on chord-tones, with due items.
	progressions := func(t *testing.T) *practiceFixture {
		f := newPracticeFixture(t)
		f.childSkill("blues-form", "progressions")
		f.exercisesOf("prog", "progressions", domain.ExerciseTypeTextResponse, 10)
		f.exercisesOf("blues", "blues-form", domain.ExerciseTypeTextResponse, 10)
		f.onPath("chord-tones")
		f.due(f.exercisesOf("tones", "chord-tones", domain.ExerciseTypeTextResponse, 10)...)
		return f
	}

	t.Run("holds only items of the chosen node and its subtree, named as chosen", func(t *testing.T) {
		f := progressions(t)

		plan := f.choose(t, guitar(), "progressions", 10)

		assert.Equal(t, domain.PracticeFocusChosen, plan.FocusSource)
		assert.Equal(t, []string{"progressions"}, focusIDs(plan))
		require.NotEmpty(t, plan.Items)
		nodes := map[string]bool{}
		for _, item := range plan.Items {
			require.NotNil(t, item.NodeID, item.ItemKey)
			nodes[*item.NodeID] = true
		}
		assert.Equal(t, map[string]bool{"progressions": true, "blues-form": true}, nodes, "the due items of chord-tones wait for Today's mix")
	})

	t.Run("the chosen node's due items come first", func(t *testing.T) {
		f := progressions(t)
		f.due(f.exercisesOf("prog-due", "progressions", domain.ExerciseTypeTextResponse, 4)...)

		plan := f.choose(t, guitar(), "progressions", 10)

		for i := range 4 {
			assert.Equal(t, domain.PracticePickDue, plan.Items[i].Reason)
		}
	})

	t.Run("has no warm-up", func(t *testing.T) {
		f := progressions(t)
		f.playAlong("clean", "Clean", practiceGuitar, "chord-tones", 100)
		f.state("clean", domain.PracticeItemState{Level: domain.KnowledgeLevelFluent, Counted: 6, Box: 4, DueAt: f.inDays(5), BestCleanBPM: intPtr(100)})
		f.playAlong("prog-clean", "Prog clean", practiceGuitar, "progressions", 100)
		f.state("prog-clean", domain.PracticeItemState{Level: domain.KnowledgeLevelFluent, Counted: 6, Box: 4, DueAt: f.inDays(5), BestCleanBPM: intPtr(100)})

		plan := f.choose(t, guitar(), "progressions", 10)

		for _, item := range plan.Items {
			assert.NotEqual(t, domain.PracticePickWarmUp, item.Reason, item.ItemKey)
		}
	})

	t.Run("ends with a play-along of its subtree", func(t *testing.T) {
		f := progressions(t)
		f.playAlong("pop", "Pop", practiceGuitar, "blues-form", 100)

		plan := f.choose(t, guitar(), "progressions", 10)

		last := plan.Items[len(plan.Items)-1]
		assert.Equal(t, "play_along:pop", last.ItemKey)
		assert.Equal(t, domain.PracticePickApplication, last.Reason)
	})

	t.Run("with no play-along in its subtree, has no application ending", func(t *testing.T) {
		f := progressions(t)
		f.playAlong("riff", "Riff", practiceGuitar, "chord-tones", 100)

		plan := f.choose(t, guitar(), "progressions", 10)

		for _, item := range plan.Items {
			assert.NotEqual(t, domain.PracticePickApplication, item.Reason, item.ItemKey)
		}
	})

	t.Run("a node whose requirements aren't met can be chosen", func(t *testing.T) {
		f := progressions(t)
		f.requires("progressions", "diatonic", domain.KnowledgeLevelAccurate)
		f.exercisesOf("diatonic", "diatonic", domain.ExerciseTypeTextResponse, 4)

		plan := f.choose(t, nil, "progressions", 10)

		for _, item := range plan.Items {
			assert.Contains(t, []string{"progressions", "blues-form"}, *item.NodeID, item.ItemKey)
		}
	})

	t.Run("when its subject runs out, it ends early rather than leave it", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("other")
		f.exercisesOf("other", "other", domain.ExerciseTypeTextResponse, 40)
		f.known(30, f.exercisesOf("fluent", "find-notes", domain.ExerciseTypeTextResponse, 6)...)

		plan := f.choose(t, nil, "find-notes", 30)

		require.Len(t, plan.Items, 6, "every known item is reviewed ahead, however far its due date")
		for _, item := range plan.Items {
			assert.Equal(t, "find-notes", *item.NodeID)
			assert.Equal(t, domain.PracticePickReviewAhead, item.Reason)
		}
	})

	t.Run("time left goes to reviewing the subject ahead, then to more of its new items", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.due(f.exercisesOf("due", "progressions", domain.ExerciseTypeTextResponse, 2)...)
		f.known(3, f.exercisesOf("known", "progressions", domain.ExerciseTypeTextResponse, 4)...)
		f.exercisesOf("new", "progressions", domain.ExerciseTypeTextResponse, 20)

		plan := f.choose(t, guitar(), "progressions", 20)

		seconds := secondsByReason(plan)
		assert.Equal(t, 120, seconds[domain.PracticePickReviewAhead])
		assert.Greater(t, seconds[domain.PracticePickNew], 180)
	})

	t.Run("a concept can be chosen, its items classified under it", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.knowledgeNodes.put(domain.KnowledgeNode{ID: "concept-tones", Kind: domain.KnowledgeNodeKindConcept, Key: "concept-tones"})
		seconds := 30
		for _, id := range []string{"tones-1", "tones-2"} {
			require.NoError(t, f.exercises.Create(context.Background(), domain.Exercise{
				ID: id, Title: id, ExerciseType: domain.ExerciseTypeTextResponse,
				Concepts: []domain.KnowledgeNode{{ID: "concept-tones"}}, EstimatedDurationSeconds: &seconds,
			}))
		}
		f.playAlong("tones-riff", "Tones riff", practiceGuitar, "skill-riffs", 100)
		f.conceptOf("tones-riff", "concept-tones")

		plan := f.choose(t, guitar(), "concept-tones", 10)

		assert.ElementsMatch(t, []string{"exercise:tones-1", "exercise:tones-2", "play_along:tones-riff"}, planKeys(plan))
	})

	t.Run("a node with nothing to practise in hand is not found", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.cellsOn(practiceGuitar, "find-notes", 3, 6)

		_, err := f.svc.ComposePlan(context.Background(), studentCaller(), guitar(), strPtr("find-notes"), 10)

		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a node that doesn't exist is not found", func(t *testing.T) {
		f := progressions(t)

		_, err := f.svc.ComposePlan(context.Background(), studentCaller(), nil, strPtr("missing"), 10)

		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}

// confusableDrillItem is a fake drill item a learner mistakes for the items
// with.
type confusableDrillItem struct {
	fakeDrillItem
	with []string
}

func (i confusableDrillItem) ConfusableWith() []string { return i.with }

// confusableDrillSource offers count fake items under skillID, whose items
// pairs maps by id are confusable, both ways.
type confusableDrillSource struct {
	fakeDrillSource
	pairs map[string]string
}

func (s confusableDrillSource) wrap(item domain.DrillItem) domain.DrillItem {
	id := item.(fakeDrillItem).id
	for a, b := range s.pairs {
		switch id {
		case a:
			return confusableDrillItem{fakeDrillItem: fakeDrillItem{id: id}, with: []string{fakeDrillItem{id: b}.ItemKey()}}
		case b:
			return confusableDrillItem{fakeDrillItem: fakeDrillItem{id: id}, with: []string{fakeDrillItem{id: a}.ItemKey()}}
		}
	}
	return item
}

func (s confusableDrillSource) SkillItems(ctx context.Context, skillID, instrumentID string, inHand bool, knowledge application.KnowledgeMap) ([]domain.DrillItem, error) {
	items, err := s.fakeDrillSource.SkillItems(ctx, skillID, instrumentID, inHand, knowledge)
	for i := range items {
		items[i] = s.wrap(items[i])
	}
	return items, err
}

func (s confusableDrillSource) Item(ctx context.Context, itemKey string) (domain.DrillItem, bool, error) {
	item, ok, err := s.fakeDrillSource.Item(ctx, itemKey)
	if !ok || err != nil {
		return item, ok, err
	}
	return s.wrap(item), true, nil
}

func TestPracticeSessionService_ComposePlan_Confusables(t *testing.T) {
	newFamily := func(t *testing.T, skillID string, pairs map[string]string) *practiceFixture {
		f := newPracticeFixture(t)
		f.skill(skillID)
		f.svc.WithDrillSources(confusableDrillSource{fakeDrillSource: fakeDrillSource{skillID: skillID, count: 8}, pairs: pairs})
		for i := range 8 {
			key := fakeDrillItem{id: fmt.Sprintf("%02d", i)}.ItemKey()
			f.cells[practiceGuitar] = append(f.cells[practiceGuitar], domain.ClassifiedItem{ItemKey: key, NodeIDs: []string{skillID}})
			f.due(key)
		}
		return f
	}
	position := func(plan domain.PracticeSessionPlan, id string) int {
		key := fakeDrillItem{id: id}.ItemKey()
		return slices.IndexFunc(plan.Items, func(item domain.PracticeSessionItem) bool { return item.ItemKey == key })
	}

	t.Run("items of the main subject a learner confuses are asked next to each other", func(t *testing.T) {
		f := newFamily(t, "skill-fake", map[string]string{"00": "05"})
		f.onPathFor([]string{practiceGuitar}, "skill-fake")

		plan := f.composeInTheHead(t, 5)

		first, second := position(plan, "00"), position(plan, "05")
		require.GreaterOrEqual(t, first, 0)
		require.GreaterOrEqual(t, second, 0)
		assert.Equal(t, 1, second-first)
		assert.Len(t, plan.Items, 8, "nothing is lost on the way")
	})
}

func TestPracticeSessionService_ComposePlan_FocusHoldsForAnyStudent(t *testing.T) {
	for seed := range uint64(300) {
		rng := rand.New(rand.NewPCG(seed, 0))
		f := newPracticeFixture(t)
		skills := make([]string, 1+rng.IntN(8))
		for i := range skills {
			skills[i] = fmt.Sprintf("skill-%d", i)
			for j, key := range f.exercisesOf(skills[i], skills[i], domain.ExerciseTypeTextResponse, rng.IntN(12)) {
				switch rng.IntN(4) {
				case 0:
					f.dueSince(rng.IntN(20), key)
				case 1:
					f.weak(key)
				case 2:
					f.known(1+rng.IntN(20), key)
				}
				if j == 0 && rng.IntN(3) == 0 {
					f.playAlong(fmt.Sprintf("riff-%d", i), "Riff", practiceGuitar, skills[i], 80+rng.IntN(60))
				}
			}
		}
		f.onPath(skills...)
		minutes := 1 + rng.IntN(60)
		var instrumentID *string
		if rng.IntN(2) == 0 {
			instrumentID = guitar()
		}

		plan, err := f.svc.ComposePlan(context.Background(), studentCaller(), instrumentID, nil, minutes)
		if errors.Is(err, domain.ErrNotFound) {
			continue
		}
		require.NoError(t, err, "seed %d", seed)

		assert.LessOrEqual(t, len(plan.FocusNodes), domain.MaxFocusNodes, "seed %d", seed)
		focus, block := focusIDs(plan), reviewBlock(plan)
		outside := 0
		for _, item := range plan.Items {
			if item.Reason != domain.PracticePickWarmUp && item.Reason != domain.PracticePickDue {
				require.NotNil(t, item.NodeID, "seed %d: %s", seed, item.ItemKey)
				assert.Contains(t, focus, *item.NodeID, "seed %d: %s", seed, item.ItemKey)
			}
			if item.Reason == domain.PracticePickWarmUp || item.Reason == domain.PracticePickApplication {
				outside += item.EstimatedSeconds
			}
		}
		if len(plan.Items) == 1 {
			continue // a session too short for anything offers its first pick anyway
		}
		blockSeconds := 0
		for _, item := range block {
			blockSeconds += item.EstimatedSeconds
		}
		assert.LessOrEqual(t, len(block), 5, "seed %d", seed)
		assert.LessOrEqual(t, blockSeconds, (minutes*60-outside)/5, "seed %d", seed)
	}
}

func TestPracticeSessionService_ListSubjects(t *testing.T) {
	// harmony puts the area harmony, whose part_of child progressions has 5
	// know and 5 hear exercises, the know ones accurate, and the area
	// fretboard, whose child notes has only guitar fretboard cells, recalled
	// in the head. The student learns progressions on guitar.
	harmony := func(t *testing.T) *practiceFixture {
		f := newPracticeFixture(t)
		f.childSkill("progressions", "harmony")
		f.accurate(f.exercisesOf("know", "progressions", domain.ExerciseTypeTextResponse, 5)...)
		f.exercisesOf("hear", "progressions", domain.ExerciseTypeAudioRecognition, 5)
		f.childSkill("notes", "fretboard")
		f.cellsOn(practiceGuitar, "notes", 3, 6)
		f.onPathFor([]string{practiceGuitar}, "progressions")
		return f
	}
	subject := func(list domain.PracticeSubjectList, nodeID string) (domain.PracticeSubject, string, bool) {
		for _, g := range list.Groups {
			for _, s := range g.Subjects {
				if s.Node.ID == nodeID {
					return s, g.Area.ID, true
				}
			}
		}
		return domain.PracticeSubject{}, "", false
	}

	t.Run("offers every node with material for the instrument in hand, grouped by area", func(t *testing.T) {
		f := harmony(t)

		list, err := f.svc.ListSubjects(context.Background(), studentCaller(), guitar())

		require.NoError(t, err)
		assert.Equal(t, practiceGuitar, *list.InstrumentID)
		require.Len(t, list.Groups, 1, "fretboard cells are recalled in the head, not played in hand")
		assert.Equal(t, "harmony", list.Groups[0].Area.ID)
		var ids []string
		for _, s := range list.Groups[0].Subjects {
			ids = append(ids, s.Node.ID)
			assert.Equal(t, 10, s.ItemCount, s.Node.ID)
		}
		assert.Equal(t, []string{"harmony", "progressions"}, ids, "catalog order")
	})

	t.Run("a subject shows the student's level and current rung; a wide node has no level", func(t *testing.T) {
		f := harmony(t)

		list, err := f.svc.ListSubjects(context.Background(), studentCaller(), guitar())

		require.NoError(t, err)
		progressions, _, ok := subject(list, "progressions")
		require.True(t, ok)
		assert.True(t, progressions.OnPath)
		require.NotNil(t, progressions.Level)
		assert.Equal(t, domain.KnowledgeLevelNew, *progressions.Level, "half its items are new")
		assert.Equal(t, domain.PracticeRungHear, progressions.CurrentRung)
		area, _, ok := subject(list, "harmony")
		require.True(t, ok)
		assert.False(t, area.OnPath)
		assert.Nil(t, area.Level)
	})

	t.Run("in the head, only nodes with recall material are offered", func(t *testing.T) {
		f := harmony(t)
		f.playAlong("riff", "Riff", practiceGuitar, "riffs", 100)

		list, err := f.svc.ListSubjects(context.Background(), studentCaller(), nil)

		require.NoError(t, err)
		assert.Nil(t, list.InstrumentID)
		_, area, ok := subject(list, "notes")
		assert.True(t, ok)
		assert.Equal(t, "fretboard", area)
		_, _, ok = subject(list, "riffs")
		assert.False(t, ok, "a play-along is played in hand")
	})

	t.Run("an instrument that doesn't exist is not found", func(t *testing.T) {
		f := harmony(t)

		_, err := f.svc.ListSubjects(context.Background(), studentCaller(), strPtr("missing"))

		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}
