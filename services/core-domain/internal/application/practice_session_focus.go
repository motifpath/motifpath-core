package application

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
)

// Today's mix review block takes at most reviewMaxItems items and a
// reviewMaxShareDivisor-th of the focus time: a reminder, not a second
// session.
const (
	reviewMaxItems        = 5
	reviewMaxShareDivisor = 5
)

// candidateNode is the node cand is offered for; "" when none.
func candidateNode(cand practiceCandidate) string {
	if cand.nodeID == nil {
		return ""
	}
	return *cand.nodeID
}

// rungOf is the rung cand trains when picked: its drill template's, the
// way a generated drill item is asked next; a play-along's is playing in
// time.
func (c *composer) rungOf(cand practiceCandidate) domain.PracticeRung {
	item := domain.PracticeSessionItem{Exercise: cand.exercise}
	switch {
	case cand.diagram != nil:
		item.PlayAlong = &domain.PlannedPlayAlong{}
	case cand.drill != nil:
		item.Kind = cand.drill.Kind()
		item.Way = domain.NextDrillWay(cand.drill, cand.state)
	}
	return domain.ItemRung(item, c.templates)
}

// setNewRungs finds the rungs new items of each subject of candidates come
// from, those candidates being the subject's material for the session: see
// domain.SubjectRungs. A subject whose rungs are already found keeps them.
func (c *composer) setNewRungs(candidates []practiceCandidate) {
	material := map[string]map[domain.PracticeRung][]string{}
	states := map[string]domain.PracticeItemState{}
	for _, cand := range candidates {
		for _, subject := range c.rungSubjects(cand) {
			if _, found := c.newRungs[subject]; found || subject == "" {
				continue
			}
			if material[subject] == nil {
				material[subject] = map[domain.PracticeRung][]string{}
			}
			rung := c.rungOf(cand)
			material[subject][rung] = append(material[subject][rung], cand.key())
		}
		if cand.state != nil {
			states[cand.key()] = *cand.state
		}
	}
	for subject, m := range material {
		_, c.newRungs[subject] = domain.SubjectRungs(m, states, c.now)
	}
}

// atNewRung reports whether cand, unseen, may be offered as a new or
// stretch item: it trains one of the rungs its subject's new items come
// from, or its subject has no rungs found.
func (c *composer) atNewRung(cand practiceCandidate) bool {
	rungs, found := c.newRungs[c.subjectOf(cand)]
	return !found || slices.Contains(rungs, c.rungOf(cand))
}

// pickSubjects picks Today's mix subjects from p, in this order until it has
// domain.MaxFocusNodes: the nodes with the most due and weak time, the
// earliest on a tie; the nodes of its new items, in path order; the nodes
// of the items it may review ahead, soonest due first. Stretch nodes may
// complete them later: see stretchSubjects.
func (c *composer) pickSubjects(p pools) {
	var order []string
	seconds := map[string]int{}
	for _, cand := range slices.Concat(p.due, p.weak) {
		node := candidateNode(cand)
		if node == "" {
			continue
		}
		if _, seen := seconds[node]; !seen {
			order = append(order, node)
		}
		seconds[node] += cand.seconds()
	}
	slices.SortStableFunc(order, func(a, b string) int { return cmp.Compare(seconds[b], seconds[a]) })
	for _, cand := range slices.Concat(p.fresh, p.known) {
		if node := candidateNode(cand); node != "" && !slices.Contains(order, node) {
			order = append(order, node)
		}
	}
	c.subjects = order[:min(len(order), domain.MaxFocusNodes)]
}

// subjectPools keeps in p only the items of the subjects, and moves the due
// items of other nodes to the review block.
func (c *composer) subjectPools(p pools) pools {
	of := func(candidates []practiceCandidate, subject bool) []practiceCandidate {
		var kept []practiceCandidate
		for _, cand := range candidates {
			if slices.Contains(c.subjects, candidateNode(cand)) == subject {
				kept = append(kept, cand)
			}
		}
		return kept
	}
	return pools{
		review: of(p.due, false), due: of(p.due, true), weak: of(p.weak, true),
		fresh: of(p.fresh, true), known: of(p.known, true),
	}
}

// stretchFits reports whether a stretch candidate is worth finding: inside
// its drill's window, of a subject or of a node that can still become one,
// and at its subject's new rungs when those are found. Candidates that
// fail it don't use up the time stretch items are found for.
func (c *composer) stretchFits(cand practiceCandidate) bool {
	node := candidateNode(cand)
	return node != "" && c.inWindow(cand) && c.atNewRung(cand) &&
		(slices.Contains(c.subjects, node) || len(c.subjects) < domain.MaxFocusNodes)
}

// stretchSubjects keeps the stretch candidates at their subject's new
// rungs that are of the subjects, a stretch node becoming one, in order,
// while there are fewer than domain.MaxFocusNodes.
func (c *composer) stretchSubjects(stretch []practiceCandidate) []practiceCandidate {
	c.setNewRungs(stretch)
	var kept []practiceCandidate
	for _, cand := range stretch {
		node := candidateNode(cand)
		if node == "" || !c.atNewRung(cand) {
			continue
		}
		if !slices.Contains(c.subjects, node) {
			if len(c.subjects) >= domain.MaxFocusNodes {
				continue
			}
			c.subjects = append(c.subjects, node)
		}
		kept = append(kept, cand)
	}
	return kept
}

// focusNodes names the subjects of the plan of items, described by nodes:
// those an item serves, or, when the student chose the subject, that one
// whatever its items serve. With none, the nodes items serve: see
// domain.ItemsFocus.
func (c *composer) focusNodes(items []domain.PracticeSessionItem, nodes map[string]domain.KnowledgeNode) []domain.KnowledgeNode {
	var focus []domain.KnowledgeNode
	for _, id := range c.subjects {
		served := c.aheadAnyTime || slices.ContainsFunc(items, func(item domain.PracticeSessionItem) bool {
			return item.NodeID != nil && *item.NodeID == id
		})
		if n, ok := nodes[id]; ok && served {
			focus = append(focus, n)
		}
	}
	if len(focus) == 0 {
		return domain.ItemsFocus(items, nodes)
	}
	return focus
}

// composeChosen composes with c a session on the one subject the student
// chose, nodeID, for the instruments the session covers, instrumentIDs,
// whose knowledge is in maps:
//   - it holds only items of the node and the nodes part_of it, at any
//     depth, on the student's paths or not, whatever it requires;
//   - it has no warm-up, and, with the instrument in hand, a session of 10
//     minutes or more ends with a play-along of the subtree, if it has one;
//   - the focus time goes to its due, weak and new items as in Today's mix,
//     with no review block; the time left goes to reviewing its known items
//     ahead, however far off they fall due, soonest first, then to more of
//     its new items, never to anything else, so it ends early when the
//     subject runs out;
//   - its new items come from its current rung and the next one.
//
// A node that doesn't exist for the session, or has nothing to practise in
// it, is not found.
func (s *PracticeSessionService) composeChosen(ctx context.Context, c *composer, studentID string, instrumentID *string, instrumentIDs []string, maps []KnowledgeMap, nodeID string) (domain.PracticeSessionPlan, error) {
	subtree := subtreeNodeIDs(maps, nodeID)
	if len(subtree) == 0 {
		return domain.PracticeSessionPlan{}, fmt.Errorf("%w: no knowledge node exists with the given focus_node_id", domain.ErrNotFound)
	}
	inHand := instrumentID != nil
	candidates, err := s.sessionCandidates(ctx, maps, instrumentIDs, subtree, inHand)
	if err != nil {
		return domain.PracticeSessionPlan{}, err
	}
	if len(candidates) == 0 {
		return domain.PracticeSessionPlan{}, fmt.Errorf("%w: the chosen node has nothing to practise in this session", domain.ErrNotFound)
	}

	c.subjects = []string{nodeID}
	c.subjectOf = func(practiceCandidate) string { return nodeID }
	c.chosenNode = nodeID
	c.aheadAnyTime = true
	c.setNewRungs(candidates)
	p := c.fretWindows(c.split(candidates))
	if inHand && c.budget >= applicationMinMinutes*60 {
		c.application(candidates, nil, nil)
	}
	c.focus(p)
	c.fill(p.known, domain.PracticePickReviewAhead, c.budget)
	c.fill(p.fresh, domain.PracticePickNew, c.budget)
	c.neverEmpty()
	return s.newPlan(ctx, studentID, instrumentID, c, domain.PracticeFocusChosen, maps)
}

// subtreeNodeIDs lists nodeID and the nodes part_of it, at any depth, among
// the nodes of maps, nodeID first, then in catalog order; none when nodeID
// is not among them.
func subtreeNodeIDs(maps []KnowledgeMap, nodeID string) []string {
	var nodes []domain.KnowledgeNode
	seen := map[string]bool{}
	for _, m := range maps {
		for _, n := range m.Nodes {
			if !seen[n.ID] {
				seen[n.ID] = true
				nodes = append(nodes, n)
			}
		}
	}
	if !seen[nodeID] {
		return nil
	}
	in := map[string]bool{nodeID: true}
	subtree := []string{nodeID}
	for grew := true; grew; {
		grew = false
		for _, n := range nodes {
			if !in[n.ID] && n.ParentID != nil && in[*n.ParentID] {
				in[n.ID] = true
				subtree = append(subtree, n.ID)
				grew = true
			}
		}
	}
	return subtree
}

// confusablesTogether gathers, in picks, the items of a subject a learner
// mistakes for one another where the first of them is, laid out so that
// items mistaken for each other come one after the other: a chain such as
// vi–IV–I–V, I–V–vi–IV, I–vi–IV–V, I–vi–ii–V keeps every link side by
// side. Any other pick keeps its place.
func (c *composer) confusablesTogether(picks []domain.PracticeSessionItem) []domain.PracticeSessionItem {
	ordered := slices.Clone(picks)
	gathered := map[string]bool{}
	for i := 0; i < len(ordered); i++ {
		if gathered[ordered[i].ItemKey] {
			continue
		}
		group := c.confusableGroup(ordered, i)
		for _, item := range group {
			gathered[item.ItemKey] = true
		}
		if len(group) == 1 {
			continue
		}
		rest := slices.DeleteFunc(slices.Clone(ordered[i:]), func(item domain.PracticeSessionItem) bool { return gathered[item.ItemKey] })
		ordered = slices.Concat(ordered[:i], c.chained(group), rest)
		i += len(group) - 1
	}
	return ordered
}

// mistaken reports whether a learner mistakes a for b, or b for a.
func (c *composer) mistaken(a, b domain.PracticeSessionItem) bool {
	return slices.Contains(c.confusable[a.ItemKey], b.ItemKey) || slices.Contains(c.confusable[b.ItemKey], a.ItemKey)
}

// confusableGroup is the item at from in picks and every later pick of its
// subject linked to it by items mistaken for one another, in picks order.
func (c *composer) confusableGroup(picks []domain.PracticeSessionItem, from int) []domain.PracticeSessionItem {
	first := picks[from]
	group := []domain.PracticeSessionItem{first}
	if first.NodeID == nil {
		return group
	}
	in := map[int]bool{from: true}
	for grew := true; grew; {
		grew = false
		for j := from + 1; j < len(picks); j++ {
			item := picks[j]
			if in[j] || item.NodeID == nil || *item.NodeID != *first.NodeID {
				continue
			}
			if slices.ContainsFunc(group, func(member domain.PracticeSessionItem) bool { return c.mistaken(member, item) }) {
				in[j], grew = true, true
				group = append(group, item)
			}
		}
	}
	slices.SortFunc(group, func(a, b domain.PracticeSessionItem) int {
		return slices.IndexFunc(picks, func(p domain.PracticeSessionItem) bool { return p.ItemKey == a.ItemKey }) -
			slices.IndexFunc(picks, func(p domain.PracticeSessionItem) bool { return p.ItemKey == b.ItemKey })
	})
	return group
}

// chained orders group so each next item is one the previous is mistaken
// for whenever one is left: it starts from the earliest item with the
// fewest such links in the group, the end of a chain.
func (c *composer) chained(group []domain.PracticeSessionItem) []domain.PracticeSessionItem {
	links := func(item domain.PracticeSessionItem, among []domain.PracticeSessionItem) int {
		n := 0
		for _, other := range among {
			if other.ItemKey != item.ItemKey && c.mistaken(item, other) {
				n++
			}
		}
		return n
	}
	left := slices.Clone(group)
	start := 0
	for i, item := range left {
		if links(item, group) < links(left[start], group) {
			start = i
		}
	}
	out := []domain.PracticeSessionItem{left[start]}
	left = slices.Delete(left, start, start+1)
	for len(left) > 0 {
		last := out[len(out)-1]
		next := slices.IndexFunc(left, func(item domain.PracticeSessionItem) bool { return c.mistaken(last, item) })
		if next < 0 {
			next = 0
		}
		out = append(out, left[next])
		left = slices.Delete(left, next, next+1)
	}
	return out
}

// ListSubjects lists the subjects caller can choose to practise with
// instrumentID in hand, or in the head when it is nil: every knowledge node
// whose subtree has an item a session could pick, on the student's paths or
// not, whatever it requires, grouped by area (its top-level node). Each
// shows how many such items it has, the student's level over them, the best
// of their instruments in the head, and its current rung: see
// domain.SubjectRungs. An instrumentID that doesn't exist is not found.
func (s *PracticeSessionService) ListSubjects(ctx context.Context, caller domain.User, instrumentID *string) (domain.PracticeSubjectList, error) {
	s = s.forComposition()
	instrumentIDs, err := s.sessionInstruments(ctx, caller.ID, instrumentID)
	if err != nil {
		return domain.PracticeSubjectList{}, err
	}
	maps, err := s.knowledgeMaps(ctx, caller.ID, instrumentIDs)
	if err != nil {
		return domain.PracticeSubjectList{}, err
	}
	if err := s.composingDiagrams.readAll(ctx, maps); err != nil {
		return domain.PracticeSubjectList{}, err
	}
	templates, err := s.drillTemplates(ctx)
	if err != nil {
		return domain.PracticeSubjectList{}, err
	}
	pathSkillIDs, err := s.learning.pathSkillIDs(ctx, caller.ID)
	if err != nil {
		return domain.PracticeSubjectList{}, err
	}
	c := newComposer(s.now(), 0, s.shuffle, templates)
	tally := newSubjectTally()
	loaded := map[string]*practiceCandidate{}
	for _, m := range maps {
		if err := s.tallyMap(ctx, c, m, instrumentID != nil, loaded, tally); err != nil {
			return domain.PracticeSubjectList{}, err
		}
	}
	return tally.list(instrumentID, allNodes(maps), pathSkillIDs, c.now), nil
}

// tallyMap tallies in tally the items of knowledge map m that suit a
// session with the instrument in hand (inHand) or in the head, each loaded
// once into loaded, under every node of m whose subtree holds them.
func (s *PracticeSessionService) tallyMap(ctx context.Context, c *composer, m KnowledgeMap, inHand bool, loaded map[string]*practiceCandidate, tally subjectTally) error {
	suits := map[string]bool{}
	for _, item := range m.Items {
		cand, err := s.loadOnce(ctx, loaded, item.ItemKey)
		if err != nil {
			return err
		}
		if cand == nil || !cand.suitsSession(inHand) {
			continue
		}
		suits[item.ItemKey] = true
		with := *cand
		if state, ok := m.States[item.ItemKey]; ok {
			with.state = &state
			tally.states[item.ItemKey] = state
		}
		tally.rungs[item.ItemKey] = c.rungOf(with)
	}
	for _, n := range m.Nodes {
		var keys []string
		for _, key := range m.Subtrees[n.ID] {
			if suits[key] {
				keys = append(keys, key)
			}
		}
		tally.add(n.ID, keys, domain.RollUpNode(keys, m.States, c.now).Level)
	}
	return nil
}

// subjectTally gathers each node's material for a session across the
// student's instruments: its items by rung, their states, and its best
// level. rungs holds the rung each item trains.
type subjectTally struct {
	levels   map[string]*domain.KnowledgeLevel
	material map[string]map[domain.PracticeRung][]string
	rungs    map[string]domain.PracticeRung
	states   map[string]domain.PracticeItemState
}

func newSubjectTally() subjectTally {
	return subjectTally{
		levels: map[string]*domain.KnowledgeLevel{}, material: map[string]map[domain.PracticeRung][]string{},
		rungs: map[string]domain.PracticeRung{}, states: map[string]domain.PracticeItemState{},
	}
}

// list is the subjects list of the tallied nodes with material, among
// nodes, for instrumentID: each grouped under its area, in catalog order,
// on the path when among pathSkillIDs, with no level when wide.
func (t subjectTally) list(instrumentID *string, nodes []domain.KnowledgeNode, pathSkillIDs []string, now time.Time) domain.PracticeSubjectList {
	list := domain.PracticeSubjectList{InstrumentID: instrumentID, Groups: []domain.PracticeSubjectGroup{}}
	for _, n := range nodes {
		if len(t.material[n.ID]) == 0 {
			continue
		}
		subject := domain.PracticeSubject{Node: n, OnPath: slices.Contains(pathSkillIDs, n.ID), ItemCount: t.count(n.ID)}
		if !hasChild(nodes, n.ID) {
			subject.Level = t.levels[n.ID]
		}
		subject.CurrentRung, _ = domain.SubjectRungs(t.material[n.ID], t.states, now)
		area := areaOf(nodes, n)
		i := slices.IndexFunc(list.Groups, func(g domain.PracticeSubjectGroup) bool { return g.Area.ID == area.ID })
		if i < 0 {
			list.Groups = append(list.Groups, domain.PracticeSubjectGroup{Area: area})
			i = len(list.Groups) - 1
		}
		list.Groups[i].Subjects = append(list.Groups[i].Subjects, subject)
	}
	slices.SortStableFunc(list.Groups, func(a, b domain.PracticeSubjectGroup) int {
		return cmp.Compare(nodeIndex(nodes, a.Area.ID), nodeIndex(nodes, b.Area.ID))
	})
	return list
}

// add tallies keys, the items of nodeID's subtree that suit the session for
// one instrument, on which the student's level is level.
func (t subjectTally) add(nodeID string, keys []string, level *domain.KnowledgeLevel) {
	if len(keys) == 0 {
		return
	}
	if t.material[nodeID] == nil {
		t.material[nodeID] = map[domain.PracticeRung][]string{}
	}
	for _, key := range keys {
		rung := t.rungs[key]
		if !slices.Contains(t.material[nodeID][rung], key) {
			t.material[nodeID][rung] = append(t.material[nodeID][rung], key)
		}
	}
	if best := t.levels[nodeID]; level != nil && (best == nil || level.Rank() > best.Rank()) {
		t.levels[nodeID] = level
	}
}

// count is how many items nodeID's subtree has for the session.
func (t subjectTally) count(nodeID string) int {
	n := 0
	for _, keys := range t.material[nodeID] {
		n += len(keys)
	}
	return n
}

// loadOnce is the candidate itemKey names, loaded once into loaded; nil
// when it is no item a session picks.
func (s *PracticeSessionService) loadOnce(ctx context.Context, loaded map[string]*practiceCandidate, itemKey string) (*practiceCandidate, error) {
	if cand, ok := loaded[itemKey]; ok {
		return cand, nil
	}
	cand, ok, err := s.candidate(ctx, itemKey)
	if err != nil {
		return nil, err
	}
	if !ok {
		loaded[itemKey] = nil
		return nil, nil
	}
	loaded[itemKey] = &cand
	return &cand, nil
}

// allNodes lists the nodes of maps, each once, in catalog order.
func allNodes(maps []KnowledgeMap) []domain.KnowledgeNode {
	var nodes []domain.KnowledgeNode
	for _, m := range maps {
		for _, n := range m.Nodes {
			if nodeIndex(nodes, n.ID) < 0 {
				nodes = append(nodes, n)
			}
		}
	}
	slices.SortStableFunc(nodes, func(a, b domain.KnowledgeNode) int {
		return cmp.Compare(catalogIndex(maps, a.ID), catalogIndex(maps, b.ID))
	})
	return nodes
}

// catalogIndex is where nodeID first comes in the nodes of maps.
func catalogIndex(maps []KnowledgeMap, nodeID string) int {
	for _, m := range maps {
		if i := nodeIndex(m.Nodes, nodeID); i >= 0 {
			return i
		}
	}
	return -1
}

func nodeIndex(nodes []domain.KnowledgeNode, id string) int {
	return slices.IndexFunc(nodes, func(n domain.KnowledgeNode) bool { return n.ID == id })
}

// hasChild reports whether a node of nodes is part_of id.
func hasChild(nodes []domain.KnowledgeNode, id string) bool {
	return slices.ContainsFunc(nodes, func(n domain.KnowledgeNode) bool { return n.ParentID != nil && *n.ParentID == id })
}

// areaOf is n's top-level node among nodes: n itself when it has no parent
// among them.
func areaOf(nodes []domain.KnowledgeNode, n domain.KnowledgeNode) domain.KnowledgeNode {
	for n.ParentID != nil {
		i := nodeIndex(nodes, *n.ParentID)
		if i < 0 {
			break
		}
		n = nodes[i]
	}
	return n
}
