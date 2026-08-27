package application_test

import (
	"context"
	"strconv"
	"sync"

	"github.com/motifpath/core-domain/internal/domain"
)

// fakeUserRepository is a minimal in-memory ports.UserRepository.
type fakeUserRepository struct {
	mu        sync.Mutex
	byClerkID map[string]domain.User
	byID      map[string]domain.User
	createErr error
	getErr    error
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{byClerkID: map[string]domain.User{}, byID: map[string]domain.User{}}
}

func (f *fakeUserRepository) Create(_ context.Context, user domain.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return f.createErr
	}
	if _, exists := f.byClerkID[user.ClerkUserID]; exists {
		return domain.ErrAlreadyExists
	}
	f.byClerkID[user.ClerkUserID] = user
	f.byID[user.ID] = user
	return nil
}

func (f *fakeUserRepository) GetByClerkUserID(_ context.Context, clerkUserID string) (domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return domain.User{}, f.getErr
	}
	user, ok := f.byClerkID[clerkUserID]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return user, nil
}

func (f *fakeUserRepository) GetByID(_ context.Context, id string) (domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return domain.User{}, f.getErr
	}
	user, ok := f.byID[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return user, nil
}

func (f *fakeUserRepository) put(user domain.User) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byClerkID[user.ClerkUserID] = user
	f.byID[user.ID] = user
}

// fakeContentNodeRepository is a minimal in-memory ports.ContentNodeRepository.
type fakeContentNodeRepository struct {
	mu        sync.Mutex
	byID      map[string]domain.ContentNode
	createErr error
	getErr    error
}

func newFakeContentNodeRepository() *fakeContentNodeRepository {
	return &fakeContentNodeRepository{byID: map[string]domain.ContentNode{}}
}

func (f *fakeContentNodeRepository) Create(_ context.Context, node domain.ContentNode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return f.createErr
	}
	f.byID[node.ID] = node
	return nil
}

func (f *fakeContentNodeRepository) GetByID(_ context.Context, id string) (domain.ContentNode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return domain.ContentNode{}, f.getErr
	}
	node, ok := f.byID[id]
	if !ok {
		return domain.ContentNode{}, domain.ErrNotFound
	}
	return node, nil
}

func (f *fakeContentNodeRepository) GetByIDs(_ context.Context, ids []string) (map[string]domain.ContentNode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return nil, f.getErr
	}
	result := map[string]domain.ContentNode{}
	for _, id := range ids {
		if node, ok := f.byID[id]; ok {
			result[id] = node
		}
	}
	return result, nil
}

func (f *fakeContentNodeRepository) put(node domain.ContentNode) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[node.ID] = node
}

// fakeChallengeRepository is a minimal in-memory ports.ChallengeRepository.
type fakeChallengeRepository struct {
	mu        sync.Mutex
	byID      map[string]domain.Challenge
	createErr error
}

func newFakeChallengeRepository() *fakeChallengeRepository {
	return &fakeChallengeRepository{byID: map[string]domain.Challenge{}}
}

func (f *fakeChallengeRepository) Create(_ context.Context, challenge domain.Challenge) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return f.createErr
	}
	f.byID[challenge.ID] = challenge
	return nil
}

func (f *fakeChallengeRepository) GetByID(_ context.Context, id string) (domain.Challenge, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	challenge, ok := f.byID[id]
	if !ok {
		return domain.Challenge{}, domain.ErrNotFound
	}
	return challenge, nil
}

func (f *fakeChallengeRepository) put(challenge domain.Challenge) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[challenge.ID] = challenge
}

// fakeExerciseRepository is a minimal in-memory ports.ExerciseRepository.
type fakeExerciseRepository struct {
	mu        sync.Mutex
	byID      map[string]domain.Exercise
	createErr error
}

func newFakeExerciseRepository() *fakeExerciseRepository {
	return &fakeExerciseRepository{byID: map[string]domain.Exercise{}}
}

func (f *fakeExerciseRepository) Create(_ context.Context, exercise domain.Exercise) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return f.createErr
	}
	f.byID[exercise.ID] = exercise
	return nil
}

func (f *fakeExerciseRepository) GetByID(_ context.Context, id string) (domain.Exercise, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	exercise, ok := f.byID[id]
	if !ok {
		return domain.Exercise{}, domain.ErrNotFound
	}
	return exercise, nil
}

// fakeExpandedContentRepository is a minimal in-memory
// ports.ExpandedContentRepository.
type fakeExpandedContentRepository struct {
	mu        sync.Mutex
	byID      map[string]domain.ExpandedContent
	byNode    map[string][]domain.ExpandedContent
	createErr error
}

func newFakeExpandedContentRepository() *fakeExpandedContentRepository {
	return &fakeExpandedContentRepository{
		byID:   map[string]domain.ExpandedContent{},
		byNode: map[string][]domain.ExpandedContent{},
	}
}

func (f *fakeExpandedContentRepository) Create(_ context.Context, item domain.ExpandedContent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return f.createErr
	}
	f.byID[item.ID] = item
	f.byNode[item.ContentNodeID] = append(f.byNode[item.ContentNodeID], item)
	return nil
}

func (f *fakeExpandedContentRepository) GetByID(_ context.Context, id string) (domain.ExpandedContent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	item, ok := f.byID[id]
	if !ok {
		return domain.ExpandedContent{}, domain.ErrNotFound
	}
	return item, nil
}

func (f *fakeExpandedContentRepository) ListByContentNode(_ context.Context, contentNodeID string) ([]domain.ExpandedContent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.byNode[contentNodeID], nil
}

// fakeLearningPathRepository is a minimal in-memory ports.LearningPathRepository.
type fakeLearningPathRepository struct {
	mu        sync.Mutex
	byID      map[string]domain.LearningPath
	createErr error
}

func newFakeLearningPathRepository() *fakeLearningPathRepository {
	return &fakeLearningPathRepository{byID: map[string]domain.LearningPath{}}
}

func (f *fakeLearningPathRepository) Create(_ context.Context, path domain.LearningPath) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return f.createErr
	}
	f.byID[path.ID] = path
	return nil
}

func (f *fakeLearningPathRepository) GetByID(_ context.Context, id string) (domain.LearningPath, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	path, ok := f.byID[id]
	if !ok {
		return domain.LearningPath{}, domain.ErrNotFound
	}
	return path, nil
}

func (f *fakeLearningPathRepository) put(path domain.LearningPath) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[path.ID] = path
}

// fakePathAssignmentRepository is a minimal in-memory
// ports.PathAssignmentRepository — ReplaceActive mirrors the real
// implementation's "delete then insert, keyed by student_id" semantics.
type fakePathAssignmentRepository struct {
	mu          sync.Mutex
	byStudentID map[string]domain.PathAssignment
	replaceErr  error
}

func newFakePathAssignmentRepository() *fakePathAssignmentRepository {
	return &fakePathAssignmentRepository{byStudentID: map[string]domain.PathAssignment{}}
}

func (f *fakePathAssignmentRepository) ReplaceActive(_ context.Context, assignment domain.PathAssignment) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.replaceErr != nil {
		return f.replaceErr
	}
	f.byStudentID[assignment.StudentID] = assignment
	return nil
}

func (f *fakePathAssignmentRepository) GetActiveByStudentID(_ context.Context, studentID string) (domain.PathAssignment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	assignment, ok := f.byStudentID[studentID]
	if !ok {
		return domain.PathAssignment{}, domain.ErrNotFound
	}
	return assignment, nil
}

// fakeCompletionStateReader is a minimal in-memory ports.CompletionStateReader.
type fakeCompletionStateReader struct {
	statuses map[string]map[string]domain.CompletionStatus // studentID -> contentNodeID -> status
	err      error
}

func newFakeCompletionStateReader() *fakeCompletionStateReader {
	return &fakeCompletionStateReader{statuses: map[string]map[string]domain.CompletionStatus{}}
}

func (f *fakeCompletionStateReader) GetStatuses(_ context.Context, studentID string, contentNodeIDs []string) (map[string]domain.CompletionStatus, error) {
	if f.err != nil {
		return nil, f.err
	}
	result := map[string]domain.CompletionStatus{}
	for _, id := range contentNodeIDs {
		if status, ok := f.statuses[studentID][id]; ok {
			result[id] = status
		}
	}
	return result, nil
}

func (f *fakeCompletionStateReader) set(studentID, contentNodeID string, status domain.CompletionStatus) {
	if f.statuses[studentID] == nil {
		f.statuses[studentID] = map[string]domain.CompletionStatus{}
	}
	f.statuses[studentID][contentNodeID] = status
}

// idSequence returns a deterministic newID func for tests: "id-1", "id-2", ...
func idSequence() func() string {
	n := 0
	return func() string {
		n++
		return "id-" + strconv.Itoa(n)
	}
}
