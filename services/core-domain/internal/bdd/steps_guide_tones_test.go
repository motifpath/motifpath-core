//go:build integration

package bdd

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/cucumber/godog"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// guideToneSkill is the node every guide tone is classified under here.
const guideToneSkill = "target-chord-tones"

func registerGuideToneSteps(sc *godog.ScenarioContext, w *world) {
	// ── The catalog ────────────────────────────────────────────────────────────
	sc.Step(`^the progression catalog has "([^"]+)" with the bars ((?:\S+ ?)+)$`, w.catalogHasProgressionBars)
	sc.Step(`^the progression catalog has "([^"]+)" with (\d+) bars$`, w.catalogHasProgressionOfLength)

	// ── Givens ─────────────────────────────────────────────────────────────────
	sc.Step(`^"([^"]+)"'s path skill "target-chord-tones" has new guide tones on guitar$`, w.hasNewGuideTones)
	sc.Step(`^"([^"]+)" has new guide tones on guitar$`, w.hasNewGuideTones)
	sc.Step(`^"([^"]+)" has new guide tones of every progression on guitar$`, w.hasNewGuideTones)
	sc.Step(`^"([^"]+)" has 10 new guide tones on guitar, and nothing else$`, w.hasOnlyGuideTones)
	sc.Step(`^"([^"]+)" plays only "electric-bass" and has new guide tones$`, w.playsOnlyBassWithGuideTones)
	sc.Step(`^"([^"]+)" has never practised the guide tone of "([^"]+)" on guitar$`, w.neverPractisedGuideTone)
	sc.Step(`^"([^"]+)" is (accurate|fluent) on the guide tone of "([^"]+)" on guitar$`, w.levelOnGuideTone)
	sc.Step(`^"([^"]+)"'s last take of the guide tone of "([^"]+)" on guitar was in ([A-G][b#]?)$`, w.lastGuideToneTakeIn)
	sc.Step(`^"([^"]+)" is learning "target-chord-tones"$`, w.learningTargetChordTones)
	sc.Step(`^"([^"]+)" is accurate on "target-chord-tones"$`, w.accurateOnTargetChordTones)
	sc.Step(`^the only guide tone "([^"]+)" has left to practise on guitar is "([^"]+)"$`, w.onlyGuideToneLeft)
	sc.Step(`^"([^"]+)" has evidence for the guide tone of "([^"]+)" on guitar$`, w.hasEvidenceForGuideTone)

	// ── Composing ──────────────────────────────────────────────────────────────
	sc.Step(`^the guide tone is picked for "([^"]+)"'s session(?: with no instrument in hand)?$`, w.guideTonePicked)
	sc.Step(`^the guide tone is picked for "([^"]+)"'s session (\d+) times$`, w.guideTonePickedTimes)
	sc.Step(`^the guide tone is picked for "([^"]+)"'s session in ([A-G][b#]?)$`, w.guideTonePickedIn)
	sc.Step(`^the guide tone of "([^"]+)" is picked for "([^"]+)"'s session on (guitar|electric bass)$`, w.guideToneOfPickedOn)
	sc.Step(`^"([^"]+)" composes (\d+) sessions with no instrument in hand$`, w.composesSessionsInTheHead)
	sc.Step(`^"([^"]+)" composes a session with no instrument in hand$`, w.composesASessionInTheHead)

	// ── Thens ──────────────────────────────────────────────────────────────────
	sc.Step(`^the session includes a guide tone of "target-chord-tones" with the reason new$`, w.sessionIncludesNewGuideTone)
	sc.Step(`^the guide tone is at the target rung$`, w.guideToneIsAtTheTargetRung)
	sc.Step(`^the session has exactly one guide tone$`, w.exactlyOneGuideTone)
	sc.Step(`^it is estimated at (\d+) takes of a bar of count-in and (\d+) bars at (\d+) BPM, each followed by (\d+) seconds to see the result$`, w.guideToneEstimatedAt)
	sc.Step(`^every guide tone in the session is on the "electric-bass" layout$`, w.everyGuideToneOnBass)
	sc.Step(`^no item in the session is a guide tone$`, w.noGuideTones)
	sc.Step(`^it asks the tone (3|7) at a start tempo of (\d+) BPM, with a target of (\d+) BPM$`, w.asksToneWithTarget)
	sc.Step(`^it asks the tone (3|7) at a start tempo of (\d+) BPM$`, w.asksTone)
	sc.Step(`^it plays over at most (\d+) takes$`, w.guideToneTakes)
	sc.Step(`^it is never planned in ([A-G][b#]?)$`, w.neverPlannedIn)
	sc.Step(`^every key it is planned in is a major key$`, w.everyKeyIsMajor)
	sc.Step(`^its bars are ((?:\S+ ?)+) with the chords "([^"]+)"$`, w.guideToneBarsAre)
	sc.Step(`^it has a voicing for each of "([^"]+)", "([^"]+)" and "([^"]+)"$`, w.guideToneHasVoicings)
	sc.Step(`^the plan names no note to tap and no place on the fretboard$`, w.guideToneGivesNothingAway)
	sc.Step(`^both are asked within frets 0 to 5, the open string included$`, w.bothAskedWithinFretsZeroToFive)
	sc.Step(`^no session plans the guide tone of "([^"]+)"$`, w.noSessionPlansGuideToneOf)
	sc.Step(`^the session plans the guide tone of "([^"]+)" with (\d+) bars$`, w.sessionPlansGuideToneWithBars)
	sc.Step(`^it asks the tone 7 and the bars start with the chord "([^"]+)"$`, w.asksSeventhStartingWith)
	sc.Step(`^no session item is the guide tone of "([^"]+)"$`, w.noSessionItemIsGuideToneOf)
}

// catalogProgression is the progression catalog's entry keyed key.
func catalogProgression(key string) (domain.Progression, error) {
	progressions, err := catalogProgressions{}.List(context.Background())
	if err != nil {
		return domain.Progression{}, err
	}
	i := slices.IndexFunc(progressions, func(p domain.Progression) bool { return p.Key == key })
	if i < 0 {
		return domain.Progression{}, fmt.Errorf("the progression catalog has no %q", key)
	}
	return progressions[i], nil
}

func (w *world) catalogHasProgressionBars(key, bars string) error {
	p, err := catalogProgression(key)
	if err != nil {
		return err
	}
	return expectChord(slices.Equal(p.Bars, strings.Fields(bars)), "%s has the bars %v", key, p.Bars)
}

func (w *world) catalogHasProgressionOfLength(key string, bars int) error {
	p, err := catalogProgression(key)
	if err != nil {
		return err
	}
	return expectChord(len(p.Bars) == bars, "%s has %d bars", key, len(p.Bars))
}

// guideToneKeys are every progression of the catalog as a guide tone on
// instrument's layout.
func guideToneKeys(instrument string) ([]string, error) {
	progressions, err := catalogProgressions{}.List(context.Background())
	if err != nil {
		return nil, err
	}
	keys := make([]string, len(progressions))
	for i, p := range progressions {
		keys[i] = domain.GuideToneItemKey(instrumentID(instrument).String(), p.Key)
	}
	return keys, nil
}

// fourBarGuideToneKeys are guideToneKeys but the 12-bar forms.
func fourBarGuideToneKeys(instrument string) ([]string, error) {
	progressions, err := catalogProgressions{}.List(context.Background())
	if err != nil {
		return nil, err
	}
	var keys []string
	for _, p := range progressions {
		if len(p.Bars) == domain.HeardProgressionBars {
			keys = append(keys, domain.GuideToneItemKey(instrumentID(instrument).String(), p.Key))
		}
	}
	return keys, nil
}

// classifyGuideTones puts guideToneSkill on name's path and classifies every
// guide tone on instrument under it.
func (w *world) classifyGuideTones(name, instrument string) error {
	if err := w.onGuideTonePath(name); err != nil {
		return err
	}
	keys, err := guideToneKeys(instrument)
	if err != nil {
		return err
	}
	w.classifyChords(instrument, guideToneSkill, keys...)
	return nil
}

// onGuideTonePath seeds the guitar and the four-string bass and puts
// guideToneSkill on name's path for both.
func (w *world) onGuideTonePath(name string) error {
	if _, err := w.ensureInstrumentSeeded("guitar"); err != nil {
		return err
	}
	if err := w.seedFourStringBass(); err != nil {
		return err
	}
	w.authenticateAs(name, domain.RoleStudent)
	w.onChordPath(name, guideToneSkill)
	return nil
}

func (w *world) seedFourStringBass() error {
	bass, err := w.ensureInstrumentSeeded("electric-bass")
	if err != nil {
		return err
	}
	four := 4
	bass.StringCount, bass.Tuning = &four, []string{"E1", "A1", "D2", "G2"}
	w.instruments.put(bass)
	return nil
}

func (w *world) hasNewGuideTones(name string) error {
	return w.classifyGuideTones(name, "guitar")
}

// hasOnlyGuideTones leaves the path's play-alongs out, so only the guide tones
// can be practised.
func (w *world) hasOnlyGuideTones(name string) error {
	w.diagrams.mu.Lock()
	delete(w.diagrams.byID, diagramID("guitar-lick").String())
	w.diagrams.mu.Unlock()
	return w.hasNewGuideTones(name)
}

// playsOnlyBassWithGuideTones gives name a path for electric bass alone, its
// one lesson teaching guideToneSkill, and every guide tone on the bass.
func (w *world) playsOnlyBassWithGuideTones(name string) error {
	if err := w.seedFourStringBass(); err != nil {
		return err
	}
	// The session is composed as name, whose one instrument is the bass.
	w.authenticateAs(name, domain.RoleStudent)
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	node := domain.ContentNode{
		ID: nodeID("bass-" + guideToneSkill).String(), Title: "bass-" + guideToneSkill, ContentType: domain.ContentTypeArticle,
		Classification: domain.Classification{Skills: []domain.KnowledgeNode{{ID: w.skillIDFor(guideToneSkill).String()}}},
	}
	w.nodes.put(node)
	w.studentPaths.put(domain.StudentPath{
		ID: deterministicUUID("student-path", name+"/bass-"+guideToneSkill).String(), StudentID: studentID,
		SourceTemplateID: pathID("bass-" + guideToneSkill).String(), AssignedAt: fixedNow,
		Items: []domain.StudentPathItemRecord{{Position: 1, ContentNodeID: node.ID}},
	})
	w.paths.put(domain.LearningPath{ID: pathID("bass-" + guideToneSkill).String(), InstrumentIDs: []string{instrumentID("electric-bass").String()}})
	keys, err := guideToneKeys("electric-bass")
	if err != nil {
		return err
	}
	w.classifyChords("electric-bass", guideToneSkill, keys...)
	return nil
}

// neverPractisedGuideTone puts the one guide tone on name's path.
func (w *world) neverPractisedGuideTone(name, progression string) error {
	if err := w.onGuideTonePath(name); err != nil {
		return err
	}
	w.guideToneKey = domain.GuideToneItemKey(instrumentID("guitar").String(), progression)
	w.classifyChords("guitar", guideToneSkill, w.guideToneKey)
	return nil
}

func (w *world) levelOnGuideTone(name, level, progression string) error {
	if err := w.neverPractisedGuideTone(name, progression); err != nil {
		return err
	}
	w.putItemStates(name, domain.KnowledgeLevel(level), 5, 2, 0, w.guideToneKey)
	return nil
}

func (w *world) lastGuideToneTakeIn(name, progression, key string) error {
	if err := w.neverPractisedGuideTone(name, progression); err != nil {
		return err
	}
	w.putGuideToneState(name, w.guideToneKey, domain.KnowledgeLevelLearning, key)
	return nil
}

// putGuideToneState makes itemKey due at level, its last take in lastKey.
func (w *world) putGuideToneState(name, itemKey string, level domain.KnowledgeLevel, lastKey string) {
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	dueAt := fixedNow.AddDate(0, 0, -1)
	w.practiceStates.put(studentID, domain.PracticeItemState{
		ItemKey: itemKey, RulesVersion: domain.PracticeRulesVersion, Level: level,
		Counted: 2, Box: 1, DueAt: &dueAt, LastAt: &fixedNow, LastMusicKey: lastKey,
	})
}

// learningTargetChordTones has name learning every four-bar guide tone on
// guitar, so the node stands at learning.
func (w *world) learningTargetChordTones(name string) error {
	keys, err := fourBarGuideToneKeys("guitar")
	if err != nil {
		return err
	}
	w.putItemStates(name, domain.KnowledgeLevelLearning, 1, 1, 3, keys...)
	return nil
}

// accurateOnTargetChordTones has name fluent on every four-bar guide tone on
// guitar and not due, which stands the node at accurate or above.
func (w *world) accurateOnTargetChordTones(name string) error {
	if err := w.hasNewGuideTones(name); err != nil {
		return err
	}
	keys, err := fourBarGuideToneKeys("guitar")
	if err != nil {
		return err
	}
	w.knownItems(name, 10, keys...)
	return nil
}

// onlyGuideToneLeft leaves progression the one guide tone name hasn't made
// fluent on guitar.
func (w *world) onlyGuideToneLeft(name, progression string) error {
	keys, err := guideToneKeys("guitar")
	if err != nil {
		return err
	}
	left := domain.GuideToneItemKey(instrumentID("guitar").String(), progression)
	w.knownItems(name, 10, slices.DeleteFunc(keys, func(k string) bool { return k == left })...)
	return nil
}

func (w *world) hasEvidenceForGuideTone(name, progression string) error {
	if err := w.hasNewGuideTones(name); err != nil {
		return err
	}
	w.guideToneKey = domain.GuideToneItemKey(instrumentID("guitar").String(), progression)
	w.classifyChords("guitar", guideToneSkill, w.guideToneKey)
	w.dueItems(name, w.guideToneKey)
	return nil
}

// guideTonePicked composes name's session in the head with the guide tone due
// unless a step already set its state, so it is picked whatever else is new.
func (w *world) guideTonePicked(name string) error {
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	if states, err := w.practiceStates.GetStates(context.Background(), studentID, []string{w.guideToneKey}); err == nil && len(states) == 0 {
		w.dueItems(name, w.guideToneKey)
	}
	w.authenticateAs(name, domain.RoleStudent)
	return w.composeSession(30, nil)
}

func (w *world) guideTonePickedTimes(name string, times int) error {
	w.guideToneKeys = nil
	for range times {
		if err := w.guideTonePicked(name); err != nil {
			return err
		}
		g, err := w.theGuideTone()
		if err != nil {
			return err
		}
		w.guideToneKeys = append(w.guideToneKeys, string(g.MusicKey))
	}
	return nil
}

// guideTonePickedIn composes until the guide tone is planned in key; a key is
// picked at random, so it takes a few tries at most.
func (w *world) guideTonePickedIn(name, key string) error {
	for range 200 {
		if err := w.guideTonePicked(name); err != nil {
			return err
		}
		g, err := w.theGuideTone()
		if err != nil {
			return err
		}
		if string(g.MusicKey) == key {
			return nil
		}
	}
	return fmt.Errorf("the guide tone was never planned in %s", key)
}

// guideToneOfPickedOn plans name's guide tone of progression on instrument and
// keeps it, so a later step can compare it with another student's.
func (w *world) guideToneOfPickedOn(progression, name, instrument string) error {
	instrument = strings.ReplaceAll(instrument, " ", "-")
	if err := w.onGuideTonePath(name); err != nil {
		return err
	}
	w.guideToneKey = domain.GuideToneItemKey(instrumentID(instrument).String(), progression)
	w.classifyChords(instrument, guideToneSkill, w.guideToneKey)
	if err := w.guideTonePicked(name); err != nil {
		return err
	}
	g, err := w.theGuideTone()
	if err != nil {
		return err
	}
	w.pickedGuideTones = append(w.pickedGuideTones, *g)
	return nil
}

func (w *world) composesSessionsInTheHead(name string, sessions int) error {
	w.guideTonePlans = nil
	for range sessions {
		if err := w.composesASessionInTheHead(name); err != nil {
			return err
		}
		if plan, ok := w.lastResp.(generated.CreatePracticeSessionPlan200JSONResponse); ok {
			w.guideTonePlans = append(w.guideTonePlans, plan)
		}
	}
	return nil
}

func (w *world) composesASessionInTheHead(name string) error {
	w.authenticateAs(name, domain.RoleStudent)
	return w.composeSession(30, nil)
}

func (w *world) plannedGuideTones() ([]generated.PracticeSessionItem, error) {
	return w.planItemsOfKind(generated.PracticeItemKindGuideTone)
}

// theGuideTone is the planned guide tone the scenario named.
func (w *world) theGuideTone() (*generated.PlannedGuideTone, error) {
	tones, err := w.plannedGuideTones()
	if err != nil {
		return nil, err
	}
	for _, item := range tones {
		if item.ItemKey == w.guideToneKey && item.GuideTone != nil {
			return item.GuideTone, nil
		}
	}
	return nil, fmt.Errorf("the session has no guide tone %s, of %d guide tones", w.guideToneKey, len(tones))
}

func (w *world) sessionIncludesNewGuideTone() error {
	tones, err := w.plannedGuideTones()
	if err != nil {
		return err
	}
	if len(tones) == 0 || tones[0].Reason != generated.PracticePickReasonNew {
		return fmt.Errorf("the session has no guide tone with the reason new, of %d", len(tones))
	}
	return nil
}

func (w *world) guideToneIsAtTheTargetRung() error {
	return w.everyChordOfKind(generated.PracticeItemKindGuideTone, func(item generated.PracticeSessionItem) error {
		return expectChord(item.Rung == generated.Target, "rung %s", item.Rung)
	})()
}

func (w *world) exactlyOneGuideTone() error {
	tones, err := w.plannedGuideTones()
	if err != nil {
		return err
	}
	return expectChord(len(tones) == 1, "the session has %d guide tones, want 1", len(tones))
}

func (w *world) guideToneEstimatedAt(takes, bars, bpm, resultSeconds int) error {
	want := takes * ((bars+1)*4*60/bpm + resultSeconds)
	return w.everyChordEstimatedAt(generated.PracticeItemKindGuideTone)(want)
}

func (w *world) everyGuideToneOnBass() error {
	bass := openapi_types.UUID(instrumentID("electric-bass"))
	return w.everyChordOfKind(generated.PracticeItemKindGuideTone, func(item generated.PracticeSessionItem) error {
		return expectChord(item.GuideTone != nil && item.GuideTone.LayoutInstrumentId == bass, "%s is not on the bass layout", item.ItemKey)
	})()
}

func (w *world) noGuideTones() error {
	tones, err := w.plannedGuideTones()
	if err != nil {
		return err
	}
	return expectChord(len(tones) == 0, "the session has %d guide tones", len(tones))
}

func (w *world) asksTone(tone string, bpm int) error {
	g, err := w.theGuideTone()
	if err != nil {
		return err
	}
	return expectChord(string(g.Tone) == tone && g.TempoBpm == bpm, "it asks the tone %s at %d BPM", g.Tone, g.TempoBpm)
}

func (w *world) asksToneWithTarget(tone string, bpm, target int) error {
	if err := w.asksTone(tone, bpm); err != nil {
		return err
	}
	g, err := w.theGuideTone()
	if err != nil {
		return err
	}
	return expectChord(g.TargetTempoBpm == target, "its target is %d BPM", g.TargetTempoBpm)
}

func (w *world) guideToneTakes(takes int) error {
	g, err := w.theGuideTone()
	if err != nil {
		return err
	}
	return expectChord(g.Takes == takes, "it plays over %d takes", g.Takes)
}

func (w *world) neverPlannedIn(key string) error {
	return expectChord(!slices.Contains(w.guideToneKeys, key), "it was planned in %v", w.guideToneKeys)
}

func (w *world) everyKeyIsMajor() error {
	for _, k := range w.guideToneKeys {
		if !slices.Contains(domain.MusicKeys, k) {
			return fmt.Errorf("%s is no major key", k)
		}
	}
	return expectChord(len(w.guideToneKeys) > 0, "it was never planned")
}

func (w *world) guideToneBarsAre(numerals, chords string) error {
	g, err := w.theGuideTone()
	if err != nil {
		return err
	}
	var gotNumerals, gotChords []string
	for _, b := range g.Bars {
		gotNumerals, gotChords = append(gotNumerals, b.Numeral), append(gotChords, b.Chord)
	}
	return expectChord(slices.Equal(gotNumerals, strings.Fields(numerals)) && slices.Equal(gotChords, strings.Fields(chords)),
		"its bars are %v with the chords %v", gotNumerals, gotChords)
}

func (w *world) guideToneHasVoicings(first, second, third string) error {
	g, err := w.theGuideTone()
	if err != nil {
		return err
	}
	for _, chord := range []string{first, second, third} {
		if !slices.ContainsFunc(g.Voicings, func(v generatedGuideToneVoicing) bool { return v.Chord == chord }) {
			return fmt.Errorf("%s has no voicing", chord)
		}
	}
	return nil
}

// guideToneGivesNothingAway checks the planned guide tone carries only what
// asks the question: no note name, string or place anywhere in it.
func (w *world) guideToneGivesNothingAway() error {
	g, err := w.theGuideTone()
	if err != nil {
		return err
	}
	raw, err := json.Marshal(g)
	if err != nil {
		return err
	}
	for _, giveaway := range []string{`"note_name"`, `"string"`, `"fret"`, `"places"`, `"cells"`} {
		if strings.Contains(string(raw), giveaway) {
			return fmt.Errorf("the plan carries %s: %s", giveaway, raw)
		}
	}
	return nil
}

func (w *world) bothAskedWithinFretsZeroToFive() error {
	if len(w.pickedGuideTones) != 2 {
		return fmt.Errorf("%d guide tones were picked, want 2", len(w.pickedGuideTones))
	}
	for _, g := range w.pickedGuideTones {
		if g.FretWindow.FromFret != generated.FretWindowFromFretN0 {
			return fmt.Errorf("a guide tone is asked from fret %d", g.FretWindow.FromFret)
		}
	}
	return nil
}

func (w *world) noSessionPlansGuideToneOf(progression string) error {
	for _, plan := range w.guideTonePlans {
		for _, item := range plan.Items {
			if item.GuideTone != nil && item.GuideTone.Progression.Key == progression {
				return fmt.Errorf("a session plans the guide tone of %s", progression)
			}
		}
	}
	return nil
}

func (w *world) sessionPlansGuideToneWithBars(progression string, bars int) error {
	tones, err := w.plannedGuideTones()
	if err != nil {
		return err
	}
	for _, item := range tones {
		if item.GuideTone != nil && item.GuideTone.Progression.Key == progression {
			return expectChord(len(item.GuideTone.Bars) == bars, "it has %d bars", len(item.GuideTone.Bars))
		}
	}
	return fmt.Errorf("the session plans no guide tone of %s", progression)
}

func (w *world) asksSeventhStartingWith(chord string) error {
	g, err := w.theGuideTone()
	if err != nil {
		return err
	}
	return expectChord(g.Tone == "7" && len(g.Bars) > 0 && g.Bars[0].Chord == chord, "it asks the tone %s, starting on %v", g.Tone, g.Bars)
}

func (w *world) noSessionItemIsGuideToneOf(progression string) error {
	if w.lastErr != nil {
		// A session that can't be composed plans no guide tone either.
		return nil
	}
	plan, ok := w.lastResp.(generated.CreatePracticeSessionPlan200JSONResponse)
	if !ok {
		return nil
	}
	for _, item := range plan.Items {
		if item.GuideTone != nil && item.GuideTone.Progression.Key == progression {
			return fmt.Errorf("the session plans the guide tone of %s", progression)
		}
	}
	return nil
}

// The generated guide-tone voicing type is anonymous; this alias names it.
type generatedGuideToneVoicing = struct {
	Chord     string             `json:"chord"`
	DiagramId openapi_types.UUID `json:"diagram_id"`
}
