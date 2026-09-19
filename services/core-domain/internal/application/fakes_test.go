package application_test

import (
	"context"
	"strconv"
	"sync"
	"time"

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

func (f *fakeContentNodeRepository) List(_ context.Context, contentType domain.ContentType, skill string, difficulty domain.DifficultyLevel) ([]domain.ContentNode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []domain.ContentNode
	for _, node := range f.byID {
		if contentType != "" && node.ContentType != contentType {
			continue
		}
		if skill != "" && node.Classification.Skill != skill {
			continue
		}
		if difficulty != "" && node.Classification.DifficultyLevel != difficulty {
			continue
		}
		result = append(result, node)
	}
	return result, nil
}

func (f *fakeContentNodeRepository) Update(_ context.Context, node domain.ContentNode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[node.ID]; !ok {
		return domain.ErrNotFound
	}
	f.byID[node.ID] = node
	return nil
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

func (f *fakeChallengeRepository) ListByContentNodeID(_ context.Context, contentNodeID string) ([]domain.Challenge, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []domain.Challenge
	for _, c := range f.byID {
		if c.ContentNodeID == contentNodeID {
			result = append(result, c)
		}
	}
	return result, nil
}

func (f *fakeChallengeRepository) Update(_ context.Context, challenge domain.Challenge) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[challenge.ID]; !ok {
		return domain.ErrNotFound
	}
	f.byID[challenge.ID] = challenge
	return nil
}

// fakeExerciseRepository is a minimal in-memory ports.ExerciseRepository.
// byChallengeOrder/byNodeOrder track link order per challenge/node
// separately from each exercise's own ChallengeIDs/ContentNodeIDs slice,
// since two different challenges can share exercises linked in different
// orders.
type fakeExerciseRepository struct {
	mu               sync.Mutex
	byID             map[string]domain.Exercise
	byChallengeOrder map[string][]string
	byNodeOrder      map[string][]string
	createErr        error
}

func newFakeExerciseRepository() *fakeExerciseRepository {
	return &fakeExerciseRepository{
		byID:             map[string]domain.Exercise{},
		byChallengeOrder: map[string][]string{},
		byNodeOrder:      map[string][]string{},
	}
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

func (f *fakeExerciseRepository) LinkChallenge(_ context.Context, exerciseID, challengeID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	exercise, ok := f.byID[exerciseID]
	if !ok {
		return domain.ErrNotFound
	}
	exercise.ChallengeIDs = append(exercise.ChallengeIDs, challengeID)
	f.byID[exerciseID] = exercise
	f.byChallengeOrder[challengeID] = append(f.byChallengeOrder[challengeID], exerciseID)
	return nil
}

func (f *fakeExerciseRepository) UnlinkChallenge(_ context.Context, exerciseID, challengeID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	exercise, ok := f.byID[exerciseID]
	if !ok {
		return domain.ErrNotFound
	}
	remaining := make([]string, 0, len(exercise.ChallengeIDs))
	for _, id := range exercise.ChallengeIDs {
		if id != challengeID {
			remaining = append(remaining, id)
		}
	}
	exercise.ChallengeIDs = remaining
	f.byID[exerciseID] = exercise

	remainingOrder := make([]string, 0, len(f.byChallengeOrder[challengeID]))
	for _, id := range f.byChallengeOrder[challengeID] {
		if id != exerciseID {
			remainingOrder = append(remainingOrder, id)
		}
	}
	f.byChallengeOrder[challengeID] = remainingOrder
	return nil
}

func (f *fakeExerciseRepository) ListByChallengeID(_ context.Context, challengeID string) ([]domain.Exercise, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]domain.Exercise, 0, len(f.byChallengeOrder[challengeID]))
	for _, id := range f.byChallengeOrder[challengeID] {
		result = append(result, f.byID[id])
	}
	return result, nil
}

func (f *fakeExerciseRepository) ListByChallengeIDs(_ context.Context, challengeIDs []string) (map[string][]domain.Exercise, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := map[string][]domain.Exercise{}
	for _, challengeID := range challengeIDs {
		ids := f.byChallengeOrder[challengeID]
		if len(ids) == 0 {
			continue
		}
		exercises := make([]domain.Exercise, 0, len(ids))
		for _, id := range ids {
			exercises = append(exercises, f.byID[id])
		}
		result[challengeID] = exercises
	}
	return result, nil
}

func (f *fakeExerciseRepository) LinkContentNode(_ context.Context, exerciseID, contentNodeID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	exercise, ok := f.byID[exerciseID]
	if !ok {
		return domain.ErrNotFound
	}
	exercise.ContentNodeIDs = append(exercise.ContentNodeIDs, contentNodeID)
	f.byID[exerciseID] = exercise
	f.byNodeOrder[contentNodeID] = append(f.byNodeOrder[contentNodeID], exerciseID)
	return nil
}

func (f *fakeExerciseRepository) UnlinkContentNode(_ context.Context, exerciseID, contentNodeID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	exercise, ok := f.byID[exerciseID]
	if !ok {
		return domain.ErrNotFound
	}
	remaining := make([]string, 0, len(exercise.ContentNodeIDs))
	for _, id := range exercise.ContentNodeIDs {
		if id != contentNodeID {
			remaining = append(remaining, id)
		}
	}
	exercise.ContentNodeIDs = remaining
	f.byID[exerciseID] = exercise

	remainingOrder := make([]string, 0, len(f.byNodeOrder[contentNodeID]))
	for _, id := range f.byNodeOrder[contentNodeID] {
		if id != exerciseID {
			remainingOrder = append(remainingOrder, id)
		}
	}
	f.byNodeOrder[contentNodeID] = remainingOrder
	return nil
}

func (f *fakeExerciseRepository) ListByContentNodeID(_ context.Context, contentNodeID string) ([]domain.Exercise, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]domain.Exercise, 0, len(f.byNodeOrder[contentNodeID]))
	for _, id := range f.byNodeOrder[contentNodeID] {
		result = append(result, f.byID[id])
	}
	return result, nil
}

func (f *fakeExerciseRepository) ListBySkillTag(_ context.Context, skillTag string) ([]domain.Exercise, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []domain.Exercise
	for _, ex := range f.byID {
		for _, tag := range ex.SkillTags {
			if tag == skillTag {
				result = append(result, ex)
				break
			}
		}
	}
	return result, nil
}

func (f *fakeExerciseRepository) List(_ context.Context, skillTag string, exerciseType domain.ExerciseType) ([]domain.Exercise, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []domain.Exercise
	for _, ex := range f.byID {
		if exerciseType != "" && ex.ExerciseType != exerciseType {
			continue
		}
		if skillTag != "" {
			matched := false
			for _, tag := range ex.SkillTags {
				if tag == skillTag {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		result = append(result, ex)
	}
	return result, nil
}

func (f *fakeExerciseRepository) Update(_ context.Context, exercise domain.Exercise) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[exercise.ID]; !ok {
		return domain.ErrNotFound
	}
	f.byID[exercise.ID] = exercise
	return nil
}

// put seeds exercise directly, also indexing it under its pre-set
// ChallengeIDs/ContentNodeIDs (in the order they appear there) so tests that
// construct an already-linked domain.Exercise literal work with
// ListByChallengeID/ListByContentNodeID without going through
// LinkChallenge/LinkContentNode first.
func (f *fakeExerciseRepository) put(exercise domain.Exercise) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[exercise.ID] = exercise
	for _, challengeID := range exercise.ChallengeIDs {
		f.byChallengeOrder[challengeID] = append(f.byChallengeOrder[challengeID], exercise.ID)
	}
	for _, nodeID := range exercise.ContentNodeIDs {
		f.byNodeOrder[nodeID] = append(f.byNodeOrder[nodeID], exercise.ID)
	}
}

func (f *fakeExerciseRepository) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.byID)
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

func (f *fakeExpandedContentRepository) Update(_ context.Context, item domain.ExpandedContent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[item.ID]; !ok {
		return domain.ErrNotFound
	}
	f.byID[item.ID] = item
	nodeItems := f.byNode[item.ContentNodeID]
	for i, existing := range nodeItems {
		if existing.ID == item.ID {
			nodeItems[i] = item
			break
		}
	}
	return nil
}

func (f *fakeExpandedContentRepository) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	item, ok := f.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	delete(f.byID, id)
	nodeItems := f.byNode[item.ContentNodeID]
	remaining := make([]domain.ExpandedContent, 0, len(nodeItems))
	for _, existing := range nodeItems {
		if existing.ID != id {
			remaining = append(remaining, existing)
		}
	}
	f.byNode[item.ContentNodeID] = remaining
	return nil
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

func (f *fakeLearningPathRepository) List(_ context.Context) ([]domain.LearningPath, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]domain.LearningPath, 0, len(f.byID))
	for _, path := range f.byID {
		result = append(result, path)
	}
	return result, nil
}

func (f *fakeLearningPathRepository) Replace(_ context.Context, path domain.LearningPath) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[path.ID]; !ok {
		return domain.ErrNotFound
	}
	f.byID[path.ID] = path
	return nil
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

// fakeMediaStorage is a minimal in-memory ports.MediaStorage.
type fakeMediaStorage struct {
	mu          sync.Mutex
	presignErr  error
	lastKey     string
	lastContent domain.MediaContentType
}

func newFakeMediaStorage() *fakeMediaStorage {
	return &fakeMediaStorage{}
}

func (f *fakeMediaStorage) PresignUpload(_ context.Context, objectKey string, contentType domain.MediaContentType) (domain.MediaUploadURL, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.presignErr != nil {
		return domain.MediaUploadURL{}, f.presignErr
	}
	f.lastKey = objectKey
	f.lastContent = contentType
	return domain.MediaUploadURL{
		UploadURL: "https://storage.example.com/" + objectKey + "?presigned=1",
		ObjectURL: "https://cdn.example.com/" + objectKey,
		ExpiresAt: fixedCreatedAt.Add(15 * time.Minute),
	}, nil
}

// idSequence returns a deterministic newID func for tests: "id-1", "id-2", ...
func idSequence() func() string {
	n := 0
	return func() string {
		n++
		return "id-" + strconv.Itoa(n)
	}
}
