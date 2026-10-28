//go:build integration

package bdd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// Steps of a session's focus: Today's mix subjects and review block, the
// rungs new items come from, a session on a subject the student chose, and
// the subjects they can choose from.
func registerPracticeFocusSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)" has due and weak items on (\d+) skills of their path on guitar$`, w.hasDueAndWeakOnSkills)
	sc.Step(`^"([^"]+)" has (\d+) minutes of due and weak items on "([^"]+)" and (\d+) minutes on "([^"]+)" on guitar$`, w.hasMinutesOfDueAndWeakOn)
	sc.Step(`^"([^"]+)" has due items on (\d+) skills of their path on guitar, (\d+) of them outside the session's focus nodes$`, w.hasDueItemsOutsideSubjects)
	sc.Step(`^"([^"]+)" has due items on (\d+) skills of their path on guitar$`, w.hasDueItemsOnSkills)
	sc.Step(`^"([^"]+)" has due items outside the session's focus nodes, each estimated at (\d+) seconds$`, w.hasDueItemsOutsideEachAt)
	sc.Step(`^"([^"]+)" has due items only on "([^"]+)" on guitar$`, w.hasDueItemsOnlyOn)
	sc.Step(`^"([^"]+)" has due items only on "([^"]+)", "([^"]+)" and "([^"]+)" on guitar, and none of them has a play-along$`, w.hasDueItemsOnlyOnThree)
	sc.Step(`^"([^"]+)"'s path skills "([^"]+)" and "([^"]+)" have new items on guitar$`, w.pathSkillsHaveNewItems)
	sc.Step(`^"([^"]+)" has known items of "([^"]+)" coming due within the week$`, w.hasKnownItemsOfComingDue)
	sc.Step(`^"([^"]+)" is accurate on every know and hear item of "([^"]+)" on guitar$`, w.isAccurateOnKnowAndHear)
	sc.Step(`^"([^"]+)" is accurate on every know item of "([^"]+)" on guitar$`, w.isAccurateOnKnow)
	sc.Step(`^(\d+) know items of "([^"]+)" are due$`, w.knowItemsAreDue)
	sc.Step(`^"([^"]+)" has items only at the know rung$`, w.hasItemsOnlyAtKnow)
	sc.Step(`^"([^"]+)" has never practised "([^"]+)", which has items at the know, hear and play_clean rungs on guitar$`, w.hasNeverPractisedRungItems)
	sc.Step(`^"([^"]+)" has items at the know, hear and play_clean rungs on guitar$`, w.hasRungItems)
	sc.Step(`^"([^"]+)" has items at the know, hear and play_clean rungs, and only its know and hear items need no instrument in hand$`, w.hasRungItems)
	sc.Step(`^every new item of "([^"]+)" in the session is at the (\w+) rung$`, func(node, a string) error { return w.newItemsAreAt(node, a) })
	sc.Step(`^every new item of "([^"]+)" in the session is at the (\w+) or (\w+) rung$`, func(node, a, b string) error { return w.newItemsAreAt(node, a, b) })
	sc.Step(`^"([^"]+)" is accurate on (\d+)% of its cells on guitar, and (\d+) of them are new$`, w.isAccurateOnShareOfCells)

	sc.Step(`^"([^"]+)" composes a (\d+)-minute session with "([a-z-]+)" in hand and "([^"]+)" is a focus node$`, w.composesWithFocusNode)
	sc.Step(`^"([^"]+)" composes a (\d+)-minute session with no instrument in hand and "([^"]+)" is a focus node$`, w.composesInTheHeadWithFocusNode)

	sc.Step(`^the plan names at most (\d+) focus nodes$`, w.planNamesAtMostFocusNodes)
	sc.Step(`^every item other than the warm-up and the review block practises one of them$`, w.everyOtherItemPractisesASubject)
	sc.Step(`^the plan's first focus node is "([^"]+)"$`, w.firstFocusNodeIs)
	sc.Step(`^the plan's focus nodes are "([^"]+)", "([^"]+)" and "([^"]+)", in that order$`, func(a, b, c string) error { return w.focusNodesAre(a, b, c) })
	sc.Step(`^the plan's focus nodes are "([^"]+)" and "([^"]+)", in that order$`, func(a, b string) error { return w.focusNodesAre(a, b) })
	sc.Step(`^the plan's only focus node is "([^"]+)"$`, func(a string) error { return w.focusNodesAre(a) })
	sc.Step(`^the plan says its focus was chosen by the student$`, w.focusWasChosen)
	sc.Step(`^exactly (\d+) items with the reason due practise none of the plan's focus nodes$`, w.reviewBlockHasItems)
	sc.Step(`^they are the most overdue of those (\d+)$`, w.reviewBlockIsMostOverdue)
	sc.Step(`^the due items outside the focus nodes take at most a fifth of the focus time$`, w.reviewBlockWithinFifth)
	sc.Step(`^the due items that practise none of the plan's focus nodes come right after the warm-up, before any other item$`, w.reviewBlockAfterWarmUp)
	sc.Step(`^the (\d+) due know items are in the session with the reason due$`, w.dueKnowItemsInSession)
	sc.Step(`^the session may include new know items of "([^"]+)"$`, w.sessionIncludesNewKnowItemsOf)
	sc.Step(`^every item in the session carries one of the rungs know, hear, play_clean, play_in_time, move, target or use$`, w.everyItemCarriesARung)

	sc.Step(`^the skill "([^"]+)" has items on guitar, and its part_of child "([^"]+)" has items too$`, w.skillAndChildHaveItems)
	sc.Step(`^"([^"]+)" has due, weak and new items on "([^"]+)" on guitar$`, w.hasDueWeakAndNewOn)
	sc.Step(`^"([^"]+)" has due items on "([^"]+)" on guitar$`, w.hasDueItemsOnPathSkill)
	sc.Step(`^"([^"]+)" has the play-along "([^"]+)" on guitar$`, w.nodeHasPlayAlong)
	sc.Step(`^none of "([^"]+)"'s paths includes "([^"]+)", which has items in the head$`, w.offPathNodeHasItemsInTheHead)
	sc.Step(`^"([^"]+)" requires "([^"]+)" at level "([^"]+)", and "([^"]+)" is new on "([^"]+)"$`, w.requiresAndIsNewOn)
	sc.Step(`^no item of "([^"]+)" or its subtree is a play-along on guitar$`, w.nodeHasNoPlayAlong)
	sc.Step(`^"([^"]+)" has (\d+) cells on guitar, all fluent for "([^"]+)" and none due within the week$`, w.hasFluentCellsNotDueSoon)
	sc.Step(`^"([^"]+)" has (\d+) minute of due items, nothing weak and (\d+) new items on "([^"]+)" on guitar$`, w.hasDueMinuteAndNewItemsOn)
	sc.Step(`^"([^"]+)" is practised only by fretboard cells, recalled in the head$`, w.isPractisedOnlyByCells)
	sc.Step(`^"([^"]+)" has only self-rated chord items, played in hand$`, w.hasOnlySelfRatedChords)

	sc.Step(`^"([^"]+)" composes a (\d+)-minute session with "([a-z-]+)" in hand, choosing "([^"]+)"$`, w.composesChoosing)
	sc.Step(`^"([^"]+)" composes a (\d+)-minute session with no instrument in hand, choosing "([^"]+)"$`, w.composesInTheHeadChoosing)
	sc.Step(`^"([^"]+)" composes a (\d+)-minute session with no instrument in hand, choosing a node that doesn't exist$`, w.composesChoosingMissingNode)
	sc.Step(`^"([^"]+)" composes a (\d+)-minute session with no instrument in hand, choosing the node "([^"]+)"$`, w.composesChoosingRawNode)

	sc.Step(`^every item in the session practises "([^"]+)"$`, func(a string) error { return w.everyItemPractises(a) })
	sc.Step(`^every item in the session practises "([^"]+)" or "([^"]+)"$`, func(a, b string) error { return w.everyItemPractises(a, b) })
	sc.Step(`^no item in the session practises "([^"]+)"$`, w.noItemIsOf)
	sc.Step(`^more than (\d+) minutes go to new items of "([^"]+)"$`, w.moreMinutesToNewItemsOf)

	sc.Step(`^"([^"]+)" lists the subjects they can practise with "([a-z-]+)" in hand$`, w.listsSubjectsInHand)
	sc.Step(`^"([^"]+)" lists the subjects they can practise with no instrument in hand$`, w.listsSubjectsInTheHead)
	sc.Step(`^"([^"]+)" is listed in the area "([^"]+)"$`, w.isListedInArea)
	sc.Step(`^"([^"]+)" is listed$`, w.isListed)
	sc.Step(`^"([^"]+)" is not listed$`, w.isNotListed)
	sc.Step(`^every listed node has at least one item that suits "([^"]+)" and can be played in hand$`, w.everyListedNodeHasItems)
	sc.Step(`^"([^"]+)" shows "([^"]+)"'s level on guitar and the current rung (\w+)$`, w.subjectShowsLevelAndRung)
}

// focusSubjectItems is how many due items a seeded subject has: more than
// any other node's, so the session picks it.
const focusSubjectItems = 10

// putTypedExercise seeds a 30-second guitar exercise of exerciseType
// classified under skill and returns its item key.
func (w *world) putTypedExercise(slug, skill string, exerciseType domain.ExerciseType) string {
	seconds := practiceExerciseSeconds
	id := exerciseID(slug).String()
	w.exercises.put(domain.Exercise{
		ID: id, Title: slug, ExerciseType: exerciseType,
		Skills:                   []domain.KnowledgeNode{{ID: w.skillIDFor(skill).String()}},
		EstimatedDurationSeconds: &seconds,
		InstrumentIDs:            []string{instrumentID("guitar").String()},
		CreatedAt:                fixedNow,
	})
	return domain.ExerciseItemKey(id)
}

// dueSince gives name a due state on each of keys, due since days ago.
func (w *world) dueSince(name string, days int, keys ...string) {
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	dueAt := fixedNow.AddDate(0, 0, -days).Add(-time.Hour)
	for _, key := range keys {
		w.practiceStates.put(studentID, domain.PracticeItemState{
			ItemKey: key, RulesVersion: domain.PracticeRulesVersion, Level: domain.KnowledgeLevelLearning,
			Counted: 2, Box: 1, DueAt: &dueAt, LastAt: &fixedNow,
		})
	}
}

// focusSkill names the i-th skill a focus scenario puts on the path.
func focusSkill(i int) string { return fmt.Sprintf("focus-skill-%d", i) }

func (w *world) hasDueAndWeakOnSkills(name string, skills int) error {
	for i := range skills {
		skill := focusSkill(i)
		w.addPathSkill(name, skill)
		w.dueItems(name, w.putGuitarExercises(skill+"-due", skill, 3)...)
		w.weakItems(name, w.putGuitarExercises(skill+"-weak", skill, 3)...)
	}
	return nil
}

func (w *world) hasMinutesOfDueAndWeakOn(name string, firstMinutes int, first string, secondMinutes int, second string) error {
	for _, s := range []struct {
		skill   string
		minutes int
	}{{second, secondMinutes}, {first, firstMinutes}} {
		w.addPathSkill(name, s.skill)
		half := s.minutes * 60 / practiceExerciseSeconds / 2
		w.dueItems(name, w.putGuitarExercises(s.skill+"-due", s.skill, half)...)
		w.weakItems(name, w.putGuitarExercises(s.skill+"-weak", s.skill, half)...)
	}
	return nil
}

// seedSubjectsAndOthers puts name on skills path skills: the first three
// with focusSubjectItems due items each, the session's subjects, and the
// rest sharing others due items, due since 1 to others days ago, most
// overdue remembered first.
func (w *world) seedSubjectsAndOthers(name string, skills, others int) {
	subjects := min(skills, 3)
	for i := range subjects {
		w.addPathSkill(name, focusSkill(i))
		w.dueItems(name, w.putGuitarExercises(focusSkill(i), focusSkill(i), focusSubjectItems)...)
	}
	w.focusOthers = nil
	for j := range others {
		skill := focusSkill(subjects + j%max(skills-subjects, 1))
		w.addPathSkill(name, skill)
		key := w.putPracticeExercise(fmt.Sprintf("%s-other-%02d", skill, j), skill, "guitar")
		w.dueSince(name, 1+j, key)
		w.focusOthers = append([]string{key}, w.focusOthers...)
	}
}

func (w *world) hasDueItemsOutsideSubjects(name string, skills, others int) error {
	w.seedSubjectsAndOthers(name, skills, others)
	return nil
}

func (w *world) hasDueItemsOnSkills(name string, skills int) error {
	w.seedSubjectsAndOthers(name, skills, 2*(skills-3))
	return nil
}

func (w *world) hasDueItemsOutsideEachAt(name string, seconds int) error {
	if seconds != practiceExerciseSeconds {
		return fmt.Errorf("practice exercises are seeded at %d seconds, not %d", practiceExerciseSeconds, seconds)
	}
	w.seedSubjectsAndOthers(name, 4, focusSubjectItems)
	return nil
}

// hasDueItemsOnlyOn makes skill's items the only due ones: the path's own
// play-along is played clean and far from due.
func (w *world) hasDueItemsOnlyOn(name, skill string) error {
	w.cleanGuitarLick(name, 30)
	w.addPathSkill(name, skill)
	w.dueItems(name, w.putGuitarExercises(skill+"-due", skill, 4)...)
	return nil
}

// hasDueItemsOnlyOnThree makes the three skills' items the only due ones,
// none of them with a play-along: the path's own play-along is played
// clean and far from due.
func (w *world) hasDueItemsOnlyOnThree(name, first, second, third string) error {
	w.cleanGuitarLick(name, 30)
	for _, skill := range []string{first, second, third} {
		w.addPathSkill(name, skill)
		w.dueItems(name, w.putGuitarExercises(skill+"-due", skill, 4)...)
	}
	return nil
}

func (w *world) pathSkillsHaveNewItems(name, first, second string) error {
	for _, skill := range []string{first, second} {
		w.addPathSkill(name, skill)
		w.putGuitarExercises(skill+"-new", skill, 4)
	}
	return nil
}

func (w *world) hasKnownItemsOfComingDue(name, node string) error {
	w.addPathSkill(name, node)
	w.knownItems(name, 3, w.putGuitarExercises(node+"-known", node, 4)...)
	return nil
}

// rungItems is how many items a rung scenario seeds at each rung.
const rungItems = 5

// putRungItems puts skill on name's path with rungItems know (text) and
// hear (listening) guitar exercises, and returns their item keys.
func (w *world) putRungItems(name, skill string) (know, hear []string) {
	w.addPathSkill(name, skill)
	for i := range rungItems {
		know = append(know, w.putTypedExercise(fmt.Sprintf("%s-know-%02d", skill, i), skill, domain.ExerciseTypeTextResponse))
		hear = append(hear, w.putTypedExercise(fmt.Sprintf("%s-hear-%02d", skill, i), skill, domain.ExerciseTypeAudioRecognition))
	}
	return know, hear
}

// hasRungItems gives skill play_clean items on top of its know and hear
// ones: every catalog chord voicing, played in hand, which an exercise needn't be.
func (w *world) hasRungItems(skill string) error {
	w.putRungItems("alice", skill)
	w.classifyChords("guitar", skill, allVoicingKeys()...)
	return nil
}

func (w *world) hasNeverPractisedRungItems(name, skill string) error {
	w.putRungItems(name, skill)
	w.classifyChords("guitar", skill, allVoicingKeys()...)
	return nil
}

// accurateItems gives name an accurate state on keys, due in 5 days.
func (w *world) accurateItems(name string, keys ...string) {
	w.putItemStates(name, domain.KnowledgeLevelAccurate, 3, 2, 5, keys...)
}

func (w *world) isAccurateOnKnowAndHear(name, skill string) error {
	know, hear := w.putRungItems(name, skill)
	w.accurateItems(name, append(know, hear...)...)
	return nil
}

func (w *world) isAccurateOnKnow(name, skill string) error {
	know, _ := w.putRungItems(name, skill)
	w.accurateItems(name, know...)
	return nil
}

func (w *world) knowItemsAreDue(count int, skill string) error {
	w.focusDueKeys = nil
	for i := range count {
		w.focusDueKeys = append(w.focusDueKeys, w.putTypedExercise(fmt.Sprintf("%s-due-know-%02d", skill, i), skill, domain.ExerciseTypeTextResponse))
	}
	w.dueItems("alice", w.focusDueKeys...)
	return nil
}

// topRungCells is how many fretboard cells a top-rung scenario seeds.
const topRungCells = 50

func (w *world) hasItemsOnlyAtKnow(skill string) error {
	w.focusNode = skill
	w.putCells("guitar", skill, topRungCells/5, 5, 4, 3, 2, 1)
	return nil
}

// isAccurateOnShareOfCells makes name accurate on percent of the cells of
// the node the scenario set up, leaving fresh of them new. The accurate ones were found right before, so
// they are asked the other way and leave the new cells room in the
// session.
func (w *world) isAccurateOnShareOfCells(name string, percent, fresh int) error {
	skill := w.focusNode
	var keys []string
	for _, item := range w.practiceItems.cells[instrumentID("guitar").String()] {
		if slices.Contains(item.NodeIDs, w.skillIDFor(skill).String()) {
			keys = append(keys, item.ItemKey)
		}
	}
	accurate := len(keys) * percent / 100
	if accurate+fresh != len(keys) {
		return fmt.Errorf("%d%% of %d cells and %d new don't add up", percent, len(keys), fresh)
	}
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	dueAt := fixedNow.AddDate(0, 0, 5)
	for _, key := range keys[:accurate] {
		w.practiceStates.put(studentID, domain.PracticeItemState{
			ItemKey: key, RulesVersion: domain.PracticeRulesVersion, Level: domain.KnowledgeLevelAccurate,
			Counted: 3, Box: 2, DueAt: &dueAt, LastAt: &fixedNow,
			RightByResponse: map[string]int{string(domain.FretboardDrillNameTheNote): 3},
		})
	}
	w.addPathSkill(name, skill)
	return nil
}

func (w *world) composesWithFocusNode(name string, minutes int, instrument, node string) error {
	if err := w.composesSession(name, minutes, instrument); err != nil {
		return err
	}
	return w.isAFocusNode(node)
}

func (w *world) composesInTheHeadWithFocusNode(_ string, minutes int, node string) error {
	if err := w.composeSession(minutes, nil); err != nil {
		return err
	}
	return w.isAFocusNode(node)
}

// isAFocusNode checks the composed plan has node among its subjects, as a
// scenario's premise.
func (w *world) isAFocusNode(node string) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	if !slices.Contains(focusNodeIDs(plan), w.nodeIDByName(node)) {
		return fmt.Errorf("expected %q among the plan's focus nodes, got %v", node, focusNodeIDs(plan))
	}
	return nil
}

func focusNodeIDs(plan generated.PracticeSessionPlan) []openapi_types.UUID {
	ids := make([]openapi_types.UUID, len(plan.FocusNodes))
	for i, n := range plan.FocusNodes {
		ids[i] = n.NodeId
	}
	return ids
}

// reviewBlock lists the plan's due items on none of its focus nodes.
func reviewBlock(plan generated.PracticeSessionPlan) []generated.PracticeSessionItem {
	focus := focusNodeIDs(plan)
	var block []generated.PracticeSessionItem
	for _, item := range plan.Items {
		if item.Reason == generated.PracticePickReasonDue && (item.NodeId == nil || !slices.Contains(focus, *item.NodeId)) {
			block = append(block, item)
		}
	}
	return block
}

func (w *world) planNamesAtMostFocusNodes(most int) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	if len(plan.FocusNodes) == 0 || len(plan.FocusNodes) > most {
		return fmt.Errorf("expected 1 to %d focus nodes, got %d", most, len(plan.FocusNodes))
	}
	return nil
}

func (w *world) everyOtherItemPractisesASubject() error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	focus := focusNodeIDs(plan)
	for _, item := range plan.Items {
		if item.Reason == generated.PracticePickReasonWarmUp || item.Reason == generated.PracticePickReasonDue {
			continue
		}
		if item.NodeId == nil || !slices.Contains(focus, *item.NodeId) {
			return fmt.Errorf("%s (%s) practises none of the focus nodes %v", item.ItemKey, item.Reason, focus)
		}
	}
	return nil
}

func (w *world) firstFocusNodeIs(node string) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	if len(plan.FocusNodes) == 0 || plan.FocusNodes[0].NodeId != w.nodeIDByName(node) {
		return fmt.Errorf("expected %q to be the first focus node, got %v", node, focusNodeIDs(plan))
	}
	return nil
}

func (w *world) focusNodesAre(nodes ...string) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	want := make([]openapi_types.UUID, len(nodes))
	for i, n := range nodes {
		want[i] = w.nodeIDByName(n)
	}
	if !slices.Equal(want, focusNodeIDs(plan)) {
		return fmt.Errorf("expected the focus nodes %v (%v), got %v", nodes, want, focusNodeIDs(plan))
	}
	return nil
}

func (w *world) focusWasChosen() error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	if plan.FocusSource != generated.PracticeFocusSource(domain.PracticeFocusChosen) {
		return fmt.Errorf("expected the focus to be chosen, got %s", plan.FocusSource)
	}
	return nil
}

func (w *world) reviewBlockHasItems(count int) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	if block := reviewBlock(plan); len(block) != count {
		return fmt.Errorf("expected %d due items outside the focus nodes, got %d", count, len(block))
	}
	return nil
}

func (w *world) reviewBlockIsMostOverdue(of int) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	if len(w.focusOthers) != of {
		return fmt.Errorf("the scenario seeded %d items outside the focus nodes, not %d", len(w.focusOthers), of)
	}
	block := reviewBlock(plan)
	for _, item := range block {
		if !slices.Contains(w.focusOthers[:len(block)], item.ItemKey) {
			return fmt.Errorf("%s is not among the %d most overdue of %v", item.ItemKey, len(block), w.focusOthers)
		}
	}
	return nil
}

func (w *world) reviewBlockWithinFifth() error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	seconds := 0
	for _, item := range reviewBlock(plan) {
		seconds += item.EstimatedSeconds
	}
	if focus := focusSeconds(plan); seconds == 0 || seconds*5 > focus {
		return fmt.Errorf("expected the review block to take some but at most a fifth of %d seconds of focus time, got %d", focus, seconds)
	}
	return nil
}

func (w *world) reviewBlockAfterWarmUp() error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	block := reviewBlock(plan)
	if len(block) == 0 || plan.Items[0].Reason != generated.PracticePickReasonWarmUp {
		return fmt.Errorf("expected a warm-up and a review block, got %d review items after %s", len(block), plan.Items[0].Reason)
	}
	for i, item := range block {
		if plan.Items[1+i].ItemKey != item.ItemKey {
			return fmt.Errorf("expected the review block right after the warm-up, but item %d is %s", 1+i, plan.Items[1+i].ItemKey)
		}
	}
	return nil
}

func (w *world) dueKnowItemsInSession(count int) error {
	if len(w.focusDueKeys) != count {
		return fmt.Errorf("the scenario made %d know items due, not %d", len(w.focusDueKeys), count)
	}
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	for _, key := range w.focusDueKeys {
		i := slices.IndexFunc(plan.Items, func(item generated.PracticeSessionItem) bool { return item.ItemKey == key })
		if i < 0 || plan.Items[i].Reason != generated.PracticePickReasonDue {
			return fmt.Errorf("expected %s in the session with the reason due", key)
		}
	}
	return nil
}

func (w *world) sessionIncludesNewKnowItemsOf(node string) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	for _, item := range plan.Items {
		if item.NodeId != nil && *item.NodeId == w.nodeIDByName(node) && item.Rung == generated.PracticeRung(domain.PracticeRungKnow) &&
			(item.Reason == generated.PracticePickReasonNew || item.Reason == generated.PracticePickReasonStretch) {
			return nil
		}
	}
	return fmt.Errorf("expected new know items of %q in the session", node)
}

func (w *world) everyItemCarriesARung() error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	for _, item := range plan.Items {
		if !domain.PracticeRung(item.Rung).Valid() {
			return fmt.Errorf("%s carries no rung of the ladder: %q", item.ItemKey, item.Rung)
		}
	}
	return nil
}

// skillAndChildHaveItems puts skill, part_of the area harmony, with 4
// guitar exercises, and its child, with 4 more.
func (w *world) skillAndChildHaveItems(skill, child string) error {
	area := w.skillIDFor("harmony")
	parent := w.putSkill(skill, &area)
	w.putSkill(child, &parent)
	w.putGuitarExercises(skill, skill, 4)
	w.putGuitarExercises(child, child, 4)
	return nil
}

func (w *world) hasDueWeakAndNewOn(name, node string) error {
	w.dueItems(name, w.putGuitarExercises(node+"-due", node, plentyOfItems)...)
	w.weakItems(name, w.putGuitarExercises(node+"-weak", node, plentyOfItems)...)
	w.putGuitarExercises(node+"-new", node, plentyOfItems)
	return nil
}

func (w *world) hasDueItemsOnPathSkill(name, skill string) error {
	w.addPathSkill(name, skill)
	w.dueItems(name, w.putGuitarExercises(skill+"-due", skill, 4)...)
	return nil
}

func (w *world) nodeHasPlayAlong(node, slug string) error {
	w.putPlayAlongOn(slug, "guitar", node, 100)
	return nil
}

// offPathNodeHasItemsInTheHead gives node 4 exercises for every instrument,
// and no path.
func (w *world) offPathNodeHasItemsInTheHead(_, node string) error {
	for i := range 4 {
		w.putPracticeExercise(fmt.Sprintf("%s-%02d", node, i), node)
	}
	return nil
}

func (w *world) requiresAndIsNewOn(from, to, level, _, _ string) error {
	if err := w.skillRequiresSkill(from, to, level); err != nil {
		return err
	}
	for i := range 4 {
		w.putPracticeExercise(fmt.Sprintf("%s-%02d", from, i), from)
		w.putPracticeExercise(fmt.Sprintf("%s-%02d", to, i), to)
	}
	return nil
}

// nodeHasNoPlayAlong gives node exercises only, while the path's skill
// keeps its play-along.
func (w *world) nodeHasNoPlayAlong(node string) error {
	w.putGuitarExercises(node, node, 4)
	return nil
}

func (w *world) hasFluentCellsNotDueSoon(skill string, count int, name string) error {
	if count%2 != 0 {
		return fmt.Errorf("cells are seeded on two strings: %d is odd", count)
	}
	w.putCells("guitar", skill, count/2, 6, 5)
	var keys []string
	for _, item := range w.practiceItems.cells[instrumentID("guitar").String()] {
		if slices.Contains(item.NodeIDs, w.skillIDFor(skill).String()) {
			keys = append(keys, item.ItemKey)
		}
	}
	w.knownItems(name, 30, keys...)
	return nil
}

func (w *world) hasDueMinuteAndNewItemsOn(name string, minutes, fresh int, node string) error {
	w.dueItems(name, w.putGuitarExercises(node+"-due", node, minutes*60/practiceExerciseSeconds)...)
	w.putGuitarExercises(node+"-new", node, fresh)
	return nil
}

func (w *world) isPractisedOnlyByCells(skill string) error {
	w.putCells("guitar", skill, 12, 6, 5)
	return nil
}

// hasOnlySelfRatedChords classifies every catalog voicing under skill on
// guitar: chords are played on the instrument and rated, never recalled on
// the screen.
func (w *world) hasOnlySelfRatedChords(skill string) error {
	w.classifyChords("guitar", skill, allVoicingKeys()...)
	return nil
}

func (w *world) composeChoosing(minutes int, instrument *openapi_types.UUID, node openapi_types.UUID) error {
	resp, err := w.handler.CreatePracticeSessionPlan(w.ctx(), generated.CreatePracticeSessionPlanRequestObject{
		Body: &generated.CreatePracticeSessionPlanRequest{InstrumentId: instrument, Minutes: minutes, FocusNodeId: &node},
	})
	w.lastResp, w.lastErr = resp, err
	return nil
}

func (w *world) composesChoosing(_ string, minutes int, instrument, node string) error {
	id := instrumentID(instrument)
	return w.composeChoosing(minutes, &id, w.nodeIDByName(node))
}

func (w *world) composesInTheHeadChoosing(_ string, minutes int, node string) error {
	return w.composeChoosing(minutes, nil, w.nodeIDByName(node))
}

func (w *world) composesChoosingMissingNode(_ string, minutes int) error {
	return w.composeChoosing(minutes, nil, uuid.NewSHA1(uuid.NameSpaceOID, []byte("no-such-node")))
}

// composesChoosingRawNode sends the plan request through the generated
// router, since only the router can receive a node that is not a UUID.
func (w *world) composesChoosingRawNode(_ string, minutes int, node string) error {
	body, err := json.Marshal(struct {
		Minutes     int    `json:"minutes"`
		FocusNodeID string `json:"focus_node_id"`
	}{minutes, node})
	if err != nil {
		return err
	}
	router := generated.HandlerWithOptions(generated.NewStrictHandler(w.handler, nil), generated.ChiServerOptions{})
	req := httptest.NewRequestWithContext(w.ctx(), http.MethodPost, "/students/me/practice-sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	w.lastResp, w.lastErr = nil, nil
	w.lastRouterStatus = rec.Code
	return nil
}

// newItemsAreAt checks that the session has new items of node, every one at
// one of rungs.
func (w *world) newItemsAreAt(node string, rungs ...string) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	var seen []string
	for _, item := range plan.Items {
		if item.NodeId == nil || *item.NodeId != w.nodeIDByName(node) || item.Reason != generated.PracticePickReasonNew {
			continue
		}
		if !slices.Contains(rungs, string(item.Rung)) {
			return fmt.Errorf("new item %s of %q is at the %s rung, not %v", item.ItemKey, node, item.Rung, rungs)
		}
		seen = append(seen, string(item.Rung))
	}
	if len(seen) == 0 {
		return fmt.Errorf("expected new items of %q in the session, got none", node)
	}
	return nil
}

func (w *world) everyItemPractises(nodes ...string) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	var ids []openapi_types.UUID
	for _, n := range nodes {
		ids = append(ids, w.nodeIDByName(n))
	}
	for _, item := range plan.Items {
		if item.NodeId == nil || !slices.Contains(ids, *item.NodeId) {
			return fmt.Errorf("%s practises none of %v", item.ItemKey, nodes)
		}
	}
	return nil
}

func (w *world) moreMinutesToNewItemsOf(minutes int, node string) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	seconds := 0
	for _, item := range plan.Items {
		if item.Reason == generated.PracticePickReasonNew && item.NodeId != nil && *item.NodeId == w.nodeIDByName(node) {
			seconds += item.EstimatedSeconds
		}
	}
	if seconds <= minutes*60 {
		return fmt.Errorf("expected more than %d minutes of new items of %q, got %d seconds", minutes, node, seconds)
	}
	return nil
}

func (w *world) listSubjects(instrument *openapi_types.UUID) error {
	resp, err := w.handler.ListPracticeSubjects(w.ctx(), generated.ListPracticeSubjectsRequestObject{
		Params: generated.ListPracticeSubjectsParams{InstrumentId: instrument},
	})
	w.lastResp, w.lastErr = resp, err
	return nil
}

func (w *world) listsSubjectsInHand(_, instrument string) error {
	id := instrumentID(instrument)
	return w.listSubjects(&id)
}

func (w *world) listsSubjectsInTheHead(string) error {
	return w.listSubjects(nil)
}

func (w *world) subjectList() (generated.PracticeSubjectList, error) {
	if w.lastErr != nil {
		return generated.PracticeSubjectList{}, w.lastErr
	}
	list, ok := w.lastResp.(generated.ListPracticeSubjects200JSONResponse)
	if !ok {
		return generated.PracticeSubjectList{}, fmt.Errorf("expected the subjects list, got %#v", w.lastResp)
	}
	return generated.PracticeSubjectList(list), nil
}

// listedSubject finds node in the subjects list, with its area.
func (w *world) listedSubject(node string) (generated.PracticeSubject, openapi_types.UUID, error) {
	list, err := w.subjectList()
	if err != nil {
		return generated.PracticeSubject{}, openapi_types.UUID{}, err
	}
	for _, g := range list.Groups {
		for _, s := range g.Subjects {
			if s.NodeId == w.nodeIDByName(node) {
				return s, g.AreaNodeId, nil
			}
		}
	}
	return generated.PracticeSubject{}, openapi_types.UUID{}, fmt.Errorf("%q is not listed", node)
}

func (w *world) isListedInArea(node, area string) error {
	_, got, err := w.listedSubject(node)
	if err != nil {
		return err
	}
	if got != w.nodeIDByName(area) {
		return fmt.Errorf("expected %q in the area %q, got %v", node, area, got)
	}
	return nil
}

func (w *world) isListed(node string) error {
	_, _, err := w.listedSubject(node)
	return err
}

func (w *world) isNotListed(node string) error {
	list, err := w.subjectList()
	if err != nil {
		return err
	}
	for _, g := range list.Groups {
		for _, s := range g.Subjects {
			if s.NodeId == w.nodeIDByName(node) {
				return fmt.Errorf("expected %q not to be listed", node)
			}
		}
	}
	return nil
}

func (w *world) everyListedNodeHasItems(string) error {
	list, err := w.subjectList()
	if err != nil {
		return err
	}
	if len(list.Groups) == 0 {
		return fmt.Errorf("expected subjects to be listed")
	}
	for _, g := range list.Groups {
		for _, s := range g.Subjects {
			if s.ItemCount < 1 {
				return fmt.Errorf("%v is listed with no item", s.NodeId)
			}
		}
	}
	return nil
}

func (w *world) subjectShowsLevelAndRung(node, _, rung string) error {
	s, _, err := w.listedSubject(node)
	if err != nil {
		return err
	}
	if s.Level == nil {
		return fmt.Errorf("expected %q to show the student's level", node)
	}
	if string(s.CurrentRung) != rung {
		return fmt.Errorf("expected %q at the rung %s, got %s", node, rung, s.CurrentRung)
	}
	return nil
}
