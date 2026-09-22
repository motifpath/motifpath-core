package application_test

import (
	"context"
	"sort"
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

func (f *fakeUserRepository) UpdateLocale(_ context.Context, id, locale string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	user, ok := f.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	user.Locale = domain.Language{Code: locale}
	f.byID[id] = user
	f.byClerkID[user.ClerkUserID] = user
	return nil
}

// fakeLanguageRepository is a minimal in-memory ports.LanguageRepository,
// pre-seeded with the system rows the Atlas migration seeds in production:
// en, pt_BR, any.
type fakeLanguageRepository struct {
	mu     sync.Mutex
	byCode map[string]domain.Language
}

func newFakeLanguageRepository() *fakeLanguageRepository {
	return &fakeLanguageRepository{byCode: map[string]domain.Language{
		"en":                   {Code: "en", Name: "English"},
		"pt_BR":                {Code: "pt_BR", Name: "Portuguese (Brazil)"},
		domain.LanguageCodeAny: {Code: domain.LanguageCodeAny, Name: "Language-agnostic"},
	}}
}

func (f *fakeLanguageRepository) GetByCode(_ context.Context, code string) (domain.Language, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	lang, ok := f.byCode[code]
	if !ok {
		return domain.Language{}, domain.ErrNotFound
	}
	return lang, nil
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

func (f *fakeContentNodeRepository) List(_ context.Context, contentType domain.ContentType, skillID, conceptID string, difficulty domain.DifficultyLevel) ([]domain.ContentNode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []domain.ContentNode
	for _, node := range f.byID {
		if contentType != "" && node.ContentType != contentType {
			continue
		}
		if skillID != "" && !containsID(node.Classification.SkillIDs(), skillID) {
			continue
		}
		if conceptID != "" && !containsID(node.Classification.ConceptIDs(), conceptID) {
			continue
		}
		if difficulty != "" && node.Classification.DifficultyLevel != difficulty {
			continue
		}
		result = append(result, node)
	}
	return result, nil
}

func containsID(ids []string, id string) bool {
	for _, existing := range ids {
		if existing == id {
			return true
		}
	}
	return false
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

func (f *fakeExerciseRepository) ListBySkillID(_ context.Context, skillID string) ([]domain.Exercise, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []domain.Exercise
	for _, ex := range f.byID {
		if containsID(exerciseSkillIDs(ex), skillID) {
			result = append(result, ex)
		}
	}
	return result, nil
}

func (f *fakeExerciseRepository) List(_ context.Context, skillID string, exerciseType domain.ExerciseType) ([]domain.Exercise, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []domain.Exercise
	for _, ex := range f.byID {
		if exerciseType != "" && ex.ExerciseType != exerciseType {
			continue
		}
		if skillID != "" && !containsID(exerciseSkillIDs(ex), skillID) {
			continue
		}
		result = append(result, ex)
	}
	return result, nil
}

func exerciseSkillIDs(ex domain.Exercise) []string {
	ids := make([]string, len(ex.Skills))
	for i, s := range ex.Skills {
		ids[i] = s.ID
	}
	return ids
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

// fakeCourseRepository is a minimal in-memory ports.CourseRepository.
type fakeCourseRepository struct {
	mu   sync.Mutex
	byID map[string]domain.Course
}

func newFakeCourseRepository() *fakeCourseRepository {
	return &fakeCourseRepository{byID: map[string]domain.Course{}}
}

func (f *fakeCourseRepository) Create(_ context.Context, course domain.Course) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[course.ID] = course
	return nil
}

func (f *fakeCourseRepository) GetByID(_ context.Context, id string) (domain.Course, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	course, ok := f.byID[id]
	if !ok {
		return domain.Course{}, domain.ErrNotFound
	}
	return course, nil
}

func (f *fakeCourseRepository) List(_ context.Context, status *domain.CourseStatus) ([]domain.Course, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]domain.Course, 0, len(f.byID))
	for _, course := range f.byID {
		if status != nil && course.Status != *status {
			continue
		}
		result = append(result, course)
	}
	return result, nil
}

func (f *fakeCourseRepository) Replace(_ context.Context, course domain.Course) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[course.ID]; !ok {
		return domain.ErrNotFound
	}
	f.byID[course.ID] = course
	return nil
}

func (f *fakeCourseRepository) put(course domain.Course) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[course.ID] = course
}

func (f *fakeCourseRepository) UpdateStatus(_ context.Context, id string, status domain.CourseStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	course, ok := f.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	course.Status = status
	f.byID[id] = course
	return nil
}

// fakeCourseVersionRepository is a minimal in-memory
// ports.CourseVersionRepository.
type fakeCourseVersionRepository struct {
	mu        sync.Mutex
	byCourse  map[string][]domain.CourseVersion
	createErr error
}

func newFakeCourseVersionRepository() *fakeCourseVersionRepository {
	return &fakeCourseVersionRepository{byCourse: map[string][]domain.CourseVersion{}}
}

func (f *fakeCourseVersionRepository) Create(_ context.Context, version domain.CourseVersion) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return f.createErr
	}
	f.byCourse[version.CourseID] = append(f.byCourse[version.CourseID], version)
	return nil
}

func (f *fakeCourseVersionRepository) GetLatestByCourseID(_ context.Context, courseID string) (domain.CourseVersion, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	versions := f.byCourse[courseID]
	if len(versions) == 0 {
		return domain.CourseVersion{}, domain.ErrNotFound
	}
	latest := versions[0]
	for _, v := range versions[1:] {
		if v.VersionNumber > latest.VersionNumber {
			latest = v
		}
	}
	return latest, nil
}

// fakeStudentPathRepository is a minimal in-memory ports.StudentPathRepository.
type fakeStudentPathRepository struct {
	mu        sync.Mutex
	byID      map[string]domain.StudentPath
	createErr error
}

func newFakeStudentPathRepository() *fakeStudentPathRepository {
	return &fakeStudentPathRepository{byID: map[string]domain.StudentPath{}}
}

func (f *fakeStudentPathRepository) Create(_ context.Context, path domain.StudentPath) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return f.createErr
	}
	f.byID[path.ID] = path
	return nil
}

func (f *fakeStudentPathRepository) GetByID(_ context.Context, id string) (domain.StudentPath, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	path, ok := f.byID[id]
	if !ok {
		return domain.StudentPath{}, domain.ErrNotFound
	}
	return path, nil
}

func (f *fakeStudentPathRepository) ListActiveStandaloneByStudentID(_ context.Context, studentID string) ([]domain.StudentPath, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []domain.StudentPath
	for _, path := range f.byID {
		if path.StudentID != studentID {
			continue
		}
		if path.ArchivedAt != nil || path.SourceCourseEnrollmentID != nil {
			continue
		}
		result = append(result, path)
	}
	return result, nil
}

func (f *fakeStudentPathRepository) Archive(_ context.Context, id string, archivedAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	path, ok := f.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	path.ArchivedAt = &archivedAt
	f.byID[id] = path
	return nil
}

// fakeCourseEnrollmentRepository is a minimal in-memory
// ports.CourseEnrollmentRepository.
type fakeCourseEnrollmentRepository struct {
	mu   sync.Mutex
	byID map[string]domain.CourseEnrollment
}

func newFakeCourseEnrollmentRepository() *fakeCourseEnrollmentRepository {
	return &fakeCourseEnrollmentRepository{byID: map[string]domain.CourseEnrollment{}}
}

func (f *fakeCourseEnrollmentRepository) Create(_ context.Context, e domain.CourseEnrollment) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[e.ID] = e
	return nil
}

func (f *fakeCourseEnrollmentRepository) GetByID(_ context.Context, id string) (domain.CourseEnrollment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.byID[id]
	if !ok {
		return domain.CourseEnrollment{}, domain.ErrNotFound
	}
	return e, nil
}

func (f *fakeCourseEnrollmentRepository) GetActiveByCourseID(_ context.Context, studentID, courseID string) (domain.CourseEnrollment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range f.byID {
		if e.StudentID == studentID && e.CourseID == courseID && e.IsActive() {
			return e, nil
		}
	}
	return domain.CourseEnrollment{}, domain.ErrNotFound
}

func (f *fakeCourseEnrollmentRepository) ListByStudentID(_ context.Context, studentID string) ([]domain.CourseEnrollment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []domain.CourseEnrollment
	for _, e := range f.byID {
		if e.StudentID == studentID {
			result = append(result, e)
		}
	}
	return result, nil
}

func (f *fakeCourseEnrollmentRepository) ListActiveByStudentID(_ context.Context, studentID string) ([]domain.CourseEnrollment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []domain.CourseEnrollment
	for _, e := range f.byID {
		if e.StudentID == studentID && e.IsActive() {
			result = append(result, e)
		}
	}
	return result, nil
}

func (f *fakeCourseEnrollmentRepository) Abandon(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	e.Status = domain.CourseEnrollmentStatusAbandoned
	e.ActiveCheckpointStudentPathID = nil
	e.ActiveCheckpointPosition = nil
	f.byID[id] = e
	return nil
}

func (f *fakeCourseEnrollmentRepository) put(e domain.CourseEnrollment) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[e.ID] = e
}

// fakeContentNodeVersionRepository is a minimal in-memory
// ports.ContentNodeVersionRepository.
type fakeContentNodeVersionRepository struct {
	mu      sync.Mutex
	byNode  map[string][]domain.ContentNodeVersion
	nextErr error
}

func newFakeContentNodeVersionRepository() *fakeContentNodeVersionRepository {
	return &fakeContentNodeVersionRepository{byNode: map[string][]domain.ContentNodeVersion{}}
}

func (f *fakeContentNodeVersionRepository) Create(_ context.Context, version domain.ContentNodeVersion) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.nextErr != nil {
		return f.nextErr
	}
	f.byNode[version.ContentNodeID] = append(f.byNode[version.ContentNodeID], version)
	return nil
}

func (f *fakeContentNodeVersionRepository) GetLatestByContentNodeID(_ context.Context, contentNodeID string) (domain.ContentNodeVersion, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	versions := f.byNode[contentNodeID]
	if len(versions) == 0 {
		return domain.ContentNodeVersion{}, domain.ErrNotFound
	}
	latest := versions[0]
	for _, v := range versions[1:] {
		if v.VersionNumber > latest.VersionNumber {
			latest = v
		}
	}
	return latest, nil
}

// fakeStudentLearningStateRepository is a minimal in-memory
// ports.StudentLearningStateRepository.
type fakeStudentLearningStateRepository struct {
	mu        sync.Mutex
	byStudent map[string]domain.StudentLearningState
}

func newFakeStudentLearningStateRepository() *fakeStudentLearningStateRepository {
	return &fakeStudentLearningStateRepository{byStudent: map[string]domain.StudentLearningState{}}
}

func (f *fakeStudentLearningStateRepository) GetByStudentID(_ context.Context, studentID string) (domain.StudentLearningState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	state, ok := f.byStudent[studentID]
	if !ok {
		return domain.StudentLearningState{}, domain.ErrNotFound
	}
	return state, nil
}

func (f *fakeStudentLearningStateRepository) Upsert(_ context.Context, state domain.StudentLearningState) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byStudent[state.StudentID] = state
	return nil
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

// fakeSkillRepository is a minimal in-memory ports.SkillRepository.
type fakeSkillRepository struct {
	mu   sync.Mutex
	byID map[string]domain.Skill
}

func newFakeSkillRepository() *fakeSkillRepository {
	return &fakeSkillRepository{byID: map[string]domain.Skill{}}
}

func (f *fakeSkillRepository) Create(_ context.Context, skill domain.Skill) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[skill.ID] = skill
	return nil
}

func (f *fakeSkillRepository) GetByID(_ context.Context, id string) (domain.Skill, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	skill, ok := f.byID[id]
	if !ok {
		return domain.Skill{}, domain.ErrNotFound
	}
	return skill, nil
}

func (f *fakeSkillRepository) GetByIDs(_ context.Context, ids []string) (map[string]domain.Skill, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := map[string]domain.Skill{}
	for _, id := range ids {
		if skill, ok := f.byID[id]; ok {
			result[id] = skill
		}
	}
	return result, nil
}

func (f *fakeSkillRepository) List(_ context.Context) ([]domain.Skill, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]domain.Skill, 0, len(f.byID))
	for _, skill := range f.byID {
		result = append(result, skill)
	}
	return result, nil
}

func (f *fakeSkillRepository) ExistsSibling(_ context.Context, parentID *string, name string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, skill := range f.byID {
		if skill.Name != name {
			continue
		}
		if samePointerValue(skill.ParentID, parentID) {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeSkillRepository) put(skill domain.Skill) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[skill.ID] = skill
}

// fakeConceptRepository is a minimal in-memory ports.ConceptRepository.
type fakeConceptRepository struct {
	mu   sync.Mutex
	byID map[string]domain.Concept
}

func newFakeConceptRepository() *fakeConceptRepository {
	return &fakeConceptRepository{byID: map[string]domain.Concept{}}
}

func (f *fakeConceptRepository) Create(_ context.Context, concept domain.Concept) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[concept.ID] = concept
	return nil
}

func (f *fakeConceptRepository) GetByID(_ context.Context, id string) (domain.Concept, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	concept, ok := f.byID[id]
	if !ok {
		return domain.Concept{}, domain.ErrNotFound
	}
	return concept, nil
}

func (f *fakeConceptRepository) GetByIDs(_ context.Context, ids []string) (map[string]domain.Concept, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := map[string]domain.Concept{}
	for _, id := range ids {
		if concept, ok := f.byID[id]; ok {
			result[id] = concept
		}
	}
	return result, nil
}

func (f *fakeConceptRepository) List(_ context.Context) ([]domain.Concept, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]domain.Concept, 0, len(f.byID))
	for _, concept := range f.byID {
		result = append(result, concept)
	}
	return result, nil
}

func (f *fakeConceptRepository) ExistsSibling(_ context.Context, parentID *string, name string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, concept := range f.byID {
		if concept.Name != name {
			continue
		}
		if samePointerValue(concept.ParentID, parentID) {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeConceptRepository) put(concept domain.Concept) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[concept.ID] = concept
}

// samePointerValue reports whether a and b are both nil, or both non-nil
// and pointing at equal values — the "same parent_id" comparison
// ExistsSibling needs on plain *string ids.
func samePointerValue(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// fakeInstrumentRepository is a minimal in-memory ports.InstrumentRepository.
type fakeInstrumentRepository struct {
	mu   sync.Mutex
	byID map[string]domain.Instrument
}

func newFakeInstrumentRepository() *fakeInstrumentRepository {
	return &fakeInstrumentRepository{byID: map[string]domain.Instrument{}}
}

func (f *fakeInstrumentRepository) Create(_ context.Context, instrument domain.Instrument) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[instrument.ID] = instrument
	return nil
}

func (f *fakeInstrumentRepository) GetByID(_ context.Context, id string) (domain.Instrument, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	instrument, ok := f.byID[id]
	if !ok {
		return domain.Instrument{}, domain.ErrNotFound
	}
	return instrument, nil
}

func (f *fakeInstrumentRepository) List(_ context.Context) ([]domain.Instrument, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]domain.Instrument, 0, len(f.byID))
	for _, instrument := range f.byID {
		result = append(result, instrument)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (f *fakeInstrumentRepository) put(instrument domain.Instrument) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[instrument.ID] = instrument
}

// fakeDiagramRepository is a minimal in-memory ports.DiagramRepository.
type fakeDiagramRepository struct {
	mu   sync.Mutex
	byID map[string]domain.Diagram
}

func newFakeDiagramRepository() *fakeDiagramRepository {
	return &fakeDiagramRepository{byID: map[string]domain.Diagram{}}
}

func (f *fakeDiagramRepository) Create(_ context.Context, diagram domain.Diagram) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[diagram.ID] = diagram
	return nil
}

func (f *fakeDiagramRepository) GetByID(_ context.Context, id string) (domain.Diagram, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	diagram, ok := f.byID[id]
	if !ok {
		return domain.Diagram{}, domain.ErrNotFound
	}
	return diagram, nil
}

func (f *fakeDiagramRepository) List(_ context.Context, instrumentID, skillID, conceptID string) ([]domain.Diagram, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := []domain.Diagram{}
	for _, diagram := range f.byID {
		if instrumentID != "" && diagram.InstrumentID != instrumentID {
			continue
		}
		if skillID != "" && !containsID(diagram.SkillIDs(), skillID) {
			continue
		}
		if conceptID != "" && !containsID(diagram.ConceptIDs(), conceptID) {
			continue
		}
		result = append(result, diagram)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (f *fakeDiagramRepository) Update(_ context.Context, diagram domain.Diagram) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[diagram.ID]; !ok {
		return domain.ErrNotFound
	}
	f.byID[diagram.ID] = diagram
	return nil
}
