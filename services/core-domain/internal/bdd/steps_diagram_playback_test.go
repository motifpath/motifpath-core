//go:build integration

package bdd

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerDiagramPlaybackSteps(sc *godog.ScenarioContext, w *world) {
	sc.StepContext().Before(w.sendPendingDiagram)
	sc.Step(`^"([^"]+)" creates a diagram named "([^"]+)" on instrument "([^"]+)" with fretted positions:$`, w.startsDiagram)
	sc.Step(`^a playback "([^"]+)" at (\d+) BPM(?: in "(\d+/\d+)")?:$`, w.addsPendingPlayback)
	sc.Step(`^"([^"]+)" creates diagram "([^"]+)" with playbacks "([^"]+)" and no default playback$`, w.createsChordWithPlaybacks)
	sc.Step(`^"([^"]+)" creates diagram "([^"]+)" with playbacks "([^"]+)" and default playback "([^"]+)"$`, w.createsChordWithDefaultPlayback)
	sc.Step(`^"([^"]+)" creates a diagram named "([^"]+)" in "([^"]+)" and "([^"]+)" in "([^"]+)" with a playback named "([^"]+)" in "([^"]+)" and "([^"]+)" in "([^"]+)"$`, w.createsDiagramWithBilingualPlayback)
	sc.Step(`^"([^"]+)" creates a diagram named "([^"]+)" on instrument "([^"]+)" with root note "([^"]+)" and mode "([^"]+)" classified under skills "([^"]+)", concepts "([^"]+)" with fretted positions:$`, w.createsDiagramWithKey)
	sc.Step(`^"([^"]+)" submits a create diagram request on instrument "([^"]+)" with mode "([^"]+)" and no root note$`, w.submitsDiagramWithModeNoRoot)
	sc.Step(`^"([^"]+)" submits a create diagram request on instrument "([^"]+)" with root note "([^"]+)" and mode "([^"]+)"$`, w.submitsDiagramWithRootAndMode)
	sc.Step(`^"([^"]+)" submits a create diagram request on instrument "([^"]+)" with (a playback .+|two playbacks .+|17 playbacks|a default playback id .+)$`, w.submitsDiagramWithPlaybackProblem)

	sc.Step(`^a custom diagram "([^"]+)" exists on instrument "([^"]+)", created by "([^"]+)", with playbacks "([^"]+)"$`, w.aCustomDiagramWithPlaybacks)
	sc.Step(`^a custom diagram "([^"]+)" exists on instrument "([^"]+)", created by "([^"]+)", with playbacks "([^"]+)" and default playback "([^"]+)"$`, w.aCustomDiagramWithDefaultPlayback)
	sc.Step(`^a diagram "([^"]+)" exists on instrument "([^"]+)" with playbacks "([^"]+)"$`, w.aBasicDiagramWithPlaybacks)
	sc.Step(`^a diagram "([^"]+)" exists on instrument "([^"]+)" with one 3-step playback at (\d+) BPM$`, w.aBasicDiagramWithOnePlayback)
	sc.Step(`^a custom diagram "([^"]+)" exists on instrument "([^"]+)", created by "([^"]+)", with root note "([^"]+)" and mode "([^"]+)"$`, w.aCustomDiagramWithKey)
	sc.Step(`^"([^"]+)" updates diagram "([^"]+)" with only the playback "([^"]+)" at (\d+) BPM in "(\d+/\d+)":$`, w.updatesWithOnlyPlayback)
	sc.Step(`^"([^"]+)" updates diagram "([^"]+)" resending playback "([^"]+)" with its id at (\d+) BPM, without "([^"]+)"$`, w.resendsPlaybackWithItsID)
	sc.Step(`^"([^"]+)" updates diagram "([^"]+)" adding a playback "([^"]+)" first and resending "([^"]+)" with their ids$`, w.addsPlaybackFirst)
	sc.Step(`^"([^"]+)" updates diagram "([^"]+)" setting the default playback to "([^"]+)"$`, w.setsDefaultPlayback)
	sc.Step(`^"([^"]+)" updates diagram "([^"]+)" with no playbacks$`, w.removesEveryPlayback)
	sc.Step(`^"([^"]+)" updates diagram "([^"]+)" replacing its positions with only positions (\d+) and (\d+)$`, w.replacesDiagramPositions)
	sc.Step(`^"([^"]+)" updates diagram "([^"]+)" clearing its mode$`, w.clearsDiagramMode)

	sc.Step(`^the diagram has no playbacks$`, w.diagramHasNoPlaybacks)
	sc.Step(`^the diagram has no default playback$`, w.diagramHasNoDefaultPlayback)
	sc.Step(`^the diagram has (\d+) playbacks?$`, w.diagramHasPlaybackCount)
	sc.Step(`^the diagram has (\d+) playbacks, in the order "([^"]+)"$`, w.diagramHasPlaybacksInOrder)
	sc.Step(`^the diagram's default playback is "([^"]+)"$`, w.diagramDefaultPlaybackIs)
	sc.Step(`^playback "([^"]+)" is at (\d+) BPM in "(\d+)/(\d+)"$`, w.playbackTempoAndMeterAre)
	sc.Step(`^playback "([^"]+)" has the steps:$`, w.playbackStepsAre)
	sc.Step(`^playback "([^"]+)" keeps its id$`, w.playbackKeepsItsID)
	sc.Step(`^in playback "([^"]+)", position (\d+) sounds in steps (\d+) and (\d+)$`, w.positionSoundsInSteps)
	sc.Step(`^step (\d+) of playback "([^"]+)" is a rest of (\d+/\d+)$`, w.stepIsRest)
	sc.Step(`^the step values of playback "([^"]+)" are "([^"]+)"$`, w.playbackStepValuesAre)
	sc.Step(`^the diagram's playback is named "([^"]+)" in "([^"]+)" and "([^"]+)" in "([^"]+)"$`, w.playbackIsNamed)
	sc.Step(`^the diagram's mode is "([^"]+)"$`, w.diagramModeIs)
	sc.Step(`^the diagram has no mode$`, w.diagramHasNoMode)

	sc.Step(`^"([^"]+)" adds diagram "([^"]+)" to "([^"]+)" with trigger_at_seconds (\d+) and hide_at_seconds (\d+), playing (.+)$`, w.addsPlayingDiagram)
	sc.Step(`^a video content node "([^"]+)" has diagram "([^"]+)" as expanded content, playing playback "([^"]+)"$`, w.nodeHasDiagramPlaying)
	sc.Step(`^the playback "([^"]+)" is removed from diagram "([^"]+)"$`, w.playbackIsRemoved)
	sc.Step(`^the item's diagram plays reversed and looping at (\d+) BPM with voice "([^"]+)"$`, w.itemPlaysReversedLooping)
	sc.Step(`^the item's diagram plays its default playback as authored, not looping, with no tempo or voice of its own$`, w.itemPlaysWithNoOverrides)
	sc.Step(`^the item's diagram plays playback "([^"]+)"$`, w.itemPlaysPlayback)
	sc.Step(`^the item's diagram still names playback "([^"]+)"$`, w.itemStillNamesPlayback)
}

// playbackPositionID is the id a playback scenario's position number (from
// 1) gets in the diagram named name, so a step table can name it.
func playbackPositionID(name string, number int) uuid.UUID {
	return deterministicUUID("playback-position", fmt.Sprintf("%s-%d", name, number))
}

// playbackID is the id a step gives the playback named name of the diagram
// with id diagram, so a later step can find it, or check it was kept, from
// the diagram's id alone.
func playbackID(diagram, name string) uuid.UUID {
	return deterministicUUID("playback", diagram, name)
}

// startsDiagram begins a create request for name with positions from table,
// numbered from 1 for the "a playback" steps after it. The request is sent
// before the first step that isn't one of those (see sendPendingDiagram).
func (w *world) startsDiagram(_, name, instrument string, table *godog.Table) error {
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
	}
	return nil
}

// pendingPlaybackStep matches the steps that add a playback to a diagram
// being created; any other step sends the pending create request first.
var pendingPlaybackStep = regexp.MustCompile(`^a playback "[^"]+" at \d+ BPM`)

func (w *world) sendPendingDiagram(ctx context.Context, step *godog.Step) (context.Context, error) {
	if w.pendingDiagram == nil || pendingPlaybackStep.MatchString(step.Text) {
		return ctx, nil
	}
	body := *w.pendingDiagram
	w.pendingDiagram = nil
	return ctx, w.createDiagram(body)
}

// addsPendingPlayback adds a playback named in English to the diagram being
// created, with steps from table; signature is "beats/beat_value", or empty
// to leave the time signature to its default.
func (w *world) addsPendingPlayback(name, tempoText, signature string, table *godog.Table) error {
	if w.pendingDiagram == nil {
		return fmt.Errorf("a playback needs a diagram being created by a step before it")
	}
	tempo, err := strconv.Atoi(tempoText)
	if err != nil {
		return err
	}
	playback, err := playbackInput(name, english(name), tempo, signature, table, w.pendingDiagram.Positions)
	if err != nil {
		return err
	}
	var playbacks []generated.DiagramPlaybackInput
	if w.pendingDiagram.Playbacks != nil {
		playbacks = *w.pendingDiagram.Playbacks
	}
	playbacks = append(playbacks, playback)
	w.pendingDiagram.Playbacks = &playbacks
	return nil
}

// playbackInput is a playback named names at tempo, in signature (empty
// for none given), with steps from table over positions.
func playbackInput(name string, names generated.LocalizedNames, tempo int, signature string, table *godog.Table, positions []generated.DiagramPosition) (generated.DiagramPlaybackInput, error) {
	steps, err := stepsFromTable(table, positions)
	if err != nil {
		return generated.DiagramPlaybackInput{}, fmt.Errorf("playback %q: %w", name, err)
	}
	playback := generated.DiagramPlaybackInput{Names: names, TempoBpm: tempo, Steps: steps}
	if signature != "" {
		meter, err := parseNoteValue(signature)
		if err != nil {
			return generated.DiagramPlaybackInput{}, err
		}
		playback.TimeSignature = &generated.TimeSignature{Beats: meter.Num, BeatValue: generated.TimeSignatureBeatValue(meter.Den)}
	}
	return playback, nil
}

// stepsFromTable reads positions/value/strum rows, where positions are
// numbers (from 1) into positions, comma-separated, or empty for a rest.
func stepsFromTable(table *godog.Table, positions []generated.DiagramPosition) ([]generated.SequenceStep, error) {
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

// a5Positions are the A5 chord's root, fifth and octave, with ids for the
// diagram named name.
func a5Positions(name string) []generated.DiagramPosition {
	positions := make([]generated.DiagramPosition, 3)
	for i, spec := range []struct {
		interval, note string
		str, fret      int
	}{{"R", "A", 5, 0}, {"5", "E", 4, 2}, {"R", "A", 3, 2}} {
		id, str, fret := playbackPositionID(name, i+1), spec.str, spec.fret
		positions[i] = generated.DiagramPosition{PositionId: &id, Interval: generated.DiagramPositionInterval(spec.interval), NoteName: spec.note, String: &str, Fret: &fret}
	}
	return positions
}

// a5Playback is the playback named name over the A5 positions: a strum of
// all three when it is "Strum", each position in turn otherwise.
func a5Playback(name string, names generated.LocalizedNames, positions []generated.DiagramPosition) generated.DiagramPlaybackInput {
	ids := make([]uuid.UUID, len(positions))
	for i, p := range positions {
		ids[i] = *p.PositionId
	}
	if name == "Strum" {
		down := generated.SequenceStepStrum("down")
		return generated.DiagramPlaybackInput{Names: names, TempoBpm: 90, Steps: []generated.SequenceStep{{PositionIds: ids, Value: generated.NoteValue{Num: 1, Den: 2}, Strum: &down}}}
	}
	steps := make([]generated.SequenceStep, len(ids))
	for i, id := range ids {
		steps[i] = generated.SequenceStep{PositionIds: []uuid.UUID{id}, Value: generated.NoteValue{Num: 1, Den: 8}}
	}
	return generated.DiagramPlaybackInput{Names: names, TempoBpm: 70, Steps: steps}
}

// chordRequest is a create request for the A5 chord named name on guitar,
// with a playback for each of names.
func (w *world) chordRequest(name, playbackNames string) generated.CreateDiagramRequest {
	positions := a5Positions(name)
	playbacks := []generated.DiagramPlaybackInput{}
	for _, playback := range splitCommaList(playbackNames) {
		playbacks = append(playbacks, a5Playback(playback, english(playback), positions))
	}
	return generated.CreateDiagramRequest{
		InstrumentIds: []openapi_types.UUID{instrumentID("guitar")}, Names: english(name), Positions: positions,
		Classification: w.diagramClassification("seeded-skill", "seeded-concept"), Playbacks: &playbacks,
	}
}

func (w *world) createsChordWithPlaybacks(_, name, playbacks string) error {
	return w.createDiagram(w.chordRequest(name, playbacks))
}

// createsChordWithDefaultPlayback gives each playback an id, so the request
// can name the one that is the default.
func (w *world) createsChordWithDefaultPlayback(_, name, playbacks, defaultName string) error {
	body := w.chordRequest(name, playbacks)
	for i := range *body.Playbacks {
		playback := &(*body.Playbacks)[i]
		id := playbackID(name, playback.Names["en"])
		playback.PlaybackId = &id
		if playback.Names["en"] == defaultName {
			body.DefaultPlaybackId = &id
		}
	}
	return w.createDiagram(body)
}

func (w *world) createsDiagramWithBilingualPlayback(_, name, lang1, name2, lang2, playback, playbackLang1, playback2, playbackLang2 string) error {
	body := w.chordRequest(name, "")
	body.Names = generated.LocalizedNames{lang1: name, lang2: name2}
	body.Playbacks = &[]generated.DiagramPlaybackInput{a5Playback(playback, generated.LocalizedNames{playbackLang1: playback, playbackLang2: playback2}, body.Positions)}
	return w.createDiagram(body)
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

// playableDiagramRequest is a valid create request on instrument for the A5
// chord with one playback, "Arpeggio", playing each position in turn.
func (w *world) playableDiagramRequest(instrument string) generated.CreateDiagramRequest {
	body := w.chordRequest("Playable", "Arpeggio")
	body.InstrumentIds = []openapi_types.UUID{instrumentID(instrument)}
	return body
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
	bpmProblemPattern           = regexp.MustCompile(`^a playback at (\d+) BPM$`)
	noteValueProblemPattern     = regexp.MustCompile(`^a playback step with a note value of (\d+/\d+)$`)
	strumProblemPattern         = regexp.MustCompile(`^a playback step with an unrecognised strum "([^"]+)"$`)
	timeSignatureProblemPattern = regexp.MustCompile(`^a playback with a time signature of "(\d+/\d+)"$`)
	sameNameProblemPattern      = regexp.MustCompile(`^two playbacks both named "([^"]+)" in "([^"]+)"$`)
	fewerLanguagesProblem       = regexp.MustCompile(`^a playback named only in "([^"]+)" on a diagram named in "([^"]+)" and "([^"]+)"$`)
)

// submitsDiagramWithPlaybackProblem submits a playable diagram with the one
// thing problem describes wrong about its playbacks.
func (w *world) submitsDiagramWithPlaybackProblem(_, instrument, problem string) error {
	body := w.playableDiagramRequest(instrument)
	playback := &(*body.Playbacks)[0]
	steps := playback.Steps
	switch {
	case problem == "a playback step naming a position not in the diagram":
		steps[0].PositionIds = []uuid.UUID{uuid.New()}
	case problem == "a playback step naming the same position twice":
		steps[0].PositionIds = []uuid.UUID{steps[0].PositionIds[0], steps[0].PositionIds[0]}
	case problem == "a playback with no steps":
		playback.Steps = []generated.SequenceStep{}
	case problem == "a playback with no tempo":
		playback.TempoBpm = 0
	case problem == "two playbacks with the same id":
		id := uuid.New()
		second := a5Playback("Strum", english("Strum"), body.Positions)
		playback.PlaybackId, second.PlaybackId = &id, &id
		*body.Playbacks = append(*body.Playbacks, second)
	case problem == "17 playbacks":
		for i := len(*body.Playbacks); i < 17; i++ {
			*body.Playbacks = append(*body.Playbacks, a5Playback("Take", english(fmt.Sprintf("Take %d", i)), body.Positions))
		}
	case problem == "a default playback id that is none of its playbacks":
		id := uuid.New()
		body.DefaultPlaybackId = &id
	case problem == "a default playback id and no playbacks":
		id := uuid.New()
		body.Playbacks, body.DefaultPlaybackId = nil, &id
	case noteValueProblemPattern.MatchString(problem):
		value, err := parseNoteValue(noteValueProblemPattern.FindStringSubmatch(problem)[1])
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
		playback.TempoBpm = tempo
	case timeSignatureProblemPattern.MatchString(problem):
		meter, err := parseNoteValue(timeSignatureProblemPattern.FindStringSubmatch(problem)[1])
		if err != nil {
			return err
		}
		playback.TimeSignature = &generated.TimeSignature{Beats: meter.Num, BeatValue: generated.TimeSignatureBeatValue(meter.Den)}
	case sameNameProblemPattern.MatchString(problem):
		match := sameNameProblemPattern.FindStringSubmatch(problem)
		playback.Names = generated.LocalizedNames{match[2]: match[1]}
		second := a5Playback("Strum", generated.LocalizedNames{match[2]: match[1]}, body.Positions)
		*body.Playbacks = append(*body.Playbacks, second)
	case fewerLanguagesProblem.MatchString(problem):
		match := fewerLanguagesProblem.FindStringSubmatch(problem)
		body.Names = generated.LocalizedNames{match[2]: "Playable", match[3]: "Tocável"}
		playback.Names = generated.LocalizedNames{match[1]: "Arpeggio"}
	default:
		return fmt.Errorf("unknown playback problem %q", problem)
	}
	return w.createDiagram(body)
}

// seedChord seeds the A5 chord as slug, named in every language, with a
// playback for each of playbackNames, each with the id playbackID gives it,
// and defaultName (or the first, when empty) the default.
func (w *world) seedChord(slug, instrumentName string, kind domain.DiagramKind, owner, playbackNames, defaultName string) error {
	instrument, err := w.ensureInstrumentSeeded(instrumentName)
	if err != nil {
		return err
	}
	generatedPositions := a5Positions(slug)
	positions := make([]domain.Position, len(generatedPositions))
	for i, p := range generatedPositions {
		positions[i] = domain.Position{ID: p.PositionId.String(), Interval: string(p.Interval), NoteName: p.NoteName, String: p.String, Fret: p.Fret}
	}
	var playbacks []domain.DiagramPlayback
	var defaultID *string
	for _, name := range splitCommaList(playbackNames) {
		input := a5Playback(name, generated.LocalizedNames(bothLanguages(name)), generatedPositions)
		id := playbackID(diagramID(slug).String(), name).String()
		playbacks = append(playbacks, domain.DiagramPlayback{ID: id, Names: domain.LocalizedText(input.Names), TempoBPM: input.TempoBpm, Steps: domainSteps(input.Steps)})
		if name == defaultName {
			defaultID = &id
		}
	}
	skillID, conceptID := w.skillIDFor("seeded-skill"), w.conceptIDFor("seeded-concept")
	diagram, err := domain.NewDiagram(diagramID(slug).String(), owner, instrument, bothLanguages(slug), offeredLanguages, positions, []string{skillID.String()}, []string{conceptID.String()},
		domain.DiagramOptions{LabelDisplay: domain.LabelDisplayInterval, Kind: kind, Playbacks: playbacks, DefaultPlaybackID: defaultID}, fixedNow)
	if err != nil {
		return fmt.Errorf("seeding diagram %q: %w", slug, err)
	}
	w.diagrams.put(diagram)
	return nil
}

// domainSteps converts generated steps for a diagram seeded straight into
// the repository.
func domainSteps(steps []generated.SequenceStep) []domain.SequenceStep {
	out := make([]domain.SequenceStep, len(steps))
	for i, s := range steps {
		ids := make([]string, len(s.PositionIds))
		for j, id := range s.PositionIds {
			ids[j] = id.String()
		}
		out[i] = domain.SequenceStep{PositionIDs: ids, Value: domain.NoteValue{Num: s.Value.Num, Den: s.Value.Den}}
		if s.Strum != nil {
			out[i].Strum = domain.Strum(*s.Strum)
		}
	}
	return out
}

func (w *world) aCustomDiagramWithPlaybacks(slug, instrument, creator, playbacks string) error {
	return w.aCustomDiagramWithDefaultPlayback(slug, instrument, creator, playbacks, "")
}

func (w *world) aCustomDiagramWithDefaultPlayback(slug, instrument, creator, playbacks, defaultName string) error {
	owner := w.ensureRegistered(creator, domain.RoleTeacher).String()
	return w.seedChord(slug, instrument, domain.DiagramKindCustom, owner, playbacks, defaultName)
}

func (w *world) aBasicDiagramWithPlaybacks(slug, instrument, playbacks string) error {
	return w.seedChord(slug, instrument, domain.DiagramKindBasic, w.curatorID(), playbacks, "")
}

// aBasicDiagramWithOnePlayback seeds slug with one playback playing each
// position in turn at tempo.
func (w *world) aBasicDiagramWithOnePlayback(slug, instrument string, tempo int) error {
	if err := w.seedChord(slug, instrument, domain.DiagramKindBasic, w.curatorID(), "Arpeggio", ""); err != nil {
		return err
	}
	diagram, err := w.diagrams.GetByID(w.ctx(), diagramID(slug).String())
	if err != nil {
		return err
	}
	diagram.Playbacks[0].TempoBPM = tempo
	w.diagrams.put(diagram)
	return nil
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

// namesIn is name in every language of diagram.
func namesIn(diagram generated.Diagram, name string) generated.LocalizedNames {
	names := generated.LocalizedNames{}
	for _, language := range diagram.Languages {
		names[language] = name
	}
	return names
}

// resent is playback as an update request resends it, keeping its id.
func resent(playback generated.DiagramPlayback) generated.DiagramPlaybackInput {
	id, signature := playback.PlaybackId, playback.TimeSignature
	return generated.DiagramPlaybackInput{PlaybackId: &id, Names: playback.Names, TempoBpm: playback.TempoBpm, TimeSignature: &signature, Steps: playback.Steps}
}

func (w *world) updatesWithOnlyPlayback(_, slug, name string, tempo int, signature string, table *godog.Table) error {
	stored, err := w.storedDiagram(slug)
	if err != nil {
		return err
	}
	playback, err := playbackInput(name, namesIn(stored, name), tempo, signature, table, stored.Positions)
	if err != nil {
		return err
	}
	return w.updateDiagram(slug, generated.UpdateDiagramRequest{Playbacks: &[]generated.DiagramPlaybackInput{playback}})
}

func (w *world) resendsPlaybackWithItsID(_, slug, name string, tempo int, _ string) error {
	stored, err := w.storedDiagram(slug)
	if err != nil {
		return err
	}
	playback, err := playbackNamed(stored, name)
	if err != nil {
		return err
	}
	input := resent(playback)
	input.TempoBpm = tempo
	return w.updateDiagram(slug, generated.UpdateDiagramRequest{Playbacks: &[]generated.DiagramPlaybackInput{input}})
}

func (w *world) addsPlaybackFirst(_, slug, name, resentNames string) error {
	stored, err := w.storedDiagram(slug)
	if err != nil {
		return err
	}
	playbacks := []generated.DiagramPlaybackInput{a5Playback(name, namesIn(stored, name), stored.Positions)}
	for _, kept := range splitCommaList(resentNames) {
		playback, err := playbackNamed(stored, kept)
		if err != nil {
			return err
		}
		playbacks = append(playbacks, resent(playback))
	}
	return w.updateDiagram(slug, generated.UpdateDiagramRequest{Playbacks: &playbacks})
}

func (w *world) setsDefaultPlayback(_, slug, name string) error {
	stored, err := w.storedDiagram(slug)
	if err != nil {
		return err
	}
	playback, err := playbackNamed(stored, name)
	if err != nil {
		return err
	}
	return w.updateDiagram(slug, generated.UpdateDiagramRequest{DefaultPlaybackId: nullable.NewNullableWithValue(playback.PlaybackId)})
}

func (w *world) removesEveryPlayback(_, slug string) error {
	return w.updateDiagram(slug, generated.UpdateDiagramRequest{Playbacks: &[]generated.DiagramPlaybackInput{}})
}

// replacesDiagramPositions resends only two of slug's positions, keeping
// the playbacks as they are. It remembers the diagram as it was first, for
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

// playbackNamed is diagram's playback named name in English.
func playbackNamed(diagram generated.Diagram, name string) (generated.DiagramPlayback, error) {
	for _, playback := range diagram.Playbacks {
		if playback.Names["en"] == name {
			return playback, nil
		}
	}
	return generated.DiagramPlayback{}, fmt.Errorf("the diagram has no playback named %q; it has %v", name, playbackNames(diagram))
}

// playbackNames is the English name of each of diagram's playbacks, in order.
func playbackNames(diagram generated.Diagram) []string {
	names := make([]string, len(diagram.Playbacks))
	for i, playback := range diagram.Playbacks {
		names[i] = playback.Names["en"]
	}
	return names
}

// currentPlayback is the playback named name of the diagram the last step
// returned.
func (w *world) currentPlayback(name string) (generated.DiagramPlayback, generated.Diagram, error) {
	diagram, err := w.currentDiagram()
	if err != nil {
		return generated.DiagramPlayback{}, generated.Diagram{}, err
	}
	playback, err := playbackNamed(diagram, name)
	return playback, diagram, err
}

func (w *world) diagramHasNoPlaybacks() error {
	return w.diagramHasPlaybackCount(0)
}

func (w *world) diagramHasPlaybackCount(want int) error {
	diagram, err := w.currentDiagram()
	if err != nil {
		return err
	}
	if len(diagram.Playbacks) != want {
		return fmt.Errorf("expected %d playbacks, got %v", want, playbackNames(diagram))
	}
	return nil
}

func (w *world) diagramHasNoDefaultPlayback() error {
	diagram, err := w.currentDiagram()
	if err != nil {
		return err
	}
	if diagram.DefaultPlaybackId != nil {
		return fmt.Errorf("expected no default playback, got %s", diagram.DefaultPlaybackId)
	}
	return nil
}

func (w *world) diagramHasPlaybacksInOrder(count int, order string) error {
	diagram, err := w.currentDiagram()
	if err != nil {
		return err
	}
	if got, want := playbackNames(diagram), splitCommaList(order); len(got) != count || !slices.Equal(got, want) {
		return fmt.Errorf("expected %d playbacks %v, got %v", count, want, got)
	}
	return nil
}

func (w *world) diagramDefaultPlaybackIs(name string) error {
	playback, diagram, err := w.currentPlayback(name)
	if err != nil {
		return err
	}
	if diagram.DefaultPlaybackId == nil || *diagram.DefaultPlaybackId != playback.PlaybackId {
		return fmt.Errorf("expected %q (%s) to be the default playback, got %v", name, playback.PlaybackId, diagram.DefaultPlaybackId)
	}
	return nil
}

func (w *world) playbackTempoAndMeterAre(name string, tempo, beats, beatValue int) error {
	playback, _, err := w.currentPlayback(name)
	if err != nil {
		return err
	}
	if playback.TempoBpm != tempo || playback.TimeSignature.Beats != beats || int(playback.TimeSignature.BeatValue) != beatValue {
		return fmt.Errorf("expected %q at %d BPM in %d/%d, got %d BPM in %d/%d", name, tempo, beats, beatValue, playback.TempoBpm, playback.TimeSignature.Beats, playback.TimeSignature.BeatValue)
	}
	return nil
}

// describeSteps renders steps as "positions | value | strum" rows, with
// positions numbered (from 1) by their place in positions.
func describeSteps(steps []generated.SequenceStep, positions []generated.DiagramPosition) []string {
	rows := make([]string, len(steps))
	for i, step := range steps {
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

func (w *world) playbackStepsAre(name string, table *godog.Table) error {
	playback, diagram, err := w.currentPlayback(name)
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
	if got := describeSteps(playback.Steps, diagram.Positions); !slices.Equal(got, want) {
		return fmt.Errorf("expected steps\n%s\ngot\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
	}
	return nil
}

// playbackKeepsItsID checks name still has the id it was seeded with.
func (w *world) playbackKeepsItsID(name string) error {
	playback, diagram, err := w.currentPlayback(name)
	if err != nil {
		return err
	}
	if want := playbackID(diagram.DiagramId.String(), name); playback.PlaybackId != want {
		return fmt.Errorf("expected %q to keep id %s, got %s", name, want, playback.PlaybackId)
	}
	return nil
}

func (w *world) positionSoundsInSteps(name string, position, first, second int) error {
	playback, diagram, err := w.currentPlayback(name)
	if err != nil {
		return err
	}
	if position < 1 || position > len(diagram.Positions) || diagram.Positions[position-1].PositionId == nil {
		return fmt.Errorf("the diagram has no position %d", position)
	}
	id := *diagram.Positions[position-1].PositionId
	var steps []int
	for i, step := range playback.Steps {
		if slices.Contains(step.PositionIds, id) {
			steps = append(steps, i+1)
		}
	}
	if !slices.Equal(steps, []int{first, second}) {
		return fmt.Errorf("expected position %d to sound in steps %d and %d, got %v", position, first, second, steps)
	}
	return nil
}

func (w *world) stepIsRest(number int, name, value string) error {
	playback, diagram, err := w.currentPlayback(name)
	if err != nil {
		return err
	}
	if number < 1 || number > len(playback.Steps) {
		return fmt.Errorf("playback %q has no step %d", name, number)
	}
	step := playback.Steps[number-1]
	if len(step.PositionIds) != 0 || fmt.Sprintf("%d/%d", step.Value.Num, step.Value.Den) != value {
		return fmt.Errorf("expected step %d to be a rest of %s, got %v", number, value, describeSteps([]generated.SequenceStep{step}, diagram.Positions))
	}
	return nil
}

func (w *world) playbackStepValuesAre(name, list string) error {
	playback, _, err := w.currentPlayback(name)
	if err != nil {
		return err
	}
	got := make([]string, len(playback.Steps))
	for i, step := range playback.Steps {
		got[i] = fmt.Sprintf("%d/%d", step.Value.Num, step.Value.Den)
	}
	if want := splitCommaList(list); !slices.Equal(got, want) {
		return fmt.Errorf("expected step values %v, got %v", want, got)
	}
	return nil
}

func (w *world) playbackIsNamed(name1, lang1, name2, lang2 string) error {
	diagram, err := w.currentDiagram()
	if err != nil {
		return err
	}
	if len(diagram.Playbacks) != 1 {
		return fmt.Errorf("expected one playback, got %v", playbackNames(diagram))
	}
	want := generated.LocalizedNames{lang1: name1, lang2: name2}
	if got := diagram.Playbacks[0].Names; !maps.Equal(got, want) {
		return fmt.Errorf("expected the playback named %v, got %v", want, got)
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

var (
	reversedLoopingPattern = regexp.MustCompile(`^reversed and looping at (\d+) BPM with voice "([^"]+)"$`)
	voicePlaybackPattern   = regexp.MustCompile(`^with voice "([^"]+)"$`)
	tempoPlaybackPattern   = regexp.MustCompile(`^at (\d+) BPM$`)
	directionPattern       = regexp.MustCompile(`^in direction "([^"]+)"$`)
	chosenPlaybackPattern  = regexp.MustCompile(`^playback "([^"]+)"$`)
)

// diagramRefPlayback is the playback field of a generated.DiagramRef.
type diagramRefPlayback = struct {
	Direction  *generated.DiagramRefPlaybackDirection `json:"direction,omitempty"`
	Loop       *bool                                  `json:"loop,omitempty"`
	PlaybackId *openapi_types.UUID                    `json:"playback_id"`
	TempoBpm   *int                                   `json:"tempo_bpm"`
	VoiceId    *string                                `json:"voice_id"`
}

// playbackFrom reads how a step says diagramSlug plays: "with no
// overrides", "reversed and looping at 60 BPM with voice ...", one override
// alone, or a playback chosen by name.
func playbackFrom(diagramSlug, phrase string) (*diagramRefPlayback, error) {
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
	case chosenPlaybackPattern.MatchString(phrase):
		id := playbackID(diagramID(diagramSlug).String(), chosenPlaybackPattern.FindStringSubmatch(phrase)[1])
		playback.PlaybackId = &id
	default:
		return nil, fmt.Errorf("unknown playback %q", phrase)
	}
	return playback, nil
}

func (w *world) addsPlayingDiagram(_, diagramSlug, nodeSlug string, trigger, hide int, phrase string) error {
	playback, err := playbackFrom(diagramSlug, phrase)
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

// playingItemSlug is the expanded content item a "has diagram ... as
// expanded content" step seeds on nodeSlug.
func playingItemSlug(nodeSlug string) string {
	return nodeSlug + "-playing-diagram"
}

func (w *world) nodeHasDiagramPlaying(nodeSlug, diagramSlug, playbackName string) error {
	if err := w.putContentNode(nodeSlug, domain.ContentTypeVideo); err != nil {
		return err
	}
	chosen := playbackID(diagramID(diagramSlug).String(), playbackName).String()
	trigger, hide := 150, 165
	w.expanded.put(domain.ExpandedContent{
		ID:            expandedID(playingItemSlug(nodeSlug)).String(),
		ContentNodeID: nodeID(nodeSlug).String(),
		ContentType:   domain.ExpandedContentTypeDiagram,
		DiagramRef: &domain.DiagramRef{
			DiagramID: diagramID(diagramSlug).String(),
			Layers:    domain.DiagramLayers{Intervals: true},
			Playback:  &domain.DiagramRefPlayback{Direction: domain.DiagramPlaybackDirectionAsAuthored, PlaybackID: &chosen},
		},
		TriggerAtSeconds: &trigger, HideAtSeconds: &hide,
		CreatedAt: fixedNow,
	})
	w.playingNode = nodeSlug
	return nil
}

// playbackIsRemoved has the curator, who owns the seeded basic diagrams,
// resend every playback of diagramSlug but name, through the API.
func (w *world) playbackIsRemoved(name, diagramSlug string) error {
	w.curatorID()
	curator := w.identityCtx("admin")
	resp, err := w.handler.GetDiagram(curator, generated.GetDiagramRequestObject{DiagramId: diagramID(diagramSlug)})
	if err != nil {
		return err
	}
	stored, ok := resp.(generated.GetDiagram200JSONResponse)
	if !ok {
		return fmt.Errorf("expected to read diagram %q, got %#v", diagramSlug, resp)
	}
	kept := []generated.DiagramPlaybackInput{}
	for _, playback := range stored.Playbacks {
		if playback.Names["en"] != name {
			kept = append(kept, resent(playback))
		}
	}
	updated, err := w.handler.UpdateDiagram(curator, generated.UpdateDiagramRequestObject{DiagramId: diagramID(diagramSlug), Body: &generated.UpdateDiagramRequest{Playbacks: &kept}})
	if err != nil {
		return err
	}
	if _, ok := updated.(generated.UpdateDiagram200JSONResponse); !ok {
		return fmt.Errorf("expected removing playback %q to succeed, got %#v", name, updated)
	}
	return nil
}

func (w *world) itemStillNamesPlayback(name string) error {
	item, err := w.expanded.GetByID(context.Background(), expandedID(playingItemSlug(w.playingNode)).String())
	if err != nil {
		return err
	}
	if item.DiagramRef == nil || item.DiagramRef.Playback == nil || item.DiagramRef.Playback.PlaybackID == nil {
		return fmt.Errorf("expected the item's diagram to name a playback, got %+v", item.DiagramRef)
	}
	if want := playbackID(item.DiagramRef.DiagramID, name).String(); *item.DiagramRef.Playback.PlaybackID != want {
		return fmt.Errorf("expected the item's diagram to name playback %q (%s), got %s", name, want, *item.DiagramRef.Playback.PlaybackID)
	}
	return nil
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
	if playback.PlaybackId != nil || playback.Direction == nil || *playback.Direction != "as_authored" || (playback.Loop != nil && *playback.Loop) ||
		playback.TempoBpm != nil || playback.VoiceId != nil {
		return fmt.Errorf("expected the default playback, as authored, not looping, no tempo or voice; got %s", describePlayback(playback))
	}
	return nil
}

func (w *world) itemPlaysPlayback(name string) error {
	playback, err := w.createdItemPlayback()
	if err != nil {
		return err
	}
	resp := w.lastResp.(generated.CreateExpandedContent201JSONResponse)
	if want := playbackID(resp.DiagramRef.DiagramId.String(), name); playback.PlaybackId == nil || *playback.PlaybackId != want {
		return fmt.Errorf("expected the item's diagram to play %q (%s); got %s", name, want, describePlayback(playback))
	}
	return nil
}

// describePlayback renders the fields p sets, for a failure message.
func describePlayback(p *diagramRefPlayback) string {
	var fields []string
	if p.PlaybackId != nil {
		fields = append(fields, "playback_id="+p.PlaybackId.String())
	}
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
