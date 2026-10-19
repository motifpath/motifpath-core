package application

import (
	"cmp"
	"context"
	"slices"
	"time"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// AlphaObservationService measures the alpha — every student given at
// least one path — a week at a time and one student at a time. Reading it
// changes nothing, and only admins may read it.
type AlphaObservationService struct {
	users        ports.UserRepository
	studentPaths ports.StudentPathRepository
	contentNodes ports.ContentNodeRepository
	completion   ports.CompletionStateReader
	activity     ports.AlphaActivityReader
	now          func() time.Time
}

func NewAlphaObservationService(
	users ports.UserRepository,
	studentPaths ports.StudentPathRepository,
	contentNodes ports.ContentNodeRepository,
	completion ports.CompletionStateReader,
	activity ports.AlphaActivityReader,
	now func() time.Time,
) *AlphaObservationService {
	return &AlphaObservationService{
		users: users, studentPaths: studentPaths, contentNodes: contentNodes,
		completion: completion, activity: activity, now: now,
	}
}

// AlphaObservation is the alpha measured for one week.
type AlphaObservation struct {
	Week    domain.AlphaWeek
	Running bool
	// Summary and FilterCounts describe the whole alpha, whatever the
	// filter and the page.
	Summary      domain.AlphaCohortSummary
	FilterCounts domain.AlphaFilterCounts
	// Students is the page of the students the filter keeps, in
	// domain.NewAlphaStudentOrder's order; StudentsTotal counts all of them.
	Students      []AlphaStudentView
	StudentsTotal int
	Page          domain.PageRequest
}

// AlphaStudentRef names one student of the alpha.
type AlphaStudentRef struct {
	ID          string
	DisplayName string
}

// AlphaStudentView is one student of the alpha with what their measures
// were taken from.
type AlphaStudentView struct {
	Student      domain.AlphaStudent
	RegisteredAt time.Time
	// LastActiveAt is nil when the student has no activity at all.
	LastActiveAt     *time.Time
	FirstPathID      string
	FirstPathTitle   string
	FirstPathGivenAt time.Time
}

// AlphaStudentObservation is one student measured for one week, with the
// sessions and first-path nodes behind the measures.
type AlphaStudentObservation struct {
	Week    domain.AlphaWeek
	Running bool
	Student AlphaStudentView
	// SessionWeeks are the measured week, then the week before it.
	SessionWeeks []AlphaSessionWeek
	// Nodes are the first path's nodes in path order.
	Nodes []AlphaPathNode
}

// AlphaSessionWeek is one week of a student's sessions, newest first.
type AlphaSessionWeek struct {
	Week     domain.AlphaWeek
	Running  bool
	Finished int
	Sessions []AlphaSession
}

// AlphaSession is one practice session as it stands now.
type AlphaSession struct {
	Record domain.PracticeSessionRecord
	State  domain.PracticeSessionState
	// EndedAt is nil while the session is in progress.
	EndedAt *time.Time
}

// AlphaNodeStatus is where a student is with one node of their first path.
type AlphaNodeStatus string

const (
	AlphaNodeCompleted  AlphaNodeStatus = "completed"
	AlphaNodeInProgress AlphaNodeStatus = "in_progress"
	AlphaNodeNotStarted AlphaNodeStatus = "not_started"
)

// AlphaPathNode is one node of a student's first path.
type AlphaPathNode struct {
	Position      int
	ContentNodeID string
	Title         string
	Status        AlphaNodeStatus
	// CompletedAt is set only when completed; StartedAt when started.
	CompletedAt *time.Time
	StartedAt   *time.Time
}

// alphaMember is a student of the alpha with their first path.
type alphaMember struct {
	user      domain.User
	firstPath domain.StudentPath
}

// Observe measures the alpha for week, the Monday it starts on (nil: the
// running week), and returns page of the students filter keeps. Every
// student is measured, since the summary and the filter counts describe the
// whole alpha.
func (s *AlphaObservationService) Observe(ctx context.Context, caller domain.User, week *time.Time, filter domain.AlphaStudentFilter, page domain.PageRequest) (AlphaObservation, error) {
	if caller.Role != domain.RoleAdmin {
		return AlphaObservation{}, domain.ErrForbidden
	}
	if !filter.Keep.Valid() {
		return AlphaObservation{}, domain.NewValidationError("filter", "must be one of: below_target, quiet")
	}
	now := s.now()
	measured, err := domain.ParseAlphaWeek(week, now)
	if err != nil {
		return AlphaObservation{}, err
	}
	members, err := s.alphaMembers(ctx)
	if err != nil {
		return AlphaObservation{}, err
	}
	views, err := s.measure(ctx, members, measured, now)
	if err != nil {
		return AlphaObservation{}, err
	}

	students := make([]domain.AlphaStudent, len(views))
	for i, v := range views {
		students[i] = v.Student
	}
	order := domain.NewAlphaStudentOrder()
	slices.SortFunc(views, func(a, b AlphaStudentView) int { return order(a.Student, b.Student) })
	kept := slices.DeleteFunc(views, func(v AlphaStudentView) bool { return !filter.Matches(v.Student) })
	return AlphaObservation{
		Week:          measured,
		Running:       measured.IsRunning(now),
		Summary:       domain.SummarizeAlpha(students),
		FilterCounts:  domain.CountAlphaFilters(students),
		Students:      window(kept, page),
		StudentsTotal: len(kept),
		Page:          page,
	}, nil
}

// ListStudents names the students of the alpha whose display name contains
// name (ignoring case and accents) and, when studentIDs is given, who are
// among them; ids outside the alpha are left out. Nobody is measured. They
// come alphabetically as a person reads names, then by id.
func (s *AlphaObservationService) ListStudents(ctx context.Context, caller domain.User, name string, studentIDs []string, page domain.PageRequest) (domain.Page[AlphaStudentRef], error) {
	if caller.Role != domain.RoleAdmin {
		return domain.Page[AlphaStudentRef]{}, domain.ErrForbidden
	}
	if len(studentIDs) > domain.MaxPageLimit {
		return domain.Page[AlphaStudentRef]{}, domain.NewValidationError("student_id", "must name at most 100 students")
	}
	members, err := s.alphaMembers(ctx)
	if err != nil {
		return domain.Page[AlphaStudentRef]{}, err
	}
	refs := []AlphaStudentRef{}
	for _, m := range members {
		if len(studentIDs) > 0 && !slices.Contains(studentIDs, m.user.ID) {
			continue
		}
		if name != "" && !domain.ContainsLoosely(m.user.DisplayName, name) {
			continue
		}
		refs = append(refs, AlphaStudentRef{ID: m.user.ID, DisplayName: m.user.DisplayName})
	}
	names := collate.New(language.BrazilianPortuguese)
	slices.SortFunc(refs, func(a, b AlphaStudentRef) int {
		return cmp.Or(names.CompareString(a.DisplayName, b.DisplayName), cmp.Compare(a.ID, b.ID))
	})
	return domain.Page[AlphaStudentRef]{Items: window(refs, page), Total: len(refs)}, nil
}

// window is the part of items page asks for; past the end it is empty.
func window[T any](items []T, page domain.PageRequest) []T {
	start := min(page.Offset, len(items))
	end := min(start+page.Limit, len(items))
	return slices.Clone(items[start:end])
}

// ObserveStudent measures studentID for week, as Observe does, with their
// sessions of the week and the week before and their first path node by
// node. A user who isn't in the alpha is not found.
func (s *AlphaObservationService) ObserveStudent(ctx context.Context, caller domain.User, studentID string, week *time.Time) (AlphaStudentObservation, error) {
	if caller.Role != domain.RoleAdmin {
		return AlphaStudentObservation{}, domain.ErrForbidden
	}
	now := s.now()
	measured, err := domain.ParseAlphaWeek(week, now)
	if err != nil {
		return AlphaStudentObservation{}, err
	}
	member, err := s.alphaMember(ctx, studentID)
	if err != nil {
		return AlphaStudentObservation{}, err
	}
	views, err := s.measure(ctx, []alphaMember{member}, measured, now)
	if err != nil {
		return AlphaStudentObservation{}, err
	}
	sessions, err := s.activity.SessionsStartedBetween(ctx, []string{studentID}, measured.Previous().Start(), measured.End())
	if err != nil {
		return AlphaStudentObservation{}, err
	}
	nodes, err := s.pathNodes(ctx, member)
	if err != nil {
		return AlphaStudentObservation{}, err
	}
	return AlphaStudentObservation{
		Week:    measured,
		Running: measured.IsRunning(now),
		Student: views[0],
		SessionWeeks: []AlphaSessionWeek{
			sessionWeek(sessions, measured, now),
			sessionWeek(sessions, measured.Previous(), now),
		},
		Nodes: nodes,
	}, nil
}

// alphaMembers is every student holding a path, with their first one.
func (s *AlphaObservationService) alphaMembers(ctx context.Context) ([]alphaMember, error) {
	students, err := s.users.ListByRole(ctx, domain.RoleStudent)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(students))
	for i, u := range students {
		ids[i] = u.ID
	}
	firstPaths, err := s.studentPaths.FirstGivenByStudentIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	var members []alphaMember
	for _, u := range students {
		if path, ok := firstPaths[u.ID]; ok {
			members = append(members, alphaMember{user: u, firstPath: path})
		}
	}
	return members, nil
}

// alphaMember is studentID with their first path, or not found when they
// aren't a student holding a path.
func (s *AlphaObservationService) alphaMember(ctx context.Context, studentID string) (alphaMember, error) {
	user, err := s.users.GetByID(ctx, studentID)
	if err != nil {
		return alphaMember{}, err
	}
	if user.Role != domain.RoleStudent {
		return alphaMember{}, domain.ErrNotFound
	}
	firstPaths, err := s.studentPaths.FirstGivenByStudentIDs(ctx, []string{studentID})
	if err != nil {
		return alphaMember{}, err
	}
	path, ok := firstPaths[studentID]
	if !ok {
		return alphaMember{}, domain.ErrNotFound
	}
	return alphaMember{user: user, firstPath: path}, nil
}

// measure measures each member for week. Sessions are read once for all of
// them, from the earliest week any of their measures looks at.
func (s *AlphaObservationService) measure(ctx context.Context, members []alphaMember, week domain.AlphaWeek, now time.Time) ([]AlphaStudentView, error) {
	if len(members) == 0 {
		return []AlphaStudentView{}, nil
	}
	ids := make([]string, len(members))
	from := week.Previous().Start()
	for i, m := range members {
		ids[i] = m.user.ID
		if first := domain.AlphaWeekOf(m.firstPath.AssignedAt).Start(); first.Before(from) {
			from = first
		}
	}
	// Only the counts of finished sessions per week are read, never the
	// sessions themselves: the alpha's whole history would be too much to
	// load on every page.
	finished, err := s.activity.FinishedByWeek(ctx, ids, from, domain.AlphaWeekOf(now).End())
	if err != nil {
		return nil, err
	}
	lastActive, err := s.activity.LastActive(ctx, ids)
	if err != nil {
		return nil, err
	}
	nodesByStudent := make(map[string][]string, len(members))
	for _, m := range members {
		nodesByStudent[m.user.ID] = pathNodeIDs(m.firstPath)
	}
	statuses, err := s.completion.GetStatusesForStudents(ctx, nodesByStudent)
	if err != nil {
		return nil, err
	}

	views := make([]AlphaStudentView, len(members))
	for i, m := range members {
		progress := firstPathProgress(m.firstPath, statuses[m.user.ID])
		activity := domain.AlphaStudentActivity{
			FirstPathGivenAt: m.firstPath.AssignedAt,
			Finished:         finished[m.user.ID],
		}
		if at, ok := lastActive[m.user.ID]; ok {
			activity.LastActiveAt = &at
		}
		views[i] = AlphaStudentView{
			Student: domain.AlphaStudent{
				StudentID: m.user.ID, DisplayName: m.user.DisplayName, FirstPath: progress,
				Measure: domain.MeasureAlphaStudent(activity, week, now),
			},
			RegisteredAt:     m.user.RegisteredAt,
			LastActiveAt:     activity.LastActiveAt,
			FirstPathID:      m.firstPath.ID,
			FirstPathTitle:   m.firstPath.Title,
			FirstPathGivenAt: m.firstPath.AssignedAt,
		}
	}
	return views, nil
}

// firstPathProgress counts path's completed nodes among statuses, the
// student's statuses for its nodes; a node completed in any path counts.
func firstPathProgress(path domain.StudentPath, statuses map[string]domain.CompletionStatus) domain.FirstPathProgress {
	progress := domain.FirstPathProgress{Total: len(path.Items)}
	for _, item := range path.Items {
		if statuses[item.ContentNodeID] == domain.CompletionStatusCompleted {
			progress.Completed++
		}
	}
	return progress
}

// pathNodes lists the first path's nodes in path order with where the
// student is with each. A node the student can't open yet has no status of
// its own, so it is not started.
func (s *AlphaObservationService) pathNodes(ctx context.Context, m alphaMember) ([]AlphaPathNode, error) {
	ids := pathNodeIDs(m.firstPath)
	nodes, err := s.contentNodes.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	statuses, err := s.completion.GetStatuses(ctx, m.user.ID, ids)
	if err != nil {
		return nil, err
	}
	completions, err := s.activity.FirstCompletions(ctx, m.user.ID, ids)
	if err != nil {
		return nil, err
	}
	starts, err := s.activity.FirstStarts(ctx, m.user.ID, ids)
	if err != nil {
		return nil, err
	}

	items := slices.Clone(m.firstPath.Items)
	slices.SortFunc(items, func(a, b domain.StudentPathItemRecord) int { return cmp.Compare(a.Position, b.Position) })
	out := make([]AlphaPathNode, len(items))
	for i, item := range items {
		node := AlphaPathNode{
			Position: item.Position, ContentNodeID: item.ContentNodeID,
			Title: nodes[item.ContentNodeID].Title, Status: AlphaNodeNotStarted,
		}
		switch statuses[item.ContentNodeID] {
		case domain.CompletionStatusCompleted:
			node.Status = AlphaNodeCompleted
			node.CompletedAt = timeIn(completions, item.ContentNodeID)
			node.StartedAt = timeIn(starts, item.ContentNodeID)
		case domain.CompletionStatusInProgress:
			node.Status = AlphaNodeInProgress
			node.StartedAt = timeIn(starts, item.ContentNodeID)
		case domain.CompletionStatusNotStarted, domain.CompletionStatusLocked:
		}
		out[i] = node
	}
	return out, nil
}

// sessionWeek is week's sessions of any state, newest first, read at now.
func sessionWeek(sessions []domain.PracticeSessionRecord, week domain.AlphaWeek, now time.Time) AlphaSessionWeek {
	out := AlphaSessionWeek{Week: week, Running: week.IsRunning(now), Sessions: []AlphaSession{}}
	for _, r := range sessions {
		if !week.Contains(r.StartedAt) {
			continue
		}
		state, endedAt := r.StateAt(now)
		out.Sessions = append(out.Sessions, AlphaSession{Record: r, State: state, EndedAt: endedAt})
		if r.Finished() {
			out.Finished++
		}
	}
	slices.SortFunc(out.Sessions, func(a, b AlphaSession) int {
		return cmp.Or(b.Record.StartedAt.Compare(a.Record.StartedAt), cmp.Compare(a.Record.ID, b.Record.ID))
	})
	return out
}


func pathNodeIDs(path domain.StudentPath) []string {
	ids := make([]string, len(path.Items))
	for i, item := range path.Items {
		ids[i] = item.ContentNodeID
	}
	return ids
}

func timeIn(times map[string]time.Time, key string) *time.Time {
	if t, ok := times[key]; ok {
		return &t
	}
	return nil
}
