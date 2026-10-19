//go:build integration

package bdd

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// catalogChordVoicings are the drill catalog's chord drills, by voicing key, in
// catalog order, and catalogChordPairs its change pairs.
var (
	catalogChordVoicings = []struct{ voicing, chord string }{
		{"c-open", "C"}, {"a-open", "A"}, {"g-open", "G"}, {"e-open", "E"},
		{"d-open", "D"}, {"a-minor-open", "Am"}, {"e-minor-open", "Em"}, {"d-minor-open", "Dm"},
	}
	catalogChordPairs = [][2]string{
		{"a-minor-open", "c-open"}, {"c-open", "g-open"}, {"g-open", "d-open"}, {"d-open", "a-open"}, {"a-open", "e-open"},
		{"a-minor-open", "e-open"}, {"e-minor-open", "g-open"}, {"d-open", "e-minor-open"}, {"a-minor-open", "d-minor-open"}, {"e-minor-open", "c-open"},
	}
)

// chordDrillCatalog is a ports.ChordDrillRepository holding the drill
// catalog's chord drills, each playing the diagram its voicing key names, and
// a change target of 60 in force since before every scenario.
type chordDrillCatalog struct{}

func (chordDrillCatalog) Catalog(context.Context) (domain.ChordDrillCatalog, error) {
	var catalog domain.ChordDrillCatalog
	for i, v := range catalogChordVoicings {
		catalog.Drills = append(catalog.Drills, domain.ChordDrill{ID: "drill-" + v.voicing, DiagramID: diagramID(v.voicing).String(), Chord: v.chord, Ordinal: i + 1})
	}
	for i, p := range catalogChordPairs {
		catalog.Changes = append(catalog.Changes, domain.ChordChangePair{ID: fmt.Sprintf("pair-%d", i), FromChordDrillID: "drill-" + p[0], ToChordDrillID: "drill-" + p[1], Ordinal: i + 1})
	}
	catalog.Targets = []domain.ChangeTarget{{ID: "target-1", Version: 1, EffectiveFrom: fixedNow.AddDate(0, -1, 0), ChangesPerMinute: 60}}
	return catalog, nil
}

func chordSymbolOf(voicing string) string {
	i := slices.IndexFunc(catalogChordVoicings, func(v struct{ voicing, chord string }) bool { return v.voicing == voicing })
	if i < 0 {
		return ""
	}
	return catalogChordVoicings[i].chord
}

func chordVoicingKey(voicing string) string {
	return "chord_voicing:" + diagramID(voicing).String()
}

func chordChangeKey(from, to string) string {
	return "chord_change:" + diagramID(from).String() + ":" + diagramID(to).String()
}

// chordDrillWorld is what the chord drill steps share in a scenario: the
// change a step named, whose plan "its minute" reads.
type chordDrillWorld struct {
	changeKey string
}

func (w *world) chordDrill() *chordDrillWorld {
	if w.chordDrills == nil {
		w.chordDrills = &chordDrillWorld{}
	}
	return w.chordDrills
}

func registerChordDrillSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)"'s path skill "(play-open-chords)" has new single chords on guitar$`, w.pathSkillHasSingleChords)
	sc.Step(`^"([^"]+)"'s path skill "(change-chords)" has new chord changes on guitar$`, w.pathSkillHasChordChanges)
	sc.Step(`^"([^"]+)"'s path skill "(play-root-fifth-lines)" has new root and fifth items on electric bass$`, w.pathSkillHasRootFifths)
	sc.Step(`^"([^"]+)" has new single chords and new chord changes on guitar$`, w.hasSingleChordsAndChanges)
	sc.Step(`^"([^"]+)" has 8 new single chords and 10 new chord changes on guitar, and nothing else$`, w.hasOnlySingleChordsAndChanges)
	sc.Step(`^"([^"]+)" has the new single chord "([^"]+)" and the new chord change "([^"]+)" to "([^"]+)" on guitar$`, w.hasChordAndChange)
	sc.Step(`^"([^"]+)" is fluent on every single chord of "play-open-chords" on guitar$`, w.fluentOnEverySingleChord)
	sc.Step(`^student "([^"]+)"'s only path skill is "play-open-chords", practised by single chords on guitar$`, w.onlySingleChordsOnPath)

	sc.Step(`^"([^"]+)" has never played the chord change "([^"]+)" to "([^"]+)"$`, w.neverPlayedChange)
	sc.Step(`^"([^"]+)"'s last minute of "([^"]+)" to "([^"]+)" was almost, (\d+) clean changes at (\d+) BPM$`, w.lastMinuteAlmost)
	sc.Step(`^"([^"]+)"'s last minute of "([^"]+)" to "([^"]+)" was struggled at (\d+) BPM$`, w.lastMinuteStruggled)
	sc.Step(`^"([^"]+)"'s last two minutes of "([^"]+)" to "([^"]+)" were clean, the last at (\d+) BPM$`, w.lastTwoMinutesClean)
	sc.Step(`^"([^"]+)"'s minutes of "([^"]+)" to "([^"]+)" were struggled, then clean at (\d+) BPM$`, w.struggledThenClean)
	sc.Step(`^"([^"]+)" counted (\d+), then (\d+), then (\d+) clean changes of "([^"]+)" to "([^"]+)", each minute clean$`, w.countedCleanMinutes)

	sc.Step(`^the single chord "([^"]+)" is picked for "([^"]+)"'s session with "guitar" in hand$`, w.composesChordSession("guitar"))
	sc.Step(`^the root and fifth of "([^"]+)" is picked for "([^"]+)"'s session with "electric-bass" in hand$`, w.composesChordSession("electric-bass"))
	sc.Step(`^both are picked for "([^"]+)"'s session with "guitar" in hand$`, w.bothPicked)
	sc.Step(`^the chord change is picked as a (?:new|due) item$`, w.chordChangePicked)

	sc.Step(`^the session includes single chords of "play-open-chords" with the reason new$`, w.sessionIncludesNewSingleChords)
	sc.Step(`^every single chord in the session is at the play_clean rung$`, w.everyChordOfKind(generated.PracticeItemKindChordVoicing, func(item generated.PracticeSessionItem) error {
		return expectChord(item.Rung == generated.PlayClean, "rung %s", item.Rung)
	}))
	sc.Step(`^every single chord in the session is estimated at (\d+) seconds$`, w.everyChordEstimatedAt(generated.PracticeItemKindChordVoicing))
	sc.Step(`^every chord change in the session is estimated at (\d+) seconds$`, w.everyChordEstimatedAt(generated.PracticeItemKindChordChange))
	sc.Step(`^it names the voicing diagram of "([^"]+)" and the chord "([^"]+)"$`, w.namesVoicingAndChord)
	sc.Step(`^the session includes a chord change of "change-chords" at the play_in_time rung$`, w.sessionIncludesChange)
	sc.Step(`^it names both chords' voicing diagrams and symbols, and a target of (\d+) clean changes a minute$`, w.changeNamesBothChords)
	sc.Step(`^its root is on string (\d+) at fret (\d+) and its fifth on string (\d+) at fret (\d+) of the "electric-bass" layout$`, w.rootAndFifthAt)
	sc.Step(`^the session has at most (\d+) single chords and at most (\d+) chord changes$`, w.atMostChordsAndChanges)
	sc.Step(`^the single chord "([^"]+)" comes before the chord change$`, w.singleChordBeforeChange)
	sc.Step(`^its minute starts at (\d+) BPM, with no best clean count and no last count$`, w.minuteStartsFresh)
	sc.Step(`^its minute starts at (\d+) BPM, and its last count is (\d+)$`, w.minuteStartsWithLastCount)
	sc.Step(`^its minute starts at (\d+) BPM$`, w.minuteStartsAt)
	sc.Step(`^its best clean count is (\d+) and its last count is (\d+)$`, w.bestAndLastCounts)
	sc.Step(`^no item in the session is a single chord, a chord change, a root and fifth or a root change$`, w.noChordItems)
	sc.Step(`^no single chord or chord change has the reason warm_up or application$`, w.noChordWarmUpOrEnding)
	sc.Step(`^the session has at least one single chord with the reason new$`, w.atLeastOneNewSingleChord)
}

func expectChord(ok bool, format string, args ...any) error {
	if !ok {
		return fmt.Errorf(format, args...)
	}
	return nil
}

// onChordPath puts skill on name's path, through a lesson classified under it.
func (w *world) onChordPath(name, skill string) {
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	node := domain.ContentNode{
		ID: nodeID("chords-" + skill).String(), Title: "chords-" + skill, ContentType: domain.ContentTypeArticle,
		Classification: domain.Classification{Skills: []domain.KnowledgeNode{{ID: w.skillIDFor(skill).String()}}},
	}
	w.nodes.put(node)
	w.studentPaths.put(domain.StudentPath{
		ID: deterministicUUID("student-path", name+"/"+skill).String(), StudentID: studentID,
		SourceTemplateID: pathID("chords-" + skill).String(), AssignedAt: fixedNow,
		Items: []domain.StudentPathItemRecord{{Position: 1, ContentNodeID: node.ID}},
	})
	w.paths.put(domain.LearningPath{ID: pathID("chords-" + skill).String(), InstrumentIDs: []string{instrumentID("guitar").String(), instrumentID("electric-bass").String()}})
}

func (w *world) classifyChords(instrument, skill string, keys ...string) {
	id := instrumentID(instrument).String()
	for _, key := range keys {
		w.practiceItems.chords[id] = append(w.practiceItems.chords[id], domain.ClassifiedItem{ItemKey: key, NodeIDs: []string{w.skillIDFor(skill).String()}})
	}
}

func allVoicingKeys() []string {
	keys := make([]string, len(catalogChordVoicings))
	for i, v := range catalogChordVoicings {
		keys[i] = chordVoicingKey(v.voicing)
	}
	return keys
}

func allChangeKeys() []string {
	keys := make([]string, len(catalogChordPairs))
	for i, p := range catalogChordPairs {
		keys[i] = chordChangeKey(p[0], p[1])
	}
	return keys
}

func (w *world) pathSkillHasSingleChords(name, skill string) error {
	w.onChordPath(name, skill)
	w.classifyChords("guitar", skill, allVoicingKeys()...)
	return nil
}

func (w *world) pathSkillHasChordChanges(name, skill string) error {
	w.onChordPath(name, skill)
	w.classifyChords("guitar", skill, allChangeKeys()...)
	return nil
}

// pathSkillHasRootFifths tunes the bass as a four-string bass, whose places the
// roots and fifths are computed on.
func (w *world) pathSkillHasRootFifths(name, skill string) error {
	bass, err := w.ensureInstrumentSeeded("electric-bass")
	if err != nil {
		return err
	}
	four := 4
	bass.StringCount, bass.Tuning = &four, []string{"E1", "A1", "D2", "G2"}
	// Strings and tuning never change through an update, so the bass is put
	// anew.
	w.instruments.put(bass)
	w.onChordPath(name, skill)
	for _, v := range catalogChordVoicings {
		w.classifyChords("electric-bass", skill, domain.RootFifthItemKey(bass.ID, v.chord))
	}
	return nil
}

func (w *world) hasSingleChordsAndChanges(name string) error {
	if err := w.pathSkillHasSingleChords(name, "play-open-chords"); err != nil {
		return err
	}
	return w.pathSkillHasChordChanges(name, "change-chords")
}

// hasOnlySingleChordsAndChanges leaves the path's play-alongs out, so only the
// chords can be played.
func (w *world) hasOnlySingleChordsAndChanges(name string) error {
	w.diagrams.mu.Lock()
	delete(w.diagrams.byID, diagramID("guitar-lick").String())
	w.diagrams.mu.Unlock()
	return w.hasSingleChordsAndChanges(name)
}

func (w *world) hasChordAndChange(name, chord, from, to string) error {
	w.onChordPath(name, "play-open-chords")
	w.onChordPath(name, "change-chords")
	w.classifyChords("guitar", "play-open-chords", chordVoicingKey(chord))
	w.classifyChords("guitar", "change-chords", chordChangeKey(from, to))
	w.chordDrill().changeKey = chordChangeKey(from, to)
	// Due, the change would otherwise come before the new chord.
	w.putChangeState(name, chordChangeKey(from, to), domain.PracticeItemState{Level: domain.KnowledgeLevelLearning, Counted: 2, Box: 1})
	return nil
}

func (w *world) fluentOnEverySingleChord(name string) error {
	if err := w.pathSkillHasSingleChords(name, "play-open-chords"); err != nil {
		return err
	}
	w.knownItems(name, 10, allVoicingKeys()...)
	return nil
}

func (w *world) onlySingleChordsOnPath(name string) error {
	w.authenticateAs(name, domain.RoleStudent)
	if _, err := w.ensureInstrumentSeeded("guitar"); err != nil {
		return err
	}
	return w.pathSkillHasSingleChords(name, "play-open-chords")
}

// change puts the change from to on name's path for guitar.
func (w *world) change(name, from, to string) string {
	w.onChordPath(name, "change-chords")
	key := chordChangeKey(from, to)
	w.classifyChords("guitar", "change-chords", key)
	w.chordDrill().changeKey = key
	return key
}

// putChangeState gives name state s on the change key, due an hour ago.
func (w *world) putChangeState(name, key string, s domain.PracticeItemState) {
	dueAt := fixedNow.Add(-time.Hour)
	s.ItemKey, s.RulesVersion, s.DueAt, s.LastAt = key, domain.PracticeRulesVersion, &dueAt, &fixedNow
	w.practiceStates.put(w.ensureRegistered(name, domain.RoleStudent).String(), s)
}

func (w *world) lastMinute(name, from, to string, last domain.ChangeMinuteState, best *int) {
	w.putChangeState(name, w.change(name, from, to), domain.PracticeItemState{
		Level: domain.KnowledgeLevelLearning, Counted: 2, Box: 1, BestChangesPerMinute: best, LastChangeMinute: &last,
	})
}

func (w *world) neverPlayedChange(name, from, to string) error {
	w.change(name, from, to)
	return nil
}

func (w *world) lastMinuteAlmost(name, from, to string, count, bpm int) error {
	w.lastMinute(name, from, to, domain.ChangeMinuteState{TempoBPM: bpm, ChangesPerMinute: count}, nil)
	return nil
}

func (w *world) lastMinuteStruggled(name, from, to string, bpm int) error {
	w.lastMinute(name, from, to, domain.ChangeMinuteState{TempoBPM: bpm, ChangesPerMinute: bpm / 4, Struggled: true}, nil)
	return nil
}

func (w *world) lastTwoMinutesClean(name, from, to string, bpm int) error {
	w.lastMinute(name, from, to, domain.ChangeMinuteState{TempoBPM: bpm, ChangesPerMinute: bpm, CleanInARow: 2}, &bpm)
	return nil
}

func (w *world) struggledThenClean(name, from, to string, bpm int) error {
	w.lastMinute(name, from, to, domain.ChangeMinuteState{TempoBPM: bpm, ChangesPerMinute: bpm, CleanInARow: 1}, &bpm)
	return nil
}

// countedCleanMinutes plays three clean minutes, every beat changed cleanly,
// so each count is its minute's tempo.
func (w *world) countedCleanMinutes(name string, first, second, third int, from, to string) error {
	best := max(first, second, third)
	w.lastMinute(name, from, to, domain.ChangeMinuteState{TempoBPM: third, ChangesPerMinute: third, CleanInARow: 3}, &best)
	return nil
}

// composesChordSession composes name's session with the named chord due, so
// it is picked whatever order the new chords would come in.
func (w *world) composesChordSession(instrument string) func(string, string) error {
	return func(chord, name string) error {
		key := chordVoicingKey(chord)
		if instrument == "electric-bass" {
			key = domain.RootFifthItemKey(instrumentID(instrument).String(), chord)
		}
		w.dueItems(name, key)
		w.authenticateAs(name, domain.RoleStudent)
		return w.composesSession(name, 10, instrument)
	}
}

func (w *world) bothPicked(name string) error {
	w.authenticateAs(name, domain.RoleStudent)
	return w.composesSession(name, 10, "guitar")
}

func (w *world) chordChangePicked() error {
	return w.composesSession("alice", 10, "guitar")
}

// planItemsOfKind lists the composed plan's items of kind, in plan order.
func (w *world) planItemsOfKind(kind generated.PracticeItemKind) ([]generated.PracticeSessionItem, error) {
	plan, err := w.composedPlan()
	if err != nil {
		return nil, err
	}
	var items []generated.PracticeSessionItem
	for _, item := range plan.Items {
		if item.Kind == kind {
			items = append(items, item)
		}
	}
	return items, nil
}

// sessionIncludesNewSingleChords holds when some single chords are picked as
// new; past the new share, more unseen ones may come as stretch.
func (w *world) sessionIncludesNewSingleChords() error {
	chords, err := w.planItemsOfKind(generated.PracticeItemKindChordVoicing)
	if err != nil {
		return err
	}
	for _, item := range chords {
		if item.Reason == generated.PracticePickReasonNew {
			return nil
		}
	}
	return fmt.Errorf("the session has no single chord with the reason new, of %d", len(chords))
}

func (w *world) everyChordOfKind(kind generated.PracticeItemKind, check func(generated.PracticeSessionItem) error) func() error {
	return func() error {
		items, err := w.planItemsOfKind(kind)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return fmt.Errorf("the session has no %s", kind)
		}
		for _, item := range items {
			if err := check(item); err != nil {
				return fmt.Errorf("%s: %w", item.ItemKey, err)
			}
		}
		return nil
	}
}

func (w *world) everyChordEstimatedAt(kind generated.PracticeItemKind) func(int) error {
	return func(seconds int) error {
		return w.everyChordOfKind(kind, func(item generated.PracticeSessionItem) error {
			return expectChord(item.EstimatedSeconds == seconds, "estimated at %d seconds, want %d", item.EstimatedSeconds, seconds)
		})()
	}
}

func (w *world) namesVoicingAndChord(voicing, chord string) error {
	chords, err := w.planItemsOfKind(generated.PracticeItemKindChordVoicing)
	if err != nil {
		return err
	}
	for _, item := range chords {
		if item.ItemKey == chordVoicingKey(voicing) {
			if item.ChordVoicing == nil || item.ChordVoicing.DiagramId.String() != diagramID(voicing).String() || item.ChordVoicing.ChordSymbol != chord {
				return fmt.Errorf("single chord %s plans %+v", voicing, item.ChordVoicing)
			}
			return nil
		}
	}
	return fmt.Errorf("the session has no single chord %q", voicing)
}

func (w *world) sessionIncludesChange() error {
	changes, err := w.planItemsOfKind(generated.PracticeItemKindChordChange)
	if err != nil {
		return err
	}
	if len(changes) == 0 {
		return fmt.Errorf("the session has no chord change")
	}
	return expectChord(changes[0].Rung == generated.PlayInTime, "rung %s", changes[0].Rung)
}

func (w *world) changeNamesBothChords(target int) error {
	changes, err := w.planItemsOfKind(generated.PracticeItemKindChordChange)
	if err != nil || len(changes) == 0 {
		return fmt.Errorf("the session has no chord change: %v", err)
	}
	for _, item := range changes {
		c := item.ChordChange
		if c == nil {
			return fmt.Errorf("%s plans no chord change", item.ItemKey)
		}
		key := "chord_change:" + c.FromDiagramId.String() + ":" + c.ToDiagramId.String()
		if key != item.ItemKey || c.FromChordSymbol == "" || c.ToChordSymbol == "" || c.Minute.TargetChangesPerMinute != target {
			return fmt.Errorf("%s plans %+v", item.ItemKey, *c)
		}
	}
	return nil
}

func (w *world) rootAndFifthAt(rootString, rootFret, fifthString, fifthFret int) error {
	items, err := w.planItemsOfKind(generated.PracticeItemKindRootFifth)
	if err != nil {
		return err
	}
	for _, item := range items {
		r := item.RootFifth
		if r != nil && r.ChordSymbol == "Am" {
			return expectChord(r.Root == generated.FretPlace{String: rootString, Fret: rootFret} && r.Fifth == generated.FretPlace{String: fifthString, Fret: fifthFret},
				"root %+v and fifth %+v", r.Root, r.Fifth)
		}
	}
	return fmt.Errorf("the session has no root and fifth of Am")
}

func (w *world) atMostChordsAndChanges(chords, changes int) error {
	single, err := w.planItemsOfKind(generated.PracticeItemKindChordVoicing)
	if err != nil {
		return err
	}
	pairs, err := w.planItemsOfKind(generated.PracticeItemKindChordChange)
	if err != nil {
		return err
	}
	return expectChord(len(single) <= chords && len(pairs) <= changes && len(single) > 0 && len(pairs) > 0,
		"%d single chords and %d changes", len(single), len(pairs))
}

func (w *world) singleChordBeforeChange(voicing string) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	keys := make([]string, len(plan.Items))
	for i, item := range plan.Items {
		keys[i] = item.ItemKey
	}
	chord, change := slices.Index(keys, chordVoicingKey(voicing)), slices.Index(keys, w.chordDrill().changeKey)
	return expectChord(chord >= 0 && change >= 0 && chord < change, "single chord at %d, change at %d", chord, change)
}

// plannedMinute is the minute of the change a step named.
func (w *world) plannedMinute() (generated.ChangeMinute, error) {
	changes, err := w.planItemsOfKind(generated.PracticeItemKindChordChange)
	if err != nil {
		return generated.ChangeMinute{}, err
	}
	for _, item := range changes {
		if item.ItemKey == w.chordDrill().changeKey && item.ChordChange != nil {
			return item.ChordChange.Minute, nil
		}
	}
	return generated.ChangeMinute{}, fmt.Errorf("the session has no chord change %s", w.chordDrill().changeKey)
}

func (w *world) minuteStartsAt(bpm int) error {
	minute, err := w.plannedMinute()
	if err != nil {
		return err
	}
	return expectChord(minute.StartTempoBpm == bpm, "the minute starts at %d BPM, want %d", minute.StartTempoBpm, bpm)
}

func (w *world) minuteStartsFresh(bpm int) error {
	minute, err := w.plannedMinute()
	if err != nil {
		return err
	}
	return expectChord(minute.StartTempoBpm == bpm && minute.BestCleanChangesPerMinute == nil && minute.LastChangesPerMinute == nil, "minute %+v", minute)
}

func (w *world) minuteStartsWithLastCount(bpm, last int) error {
	minute, err := w.plannedMinute()
	if err != nil {
		return err
	}
	return expectChord(minute.StartTempoBpm == bpm && minute.LastChangesPerMinute != nil && *minute.LastChangesPerMinute == last, "minute %+v", minute)
}

func (w *world) bestAndLastCounts(best, last int) error {
	minute, err := w.plannedMinute()
	if err != nil {
		return err
	}
	return expectChord(minute.BestCleanChangesPerMinute != nil && *minute.BestCleanChangesPerMinute == best &&
		minute.LastChangesPerMinute != nil && *minute.LastChangesPerMinute == last, "minute %+v", minute)
}

func (w *world) noChordItems() error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	for _, item := range plan.Items {
		switch item.Kind {
		case generated.PracticeItemKindChordVoicing, generated.PracticeItemKindChordChange, generated.PracticeItemKindRootFifth, generated.PracticeItemKindRootChange:
			return fmt.Errorf("the session plays %s", item.ItemKey)
		}
	}
	return nil
}

func (w *world) noChordWarmUpOrEnding() error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	for _, item := range plan.Items {
		if (item.Kind == generated.PracticeItemKindChordVoicing || item.Kind == generated.PracticeItemKindChordChange) &&
			(item.Reason == generated.PracticePickReasonWarmUp || item.Reason == generated.PracticePickReasonApplication) {
			return fmt.Errorf("%s is the %s", item.ItemKey, item.Reason)
		}
	}
	return nil
}

func (w *world) atLeastOneNewSingleChord() error {
	return w.sessionIncludesNewSingleChords()
}
