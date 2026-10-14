package application_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// fakeAlphaActivityReader is an in-memory ports.AlphaActivityReader.
type fakeAlphaActivityReader struct {
	sessions    []domain.PracticeSessionRecord
	completions map[string]map[string]time.Time // studentID -> contentNodeID -> first completion
	starts      map[string]map[string]time.Time // studentID -> contentNodeID -> first start
	lastActive  map[string]time.Time
}

func newFakeAlphaActivityReader() *fakeAlphaActivityReader {
	return &fakeAlphaActivityReader{
		completions: map[string]map[string]time.Time{},
		starts:      map[string]map[string]time.Time{},
		lastActive:  map[string]time.Time{},
	}
}

func (f *fakeAlphaActivityReader) SessionsStartedBetween(_ context.Context, studentIDs []string, from, to time.Time) ([]domain.PracticeSessionRecord, error) {
	var out []domain.PracticeSessionRecord
	for _, s := range f.sessions {
		if slices.Contains(studentIDs, s.StudentID) && !s.StartedAt.Before(from) && s.StartedAt.Before(to) {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeAlphaActivityReader) FirstCompletions(_ context.Context, studentID string, contentNodeIDs []string) (map[string]time.Time, error) {
	return pick(f.completions[studentID], contentNodeIDs), nil
}

func (f *fakeAlphaActivityReader) FirstStarts(_ context.Context, studentID string, contentNodeIDs []string) (map[string]time.Time, error) {
	return pick(f.starts[studentID], contentNodeIDs), nil
}

func (f *fakeAlphaActivityReader) LastActive(_ context.Context, studentIDs []string) (map[string]time.Time, error) {
	return pick(f.lastActive, studentIDs), nil
}

func pick(from map[string]time.Time, keys []string) map[string]time.Time {
	out := map[string]time.Time{}
	for _, k := range keys {
		if t, ok := from[k]; ok {
			out[k] = t
		}
	}
	return out
}

type alphaFixture struct {
	users      *fakeUserRepository
	paths      *fakeStudentPathRepository
	nodes      *fakeContentNodeRepository
	completion *fakeCompletionStateReader
	activity   *fakeAlphaActivityReader
	svc        *application.AlphaObservationService
	now        time.Time
	admin      domain.User
}

func newAlphaFixture(t *testing.T) *alphaFixture {
	t.Helper()
	loc, err := time.LoadLocation("America/Sao_Paulo")
	require.NoError(t, err)
	f := &alphaFixture{
		users: newFakeUserRepository(), paths: newFakeStudentPathRepository(), nodes: newFakeContentNodeRepository(),
		completion: newFakeCompletionStateReader(), activity: newFakeAlphaActivityReader(),
		now:   time.Date(2026, 10, 14, 10, 0, 0, 0, loc), // a Wednesday
		admin: domain.User{ID: "admin-ana", Role: domain.RoleAdmin, DisplayName: "ana"},
	}
	f.svc = application.NewAlphaObservationService(f.users, f.paths, f.nodes, f.completion, f.activity, func() time.Time { return f.now })
	return f
}

func (f *alphaFixture) at(month time.Month, day, hour, minute int) time.Time {
	return time.Date(2026, month, day, hour, minute, 0, 0, f.now.Location())
}

// user registers a user with role, named name.
func (f *alphaFixture) user(name string, role domain.Role) string {
	id := "user-" + name
	f.users.put(domain.User{ID: id, ClerkUserID: "clerk-" + name, Role: role, DisplayName: name, RegisteredAt: f.at(time.September, 1, 9, 0)})
	return id
}

// path gives studentID a path titled title of nodes nodes, at givenAt.
func (f *alphaFixture) path(studentID, title string, nodes int, givenAt time.Time) string {
	id := fmt.Sprintf("path-%s-%s", studentID, title)
	items := make([]domain.StudentPathItemRecord, nodes)
	for i := range items {
		nodeID := fmt.Sprintf("%s-node-%02d", title, i+1)
		items[i] = domain.StudentPathItemRecord{Position: i + 1, ContentNodeID: nodeID}
		f.nodes.byID[nodeID] = domain.ContentNode{ID: nodeID, Title: fmt.Sprintf("%s %d", title, i+1)}
	}
	f.paths.byID[id] = domain.StudentPath{ID: id, StudentID: studentID, Title: title, AssignedAt: givenAt, Items: items}
	return id
}

func (f *alphaFixture) finished(studentID string, start time.Time, answered int) {
	f.activity.sessions = append(f.activity.sessions, domain.PracticeSessionRecord{
		ID: fmt.Sprintf("session-%s-%d", studentID, len(f.activity.sessions)), StudentID: studentID,
		StartedAt: start, PlannedMinutes: 10, LastEventAt: start.Add(10 * time.Minute),
		End: &domain.PracticeSessionRecordEnd{At: start.Add(10 * time.Minute), AnsweredCount: answered},
	})
}

func studentIDs(o application.AlphaObservation) []string {
	var ids []string
	for _, s := range o.Students {
		ids = append(ids, s.Student.StudentID)
	}
	return ids
}

func TestAlphaObservationService_Observe(t *testing.T) {
	t.Run("the alpha is every student given a path", func(t *testing.T) {
		f := newAlphaFixture(t)
		alice := f.user("alice", domain.RoleStudent)
		f.path(alice, "Guitar foundations", 18, f.at(time.September, 21, 9, 0))
		f.user("bruno", domain.RoleStudent)
		tina := f.user("tina", domain.RoleTeacher)
		f.path(tina, "Blues guitar basics", 6, f.at(time.September, 21, 9, 0))

		got, err := f.svc.Observe(context.Background(), f.admin, nil)

		require.NoError(t, err)
		assert.Equal(t, []string{alice}, studentIDs(got))
		assert.Equal(t, 1, got.Summary.StudentCount)
		assert.Equal(t, time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC), got.Week.FirstDay())
		assert.True(t, got.Running)
	})

	t.Run("a student is measured from their first path, kept after a later one replaced it", func(t *testing.T) {
		f := newAlphaFixture(t)
		alice := f.user("alice", domain.RoleStudent)
		first := f.path(alice, "Guitar foundations", 18, f.at(time.September, 21, 9, 0))
		archivedAt := f.at(time.October, 5, 9, 0)
		archived := f.paths.byID[first]
		archived.ArchivedAt = &archivedAt
		f.paths.byID[first] = archived
		f.path(alice, "Blues guitar basics", 6, f.at(time.October, 5, 9, 0))
		for i := 1; i <= 11; i++ {
			f.completion.set(alice, fmt.Sprintf("Guitar foundations-node-%02d", i), domain.CompletionStatusCompleted)
		}
		f.completion.set(alice, "Guitar foundations-node-12", domain.CompletionStatusInProgress)
		f.finished(alice, f.at(time.October, 12, 18, 30), 9)
		f.finished(alice, f.at(time.October, 14, 7, 40), 6)
		f.activity.lastActive[alice] = f.at(time.October, 14, 7, 50)

		got, err := f.svc.Observe(context.Background(), f.admin, nil)

		require.NoError(t, err)
		require.Len(t, got.Students, 1)
		s := got.Students[0]
		assert.Equal(t, first, s.FirstPathID)
		assert.Equal(t, "Guitar foundations", s.FirstPathTitle)
		assert.Equal(t, f.at(time.September, 21, 9, 0), s.FirstPathGivenAt)
		assert.Equal(t, domain.FirstPathProgress{Completed: 11, Total: 18}, s.Student.FirstPath)
		assert.Equal(t, 2, s.Student.Measure.FinishedInWeek)
		assert.Equal(t, 3, s.Student.Measure.WeeksSoFarCount)
		assert.Equal(t, ptr(f.at(time.October, 14, 7, 50)), s.LastActiveAt)
		assert.False(t, s.Student.Measure.Quiet)
		assert.Equal(t, f.at(time.September, 1, 9, 0), s.RegisteredAt)
	})

	t.Run("students are listed by fewest counted sessions, then by name", func(t *testing.T) {
		f := newAlphaFixture(t)
		for _, name := range []string{"bia", "caio", "alice"} {
			id := f.user(name, domain.RoleStudent)
			f.path(id, "Guitar foundations", 18, f.at(time.September, 21, 9, 0))
			if name != "caio" {
				f.finished(id, f.at(time.October, 12, 18, 30), 4)
			}
		}

		got, err := f.svc.Observe(context.Background(), f.admin, nil)

		require.NoError(t, err)
		assert.Equal(t, []string{"user-caio", "user-alice", "user-bia"}, studentIDs(got))
	})

	t.Run("only admins may observe", func(t *testing.T) {
		f := newAlphaFixture(t)

		_, err := f.svc.Observe(context.Background(), domain.User{ID: "t", Role: domain.RoleTeacher}, nil)

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("a week that isn't a Monday is invalid", func(t *testing.T) {
		f := newAlphaFixture(t)
		wednesday := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)

		_, err := f.svc.Observe(context.Background(), f.admin, &wednesday)

		var verr *domain.ValidationError
		require.ErrorAs(t, err, &verr)
		assert.Equal(t, "week", verr.Fields[0].Field)
	})
}

func TestAlphaObservationService_ObserveStudent(t *testing.T) {
	t.Run("lists the sessions of the week and the week before, newest first, and the first path node by node", func(t *testing.T) {
		f := newAlphaFixture(t)
		alice := f.user("alice", domain.RoleStudent)
		f.path(alice, "Guitar foundations", 14, f.at(time.September, 21, 9, 0))
		for i := 1; i <= 11; i++ {
			node := fmt.Sprintf("Guitar foundations-node-%02d", i)
			f.completion.set(alice, node, domain.CompletionStatusCompleted)
			f.activity.completions[alice] = merge(f.activity.completions[alice], node, f.at(time.September, 21+i, 20, 0))
			f.activity.starts[alice] = merge(f.activity.starts[alice], node, f.at(time.September, 21+i, 19, 0))
		}
		f.completion.set(alice, "Guitar foundations-node-12", domain.CompletionStatusInProgress)
		f.activity.starts[alice] = merge(f.activity.starts[alice], "Guitar foundations-node-12", f.at(time.October, 13, 19, 0))
		f.completion.set(alice, "Guitar foundations-node-13", domain.CompletionStatusLocked)
		f.finished(alice, f.at(time.October, 12, 18, 30), 9)
		earlyStart := f.at(time.October, 10, 9, 0)
		f.activity.sessions = append(f.activity.sessions, domain.PracticeSessionRecord{
			ID: "early", StudentID: alice, StartedAt: earlyStart, PlannedMinutes: 10, LastEventAt: earlyStart.Add(4 * time.Minute),
			End: &domain.PracticeSessionRecordEnd{At: earlyStart.Add(4 * time.Minute), LeftEarly: true, AnsweredCount: 2},
		})
		f.finished(alice, f.at(time.September, 30, 9, 0), 3) // two weeks before: not listed

		got, err := f.svc.ObserveStudent(context.Background(), f.admin, alice, nil)

		require.NoError(t, err)
		assert.Equal(t, alice, got.Student.Student.StudentID)
		require.Len(t, got.SessionWeeks, 2)
		assert.Equal(t, time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC), got.SessionWeeks[0].Week.FirstDay())
		assert.Equal(t, 1, got.SessionWeeks[0].Finished)
		require.Len(t, got.SessionWeeks[0].Sessions, 1)
		assert.Equal(t, domain.PracticeSessionFinished, got.SessionWeeks[0].Sessions[0].State)
		assert.Equal(t, time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), got.SessionWeeks[1].Week.FirstDay())
		assert.Equal(t, 0, got.SessionWeeks[1].Finished)
		require.Len(t, got.SessionWeeks[1].Sessions, 1)
		assert.Equal(t, domain.PracticeSessionEndedEarly, got.SessionWeeks[1].Sessions[0].State)
		assert.Equal(t, ptr(earlyStart.Add(4*time.Minute)), got.SessionWeeks[1].Sessions[0].EndedAt)

		require.Len(t, got.Nodes, 14)
		assert.Equal(t, application.AlphaPathNode{
			Position: 1, ContentNodeID: "Guitar foundations-node-01", Title: "Guitar foundations 1",
			Status: application.AlphaNodeCompleted, CompletedAt: ptr(f.at(time.September, 22, 20, 0)), StartedAt: ptr(f.at(time.September, 22, 19, 0)),
		}, got.Nodes[0])
		assert.Equal(t, application.AlphaNodeInProgress, got.Nodes[11].Status)
		assert.Nil(t, got.Nodes[11].CompletedAt)
		assert.Equal(t, ptr(f.at(time.October, 13, 19, 0)), got.Nodes[11].StartedAt)
		assert.Equal(t, application.AlphaNodeNotStarted, got.Nodes[12].Status, "a locked node is not started")
		assert.Equal(t, application.AlphaNodeNotStarted, got.Nodes[13].Status)
		assert.Nil(t, got.Nodes[13].StartedAt)
	})

	t.Run("sessions in the same week are newest first", func(t *testing.T) {
		f := newAlphaFixture(t)
		alice := f.user("alice", domain.RoleStudent)
		f.path(alice, "Guitar foundations", 3, f.at(time.September, 21, 9, 0))
		f.finished(alice, f.at(time.October, 12, 18, 30), 9)
		f.finished(alice, f.at(time.October, 14, 7, 40), 6)

		got, err := f.svc.ObserveStudent(context.Background(), f.admin, alice, nil)

		require.NoError(t, err)
		require.Len(t, got.SessionWeeks[0].Sessions, 2)
		assert.Equal(t, f.at(time.October, 14, 7, 40), got.SessionWeeks[0].Sessions[0].Record.StartedAt)
	})

	t.Run("someone outside the alpha is not found", func(t *testing.T) {
		f := newAlphaFixture(t)
		bruno := f.user("bruno", domain.RoleStudent)
		tina := f.user("tina", domain.RoleTeacher)
		f.path(tina, "Blues guitar basics", 6, f.at(time.September, 21, 9, 0))

		for _, id := range []string{bruno, tina, "nobody"} {
			_, err := f.svc.ObserveStudent(context.Background(), f.admin, id, nil)
			assert.ErrorIs(t, err, domain.ErrNotFound, id)
		}
	})

	t.Run("only admins may observe a student", func(t *testing.T) {
		f := newAlphaFixture(t)

		_, err := f.svc.ObserveStudent(context.Background(), domain.User{ID: "t", Role: domain.RoleStudent}, "user-alice", nil)

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func merge(m map[string]time.Time, key string, t time.Time) map[string]time.Time {
	if m == nil {
		m = map[string]time.Time{}
	}
	m[key] = t
	return m
}
