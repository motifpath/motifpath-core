//go:build integration

package bdd

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/cucumber/godog"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// Steps for observing the alpha. State is seeded straight into the fakes,
// as the worker and earlier requests would have left it; the reads go
// through the handler.

var saoPaulo = func() *time.Location {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		panic(err)
	}
	return loc
}()

// fakeAlphaActivity is an in-memory ports.AlphaActivityReader.
type fakeAlphaActivity struct {
	sessions    []domain.PracticeSessionRecord
	completions map[string]map[string]time.Time
	starts      map[string]map[string]time.Time
	lastActive  map[string]time.Time
}

func (f *fakeAlphaActivity) SessionsStartedBetween(_ context.Context, studentIDs []string, from, to time.Time) ([]domain.PracticeSessionRecord, error) {
	var out []domain.PracticeSessionRecord
	for _, s := range f.sessions {
		if slices.Contains(studentIDs, s.StudentID) && !s.StartedAt.Before(from) && s.StartedAt.Before(to) {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeAlphaActivity) FirstCompletions(_ context.Context, studentID string, ids []string) (map[string]time.Time, error) {
	return pickTimes(f.completions[studentID], ids), nil
}

func (f *fakeAlphaActivity) FirstStarts(_ context.Context, studentID string, ids []string) (map[string]time.Time, error) {
	return pickTimes(f.starts[studentID], ids), nil
}

func (f *fakeAlphaActivity) LastActive(_ context.Context, studentIDs []string) (map[string]time.Time, error) {
	return pickTimes(f.lastActive, studentIDs), nil
}

func pickTimes(from map[string]time.Time, keys []string) map[string]time.Time {
	out := map[string]time.Time{}
	for _, k := range keys {
		if t, ok := from[k]; ok {
			out[k] = t
		}
	}
	return out
}

// alphaWorld is what the alpha observation scenarios share.
type alphaWorld struct {
	now      time.Time
	activity *fakeAlphaActivity
	// cohort lists the students "the alpha has", in the order "the first
	// of them" counts.
	cohort      []string
	observation *generated.AlphaObservation
	student     *generated.AlphaStudentObservation
}

func newAlphaWorld() *alphaWorld {
	return &alphaWorld{
		now: time.Date(2026, 10, 14, 10, 0, 0, 0, saoPaulo),
		activity: &fakeAlphaActivity{
			completions: map[string]map[string]time.Time{},
			starts:      map[string]map[string]time.Time{},
			lastActive:  map[string]time.Time{},
		},
	}
}

func registerAlphaObservationSteps(sc *godog.ScenarioContext, w *world) {
	day := `(\w+ \d+ \w+)`        // Monday 12 Oct
	dated := `(\w+ \d+ \w+ \d{4})` // Monday 12 Oct 2026
	clock := `(\d\d:\d\d)`

	// ── Givens ───────────────────────────────────────────────────────────
	sc.Step(`^"([^"]+)" is signed in as an admin$`, func(name string) error { w.authenticateAs(name, domain.RoleAdmin); return nil })
	sc.Step(`^"([^"]+)" is signed in as a teacher$`, func(name string) error { w.authenticateAs(name, domain.RoleTeacher); return nil })
	sc.Step(`^now is `+dated+` at `+clock+`, São Paulo time$`, w.alphaNowIs)
	sc.Step(`^"([^"]+)" is a student who was given the path "([^"]+)", (\d+) nodes, on `+dated+`$`, w.studentGivenPath)
	sc.Step(`^"([^"]+)" is a student who has never been given a path$`, func(name string) error { w.ensureRegistered(name, domain.RoleStudent); return nil })
	sc.Step(`^"([^"]+)" is a teacher who was given the path "([^"]+)"$`, w.teacherGivenPath)
	sc.Step(`^"([^"]+)" is a student who was given a path on `+dated+`$`, w.studentGivenAPathOn)
	sc.Step(`^"([^"]+)" is a student who was given a path on `+dated+` and finished 3 practice sessions since$`, w.studentGivenAPathAndFinishedThree)
	sc.Step(`^"([^"]+)" is a student who was given a path on `+dated+` and has no activity$`, w.studentGivenAPathOn)
	sc.Step(`^"([^"]+)" was given the path "([^"]+)" on `+dated+`, now "[^"]+"'s current path$`, w.givenALaterPath)
	sc.Step(`^"([^"]+)" was given a path of (\d+) nodes and has completed (\d+) of them$`, w.givenAPathAndCompleted)

	sc.Step(`^"([^"]+)" finished practice sessions on `+day+` at `+clock+` and on `+day+` at `+clock+`$`, w.finishedTwoAt)
	sc.Step(`^"([^"]+)" finished a practice session on `+day+` at `+clock+`$`, w.finishedOneAt)
	sc.Step(`^"([^"]+)" ended a practice session early on `+day+` at `+clock+`$`, w.endedEarlyAt)
	sc.Step(`^"([^"]+)" abandoned a practice session on `+day+`$`, w.abandonedOn)
	sc.Step(`^"([^"]+)" has a practice session in progress since `+day+` at `+clock+`$`, w.inProgressSince)
	sc.Step(`^"([^"]+)" finished practice sessions on `+day+`, `+day+` and `+day+`$`, w.finishedThreeOn)
	sc.Step(`^"([^"]+)" finished a practice session that started on `+day+` at `+clock+`, São Paulo time, and ended on `+day+` at `+clock+`$`, w.finishedAcrossMidnight)
	sc.Step(`^"([^"]+)" finished (\d+), (\d+) and (\d+) practice sessions in the weeks of (\d+ \w+), (\d+ \w+) and (\d+ \w+) 2026$`, w.finishedInWeeks)
	sc.Step(`^"([^"]+)" finished a practice session on `+day+` at `+clock+` and ended one early on `+day+`$`, w.finishedAndEndedEarly)
	sc.Step(`^"([^"]+)" and "([^"]+)" each have 1 counted session in the week of 12 Oct 2026 and "([^"]+)" has 0$`, w.oneOneZero)

	sc.Step(`^the (\d+)(?:st|nd|rd|th) node of "([^"]+)" is locked for "([^"]+)"$`, w.nodeLockedFor)
	sc.Step(`^"([^"]+)" has completed (\d+) of the (\d+) nodes of "([^"]+)"$`, w.completedOfNodes)
	sc.Step(`^"([^"]+)" has completed the first (\d+) nodes of "([^"]+)" and started the (\d+)(?:st|nd|rd|th)$`, w.completedFirstAndStarted)

	sc.Step(`^the alpha has (\d+) students$`, w.alphaHasStudents)
	sc.Step(`^(\d+) of them has 3 or more counted sessions in the week of (\d+ \w+ \d{4}), and (\d+) had in the week of (\d+ \w+)$`, w.cohortAtTargetInWeeks)
	sc.Step(`^(\d+) of them have been at the target every week so far$`, w.cohortAtTargetEveryWeek)
	sc.Step(`^their first paths are at (\d+)%, (\d+)%, (\d+)% and (\d+)%$`, w.cohortFirstPaths)

	sc.Step(`^"([^"]+)"'s last activity was an? [a-z_]+\.[a-z_]+ on `+day+` at `+clock+`$`, w.lastActivityWas)

	// ── Whens ────────────────────────────────────────────────────────────
	sc.Step(`^"([^"]+)" observes the alpha for the week of (\d+ \w+ \d{4})$`, w.observesAlphaForWeek)
	sc.Step(`^"([^"]+)" observes the alpha without choosing a week$`, func(name string) error { return w.observeAlpha(name, nil) })
	sc.Step(`^"([^"]+)" observes "([^"]+)" for the week of (\d+ \w+ \d{4})$`, w.observesStudentForWeek)

	// ── Thens ────────────────────────────────────────────────────────────
	sc.Step(`^"([^"]+)" is (not )?one of its students$`, w.isOneOfItsStudents)
	sc.Step(`^"([^"]+)" has (\d+) counted sessions? in that week$`, w.hasCountedSessions)
	sc.Step(`^"([^"]+)" is (not )?at the target in that week$`, w.isAtTargetInWeek)
	sc.Step(`^the week of (\d+ \w+ \d{4}) is reported as still running$`, w.weekIsRunning)
	sc.Step(`^the alpha is observed for the week of (\d+ \w+ \d{4})$`, w.observedForWeek)
	sc.Step(`^"([^"]+)"'s weeks so far are (\d+ \w+) with (\d+) sessions, (\d+ \w+) with (\d+) and (\d+ \w+) with (\d+)$`, w.weeksSoFarAre)
	sc.Step(`^"([^"]+)" is at the target in (\d+) of (\d+) weeks so far$`, w.atTargetInWeeksSoFar)
	sc.Step(`^"([^"]+)" is not counted as at the target every week so far$`, w.notAtTargetEveryWeek)
	sc.Step(`^"([^"]+)" has no weeks so far$`, w.hasNoWeeksSoFar)
	sc.Step(`^"([^"]+)"'s weeks so far list the (\d+) weeks from (\d+ \w+) to (\d+ \w+) 2026$`, w.weeksSoFarList)
	sc.Step(`^"([^"]+)"'s count of weeks at the target covers all (\d+) weeks so far$`, w.weeksSoFarCount)
	sc.Step(`^the (\d+)(?:st|nd|rd|th) node is listed as not started$`, w.nodeListedNotStarted)
	sc.Step(`^"([^"]+)"'s first path is "([^"]+)" at (\d+) of (\d+) nodes, (\d+)%$`, w.firstPathAt)
	sc.Step(`^"([^"]+)"'s first path is (not )?over the target$`, w.firstPathOverTarget)
	sc.Step(`^"([^"]+)"'s first path is at (\d+)%$`, w.firstPathPercent)
	sc.Step(`^"([^"]+)"'s first path is "([^"]+)"$`, w.firstPathTitled)
	sc.Step(`^(\d+) of (\d+) students is at the target this week and (\d+) of (\d+) were the week before$`, w.cohortAtTargetThisAndLastWeek)
	sc.Step(`^(\d+) of (\d+) students have been at the target every week so far$`, w.cohortEveryWeekIs)
	sc.Step(`^(\d+) of (\d+) students have a first path over the target$`, w.cohortOverTargetIs)
	sc.Step(`^the median first path is (\d+)%$`, w.medianIs)
	sc.Step(`^the students are listed as "([^"]+)", "([^"]+)", "([^"]+)"$`, w.studentsListedAs)
	sc.Step(`^"([^"]+)"'s last activity is `+day+` at `+clock+`$`, w.lastActivityIs)
	sc.Step(`^"([^"]+)" is (not )?quiet$`, w.isQuiet)
	sc.Step(`^"([^"]+)" has no last activity$`, w.hasNoLastActivity)
	sc.Step(`^the sessions of the weeks of (\d+ \w+) and (\d+ \w+) 2026 are listed, newest first$`, w.sessionWeeksListed)
	sc.Step(`^each session shows its start, planned minutes, end, state and items answered$`, w.eachSessionShowsItsFacts)
	sc.Step(`^the (\d+) nodes of "([^"]+)" are listed in path order$`, w.nodesListedInOrder)
	sc.Step(`^nodes 1 to (\d+) are completed with their completion dates, node (\d+) is in progress and nodes (\d+) to (\d+) are not started$`, w.nodesByStatus)
	sc.Step(`^"([^"]+)" is refused for lacking permission$`, w.alphaRefused)
	sc.Step(`^no student is found$`, w.noStudentFound)
}

// ── Time ─────────────────────────────────────────────────────────────────

// alphaDay is "Monday 12 Oct" or "Monday 12 Oct 2026" at hhmm, São Paulo
// time; the year is 2026 when not given.
func alphaDay(day, hhmm string) (time.Time, error) {
	if len(day) < 5 || day[len(day)-5] != ' ' {
		day += " 2026"
	}
	t, err := time.ParseInLocation("Monday 2 Jan 2006 15:04", day+" "+hhmm, saoPaulo)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse %q: %w", day, err)
	}
	return t, nil
}

// weekMonday is the Monday "12 Oct 2026" or "12 Oct" names, as a date.
func weekMonday(s string) (time.Time, error) {
	if len(s) < 5 || s[len(s)-5] != ' ' {
		s += " 2026"
	}
	return time.Parse("2 Jan 2006", s)
}

func (w *world) alphaNowIs(day, hhmm string) error {
	now, err := alphaDay(day, hhmm)
	w.alpha.now = now
	return err
}

// ── Seeding ──────────────────────────────────────────────────────────────

func (w *world) studentID(name string) string {
	return w.ensureRegistered(name, domain.RoleStudent).String()
}

// givePath gives name a path titled title of nodes nodes at givenAt.
func (w *world) givePath(userID, title string, nodes int, givenAt time.Time) domain.StudentPath {
	items := make([]domain.StudentPathItemRecord, nodes)
	for i := range items {
		nodeID := deterministicUUID("alpha-node", title, strconv.Itoa(i+1)).String()
		w.nodes.put(domain.ContentNode{ID: nodeID, Title: fmt.Sprintf("%s %d", title, i+1), ContentType: domain.ContentTypeArticle})
		items[i] = domain.StudentPathItemRecord{Position: i + 1, ContentNodeID: nodeID}
	}
	path := domain.StudentPath{
		ID: deterministicUUID("alpha-path", userID, title).String(), StudentID: userID, Title: title,
		AssignedBy: userID, AssignedAt: givenAt, Items: items,
	}
	w.studentPaths.put(path)
	return path
}

func (w *world) studentGivenPath(name, title string, nodes int, day string) error {
	at, err := alphaDay(day, "09:00")
	if err != nil {
		return err
	}
	w.givePath(w.studentID(name), title, nodes, at)
	return nil
}

func (w *world) teacherGivenPath(name, title string) error {
	w.givePath(w.ensureRegistered(name, domain.RoleTeacher).String(), title, 6, time.Date(2026, 9, 21, 9, 0, 0, 0, saoPaulo))
	return nil
}

func (w *world) studentGivenAPathOn(name, day string) error {
	return w.studentGivenPath(name, "First path of "+name, 10, day)
}

func (w *world) studentGivenAPathAndFinishedThree(name, day string) error {
	if err := w.studentGivenAPathOn(name, day); err != nil {
		return err
	}
	monday, err := alphaDay(day, "09:00")
	if err != nil {
		return err
	}
	w.threeSessionsInWeekOf(w.studentID(name), monday)
	return nil
}

func (w *world) givenALaterPath(name, title, day string) error {
	at, err := alphaDay(day, "09:00")
	if err != nil {
		return err
	}
	id := w.studentID(name)
	first, err := w.firstPathOf(id)
	if err != nil {
		return err
	}
	first.ArchivedAt = &at
	w.studentPaths.put(first)
	w.givePath(id, title, 6, at)
	return nil
}

func (w *world) givenAPathAndCompleted(name string, nodes, completed int) error {
	id := w.studentID(name)
	path := w.givePath(id, "First path of "+name, nodes, time.Date(2026, 9, 21, 9, 0, 0, 0, saoPaulo))
	w.completeFirst(id, path, completed)
	return nil
}

func (w *world) firstPathOf(studentID string) (domain.StudentPath, error) {
	firsts, err := w.studentPaths.FirstGivenByStudentIDs(context.Background(), []string{studentID})
	if err != nil {
		return domain.StudentPath{}, err
	}
	path, ok := firsts[studentID]
	if !ok {
		return domain.StudentPath{}, fmt.Errorf("student %s holds no path", studentID)
	}
	return path, nil
}

func (w *world) pathTitled(studentID, title string) (domain.StudentPath, error) {
	path, ok := w.studentPaths.byID[deterministicUUID("alpha-path", studentID, title).String()]
	if !ok {
		return domain.StudentPath{}, fmt.Errorf("student %s holds no path %q", studentID, title)
	}
	return path, nil
}

func (w *world) setStatus(studentID, nodeID string, status domain.CompletionStatus) {
	w.completion.mu.Lock()
	defer w.completion.mu.Unlock()
	if w.completion.statuses[studentID] == nil {
		w.completion.statuses[studentID] = map[string]domain.CompletionStatus{}
	}
	w.completion.statuses[studentID][nodeID] = status
}

func (w *world) completeFirst(studentID string, path domain.StudentPath, n int) {
	for _, item := range path.Items[:n] {
		w.setStatus(studentID, item.ContentNodeID, domain.CompletionStatusCompleted)
	}
}

func (w *world) addSession(studentID string, start time.Time, end *time.Time, leftEarly bool, lastEvent time.Time) {
	record := domain.PracticeSessionRecord{
		ID:        deterministicUUID("alpha-session", studentID, start.Format(time.RFC3339)).String(),
		StudentID: studentID, StartedAt: start, PlannedMinutes: 10, LastEventAt: lastEvent, AnswersArrived: 4,
	}
	if end != nil {
		record.End = &domain.PracticeSessionRecordEnd{At: *end, LeftEarly: leftEarly, AnsweredCount: 6}
	}
	w.alpha.activity.sessions = append(w.alpha.activity.sessions, record)
}

func (w *world) finishedAt(studentID string, start time.Time) {
	end := start.Add(10 * time.Minute)
	w.addSession(studentID, start, &end, false, end)
}

// threeSessionsInWeekOf finishes sessions on the Monday, Tuesday and
// Wednesday morning of the week starting at monday.
func (w *world) threeSessionsInWeekOf(studentID string, monday time.Time) {
	base := time.Date(monday.Year(), monday.Month(), monday.Day(), 7, 0, 0, 0, saoPaulo)
	for i := range 3 {
		w.finishedAt(studentID, base.AddDate(0, 0, i))
	}
}

func (w *world) finishedOneAt(name, day, hhmm string) error {
	start, err := alphaDay(day, hhmm)
	if err != nil {
		return err
	}
	w.finishedAt(w.studentID(name), start)
	return nil
}

func (w *world) finishedTwoAt(name, day1, hhmm1, day2, hhmm2 string) error {
	if err := w.finishedOneAt(name, day1, hhmm1); err != nil {
		return err
	}
	return w.finishedOneAt(name, day2, hhmm2)
}

func (w *world) finishedThreeOn(name, day1, day2, day3 string) error {
	for _, d := range []string{day1, day2, day3} {
		if err := w.finishedOneAt(name, d, "09:00"); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) endedEarlyAt(name, day, hhmm string) error {
	start, err := alphaDay(day, hhmm)
	if err != nil {
		return err
	}
	end := start.Add(4 * time.Minute)
	w.addSession(w.studentID(name), start, &end, true, end)
	return nil
}

func (w *world) abandonedOn(name, day string) error {
	start, err := alphaDay(day, "09:00")
	if err != nil {
		return err
	}
	w.addSession(w.studentID(name), start, nil, false, start.Add(3*time.Minute))
	return nil
}

func (w *world) inProgressSince(name, day, hhmm string) error {
	start, err := alphaDay(day, hhmm)
	if err != nil {
		return err
	}
	w.addSession(w.studentID(name), start, nil, false, start)
	return nil
}

func (w *world) finishedAcrossMidnight(name, startDay, startAt, endDay, endAt string) error {
	start, err := alphaDay(startDay, startAt)
	if err != nil {
		return err
	}
	end, err := alphaDay(endDay, endAt)
	if err != nil {
		return err
	}
	w.addSession(w.studentID(name), start, &end, false, end)
	return nil
}

func (w *world) finishedInWeeks(name string, n1, n2, n3 int, week1, week2, week3 string) error {
	id := w.studentID(name)
	for i, week := range []string{week1, week2, week3} {
		monday, err := weekMonday(week)
		if err != nil {
			return err
		}
		base := time.Date(monday.Year(), monday.Month(), monday.Day(), 9, 0, 0, 0, saoPaulo)
		for j := range []int{n1, n2, n3}[i] {
			w.finishedAt(id, base.AddDate(0, 0, j))
		}
	}
	return nil
}

func (w *world) finishedAndEndedEarly(name, day, hhmm, earlyDay string) error {
	if err := w.finishedOneAt(name, day, hhmm); err != nil {
		return err
	}
	return w.endedEarlyAt(name, earlyDay, "09:00")
}

func (w *world) oneOneZero(a, b, c string) error {
	given := time.Date(2026, 9, 21, 9, 0, 0, 0, saoPaulo)
	monday := time.Date(2026, 10, 12, 18, 30, 0, 0, saoPaulo)
	for _, name := range []string{a, b, c} {
		id := w.studentID(name)
		if _, err := w.firstPathOf(id); err != nil {
			w.givePath(id, "First path of "+name, 10, given)
		}
		if name != c {
			w.finishedAt(id, monday)
		}
	}
	return nil
}

func (w *world) nodeLockedFor(position int, title, name string) error {
	id := w.studentID(name)
	path, err := w.pathTitled(id, title)
	if err != nil {
		return err
	}
	w.setStatus(id, path.Items[position-1].ContentNodeID, domain.CompletionStatusLocked)
	return nil
}

func (w *world) completedOfNodes(name string, completed, _ int, title string) error {
	id := w.studentID(name)
	path, err := w.pathTitled(id, title)
	if err != nil {
		return err
	}
	w.completeFirst(id, path, completed)
	return nil
}

func (w *world) completedFirstAndStarted(name string, completed int, title string, started int) error {
	id := w.studentID(name)
	path, err := w.pathTitled(id, title)
	if err != nil {
		return err
	}
	w.completeFirst(id, path, completed)
	activity := w.alpha.activity
	activity.completions[id], activity.starts[id] = map[string]time.Time{}, map[string]time.Time{}
	for i, item := range path.Items[:completed] {
		day := time.Date(2026, 9, 22+i, 19, 0, 0, 0, saoPaulo)
		activity.starts[id][item.ContentNodeID] = day
		activity.completions[id][item.ContentNodeID] = day.Add(time.Hour)
	}
	node := path.Items[started-1].ContentNodeID
	w.setStatus(id, node, domain.CompletionStatusInProgress)
	activity.starts[id][node] = time.Date(2026, 10, 13, 19, 0, 0, 0, saoPaulo)
	return nil
}

// alphaHasStudents makes n students of the alpha: "alice" from the
// background, then others given a path on Monday 28 Sep 2026.
func (w *world) alphaHasStudents(n int) error {
	w.alpha.cohort = []string{"alice"}
	for i := 2; i <= n; i++ {
		name := fmt.Sprintf("student-%d", i)
		w.givePath(w.studentID(name), "First path of "+name, 10, time.Date(2026, 9, 28, 9, 0, 0, 0, saoPaulo))
		w.alpha.cohort = append(w.alpha.cohort, name)
	}
	return nil
}

func (w *world) cohortAtTargetInWeeks(n int, week string, m int, previous string) error {
	for _, plan := range []struct {
		count int
		week  string
	}{{n, week}, {m, previous}} {
		monday, err := weekMonday(plan.week)
		if err != nil {
			return err
		}
		for _, name := range w.alpha.cohort[:plan.count] {
			w.threeSessionsInWeekOf(w.studentID(name), monday)
		}
	}
	return nil
}

// cohortAtTargetEveryWeek tops the first n students up to 3 finished
// sessions in each of their weeks so far.
func (w *world) cohortAtTargetEveryWeek(n int) error {
	running := domain.AlphaWeekOf(w.alpha.now)
	for _, name := range w.alpha.cohort[:n] {
		id := w.studentID(name)
		path, err := w.firstPathOf(id)
		if err != nil {
			return err
		}
		for week := domain.AlphaWeekOf(path.AssignedAt); week.Start().Before(running.Start()); week = week.Next() {
			if w.finishedIn(id, week) < domain.AlphaTargetSessions {
				w.threeSessionsInWeekOf(id, week.Start())
			}
		}
	}
	return nil
}

func (w *world) finishedIn(studentID string, week domain.AlphaWeek) int {
	n := 0
	for _, s := range w.alpha.activity.sessions {
		if s.StudentID == studentID && s.Finished() && week.Contains(s.StartedAt) {
			n++
		}
	}
	return n
}

// cohortFirstPaths makes each cohort student's first path 100 nodes long,
// with the given share completed.
func (w *world) cohortFirstPaths(p1, p2, p3, p4 int) error {
	for i, percent := range []int{p1, p2, p3, p4} {
		id := w.studentID(w.alpha.cohort[i])
		first, err := w.firstPathOf(id)
		if err != nil {
			return err
		}
		resized := w.givePath(id, "Hundred-node path of "+w.alpha.cohort[i], 100, first.AssignedAt)
		delete(w.studentPaths.byID, resized.ID)
		first.Items = resized.Items
		w.studentPaths.put(first)
		w.completeFirst(id, first, percent)
	}
	return nil
}

func (w *world) lastActivityWas(name, day, hhmm string) error {
	at, err := alphaDay(day, hhmm)
	if err != nil {
		return err
	}
	w.alpha.activity.lastActive[w.studentID(name)] = at
	return nil
}

// ── Reads ────────────────────────────────────────────────────────────────

func (w *world) observeAlpha(name string, week *time.Time) error {
	params := generated.GetAlphaObservationParams{}
	if week != nil {
		params.Week = &openapi_types.Date{Time: *week}
	}
	resp, err := w.handler.GetAlphaObservation(w.identityCtx(name), generated.GetAlphaObservationRequestObject{Params: params})
	w.lastResp, w.lastErr = resp, err
	if ok, isOK := resp.(generated.GetAlphaObservation200JSONResponse); isOK {
		observation := generated.AlphaObservation(ok)
		w.alpha.observation = &observation
	}
	return err
}

func (w *world) observesAlphaForWeek(name, week string) error {
	monday, err := weekMonday(week)
	if err != nil {
		return err
	}
	return w.observeAlpha(name, &monday)
}

func (w *world) observesStudentForWeek(name, student, week string) error {
	monday, err := weekMonday(week)
	if err != nil {
		return err
	}
	id, ok := w.userMotifID[student]
	if !ok {
		return fmt.Errorf("%s was never registered", student)
	}
	resp, err := w.handler.GetAlphaStudentObservation(w.identityCtx(name), generated.GetAlphaStudentObservationRequestObject{
		StudentId: id, Params: generated.GetAlphaStudentObservationParams{Week: &openapi_types.Date{Time: monday}},
	})
	w.lastResp, w.lastErr = resp, err
	if ok, isOK := resp.(generated.GetAlphaStudentObservation200JSONResponse); isOK {
		observation := generated.AlphaStudentObservation(ok)
		w.alpha.student = &observation
	}
	return err
}

// observed is name as the last read reported them: from the alpha's list,
// or the one student read.
func (w *world) observed(name string) (generated.AlphaStudent, error) {
	id, ok := w.userMotifID[name]
	if !ok {
		return generated.AlphaStudent{}, fmt.Errorf("%s was never registered", name)
	}
	if w.alpha.student != nil && w.alpha.student.Student.StudentId == id {
		return w.alpha.student.Student, nil
	}
	if w.alpha.observation == nil {
		return generated.AlphaStudent{}, fmt.Errorf("the alpha wasn't observed: %#v (err=%v)", w.lastResp, w.lastErr)
	}
	for _, s := range w.alpha.observation.Students {
		if s.StudentId == id {
			return s, nil
		}
	}
	return generated.AlphaStudent{}, fmt.Errorf("%s is not one of the alpha's students", name)
}

func (w *world) alphaSummary() (generated.AlphaCohortSummary, error) {
	if w.alpha.observation == nil {
		return generated.AlphaCohortSummary{}, fmt.Errorf("the alpha wasn't observed: %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return w.alpha.observation.Summary, nil
}

func expect[T comparable](what string, got, want T) error {
	if got != want {
		return fmt.Errorf("%s: got %v, want %v", what, got, want)
	}
	return nil
}

func (w *world) isOneOfItsStudents(name, not string) error {
	_, err := w.observed(name)
	if not == "" {
		return err
	}
	if w.alpha.observation == nil {
		return fmt.Errorf("the alpha wasn't observed: %#v", w.lastResp)
	}
	if err == nil {
		return fmt.Errorf("%s is one of the alpha's students", name)
	}
	return nil
}

func (w *world) hasCountedSessions(name string, n int) error {
	s, err := w.observed(name)
	if err != nil {
		return err
	}
	return expect("counted sessions", s.FinishedSessionsInWeek, n)
}

func (w *world) isAtTargetInWeek(name, not string) error {
	s, err := w.observed(name)
	if err != nil {
		return err
	}
	return expect("at the target in the week", s.AtTargetInWeek, not == "")
}

func (w *world) weekIsRunning(week string) error {
	if err := w.observedForWeek(week); err != nil {
		return err
	}
	return expect("running", w.alpha.observation.Week.Running, true)
}

func (w *world) observedForWeek(week string) error {
	monday, err := weekMonday(week)
	if err != nil {
		return err
	}
	if w.alpha.observation == nil {
		return fmt.Errorf("the alpha wasn't observed: %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return expect("week start", w.alpha.observation.Week.WeekStart.Time, monday)
}

func (w *world) weeksSoFarAre(name, w1 string, n1 int, w2 string, n2 int, w3 string, n3 int) error {
	s, err := w.observed(name)
	if err != nil {
		return err
	}
	var want []generated.AlphaWeekCount
	for i, week := range []string{w1, w2, w3} {
		monday, err := weekMonday(week)
		if err != nil {
			return err
		}
		want = append(want, generated.AlphaWeekCount{WeekStart: openapi_types.Date{Time: monday}, FinishedSessions: []int{n1, n2, n3}[i]})
	}
	if !slices.Equal(s.WeeksSoFar, want) {
		return fmt.Errorf("weeks so far: got %+v, want %+v", s.WeeksSoFar, want)
	}
	return nil
}

func (w *world) atTargetInWeeksSoFar(name string, atTarget, of int) error {
	s, err := w.observed(name)
	if err != nil {
		return err
	}
	if err := expect("weeks at the target", s.WeeksAtTargetCount, atTarget); err != nil {
		return err
	}
	return expect("weeks so far", s.WeeksSoFarCount, of)
}

func (w *world) notAtTargetEveryWeek(name string) error {
	s, err := w.observed(name)
	if err != nil {
		return err
	}
	return expect("at the target every week", s.AtTargetEveryWeek, false)
}

func (w *world) hasNoWeeksSoFar(name string) error {
	s, err := w.observed(name)
	if err != nil {
		return err
	}
	if len(s.WeeksSoFar) != 0 || s.WeeksSoFarCount != 0 {
		return fmt.Errorf("weeks so far: got %+v (%d)", s.WeeksSoFar, s.WeeksSoFarCount)
	}
	return nil
}

func (w *world) weeksSoFarList(name string, n int, from, to string) error {
	s, err := w.observed(name)
	if err != nil {
		return err
	}
	first, err := weekMonday(from)
	if err != nil {
		return err
	}
	last, err := weekMonday(to)
	if err != nil {
		return err
	}
	if len(s.WeeksSoFar) != n {
		return fmt.Errorf("weeks listed: got %d, want %d", len(s.WeeksSoFar), n)
	}
	if err := expect("first week listed", s.WeeksSoFar[0].WeekStart.Time, first); err != nil {
		return err
	}
	return expect("last week listed", s.WeeksSoFar[n-1].WeekStart.Time, last)
}

func (w *world) weeksSoFarCount(name string, n int) error {
	s, err := w.observed(name)
	if err != nil {
		return err
	}
	return expect("weeks so far counted", s.WeeksSoFarCount, n)
}

func (w *world) studentObservation() (*generated.AlphaStudentObservation, error) {
	if w.alpha.student == nil {
		return nil, fmt.Errorf("no student was observed: %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return w.alpha.student, nil
}

func (w *world) nodeListedNotStarted(position int) error {
	o, err := w.studentObservation()
	if err != nil {
		return err
	}
	node := o.FirstPathNodes[position-1]
	if err := expect("position", node.Position, position); err != nil {
		return err
	}
	return expect("status", node.Status, generated.AlphaPathNodeStatusNotStarted)
}

func (w *world) firstPathAt(name, title string, completed, total, percent int) error {
	s, err := w.observed(name)
	if err != nil {
		return err
	}
	p := s.FirstPath
	return expect("first path", fmt.Sprintf("%s %d/%d %d%%", p.Title, p.CompletedNodes, p.TotalNodes, p.CompletedPercent),
		fmt.Sprintf("%s %d/%d %d%%", title, completed, total, percent))
}

func (w *world) firstPathOverTarget(name, not string) error {
	s, err := w.observed(name)
	if err != nil {
		return err
	}
	return expect("over the target", s.FirstPath.OverTarget, not == "")
}

func (w *world) firstPathPercent(name string, percent int) error {
	s, err := w.observed(name)
	if err != nil {
		return err
	}
	return expect("first path percent", s.FirstPath.CompletedPercent, percent)
}

func (w *world) firstPathTitled(name, title string) error {
	s, err := w.observed(name)
	if err != nil {
		return err
	}
	return expect("first path title", s.FirstPath.Title, title)
}

func (w *world) cohortAtTargetThisAndLastWeek(inWeek, of1, inPrevious, of2 int) error {
	s, err := w.alphaSummary()
	if err != nil {
		return err
	}
	return expect("summary", fmt.Sprintf("%d/%d %d/%d", s.AtTargetInWeek, s.StudentCount, s.AtTargetInPreviousWeek, s.StudentCount),
		fmt.Sprintf("%d/%d %d/%d", inWeek, of1, inPrevious, of2))
}

func (w *world) cohortEveryWeekIs(n, of int) error {
	s, err := w.alphaSummary()
	if err != nil {
		return err
	}
	return expect("every week", fmt.Sprintf("%d/%d", s.AtTargetEveryWeek, s.StudentCount), fmt.Sprintf("%d/%d", n, of))
}

func (w *world) cohortOverTargetIs(n, of int) error {
	s, err := w.alphaSummary()
	if err != nil {
		return err
	}
	return expect("over the target", fmt.Sprintf("%d/%d", s.FirstPathOverTarget, s.StudentCount), fmt.Sprintf("%d/%d", n, of))
}

func (w *world) medianIs(percent int) error {
	s, err := w.alphaSummary()
	if err != nil {
		return err
	}
	if s.MedianFirstPathPercent == nil {
		return fmt.Errorf("no median")
	}
	return expect("median", *s.MedianFirstPathPercent, percent)
}

func (w *world) studentsListedAs(a, b, c string) error {
	if w.alpha.observation == nil {
		return fmt.Errorf("the alpha wasn't observed: %#v", w.lastResp)
	}
	var got []string
	for _, s := range w.alpha.observation.Students {
		got = append(got, s.DisplayName)
	}
	want := []string{w.nameClaim(a), w.nameClaim(b), w.nameClaim(c)}
	if !slices.Equal(got, want) {
		return fmt.Errorf("students listed: got %v, want %v", got, want)
	}
	return nil
}

func (w *world) lastActivityIs(name, day, hhmm string) error {
	s, err := w.observed(name)
	if err != nil {
		return err
	}
	want, err := alphaDay(day, hhmm)
	if err != nil {
		return err
	}
	if s.LastActiveAt == nil || !s.LastActiveAt.Equal(want) {
		return fmt.Errorf("last activity: got %v, want %v", s.LastActiveAt, want)
	}
	return nil
}

func (w *world) isQuiet(name, not string) error {
	s, err := w.observed(name)
	if err != nil {
		return err
	}
	return expect("quiet", s.Quiet, not == "")
}

func (w *world) hasNoLastActivity(name string) error {
	s, err := w.observed(name)
	if err != nil {
		return err
	}
	if s.LastActiveAt != nil {
		return fmt.Errorf("last activity: got %v, want none", *s.LastActiveAt)
	}
	return nil
}

func (w *world) sessionWeeksListed(week, previous string) error {
	o, err := w.studentObservation()
	if err != nil {
		return err
	}
	for i, name := range []string{week, previous} {
		monday, err := weekMonday(name)
		if err != nil {
			return err
		}
		listed := o.SessionWeeks[i]
		if err := expect("session week", listed.Week.WeekStart.Time, monday); err != nil {
			return err
		}
		if !slices.IsSortedFunc(listed.Sessions, func(a, b generated.AlphaSession) int { return b.StartedAt.Compare(a.StartedAt) }) {
			return fmt.Errorf("sessions of the week of %s aren't newest first", name)
		}
		if len(listed.Sessions) == 0 {
			return fmt.Errorf("no session listed in the week of %s", name)
		}
	}
	return nil
}

func (w *world) eachSessionShowsItsFacts() error {
	o, err := w.studentObservation()
	if err != nil {
		return err
	}
	want := []struct {
		state  generated.AlphaSessionState
		ended  time.Duration
		answer int
	}{
		{generated.AlphaSessionStateFinished, 10 * time.Minute, 6},
		{generated.AlphaSessionStateEndedEarly, 4 * time.Minute, 6},
	}
	for i, week := range o.SessionWeeks {
		s := week.Sessions[0]
		if s.EndedAt == nil || !s.EndedAt.Equal(s.StartedAt.Add(want[i].ended)) {
			return fmt.Errorf("session %d end: got %v", i, s.EndedAt)
		}
		if err := expect("state", s.State, want[i].state); err != nil {
			return err
		}
		if err := expect("planned minutes", s.PlannedMinutes, 10); err != nil {
			return err
		}
		if err := expect("items answered", s.ItemsAnswered, want[i].answer); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) nodesListedInOrder(n int, title string) error {
	o, err := w.studentObservation()
	if err != nil {
		return err
	}
	if len(o.FirstPathNodes) != n {
		return fmt.Errorf("nodes listed: got %d, want %d", len(o.FirstPathNodes), n)
	}
	for i, node := range o.FirstPathNodes {
		if err := expect("node", node.Title, fmt.Sprintf("%s %d", title, i+1)); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) nodesByStatus(completedTo, inProgress, notStartedFrom, notStartedTo int) error {
	o, err := w.studentObservation()
	if err != nil {
		return err
	}
	for _, node := range o.FirstPathNodes {
		switch {
		case node.Position <= completedTo:
			if node.Status != generated.AlphaPathNodeStatusCompleted || node.CompletedAt == nil || node.StartedAt == nil {
				return fmt.Errorf("node %d: got %s, completed %v, started %v", node.Position, node.Status, node.CompletedAt, node.StartedAt)
			}
		case node.Position == inProgress:
			if node.Status != generated.AlphaPathNodeStatusInProgress || node.CompletedAt != nil || node.StartedAt == nil {
				return fmt.Errorf("node %d: got %s, completed %v, started %v", node.Position, node.Status, node.CompletedAt, node.StartedAt)
			}
		case node.Position >= notStartedFrom && node.Position <= notStartedTo:
			if node.Status != generated.AlphaPathNodeStatusNotStarted || node.CompletedAt != nil || node.StartedAt != nil {
				return fmt.Errorf("node %d: got %s, completed %v, started %v", node.Position, node.Status, node.CompletedAt, node.StartedAt)
			}
		}
	}
	return nil
}

func (w *world) alphaRefused(string) error {
	switch w.lastResp.(type) {
	case generated.GetAlphaObservation403JSONResponse, generated.GetAlphaStudentObservation403JSONResponse:
		return nil
	}
	return fmt.Errorf("expected a refusal for lacking permission, got %#v (err=%v)", w.lastResp, w.lastErr)
}

func (w *world) noStudentFound() error {
	if _, ok := w.lastResp.(generated.GetAlphaStudentObservation404JSONResponse); ok {
		return nil
	}
	return fmt.Errorf("expected no student found, got %#v (err=%v)", w.lastResp, w.lastErr)
}
