package application_test

import (
	"context"
	"fmt"
	"slices"
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

// fakeTapCheckReader is an in-memory ports.TapCheckReader holding each
// student's newest tap check.
type fakeTapCheckReader struct {
	last  map[string]time.Time
	err   error
	reads int
}

func (f *fakeTapCheckReader) LastTapCheck(_ context.Context, studentID string) (time.Time, bool, error) {
	f.reads++
	if f.err != nil {
		return time.Time{}, false, f.err
	}
	at, ok := f.last[studentID]
	return at, ok, nil
}

// fakeFeltRatingReader is an in-memory ports.FeltRatingReader holding the
// felt-rated sessions of each drill template.
type fakeFeltRatingReader struct {
	sessions map[string]int
	asked    [][]string
	err      error
}

func (f *fakeFeltRatingReader) FeltRatedSessions(_ context.Context, templateKeys []string) (map[string]int, error) {
	f.asked = append(f.asked, templateKeys)
	if f.err != nil {
		return nil, f.err
	}
	counts := map[string]int{}
	for _, key := range templateKeys {
		if n, ok := f.sessions[key]; ok {
			counts[key] = n
		}
	}
	return counts, nil
}

const (
	practiceGuitar = "guitar"
	practiceBass   = "electric-bass"
)

type practiceFixture struct {
	svc            *application.PracticeSessionService
	instruments    *fakeInstrumentRepository
	studentPaths   *fakeStudentPathRepository
	enrollments    *fakeCourseEnrollmentRepository
	contentNodes   *fakeContentNodeRepository
	diagrams       *fakeDiagramRepository
	exercises      *fakeExerciseRepository
	knowledgeNodes *fakeKnowledgeNodeRepository
	edges          *fakeKnowledgeEdgeRepository
	states         *fakePracticeItemStateReader
	learningPaths  *fakeLearningPathRepository
	courseVersions *fakeCourseVersionRepository
	tapChecks      *fakeTapCheckReader
	feltRatings    *fakeFeltRatingReader
	// cells holds the fretboard cells that suit each instrument.
	cells     map[string][]domain.ClassifiedItem
	now       time.Time
	pathCount int
	t         *testing.T
}

// practiceItemSource is a ports.NodeItemSource over the fixture's diagrams,
// exercises and fretboard cells, classifying them as the Postgres source does.
type practiceItemSource struct{ f *practiceFixture }

func (s practiceItemSource) ClassifiedItems(ctx context.Context, instrumentID string) ([]domain.ClassifiedItem, error) {
	diagrams, err := s.f.diagrams.List(ctx, domain.DiagramListFilter{InstrumentID: instrumentID, Kind: domain.DiagramKindBasic}, domain.PageRequest{Limit: 1000})
	if err != nil {
		return nil, err
	}
	var items []domain.ClassifiedItem
	for _, d := range diagrams.Items {
		if _, ok := d.DefaultPlayback(); ok {
			items = append(items, domain.ClassifiedItem{ItemKey: domain.PlayAlongItemKey(d.ID), NodeIDs: d.SkillIDs()})
		}
	}
	exercises, err := s.f.exercises.List(ctx, domain.ExerciseFilter{InstrumentIDs: []string{instrumentID}}, domain.PageRequest{Limit: 1000})
	if err != nil {
		return nil, err
	}
	for _, e := range exercises.Items {
		items = append(items, domain.ClassifiedItem{ItemKey: domain.ExerciseItemKey(e.ID), NodeIDs: exerciseSkillIDs(e)})
	}
	return append(items, s.f.cells[instrumentID]...), nil
}

func newPracticeFixture(t *testing.T) *practiceFixture {
	f := &practiceFixture{
		instruments:    newFakeInstrumentRepository(),
		studentPaths:   newFakeStudentPathRepository(),
		enrollments:    newFakeCourseEnrollmentRepository(),
		contentNodes:   newFakeContentNodeRepository(),
		diagrams:       newFakeDiagramRepository(),
		exercises:      newFakeExerciseRepository(),
		knowledgeNodes: newFakeKnowledgeNodeRepository(),
		edges:          newFakeKnowledgeEdgeRepository(),
		states:         newFakePracticeItemStateReader(),
		learningPaths:  newFakeLearningPathRepository(),
		courseVersions: newFakeCourseVersionRepository(),
		tapChecks:      &fakeTapCheckReader{last: map[string]time.Time{}},
		feltRatings:    &fakeFeltRatingReader{sessions: map[string]int{}},
		cells:          map[string][]domain.ClassifiedItem{},
		now:            time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC),
		t:              t,
	}
	f.instruments.put(domain.Instrument{ID: practiceGuitar})
	f.instruments.put(domain.Instrument{ID: practiceBass})
	now := func() time.Time { return f.now }
	rollup := application.NewKnowledgeRollupService(f.knowledgeNodes, f.edges, practiceItemSource{f: f}, f.states, now)
	f.svc = application.NewPracticeSessionService(
		f.instruments, f.studentPaths, f.enrollments, f.learningPaths, f.courseVersions, f.contentNodes, f.diagrams, f.exercises, rollup, f.tapChecks, f.feltRatings,
		func() string { return "session-1" },
		now,
	)
	return f
}

// skill puts a skill for every instrument on the knowledge map, keyed by
// its id so catalog order is id order, unless it is already there.
func (f *practiceFixture) skill(id string) {
	if _, err := f.knowledgeNodes.GetByID(context.Background(), id); err == nil {
		return
	}
	f.knowledgeNodes.put(domain.KnowledgeNode{ID: id, Kind: domain.KnowledgeNodeKindSkill, Key: id})
}

// requires makes skill from require skill to at level.
func (f *practiceFixture) requires(from, to string, level domain.KnowledgeLevel) {
	f.skill(from)
	f.skill(to)
	l := domain.MasteryLevel(level)
	require.NoError(f.t, f.edges.Create(context.Background(), domain.KnowledgeEdge{ID: from + "->" + to, FromID: from, ToID: to, Type: domain.KnowledgeEdgeTypeRequires, Level: &l}))
}

// applies links the skill from to the node to with an applies edge.
func (f *practiceFixture) applies(from, to string) {
	f.skill(from)
	f.skill(to)
	require.NoError(f.t, f.edges.Create(context.Background(), domain.KnowledgeEdge{ID: from + "~>" + to, FromID: from, ToID: to, Type: domain.KnowledgeEdgeTypeApplies}))
}

// exercise puts an exercise taking seconds, classified under skillID, for
// instrumentIDs (none: every instrument).
func (f *practiceFixture) exercise(id, skillID string, seconds int, instrumentIDs ...string) {
	f.skill(skillID)
	require.NoError(f.t, f.exercises.Create(context.Background(), domain.Exercise{
		ID:                       id,
		Title:                    id,
		ExerciseType:             domain.ExerciseTypeTextResponse,
		Skills:                   []domain.KnowledgeNode{{ID: skillID}},
		EstimatedDurationSeconds: &seconds,
		InstrumentIDs:            instrumentIDs,
	}))
}

// exerciseBatch puts count exercises of seconds each under skillID for every
// instrument, ids prefix-00, prefix-01, ..., and returns their item keys.
func (f *practiceFixture) exerciseBatch(prefix, skillID string, count, seconds int) []string {
	keys := make([]string, count)
	for i := range count {
		id := fmt.Sprintf("%s-%02d", prefix, i)
		f.exercise(id, skillID, seconds)
		keys[i] = domain.ExerciseItemKey(id)
	}
	return keys
}

// stateOf puts the student's state on the item itemKey.
func (f *practiceFixture) stateOf(itemKey string, s domain.PracticeItemState) {
	s.ItemKey = itemKey
	if s.RulesVersion == 0 {
		s.RulesVersion = domain.PracticeRulesVersion
	}
	f.states.put(studentCaller().ID, s)
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

// onPathFor puts the student on a path teaching skillIDs whose template is
// for instrumentIDs (none: every instrument).
func (f *practiceFixture) onPathFor(instrumentIDs []string, skillIDs ...string) {
	f.onPath(skillIDs...)
	f.learningPaths.put(domain.LearningPath{ID: f.lastTemplateID(), InstrumentIDs: instrumentIDs})
}

func (f *practiceFixture) lastTemplateID() string {
	paths, err := f.studentPaths.ListActiveStandaloneByStudentID(context.Background(), studentCaller().ID)
	require.NoError(f.t, err)
	latest := paths[0]
	for _, p := range paths {
		if p.AssignedAt.After(latest.AssignedAt) {
			latest = p
		}
	}
	return latest.SourceTemplateID
}

// playAlong puts a diagram of eight quarter notes in 4/4 for instrumentID
// at tempo, classified under skillID. A tempo of 0 leaves it without
// playback.
func (f *practiceFixture) playAlong(id, name, instrumentID, skillID string, tempo int) {
	f.skill(skillID)
	d := domain.Diagram{
		ID:            id,
		InstrumentID:  instrumentID,
		InstrumentIDs: []string{instrumentID},
		Names:         names(name),
		Kind:          domain.DiagramKindBasic,
		Skills:        []domain.KnowledgeNode{{ID: skillID}},
	}
	if tempo > 0 {
		d.Playbacks = []domain.DiagramPlayback{{ID: "pb-default", TempoBPM: tempo, TimeSignature: domain.DefaultTimeSignature, Steps: quarterNotes(8)}}
		d.DefaultPlaybackID = strPtr("pb-default")
	}
	require.NoError(f.t, f.diagrams.Create(context.Background(), d))
}

// quarterNotes is n quarter-note steps on position p1.
func quarterNotes(n int) []domain.SequenceStep {
	steps := make([]domain.SequenceStep, n)
	for i := range steps {
		steps[i] = domain.SequenceStep{PositionIDs: []string{"p1"}, Value: domain.NoteValue{Num: 1, Den: 4}}
	}
	return steps
}

func (f *practiceFixture) state(diagramID string, s domain.PracticeItemState) {
	s.ItemKey = "play_along:" + diagramID
	if s.RulesVersion == 0 {
		s.RulesVersion = domain.PracticeRulesVersion
	}
	f.states.put(studentCaller().ID, s)
}

// stateUnder puts a state folded under the given mastery rules version.
func (f *practiceFixture) stateUnder(version int, diagramID string, s domain.PracticeItemState) {
	s.ItemKey = "play_along:" + diagramID
	s.RulesVersion = version
	f.states.put(studentCaller().ID, s)
}

// reshape replaces a seeded diagram's kind and, when steps > 0, its
// default playback's steps with that many quarter notes.
func (f *practiceFixture) reshape(id string, kind domain.DiagramKind, steps int) {
	d, err := f.diagrams.GetByID(context.Background(), id)
	require.NoError(f.t, err)
	d.Kind = kind
	if steps > 0 {
		d.Playbacks[0].Steps = quarterNotes(steps)
	}
	require.NoError(f.t, f.diagrams.Update(context.Background(), d))
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

	t.Run("a session in the head never picks a play-along, so one with only play-alongs is not found", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPathFor([]string{practiceGuitar}, "skill-1")
		f.playAlong("d1", "Lick", practiceGuitar, "skill-1", 120)

		_, err := f.svc.ComposePlan(ctx, studentCaller(), nil, 10)

		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a play-along with no clean take yet is new, starting at 60% of the diagram's tempo rounded down to 5 BPM", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1", "skill-2")
		f.playAlong("d1", "Pentatonic run", practiceGuitar, "skill-1", 120)
		// A due exercise on skill-2 makes skill-2's play-along the ending,
		// leaving d1 to the focus block, whose new share at 20 minutes fits it.
		f.exercise("due", "skill-2", 30)
		f.due("exercise:due")
		f.playAlong("ending", "Ending", practiceGuitar, "skill-2", 100)

		plan := f.compose(t, practiceGuitar, 20)

		assert.Equal(t, "session-1", plan.ID)
		require.NotNil(t, plan.InstrumentID)
		assert.Equal(t, practiceGuitar, *plan.InstrumentID)
		assert.Equal(t, 20, plan.Minutes)
		i := slices.Index(planKeys(plan), "play_along:d1")
		require.NotEqual(t, -1, i)
		item := plan.Items[i]
		assert.Equal(t, "play_along:d1", item.ItemKey)
		assert.Equal(t, domain.PracticeItemKindPlayAlong, item.Kind)
		assert.Equal(t, domain.PracticePickNew, item.Reason)
		assert.Equal(t, domain.KnowledgeLevelNew, item.Level)
		require.NotNil(t, item.NodeID)
		assert.Equal(t, "skill-1", *item.NodeID)
		require.NotNil(t, item.PlayAlong)
		assert.Equal(t, domain.PlannedPlayAlong{DiagramID: "d1", StartTempoBPM: 70, TargetTempoBPM: 120}, *item.PlayAlong)
	})

	t.Run("a play-along targets its default playback's tempo, whichever playback comes first", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.playAlong("d1", "Pentatonic run", practiceGuitar, "skill-1", 120)
		d, err := f.diagrams.GetByID(context.Background(), "d1")
		require.NoError(t, err)
		d.Playbacks = append([]domain.DiagramPlayback{{ID: "pb-fast", TempoBPM: 200, TimeSignature: domain.DefaultTimeSignature, Steps: quarterNotes(8)}}, d.Playbacks...)
		require.NoError(t, f.diagrams.Update(context.Background(), d))

		plan := f.compose(t, practiceGuitar, 20)

		i := slices.Index(planKeys(plan), "play_along:d1")
		require.NotEqual(t, -1, i)
		assert.Equal(t, domain.PlannedPlayAlong{DiagramID: "d1", StartTempoBPM: 70, TargetTempoBPM: 120}, *plan.Items[i].PlayAlong)
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

	t.Run("start tempos stay between 20 BPM and the target, or a best clean tempo past it", func(t *testing.T) {
		for _, tc := range []struct {
			name      string
			target    int
			bestClean *int
			want      int
		}{
			{name: "a best clean tempo past the target is kept", target: 100, bestClean: intPtr(130), want: 130},
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

		plan := f.compose(t, practiceGuitar, 9)

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

	t.Run("the warm-up comes first, then due items, most overdue first, then the rest", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.playAlong("a-new", "A new", practiceGuitar, "skill-1", 100)
		f.playAlong("b-due-today", "B due today", practiceGuitar, "skill-1", 100)
		f.playAlong("c-due-long-ago", "C due long ago", practiceGuitar, "skill-1", 100)
		f.playAlong("d-known", "D known", practiceGuitar, "skill-1", 100)
		f.state("b-due-today", domain.PracticeItemState{Level: domain.KnowledgeLevelLearning, Counted: 1, Box: 1, DueAt: f.daysAgo(0)})
		f.state("c-due-long-ago", domain.PracticeItemState{Level: domain.KnowledgeLevelLearning, Counted: 1, Box: 1, DueAt: f.daysAgo(1)})
		f.state("d-known", domain.PracticeItemState{Level: domain.KnowledgeLevelFluent, Counted: 6, Box: 4, DueAt: f.inDays(5), BestCleanBPM: intPtr(100)})

		plan := f.compose(t, practiceGuitar, 9)

		assert.Equal(t, []string{"play_along:d-known", "play_along:c-due-long-ago", "play_along:b-due-today", "play_along:a-new"}, planKeys(plan))
		reasons := []domain.PracticePickReason{}
		for _, item := range plan.Items {
			reasons = append(reasons, item.Reason)
		}
		assert.Equal(t, []domain.PracticePickReason{domain.PracticePickWarmUp, domain.PracticePickDue, domain.PracticePickDue, domain.PracticePickStretch}, reasons,
			"a-new is too long for the new share of 9 minutes, so it comes back as a stretch on the path's skill")
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
		f.exercise("standalone", "skill-standalone", 30)
		f.exercise("course", "skill-course", 30)
		f.exercise("archived", "skill-archived", 30)

		plan := f.compose(t, practiceGuitar, 9)

		reasons := map[string]domain.PracticePickReason{}
		for _, item := range plan.Items {
			reasons[item.ItemKey] = item.Reason
		}
		assert.Equal(t, domain.PracticePickNew, reasons["exercise:standalone"])
		assert.Equal(t, domain.PracticePickNew, reasons["exercise:course"])
		assert.NotEqual(t, domain.PracticePickNew, reasons["exercise:archived"], "an archived path's skill is only ever a stretch")
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

	t.Run("with nothing on the student's paths, nothing unconnected to their learning is stretched to", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.playAlong("elsewhere", "Elsewhere", practiceGuitar, "skill-other", 100)

		guitar := practiceGuitar
		_, err := f.svc.ComposePlan(context.Background(), studentCaller(), &guitar, 5)

		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("stretch takes play-alongs for the instrument in hand on a node a path skill applies", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.applies("skill-1", "skill-other")
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
		f.state("later", domain.PracticeItemState{Level: domain.KnowledgeLevelFluent, Counted: 6, Box: 4, DueAt: f.inDays(2)})
		f.state("sooner", domain.PracticeItemState{Level: domain.KnowledgeLevelFluent, Counted: 6, Box: 4, DueAt: f.inDays(1)})

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

	t.Run("a state folded under older mastery rules is due, keeping its best clean tempo", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.playAlong("old", "Old rules", practiceGuitar, "skill-1", 120)
		f.stateUnder(domain.PracticeRulesVersion-1, "old", domain.PracticeItemState{Level: domain.KnowledgeLevelFluent, Counted: 6, Box: 4, DueAt: f.inDays(5), BestCleanBPM: intPtr(110)})

		plan := f.compose(t, practiceGuitar, 3)

		require.Len(t, plan.Items, 1)
		assert.Equal(t, domain.PracticePickDue, plan.Items[0].Reason)
		assert.Equal(t, 110, plan.Items[0].PlayAlong.StartTempoBPM)
	})

	t.Run("a state folded under newer mastery rules than these is read as it is", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.playAlong("known", "Known", practiceGuitar, "skill-1", 100)
		f.playAlong("fresh", "Fresh", practiceGuitar, "skill-1", 100)
		f.stateUnder(domain.PracticeRulesVersion+1, "known", domain.PracticeItemState{Level: domain.KnowledgeLevelFluent, Counted: 6, Box: 4, DueAt: f.inDays(2)})

		plan := f.compose(t, practiceGuitar, 3)

		require.Contains(t, planKeys(plan), "play_along:known")
		assert.Equal(t, domain.PracticePickReviewAhead, plan.Items[slices.Index(planKeys(plan), "play_along:known")].Reason,
			"a known item not yet due is reviewed ahead, not due")
	})

	t.Run("only basic diagrams are offered, never a teacher's own custom ones", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.playAlong("basic", "Basic", practiceGuitar, "skill-1", 100)
		f.playAlong("custom-on-path", "Custom on path", practiceGuitar, "skill-1", 100)
		f.playAlong("custom-elsewhere", "Custom elsewhere", practiceGuitar, "skill-other", 100)
		f.reshape("custom-on-path", domain.DiagramKindCustom, 0)
		f.reshape("custom-elsewhere", domain.DiagramKindCustom, 0)
		f.state("basic", domain.PracticeItemState{Level: domain.KnowledgeLevelLearning, Counted: 1, Box: 2, DueAt: f.inDays(2)})

		plan := f.compose(t, practiceGuitar, 3)

		assert.Equal(t, []string{"play_along:basic"}, planKeys(plan), "neither the path nor the stretch offers a custom diagram")
	})

	t.Run("a warm-up taking more than a quarter of the session is skipped", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.playAlong("long", "Long known", practiceGuitar, "skill-1", 120)
		f.playAlong("run", "Pentatonic run", practiceGuitar, "skill-1", 120)
		f.reshape("long", domain.DiagramKindBasic, 200)
		f.state("long", domain.PracticeItemState{Level: domain.KnowledgeLevelAccurate, Counted: 4, Box: 3, DueAt: f.inDays(3), BestCleanBPM: intPtr(100)})

		plan := f.compose(t, practiceGuitar, 5)

		for _, item := range plan.Items {
			assert.NotEqual(t, domain.PracticePickWarmUp, item.Reason)
		}
		assert.Contains(t, planKeys(plan), "play_along:run")
	})

	t.Run("a due play-along keeps its review instead of becoming the warm-up", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.playAlong("due", "Due", practiceGuitar, "skill-1", 100)
		f.state("due", domain.PracticeItemState{Level: domain.KnowledgeLevelAccurate, Counted: 4, Box: 3, DueAt: f.daysAgo(0), BestCleanBPM: intPtr(100)})

		plan := f.compose(t, practiceGuitar, 10)

		require.Len(t, plan.Items, 1)
		assert.Equal(t, domain.PracticePickDue, plan.Items[0].Reason)
		assert.Equal(t, 100, plan.Items[0].PlayAlong.StartTempoBPM, "a due review starts at the best clean tempo, not a warm-up's 80%")
	})

	t.Run("a review-ahead item too long for its half leaves the time to stretch", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.playAlong("long-known", "Long known", practiceGuitar, "skill-1", 20)
		f.reshape("long-known", domain.DiagramKindBasic, 6)
		f.state("long-known", domain.PracticeItemState{Level: domain.KnowledgeLevelLearning, Counted: 1, Box: 2, DueAt: f.inDays(2)})
		f.applies("skill-1", "skill-other")
		f.playAlong("stretch-a", "Stretch A", practiceGuitar, "skill-other", 100)
		f.playAlong("stretch-b", "Stretch B", practiceGuitar, "skill-other", 100)

		plan := f.compose(t, practiceGuitar, 3)

		assert.Equal(t, []string{"play_along:stretch-a", "play_along:stretch-b"}, planKeys(plan))
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

// secondsByReason sums the estimated seconds of a plan's items per reason.
func secondsByReason(plan domain.PracticeSessionPlan) map[domain.PracticePickReason]int {
	seconds := map[domain.PracticePickReason]int{}
	for _, item := range plan.Items {
		seconds[item.Reason] += item.EstimatedSeconds
	}
	return seconds
}

func planReasons(plan domain.PracticeSessionPlan) []domain.PracticePickReason {
	reasons := make([]domain.PracticePickReason, len(plan.Items))
	for i, item := range plan.Items {
		reasons[i] = item.Reason
	}
	return reasons
}

// due, weak and known give the student a state on each of keys: due
// today; accurate and due in two days; fluent and due in dueInDays.
func (f *practiceFixture) due(keys ...string) {
	for _, k := range keys {
		f.stateOf(k, domain.PracticeItemState{Level: domain.KnowledgeLevelLearning, Counted: 2, Box: 1, DueAt: f.daysAgo(0)})
	}
}

func (f *practiceFixture) weak(keys ...string) {
	for _, k := range keys {
		f.stateOf(k, domain.PracticeItemState{Level: domain.KnowledgeLevelAccurate, Counted: 3, Box: 2, DueAt: f.inDays(2)})
	}
}

func (f *practiceFixture) known(dueInDays int, keys ...string) {
	for _, k := range keys {
		f.stateOf(k, domain.PracticeItemState{Level: domain.KnowledgeLevelFluent, Counted: 6, Box: 4, DueAt: f.inDays(dueInDays)})
	}
}

func TestPracticeSessionService_ComposePlanMix(t *testing.T) {
	// Every item below is a 30-second exercise, with no play-along, so a
	// session has no warm-up and no application ending: the focus time is
	// the whole session.
	t.Run("an item practised, not due and below fluent is picked as weak; a fluent one never is", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.exercise("accurate", "skill-1", 30)
		f.exercise("fluent", "skill-1", 30)
		f.weak("exercise:accurate")
		f.known(5, "exercise:fluent")

		plan := f.compose(t, practiceGuitar, 20)

		reasons := map[string]domain.PracticePickReason{}
		for _, item := range plan.Items {
			reasons[item.ItemKey] = item.Reason
		}
		assert.Equal(t, domain.PracticePickWeak, reasons["exercise:accurate"])
		assert.NotEqual(t, domain.PracticePickWeak, reasons["exercise:fluent"])
	})

	t.Run("due, weak and new items share the focus time 60, 25 and 15", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.due(f.exerciseBatch("due", "skill-1", 40, 30)...)
		f.weak(f.exerciseBatch("weak", "skill-1", 40, 30)...)
		f.exerciseBatch("new", "skill-1", 40, 30)

		plan := f.compose(t, practiceGuitar, 20)

		assert.Equal(t, map[domain.PracticePickReason]int{
			domain.PracticePickDue:  720,
			domain.PracticePickWeak: 300,
			domain.PracticePickNew:  180,
		}, secondsByReason(plan))
		assert.Equal(t, []domain.PracticePickReason{domain.PracticePickDue, domain.PracticePickWeak, domain.PracticePickNew},
			slices.Compact(planReasons(plan)), "due items come first, then weak, then new")
	})

	t.Run("due items take over the time weak items don't use", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.due(f.exerciseBatch("due", "skill-1", 40, 30)...)
		f.exerciseBatch("new", "skill-1", 40, 30)

		plan := f.compose(t, practiceGuitar, 20)

		assert.Equal(t, map[domain.PracticePickReason]int{domain.PracticePickDue: 1020, domain.PracticePickNew: 180}, secondsByReason(plan))
	})

	t.Run("weak items take over the time due items don't use", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.weak(f.exerciseBatch("weak", "skill-1", 40, 30)...)
		f.exerciseBatch("new", "skill-1", 40, 30)

		plan := f.compose(t, practiceGuitar, 20)

		assert.Equal(t, map[domain.PracticePickReason]int{domain.PracticePickWeak: 1020, domain.PracticePickNew: 180}, secondsByReason(plan))
	})

	t.Run("the new share is a ceiling, never filled past it", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.due(f.exerciseBatch("due", "skill-1", 2, 30)...)
		f.exerciseBatch("new", "skill-1", 200, 30)

		plan := f.compose(t, practiceGuitar, 20)

		seconds := secondsByReason(plan)
		assert.Equal(t, 60, seconds[domain.PracticePickDue])
		assert.Equal(t, 180, seconds[domain.PracticePickNew])
	})

	t.Run("time left after due, weak and new is split evenly between review ahead and stretch", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.due(f.exerciseBatch("due", "skill-1", 8, 30)...)
		f.exerciseBatch("new", "skill-1", 2, 30)
		f.known(3, f.exerciseBatch("known", "skill-1", 40, 30)...)
		f.applies("skill-1", "skill-ready")
		f.exerciseBatch("stretch", "skill-ready", 40, 30)

		plan := f.compose(t, practiceGuitar, 20)

		assert.Equal(t, map[domain.PracticePickReason]int{
			domain.PracticePickDue:         240,
			domain.PracticePickNew:         60,
			domain.PracticePickReviewAhead: 450,
			domain.PracticePickStretch:     450,
		}, secondsByReason(plan))
	})

	t.Run("a caught-up student with nothing coming due stretches for the whole session", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.applies("skill-1", "skill-ready")
		f.exerciseBatch("stretch", "skill-ready", 40, 30)

		plan := f.compose(t, practiceGuitar, 10)

		assert.Equal(t, map[domain.PracticePickReason]int{domain.PracticePickStretch: 600}, secondsByReason(plan))
		for _, item := range plan.Items {
			require.NotNil(t, item.NodeID)
			assert.Equal(t, "skill-ready", *item.NodeID, "a stretch pick names the node it starts")
		}
	})

	t.Run("review ahead looks a week ahead: a known item due later is left to stretch", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.known(7, f.exerciseBatch("week", "skill-1", 1, 30)...)
		f.known(8, f.exerciseBatch("later", "skill-1", 1, 30)...)
		f.applies("skill-1", "skill-ready")
		f.exerciseBatch("stretch", "skill-ready", 40, 30)

		plan := f.compose(t, practiceGuitar, 10)

		assert.Contains(t, planKeys(plan), "exercise:week-00")
		assert.NotContains(t, planKeys(plan), "exercise:later-00")
		assert.Equal(t, map[domain.PracticePickReason]int{domain.PracticePickReviewAhead: 30, domain.PracticePickStretch: 570}, secondsByReason(plan))
	})

	t.Run("stretch starts with nodes that build on what the student meets, then the shallowest, then catalog order", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.exercise("met", "skill-1", 30)
		f.known(5, "exercise:met")
		f.requires("skill-b-deep", "skill-c-shallow", domain.KnowledgeLevelAccurate)
		f.requires("skill-c-shallow", "skill-1", domain.KnowledgeLevelAccurate)
		f.requires("skill-d-unmet", "skill-unmet", domain.KnowledgeLevelAccurate)
		f.applies("skill-1", "skill-a-free")
		f.exerciseBatch("a", "skill-a-free", 1, 30)
		f.exerciseBatch("b", "skill-b-deep", 1, 30)
		// Four of skill-c-shallow's five items are fluent, so the node is
		// fluent and skill-b-deep's requirement is met; one is still unseen.
		f.known(5, f.exerciseBatch("c", "skill-c-shallow", 5, 30)[:4]...)
		f.exerciseBatch("d", "skill-d-unmet", 1, 30)
		f.exerciseBatch("u", "skill-unmet", 1, 30)

		plan := f.compose(t, practiceGuitar, 10)

		var stretch []string
		for _, item := range plan.Items {
			if item.Reason == domain.PracticePickStretch {
				stretch = append(stretch, *item.NodeID)
			}
		}
		assert.Equal(t, []string{"skill-c-shallow", "skill-b-deep", "skill-a-free"}, stretch,
			"skill-d-unmet's requirement isn't met, and skill-unmet has no link to the path, so neither is a stretch")
	})

	t.Run("past the new share, a ready path skill's unseen items come back as stretch before any other node's", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-z-path")
		f.exerciseBatch("path", "skill-z-path", 40, 30)
		f.exerciseBatch("other", "skill-a-other", 40, 30)

		plan := f.compose(t, practiceGuitar, 10)

		assert.Equal(t, map[domain.PracticePickReason]int{domain.PracticePickNew: 90, domain.PracticePickStretch: 510}, secondsByReason(plan))
		for _, item := range plan.Items {
			require.NotNil(t, item.NodeID)
			assert.Equal(t, "skill-z-path", *item.NodeID, item.ItemKey)
		}
	})

	t.Run("an unmet requirement never keeps a skill on the student's path out of practice", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-all-strings")
		f.requires("skill-all-strings", "skill-low-strings", domain.KnowledgeLevelAccurate)
		f.exerciseBatch("all", "skill-all-strings", 2, 30)

		plan := f.compose(t, practiceGuitar, 10)

		assert.Equal(t, []string{"exercise:all-00", "exercise:all-01"}, planKeys(plan)[:2])
		assert.Equal(t, domain.PracticePickNew, plan.Items[0].Reason)
	})

	t.Run("an exercise item carries the exercise, sized by its estimate", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.exercise("name-the-third", "skill-1", 25)
		f.exercise("bass-only", "skill-1", 25, practiceBass)

		plan := f.compose(t, practiceGuitar, 4)

		require.Equal(t, []string{"exercise:name-the-third"}, planKeys(plan), "an exercise for another instrument is never picked")
		item := plan.Items[0]
		assert.Equal(t, domain.PracticeItemKindExercise, item.Kind)
		assert.Equal(t, domain.PracticePickNew, item.Reason)
		assert.Equal(t, 25, item.EstimatedSeconds)
		assert.Equal(t, domain.KnowledgeLevelNew, item.Level)
		require.NotNil(t, item.NodeID)
		assert.Equal(t, "skill-1", *item.NodeID)
		require.NotNil(t, item.Exercise)
		assert.Equal(t, "name-the-third", item.Exercise.ID)
		assert.Nil(t, item.PlayAlong)
	})
}

func TestPracticeSessionService_ComposePlanApplication(t *testing.T) {
	t.Run("a session of 10 minutes or more ends with a play-along applying a skill its focus items practise", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-other", "chord-tones")
		f.playAlong("other-riff", "Other riff", practiceGuitar, "skill-other", 100)
		f.playAlong("chord-tone-riff", "Chord-tone riff", practiceGuitar, "chord-tones", 100)
		f.due(f.exerciseBatch("third", "chord-tones", 4, 30)...)

		plan := f.compose(t, practiceGuitar, 10)

		last := plan.Items[len(plan.Items)-1]
		assert.Equal(t, "play_along:chord-tone-riff", last.ItemKey)
		assert.Equal(t, domain.PracticePickApplication, last.Reason)
		assert.Equal(t, 60, last.PlayAlong.StartTempoBPM, "the ending is played on the tempo ladder")
		count := 0
		for _, item := range plan.Items {
			if item.Reason == domain.PracticePickApplication {
				count++
			}
		}
		assert.Equal(t, 1, count)
	})

	t.Run("the application ending is taken out of the focus time", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("chord-tones")
		f.playAlong("chord-tone-riff", "Chord-tone riff", practiceGuitar, "chord-tones", 100)
		f.due(f.exerciseBatch("due", "chord-tones", 40, 30)...)

		plan := f.compose(t, practiceGuitar, 20)

		seconds := secondsByReason(plan)
		ending := seconds[domain.PracticePickApplication]
		require.Positive(t, ending)
		focus := 20*60 - ending
		assert.Equal(t, (focus*60/100+focus*25/100)/30*30, seconds[domain.PracticePickDue],
			"due takes its share and weak's of the focus time: the session less the ending")
	})

	t.Run("with no play-along on a focus skill, the ending applies another skill of the path", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1", "chord-tones")
		f.due(f.exerciseBatch("due", "skill-1", 4, 30)...)
		f.playAlong("chord-tone-riff", "Chord-tone riff", practiceGuitar, "chord-tones", 100)

		plan := f.compose(t, practiceGuitar, 10)

		last := plan.Items[len(plan.Items)-1]
		assert.Equal(t, "play_along:chord-tone-riff", last.ItemKey)
		assert.Equal(t, domain.PracticePickApplication, last.Reason)
	})

	t.Run("a due play-along keeps its review rather than becoming the ending", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.playAlong("due-riff", "Due riff", practiceGuitar, "skill-1", 100)
		f.due("play_along:due-riff")

		plan := f.compose(t, practiceGuitar, 10)

		assert.Equal(t, []domain.PracticePickReason{domain.PracticePickDue}, planReasons(plan))
	})

	t.Run("an ending taking more than a quarter of the session is skipped", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPath("skill-1")
		f.playAlong("long-riff", "Long riff", practiceGuitar, "skill-1", 100)
		f.reshape("long-riff", domain.DiagramKindBasic, 200)
		f.due(f.exerciseBatch("due", "skill-1", 40, 30)...)

		plan := f.compose(t, practiceGuitar, 10)

		assert.NotContains(t, planReasons(plan), domain.PracticePickApplication)
	})

	t.Run("with no play-along on the instrument at all, or a session under 10 minutes, there is no ending", func(t *testing.T) {
		for _, tc := range []struct {
			name       string
			instrument string
			minutes    int
		}{
			{name: "no play-along on the bass", instrument: practiceBass, minutes: 10},
			{name: "a 9-minute session", instrument: practiceGuitar, minutes: 9},
		} {
			f := newPracticeFixture(t)
			f.onPath("skill-1")
			f.playAlong("guitar-riff", "Guitar riff", practiceGuitar, "skill-1", 100)
			f.due(f.exerciseBatch("due", "skill-1", 40, 30)...)

			plan := f.compose(t, tc.instrument, tc.minutes)

			assert.NotContains(t, planReasons(plan), domain.PracticePickApplication, tc.name)
		}
	})
}

// cellsOn puts the fretboard cells of strs at frets 0 to frets-1 on
// instrumentID's own layout, classified under skillID, and returns their
// item keys.
func (f *practiceFixture) cellsOn(instrumentID, skillID string, frets int, strs ...int) []string {
	f.skill(skillID)
	var keys []string
	for _, str := range strs {
		for fret := range frets {
			key := domain.FretboardCellItemKey(instrumentID, str, fret)
			f.cells[instrumentID] = append(f.cells[instrumentID], domain.ClassifiedItem{ItemKey: key, NodeIDs: []string{skillID}})
			keys = append(keys, key)
		}
	}
	return keys
}

func (f *practiceFixture) composeInTheHead(t *testing.T, minutes int) domain.PracticeSessionPlan {
	t.Helper()
	plan, err := f.svc.ComposePlan(context.Background(), studentCaller(), nil, minutes)
	require.NoError(t, err)
	return plan
}

// itemsOfKind lists the plan's items of kind.
func itemsOfKind(plan domain.PracticeSessionPlan, kind domain.PracticeItemKind) []domain.PracticeSessionItem {
	var items []domain.PracticeSessionItem
	for _, item := range plan.Items {
		if item.Kind == kind {
			items = append(items, item)
		}
	}
	return items
}

func TestPracticeSessionService_ComposePlanFretboardCells(t *testing.T) {
	ctx := context.Background()

	t.Run("in the head: cells of every layout among the student's instruments and exercises, no play-along", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPathFor([]string{practiceGuitar, practiceBass}, "root-strings")
		f.cellsOn(practiceGuitar, "root-strings", 2, 6)
		f.cellsOn(practiceBass, "root-strings", 2, 4)
		f.exercise("name-the-root", "root-strings", 30)
		f.playAlong("lick", "Lick", practiceGuitar, "root-strings", 100)

		plan := f.composeInTheHead(t, 5)

		assert.Nil(t, plan.InstrumentID)
		assert.Empty(t, itemsOfKind(plan, domain.PracticeItemKindPlayAlong))
		layouts := map[string]bool{}
		for _, item := range itemsOfKind(plan, domain.PracticeItemKindFretboardCell) {
			require.NotNil(t, item.FretboardCell)
			layouts[item.FretboardCell.LayoutInstrumentID] = true
		}
		assert.Equal(t, map[string]bool{practiceGuitar: true, practiceBass: true}, layouts)
		assert.Contains(t, planKeys(plan), domain.ExerciseItemKey("name-the-root"))
	})

	t.Run("in the head: no warm-up and no application ending, even with a clean play-along", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPathFor([]string{practiceGuitar}, "root-strings")
		f.cellsOn(practiceGuitar, "root-strings", 12, 6, 5)
		f.playAlong("lick", "Lick", practiceGuitar, "root-strings", 100)
		f.playAlong("riff", "Riff", practiceGuitar, "root-strings", 100)
		clean := 100
		f.state("lick", domain.PracticeItemState{Level: domain.KnowledgeLevelFluent, Counted: 6, Box: 4, DueAt: f.inDays(5), LastAt: f.daysAgo(1), BestCleanBPM: &clean})

		plan := f.composeInTheHead(t, 15)

		for _, item := range plan.Items {
			assert.NotEqual(t, domain.PracticePickWarmUp, item.Reason, item.ItemKey)
			assert.NotEqual(t, domain.PracticePickApplication, item.Reason, item.ItemKey)
		}
	})

	t.Run("a cell is asked the way it has fewer right answers, and takes 8 seconds", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPathFor([]string{practiceGuitar}, "root-strings")
		keys := f.cellsOn(practiceGuitar, "root-strings", 1, 6, 5)
		f.stateOf(keys[0], domain.PracticeItemState{
			Level: domain.KnowledgeLevelLearning, Counted: 5, Box: 1, DueAt: f.daysAgo(1), LastAt: f.daysAgo(2),
			RightByResponse: map[string]int{"name_the_note": 4, "find_the_note": 1},
		})

		plan := f.composeInTheHead(t, 5)

		cells := map[string]domain.PracticeSessionItem{}
		for _, item := range itemsOfKind(plan, domain.PracticeItemKindFretboardCell) {
			cells[item.ItemKey] = item
		}
		require.Contains(t, cells, keys[0])
		require.Contains(t, cells, keys[1])
		assert.Equal(t, domain.PlannedFretboardCell{
			FretboardCell: domain.FretboardCell{LayoutInstrumentID: practiceGuitar, String: 6, Fret: 0},
			Drill:         domain.FretboardDrillFindTheNote,
		}, *cells[keys[0]].FretboardCell)
		assert.Equal(t, domain.FretboardDrillNameTheNote, cells[keys[1]].FretboardCell.Drill)
		for _, item := range cells {
			assert.Equal(t, domain.FretboardCellSeconds, item.EstimatedSeconds)
		}
	})

	t.Run("with an instrument in hand, cells of its layout are picked like any focus item", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPathFor([]string{practiceGuitar}, "root-strings")
		guitarCells := f.cellsOn(practiceGuitar, "root-strings", 12, 6, 5)
		f.cellsOn(practiceBass, "root-strings", 12, 4, 3)

		plan := f.compose(t, practiceGuitar, 10)

		cells := itemsOfKind(plan, domain.PracticeItemKindFretboardCell)
		require.NotEmpty(t, cells)
		assert.Equal(t, domain.PracticePickNew, cells[0].Reason)
		for _, item := range cells {
			assert.Contains(t, guitarCells, item.ItemKey)
			assert.Contains(t, []domain.PracticePickReason{domain.PracticePickNew, domain.PracticePickStretch}, item.Reason)
		}
	})

	t.Run("new items are balanced across the student's instruments", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPathFor([]string{practiceGuitar, practiceBass}, "root-strings")
		f.cellsOn(practiceGuitar, "root-strings", 10, 6, 5, 4, 3)
		f.cellsOn(practiceBass, "root-strings", 10, 4, 3, 2, 1)

		plan := f.composeInTheHead(t, 10)

		count := map[string]int{}
		for _, item := range plan.Items {
			if item.Reason == domain.PracticePickNew {
				count[item.FretboardCell.LayoutInstrumentID]++
			}
		}
		require.NotZero(t, count[practiceGuitar])
		assert.InDelta(t, count[practiceGuitar], count[practiceBass], 1, "%v", count)
	})

	t.Run("past the new share, stretch is balanced across the student's instruments too", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPathFor([]string{practiceGuitar, practiceBass}, "root-strings")
		f.cellsOn(practiceGuitar, "root-strings", 12, 6, 5, 4, 3)
		f.cellsOn(practiceBass, "root-strings", 12, 4, 3, 2, 1)

		plan := f.composeInTheHead(t, 10)

		count := map[string]int{}
		for _, item := range plan.Items {
			if item.Reason == domain.PracticePickStretch {
				count[item.FretboardCell.LayoutInstrumentID]++
			}
		}
		require.NotZero(t, count[practiceGuitar])
		assert.InDelta(t, count[practiceGuitar], count[practiceBass], 1, "%v", count)
	})

	t.Run("in the head, a student with no history learning the fretboard gets a new cell", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPathFor([]string{practiceGuitar}, "root-strings")
		f.cellsOn(practiceGuitar, "root-strings", 12, 6, 5)

		plan := f.composeInTheHead(t, 5)

		cells := itemsOfKind(plan, domain.PracticeItemKindFretboardCell)
		require.NotEmpty(t, cells)
		assert.Equal(t, domain.PracticePickNew, cells[0].Reason)
	})

	t.Run("in the head, a student learning nothing gets no session", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.cellsOn(practiceGuitar, "root-strings", 12, 6, 5)

		_, err := f.svc.ComposePlan(ctx, studentCaller(), nil, 5)

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestPracticeSessionService_ComposePlanTapCheck(t *testing.T) {
	headWithCells := func(t *testing.T) *practiceFixture {
		f := newPracticeFixture(t)
		f.onPathFor([]string{practiceGuitar}, "root-strings")
		f.cellsOn(practiceGuitar, "root-strings", 12, 6, 5)
		return f
	}

	t.Run("a plan with fretboard cells asks for a tap check when the student never did one", func(t *testing.T) {
		f := headWithCells(t)

		plan := f.composeInTheHead(t, 5)

		require.NotEmpty(t, itemsOfKind(plan, domain.PracticeItemKindFretboardCell))
		assert.True(t, plan.TapCheckDue)
	})

	t.Run("a tap check within the last 30 days isn't asked for again", func(t *testing.T) {
		f := headWithCells(t)
		f.tapChecks.last[studentCaller().ID] = f.now.AddDate(0, 0, -12)

		assert.False(t, f.composeInTheHead(t, 5).TapCheckDue)
	})

	t.Run("a tap check older than 30 days is asked for again", func(t *testing.T) {
		f := headWithCells(t)
		f.tapChecks.last[studentCaller().ID] = f.now.AddDate(0, 0, -31)

		assert.True(t, f.composeInTheHead(t, 5).TapCheckDue)
	})

	t.Run("a plan without fretboard cells never asks, nor reads the student's tap checks", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPathFor([]string{practiceGuitar}, "root-strings")
		f.exercise("name-the-root", "root-strings", 30)
		f.playAlong("lick", "Lick", practiceGuitar, "root-strings", 100)

		plan := f.compose(t, practiceGuitar, 10)

		assert.Empty(t, itemsOfKind(plan, domain.PracticeItemKindFretboardCell))
		assert.False(t, plan.TapCheckDue)
		assert.Zero(t, f.tapChecks.reads)
	})

	t.Run("a failure reading tap checks fails the plan", func(t *testing.T) {
		f := headWithCells(t)
		boom := fmt.Errorf("mongo down")
		f.tapChecks.err = boom

		_, err := f.svc.ComposePlan(context.Background(), studentCaller(), nil, 5)

		assert.ErrorIs(t, err, boom)
	})
}

func TestPracticeSessionService_ComposePlanFeltQuestions(t *testing.T) {
	t.Run("the plan asks about its two least felt-rated timed drills, fewest first", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPathFor([]string{practiceGuitar}, "root-strings")
		f.cellsOn(practiceGuitar, "root-strings", 12, 6, 5)
		f.exercise("name-the-root", "root-strings", 30)
		f.feltRatings.sessions = map[string]int{"fretboard_cell:name_the_note": 50, "exercise:text_response": 4}

		plan := f.composeInTheHead(t, 10)

		require.NotEmpty(t, itemsOfKind(plan, domain.PracticeItemKindExercise))
		assert.Equal(t, domain.FeltQuestions(plan.Items, f.feltRatings.sessions), plan.FeltQuestions)
		assert.Equal(t, []string{"exercise:text_response", "fretboard_cell:name_the_note"}, plan.FeltQuestions)
		assert.Equal(t, [][]string{domain.PlanDrillTemplates(plan.Items)}, f.feltRatings.asked)
	})

	t.Run("a plan of play-alongs only asks nothing, nor reads felt ratings", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPathFor([]string{practiceGuitar}, "root-strings")
		f.playAlong("lick", "Lick", practiceGuitar, "root-strings", 100)
		f.playAlong("riff", "Riff", practiceGuitar, "root-strings", 100)

		plan := f.compose(t, practiceGuitar, 10)

		require.NotEmpty(t, plan.Items)
		assert.Empty(t, plan.FeltQuestions)
		assert.NotNil(t, plan.FeltQuestions)
		assert.Empty(t, f.feltRatings.asked)
	})

	t.Run("a failure reading felt ratings fails the plan", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.onPathFor([]string{practiceGuitar}, "root-strings")
		f.cellsOn(practiceGuitar, "root-strings", 12, 6, 5)
		boom := fmt.Errorf("mongo down")
		f.feltRatings.err = boom

		_, err := f.svc.ComposePlan(context.Background(), studentCaller(), nil, 5)

		assert.ErrorIs(t, err, boom)
	})
}
