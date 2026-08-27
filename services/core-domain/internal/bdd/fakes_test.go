//go:build integration

package bdd

import (
	"context"
	"sync"

	"github.com/motifpath/core-domain/internal/domain"
)

// The fakes below are minimal in-memory ports implementations local to the
// bdd package (they can't reuse internal/application's unexported fakes —
// different package, different test binary). Unlike those, these have no
// error-injection knobs: BDD exercises business behavior end-to-end through
// the real Handler and application services, not repository failure modes,
// which the internal/application unit tests already cover.

type fakeUserRepo struct {
	mu        sync.Mutex
	byClerkID map[string]domain.User
	byID      map[string]domain.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{byClerkID: map[string]domain.User{}, byID: map[string]domain.User{}}
}

func (f *fakeUserRepo) Create(_ context.Context, u domain.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.byClerkID[u.ClerkUserID]; exists {
		return domain.ErrAlreadyExists
	}
	f.byClerkID[u.ClerkUserID] = u
	f.byID[u.ID] = u
	return nil
}

func (f *fakeUserRepo) GetByClerkUserID(_ context.Context, clerkUserID string) (domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.byClerkID[clerkUserID]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

func (f *fakeUserRepo) GetByID(_ context.Context, id string) (domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.byID[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

func (f *fakeUserRepo) put(u domain.User) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byClerkID[u.ClerkUserID] = u
	f.byID[u.ID] = u
}

type fakeContentNodeRepo struct {
	mu   sync.Mutex
	byID map[string]domain.ContentNode
}

func newFakeContentNodeRepo() *fakeContentNodeRepo {
	return &fakeContentNodeRepo{byID: map[string]domain.ContentNode{}}
}

func (f *fakeContentNodeRepo) Create(_ context.Context, n domain.ContentNode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[n.ID] = n
	return nil
}

func (f *fakeContentNodeRepo) GetByID(_ context.Context, id string) (domain.ContentNode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n, ok := f.byID[id]
	if !ok {
		return domain.ContentNode{}, domain.ErrNotFound
	}
	return n, nil
}

func (f *fakeContentNodeRepo) GetByIDs(_ context.Context, ids []string) (map[string]domain.ContentNode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := map[string]domain.ContentNode{}
	for _, id := range ids {
		if n, ok := f.byID[id]; ok {
			result[id] = n
		}
	}
	return result, nil
}

func (f *fakeContentNodeRepo) put(n domain.ContentNode) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[n.ID] = n
}

type fakeChallengeRepo struct {
	mu   sync.Mutex
	byID map[string]domain.Challenge
}

func newFakeChallengeRepo() *fakeChallengeRepo {
	return &fakeChallengeRepo{byID: map[string]domain.Challenge{}}
}

func (f *fakeChallengeRepo) Create(_ context.Context, c domain.Challenge) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[c.ID] = c
	return nil
}

func (f *fakeChallengeRepo) GetByID(_ context.Context, id string) (domain.Challenge, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.byID[id]
	if !ok {
		return domain.Challenge{}, domain.ErrNotFound
	}
	return c, nil
}

func (f *fakeChallengeRepo) put(c domain.Challenge) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[c.ID] = c
}

type fakeExerciseRepo struct {
	mu   sync.Mutex
	byID map[string]domain.Exercise
}

func newFakeExerciseRepo() *fakeExerciseRepo {
	return &fakeExerciseRepo{byID: map[string]domain.Exercise{}}
}

func (f *fakeExerciseRepo) Create(_ context.Context, e domain.Exercise) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[e.ID] = e
	return nil
}

func (f *fakeExerciseRepo) GetByID(_ context.Context, id string) (domain.Exercise, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.byID[id]
	if !ok {
		return domain.Exercise{}, domain.ErrNotFound
	}
	return e, nil
}

func (f *fakeExerciseRepo) put(e domain.Exercise) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[e.ID] = e
}

type fakeExpandedContentRepo struct {
	mu     sync.Mutex
	byID   map[string]domain.ExpandedContent
	byNode map[string][]domain.ExpandedContent
}

func newFakeExpandedContentRepo() *fakeExpandedContentRepo {
	return &fakeExpandedContentRepo{byID: map[string]domain.ExpandedContent{}, byNode: map[string][]domain.ExpandedContent{}}
}

func (f *fakeExpandedContentRepo) Create(_ context.Context, item domain.ExpandedContent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[item.ID] = item
	f.byNode[item.ContentNodeID] = append(f.byNode[item.ContentNodeID], item)
	return nil
}

func (f *fakeExpandedContentRepo) GetByID(_ context.Context, id string) (domain.ExpandedContent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	item, ok := f.byID[id]
	if !ok {
		return domain.ExpandedContent{}, domain.ErrNotFound
	}
	return item, nil
}

func (f *fakeExpandedContentRepo) put(item domain.ExpandedContent) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[item.ID] = item
	f.byNode[item.ContentNodeID] = append(f.byNode[item.ContentNodeID], item)
}

func (f *fakeExpandedContentRepo) ListByContentNode(_ context.Context, contentNodeID string) ([]domain.ExpandedContent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	items := append([]domain.ExpandedContent(nil), f.byNode[contentNodeID]...)
	// Mirror the ent adapter's ordering contract for scenarios that assert it.
	sortExpandedContent(items)
	return items, nil
}

func sortExpandedContent(items []domain.ExpandedContent) {
	less := func(i, j int) bool {
		key := func(item domain.ExpandedContent) int {
			if item.TriggerAtSeconds != nil {
				return *item.TriggerAtSeconds
			}
			if item.TriggerAtParagraph != nil {
				return *item.TriggerAtParagraph
			}
			return 0
		}
		return key(items[i]) < key(items[j])
	}
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && less(j, j-1); j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

type fakeLearningPathRepo struct {
	mu   sync.Mutex
	byID map[string]domain.LearningPath
}

func newFakeLearningPathRepo() *fakeLearningPathRepo {
	return &fakeLearningPathRepo{byID: map[string]domain.LearningPath{}}
}

func (f *fakeLearningPathRepo) Create(_ context.Context, p domain.LearningPath) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[p.ID] = p
	return nil
}

func (f *fakeLearningPathRepo) GetByID(_ context.Context, id string) (domain.LearningPath, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.byID[id]
	if !ok {
		return domain.LearningPath{}, domain.ErrNotFound
	}
	return p, nil
}

func (f *fakeLearningPathRepo) put(p domain.LearningPath) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[p.ID] = p
}

type fakePathAssignmentRepo struct {
	mu          sync.Mutex
	byStudentID map[string]domain.PathAssignment
}

func newFakePathAssignmentRepo() *fakePathAssignmentRepo {
	return &fakePathAssignmentRepo{byStudentID: map[string]domain.PathAssignment{}}
}

func (f *fakePathAssignmentRepo) ReplaceActive(_ context.Context, a domain.PathAssignment) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byStudentID[a.StudentID] = a
	return nil
}

func (f *fakePathAssignmentRepo) GetActiveByStudentID(_ context.Context, studentID string) (domain.PathAssignment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.byStudentID[studentID]
	if !ok {
		return domain.PathAssignment{}, domain.ErrNotFound
	}
	return a, nil
}

type fakeCompletionReader struct {
	mu       sync.Mutex
	statuses map[string]map[string]domain.CompletionStatus
}

func newFakeCompletionReader() *fakeCompletionReader {
	return &fakeCompletionReader{statuses: map[string]map[string]domain.CompletionStatus{}}
}

func (f *fakeCompletionReader) GetStatuses(_ context.Context, studentID string, contentNodeIDs []string) (map[string]domain.CompletionStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := map[string]domain.CompletionStatus{}
	for _, id := range contentNodeIDs {
		if status, ok := f.statuses[studentID][id]; ok {
			result[id] = status
		}
	}
	return result, nil
}

func (f *fakeCompletionReader) set(studentID, contentNodeID string, status domain.CompletionStatus) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.statuses[studentID] == nil {
		f.statuses[studentID] = map[string]domain.CompletionStatus{}
	}
	f.statuses[studentID][contentNodeID] = status
}
