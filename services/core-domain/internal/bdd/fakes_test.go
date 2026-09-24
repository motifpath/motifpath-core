//go:build integration

package bdd

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

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

func (f *fakeUserRepo) UpdateLocale(_ context.Context, id, locale string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	u.Locale = domain.Language{Code: locale}
	f.byID[id] = u
	f.byClerkID[u.ClerkUserID] = u
	return nil
}

// fakeLanguageRepo is pre-seeded with the system rows the Atlas migration
// seeds in production: en, pt_BR, any.
type fakeLanguageRepo struct {
	mu     sync.Mutex
	byCode map[string]domain.Language
}

func newFakeLanguageRepo() *fakeLanguageRepo {
	return &fakeLanguageRepo{byCode: map[string]domain.Language{
		"en":                   {Code: "en", Name: "English"},
		"pt_BR":                {Code: "pt_BR", Name: "Portuguese (Brazil)"},
		domain.LanguageCodeAny: {Code: domain.LanguageCodeAny, Name: "Language-agnostic"},
	}}
}

func (f *fakeLanguageRepo) GetByCode(_ context.Context, code string) (domain.Language, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	lang, ok := f.byCode[code]
	if !ok {
		return domain.Language{}, domain.ErrNotFound
	}
	return lang, nil
}

type fakeContentNodeRepo struct {
	mu       sync.Mutex
	byID     map[string]domain.ContentNode
	skills   *fakeSkillRepo
	concepts *fakeConceptRepo
}

func newFakeContentNodeRepo(skills *fakeSkillRepo, concepts *fakeConceptRepo) *fakeContentNodeRepo {
	return &fakeContentNodeRepo{byID: map[string]domain.ContentNode{}, skills: skills, concepts: concepts}
}

// resolveClassification replaces n.Classification.Skills/Concepts (which
// may carry only IDs, as domain.NewContentNode's placeholders do) with the
// full Skill/Concept records from f.skills/f.concepts — mirroring the real
// ent adapter's WithSkills()/WithConcepts() join on read.
func (f *fakeContentNodeRepo) resolveClassification(n domain.ContentNode) domain.ContentNode {
	skills := make([]domain.Skill, len(n.Classification.Skills))
	for i, s := range n.Classification.Skills {
		if full, ok := f.skills.byID[s.ID]; ok {
			skills[i] = full
		} else {
			skills[i] = s
		}
	}
	concepts := make([]domain.Concept, len(n.Classification.Concepts))
	for i, c := range n.Classification.Concepts {
		if full, ok := f.concepts.byID[c.ID]; ok {
			concepts[i] = full
		} else {
			concepts[i] = c
		}
	}
	n.Classification.Skills = skills
	n.Classification.Concepts = concepts
	return n
}

func (f *fakeContentNodeRepo) Create(_ context.Context, n domain.ContentNode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[n.ID] = f.resolveClassification(n)
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

func (f *fakeContentNodeRepo) List(_ context.Context, filter domain.ContentNodeFilter, page domain.PageRequest) (domain.Page[domain.ContentNode], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []domain.ContentNode
	for _, n := range f.byID {
		if filter.ContentType != "" && n.ContentType != filter.ContentType {
			continue
		}
		if filter.SkillID != "" && !containsID(n.Classification.SkillIDs(), filter.SkillID) {
			continue
		}
		if filter.ConceptID != "" && !containsID(n.Classification.ConceptIDs(), filter.ConceptID) {
			continue
		}
		if filter.Difficulty != "" && n.Classification.DifficultyLevel != filter.Difficulty {
			continue
		}
		if !containsFold(n.Title, filter.Query) {
			continue
		}
		result = append(result, n)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Title != result[j].Title {
			return result[i].Title < result[j].Title
		}
		return result[i].ID < result[j].ID
	})
	return paginate(result, page), nil
}

// containsFold reports whether s contains substr, ignoring case; an empty
// substr always matches.
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

func (f *fakeContentNodeRepo) Update(_ context.Context, n domain.ContentNode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[n.ID]; !ok {
		return domain.ErrNotFound
	}
	f.byID[n.ID] = f.resolveClassification(n)
	return nil
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

func (f *fakeChallengeRepo) ListByContentNodeID(_ context.Context, contentNodeID string) ([]domain.Challenge, error) {
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

func (f *fakeChallengeRepo) Update(_ context.Context, c domain.Challenge) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[c.ID]; !ok {
		return domain.ErrNotFound
	}
	f.byID[c.ID] = c
	return nil
}

// fakeExerciseRepo tracks link order per challenge/node separately from
// each exercise's own ChallengeIDs/ContentNodeIDs slice, since two
// different challenges/nodes can share exercises linked in different
// orders — needed for scenarios asserting stable or shuffled order.
type fakeExerciseRepo struct {
	mu               sync.Mutex
	byID             map[string]domain.Exercise
	byChallengeOrder map[string][]string
	byNodeOrder      map[string][]string
	skills           *fakeSkillRepo
	concepts         *fakeConceptRepo
}

func newFakeExerciseRepo(skills *fakeSkillRepo, concepts *fakeConceptRepo) *fakeExerciseRepo {
	return &fakeExerciseRepo{
		byID:             map[string]domain.Exercise{},
		byChallengeOrder: map[string][]string{},
		byNodeOrder:      map[string][]string{},
		skills:           skills,
		concepts:         concepts,
	}
}

// resolveClassification is fakeContentNodeRepo.resolveClassification's
// counterpart for Exercise.
func (f *fakeExerciseRepo) resolveClassification(e domain.Exercise) domain.Exercise {
	skills := make([]domain.Skill, len(e.Skills))
	for i, s := range e.Skills {
		if full, ok := f.skills.byID[s.ID]; ok {
			skills[i] = full
		} else {
			skills[i] = s
		}
	}
	concepts := make([]domain.Concept, len(e.Concepts))
	for i, c := range e.Concepts {
		if full, ok := f.concepts.byID[c.ID]; ok {
			concepts[i] = full
		} else {
			concepts[i] = c
		}
	}
	e.Skills = skills
	e.Concepts = concepts
	return e
}

func (f *fakeExerciseRepo) Create(_ context.Context, e domain.Exercise) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[e.ID] = f.resolveClassification(e)
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

func (f *fakeExerciseRepo) LinkChallenge(_ context.Context, exerciseID, challengeID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.byID[exerciseID]
	if !ok {
		return domain.ErrNotFound
	}
	e.ChallengeIDs = append(e.ChallengeIDs, challengeID)
	f.byID[exerciseID] = e
	f.byChallengeOrder[challengeID] = append(f.byChallengeOrder[challengeID], exerciseID)
	return nil
}

func (f *fakeExerciseRepo) UnlinkChallenge(_ context.Context, exerciseID, challengeID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.byID[exerciseID]
	if !ok {
		return domain.ErrNotFound
	}
	remaining := make([]string, 0, len(e.ChallengeIDs))
	for _, id := range e.ChallengeIDs {
		if id != challengeID {
			remaining = append(remaining, id)
		}
	}
	e.ChallengeIDs = remaining
	f.byID[exerciseID] = e

	remainingOrder := make([]string, 0, len(f.byChallengeOrder[challengeID]))
	for _, id := range f.byChallengeOrder[challengeID] {
		if id != exerciseID {
			remainingOrder = append(remainingOrder, id)
		}
	}
	f.byChallengeOrder[challengeID] = remainingOrder
	return nil
}

func (f *fakeExerciseRepo) ListByChallengeID(_ context.Context, challengeID string) ([]domain.Exercise, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]domain.Exercise, 0, len(f.byChallengeOrder[challengeID]))
	for _, id := range f.byChallengeOrder[challengeID] {
		result = append(result, f.byID[id])
	}
	return result, nil
}

func (f *fakeExerciseRepo) ListByChallengeIDs(_ context.Context, challengeIDs []string) (map[string][]domain.Exercise, error) {
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

func (f *fakeExerciseRepo) LinkContentNode(_ context.Context, exerciseID, contentNodeID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.byID[exerciseID]
	if !ok {
		return domain.ErrNotFound
	}
	e.ContentNodeIDs = append(e.ContentNodeIDs, contentNodeID)
	f.byID[exerciseID] = e
	f.byNodeOrder[contentNodeID] = append(f.byNodeOrder[contentNodeID], exerciseID)
	return nil
}

func (f *fakeExerciseRepo) UnlinkContentNode(_ context.Context, exerciseID, contentNodeID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.byID[exerciseID]
	if !ok {
		return domain.ErrNotFound
	}
	remaining := make([]string, 0, len(e.ContentNodeIDs))
	for _, id := range e.ContentNodeIDs {
		if id != contentNodeID {
			remaining = append(remaining, id)
		}
	}
	e.ContentNodeIDs = remaining
	f.byID[exerciseID] = e

	remainingOrder := make([]string, 0, len(f.byNodeOrder[contentNodeID]))
	for _, id := range f.byNodeOrder[contentNodeID] {
		if id != exerciseID {
			remainingOrder = append(remainingOrder, id)
		}
	}
	f.byNodeOrder[contentNodeID] = remainingOrder
	return nil
}

func (f *fakeExerciseRepo) ListByContentNodeID(_ context.Context, contentNodeID string) ([]domain.Exercise, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]domain.Exercise, 0, len(f.byNodeOrder[contentNodeID]))
	for _, id := range f.byNodeOrder[contentNodeID] {
		result = append(result, f.byID[id])
	}
	return result, nil
}

func (f *fakeExerciseRepo) ListBySkillID(_ context.Context, skillID string) ([]domain.Exercise, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []domain.Exercise
	for _, e := range f.byID {
		if containsID(exerciseSkillIDs(e), skillID) {
			result = append(result, e)
		}
	}
	return result, nil
}

func (f *fakeExerciseRepo) List(_ context.Context, filter domain.ExerciseFilter, page domain.PageRequest) (domain.Page[domain.Exercise], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []domain.Exercise
	for _, e := range f.byID {
		if filter.ExerciseType != "" && e.ExerciseType != filter.ExerciseType {
			continue
		}
		if filter.SkillID != "" && !containsID(exerciseSkillIDs(e), filter.SkillID) {
			continue
		}
		result = append(result, e)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return paginate(result, page), nil
}

func exerciseSkillIDs(e domain.Exercise) []string {
	ids := make([]string, len(e.Skills))
	for i, s := range e.Skills {
		ids[i] = s.ID
	}
	return ids
}

func (f *fakeExerciseRepo) Update(_ context.Context, e domain.Exercise) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[e.ID]; !ok {
		return domain.ErrNotFound
	}
	f.byID[e.ID] = f.resolveClassification(e)
	return nil
}

// put seeds e directly, also indexing it under its pre-set
// ChallengeIDs/ContentNodeIDs so steps that construct an already-linked
// domain.Exercise literal work with ListByChallengeID/ListByContentNodeID
// without going through LinkChallenge/LinkContentNode first.
func (f *fakeExerciseRepo) put(e domain.Exercise) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[e.ID] = e
	for _, challengeID := range e.ChallengeIDs {
		f.byChallengeOrder[challengeID] = append(f.byChallengeOrder[challengeID], e.ID)
	}
	for _, nodeID := range e.ContentNodeIDs {
		f.byNodeOrder[nodeID] = append(f.byNodeOrder[nodeID], e.ID)
	}
}

func (f *fakeExerciseRepo) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.byID)
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

func (f *fakeExpandedContentRepo) Update(_ context.Context, item domain.ExpandedContent) error {
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

func (f *fakeExpandedContentRepo) Delete(_ context.Context, id string) error {
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

func (f *fakeLearningPathRepo) List(_ context.Context, filter domain.LearningPathFilter, page domain.PageRequest) (domain.Page[domain.LearningPath], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]domain.LearningPath, 0, len(f.byID))
	for _, p := range f.byID {
		if !containsFold(p.Title, filter.Query) {
			continue
		}
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Title != result[j].Title {
			return result[i].Title < result[j].Title
		}
		return result[i].ID < result[j].ID
	})
	return paginate(result, page), nil
}

func (f *fakeLearningPathRepo) Replace(_ context.Context, p domain.LearningPath) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[p.ID]; !ok {
		return domain.ErrNotFound
	}
	f.byID[p.ID] = p
	return nil
}

func (f *fakeLearningPathRepo) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[id]; !ok {
		return domain.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

type fakeCourseRepo struct {
	mu   sync.Mutex
	byID map[string]domain.Course

	// paths, nodes and versions let List resolve the transitive
	// checkpoint -> learning path -> content node classification join and
	// the latest published version, as the real repository does in SQL.
	paths    *fakeLearningPathRepo
	nodes    *fakeContentNodeRepo
	versions *fakeCourseVersionRepo
}

func newFakeCourseRepo(paths *fakeLearningPathRepo, nodes *fakeContentNodeRepo, versions *fakeCourseVersionRepo) *fakeCourseRepo {
	return &fakeCourseRepo{byID: map[string]domain.Course{}, paths: paths, nodes: nodes, versions: versions}
}

func (f *fakeCourseRepo) Create(_ context.Context, c domain.Course) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[c.ID] = c
	return nil
}

func (f *fakeCourseRepo) GetByID(_ context.Context, id string) (domain.Course, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.byID[id]
	if !ok {
		return domain.Course{}, domain.ErrNotFound
	}
	return c, nil
}

func (f *fakeCourseRepo) List(_ context.Context, filter domain.CourseListFilter, page domain.PageRequest) (domain.Page[domain.Course], error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	type candidate struct {
		course domain.Course
		title  string
	}
	var matches []candidate
	for _, c := range f.byID {
		view := courseView{title: c.Title, summary: c.Summary, level: c.Level}
		checkpointPaths := make([]string, len(c.Checkpoints))
		for i, cp := range c.Checkpoints {
			checkpointPaths[i] = cp.LearningPathID
		}
		if filter.PublishedView {
			latest, ok := f.versions.latest(c.ID)
			if !ok {
				continue
			}
			view = courseView{title: latest.TitleSnapshot, summary: latest.SummarySnapshot, level: latest.LevelSnapshot}
			checkpointPaths = checkpointPaths[:0]
			for _, cp := range latest.Checkpoints {
				checkpointPaths = append(checkpointPaths, cp.LearningPathID)
			}
		}

		if filter.Status != nil && c.Status != *filter.Status {
			continue
		}
		if filter.CreatedBy != "" && c.CreatedBy != filter.CreatedBy {
			continue
		}
		if len(filter.Levels) > 0 && !containsLevel(filter.Levels, view.level) {
			continue
		}
		if !containsFold(view.title, filter.Query) && !containsFold(view.summary, filter.Query) {
			continue
		}
		if !f.classified(checkpointPaths, filter.SkillIDs, filter.ConceptIDs) {
			continue
		}
		matches = append(matches, candidate{course: c, title: view.title})
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].title != matches[j].title {
			return matches[i].title < matches[j].title
		}
		return matches[i].course.ID < matches[j].course.ID
	})
	courses := make([]domain.Course, len(matches))
	for i, m := range matches {
		courses[i] = m.course
	}
	return paginate(courses, page), nil
}

// courseView is the text and level a course's filters and ordering read —
// the live draft's, or the latest published version's snapshot.
type courseView struct {
	title, summary string
	level          domain.DifficultyLevel
}

func containsLevel(levels []domain.DifficultyLevel, level domain.DifficultyLevel) bool {
	for _, l := range levels {
		if l == level {
			return true
		}
	}
	return false
}

// classified reports whether some content node in one of pathIDs' learning
// paths is linked to any of skillIDs and — when both are given — any of
// conceptIDs. True when neither is given.
func (f *fakeCourseRepo) classified(pathIDs, skillIDs, conceptIDs []string) bool {
	if len(skillIDs) == 0 && len(conceptIDs) == 0 {
		return true
	}
	for _, pathID := range pathIDs {
		path, ok := f.paths.byID[pathID]
		if !ok {
			continue
		}
		for _, item := range path.Items {
			node, ok := f.nodes.byID[item.ContentNodeID]
			if !ok {
				continue
			}
			if len(skillIDs) > 0 && !overlaps(node.Classification.SkillIDs(), skillIDs) {
				continue
			}
			if len(conceptIDs) > 0 && !overlaps(node.Classification.ConceptIDs(), conceptIDs) {
				continue
			}
			return true
		}
	}
	return false
}

func overlaps(have, want []string) bool {
	for _, id := range want {
		if containsID(have, id) {
			return true
		}
	}
	return false
}

func (f *fakeCourseRepo) Replace(_ context.Context, c domain.Course) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[c.ID]; !ok {
		return domain.ErrNotFound
	}
	f.byID[c.ID] = c
	return nil
}

func (f *fakeCourseRepo) put(c domain.Course) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[c.ID] = c
}

func (f *fakeCourseRepo) UpdateStatus(_ context.Context, id string, status domain.CourseStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	c.Status = status
	f.byID[id] = c
	return nil
}

type fakeCourseVersionRepo struct {
	mu       sync.Mutex
	byCourse map[string][]domain.CourseVersion
}

func newFakeCourseVersionRepo() *fakeCourseVersionRepo {
	return &fakeCourseVersionRepo{byCourse: map[string][]domain.CourseVersion{}}
}

func (f *fakeCourseVersionRepo) Create(_ context.Context, v domain.CourseVersion) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byCourse[v.CourseID] = append(f.byCourse[v.CourseID], v)
	return nil
}

// latest returns courseID's highest-numbered version, and false if it has
// never been published.
func (f *fakeCourseVersionRepo) latest(courseID string) (domain.CourseVersion, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	versions := f.byCourse[courseID]
	if len(versions) == 0 {
		return domain.CourseVersion{}, false
	}
	latest := versions[0]
	for _, v := range versions[1:] {
		if v.VersionNumber > latest.VersionNumber {
			latest = v
		}
	}
	return latest, true
}

func (f *fakeCourseVersionRepo) GetLatestByCourseID(_ context.Context, courseID string) (domain.CourseVersion, error) {
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

func (f *fakeCourseVersionRepo) GetLatestByCourseIDs(_ context.Context, courseIDs []string) (map[string]domain.CourseVersion, error) {
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

func (f *fakeCourseVersionRepo) IsLearningPathReferenced(_ context.Context, learningPathID string) (bool, error) {
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

type fakeStudentPathRepo struct {
	mu   sync.Mutex
	byID map[string]domain.StudentPath
}

func newFakeStudentPathRepo() *fakeStudentPathRepo {
	return &fakeStudentPathRepo{byID: map[string]domain.StudentPath{}}
}

func (f *fakeStudentPathRepo) Create(_ context.Context, p domain.StudentPath) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[p.ID] = p
	return nil
}

func (f *fakeStudentPathRepo) GetByID(_ context.Context, id string) (domain.StudentPath, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.byID[id]
	if !ok {
		return domain.StudentPath{}, domain.ErrNotFound
	}
	return p, nil
}

func (f *fakeStudentPathRepo) ListActiveStandaloneByStudentID(_ context.Context, studentID string) ([]domain.StudentPath, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []domain.StudentPath
	for _, p := range f.byID {
		if p.StudentID == studentID && p.ArchivedAt == nil && p.SourceCourseEnrollmentID == nil {
			result = append(result, p)
		}
	}
	return result, nil
}

func (f *fakeStudentPathRepo) ListStandaloneByStudentID(_ context.Context, studentID string) ([]domain.StudentPath, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := []domain.StudentPath{}
	for _, p := range f.byID {
		if p.StudentID == studentID && p.SourceCourseEnrollmentID == nil {
			result = append(result, p)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].AssignedAt.After(result[j].AssignedAt) })
	return result, nil
}

func (f *fakeStudentPathRepo) Archive(_ context.Context, id string, archivedAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	p.ArchivedAt = &archivedAt
	f.byID[id] = p
	return nil
}

type fakeContentNodeVersionRepo struct {
	mu     sync.Mutex
	byNode map[string][]domain.ContentNodeVersion
}

func newFakeContentNodeVersionRepo() *fakeContentNodeVersionRepo {
	return &fakeContentNodeVersionRepo{byNode: map[string][]domain.ContentNodeVersion{}}
}

func (f *fakeContentNodeVersionRepo) Create(_ context.Context, v domain.ContentNodeVersion) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byNode[v.ContentNodeID] = append(f.byNode[v.ContentNodeID], v)
	return nil
}

func (f *fakeContentNodeVersionRepo) ListByContentNodeID(_ context.Context, contentNodeID string) ([]domain.ContentNodeVersion, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := append([]domain.ContentNodeVersion{}, f.byNode[contentNodeID]...)
	sort.Slice(result, func(i, j int) bool { return result[i].VersionNumber > result[j].VersionNumber })
	return result, nil
}

func (f *fakeContentNodeVersionRepo) GetLatestByContentNodeID(_ context.Context, contentNodeID string) (domain.ContentNodeVersion, error) {
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

type fakeStudentLearningStateRepo struct {
	mu        sync.Mutex
	byStudent map[string]domain.StudentLearningState
}

func newFakeStudentLearningStateRepo() *fakeStudentLearningStateRepo {
	return &fakeStudentLearningStateRepo{byStudent: map[string]domain.StudentLearningState{}}
}

func (f *fakeStudentLearningStateRepo) GetByStudentID(_ context.Context, studentID string) (domain.StudentLearningState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.byStudent[studentID]
	if !ok {
		return domain.StudentLearningState{}, domain.ErrNotFound
	}
	return s, nil
}

func (f *fakeStudentLearningStateRepo) Upsert(_ context.Context, s domain.StudentLearningState) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byStudent[s.StudentID] = s
	return nil
}

type fakeCourseEnrollmentRepo struct {
	mu   sync.Mutex
	byID map[string]domain.CourseEnrollment
}

func newFakeCourseEnrollmentRepo() *fakeCourseEnrollmentRepo {
	return &fakeCourseEnrollmentRepo{byID: map[string]domain.CourseEnrollment{}}
}

func (f *fakeCourseEnrollmentRepo) Create(_ context.Context, e domain.CourseEnrollment) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[e.ID] = e
	return nil
}

func (f *fakeCourseEnrollmentRepo) GetByID(_ context.Context, id string) (domain.CourseEnrollment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.byID[id]
	if !ok {
		return domain.CourseEnrollment{}, domain.ErrNotFound
	}
	return e, nil
}

func (f *fakeCourseEnrollmentRepo) GetActiveByCourseID(_ context.Context, studentID, courseID string) (domain.CourseEnrollment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range f.byID {
		if e.StudentID == studentID && e.CourseID == courseID && e.IsActive() {
			return e, nil
		}
	}
	return domain.CourseEnrollment{}, domain.ErrNotFound
}

func (f *fakeCourseEnrollmentRepo) ListByStudentID(_ context.Context, studentID string) ([]domain.CourseEnrollment, error) {
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

func (f *fakeCourseEnrollmentRepo) ListActiveByStudentID(_ context.Context, studentID string) ([]domain.CourseEnrollment, error) {
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

func (f *fakeCourseEnrollmentRepo) Abandon(_ context.Context, id string) error {
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

func (f *fakeCourseEnrollmentRepo) AdvanceCheckpoint(_ context.Context, id, studentPathID string, position int) error {
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

func (f *fakeCourseEnrollmentRepo) Complete(_ context.Context, id string) error {
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

// fakePinger backs the readiness probe. It is the one fake here with an
// error knob — the service-health feature is entirely about how the probe
// reports a dependency being reachable or not, so the down state has to be
// injectable. Zero value = reachable.
type fakePinger struct {
	mu  sync.Mutex
	err error
}

func (f *fakePinger) setReachable(reachable bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if reachable {
		f.err = nil
		return
	}
	f.err = errStoreUnreachable
}

func (f *fakePinger) Ping(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.err
}

var errStoreUnreachable = errors.New("store unreachable")

// fakeMediaStorage backs ports.MediaStorage. Presigning always succeeds —
// the media-upload feature is about request validation and authorization,
// not storage-provider failure modes.
type fakeMediaStorage struct{}

func (f *fakeMediaStorage) PresignUpload(_ context.Context, objectKey string, _ domain.MediaContentType) (domain.MediaUploadURL, error) {
	return domain.MediaUploadURL{
		UploadURL: "https://storage.example.com/" + objectKey + "?presigned=1",
		ObjectURL: "https://cdn.example.com/" + objectKey,
		ExpiresAt: fixedNow.Add(15 * time.Minute),
	}, nil
}

type fakeSkillRepo struct {
	mu   sync.Mutex
	byID map[string]domain.Skill
}

func newFakeSkillRepo() *fakeSkillRepo {
	return &fakeSkillRepo{byID: map[string]domain.Skill{}}
}

func (f *fakeSkillRepo) Create(_ context.Context, s domain.Skill) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[s.ID] = s
	return nil
}

func (f *fakeSkillRepo) GetByID(_ context.Context, id string) (domain.Skill, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.byID[id]
	if !ok {
		return domain.Skill{}, domain.ErrNotFound
	}
	return s, nil
}

func (f *fakeSkillRepo) GetByIDs(_ context.Context, ids []string) (map[string]domain.Skill, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := map[string]domain.Skill{}
	for _, id := range ids {
		if s, ok := f.byID[id]; ok {
			result[id] = s
		}
	}
	return result, nil
}

func (f *fakeSkillRepo) List(_ context.Context) ([]domain.Skill, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]domain.Skill, 0, len(f.byID))
	for _, s := range f.byID {
		result = append(result, s)
	}
	return result, nil
}

func (f *fakeSkillRepo) ExistsSibling(_ context.Context, parentID *string, name string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.byID {
		if s.Name == name && samePointerValue(s.ParentID, parentID) {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeSkillRepo) put(s domain.Skill) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[s.ID] = s
}

type fakeConceptRepo struct {
	mu   sync.Mutex
	byID map[string]domain.Concept
}

func newFakeConceptRepo() *fakeConceptRepo {
	return &fakeConceptRepo{byID: map[string]domain.Concept{}}
}

func (f *fakeConceptRepo) Create(_ context.Context, c domain.Concept) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[c.ID] = c
	return nil
}

func (f *fakeConceptRepo) GetByID(_ context.Context, id string) (domain.Concept, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.byID[id]
	if !ok {
		return domain.Concept{}, domain.ErrNotFound
	}
	return c, nil
}

func (f *fakeConceptRepo) GetByIDs(_ context.Context, ids []string) (map[string]domain.Concept, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := map[string]domain.Concept{}
	for _, id := range ids {
		if c, ok := f.byID[id]; ok {
			result[id] = c
		}
	}
	return result, nil
}

func (f *fakeConceptRepo) List(_ context.Context) ([]domain.Concept, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]domain.Concept, 0, len(f.byID))
	for _, c := range f.byID {
		result = append(result, c)
	}
	return result, nil
}

func (f *fakeConceptRepo) ExistsSibling(_ context.Context, parentID *string, name string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.byID {
		if c.Name == name && samePointerValue(c.ParentID, parentID) {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeConceptRepo) put(c domain.Concept) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[c.ID] = c
}

// samePointerValue reports whether a and b are both nil, or both non-nil
// and pointing at equal values.
func samePointerValue(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

type fakeInstrumentRepo struct {
	mu   sync.Mutex
	byID map[string]domain.Instrument
}

func newFakeInstrumentRepo() *fakeInstrumentRepo {
	return &fakeInstrumentRepo{byID: map[string]domain.Instrument{}}
}

func (f *fakeInstrumentRepo) Create(_ context.Context, i domain.Instrument) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[i.ID] = i
	return nil
}

func (f *fakeInstrumentRepo) GetByID(_ context.Context, id string) (domain.Instrument, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	i, ok := f.byID[id]
	if !ok {
		return domain.Instrument{}, domain.ErrNotFound
	}
	return i, nil
}

func (f *fakeInstrumentRepo) List(_ context.Context) ([]domain.Instrument, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]domain.Instrument, 0, len(f.byID))
	for _, i := range f.byID {
		result = append(result, i)
	}
	sort.Slice(result, func(a, b int) bool { return result[a].ID < result[b].ID })
	return result, nil
}

func (f *fakeInstrumentRepo) put(i domain.Instrument) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[i.ID] = i
}

type fakeDiagramRepo struct {
	mu       sync.Mutex
	byID     map[string]domain.Diagram
	skills   *fakeSkillRepo
	concepts *fakeConceptRepo
}

func newFakeDiagramRepo(skills *fakeSkillRepo, concepts *fakeConceptRepo) *fakeDiagramRepo {
	return &fakeDiagramRepo{byID: map[string]domain.Diagram{}, skills: skills, concepts: concepts}
}

// resolveClassification replaces d.Skills/Concepts (id-only until read
// back) with the full records from f.skills/f.concepts — mirroring the real
// ent adapter's WithSkills()/WithConcepts() join on read.
func (f *fakeDiagramRepo) resolveClassification(d domain.Diagram) domain.Diagram {
	skills := make([]domain.Skill, len(d.Skills))
	for i, s := range d.Skills {
		if full, ok := f.skills.byID[s.ID]; ok {
			skills[i] = full
		} else {
			skills[i] = s
		}
	}
	concepts := make([]domain.Concept, len(d.Concepts))
	for i, c := range d.Concepts {
		if full, ok := f.concepts.byID[c.ID]; ok {
			concepts[i] = full
		} else {
			concepts[i] = c
		}
	}
	d.Skills, d.Concepts = skills, concepts
	return d
}

func (f *fakeDiagramRepo) Create(_ context.Context, d domain.Diagram) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[d.ID] = d
	return nil
}

func (f *fakeDiagramRepo) GetByID(_ context.Context, id string) (domain.Diagram, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d, ok := f.byID[id]
	if !ok {
		return domain.Diagram{}, domain.ErrNotFound
	}
	return f.resolveClassification(d), nil
}

func (f *fakeDiagramRepo) List(_ context.Context, instrumentID, skillID, conceptID string) ([]domain.Diagram, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := []domain.Diagram{}
	for _, d := range f.byID {
		if instrumentID != "" && d.InstrumentID != instrumentID {
			continue
		}
		if skillID != "" && !containsID(d.SkillIDs(), skillID) {
			continue
		}
		if conceptID != "" && !containsID(d.ConceptIDs(), conceptID) {
			continue
		}
		result = append(result, f.resolveClassification(d))
	}
	sort.Slice(result, func(a, b int) bool { return result[a].ID < result[b].ID })
	return result, nil
}

func (f *fakeDiagramRepo) Update(_ context.Context, d domain.Diagram) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[d.ID]; !ok {
		return domain.ErrNotFound
	}
	f.byID[d.ID] = d
	return nil
}

func (f *fakeDiagramRepo) put(d domain.Diagram) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[d.ID] = d
}
