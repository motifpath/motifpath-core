//go:build integration

package bdd

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/oapi-codegen/nullable"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerDiagramPlaybackSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)" creates a diagram named "([^"]+)" on instrument "([^"]+)" at (\d+) BPM in "(\d+)/(\d+)" with fretted positions:$`, w.startsDiagramWithTempo)
	sc.Step(`^the sequence:$`, w.completesDiagramWithSequence)
	sc.Step(`^"([^"]+)" creates a diagram named "([^"]+)" on instrument "([^"]+)" with root note "([^"]+)" and mode "([^"]+)" classified under skills "([^"]+)", concepts "([^"]+)" with fretted positions:$`, w.createsDiagramWithKey)
	sc.Step(`^"([^"]+)" submits a create diagram request on instrument "([^"]+)" with mode "([^"]+)" and no root note$`, w.submitsDiagramWithModeNoRoot)
	sc.Step(`^"([^"]+)" submits a create diagram request on instrument "([^"]+)" with root note "([^"]+)" and mode "([^"]+)"$`, w.submitsDiagramWithRootAndMode)
	sc.Step(`^"([^"]+)" submits a create diagram request on instrument "([^"]+)" with (a sequence .+|a tempo and an empty sequence|a time signature of .+)$`, w.submitsDiagramWithPlaybackProblem)

	sc.Step(`^a custom diagram "([^"]+)" exists on instrument "([^"]+)", created by "([^"]+)", with a 3-step sequence at (\d+) BPM$`, w.aCustomDiagramWithSequence)
	sc.Step(`^a diagram "([^"]+)" exists on instrument "([^"]+)" with a 3-step sequence at (\d+) BPM$`, w.aBasicDiagramWithSequence)
	sc.Step(`^a custom diagram "([^"]+)" exists on instrument "([^"]+)", created by "([^"]+)", with root note "([^"]+)" and mode "([^"]+)"$`, w.aCustomDiagramWithKey)
	sc.Step(`^"([^"]+)" updates diagram "([^"]+)" with the sequence:$`, w.updatesDiagramSequence)
	sc.Step(`^"([^"]+)" updates diagram "([^"]+)" with an empty sequence and no tempo$`, w.removesDiagramPlayback)
	sc.Step(`^"([^"]+)" updates diagram "([^"]+)" replacing its positions with only positions (\d+) and (\d+)$`, w.replacesDiagramPositions)
	sc.Step(`^"([^"]+)" updates diagram "([^"]+)" clearing its mode$`, w.clearsDiagramMode)

	sc.Step(`^the diagram's sequence is:$`, w.diagramSequenceIs)
	sc.Step(`^the diagram's sequence has (\d+) steps$`, w.diagramSequenceHasSteps)
	sc.Step(`^the diagram has an empty sequence$`, w.diagramHasEmptySequence)
	sc.Step(`^the diagram's tempo is (\d+) BPM$`, w.diagramTempoIs)
	sc.Step(`^the diagram has no tempo$`, w.diagramHasNoTempo)
	sc.Step(`^the diagram's time signature is "(\d+)/(\d+)"$`, w.diagramTimeSignatureIs)
	sc.Step(`^the diagram's mode is "([^"]+)"$`, w.diagramModeIs)
	sc.Step(`^the diagram has no mode$`, w.diagramHasNoMode)
	sc.Step(`^position (\d+) sounds in steps (\d+) and (\d+)$`, w.positionSoundsInSteps)
	sc.Step(`^step (\d+) of the diagram's sequence is a rest of (\d+/\d+)$`, w.stepIsRest)
	sc.Step(`^the diagram's step values are "([^"]+)"$`, w.diagramStepValuesAre)

	sc.Step(`^"([^"]+)" adds diagram "([^"]+)" to "([^"]+)" with trigger_at_seconds (\d+) and hide_at_seconds (\d+), playing (.+)$`, w.addsPlayingDiagram)
	sc.Step(`^the item's diagram plays reversed and looping at (\d+) BPM with voice "([^"]+)"$`, w.itemPlaysReversedLooping)
	sc.Step(`^the item's diagram plays as authored, not looping, with no tempo or voice of its own$`, w.itemPlaysWithNoOverrides)
}

// playbackPositionID is the id a playback scenario's position number (from
// 1) gets in the diagram named name, so a sequence table can name it.
func playbackPositionID(name string, number int) uuid.UUID {
	return deterministicUUID("playback-position", fmt.Sprintf("%s-%d", name, number))
}

func (w *world) startsDiagramWithTempo(_, name, instrument string, tempo, beats, beatValue int, table *godog.Table) error {
	positions, err := frettedPositionsFromTable(table)
	if err != nil {
		return err
	}
	for i := range positions {
		id := playbackPositionID(name, i+1)
		positions[i].PositionId = &id
	}
	w.pendingDiagram = &generated.CreateDiagramRequest{
		InstrumentIds: []openapi_types.UUID{instrumentID(instrument)}, Names: english(name), Positions: positions,
		Classification: w.diagramClassification("seeded-skill", "seeded-concept"),
		TempoBpm:       &tempo,
		TimeSignature:  &generated.TimeSignature{Beats: beats, BeatValue: generated.TimeSignatureBeatValue(beatValue)},
	}
	return nil
}

func (w *world) completesDiagramWithSequence(table *godog.Table) error {
	if w.pendingDiagram == nil {
		return fmt.Errorf("a sequence needs a diagram being created by the step before it")
	}
	body := *w.pendingDiagram
	w.pendingDiagram = nil
	steps, err := sequenceFromTable(table, body.Positions)
	if err != nil {
		return err
	}
	body.Sequence = &steps
	return w.createDiagram(body)
}

// sequenceFromTable reads positions/value/strum rows, where positions are
// numbers (from 1) into positions, comma-separated, or empty for a rest.
func sequenceFromTable(table *godog.Table, positions []generated.DiagramPosition) ([]generated.SequenceStep, error) {
	steps := make([]generated.SequenceStep, 0, len(table.Rows)-1)
	for row := 1; row < len(table.Rows); row++ {
		numbers, err := cell(table, row, "positions")
		if err != nil {
			return nil, err
		}
		ids := []uuid.UUID{}
		for _, number := range splitCommaList(numbers) {
			if number == "" {
				continue // an empty positions cell is a rest
			}
			n, err := strconv.Atoi(number)
			if err != nil || n < 1 || n > len(positions) || positions[n-1].PositionId == nil {
				return nil, fmt.Errorf("row %d names position %q, which isn't one of the %d positions", row, number, len(positions))
			}
			ids = append(ids, *positions[n-1].PositionId)
		}
		valueCell, err := cell(table, row, "value")
		if err != nil {
			return nil, err
		}
		value, err := parseNoteValue(valueCell)
		if err != nil {
			return nil, err
		}
		step := generated.SequenceStep{PositionIds: ids, Value: value}
		if strum := optionalCell(table, row, "strum"); strum != "" {
			s := generated.SequenceStepStrum(strum)
			step.Strum = &s
		}
		steps = append(steps, step)
	}
	return steps, nil
}

var noteValuePattern = regexp.MustCompile(`^(\d+)/(\d+)$`)

func parseNoteValue(text string) (generated.NoteValue, error) {
	match := noteValuePattern.FindStringSubmatch(strings.TrimSpace(text))
	if match == nil {
		return generated.NoteValue{}, fmt.Errorf("note value %q is not a fraction like 1/8", text)
	}
	num, err := strconv.Atoi(match[1])
	if err != nil {
		return generated.NoteValue{}, err
	}
	den, err := strconv.Atoi(match[2])
	if err != nil {
		return generated.NoteValue{}, err
	}
	return generated.NoteValue{Num: num, Den: den}, nil
}

func (w *world) createsDiagramWithKey(_, name, instrument, root, mode, skills, concepts string, table *godog.Table) error {
	positions, err := frettedPositionsFromTable(table)
	if err != nil {
		return err
	}
	diagramMode := generated.DiagramMode(mode)
	return w.createDiagram(generated.CreateDiagramRequest{
		InstrumentIds: []openapi_types.UUID{instrumentID(instrument)}, Names: english(name), Positions: positions,
		Classification: w.diagramClassification(skills, concepts), RootNote: &root, Mode: &diagramMode,
	})
}

// playableDiagramRequest is a valid create request on instrument for a
// three-position diagram that plays each position in turn at 90 BPM.
func (w *world) playableDiagramRequest(instrument string) generated.CreateDiagramRequest {
	name := "Playable"
	positions := make([]generated.DiagramPosition, 3)
	for i, coords := range [][2]int{{5, 0}, {4, 2}, {3, 2}} {
		id := playbackPositionID(name, i+1)
		str, fret := coords[0], coords[1]
		positions[i] = generated.DiagramPosition{PositionId: &id, Interval: "R", NoteName: "A", String: &str, Fret: &fret}
	}
	eighth := generated.NoteValue{Num: 1, Den: 8}
	sequence := []generated.SequenceStep{
		{PositionIds: []uuid.UUID{*positions[0].PositionId}, Value: eighth},
		{PositionIds: []uuid.UUID{*positions[1].PositionId}, Value: eighth},
		{PositionIds: []uuid.UUID{*positions[2].PositionId}, Value: eighth},
	}
	tempo := 90
	return generated.CreateDiagramRequest{
		InstrumentIds: []openapi_types.UUID{instrumentID(instrument)}, Names: english(name), Positions: positions,
		Classification: w.diagramClassification("seeded-skill", "seeded-concept"),
		TempoBpm:       &tempo, Sequence: &sequence,
	}
}

func (w *world) submitsDiagramWithModeNoRoot(_, instrument, mode string) error {
	body := w.playableDiagramRequest(instrument)
	diagramMode := generated.DiagramMode(mode)
	body.Mode = &diagramMode
	return w.createDiagram(body)
}

func (w *world) submitsDiagramWithRootAndMode(_, instrument, root, mode string) error {
	body := w.playableDiagramRequest(instrument)
	diagramMode := generated.DiagramMode(mode)
	body.RootNote, body.Mode = &root, &diagramMode
	return w.createDiagram(body)
}

var (
	bpmProblemPattern           = regexp.MustCompile(`^a sequence at (\d+) BPM$`)
	noteValueProblemPattern     = regexp.MustCompile(`^a sequence step with a note value of (\d+)/(\d+)$`)
	strumProblemPattern         = regexp.MustCompile(`^a sequence step with an unrecognised strum "([^"]+)"$`)
	timeSignatureProblemPattern = regexp.MustCompile(`^a time signature of "(\d+)/(\d+)"$`)
)

// submitsDiagramWithPlaybackProblem submits a playable diagram with the one
// thing problem describes wrong about its playback.
func (w *world) submitsDiagramWithPlaybackProblem(_, instrument, problem string) error {
	body := w.playableDiagramRequest(instrument)
	steps := *body.Sequence
	switch {
	case problem == "a sequence step naming a position not in the diagram":
		steps[0].PositionIds = []uuid.UUID{uuid.New()}
	case problem == "a sequence step naming the same position twice":
		steps[0].PositionIds = []uuid.UUID{steps[0].PositionIds[0], steps[0].PositionIds[0]}
	case problem == "a sequence and no tempo":
		body.TempoBpm = nil
	case problem == "a tempo and an empty sequence":
		body.Sequence = nil
	case noteValueProblemPattern.MatchString(problem):
		value, err := parseNoteValue(strings.TrimPrefix(problem, "a sequence step with a note value of "))
		if err != nil {
			return err
		}
		steps[0].Value = value
	case strumProblemPattern.MatchString(problem):
		strum := generated.SequenceStepStrum(strumProblemPattern.FindStringSubmatch(problem)[1])
		steps[0].Strum = &strum
	case bpmProblemPattern.MatchString(problem):
		tempo, err := strconv.Atoi(bpmProblemPattern.FindStringSubmatch(problem)[1])
		if err != nil {
			return err
		}
		body.TempoBpm = &tempo
	case timeSignatureProblemPattern.MatchString(problem):
		signature, err := parseNoteValue(strings.TrimSuffix(strings.TrimPrefix(problem, `a time signature of "`), `"`))
		if err != nil {
			return err
		}
		body.TimeSignature = &generated.TimeSignature{Beats: signature.Num, BeatValue: generated.TimeSignatureBeatValue(signature.Den)}
	default:
		return fmt.Errorf("unknown playback problem %q", problem)
	}
	return w.createDiagram(body)
}

// seedPlayingDiagram seeds the A5 arpeggio — root, fifth and octave — as
// slug, playing each position in turn at tempo BPM.
func (w *world) seedPlayingDiagram(slug, instrumentName string, kind domain.DiagramKind, owner string, tempo int) error {
	instrument, err := w.ensureInstrumentSeeded(instrumentName)
	if err != nil {
		return err
	}
	positions := make([]domain.Position, 3)
	for i, spec := range []struct {
		interval, note string
		str, fret      int
	}{{"R", "A", 5, 0}, {"5", "E", 4, 2}, {"R", "A", 3, 2}} {
		str, fret := spec.str, spec.fret
		positions[i] = domain.Position{ID: playbackPositionID(slug, i+1).String(), Interval: spec.interval, NoteName: spec.note, String: &str, Fret: &fret}
	}
	sequence := []domain.SequenceStep{
		{PositionIDs: []string{positions[0].ID}, Value: domain.NoteValue{Num: 1, Den: 8}},
		{PositionIDs: []string{positions[1].ID}, Value: domain.NoteValue{Num: 1, Den: 8}},
		{PositionIDs: []string{positions[2].ID}, Value: domain.NoteValue{Num: 1, Den: 4}},
	}
	skillID, conceptID := w.skillIDFor("seeded-skill"), w.conceptIDFor("seeded-concept")
	diagram, err := domain.NewDiagram(diagramID(slug).String(), owner, instrument, bothLanguages(slug), offeredLanguages, positions, []string{skillID.String()}, []string{conceptID.String()},
		domain.DiagramOptions{LabelDisplay: domain.LabelDisplayInterval, Kind: kind, TempoBPM: &tempo, Sequence: sequence}, fixedNow)
	if err != nil {
		return fmt.Errorf("seeding diagram %q: %w", slug, err)
	}
	w.diagrams.put(diagram)
	return nil
}

func (w *world) aCustomDiagramWithSequence(slug, instrument, creator string, tempo int) error {
	owner := w.ensureRegistered(creator, domain.RoleTeacher).String()
	return w.seedPlayingDiagram(slug, instrument, domain.DiagramKindCustom, owner, tempo)
}

func (w *world) aBasicDiagramWithSequence(slug, instrument string, tempo int) error {
	return w.seedPlayingDiagram(slug, instrument, domain.DiagramKindBasic, w.curatorID(), tempo)
}

func (w *world) aCustomDiagramWithKey(slug, instrumentName, creator, root, mode string) error {
	owner := w.ensureRegistered(creator, domain.RoleTeacher).String()
	if err := w.seedDiagram(slug, map[string]string{"en": slug}, instrumentName, domain.DiagramKindCustom, owner, &root); err != nil {
		return err
	}
	diagram, err := w.diagrams.GetByID(w.ctx(), diagramID(slug).String())
	if err != nil {
		return err
	}
	diagramMode := domain.DiagramMode(mode)
	diagram.Mode = &diagramMode
	w.diagrams.put(diagram)
	return nil
}

func (w *world) updateDiagram(slug string, body generated.UpdateDiagramRequest) error {
	resp, err := w.handler.UpdateDiagram(w.ctx(), generated.UpdateDiagramRequestObject{DiagramId: diagramID(slug), Body: &body})
	w.lastResp, w.lastErr = resp, err
	return err
}

// storedDiagram reads slug through the API, as the caller of the step.
func (w *world) storedDiagram(slug string) (generated.Diagram, error) {
	resp, err := w.handler.GetDiagram(w.ctx(), generated.GetDiagramRequestObject{DiagramId: diagramID(slug)})
	if err != nil {
		return generated.Diagram{}, err
	}
	diagram, ok := resp.(generated.GetDiagram200JSONResponse)
	if !ok {
		return generated.Diagram{}, fmt.Errorf("expected to read diagram %q, got %#v", slug, resp)
	}
	return generated.Diagram(diagram), nil
}

func (w *world) updatesDiagramSequence(_, slug string, table *godog.Table) error {
	stored, err := w.storedDiagram(slug)
	if err != nil {
		return err
	}
	steps, err := sequenceFromTable(table, stored.Positions)
	if err != nil {
		return err
	}
	return w.updateDiagram(slug, generated.UpdateDiagramRequest{Sequence: &steps})
}

func (w *world) removesDiagramPlayback(_, slug string) error {
	return w.updateDiagram(slug, generated.UpdateDiagramRequest{Sequence: &[]generated.SequenceStep{}, TempoBpm: nullable.NewNullNullable[int]()})
}

// replacesDiagramPositions resends only two of slug's positions, keeping
// the sequence as it is. It remembers the diagram as it was first, for
// "diagram ... is unchanged" to compare against.
func (w *world) replacesDiagramPositions(_, slug string, first, second int) error {
	stored, err := w.storedDiagram(slug)
	if err != nil {
		return err
	}
	w.copySource = stored
	kept := []generated.DiagramPosition{stored.Positions[first-1], stored.Positions[second-1]}
	return w.updateDiagram(slug, generated.UpdateDiagramRequest{Positions: &kept})
}

func (w *world) clearsDiagramMode(_, slug string) error {
	return w.updateDiagram(slug, generated.UpdateDiagramRequest{Mode: nullable.NewNullNullable[generated.DiagramMode]()})
}

// describeSequence renders sequence as "positions value strum" rows, with
// positions numbered (from 1) by their place in positions.
func describeSequence(sequence []generated.SequenceStep, positions []generated.DiagramPosition) []string {
	rows := make([]string, len(sequence))
	for i, step := range sequence {
		numbers := make([]string, len(step.PositionIds))
		for j, id := range step.PositionIds {
			numbers[j] = "?" + id.String()
			for k, p := range positions {
				if p.PositionId != nil && *p.PositionId == id {
					numbers[j] = strconv.Itoa(k + 1)
				}
			}
		}
		strum := "<none given>"
		if step.Strum != nil {
			strum = string(*step.Strum)
		}
		rows[i] = fmt.Sprintf("%s | %d/%d | %s", strings.Join(numbers, ", "), step.Value.Num, step.Value.Den, strum)
	}
	return rows
}

func (w *world) diagramSequenceIs(table *godog.Table) error {
	diagram, err := w.currentDiagram()
	if err != nil {
		return err
	}
	want := make([]string, 0, len(table.Rows)-1)
	for row := 1; row < len(table.Rows); row++ {
		positions, err := cell(table, row, "positions")
		if err != nil {
			return err
		}
		value, err := cell(table, row, "value")
		if err != nil {
			return err
		}
		strum, err := cell(table, row, "strum")
		if err != nil {
			return err
		}
		want = append(want, fmt.Sprintf("%s | %s | %s", strings.Join(splitCommaList(positions), ", "), value, strum))
	}
	if got := describeSequence(diagram.Sequence, diagram.Positions); !slices.Equal(got, want) {
		return fmt.Errorf("expected sequence\n%s\ngot\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
	}
	return nil
}

func (w *world) diagramSequenceHasSteps(count int) error {
	diagram, err := w.currentDiagram()
	if err != nil {
		return err
	}
	if len(diagram.Sequence) != count {
		return fmt.Errorf("expected %d steps, got %d: %v", count, len(diagram.Sequence), describeSequence(diagram.Sequence, diagram.Positions))
	}
	return nil
}

func (w *world) diagramHasEmptySequence() error {
	return w.diagramSequenceHasSteps(0)
}

func (w *world) diagramTempoIs(want int) error {
	diagram, err := w.currentDiagram()
	if err != nil {
		return err
	}
	if diagram.TempoBpm == nil || *diagram.TempoBpm != want {
		return fmt.Errorf("expected a tempo of %d BPM, got %v", want, diagram.TempoBpm)
	}
	return nil
}

func (w *world) diagramHasNoTempo() error {
	diagram, err := w.currentDiagram()
	if err != nil {
		return err
	}
	if diagram.TempoBpm != nil {
		return fmt.Errorf("expected no tempo, got %d BPM", *diagram.TempoBpm)
	}
	return nil
}

func (w *world) diagramTimeSignatureIs(beats, beatValue int) error {
	diagram, err := w.currentDiagram()
	if err != nil {
		return err
	}
	if diagram.TimeSignature.Beats != beats || int(diagram.TimeSignature.BeatValue) != beatValue {
		return fmt.Errorf("expected time signature %d/%d, got %d/%d", beats, beatValue, diagram.TimeSignature.Beats, diagram.TimeSignature.BeatValue)
	}
	return nil
}

func (w *world) diagramModeIs(want string) error {
	diagram, err := w.currentDiagram()
	if err != nil {
		return err
	}
	if diagram.Mode == nil || string(*diagram.Mode) != want {
		return fmt.Errorf("expected mode %q, got %v", want, diagram.Mode)
	}
	return nil
}

func (w *world) diagramHasNoMode() error {
	diagram, err := w.currentDiagram()
	if err != nil {
		return err
	}
	if diagram.Mode != nil {
		return fmt.Errorf("expected no mode, got %q", *diagram.Mode)
	}
	return nil
}

func (w *world) positionSoundsInSteps(position, first, second int) error {
	diagram, err := w.currentDiagram()
	if err != nil {
		return err
	}
	if position < 1 || position > len(diagram.Positions) || diagram.Positions[position-1].PositionId == nil {
		return fmt.Errorf("the diagram has no position %d", position)
	}
	id := *diagram.Positions[position-1].PositionId
	var steps []int
	for i, step := range diagram.Sequence {
		if slices.Contains(step.PositionIds, id) {
			steps = append(steps, i+1)
		}
	}
	if !slices.Equal(steps, []int{first, second}) {
		return fmt.Errorf("expected position %d to sound in steps %d and %d, got %v", position, first, second, steps)
	}
	return nil
}

func (w *world) stepIsRest(number int, value string) error {
	diagram, err := w.currentDiagram()
	if err != nil {
		return err
	}
	if number < 1 || number > len(diagram.Sequence) {
		return fmt.Errorf("the sequence has no step %d", number)
	}
	step := diagram.Sequence[number-1]
	if len(step.PositionIds) != 0 || fmt.Sprintf("%d/%d", step.Value.Num, step.Value.Den) != value {
		return fmt.Errorf("expected step %d to be a rest of %s, got %v", number, value, describeSequence([]generated.SequenceStep{step}, diagram.Positions))
	}
	return nil
}

func (w *world) diagramStepValuesAre(list string) error {
	diagram, err := w.currentDiagram()
	if err != nil {
		return err
	}
	got := make([]string, len(diagram.Sequence))
	for i, step := range diagram.Sequence {
		got[i] = fmt.Sprintf("%d/%d", step.Value.Num, step.Value.Den)
	}
	if want := splitCommaList(list); !slices.Equal(got, want) {
		return fmt.Errorf("expected step values %v, got %v", want, got)
	}
	return nil
}

var (
	reversedLoopingPattern = regexp.MustCompile(`^reversed and looping at (\d+) BPM with voice "([^"]+)"$`)
	voicePlaybackPattern   = regexp.MustCompile(`^with voice "([^"]+)"$`)
	tempoPlaybackPattern   = regexp.MustCompile(`^at (\d+) BPM$`)
	directionPattern       = regexp.MustCompile(`^in direction "([^"]+)"$`)
)

// diagramRefPlayback is the playback field of a generated.DiagramRef.
type diagramRefPlayback = struct {
	Direction *generated.DiagramRefPlaybackDirection `json:"direction,omitempty"`
	Loop      *bool                                  `json:"loop,omitempty"`
	TempoBpm  *int                                   `json:"tempo_bpm"`
	VoiceId   *string                                `json:"voice_id"`
}

// playbackFrom reads how a step says a diagram plays: "with no overrides",
// "reversed and looping at 60 BPM with voice ...", or one override alone.
func playbackFrom(phrase string) (*diagramRefPlayback, error) {
	playback := &diagramRefPlayback{}
	switch {
	case phrase == "with no overrides":
	case reversedLoopingPattern.MatchString(phrase):
		match := reversedLoopingPattern.FindStringSubmatch(phrase)
		tempo, err := strconv.Atoi(match[1])
		if err != nil {
			return nil, err
		}
		direction, loop, voice := generated.DiagramRefPlaybackDirection("reversed"), true, match[2]
		playback.Direction, playback.Loop, playback.TempoBpm, playback.VoiceId = &direction, &loop, &tempo, &voice
	case voicePlaybackPattern.MatchString(phrase):
		voice := voicePlaybackPattern.FindStringSubmatch(phrase)[1]
		playback.VoiceId = &voice
	case tempoPlaybackPattern.MatchString(phrase):
		tempo, err := strconv.Atoi(tempoPlaybackPattern.FindStringSubmatch(phrase)[1])
		if err != nil {
			return nil, err
		}
		playback.TempoBpm = &tempo
	case directionPattern.MatchString(phrase):
		direction := generated.DiagramRefPlaybackDirection(directionPattern.FindStringSubmatch(phrase)[1])
		playback.Direction = &direction
	default:
		return nil, fmt.Errorf("unknown playback %q", phrase)
	}
	return playback, nil
}

func (w *world) addsPlayingDiagram(_, diagramSlug, nodeSlug string, trigger, hide int, phrase string) error {
	playback, err := playbackFrom(phrase)
	if err != nil {
		return err
	}
	ref := diagramRefWithIntervalsLayer(diagramSlug)
	ref.Playback = playback
	resp, err := w.handler.CreateExpandedContent(w.ctx(), generated.CreateExpandedContentRequestObject{
		ContentNodeId: nodeID(nodeSlug),
		Body: &generated.CreateExpandedContentRequest{
			ContentType:      generated.CreateExpandedContentRequestContentTypeDiagram,
			DiagramRef:       &ref,
			TriggerAtSeconds: &trigger, HideAtSeconds: &hide,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) createdItemPlayback() (*diagramRefPlayback, error) {
	resp, ok := w.lastResp.(generated.CreateExpandedContent201JSONResponse)
	if !ok {
		return nil, fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.DiagramRef == nil || resp.DiagramRef.Playback == nil {
		return nil, fmt.Errorf("expected the item's diagram to carry a playback, got %+v", resp.DiagramRef)
	}
	return resp.DiagramRef.Playback, nil
}

func (w *world) itemPlaysReversedLooping(tempo int, voice string) error {
	playback, err := w.createdItemPlayback()
	if err != nil {
		return err
	}
	if playback.Direction == nil || *playback.Direction != "reversed" || playback.Loop == nil || !*playback.Loop ||
		playback.TempoBpm == nil || *playback.TempoBpm != tempo || playback.VoiceId == nil || *playback.VoiceId != voice {
		return fmt.Errorf("expected reversed, looping, %d BPM, voice %q; got %s", tempo, voice, describePlayback(playback))
	}
	return nil
}

func (w *world) itemPlaysWithNoOverrides() error {
	playback, err := w.createdItemPlayback()
	if err != nil {
		return err
	}
	if playback.Direction == nil || *playback.Direction != "as_authored" || (playback.Loop != nil && *playback.Loop) ||
		playback.TempoBpm != nil || playback.VoiceId != nil {
		return fmt.Errorf("expected as authored, not looping, no tempo or voice; got %s", describePlayback(playback))
	}
	return nil
}

// describePlayback renders the fields p sets, for a failure message.
func describePlayback(p *diagramRefPlayback) string {
	var fields []string
	if p.Direction != nil {
		fields = append(fields, "direction="+string(*p.Direction))
	}
	if p.Loop != nil {
		fields = append(fields, "loop="+strconv.FormatBool(*p.Loop))
	}
	if p.TempoBpm != nil {
		fields = append(fields, "tempo_bpm="+strconv.Itoa(*p.TempoBpm))
	}
	if p.VoiceId != nil {
		fields = append(fields, "voice_id="+*p.VoiceId)
	}
	return "{" + strings.Join(fields, " ") + "}"
}
