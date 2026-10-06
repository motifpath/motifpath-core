//go:build integration

package bdd

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// fakePracticeActivity is an in-memory ports.PracticeActivityReader. It
// keeps every session raw, as the worker does, and finds the finished ones
// when read.
type fakePracticeActivity struct {
	mu          sync.Mutex
	sessions    map[string][]fakePracticeSession
	completions map[string][]time.Time
	snapshots   map[string][]fakeItemSnapshot
}

// fakePracticeSession is one session as the worker records it: endedAt
// is nil until it ends, which an abandoned session never does.
type fakePracticeSession struct {
	instrumentID *string
	endedAt      *time.Time
	leftEarly    bool
}

// fakeItemSnapshot is an item's state at the end of the UTC day starting
// at day.
type fakeItemSnapshot struct {
	day      time.Time
	snapshot domain.PracticeItemSnapshot
}

func newFakePracticeActivity() *fakePracticeActivity {
	return &fakePracticeActivity{sessions: map[string][]fakePracticeSession{}, completions: map[string][]time.Time{}, snapshots: map[string][]fakeItemSnapshot{}}
}

func (f *fakePracticeActivity) FinishedSessions(_ context.Context, studentID string, since time.Time) ([]domain.FinishedPracticeSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.FinishedPracticeSession
	for _, s := range f.sessions[studentID] {
		if s.endedAt != nil && !s.leftEarly && !s.endedAt.Before(since) {
			out = append(out, domain.FinishedPracticeSession{InstrumentID: s.instrumentID, EndedAt: *s.endedAt})
		}
	}
	return out, nil
}

func (f *fakePracticeActivity) CompletionTimes(_ context.Context, studentID string, since time.Time) ([]time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []time.Time
	for _, t := range f.completions[studentID] {
		if !t.Before(since) {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakePracticeActivity) SnapshotsAt(_ context.Context, studentID string, itemKeys []string, at time.Time) (map[string]domain.PracticeItemSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	latest := map[string]fakeItemSnapshot{}
	for _, s := range f.snapshots[studentID] {
		key := s.snapshot.ItemKey
		if !slices.Contains(itemKeys, key) || s.day.Add(24*time.Hour).After(at) {
			continue
		}
		if prev, ok := latest[key]; !ok || s.day.After(prev.day) {
			latest[key] = s
		}
	}
	out := map[string]domain.PracticeItemSnapshot{}
	for key, s := range latest {
		out[key] = s.snapshot
	}
	return out, nil
}

func (f *fakePracticeActivity) addSession(studentID string, s fakePracticeSession) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessions[studentID] = append(f.sessions[studentID], s)
}

func (f *fakePracticeActivity) addCompletion(studentID string, at time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.completions[studentID] = append(f.completions[studentID], at)
}

func (f *fakePracticeActivity) addSnapshot(studentID string, day time.Time, s domain.PracticeItemSnapshot) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.snapshots[studentID] = append(f.snapshots[studentID], fakeItemSnapshot{day: day, snapshot: s})
}

// monday is the Monday before fixedNow, a Wednesday, in the scenarios'
// calendar, and tuesday the day after it.
var (
	monday  = time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	tuesday = monday.AddDate(0, 0, 1)
)

func registerPracticeSummarySteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^student "([^"]+)" plays "([^"]+)" and "([^"]+)" in time zone "([^"]+)"$`, w.studentPlaysInTimeZone)
	sc.Step(`^"([^"]+)" practised on (\d+) of the last 7 days$`, w.practisedOnDays)
	sc.Step(`^"([^"]+)" practised on (\d+) consecutive days and then missed a day$`, w.practisedConsecutiveDays)
	sc.Step(`^"([^"]+)"'s only session yesterday ended early$`, w.onlySessionYesterdayEndedEarly)
	sc.Step(`^"([^"]+)"'s only session yesterday, with "([^"]+)" in hand, ended early$`, w.onlySessionYesterdayWithEndedEarly)
	sc.Step(`^"([^"]+)" started a (\d+)-minute session yesterday and sent nothing for it after the first (\d+) minutes$`, w.abandonedSessionYesterday)
	sc.Step(`^"([^"]+)" finished a session with "([^"]+)" in hand yesterday$`, w.finishedSessionYesterday)
	sc.Step(`^"([^"]+)" practised at 23:30 on Monday in "([^"]+)", which is Tuesday in UTC$`, w.practisedLateMonday)
	sc.Step(`^"([^"]+)" finished a session with "([^"]+)" in hand on Monday and with "([^"]+)" in hand on Tuesday$`, w.finishedOnMondayAndTuesday)
	sc.Step(`^"([^"]+)" finished a session with "([^"]+)" in hand and another with "([^"]+)" in hand on Monday$`, w.finishedTwoOnMonday)
	sc.Step(`^"([^"]+)" finished sessions with "([^"]+)" in hand on (\d+) days and with "([^"]+)" in hand on (\d+) days?$`, w.finishedSessionsOnDays)
	sc.Step(`^"([^"]+)" completed content nodes on (\d+) of the last 7 days$`, w.completedOnDays)
	sc.Step(`^"([^"]+)" completed a content node yesterday$`, w.completedYesterday)
	sc.Step(`^"([^"]+)" completed a content node at 23:30 on Monday in "([^"]+)", which is Tuesday in UTC$`, w.completedLateMonday)

	sc.Step(`^"([^"]+)"'s accuracy on "([^"]+)" was ([\d.]+) seven days ago and is ([\d.]+) now$`, w.accuracyWasAndIs)
	sc.Step(`^"([^"]+)" first practised "([^"]+)" (\d+) days ago and their accuracy on it is ([\d.]+) now$`, w.firstPractisedDaysAgo)
	sc.Step(`^"([^"]+)" had no clean take on "([^"]+)" seven days ago and their best clean tempo on it is (\d+) BPM now$`, w.noCleanTakeThenTempoNow)
	sc.Step(`^"([^"]+)"'s accuracy on the concept "([^"]+)" improved this week$`, w.conceptAccuracyImproved)
	sc.Step(`^the concept "([^"]+)" has exercises for every instrument$`, w.conceptHasExercises)
	sc.Step(`^"([^"]+)" has (\d+) skills to refresh, strengthen or start on guitar$`, w.hasSkillsToStart)
	sc.Step(`^"([^"]+)"'s skill "([^"]+)" on guitar rests on items whose review is due$`, w.skillRestsOnDueItems)
	sc.Step(`^"([^"]+)" has a skill to strengthen and a skill ready to start on guitar$`, w.hasSkillToStrengthenAndToStart)
	sc.Step(`^"([^"]+)"'s first next step on guitar is to strengthen "([^"]+)"$`, w.firstNextStepIsToStrengthen)
	sc.Step(`^"([^"]+)" has no next step on "([^"]+)"$`, func(string, string) error { return nil })
	sc.Step(`^"([^"]+)" has never practised$`, w.hasNeverPractisedAnything)
	sc.Step(`^"([^"]+)" is also enrolled in a music-theory path for every instrument$`, w.enrolledInPathForEveryInstrument)
	sc.Step(`^student "([^"]+)" is enrolled only in a music-theory path for every instrument$`, w.onlyEnrolledInPathForEveryInstrument)
	sc.Step(`^the skill "([^"]+)" has practice items only for "([^"]+)"$`, w.skillHasItemsOnlyFor)
	sc.Step(`^"([^"]+)" is ready to start "([^"]+)"$`, w.isReadyToStartPathSkill)

	sc.Step(`^"([^"]+)" reads their practice summary for "([^"]+)"$`, w.readsSummary)
	sc.Step(`^"([^"]+)" reads their practice summary for "([^"]+)" in time zone "([^"]+)"$`, w.readsSummaryInTimeZone)
	sc.Step(`^"([^"]+)" reads their practice summary for an instrument that doesn't exist$`, w.readsSummaryForMissingInstrument)
	sc.Step(`^"([^"]+)" reads their practice overview$`, w.readsOverview)
	sc.Step(`^"([^"]+)" reads their practice overview in time zone "([^"]+)"$`, w.readsOverviewInTimeZone)

	sc.Step(`^the summary shows (\d+) practice days? in the last 7$`, w.summaryShowsPracticeDays)
	sc.Step(`^the overview shows (\d+) practice days? in the last 7$`, w.overviewShowsPracticeDays)
	sc.Step(`^the overview shows (\d+) learning days? in the last 7$`, w.overviewShowsLearningDays)
	sc.Step(`^the progress this week shows "([^"]+)" accuracy from ([\d.]+) to ([\d.]+)$`, w.progressShowsAccuracy)
	sc.Step(`^the progress this week has no tempo line for the skill of "([^"]+)"$`, w.progressHasNoTempoLineFor)
	sc.Step(`^the progress this week has no line for "([^"]+)"$`, w.progressHasNoLineFor)
	sc.Step(`^the progress this week is empty$`, w.progressIsEmpty)
	sc.Step(`^the summary shows (\d+) next steps and a total of (\d+)$`, w.summaryShowsNextSteps)
	sc.Step(`^the summary lists (?:only )?"([^"]+)" and "([^"]+)" as their instruments$`, w.summaryListsInstruments)
	sc.Step(`^the next steps are "([^"]+)" to refresh, then the skill to strengthen, then the skill ready to start$`, w.nextStepsAreRefreshStrengthenStart)
	sc.Step(`^"([^"]+)" is not among the next steps$`, w.notAmongNextSteps)
	sc.Step(`^the next steps include "([^"]+)" as ready to start$`, w.nextStepsIncludeReadyToStart)
	sc.Step(`^the next steps start with skills they are ready to start$`, w.nextStepsStartWithReadyToStart)
	sc.Step(`^"([^"]+)" is in the "Any instrument" group, not in a guitar area$`, w.isInAnyInstrumentGroup)
	sc.Step(`^that practice counts on Monday$`, w.practiceCountsOnMonday)
	sc.Step(`^that completion counts on Monday$`, w.completionCountsOnMonday)
	sc.Step(`^the "([^"]+)" card shows (\d+) practice days? and the next step to strengthen "([^"]+)"$`, w.cardShowsDaysAndStrengthen)
	sc.Step(`^the "([^"]+)" card shows (\d+) practice days?$`, w.cardShowsDays)
	sc.Step(`^the "([^"]+)" card has no next step$`, w.cardHasNoNextStep)
	sc.Step(`^the overview has no instrument cards$`, w.overviewHasNoCards)
}

// ── Givens ───────────────────────────────────────────────────────────────

// studentPlaysInTimeZone enrolls name in a path for both instruments that
// teaches no skill, so the scenario alone decides what there is to
// practise.
func (w *world) studentPlaysInTimeZone(name, first, second, timeZone string) error {
	if err := w.studentPlays(name, first, second); err != nil {
		return err
	}
	w.timeZone = timeZone
	w.enrollInTemplate(name, "instruments", instrumentID(first).String(), instrumentID(second).String())
	return nil
}

// enrollInTemplate gives name an active standalone path copied from a
// template for instrumentIDs (none: every instrument) that teaches no
// skill.
func (w *world) enrollInTemplate(name, template string, instrumentIDs ...string) {
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	templateID := pathID("summary-template-" + template).String()
	w.paths.put(domain.LearningPath{ID: templateID, Title: template, Status: domain.LearningPathStatusPublished, InstrumentIDs: instrumentIDs})
	node := domain.ContentNode{ID: nodeID("summary-lesson-" + template).String(), Title: "summary-lesson-" + template, ContentType: domain.ContentTypeArticle}
	w.nodes.put(node)
	w.studentPaths.put(domain.StudentPath{
		ID:               deterministicUUID("student-path", name+"/"+template).String(),
		StudentID:        studentID,
		SourceTemplateID: templateID,
		AssignedAt:       fixedNow.Add(-time.Hour),
		Items:            []domain.StudentPathItemRecord{{Position: 1, ContentNodeID: node.ID}},
	})
}

// location is the scenario student's time zone.
func (w *world) location() *time.Location {
	loc, err := time.LoadLocation(w.timeZone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// localTime is hour:minute on the day daysAgo days before the summary's
// today, in the student's time zone.
func (w *world) localTime(daysAgo, hour, minute int) time.Time {
	y, m, d := w.summaryNow.In(w.location()).Date()
	return time.Date(y, m, d-daysAgo, hour, minute, 0, 0, w.location())
}

func (w *world) finishSession(name, instrument string, at time.Time) {
	id := instrumentID(instrument).String()
	w.practiceActivity.addSession(w.ensureRegistered(name, domain.RoleStudent).String(), fakePracticeSession{instrumentID: &id, endedAt: &at})
}

func (w *world) practisedOnDays(name string, days int) error {
	for i := range days {
		w.finishSession(name, "guitar", w.localTime(i*2%7, 8, 0))
	}
	return nil
}

func (w *world) practisedConsecutiveDays(name string, days int) error {
	for i := 1; i <= days; i++ {
		w.finishSession(name, "guitar", w.localTime(i, 8, 0))
	}
	return nil
}

func (w *world) onlySessionYesterdayEndedEarly(name string) error {
	return w.onlySessionYesterdayWithEndedEarly(name, "guitar")
}

func (w *world) onlySessionYesterdayWithEndedEarly(name, instrument string) error {
	id, at := instrumentID(instrument).String(), w.localTime(1, 8, 0)
	w.practiceActivity.addSession(w.ensureRegistered(name, domain.RoleStudent).String(), fakePracticeSession{instrumentID: &id, endedAt: &at, leftEarly: true})
	return nil
}

func (w *world) abandonedSessionYesterday(name string, _, _ int) error {
	id := instrumentID("guitar").String()
	w.practiceActivity.addSession(w.ensureRegistered(name, domain.RoleStudent).String(), fakePracticeSession{instrumentID: &id})
	return nil
}

func (w *world) finishedSessionYesterday(name, instrument string) error {
	w.finishSession(name, instrument, w.localTime(1, 8, 0))
	return nil
}

// lateMonday is 23:30 on Monday in timeZone.
func lateMonday(timeZone string) (time.Time, error) {
	loc, err := time.LoadLocation(timeZone)
	if err != nil {
		return time.Time{}, err
	}
	at := time.Date(monday.Year(), monday.Month(), monday.Day(), 23, 30, 0, 0, loc)
	if at.UTC().Day() != tuesday.Day() {
		return time.Time{}, fmt.Errorf("23:30 on Monday in %s is not Tuesday in UTC", timeZone)
	}
	return at, nil
}

func (w *world) practisedLateMonday(name, timeZone string) error {
	at, err := lateMonday(timeZone)
	if err != nil {
		return err
	}
	w.finishSession(name, "guitar", at)
	return nil
}

func (w *world) finishedOnMondayAndTuesday(name, first, second string) error {
	w.finishSession(name, first, time.Date(monday.Year(), monday.Month(), monday.Day(), 9, 0, 0, 0, w.location()))
	w.finishSession(name, second, time.Date(tuesday.Year(), tuesday.Month(), tuesday.Day(), 9, 0, 0, 0, w.location()))
	return nil
}

func (w *world) finishedTwoOnMonday(name, first, second string) error {
	w.finishSession(name, first, time.Date(monday.Year(), monday.Month(), monday.Day(), 9, 0, 0, 0, w.location()))
	w.finishSession(name, second, time.Date(monday.Year(), monday.Month(), monday.Day(), 18, 0, 0, 0, w.location()))
	return nil
}

func (w *world) finishedSessionsOnDays(name, first string, firstDays int, second string, secondDays int) error {
	for i := range firstDays {
		w.finishSession(name, first, w.localTime(i, 8, 0))
	}
	for i := range secondDays {
		w.finishSession(name, second, w.localTime(i, 9, 0))
	}
	return nil
}

func (w *world) completedOnDays(name string, days int) error {
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	for i := range days {
		w.practiceActivity.addCompletion(studentID, w.localTime(i, 10, 0))
		w.practiceActivity.addCompletion(studentID, w.localTime(i, 11, 0))
	}
	return nil
}

func (w *world) completedYesterday(name string) error {
	w.practiceActivity.addCompletion(w.ensureRegistered(name, domain.RoleStudent).String(), w.localTime(1, 10, 0))
	return nil
}

func (w *world) completedLateMonday(name, timeZone string) error {
	at, err := lateMonday(timeZone)
	if err != nil {
		return err
	}
	w.practiceActivity.addCompletion(w.ensureRegistered(name, domain.RoleStudent).String(), at)
	return nil
}

// snapshotDayBefore is the UTC day whose end is the summary's state seven
// days ago: the day before the last 7 calendar days began.
func (w *world) snapshotDayBefore() time.Time {
	start := w.localTime(6, 0, 0)
	return time.Date(start.Year(), start.Month(), start.Day()-2, 0, 0, 0, 0, time.UTC)
}

// practisedItem gives name a counted state on a new guitar exercise of
// skill, with accuracy.
func (w *world) practisedItem(name, skill string, accuracy float64) string {
	key := w.putPracticeExercise(skill+"-item", skill, "guitar")
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	dueAt := fixedNow.AddDate(0, 0, 3)
	w.practiceStates.put(studentID, domain.PracticeItemState{
		ItemKey: key, RulesVersion: domain.PracticeRulesVersion, Level: domain.KnowledgeLevelLearning,
		Counted: 3, Box: 1, DueAt: &dueAt, LastAt: &fixedNow, Accuracy: accuracy,
	})
	return key
}

func (w *world) accuracyWasAndIs(name, skill, before, now string) error {
	var then, after float64
	if _, err := fmt.Sscan(before, &then); err != nil {
		return err
	}
	if _, err := fmt.Sscan(now, &after); err != nil {
		return err
	}
	key := w.practisedItem(name, skill, after)
	w.practiceActivity.addSnapshot(w.ensureRegistered(name, domain.RoleStudent).String(), w.snapshotDayBefore(),
		domain.PracticeItemSnapshot{ItemKey: key, Counted: 2, Accuracy: then})
	return nil
}

func (w *world) firstPractisedDaysAgo(name, skill string, daysAgo int, now string) error {
	var after float64
	if _, err := fmt.Sscan(now, &after); err != nil {
		return err
	}
	key := w.practisedItem(name, skill, after)
	day := w.localTime(daysAgo, 0, 0)
	w.practiceActivity.addSnapshot(w.ensureRegistered(name, domain.RoleStudent).String(), time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC),
		domain.PracticeItemSnapshot{ItemKey: key, Counted: 1, Accuracy: after})
	return nil
}

func (w *world) noCleanTakeThenTempoNow(name, slug string, bpm int) error {
	w.putPlayAlongOn(slug, "guitar", slug+"-skill", 120)
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	key := domain.PlayAlongItemKey(diagramID(slug).String())
	dueAt := fixedNow.AddDate(0, 0, 3)
	w.practiceStates.put(studentID, domain.PracticeItemState{
		ItemKey: key, RulesVersion: domain.PracticeRulesVersion, Level: domain.KnowledgeLevelLearning,
		Counted: 3, Box: 1, DueAt: &dueAt, LastAt: &fixedNow, Accuracy: 0.5, BestCleanBPM: &bpm,
	})
	w.practiceActivity.addSnapshot(studentID, w.snapshotDayBefore(), domain.PracticeItemSnapshot{ItemKey: key, Counted: 1, Accuracy: 0.5})
	return nil
}

func (w *world) conceptHasExercises(concept string) error {
	conceptID := w.conceptIDFor(concept)
	seconds := practiceExerciseSeconds
	w.exercises.put(domain.Exercise{
		ID: exerciseID(concept + "-exercise").String(), Title: concept, ExerciseType: domain.ExerciseTypeTextResponse,
		Concepts: []domain.KnowledgeNode{{ID: conceptID.String()}}, EstimatedDurationSeconds: &seconds, CreatedAt: fixedNow,
	})
	return nil
}

func (w *world) conceptAccuracyImproved(name, concept string) error {
	if err := w.conceptHasExercises(concept); err != nil {
		return err
	}
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	key := domain.ExerciseItemKey(exerciseID(concept + "-exercise").String())
	dueAt := fixedNow.AddDate(0, 0, 3)
	w.practiceStates.put(studentID, domain.PracticeItemState{
		ItemKey: key, RulesVersion: domain.PracticeRulesVersion, Level: domain.KnowledgeLevelLearning,
		Counted: 3, Box: 1, DueAt: &dueAt, LastAt: &fixedNow, Accuracy: 0.9,
	})
	w.practiceActivity.addSnapshot(studentID, w.snapshotDayBefore(), domain.PracticeItemSnapshot{ItemKey: key, Counted: 1, Accuracy: 0.5})
	return nil
}

// pathSkillToStart puts skill on name's path with an unseen guitar
// exercise.
func (w *world) pathSkillToStart(name, skill string) {
	w.addPathSkill(name, skill)
	w.putPracticeExercise(skill+"-start", skill, "guitar")
}

func (w *world) hasSkillsToStart(name string, count int) error {
	for i := range count {
		w.pathSkillToStart(name, fmt.Sprintf("summary-skill-%d", i))
	}
	return nil
}

func (w *world) skillRestsOnDueItems(name, skill string) error {
	key := w.practisedItem(name, skill, 0.8)
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	dueAt := fixedNow.Add(-time.Hour)
	w.practiceStates.put(studentID, domain.PracticeItemState{
		ItemKey: key, RulesVersion: domain.PracticeRulesVersion, Level: domain.KnowledgeLevelAccurate,
		Counted: 4, Box: 2, DueAt: &dueAt, LastAt: &fixedNow, Accuracy: 0.8,
	})
	return nil
}

func (w *world) hasSkillToStrengthenAndToStart(name string) error {
	w.addPathSkill(name, "skill-to-strengthen")
	w.practisedItem(name, "skill-to-strengthen", 0.6)
	w.pathSkillToStart(name, "skill-to-start")
	return nil
}

func (w *world) firstNextStepIsToStrengthen(name, skill string) error {
	w.addPathSkill(name, skill)
	w.practisedItem(name, skill, 0.6)
	return nil
}

func (w *world) hasNeverPractisedAnything(name string) error {
	w.pathSkillToStart(name, "first-skill")
	return nil
}

func (w *world) enrolledInPathForEveryInstrument(name string) error {
	w.enrollInTemplate(name, "music-theory")
	return nil
}

func (w *world) onlyEnrolledInPathForEveryInstrument(name string) error {
	w.authenticateAs(name, domain.RoleStudent)
	w.enrollInTemplate(name, "music-theory")
	return nil
}

func (w *world) skillHasItemsOnlyFor(skill, instrument string) error {
	w.putPracticeExercise(skill+"-only", skill, instrument)
	return nil
}

func (w *world) isReadyToStartPathSkill(name, skill string) error {
	w.addPathSkill(name, skill)
	return nil
}

// ── Whens ────────────────────────────────────────────────────────────────

func (w *world) readsSummary(name, instrument string) error {
	return w.readsSummaryInTimeZone(name, instrument, w.timeZone)
}

func (w *world) readsSummaryInTimeZone(_, instrument, timeZone string) error {
	id := openapi_types.UUID(instrumentID(instrument))
	return w.readSummary(&id, timeZone)
}

func (w *world) readsSummaryForMissingInstrument(string) error {
	id := openapi_types.UUID(uuid.New())
	return w.readSummary(&id, w.timeZone)
}

func (w *world) readSummary(instrumentID *openapi_types.UUID, timeZone string) error {
	resp, err := w.handler.GetPracticeSummary(w.ctx(), generated.GetPracticeSummaryRequestObject{
		Params: generated.GetPracticeSummaryParams{InstrumentId: instrumentID, TimeZone: &timeZone},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) readsOverview(name string) error {
	return w.readsOverviewInTimeZone(name, w.timeZone)
}

func (w *world) readsOverviewInTimeZone(_, timeZone string) error {
	resp, err := w.handler.GetPracticeOverview(w.ctx(), generated.GetPracticeOverviewRequestObject{
		Params: generated.GetPracticeOverviewParams{TimeZone: &timeZone},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

// ── Thens ────────────────────────────────────────────────────────────────

func (w *world) summary() (generated.PracticeSummary, error) {
	resp, ok := w.lastResp.(generated.GetPracticeSummary200JSONResponse)
	if !ok {
		return generated.PracticeSummary{}, fmt.Errorf("expected a practice summary, got %T (err %v)", w.lastResp, w.lastErr)
	}
	return generated.PracticeSummary(resp), nil
}

func (w *world) overview() (generated.PracticeOverview, error) {
	resp, ok := w.lastResp.(generated.GetPracticeOverview200JSONResponse)
	if !ok {
		return generated.PracticeOverview{}, fmt.Errorf("expected a practice overview, got %T (err %v)", w.lastResp, w.lastErr)
	}
	return generated.PracticeOverview(resp), nil
}

func (w *world) summaryShowsPracticeDays(days int) error {
	s, err := w.summary()
	if err != nil {
		return err
	}
	if s.PracticeDaysLast7 != days {
		return fmt.Errorf("expected %d practice days, got %d", days, s.PracticeDaysLast7)
	}
	return nil
}

func (w *world) overviewShowsPracticeDays(days int) error {
	o, err := w.overview()
	if err != nil {
		return err
	}
	if o.PracticeDaysLast7 != days {
		return fmt.Errorf("expected %d practice days, got %d", days, o.PracticeDaysLast7)
	}
	return nil
}

func (w *world) overviewShowsLearningDays(days int) error {
	o, err := w.overview()
	if err != nil {
		return err
	}
	if o.LearningDaysLast7 != days {
		return fmt.Errorf("expected %d learning days, got %d", days, o.LearningDaysLast7)
	}
	return nil
}

func (w *world) progressLines(nodeID uuid.UUID) ([]generated.SkillProgress, error) {
	s, err := w.summary()
	if err != nil {
		return nil, err
	}
	var lines []generated.SkillProgress
	for _, line := range s.ProgressThisWeek {
		if line.NodeId == nodeID {
			lines = append(lines, line)
		}
	}
	return lines, nil
}

func (w *world) progressShowsAccuracy(skill, before, after string) error {
	var then, now float32
	if _, err := fmt.Sscan(before, &then); err != nil {
		return err
	}
	if _, err := fmt.Sscan(after, &now); err != nil {
		return err
	}
	lines, err := w.progressLines(w.skillIDFor(skill))
	if err != nil {
		return err
	}
	for _, line := range lines {
		if line.Measure == generated.Accuracy && approxEqual(line.Before, then) && approxEqual(line.After, now) {
			return nil
		}
	}
	return fmt.Errorf("expected %q accuracy from %s to %s, got %+v", skill, before, after, lines)
}

func approxEqual(a, b float32) bool {
	d := a - b
	return d < 1e-4 && d > -1e-4
}

func (w *world) progressHasNoTempoLineFor(slug string) error {
	lines, err := w.progressLines(w.skillIDFor(slug + "-skill"))
	if err != nil {
		return err
	}
	for _, line := range lines {
		if line.Measure == generated.BestCleanTempoBpm {
			return fmt.Errorf("expected no tempo line, got %+v", line)
		}
	}
	return nil
}

func (w *world) progressHasNoLineFor(node string) error {
	lines, err := w.progressLines(w.nodeIDByName(node))
	if err != nil {
		return err
	}
	if len(lines) > 0 {
		return fmt.Errorf("expected no progress line for %q, got %+v", node, lines)
	}
	return nil
}

func (w *world) progressIsEmpty() error {
	s, err := w.summary()
	if err != nil {
		return err
	}
	if len(s.ProgressThisWeek) > 0 {
		return fmt.Errorf("expected no progress, got %+v", s.ProgressThisWeek)
	}
	return nil
}

func (w *world) summaryShowsNextSteps(shown, total int) error {
	s, err := w.summary()
	if err != nil {
		return err
	}
	if len(s.NextSteps) != shown || s.NextStepsTotal != total {
		return fmt.Errorf("expected %d next steps of %d, got %d of %d", shown, total, len(s.NextSteps), s.NextStepsTotal)
	}
	return nil
}

func (w *world) summaryListsInstruments(first, second string) error {
	s, err := w.summary()
	if err != nil {
		return err
	}
	want := []openapi_types.UUID{instrumentID(first), instrumentID(second)}
	got := slices.Clone(s.StudentInstrumentIds)
	if len(got) != len(want) || !slices.Contains(got, want[0]) || !slices.Contains(got, want[1]) {
		return fmt.Errorf("expected the instruments %s and %s, got %v", first, second, got)
	}
	return nil
}

func (w *world) nextStepsAreRefreshStrengthenStart(skill string) error {
	s, err := w.summary()
	if err != nil {
		return err
	}
	want := []struct {
		kind generated.PracticeNextStepKind
		node uuid.UUID
	}{
		{generated.Refresh, w.skillIDFor(skill)},
		{generated.Strengthen, w.skillIDFor("skill-to-strengthen")},
		{generated.ReadyToStart, w.skillIDFor("skill-to-start")},
	}
	if len(s.NextSteps) != len(want) {
		return fmt.Errorf("expected %d next steps, got %+v", len(want), s.NextSteps)
	}
	for i, step := range s.NextSteps {
		if step.Kind != want[i].kind || step.NodeId != want[i].node {
			return fmt.Errorf("next step %d: expected %s on %s, got %+v", i+1, want[i].kind, want[i].node, step)
		}
	}
	return nil
}

// allNextSteps reads every next step: the summary's top three are enough
// for the scenarios that look for one, which never set up more.
func (w *world) allNextSteps() ([]generated.PracticeNextStep, error) {
	s, err := w.summary()
	if err != nil {
		return nil, err
	}
	if s.NextStepsTotal > len(s.NextSteps) {
		return nil, fmt.Errorf("the scenario has %d next steps, more than the summary shows", s.NextStepsTotal)
	}
	return s.NextSteps, nil
}

func (w *world) notAmongNextSteps(skill string) error {
	steps, err := w.allNextSteps()
	if err != nil {
		return err
	}
	for _, step := range steps {
		if step.NodeId == w.skillIDFor(skill) {
			return fmt.Errorf("expected %q not among the next steps, got %+v", skill, step)
		}
	}
	return nil
}

func (w *world) nextStepsIncludeReadyToStart(skill string) error {
	steps, err := w.allNextSteps()
	if err != nil {
		return err
	}
	for _, step := range steps {
		if step.NodeId == w.skillIDFor(skill) && step.Kind == generated.ReadyToStart {
			return nil
		}
	}
	return fmt.Errorf("expected %q ready to start among the next steps, got %+v", skill, steps)
}

func (w *world) nextStepsStartWithReadyToStart() error {
	s, err := w.summary()
	if err != nil {
		return err
	}
	if len(s.NextSteps) == 0 || s.NextSteps[0].Kind != generated.ReadyToStart {
		return fmt.Errorf("expected the next steps to start with a skill ready to start, got %+v", s.NextSteps)
	}
	return nil
}

func (w *world) isInAnyInstrumentGroup(node string) error {
	s, err := w.summary()
	if err != nil {
		return err
	}
	id := w.nodeIDByName(node)
	for _, g := range s.Groups {
		for _, n := range g.Nodes {
			if n.NodeId != id {
				continue
			}
			if !g.AnyInstrument {
				return fmt.Errorf("expected %q in the Any instrument group, found it in the group of area %v", node, g.AreaNodeId)
			}
			return nil
		}
	}
	return fmt.Errorf("expected %q in the Any instrument group, found it in no group", node)
}

// countsOnMonday re-reads, with the summary's clock moved, how many days
// of the last 7 read counts: once with the window starting the Tuesday
// after Monday, then with it ending on Monday. Counted on Monday, the
// activity is outside the first window and inside the second.
func (w *world) countsOnMonday(read func() error, days func() (int, error)) error {
	defer func() { w.summaryNow = fixedNow }()
	loc := w.location()
	for _, check := range []struct {
		now  time.Time
		want int
	}{
		{now: time.Date(monday.Year(), monday.Month(), monday.Day()+7, 12, 0, 0, 0, loc), want: 0},
		{now: time.Date(monday.Year(), monday.Month(), monday.Day(), 23, 45, 0, 0, loc), want: 1},
	} {
		w.summaryNow = check.now
		if err := read(); err != nil {
			return err
		}
		got, err := days()
		if err != nil {
			return err
		}
		if got != check.want {
			return fmt.Errorf("at %s, expected %d days, got %d: the activity didn't count on Monday", check.now, check.want, got)
		}
	}
	return nil
}

func (w *world) practiceCountsOnMonday() error {
	return w.countsOnMonday(
		func() error { return w.readsSummary("", "guitar") },
		func() (int, error) {
			s, err := w.summary()
			return s.PracticeDaysLast7, err
		},
	)
}

func (w *world) completionCountsOnMonday() error {
	return w.countsOnMonday(
		func() error { return w.readsOverview("") },
		func() (int, error) {
			o, err := w.overview()
			return o.LearningDaysLast7, err
		},
	)
}

func (w *world) card(instrument string) (generated.PracticeInstrumentCard, error) {
	o, err := w.overview()
	if err != nil {
		return generated.PracticeInstrumentCard{}, err
	}
	for _, c := range o.Instruments {
		if c.InstrumentId == instrumentID(instrument) {
			return c, nil
		}
	}
	return generated.PracticeInstrumentCard{}, fmt.Errorf("no %q card in %+v", instrument, o.Instruments)
}

func (w *world) cardShowsDays(instrument string, days int) error {
	c, err := w.card(instrument)
	if err != nil {
		return err
	}
	if c.PracticeDaysLast7 != days {
		return fmt.Errorf("expected the %q card to show %d practice days, got %d", instrument, days, c.PracticeDaysLast7)
	}
	return nil
}

func (w *world) cardShowsDaysAndStrengthen(instrument string, days int, skill string) error {
	if err := w.cardShowsDays(instrument, days); err != nil {
		return err
	}
	c, err := w.card(instrument)
	if err != nil {
		return err
	}
	if c.TopNextStep == nil || c.TopNextStep.Kind != generated.Strengthen || c.TopNextStep.NodeId != w.skillIDFor(skill) {
		return fmt.Errorf("expected the %q card's next step to strengthen %q, got %+v", instrument, skill, c.TopNextStep)
	}
	return nil
}

func (w *world) cardHasNoNextStep(instrument string) error {
	c, err := w.card(instrument)
	if err != nil {
		return err
	}
	if c.TopNextStep != nil {
		return fmt.Errorf("expected the %q card to have no next step, got %+v", instrument, c.TopNextStep)
	}
	return nil
}

func (w *world) overviewHasNoCards() error {
	o, err := w.overview()
	if err != nil {
		return err
	}
	if len(o.Instruments) > 0 {
		return fmt.Errorf("expected no instrument cards, got %+v", o.Instruments)
	}
	return nil
}
