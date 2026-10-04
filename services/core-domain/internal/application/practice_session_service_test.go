package application_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// fakePracticeItemStateReader is an in-memory ports.PracticeItemStateReader
// holding states per student and item key.
type fakePracticeItemStateReader struct {
	mu     sync.Mutex
	states map[string]map[string]domain.PracticeItemState
	err    error
}

func newFakePracticeItemStateReader() *fakePracticeItemStateReader {
	return &fakePracticeItemStateReader{states: map[string]map[string]domain.PracticeItemState{}}
}

func (f *fakePracticeItemStateReader) GetStates(_ context.Context, studentID string, itemKeys []string) (map[string]domain.PracticeItemState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	result := map[string]domain.PracticeItemState{}
	for _, key := range itemKeys {
		if s, ok := f.states[studentID][key]; ok {
			result[key] = s
		}
	}
	return result, nil
}

func (f *fakePracticeItemStateReader) put(studentID string, s domain.PracticeItemState) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.states[studentID] == nil {
		f.states[studentID] = map[string]domain.PracticeItemState{}
	}
	f.states[studentID][s.ItemKey] = s
}

const (
	practiceGuitar = "guitar"
	practiceBass   = "electric-bass"
)

type practiceFixture struct {
	svc          *application.PracticeSessionService
	instruments  *fakeInstrumentRepository
	studentPaths *fakeStudentPathRepository
	enrollments  *fakeCourseEnrollmentRepository
	contentNodes *fakeContentNodeRepository
	diagrams     *fakeDiagramRepository
	states       *fakePracticeItemStateReader
	now          time.Time
	pathCount    int
	t            *testing.T
}

func newPracticeFixture(t *testing.T) *practiceFixture {
	f := &practiceFixture{
		instruments:  newFakeInstrumentRepository(),
		studentPaths: newFakeStudentPathRepository(),
		enrollments:  newFakeCourseEnrollmentRepository(),
		contentNodes: newFakeContentNodeRepository(),
		diagrams:     newFakeDiagramRepository(),
		states:       newFakePracticeItemStateReader(),
		now:          time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC),
		t:            t,
	}
	f.instruments.put(domain.Instrument{ID: practiceGuitar})
	f.instruments.put(domain.Instrument{ID: practiceBass})
	f.svc = application.NewPracticeSessionService(
		f.instruments, f.studentPaths, f.enrollments, f.contentNodes, f.diagrams, f.states,
		func() string { return "session-1" },
		func() time.Time { return f.now },
	)
	return f
}

// onPath puts a standalone path for the student whose one content node
// teaches skillIDs.
func (f *practiceFixture) onPath(skillIDs ...string) {
	f.pathCount++
	node := domain.ContentNode{ID: fmt.Sprintf("node-%d", f.pathCount)}
	for _, id := range skillIDs {
		node.Classification.Skills = append(node.Classification.Skills, domain.KnowledgeNode{ID: id})
	}
	f.contentNodes.put(node)
	require.NoError(f.t, f.studentPaths.Create(context.Background(), domain.StudentPath{
		ID:               fmt.Sprintf("path-%d", f.pathCount),
		StudentID:        studentCaller().ID,
		SourceTemplateID: fmt.Sprintf("template-%d", f.pathCount),
		AssignedAt:       f.now.Add(time.Duration(f.pathCount) * time.Minute),
		Items:            []domain.StudentPathItemRecord{{Position: 1, ContentNodeID: node.ID}},
	}))
}

// playAlong puts a diagram of eight quarter notes in 4/4 for instrumentID
// at tempo, classified under skillID. A tempo of 0 leaves it without
// playback.
func (f *practiceFixture) playAlong(id, name, instrumentID, skillID string, tempo int) {
	d := domain.Diagram{
		ID:            id,
		InstrumentID:  instrumentID,
		InstrumentIDs: []string{instrumentID},
		Names:         names(name),
		Kind:          domain.DiagramKindBasic,
		TimeSignature: domain.DefaultTimeSignature,
		Skills:        []domain.KnowledgeNode{{ID: skillID}},
	}
	if tempo > 0 {
		d.TempoBPM = &tempo
		for range 8 {
			d.Sequence = append(d.Sequence, domain.SequenceStep{PositionIDs: []string{"p1"}, Value: domain.NoteValue{Num: 1, Den: 4}})
		}
	}
	require.NoError(f.t, f.diagrams.Create(context.Background(), d))
}

func (f *practiceFixture) state(diagramID string, s domain.PracticeItemState) {
	s.ItemKey = "play_along:" + diagramID
	if s.RulesVersion == 0 {
		s.RulesVersion = domain.PracticeRulesVersion
	}
	f.states.put(studentCaller().ID, s)
}

func (f *practiceFixture) daysAgo(days int) *time.Time {
	t := f.now.AddDate(0, 0, -days)
	return &t
}

func (f *practiceFixture) inDays(days int) *time.Time {
	t := f.now.AddDate(0, 0, days)
	return &t
}

func (f *practiceFixture) compose(t *testing.T, instrumentID string, minutes int) domain.PracticeSessionPlan {
	t.Helper()
	plan, err := f.svc.ComposePlan(context.Background(), studentCaller(), &instrumentID, minutes)
	require.NoError(t, err)
	return plan
}

func planKeys(plan domain.PracticeSessionPlan) []string {
	keys := make([]string, len(plan.Items))
	for i, item := range plan.Items {
		keys[i] = item.ItemKey
	}
	return keys
}

func planSeconds(plan domain.PracticeSessionPlan) int {
	total := 0
	for _, item := range plan.Items {
		total += item.EstimatedSeconds
	}
	return total
}

func TestPracticeSessionService_ComposePlan(t *testing.T) {
	ctx := context.Background()

	t.Run("a session longer than an hour, or under a minute, is refused on minutes", func(t *testing.T) {
		for _, minutes := range []int{0, 61} {
			f := newPracticeFixture(t)
			guitar := practiceGuitar

			_, err := f.svc.ComposePlan(ctx, studentCaller(), &guitar, minutes)

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr, "minutes %d", minutes)
			assert.Equal(t, "minutes", valErr.Fields[0].Field)
		}
	})

	t.Run("an instrument that doesn't exist is not found", func(t *testing.T) {
		f := newPracticeFixture(t)
		missing := "no-such-instrument"

		_, err := f.svc.ComposePlan(ctx, studentCaller(), &missing, 10)

		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a session in the head has nothing to practise yet, so it is not found", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.playAlong("d1", "Lick", practiceGuitar, "skill-1", 120)

		_, err := f.svc.ComposePlan(ctx, studentCaller(), nil, 10)

		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a play-along with no clean take yet is new, starting at 60% of the diagram's tempo rounded down to 5 BPM", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.playAlong("d1", "Pentatonic run", practiceGuitar, "skill-1", 120)

		plan := f.compose(t, practiceGuitar, 10)

		assert.Equal(t, "session-1", plan.ID)
		require.NotNil(t, plan.InstrumentID)
		assert.Equal(t, practiceGuitar, *plan.InstrumentID)
		assert.Equal(t, 10, plan.Minutes)
		require.Len(t, plan.Items, 1)
		item := plan.Items[0]
		assert.Equal(t, "play_along:d1", item.ItemKey)
		assert.Equal(t, domain.PracticeItemKindPlayAlong, item.Kind)
		assert.Equal(t, domain.PracticePickNew, item.Reason)
		assert.Equal(t, domain.KnowledgeLevelNew, item.Level)
		require.NotNil(t, item.NodeID)
		assert.Equal(t, "skill-1", *item.NodeID)
		require.NotNil(t, item.PlayAlong)
		assert.Equal(t, domain.PlannedPlayAlong{DiagramID: "d1", StartTempoBPM: 70, TargetTempoBPM: 120}, *item.PlayAlong)
	})

	t.Run("a due play-along starts at the student's best clean tempo", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.playAlong("d1", "Pentatonic run", practiceGuitar, "skill-1", 120)
		f.state("d1", domain.PracticeItemState{Level: domain.KnowledgeLevelLearning, Counted: 2, Box: 1, DueAt: f.daysAgo(0), BestCleanBPM: intPtr(90)})

		plan := f.compose(t, practiceGuitar, 3)

		require.Len(t, plan.Items, 1)
		assert.Equal(t, domain.PracticePickDue, plan.Items[0].Reason)
		assert.Equal(t, domain.PlannedPlayAlong{DiagramID: "d1", StartTempoBPM: 90, TargetTempoBPM: 120, BestCleanTempoBPM: intPtr(90)}, *plan.Items[0].PlayAlong)
	})

	t.Run("start tempos stay between 20 BPM and the target", func(t *testing.T) {
		for _, tc := range []struct {
			name      string
			target    int
			bestClean *int
			want      int
		}{
			{name: "a best clean tempo above the target starts at the target", target: 100, bestClean: intPtr(130), want: 100},
			{name: "60% of a slow target below 20 BPM starts at 20 BPM", target: 25, want: 20},
		} {
			f := newPracticeFixture(t)
			f.onPath("skill-1")
			f.playAlong("d1", "Lick", practiceGuitar, "skill-1", tc.target)
			if tc.bestClean != nil {
				f.state("d1", domain.PracticeItemState{Level: domain.KnowledgeLevelLearning, Counted: 2, Box: 1, DueAt: f.daysAgo(0), BestCleanBPM: tc.bestClean})
			}

			plan := f.compose(t, practiceGuitar, 3)

			require.Len(t, plan.Items, 1, tc.name)
			assert.Equal(t, tc.want, plan.Items[0].PlayAlong.StartTempoBPM, tc.name)
		}
	})

	t.Run("a session of 5 minutes or more starts with a warm-up at 80% of a clean play-along's best tempo", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.playAlong("scale", "C major scale", practiceGuitar, "skill-1", 120)
		f.playAlong("run", "Pentatonic run", practiceGuitar, "skill-1", 120)
		f.state("scale", domain.PracticeItemState{Level: domain.KnowledgeLevelAccurate, Counted: 4, Box: 3, DueAt: f.inDays(3), BestCleanBPM: intPtr(100)})

		plan := f.compose(t, practiceGuitar, 10)

		require.Len(t, plan.Items, 2)
		warmUp := plan.Items[0]
		assert.Equal(t, "play_along:scale", warmUp.ItemKey)
		assert.Equal(t, domain.PracticePickWarmUp, warmUp.Reason)
		assert.Equal(t, 80, warmUp.PlayAlong.StartTempoBPM)
		assert.Equal(t, "play_along:run", plan.Items[1].ItemKey)
	})

	t.Run("a short session skips the warm-up", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.playAlong("scale", "C major scale", practiceGuitar, "skill-1", 120)
		f.playAlong("run", "Pentatonic run", practiceGuitar, "skill-1", 120)
		f.state("scale", domain.PracticeItemState{Level: domain.KnowledgeLevelAccurate, Counted: 4, Box: 3, DueAt: f.inDays(3), BestCleanBPM: intPtr(100)})

		plan := f.compose(t, practiceGuitar, 3)

		for _, item := range plan.Items {
			assert.NotEqual(t, domain.PracticePickWarmUp, item.Reason)
		}
	})

	t.Run("the warm-up comes first, then due items, most overdue first, then new items", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.playAlong("a-new", "A new", practiceGuitar, "skill-1", 100)
		f.playAlong("b-due-today", "B due today", practiceGuitar, "skill-1", 100)
		f.playAlong("c-due-long-ago", "C due long ago", practiceGuitar, "skill-1", 100)
		f.playAlong("d-known", "D known", practiceGuitar, "skill-1", 100)
		f.state("b-due-today", domain.PracticeItemState{Level: domain.KnowledgeLevelLearning, Counted: 1, Box: 1, DueAt: f.daysAgo(0)})
		f.state("c-due-long-ago", domain.PracticeItemState{Level: domain.KnowledgeLevelLearning, Counted: 1, Box: 1, DueAt: f.daysAgo(1)})
		f.state("d-known", domain.PracticeItemState{Level: domain.KnowledgeLevelFluent, Counted: 6, Box: 4, DueAt: f.inDays(5), BestCleanBPM: intPtr(100)})

		plan := f.compose(t, practiceGuitar, 30)

		assert.Equal(t, []string{"play_along:d-known", "play_along:c-due-long-ago", "play_along:b-due-today", "play_along:a-new"}, planKeys(plan))
		reasons := []domain.PracticePickReason{}
		for _, item := range plan.Items {
			reasons = append(reasons, item.Reason)
		}
		assert.Equal(t, []domain.PracticePickReason{domain.PracticePickWarmUp, domain.PracticePickDue, domain.PracticePickDue, domain.PracticePickNew}, reasons)
	})

	t.Run("with an instrument in hand, only play-alongs that suit it and have playback are picked", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.playAlong("guitar-lick", "Guitar lick", practiceGuitar, "skill-1", 100)
		f.playAlong("bass-line", "Bass line", practiceBass, "skill-1", 100)
		f.playAlong("silent", "No playback", practiceGuitar, "skill-1", 0)

		plan := f.compose(t, practiceBass, 10)

		assert.Equal(t, []string{"play_along:bass-line"}, planKeys(plan))
	})

	t.Run("skills come from every path the student is working through, not archived ones", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-standalone")
		node := domain.ContentNode{ID: "checkpoint-node", Classification: domain.Classification{Skills: []domain.KnowledgeNode{{ID: "skill-course"}}}}
		f.contentNodes.put(node)
		enrollmentID, checkpointPathID, position := "enrollment-1", "checkpoint-path", 1
		require.NoError(t, f.studentPaths.Create(ctx, domain.StudentPath{ID: checkpointPathID, StudentID: studentCaller().ID, SourceCourseEnrollmentID: &enrollmentID, Items: []domain.StudentPathItemRecord{{Position: 1, ContentNodeID: node.ID}}}))
		require.NoError(t, f.enrollments.Create(ctx, domain.CourseEnrollment{ID: enrollmentID, StudentID: studentCaller().ID, Status: domain.CourseEnrollmentStatusActive, ActiveCheckpointStudentPathID: &checkpointPathID, ActiveCheckpointPosition: &position}))
		archivedAt := f.now
		archivedNode := domain.ContentNode{ID: "archived-node", Classification: domain.Classification{Skills: []domain.KnowledgeNode{{ID: "skill-archived"}}}}
		f.contentNodes.put(archivedNode)
		require.NoError(t, f.studentPaths.Create(ctx, domain.StudentPath{ID: "archived-path", StudentID: studentCaller().ID, ArchivedAt: &archivedAt, Items: []domain.StudentPathItemRecord{{Position: 1, ContentNodeID: archivedNode.ID}}}))
		f.playAlong("standalone", "Standalone", practiceGuitar, "skill-standalone", 100)
		f.playAlong("course", "Course", practiceGuitar, "skill-course", 100)
		f.playAlong("archived", "Archived", practiceGuitar, "skill-archived", 100)

		plan := f.compose(t, practiceGuitar, 30)

		assert.ElementsMatch(t, []string{"play_along:standalone", "play_along:course"}, planKeys(plan))
		for _, item := range plan.Items {
			assert.Equal(t, domain.PracticePickNew, item.Reason)
		}
	})

	t.Run("each pick is fitted to the minutes by its estimated time", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		for i := range 20 {
			f.playAlong(fmt.Sprintf("d%02d", i), fmt.Sprintf("Lick %02d", i), practiceGuitar, "skill-1", 120)
		}

		plan := f.compose(t, practiceGuitar, 5)

		// Four takes of a count-in bar plus eight beats at 70 BPM, each with
		// ten seconds to rate it: 4 × (12 × 60 / 70 + 10) ≈ 82 seconds.
		assert.Equal(t, 82, plan.Items[0].EstimatedSeconds)
		assert.Len(t, plan.Items, 3)
		assert.LessOrEqual(t, planSeconds(plan), 5*60)
	})

	t.Run("a session too short for any item still offers one", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.playAlong("long", "Long", practiceGuitar, "skill-1", 20)

		plan := f.compose(t, practiceGuitar, 1)

		assert.Equal(t, []string{"play_along:long"}, planKeys(plan))
	})

	t.Run("with nothing on the student's paths, play-alongs for the instrument in hand stretch the session", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.playAlong("elsewhere", "Elsewhere", practiceGuitar, "skill-other", 100)
		f.playAlong("bass-elsewhere", "Bass elsewhere", practiceBass, "skill-other", 100)

		plan := f.compose(t, practiceGuitar, 5)

		require.Len(t, plan.Items, 1)
		assert.Equal(t, "play_along:elsewhere", plan.Items[0].ItemKey)
		assert.Equal(t, domain.PracticePickStretch, plan.Items[0].Reason)
		require.NotNil(t, plan.Items[0].NodeID)
		assert.Equal(t, "skill-other", *plan.Items[0].NodeID)
	})

	t.Run("a caught-up student with nothing unseen left reviews ahead, soonest due first", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.playAlong("later", "Later", practiceGuitar, "skill-1", 100)
		f.playAlong("sooner", "Sooner", practiceGuitar, "skill-1", 100)
		f.state("later", domain.PracticeItemState{Level: domain.KnowledgeLevelLearning, Counted: 1, Box: 2, DueAt: f.inDays(2)})
		f.state("sooner", domain.PracticeItemState{Level: domain.KnowledgeLevelLearning, Counted: 1, Box: 1, DueAt: f.inDays(1)})

		plan := f.compose(t, practiceGuitar, 3)

		assert.Equal(t, []string{"play_along:sooner", "play_along:later"}, planKeys(plan))
		for _, item := range plan.Items {
			assert.Equal(t, domain.PracticePickReviewAhead, item.Reason)
		}
	})

	t.Run("an item shows its earned level until its review is overdue by more than its wait, then one step lower", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.playAlong("fading", "Fading", practiceGuitar, "skill-1", 100)
		f.playAlong("lapsed", "Lapsed", practiceGuitar, "skill-1", 100)
		// Box 3 waits 4 days.
		f.state("fading", domain.PracticeItemState{Level: domain.KnowledgeLevelFluent, Counted: 6, Box: 3, DueAt: f.daysAgo(4)})
		f.state("lapsed", domain.PracticeItemState{Level: domain.KnowledgeLevelFluent, Counted: 6, Box: 3, DueAt: f.daysAgo(5)})

		plan := f.compose(t, practiceGuitar, 5)

		levels := map[string]domain.KnowledgeLevel{}
		for _, item := range plan.Items {
			levels[item.ItemKey] = item.Level
		}
		assert.Equal(t, domain.KnowledgeLevelFluent, levels["play_along:fading"])
		assert.Equal(t, domain.KnowledgeLevelAccurate, levels["play_along:lapsed"])
	})

	t.Run("a state folded under other mastery rules is due, keeping its best clean tempo", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.playAlong("old", "Old rules", practiceGuitar, "skill-1", 120)
		f.state("old", domain.PracticeItemState{RulesVersion: domain.PracticeRulesVersion + 1, Level: domain.KnowledgeLevelFluent, Counted: 6, Box: 4, DueAt: f.inDays(5), BestCleanBPM: intPtr(110)})

		plan := f.compose(t, practiceGuitar, 3)

		require.Len(t, plan.Items, 1)
		assert.Equal(t, domain.PracticePickDue, plan.Items[0].Reason)
		assert.Equal(t, 110, plan.Items[0].PlayAlong.StartTempoBPM)
	})

	t.Run("a failure reading item states fails the request", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.playAlong("d1", "Lick", practiceGuitar, "skill-1", 100)
		f.states.err = assert.AnError

		guitar := practiceGuitar
		_, err := f.svc.ComposePlan(ctx, studentCaller(), &guitar, 10)

		require.ErrorIs(t, err, assert.AnError)
	})
}
