package domain

import (
	"cmp"
	"maps"
	"slices"
	"time"
)

// summaryDays is how many calendar days, today included, the practice
// home counts practice and learning over.
const summaryDays = 7

// Last7DaysStart is when the last 7 calendar days began at now in loc:
// local midnight six days before the local today.
func Last7DaysStart(now time.Time, loc *time.Location) time.Time {
	y, m, d := now.In(loc).Date()
	return time.Date(y, m, d-(summaryDays-1), 0, 0, 0, 0, loc)
}

// DaysInLast7 counts the calendar days in loc, among the last 7 at now,
// on which at least one of times falls. It is a count, never a streak: a
// missed day resets nothing.
func DaysInLast7(times []time.Time, now time.Time, loc *time.Location) int {
	count := 0
	for _, day := range DaysOfLast7(times, now, loc) {
		if day.Marked {
			count++
		}
	}
	return count
}

// CalendarDay is one calendar day, at its local midnight, and whether
// something happened on it.
type CalendarDay struct {
	Date   time.Time
	Marked bool
}

// DaysOfLast7 lists the last 7 calendar days in loc at now, oldest first
// and today last, each marked when at least one of times falls on it.
// An unmarked day only says nothing happened, never that it was missed.
func DaysOfLast7(times []time.Time, now time.Time, loc *time.Location) []CalendarDay {
	y, m, d := now.In(loc).Date()
	days := make([]CalendarDay, summaryDays)
	index := make(map[time.Time]int, summaryDays)
	for i := range days {
		// Stepping by calendar date, not by 24 hours, keeps every day at
		// local midnight across a daylight saving change.
		date := time.Date(y, m, d-(summaryDays-1)+i, 0, 0, 0, 0, loc)
		days[i] = CalendarDay{Date: date}
		index[date] = i
	}
	for _, t := range times {
		ty, tm, td := t.In(loc).Date()
		if i, ok := index[time.Date(ty, tm, td, 0, 0, 0, 0, loc)]; ok {
			days[i].Marked = true
		}
	}
	return days
}

// Previous7DaysStart is when the 7 calendar days before the last 7 began at
// now in loc: local midnight thirteen days before the local today.
func Previous7DaysStart(now time.Time, loc *time.Location) time.Time {
	y, m, d := now.In(loc).Date()
	return time.Date(y, m, d-(2*summaryDays-1), 0, 0, 0, 0, loc)
}

// MinutesPractised is the whole minutes practised in the sessions among
// spans that started at or after from and before to: each counts from its
// start to its latest event, and the total rounds down. A latest event
// before the start, by a clock out of step, counts nothing.
func MinutesPractised(spans []PracticeSessionSpan, from, to time.Time) int {
	var total time.Duration
	for _, s := range spans {
		if s.StartedAt.Before(from) || !s.StartedAt.Before(to) {
			continue
		}
		total += max(s.LastEventAt.Sub(s.StartedAt), 0)
	}
	return int(total / time.Minute)
}

// DayStreaks counts runs of consecutive calendar days in loc on which at
// least one of ends falls. The current run ends today, or yesterday while
// today has none yet, so a day not over never breaks it; best is the
// longest run ever. A day after today, by a clock running ahead, is left
// out.
func DayStreaks(ends []time.Time, now time.Time, loc *time.Location) (current, best int) {
	today := localDate(now, loc)
	days := map[time.Time]bool{}
	for _, t := range ends {
		if day := localDate(t, loc); !day.After(today) {
			days[day] = true
		}
	}
	sorted := slices.SortedFunc(maps.Keys(days), time.Time.Compare)
	run := 0
	for i, day := range sorted {
		if i > 0 && nextDay(sorted[i-1]).Equal(day) {
			run++
		} else {
			run = 1
		}
		best = max(best, run)
	}
	day := today
	if !days[day] {
		day = day.AddDate(0, 0, -1)
	}
	for days[day] {
		current++
		day = day.AddDate(0, 0, -1)
	}
	return current, best
}

// localDate is t's calendar date in loc, as midnight UTC so that dates
// compare and step by whole days whatever loc's offsets do.
func localDate(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func nextDay(date time.Time) time.Time {
	return date.AddDate(0, 0, 1)
}

// FinishedPracticeSession is a practice session the student finished:
// ended without leaving early. A session that was abandoned never ended,
// so it is never finished.
type FinishedPracticeSession struct {
	// InstrumentID is the instrument in hand; nil for a session in the
	// head.
	InstrumentID *string
	EndedAt      time.Time
}

// PracticeSessionSpan is when a practice session started and when its
// latest practice event arrived, whatever became of the session: finished,
// left early, abandoned or still open.
type PracticeSessionSpan struct {
	StartedAt   time.Time
	LastEventAt time.Time
}

// PracticeItemSnapshot is a student's state on one practice item as it
// stood at the end of a past day.
type PracticeItemSnapshot struct {
	ItemKey      string
	Counted      int
	Accuracy     float64
	Fluency      float64
	BestCleanBPM *int
}

// SkillProgressMeasure is what improved in a progress line.
type SkillProgressMeasure string

const (
	SkillProgressMeasureAccuracy       SkillProgressMeasure = "accuracy"
	SkillProgressMeasureFluency        SkillProgressMeasure = "fluency"
	SkillProgressMeasureBestCleanTempo SkillProgressMeasure = "best_clean_tempo_bpm"
)

// SkillProgress is how one skill improved on one measure, with where it
// started and where it is now.
type SkillProgress struct {
	NodeID  string
	Measure SkillProgressMeasure
	Before  float64
	After   float64
}

// improvement is how much p improved, comparable across measures: a
// ratio by its gain, a tempo by its gain over where it started.
func (p SkillProgress) improvement() float64 {
	if p.Measure == SkillProgressMeasureBestCleanTempo {
		return (p.After - p.Before) / p.Before
	}
	return p.After - p.Before
}

// SkillProgressLines lists how skill improved over its items itemKeys,
// from the states before to the states now, keyed by item key. Accuracy
// and fluency are the means over the items practised at each point, and
// start from 0 for a skill first practised since; the best clean tempo is
// the best over the items, and needs a clean take on both sides. Only a
// measure that improved has a line; a skill with nothing practised now
// has none.
func SkillProgressLines(skill KnowledgeNode, itemKeys []string, now map[string]PracticeItemState, before map[string]PracticeItemSnapshot) []SkillProgress {
	var after, then progressTotals
	for _, key := range itemKeys {
		if s, ok := now[key]; ok && s.Counted > 0 {
			after.add(s.Accuracy, s.Fluency, s.BestCleanBPM)
		}
		if s, ok := before[key]; ok && s.Counted > 0 {
			then.add(s.Accuracy, s.Fluency, s.BestCleanBPM)
		}
	}
	if after.items == 0 {
		return nil
	}
	var lines []SkillProgress
	line := func(measure SkillProgressMeasure, before, after float64) {
		if after > before {
			lines = append(lines, SkillProgress{NodeID: skill.ID, Measure: measure, Before: before, After: after})
		}
	}
	line(SkillProgressMeasureAccuracy, then.meanAccuracy(), after.meanAccuracy())
	line(SkillProgressMeasureFluency, then.meanFluency(), after.meanFluency())
	if then.bestBPM != nil && after.bestBPM != nil {
		line(SkillProgressMeasureBestCleanTempo, float64(*then.bestBPM), float64(*after.bestBPM))
	}
	return lines
}

// progressTotals sums a skill's practised items at one point in time.
type progressTotals struct {
	items    int
	accuracy float64
	fluency  float64
	bestBPM  *int
}

func (t *progressTotals) add(accuracy, fluency float64, bestBPM *int) {
	t.items++
	t.accuracy += accuracy
	t.fluency += fluency
	if bestBPM != nil && (t.bestBPM == nil || *bestBPM > *t.bestBPM) {
		best := *bestBPM
		t.bestBPM = &best
	}
}

func (t progressTotals) meanAccuracy() float64 {
	if t.items == 0 {
		return 0
	}
	return t.accuracy / float64(t.items)
}

func (t progressTotals) meanFluency() float64 {
	if t.items == 0 {
		return 0
	}
	return t.fluency / float64(t.items)
}

// RankSkillProgress orders lines most improved first.
func RankSkillProgress(lines []SkillProgress) {
	slices.SortStableFunc(lines, func(a, b SkillProgress) int {
		return cmp.Compare(b.improvement(), a.improvement())
	})
}

// PracticeNextStepKind is what a next step asks of the student.
type PracticeNextStepKind string

const (
	// PracticeNextStepRefresh is a skill whose known items are fading.
	PracticeNextStepRefresh PracticeNextStepKind = "refresh"
	// PracticeNextStepStrengthen is a skill practised but not yet secure.
	PracticeNextStepStrengthen PracticeNextStepKind = "strengthen"
	// PracticeNextStepReadyToStart is a skill with every requirement met
	// and nothing practised yet.
	PracticeNextStepReadyToStart PracticeNextStepKind = "ready_to_start"
)

// PracticeNextStep is one suggested next step: a skill and what to do
// with it.
type PracticeNextStep struct {
	Kind  PracticeNextStepKind
	Node  KnowledgeNode
	Level *KnowledgeLevel
}

// RankNextSteps lists the student's next steps in view: skills to refresh,
// then to strengthen, then ready to start. Only skills with something to
// practise and no children to show them by are steps. A skill to refresh
// or strengthen ranks the student's path skills, pathNodeIDs, first, then
// those that build on something the student meets, then the shallowest
// requires depth, then catalog order; one ready to start is a node stretch
// could reach, connected to what the student is learning, in stretch's
// order.
func RankNextSteps(view KnowledgeView, pathNodeIDs []string) []PracticeNextStep {
	isStep := func(n KnowledgeNode) bool {
		return n.Kind == KnowledgeNodeKindSkill && view.Standings[n.ID].Level != nil && !view.Wide(n.ID)
	}
	var refresh, strengthen []KnowledgeNode
	for _, n := range view.Nodes {
		if !isStep(n) || !view.Practised(n.ID) {
			continue
		}
		switch s := view.Standings[n.ID]; {
		case s.Fading:
			refresh = append(refresh, n)
		case s.Level.Rank() < KnowledgeLevelFluent.Rank():
			strengthen = append(strengthen, n)
		}
	}
	rank := func(a, b KnowledgeNode) int {
		return cmp.Or(
			cmp.Compare(offPath(a.ID, pathNodeIDs), offPath(b.ID, pathNodeIDs)),
			cmp.Compare(buildsOnRank(view.Standings[a.ID]), buildsOnRank(view.Standings[b.ID])),
			cmp.Compare(view.Standings[a.ID].RequiresDepth, view.Standings[b.ID].RequiresDepth),
		)
	}
	slices.SortStableFunc(refresh, rank)
	slices.SortStableFunc(strengthen, rank)

	byID := make(map[string]KnowledgeNode, len(view.Nodes))
	for _, n := range view.Nodes {
		byID[n.ID] = n
	}
	var steps []PracticeNextStep
	add := func(kind PracticeNextStepKind, nodes []KnowledgeNode) {
		for _, n := range nodes {
			steps = append(steps, PracticeNextStep{Kind: kind, Node: n, Level: view.Standings[n.ID].Level})
		}
	}
	add(PracticeNextStepRefresh, refresh)
	add(PracticeNextStepStrengthen, strengthen)
	for _, id := range RankStretchNodes(view.Nodes, view.Standings, pathNodeIDs, view.Applies) {
		if n := byID[id]; isStep(n) && !view.Practised(id) {
			add(PracticeNextStepReadyToStart, []KnowledgeNode{n})
		}
	}
	return steps
}

// offPath ranks a node on the student's path, pathNodeIDs, first.
func offPath(id string, pathNodeIDs []string) int {
	if slices.Contains(pathNodeIDs, id) {
		return 0
	}
	return 1
}

// buildsOnRank ranks a node with requirements, building on something,
// first.
func buildsOnRank(s NodeStanding) int {
	if s.Readiness.Total > 0 {
		return 0
	}
	return 1
}

// PracticeNodeGroup is the practice nodes of one area, or those that suit
// any instrument.
type PracticeNodeGroup struct {
	// Area is the top-level node of the group; nil for the Any instrument
	// group.
	Area          *KnowledgeNode
	AnyInstrument bool
	// Nodes lists the group's nodes in catalog order.
	Nodes []KnowledgeNode
}

// GroupPracticeNodes groups the nodes, in catalog order, that have
// something to practise in standings: by their area, the top of their
// tree, in the areas' catalog order; then nodes for every instrument in an
// Any instrument group of their own.
func GroupPracticeNodes(nodes []KnowledgeNode, standings map[string]NodeStanding) []PracticeNodeGroup {
	byID := make(map[string]KnowledgeNode, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
	}
	areaOf := func(n KnowledgeNode) KnowledgeNode {
		for n.ParentID != nil {
			parent, ok := byID[*n.ParentID]
			if !ok {
				break
			}
			n = parent
		}
		return n
	}
	var groups []PracticeNodeGroup
	at := map[string]int{}
	anyInstrument := PracticeNodeGroup{AnyInstrument: true}
	for _, n := range nodes {
		if standings[n.ID].Total == 0 {
			continue
		}
		if len(n.InstrumentIDs) == 0 {
			anyInstrument.Nodes = append(anyInstrument.Nodes, n)
			continue
		}
		area := areaOf(n)
		i, ok := at[area.ID]
		if !ok {
			i = len(groups)
			at[area.ID] = i
			groups = append(groups, PracticeNodeGroup{Area: &area})
		}
		groups[i].Nodes = append(groups[i].Nodes, n)
	}
	position := make(map[string]int, len(nodes))
	for i, n := range nodes {
		position[n.ID] = i
	}
	slices.SortStableFunc(groups, func(a, b PracticeNodeGroup) int {
		return cmp.Compare(position[a.Area.ID], position[b.Area.ID])
	})
	if len(anyInstrument.Nodes) > 0 {
		groups = append(groups, anyInstrument)
	}
	return groups
}
