package application_test

import (
	"context"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// fakeUserRepository is a minimal in-memory ports.UserRepository.
type fakeUserRepository struct {
	mu        sync.Mutex
	byClerkID map[string]domain.User
	byID      map[string]domain.User
	createErr error
	getErr    error

	// displayNameWrites counts UpdateDisplayName calls, so a test can tell
	// a real refresh from a no-op.
	displayNameWrites int
	// displayNameLookups records the ids of every GetDisplayNames call, so a
	// test can check what the service actually asked for.
	displayNameLookups [][]string
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

func (f *fakeUserRepository) UpdateDisplayName(_ context.Context, id, displayName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	user, ok := f.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	f.displayNameWrites++
	user.DisplayName = displayName
	f.byID[id] = user
	f.byClerkID[user.ClerkUserID] = user
	return nil
}

func (f *fakeUserRepository) GetDisplayNames(_ context.Context, ids []string) (map[string]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.displayNameLookups = append(f.displayNameLookups, append([]string(nil), ids...))
	names := make(map[string]string, len(ids))
	for _, id := range ids {
		if user, ok := f.byID[id]; ok {
			names[id] = user.DisplayName
		}
	}
	return names, nil
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

func (f *fakeLanguageRepository) List(_ context.Context) ([]domain.Language, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]domain.Language, 0, len(f.byCode))
	for _, lang := range f.byCode {
		result = append(result, lang)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Code < result[j].Code })
	return result, nil
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
	// linkedExercises is what LinkedExerciseInstrumentSets returns per node.
	linkedExercises map[string][][]string
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

func (f *fakeContentNodeRepository) LinkedExerciseInstrumentSets(_ context.Context, id string) ([][]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.linkedExercises[id], nil
}

func (f *fakeContentNodeRepository) put(node domain.ContentNode) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[node.ID] = node
}

func (f *fakeContentNodeRepository) List(_ context.Context, filter domain.ContentNodeFilter, page domain.PageRequest) (domain.Page[domain.ContentNode], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []domain.ContentNode
	for _, node := range f.byID {
		if filter.ContentType != "" && node.ContentType != filter.ContentType {
			continue
		}
		if filter.SkillID != "" && !containsID(node.Classification.SkillIDs(), filter.SkillID) {
			continue
		}
		if filter.ConceptID != "" && !containsID(node.Classification.ConceptIDs(), filter.ConceptID) {
			continue
		}
		if filter.Difficulty != "" && node.Classification.DifficultyLevel != filter.Difficulty {
			continue
		}
		if !containsFold(node.Title, filter.Query) {
			continue
		}
		result = append(result, node)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Title != result[j].Title {
			return result[i].Title < result[j].Title
		}
		return result[i].ID < result[j].ID
	})
	return paginate(result, page), nil
}

// containsFold reports whether s contains substr, ignoring case. An empty
// substr always matches — the "no text filter" case.
func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// paginate windows items (already filtered and ordered) per page and
// reports the size of the whole set as Total.
func paginate[T any](items []T, page domain.PageRequest) domain.Page[T] {
	total := len(items)
	start := page.Offset
	if start > total {
		start = total
	}
	end := start + page.Limit
	if end > total {
		end = total
	}
	window := make([]T, end-start)
	copy(window, items[start:end])
	return domain.Page[T]{Items: window, Total: total}
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

func (f *fakeExerciseRepository) List(_ context.Context, filter domain.ExerciseFilter, page domain.PageRequest) (domain.Page[domain.Exercise], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []domain.Exercise
	for _, ex := range f.byID {
		if filter.ExerciseType != "" && ex.ExerciseType != filter.ExerciseType {
			continue
		}
		if filter.SkillID != "" && !containsID(exerciseSkillIDs(ex), filter.SkillID) {
			continue
		}
		if filter.ConceptID != "" && !slices.ContainsFunc(ex.Concepts, func(c domain.KnowledgeNode) bool { return c.ID == filter.ConceptID }) {
			continue
		}
		if filter.Language != "" && !slices.ContainsFunc(ex.Languages, func(l domain.Language) bool { return l.Code == filter.Language }) {
			continue
		}
		if filter.CreatedBy != "" && ex.CreatedBy != filter.CreatedBy {
			continue
		}
		if filter.Query != "" && !strings.Contains(strings.ToLower(ex.Title), strings.ToLower(filter.Query)) {
			continue
		}
		if len(filter.InstrumentIDs) > 0 && len(ex.InstrumentIDs) > 0 && !slices.ContainsFunc(filter.InstrumentIDs, func(id string) bool { return slices.Contains(ex.InstrumentIDs, id) }) {
			continue
		}
		result = append(result, ex)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return paginate(result, page), nil
}

func (f *fakeExerciseRepository) ListCreatorIDs(_ context.Context) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	seen := map[string]bool{}
	ids := []string{}
	for _, ex := range f.byID {
		if ex.CreatedBy != "" && !seen[ex.CreatedBy] {
			seen[ex.CreatedBy] = true
			ids = append(ids, ex.CreatedBy)
		}
	}
	return ids, nil
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
	mu              sync.Mutex
	byID            map[string]domain.LearningPath
	createErr       error
	countItemsCalls int
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

func (f *fakeLearningPathRepository) CountItems(_ context.Context, ids []string) (map[string]int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.countItemsCalls++
	counts := map[string]int{}
	for _, id := range ids {
		if path, ok := f.byID[id]; ok {
			counts[id] = len(path.Items)
		}
	}
	return counts, nil
}

// put seeds a path directly. A path seeded without an author belongs to
// teacherCaller, since a teacher works only with their own paths.
func (f *fakeLearningPathRepository) put(path domain.LearningPath) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if path.TeacherID == "" {
		path.TeacherID = teacherCaller().ID
	}
	f.byID[path.ID] = path
}

func (f *fakeLearningPathRepository) List(_ context.Context, filter domain.LearningPathFilter, page domain.PageRequest) (domain.Page[domain.LearningPath], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]domain.LearningPath, 0, len(f.byID))
	for _, path := range f.byID {
		if !containsFold(path.Title, filter.Query) {
			continue
		}
		if filter.Status != "" && path.Status != filter.Status {
			continue
		}
		if filter.Language != "" && (path.Language == nil || *path.Language != filter.Language) {
			continue
		}
		if filter.CreatedBy != "" && path.TeacherID != filter.CreatedBy {
			continue
		}
		result = append(result, path)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Title != result[j].Title {
			return result[i].Title < result[j].Title
		}
		return result[i].ID < result[j].ID
	})
	return paginate(result, page), nil
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

func (f *fakeLearningPathRepository) ListCreatorIDs(ctx context.Context, filter domain.LearningPathFilter) ([]string, error) {
	page, err := f.List(ctx, filter, domain.PageRequest{Limit: len(f.byID) + 1})
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	ids := []string{}
	for _, p := range page.Items {
		if !seen[p.TeacherID] {
			seen[p.TeacherID] = true
			ids = append(ids, p.TeacherID)
		}
	}
	return ids, nil
}

func (f *fakeLearningPathRepository) UpdateStatus(_ context.Context, id string, status domain.LearningPathStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	path, ok := f.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	path.Status = status
	f.byID[id] = path
	return nil
}

func (f *fakeLearningPathRepository) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[id]; !ok {
		return domain.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

// fakeCourseRepository is a minimal in-memory ports.CourseRepository.
type fakeCourseRepository struct {
	mu   sync.Mutex
	byID map[string]domain.Course
	// lastFilter records the filter most recently passed to List, so tests
	// can assert what the service asked the repository for.
	lastFilter domain.CourseListFilter
	// getByIDErr, when set, fails every GetByID — standing in for a live
	// draft whose checkpoints no longer resolve.
	getByIDErr         error
	getCreatorIDsCalls int
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
	if f.getByIDErr != nil {
		return domain.Course{}, f.getByIDErr
	}
	course, ok := f.byID[id]
	if !ok {
		return domain.Course{}, domain.ErrNotFound
	}
	return course, nil
}

func (f *fakeCourseRepository) GetCreatorIDs(_ context.Context, ids []string) (map[string]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getCreatorIDsCalls++
	creators := map[string]string{}
	for _, id := range ids {
		if course, ok := f.byID[id]; ok {
			creators[id] = course.CreatedBy
		}
	}
	return creators, nil
}

// List applies every filter except the classification ones (SkillIDs /
// ConceptIDs), which need the checkpoint-to-content-node join only the real
// repository has — those are covered by the repository's integration tests.
func (f *fakeCourseRepository) List(_ context.Context, filter domain.CourseListFilter, page domain.PageRequest) (domain.Page[domain.Course], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastFilter = filter
	result := make([]domain.Course, 0, len(f.byID))
	for _, course := range f.byID {
		if filter.Status != nil && course.Status != *filter.Status {
			continue
		}
		if filter.CreatedBy != "" && course.CreatedBy != filter.CreatedBy {
			continue
		}
		if len(filter.Levels) > 0 && !containsLevel(filter.Levels, course.Level) {
			continue
		}
		if !containsFold(course.Title, filter.Query) && !containsFold(course.Summary, filter.Query) {
			continue
		}
		result = append(result, course)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Title != result[j].Title {
			return result[i].Title < result[j].Title
		}
		return result[i].ID < result[j].ID
	})
	return paginate(result, page), nil
}

// ListCreatorIDs applies the same filters as List, over every matching course.
func (f *fakeCourseRepository) ListCreatorIDs(ctx context.Context, filter domain.CourseListFilter) ([]string, error) {
	all, err := f.List(ctx, filter, domain.PageRequest{Limit: len(f.byID) + 1})
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	ids := []string{}
	for _, course := range all.Items {
		if !seen[course.CreatedBy] {
			seen[course.CreatedBy] = true
			ids = append(ids, course.CreatedBy)
		}
	}
	return ids, nil
}

func containsLevel(levels []domain.DifficultyLevel, level domain.DifficultyLevel) bool {
	for _, l := range levels {
		if l == level {
			return true
		}
	}
	return false
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

func (f *fakeCourseVersionRepository) GetByCourseIDAndVersionNumber(_ context.Context, courseID string, versionNumber int) (domain.CourseVersion, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, version := range f.byCourse[courseID] {
		if version.VersionNumber == versionNumber {
			return version, nil
		}
	}
	return domain.CourseVersion{}, domain.ErrNotFound
}

func (f *fakeCourseVersionRepository) GetLatestByCourseIDs(_ context.Context, courseIDs []string) (map[string]domain.CourseVersion, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make(map[string]domain.CourseVersion, len(courseIDs))
	for _, courseID := range courseIDs {
		versions := f.byCourse[courseID]
		if len(versions) == 0 {
			continue
		}
		latest := versions[0]
		for _, v := range versions[1:] {
			if v.VersionNumber > latest.VersionNumber {
				latest = v
			}
		}
		result[courseID] = latest
	}
	return result, nil
}

func (f *fakeCourseVersionRepository) IsLearningPathReferenced(_ context.Context, learningPathID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, versions := range f.byCourse {
		for _, v := range versions {
			for _, cp := range v.Checkpoints {
				if cp.LearningPathID == learningPathID {
					return true, nil
				}
			}
		}
	}
	return false, nil
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

func (f *fakeStudentPathRepository) FindActiveStandalone(_ context.Context, studentID, templateID string) (domain.StudentPath, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, path := range f.byID {
		if path.StudentID == studentID && path.SourceTemplateID == templateID && path.ArchivedAt == nil && path.SourceCourseEnrollmentID == nil {
			return path, nil
		}
	}
	return domain.StudentPath{}, domain.ErrNotFound
}

func (f *fakeStudentPathRepository) Create(_ context.Context, path domain.StudentPath) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return f.createErr
	}
	if path.SourceCourseEnrollmentID == nil && path.ArchivedAt == nil {
		for _, existing := range f.byID {
			if existing.StudentID == path.StudentID && existing.SourceTemplateID == path.SourceTemplateID && existing.ArchivedAt == nil && existing.SourceCourseEnrollmentID == nil {
				return domain.ErrAlreadyExists
			}
		}
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

func (f *fakeStudentPathRepository) ListStandaloneByStudentID(_ context.Context, studentID string) ([]domain.StudentPath, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := []domain.StudentPath{}
	for _, path := range f.byID {
		if path.StudentID != studentID || path.SourceCourseEnrollmentID != nil {
			continue
		}
		result = append(result, path)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].AssignedAt.After(result[j].AssignedAt) })
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

func (f *fakeCourseEnrollmentRepository) AdvanceCheckpoint(_ context.Context, id, studentPathID string, position int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	e.ActiveCheckpointStudentPathID = &studentPathID
	e.ActiveCheckpointPosition = &position
	f.byID[id] = e
	return nil
}

func (f *fakeCourseEnrollmentRepository) Complete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	e.Status = domain.CourseEnrollmentStatusCompleted
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

func (f *fakeContentNodeVersionRepository) ListByContentNodeID(_ context.Context, contentNodeID string) ([]domain.ContentNodeVersion, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := append([]domain.ContentNodeVersion{}, f.byNode[contentNodeID]...)
	sort.Slice(result, func(i, j int) bool { return result[i].VersionNumber > result[j].VersionNumber })
	return result, nil
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

// fakeKnowledgeNodeRepository is a minimal in-memory
// ports.KnowledgeNodeRepository. Children and edges (when edges is set) are
// counted from its own state; the rest of a node's usage, and the
// instruments of the items classified under it, are whatever a test puts in
// usage and classified.
type fakeKnowledgeNodeRepository struct {
	mu         sync.Mutex
	byID       map[string]domain.KnowledgeNode
	edges      *fakeKnowledgeEdgeRepository
	usage      map[string]ports.KnowledgeNodeUsage
	classified map[string][][]string
}

func newFakeKnowledgeNodeRepository() *fakeKnowledgeNodeRepository {
	return &fakeKnowledgeNodeRepository{
		byID:       map[string]domain.KnowledgeNode{},
		usage:      map[string]ports.KnowledgeNodeUsage{},
		classified: map[string][][]string{},
	}
}

func (f *fakeKnowledgeNodeRepository) Create(_ context.Context, node domain.KnowledgeNode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, existing := range f.byID {
		if existing.Key == node.Key {
			return domain.ErrAlreadyExists
		}
	}
	f.byID[node.ID] = node
	return nil
}

func (f *fakeKnowledgeNodeRepository) GetByID(_ context.Context, id string) (domain.KnowledgeNode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	node, ok := f.byID[id]
	if !ok {
		return domain.KnowledgeNode{}, domain.ErrNotFound
	}
	return node, nil
}

// joined returns each of nodes as stored here, keeping one this repository
// doesn't hold as given.
func (f *fakeKnowledgeNodeRepository) joined(nodes []domain.KnowledgeNode) []domain.KnowledgeNode {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]domain.KnowledgeNode, len(nodes))
	for i, node := range nodes {
		if stored, ok := f.byID[node.ID]; ok {
			node = stored
		}
		result[i] = node
	}
	return result
}

func (f *fakeKnowledgeNodeRepository) GetByIDs(_ context.Context, ids []string) (map[string]domain.KnowledgeNode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := map[string]domain.KnowledgeNode{}
	for _, id := range ids {
		if node, ok := f.byID[id]; ok {
			result[id] = node
		}
	}
	return result, nil
}

func (f *fakeKnowledgeNodeRepository) GetByKeys(_ context.Context, keys []string) (map[string]domain.KnowledgeNode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := map[string]domain.KnowledgeNode{}
	for _, node := range f.byID {
		if slices.Contains(keys, node.Key) {
			result[node.Key] = node
		}
	}
	return result, nil
}

func (f *fakeKnowledgeNodeRepository) List(_ context.Context, filter ports.KnowledgeNodeFilter) ([]domain.KnowledgeNode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := []domain.KnowledgeNode{}
	for _, node := range f.byID {
		if filter.Kind != nil && node.Kind != *filter.Kind {
			continue
		}
		if len(filter.InstrumentIDs) > 0 && !node.Suits(filter.InstrumentIDs) {
			continue
		}
		result = append(result, node)
	}
	slices.SortFunc(result, func(a, b domain.KnowledgeNode) int { return strings.Compare(a.Key, b.Key) })
	return result, nil
}

func (f *fakeKnowledgeNodeRepository) Update(_ context.Context, node domain.KnowledgeNode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[node.ID]; !ok {
		return domain.ErrNotFound
	}
	f.byID[node.ID] = node
	return nil
}

func (f *fakeKnowledgeNodeRepository) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[id]; !ok {
		return domain.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

func (f *fakeKnowledgeNodeRepository) Children(_ context.Context, id string) ([]domain.KnowledgeNode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var children []domain.KnowledgeNode
	for _, node := range f.byID {
		if node.ParentID != nil && *node.ParentID == id {
			children = append(children, node)
		}
	}
	return children, nil
}

func (f *fakeKnowledgeNodeRepository) InSubtree(_ context.Context, rootID, candidateID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id := candidateID; ; {
		if id == rootID {
			return true, nil
		}
		node, ok := f.byID[id]
		if !ok || node.ParentID == nil {
			return false, nil
		}
		id = *node.ParentID
	}
}

func (f *fakeKnowledgeNodeRepository) Usage(ctx context.Context, id string) (ports.KnowledgeNodeUsage, error) {
	f.mu.Lock()
	usage := f.usage[id]
	for _, node := range f.byID {
		if node.ParentID != nil && *node.ParentID == id {
			usage.Children++
		}
	}
	f.mu.Unlock()
	if f.edges != nil {
		for _, filter := range []ports.KnowledgeEdgeFilter{{FromID: &id}, {ToID: &id}} {
			edges, err := f.edges.List(ctx, filter)
			if err != nil {
				return ports.KnowledgeNodeUsage{}, err
			}
			usage.Edges += len(edges)
		}
	}
	return usage, nil
}

func (f *fakeKnowledgeNodeRepository) ClassifiedInstrumentSets(_ context.Context, id string) ([][]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.classified[id], nil
}

func (f *fakeKnowledgeNodeRepository) put(node domain.KnowledgeNode) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[node.ID] = node
}

// fakeKnowledgeEdgeRepository is a minimal in-memory
// ports.KnowledgeEdgeRepository.
type fakeKnowledgeEdgeRepository struct {
	mu   sync.Mutex
	byID map[string]domain.KnowledgeEdge
}

func newFakeKnowledgeEdgeRepository() *fakeKnowledgeEdgeRepository {
	return &fakeKnowledgeEdgeRepository{byID: map[string]domain.KnowledgeEdge{}}
}

func (f *fakeKnowledgeEdgeRepository) Create(_ context.Context, edge domain.KnowledgeEdge) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, existing := range f.byID {
		if existing.FromID == edge.FromID && existing.ToID == edge.ToID && existing.Type == edge.Type {
			return domain.ErrAlreadyExists
		}
	}
	f.byID[edge.ID] = edge
	return nil
}

func (f *fakeKnowledgeEdgeRepository) GetByID(_ context.Context, id string) (domain.KnowledgeEdge, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	edge, ok := f.byID[id]
	if !ok {
		return domain.KnowledgeEdge{}, domain.ErrNotFound
	}
	return edge, nil
}

func (f *fakeKnowledgeEdgeRepository) List(_ context.Context, filter ports.KnowledgeEdgeFilter) ([]domain.KnowledgeEdge, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := []domain.KnowledgeEdge{}
	for _, edge := range f.byID {
		if filter.Type != nil && edge.Type != *filter.Type ||
			filter.FromID != nil && edge.FromID != *filter.FromID ||
			filter.ToID != nil && edge.ToID != *filter.ToID {
			continue
		}
		result = append(result, edge)
	}
	slices.SortFunc(result, func(a, b domain.KnowledgeEdge) int { return strings.Compare(a.ID, b.ID) })
	return result, nil
}

func (f *fakeKnowledgeEdgeRepository) UpdateLevel(_ context.Context, edge domain.KnowledgeEdge) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[edge.ID]; !ok {
		return domain.ErrNotFound
	}
	f.byID[edge.ID] = edge
	return nil
}

func (f *fakeKnowledgeEdgeRepository) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[id]; !ok {
		return domain.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

func (f *fakeKnowledgeEdgeRepository) RequiresPathExists(_ context.Context, fromID, toID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	seen := map[string]bool{}
	frontier := []string{fromID}
	for len(frontier) > 0 {
		id := frontier[0]
		frontier = frontier[1:]
		if id == toID {
			return true, nil
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		for _, edge := range f.byID {
			if edge.Type == domain.KnowledgeEdgeTypeRequires && edge.FromID == id {
				frontier = append(frontier, edge.ToID)
			}
		}
	}
	return false, nil
}

// fakeInstrumentRepository is a minimal in-memory ports.InstrumentRepository.
type fakeInstrumentRepository struct {
	mu   sync.Mutex
	byID map[string]domain.Instrument
	// getErrs makes GetByID fail with the given error for an id, standing in
	// for a database failure.
	getErrs map[string]error
}

func newFakeInstrumentRepository() *fakeInstrumentRepository {
	return &fakeInstrumentRepository{byID: map[string]domain.Instrument{}, getErrs: map[string]error{}}
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
	if err, ok := f.getErrs[id]; ok {
		return domain.Instrument{}, err
	}
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

func (f *fakeInstrumentRepository) Update(_ context.Context, instrument domain.Instrument) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	current, ok := f.byID[instrument.ID]
	if !ok {
		return domain.ErrNotFound
	}
	current.Names, current.DefaultVoiceID, current.Icon = instrument.Names, instrument.DefaultVoiceID, instrument.Icon
	f.byID[instrument.ID] = current
	return nil
}

func (f *fakeInstrumentRepository) put(instrument domain.Instrument) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[instrument.ID] = instrument
}

// fakeVoiceRepository is an in-memory ports.VoiceRepository holding the
// platform's voices: two fretted, one keyboard. acoustic-guitar's pitches
// are stored out of order, so a test can tell that callers sort them.
type fakeVoiceRepository struct {
	byID map[string]domain.Voice
}

func newFakeVoiceRepository() *fakeVoiceRepository {
	names := func(name string) domain.LocalizedText { return domain.LocalizedText{"en": name, "pt_BR": name} }
	return &fakeVoiceRepository{byID: map[string]domain.Voice{
		"piano":           {ID: "piano", Names: names("Piano"), Family: domain.InstrumentFamilyKeyboard, Pitches: []int{21, 24}, Attribution: "CC-BY 3.0"},
		"acoustic-guitar": {ID: "acoustic-guitar", Names: names("Acoustic guitar"), Family: domain.InstrumentFamilyFretted, Pitches: []int{43, 40}, Attribution: "CC-BY 3.0"},
		"electric-guitar": {ID: "electric-guitar", Names: names("Electric guitar"), Family: domain.InstrumentFamilyFretted, Pitches: []int{40}, Attribution: "CC-BY 3.0"},
	}}
}

func (f *fakeVoiceRepository) GetByID(_ context.Context, id string) (domain.Voice, error) {
	voice, ok := f.byID[id]
	if !ok {
		return domain.Voice{}, domain.ErrNotFound
	}
	return voice, nil
}

func (f *fakeVoiceRepository) List(_ context.Context) ([]domain.Voice, error) {
	result := make([]domain.Voice, 0, len(f.byID))
	for _, voice := range f.byID {
		result = append(result, voice)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

// fakeDiagramRepository is a minimal in-memory ports.DiagramRepository.
// With knowledge set, GetByID joins each skill and concept in from it, as the
// real repository does; otherwise they stay as stored.
type fakeDiagramRepository struct {
	mu        sync.Mutex
	byID      map[string]domain.Diagram
	knowledge *fakeKnowledgeNodeRepository
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
	if f.knowledge != nil {
		diagram.Skills = f.knowledge.joined(diagram.Skills)
		diagram.Concepts = f.knowledge.joined(diagram.Concepts)
	}
	return diagram, nil
}

func (f *fakeDiagramRepository) GetByIDs(ctx context.Context, ids []string) (map[string]domain.Diagram, error) {
	found := map[string]domain.Diagram{}
	for _, id := range ids {
		if d, err := f.GetByID(ctx, id); err == nil {
			found[id] = d
		}
	}
	return found, nil
}

func (f *fakeDiagramRepository) List(_ context.Context, filter domain.DiagramListFilter, page domain.PageRequest) (domain.Page[domain.Diagram], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	matched := []domain.Diagram{}
	for _, diagram := range f.byID {
		if filter.Matches(diagram) {
			matched = append(matched, diagram)
		}
	}
	sort.Slice(matched, func(i, j int) bool {
		a, b := matched[i].Names.Resolve(filter.Locale), matched[j].Names.Resolve(filter.Locale)
		if a != b {
			return a < b
		}
		return matched[i].ID < matched[j].ID
	})
	return paginate(matched, page), nil
}

func (f *fakeDiagramRepository) ListCreatorIDs(_ context.Context, filter domain.DiagramListFilter) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	seen := map[string]bool{}
	ids := []string{}
	for _, diagram := range f.byID {
		if filter.Matches(diagram) && !seen[diagram.CreatedBy] {
			seen[diagram.CreatedBy] = true
			ids = append(ids, diagram.CreatedBy)
		}
	}
	return ids, nil
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

// fakePracticeReferenceWriter is an in-memory ports.PracticeReferenceWriter:
// it keeps the latest diagram reference per id and counts the writes, and
// fails every write with err when set. Like a real store, it refuses a
// write whose context is already done.
type fakePracticeReferenceWriter struct {
	mu        sync.Mutex
	diagrams  map[string]domain.DiagramReference
	exercises map[string]domain.ExerciseReference
	instrs    map[string]domain.InstrumentReference
	threshold []domain.DrillThreshold
	writes    int
	err       error
}

func newFakePracticeReferenceWriter() *fakePracticeReferenceWriter {
	return &fakePracticeReferenceWriter{diagrams: map[string]domain.DiagramReference{}, exercises: map[string]domain.ExerciseReference{}, instrs: map[string]domain.InstrumentReference{}}
}

func (f *fakePracticeReferenceWriter) PutExercises(ctx context.Context, refs []domain.ExerciseReference) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if f.err != nil {
		return f.err
	}
	f.writes++
	for _, ref := range refs {
		f.exercises[ref.ID] = ref
	}
	return nil
}

func (f *fakePracticeReferenceWriter) PutDrillThresholds(ctx context.Context, thresholds []domain.DrillThreshold) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if f.err != nil {
		return f.err
	}
	f.writes++
	f.threshold = append(f.threshold, thresholds...)
	return nil
}

func (f *fakePracticeReferenceWriter) drillThresholds() []domain.DrillThreshold {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.threshold
}

// fakeDrillThresholds is a ports.DrillThresholdRepository holding the given
// installed versions.
type fakeDrillThresholds []domain.DrillThreshold

func (f fakeDrillThresholds) List(context.Context) ([]domain.DrillThreshold, error) {
	return f, nil
}

func (f *fakePracticeReferenceWriter) exercise(id string) (domain.ExerciseReference, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ref, ok := f.exercises[id]
	return ref, ok
}

func (f *fakePracticeReferenceWriter) PutDiagrams(ctx context.Context, refs []domain.DiagramReference) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if f.err != nil {
		return f.err
	}
	f.writes++
	for _, ref := range refs {
		f.diagrams[ref.ID] = ref
	}
	return nil
}

func (f *fakePracticeReferenceWriter) PutInstruments(ctx context.Context, refs []domain.InstrumentReference) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if f.err != nil {
		return f.err
	}
	f.writes++
	for _, ref := range refs {
		f.instrs[ref.ID] = ref
	}
	return nil
}

func (f *fakePracticeReferenceWriter) instrument(id string) (domain.InstrumentReference, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ref, ok := f.instrs[id]
	return ref, ok
}

func (f *fakePracticeReferenceWriter) writeCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.writes
}

func (f *fakePracticeReferenceWriter) diagram(id string) (domain.DiagramReference, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ref, ok := f.diagrams[id]
	return ref, ok
}

// fakeChordCatalogRepository holds chords with their active voicings already
// ranked, as the catalog installs them.
//
// voicings, when set, holds every voicing by id, withdrawn ones included; a
// chord is then read with only the voicings it marks active.
type fakeChordCatalogRepository struct {
	chords   []domain.ChordDefinition
	voicings map[string]domain.ChordVoicing
}

func (f *fakeChordCatalogRepository) GetChord(_ context.Context, id string) (domain.ChordDefinition, error) {
	for _, c := range f.chords {
		if c.ID == id {
			return f.activeOnly(c), nil
		}
	}
	return domain.ChordDefinition{}, domain.ErrNotFound
}

func (f *fakeChordCatalogRepository) GetChords(ctx context.Context, ids []string) (map[string]domain.ChordDefinition, error) {
	found := map[string]domain.ChordDefinition{}
	for _, id := range ids {
		if c, err := f.GetChord(ctx, id); err == nil {
			found[id] = c
		}
	}
	return found, nil
}

func (f *fakeChordCatalogRepository) GetVoicings(_ context.Context, ids []string) (map[string]domain.ChordVoicing, error) {
	found := map[string]domain.ChordVoicing{}
	for _, id := range ids {
		if v, ok := f.voicings[id]; ok {
			found[id] = v
		}
	}
	return found, nil
}

// withdraw marks a voicing withdrawn, as the catalog would.
func (f *fakeChordCatalogRepository) withdraw(id string) {
	v := f.voicings[id]
	v.Status = domain.ChordVoicingWithdrawn
	f.voicings[id] = v
}

func (f *fakeChordCatalogRepository) activeOnly(c domain.ChordDefinition) domain.ChordDefinition {
	if f.voicings == nil {
		return c
	}
	var active []domain.ChordVoicing
	for _, v := range c.Voicings {
		if f.voicings[v.ID].Status == domain.ChordVoicingActive {
			active = append(active, v)
		}
	}
	c.Voicings = active
	return c
}

func (f *fakeChordCatalogRepository) FindChord(_ context.Context, rootPitchClass int, quality domain.ChordQuality, bassPitchClass *int) (domain.ChordDefinition, error) {
	for _, c := range f.chords {
		sameBass := (c.BassPitchClass == nil && bassPitchClass == nil) ||
			(c.BassPitchClass != nil && bassPitchClass != nil && *c.BassPitchClass == *bassPitchClass)
		if c.RootPitchClass == rootPitchClass && c.Quality == quality && sameBass {
			return f.activeOnly(c), nil
		}
	}
	return domain.ChordDefinition{}, domain.ErrNotFound
}

// fakeSongChartRepository is an in-memory ports.SongChartRepository.
type fakeSongChartRepository struct {
	mu        sync.Mutex
	charts    map[string]domain.SongChart
	revisions map[string][]domain.SongChartRevision
}

func newFakeSongChartRepository() *fakeSongChartRepository {
	return &fakeSongChartRepository{charts: map[string]domain.SongChart{}, revisions: map[string][]domain.SongChartRevision{}}
}

func (f *fakeSongChartRepository) Create(_ context.Context, chart domain.SongChart) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.charts[chart.ID] = chart
	return nil
}

func (f *fakeSongChartRepository) GetByID(_ context.Context, id string) (domain.SongChart, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	chart, ok := f.charts[id]
	if !ok {
		return domain.SongChart{}, domain.ErrNotFound
	}
	return chart, nil
}

func (f *fakeSongChartRepository) List(_ context.Context, filter domain.SongChartFilter, page domain.PageRequest) (domain.Page[domain.SongChart], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var matched []domain.SongChart
	for _, c := range f.charts {
		if filter.Status != nil && c.Status != *filter.Status {
			continue
		}
		if filter.Q != "" && !chartMatches(c, filter.Q) {
			continue
		}
		matched = append(matched, c)
	}
	sort.Slice(matched, func(i, j int) bool {
		if !matched[i].Draft.UpdatedAt.Equal(matched[j].Draft.UpdatedAt) {
			return matched[i].Draft.UpdatedAt.After(matched[j].Draft.UpdatedAt)
		}
		return matched[i].ID < matched[j].ID
	})
	return paginate(matched, page), nil
}

// chartMatches is whether q is in the draft's title or artist, or the
// published revision's.
func chartMatches(c domain.SongChart, q string) bool {
	if containsFold(c.Draft.Title, q) || containsFold(c.Draft.Artist, q) {
		return true
	}
	p := c.PublishedRevision
	return p != nil && (containsFold(p.Title, q) || containsFold(p.Artist, q))
}

func (f *fakeSongChartRepository) Save(_ context.Context, chart domain.SongChart) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.charts[chart.ID]; !ok {
		return domain.ErrNotFound
	}
	f.charts[chart.ID] = chart
	return nil
}

func (f *fakeSongChartRepository) Publish(_ context.Context, chart domain.SongChart, rev domain.SongChartRevision) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.charts[chart.ID] = chart
	f.revisions[chart.ID] = append([]domain.SongChartRevision{rev}, f.revisions[chart.ID]...)
	return nil
}

func (f *fakeSongChartRepository) ListRevisions(_ context.Context, chartID string) ([]domain.SongChartRevision, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.SongChartRevision(nil), f.revisions[chartID]...), nil
}

func (f *fakeSongChartRepository) GetRevision(_ context.Context, chartID string, number int) (domain.SongChartRevision, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, rev := range f.revisions[chartID] {
		if rev.Number == number {
			return rev, nil
		}
	}
	return domain.SongChartRevision{}, domain.ErrNotFound
}
