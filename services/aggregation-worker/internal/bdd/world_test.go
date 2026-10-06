//go:build integration

package bdd

import (
	"context"
	"crypto/sha1" //nolint:gosec // used only for deterministic test UUIDs, not security
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/motifpath/aggregation-worker/internal/application"
	"github.com/motifpath/aggregation-worker/internal/domain"
)

// fakeReference stands in for core's practice_reference snapshot; the Mongo reader
// is covered by the repo package's integration tests.
type fakeReference struct {
	diagrams    map[string]domain.DiagramReference
	exercises   map[string]domain.ExerciseReference
	instruments map[string]domain.InstrumentReference
	fluentTimes map[string][]domain.FluentTime
}

func (f *fakeReference) Instruments(_ context.Context, ids []string) (map[string]domain.InstrumentReference, error) {
	found := map[string]domain.InstrumentReference{}
	for _, id := range ids {
		if i, ok := f.instruments[id]; ok {
			found[id] = i
		}
	}
	return found, nil
}

func (f *fakeReference) Exercises(_ context.Context, ids []string) (map[string]domain.ExerciseReference, error) {
	found := map[string]domain.ExerciseReference{}
	for _, id := range ids {
		if e, ok := f.exercises[id]; ok {
			found[id] = e
		}
	}
	return found, nil
}

func (f *fakeReference) FluentTimes(_ context.Context, templateKey string) ([]domain.FluentTime, error) {
	return f.fluentTimes[templateKey], nil
}

func (f *fakeReference) Diagrams(_ context.Context, ids []string) (map[string]domain.DiagramReference, error) {
	found := map[string]domain.DiagramReference{}
	for _, id := range ids {
		if d, ok := f.diagrams[id]; ok {
			found[id] = d
		}
	}
	return found, nil
}

// fakeEvidence and fakeStates keep the repositories' contracts (one evidence per
// id, stored order, one state per item); the Mongo adapters are covered by the
// repo package's integration tests.
type fakeEvidence struct {
	stored []domain.PracticeEvidence
}

func (f *fakeEvidence) Insert(_ context.Context, e domain.PracticeEvidence) (bool, error) {
	if slices.ContainsFunc(f.stored, func(s domain.PracticeEvidence) bool { return s.EvidenceID == e.EvidenceID }) {
		return false, nil
	}
	f.stored = append(f.stored, e)
	return true, nil
}

func (f *fakeEvidence) ListForItem(_ context.Context, studentID, itemKey string) ([]domain.PracticeEvidence, error) {
	var out []domain.PracticeEvidence
	for _, s := range f.stored {
		if s.StudentID == studentID && s.ItemKey == itemKey {
			out = append(out, s)
		}
	}
	return out, nil
}

// fakeHistory keeps one snapshot per item and day, like the Mongo adapter.
type fakeHistory struct {
	snapshots map[string]domain.ItemSnapshot
}

func (f *fakeHistory) Put(_ context.Context, studentID, itemKey string, snapshots []domain.ItemSnapshot) error {
	for _, s := range snapshots {
		f.snapshots[studentID+"|"+itemKey+"|"+s.Day.Format(time.DateOnly)] = s
	}
	return nil
}

// fakeSessions, fakeLearning and fakeCompletion keep their repositories'
// contracts (one session per id, one completion per event id, one status per
// node); the Mongo adapters are covered by the repo package's integration tests.
type fakeSessions struct {
	sessions map[string]domain.PracticeSession
}

func (f *fakeSessions) Get(_ context.Context, studentID, sessionID string) (domain.PracticeSession, bool, error) {
	s, ok := f.sessions[studentID+"|"+sessionID]
	return s, ok, nil
}

func (f *fakeSessions) Put(_ context.Context, s domain.PracticeSession) error {
	f.sessions[s.StudentID+"|"+s.ID] = s
	return nil
}

type fakeLearning struct {
	stored []domain.LearningActivity
}

func (f *fakeLearning) Insert(_ context.Context, a domain.LearningActivity) (bool, error) {
	if slices.ContainsFunc(f.stored, func(s domain.LearningActivity) bool { return s.EventID == a.EventID }) {
		return false, nil
	}
	f.stored = append(f.stored, a)
	return true, nil
}

type fakeCompletion struct {
	statuses map[string]domain.CompletionStatus
}

func (f *fakeCompletion) GetStatus(_ context.Context, studentID, contentNodeID string) (domain.CompletionStatus, bool, error) {
	s, ok := f.statuses[studentID+"|"+contentNodeID]
	return s, ok, nil
}

func (f *fakeCompletion) Upsert(_ context.Context, studentID, contentNodeID string, status domain.CompletionStatus) error {
	f.statuses[studentID+"|"+contentNodeID] = status
	return nil
}

type fakeStates struct {
	folds map[string]domain.ItemFold
}

func (f *fakeStates) Get(_ context.Context, studentID, itemKey string) (domain.ItemFold, int, bool, error) {
	fold, ok := f.folds[studentID+"|"+itemKey]
	return fold, domain.PracticeRulesVersion, ok, nil
}

func (f *fakeStates) Put(_ context.Context, studentID, itemKey string, fold domain.ItemFold) error {
	f.folds[studentID+"|"+itemKey] = fold
	return nil
}

// logRecorder keeps the attributes of every log record, so a scenario can see why
// an answer was rejected: the processor logs a rejection and stores nothing.
type logRecorder struct {
	mu      sync.Mutex
	records []map[string]string
}

func (h *logRecorder) Enabled(context.Context, slog.Level) bool { return true }

func (h *logRecorder) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	rec := map[string]string{"msg": r.Message}
	r.Attrs(func(a slog.Attr) bool {
		rec[a.Key] = a.Value.String()
		return true
	})
	h.records = append(h.records, rec)
	return nil
}

func (h *logRecorder) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &scopedRecorder{root: h, attrs: attrs}
}

func (h *logRecorder) WithGroup(string) slog.Handler { return h }

// scopedRecorder is a logger.With(...) child that writes into its root's records.
type scopedRecorder struct {
	root  *logRecorder
	attrs []slog.Attr
}

func (s *scopedRecorder) Enabled(context.Context, slog.Level) bool { return true }

func (s *scopedRecorder) Handle(ctx context.Context, r slog.Record) error {
	r = r.Clone()
	r.AddAttrs(s.attrs...)
	return s.root.Handle(ctx, r)
}

func (s *scopedRecorder) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &scopedRecorder{root: s.root, attrs: append(slices.Clone(s.attrs), attrs...)}
}

func (s *scopedRecorder) WithGroup(string) slog.Handler { return s }

func (h *logRecorder) rejectionReasons() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var reasons []string
	for _, r := range h.records {
		if reason, ok := r["reason"]; ok {
			reasons = append(reasons, reason)
		}
	}
	return reasons
}

// playAlongTargetBPM is the tempo every scenario diagram is written at.
const playAlongTargetBPM = 120

type world struct {
	reference *fakeReference
	evidence  *fakeEvidence
	states    *fakeStates
	history   *fakeHistory
	logs      *logRecorder
	service   *application.PracticeEvidenceService

	// sessionRecords and learning are the raw activity; events reaches every
	// event's handler, the way the Kafka consumer does.
	sessionRecords *fakeSessions
	learning       *fakeLearning
	events         *application.ProcessEventService
	activity       activityWorld

	students map[string]string
	sessions map[string]string
	// challenges holds the challenge a student is taking: their answers go to it
	// instead of a practice session.
	challenges map[string]*domain.TriggerContext
	// tapMs is each student's tap time, stamped on their timed answers the way
	// ingestion stamps it.
	tapMs map[string]int
	// clock is when the next answer is given; each answer moves it on a minute.
	clock   time.Time
	eventNo int

	// lastStudent and lastItemKey are who gave the latest answer and what it was
	// about; before is that item's state just before the answer.
	lastStudent string
	lastItemKey string
	before      domain.ItemFold

	// lastTemplate is the drill template the latest fluent time step named.
	lastTemplate string

	// lastAnswer is the latest answer sent.
	lastAnswer domain.PracticeAnswer
	// lastExercise and lastExerciseID name the exercise a scenario set up last.
	lastExercise   string
	lastExerciseID string
	cells          cellWorld
}

func newWorld() *world {
	w := &world{
		reference: &fakeReference{
			diagrams:    map[string]domain.DiagramReference{},
			exercises:   map[string]domain.ExerciseReference{},
			instruments: map[string]domain.InstrumentReference{},
			fluentTimes: map[string][]domain.FluentTime{},
		},
		evidence: &fakeEvidence{},
		states:   &fakeStates{folds: map[string]domain.ItemFold{}},
		history:  &fakeHistory{snapshots: map[string]domain.ItemSnapshot{}},
		logs:     &logRecorder{},

		sessionRecords: &fakeSessions{sessions: map[string]domain.PracticeSession{}},
		learning:       &fakeLearning{},
		students:       map[string]string{},
		sessions:       map[string]string{},
		challenges:     map[string]*domain.TriggerContext{},
		tapMs:          map[string]int{},
		clock:          time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC),
	}
	w.service = application.NewPracticeEvidenceService(w.reference, w.evidence, w.states, w.history, slog.New(w.logs))
	w.events = application.NewProcessEventService(&fakeCompletion{statuses: map[string]domain.CompletionStatus{}}, w.service,
		application.NewActivityService(w.sessionRecords, w.learning))
	return w
}

// stableUUID derives a deterministic UUID-shaped id from a scenario name, so steps
// can refer to "alice" or "pentatonic-run" by name.
func stableUUID(kind, name string) string {
	sum := sha1.Sum([]byte(kind + ":" + name)) //nolint:gosec // deterministic test ids, not security
	b := sum[:16]
	b[6] = b[6]&0x0f | 0x40 // version 4
	b[8] = b[8]&0x3f | 0x80 // RFC 4122 variant
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func (w *world) studentID(name string) string {
	if id, ok := w.students[name]; ok {
		return id
	}
	w.students[name] = stableUUID("student", name)
	return w.students[name]
}

// playAlongKey names a play-along of a diagram, which exists from the moment a
// scenario mentions it.
func (w *world) playAlongKey(diagram string) string {
	id := stableUUID("diagram", diagram)
	if _, ok := w.reference.diagrams[id]; !ok {
		tempo := playAlongTargetBPM
		w.reference.diagrams[id] = domain.DiagramReference{ID: id, TempoBPM: &tempo}
	}
	return "play_along:" + id
}

func (w *world) answer(student, itemKey string, response domain.PracticeResponse) error {
	w.eventNo++
	studentID := w.studentID(student)
	before, _, _, err := w.states.Get(context.Background(), studentID, itemKey)
	if err != nil {
		return err
	}
	w.lastStudent, w.lastItemKey, w.before = student, itemKey, before
	at := w.clock
	w.clock = w.clock.Add(time.Minute)
	answer := domain.PracticeAnswer{
		EventID:    fmt.Sprintf("e0000000-0000-4000-8000-%012d", w.eventNo),
		StudentID:  studentID,
		OccurredAt: at,
		ItemKey:    itemKey,
		Response:   response,
	}
	if challenge, ok := w.challenges[student]; ok {
		answer.TriggerContext = challenge
	} else {
		answer.PracticeSessionID = w.sessionID(student)
	}
	if tap, ok := w.tapMs[student]; ok && response.LatencyMs != nil {
		answer.TapMs = &tap
	}
	w.lastAnswer = answer
	return w.service.Process(context.Background(), answer)
}

func (w *world) sessionID(student string) string {
	if id, ok := w.sessions[student]; ok {
		return id
	}
	return stableUUID("session", student)
}

func (w *world) fold(student, itemKey string) (domain.ItemFold, error) {
	fold, _, _, err := w.states.Get(context.Background(), w.studentID(student), itemKey)
	return fold, err
}

func (w *world) evidenceFor(student, itemKey string) ([]domain.PracticeEvidence, error) {
	return w.evidence.ListForItem(context.Background(), w.studentID(student), itemKey)
}
