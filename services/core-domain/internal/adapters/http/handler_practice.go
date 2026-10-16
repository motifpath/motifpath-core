package http

import (
	"context"
	"strings"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// anyInstrumentGroupNames names the group of practice nodes that suit any
// instrument, in each language MotifPath offers.
var anyInstrumentGroupNames = generated.LocalizedNames{"en": "Any instrument", "pt_BR": "Qualquer instrumento"}

func (h *Handler) GetPracticeSummary(ctx context.Context, request generated.GetPracticeSummaryRequestObject) (generated.GetPracticeSummaryResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.GetPracticeSummary401JSONResponse(unauthorizedError()), nil
	}

	summary, err := h.practiceSummary.Summary(ctx, caller, uuidPtrToStringPtr(request.Params.InstrumentId), deref(request.Params.TimeZone))
	switch kind, valErr := classify(err); {
	case err == nil:
		return generated.GetPracticeSummary200JSONResponse(toGeneratedPracticeSummary(summary)), nil
	case kind == errKindValidation:
		return generated.GetPracticeSummary400JSONResponse(validationErrorResponse(valErr)), nil
	case kind == errKindNotFound:
		return generated.GetPracticeSummary404JSONResponse(notFoundError(notFoundMessage(err))), nil
	default:
		return nil, err
	}
}

func (h *Handler) GetPracticeOverview(ctx context.Context, request generated.GetPracticeOverviewRequestObject) (generated.GetPracticeOverviewResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.GetPracticeOverview401JSONResponse(unauthorizedError()), nil
	}

	overview, err := h.practiceSummary.Overview(ctx, caller, deref(request.Params.TimeZone))
	switch kind, valErr := classify(err); {
	case err == nil:
		return generated.GetPracticeOverview200JSONResponse(toGeneratedPracticeOverview(overview)), nil
	case kind == errKindValidation:
		return generated.GetPracticeOverview400JSONResponse(validationErrorResponse(valErr)), nil
	default:
		return nil, err
	}
}

func (h *Handler) GetFretboardMap(ctx context.Context, request generated.GetFretboardMapRequestObject) (generated.GetFretboardMapResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.GetFretboardMap401JSONResponse(unauthorizedError()), nil
	}

	fretboard, err := h.practiceSummary.FretboardMap(ctx, caller, request.Params.InstrumentId.String())
	switch kind, valErr := classify(err); {
	case err == nil:
		return generated.GetFretboardMap200JSONResponse(toGeneratedFretboardMap(fretboard)), nil
	case kind == errKindValidation:
		return generated.GetFretboardMap400JSONResponse(validationErrorResponse(valErr)), nil
	case kind == errKindNotFound:
		return generated.GetFretboardMap404JSONResponse(notFoundError(notFoundMessage(err))), nil
	default:
		return nil, err
	}
}

func toGeneratedFretboardMap(m domain.FretboardMap) generated.FretboardMap {
	out := generated.FretboardMap{
		InstrumentId:       mustUUID(m.InstrumentID),
		LayoutInstrumentId: uuidPtrFromStringPtr(m.LayoutInstrumentID),
		Cells:              make([]generated.FretboardMapCell, len(m.Cells)),
	}
	for i, c := range m.Cells {
		out.Cells[i] = generated.FretboardMapCell{
			ItemKey: c.ItemKey,
			String:  c.String,
			Fret:    c.Fret,
			Level:   generated.KnowledgeLevel(c.Level),
			Fading:  c.Fading,
		}
	}
	return out
}

func toGeneratedPracticeSummary(summary application.PracticeSummary) generated.PracticeSummary {
	out := generated.PracticeSummary{
		InstrumentId:         uuidPtrFromStringPtr(summary.InstrumentID),
		StudentInstrumentIds: make([]openapi_types.UUID, len(summary.StudentInstrumentIDs)),
		PracticeDaysLast7:    summary.PracticeDaysLast7,
		Last7Days:            make([]generated.PracticeDay, len(summary.Last7Days)),
		ProgressThisWeek:     make([]generated.SkillProgress, len(summary.Progress)),
		NextSteps:            make([]generated.PracticeNextStep, len(summary.NextSteps)),
		NextStepsTotal:       summary.NextStepsTotal,
		Groups:               make([]generated.PracticeNodeGroup, len(summary.Groups)),
	}
	for i, id := range summary.StudentInstrumentIDs {
		out.StudentInstrumentIds[i] = mustUUID(id)
	}
	for i, day := range summary.Last7Days {
		out.Last7Days[i] = generated.PracticeDay{Date: openapi_types.Date{Time: day.Date}, Practised: day.Marked}
	}
	for i, line := range summary.Progress {
		out.ProgressThisWeek[i] = generated.SkillProgress{
			NodeId:  mustUUID(line.NodeID),
			Names:   generated.LocalizedNames(summary.Nodes[line.NodeID].Names),
			Measure: generated.SkillProgressMeasure(line.Measure),
			Before:  float32(line.Before),
			After:   float32(line.After),
		}
	}
	for i, step := range summary.NextSteps {
		out.NextSteps[i] = toGeneratedPracticeNextStep(step)
	}
	for i, group := range summary.Groups {
		g := generated.PracticeNodeGroup{AnyInstrument: group.AnyInstrument, Names: anyInstrumentGroupNames, Nodes: make([]generated.PracticeNodeProgress, len(group.Nodes))}
		if group.Area != nil {
			id := mustUUID(group.Area.ID)
			g.AreaNodeId, g.Names = &id, generated.LocalizedNames(group.Area.Names)
		}
		for j, n := range group.Nodes {
			g.Nodes[j] = toGeneratedPracticeNodeProgress(n, summary.Standings[n.ID], summary.Children[n.ID])
		}
		out.Groups[i] = g
	}
	return out
}

// toGeneratedPracticeNodeProgress maps one node's standing. A wide node,
// one with children to show it by, has no level of its own.
func toGeneratedPracticeNodeProgress(node domain.KnowledgeNode, standing domain.NodeStanding, childIDs []string) generated.PracticeNodeProgress {
	out := generated.PracticeNodeProgress{
		NodeId:       mustUUID(node.ID),
		Names:        generated.LocalizedNames(node.Names),
		Fading:       standing.Fading,
		ChildNodeIds: make([]openapi_types.UUID, len(childIDs)),
	}
	if len(childIDs) == 0 && standing.Level != nil {
		level := generated.KnowledgeLevel(*standing.Level)
		out.Level = &level
	}
	out.Coverage.MetCount, out.Coverage.ItemCount = standing.Covered, standing.Total
	out.Readiness.MetCount, out.Readiness.RequiredCount = standing.Readiness.Met, standing.Readiness.Total
	for i, id := range childIDs {
		out.ChildNodeIds[i] = mustUUID(id)
	}
	return out
}

func toGeneratedPracticeNextStep(step domain.PracticeNextStep) generated.PracticeNextStep {
	out := generated.PracticeNextStep{
		Kind:   generated.PracticeNextStepKind(step.Kind),
		NodeId: mustUUID(step.Node.ID),
		Names:  generated.LocalizedNames(step.Node.Names),
	}
	if step.Level != nil {
		level := generated.KnowledgeLevel(*step.Level)
		out.Level = &level
	}
	return out
}

func toGeneratedPracticeOverview(overview application.PracticeOverview) generated.PracticeOverview {
	out := generated.PracticeOverview{
		PracticeDaysLast7:         overview.PracticeDaysLast7,
		LearningDaysLast7:         overview.LearningDaysLast7,
		MinutesPractisedLast7:     overview.MinutesPractisedLast7,
		MinutesPractisedPrevious7: overview.MinutesPractisedPrevious7,
		DayStreakCurrent:          overview.DayStreakCurrent,
		DayStreakBest:             overview.DayStreakBest,
		SkillsUpLast7:             overview.SkillsUpLast7,
		SongsPlayedTotal:          overview.SongsPlayedTotal,
		SongsPlayedLast7:          overview.SongsPlayedLast7,
		Last7Days:                 make([]generated.ActivityDay, len(overview.Last7Days)),
		Instruments:               make([]generated.PracticeInstrumentCard, len(overview.Instruments)),
	}
	for i, day := range overview.Last7Days {
		out.Last7Days[i] = generated.ActivityDay{Date: openapi_types.Date{Time: day.Date}, Practised: day.Practised, Learned: day.Learned}
	}
	for i, card := range overview.Instruments {
		out.Instruments[i] = generated.PracticeInstrumentCard{InstrumentId: mustUUID(card.InstrumentID), PracticeDaysLast7: card.PracticeDaysLast7}
		if card.TopNextStep != nil {
			step := toGeneratedPracticeNextStep(*card.TopNextStep)
			out.Instruments[i].TopNextStep = &step
		}
	}
	return out
}

func (h *Handler) CreatePracticeSessionPlan(ctx context.Context, request generated.CreatePracticeSessionPlanRequestObject) (generated.CreatePracticeSessionPlanResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.CreatePracticeSessionPlan401JSONResponse(unauthorizedError()), nil
	}
	if request.Body == nil {
		return generated.CreatePracticeSessionPlan400JSONResponse(validationErrorResponse(domain.NewValidationError("minutes", "is required"))), nil
	}

	plan, err := h.practiceSession.ComposePlan(ctx, caller, uuidPtrToStringPtr(request.Body.InstrumentId), uuidPtrToStringPtr(request.Body.FocusNodeId), request.Body.Minutes)
	switch kind, valErr := classify(err); {
	case err == nil:
		var creatorIDs []string
		for _, item := range plan.Items {
			if item.Exercise != nil && item.Exercise.CreatedBy != "" {
				creatorIDs = append(creatorIDs, item.Exercise.CreatedBy)
			}
		}
		names, err := h.loadUserNames(ctx, creatorIDs)
		if err != nil {
			return nil, err
		}
		return generated.CreatePracticeSessionPlan200JSONResponse(toGeneratedPracticeSessionPlan(plan, names, caller.Locale.Code)), nil
	case kind == errKindValidation:
		return generated.CreatePracticeSessionPlan400JSONResponse(validationErrorResponse(valErr)), nil
	case kind == errKindNotFound:
		return generated.CreatePracticeSessionPlan404JSONResponse(notFoundError(notFoundMessage(err))), nil
	default:
		return nil, err
	}
}

func (h *Handler) ListPracticeSubjects(ctx context.Context, request generated.ListPracticeSubjectsRequestObject) (generated.ListPracticeSubjectsResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.ListPracticeSubjects401JSONResponse(unauthorizedError()), nil
	}
	list, err := h.practiceSession.ListSubjects(ctx, caller, uuidPtrToStringPtr(request.Params.InstrumentId))
	switch kind, _ := classify(err); {
	case err == nil:
		return generated.ListPracticeSubjects200JSONResponse(toGeneratedPracticeSubjectList(list)), nil
	case kind == errKindNotFound:
		return generated.ListPracticeSubjects404JSONResponse(notFoundError(notFoundMessage(err))), nil
	default:
		return nil, err
	}
}

// toGeneratedPracticeSubjectList maps list; a wide node's level is null.
func toGeneratedPracticeSubjectList(list domain.PracticeSubjectList) generated.PracticeSubjectList {
	groups := make([]generated.PracticeSubjectGroup, len(list.Groups))
	for i, g := range list.Groups {
		subjects := make([]generated.PracticeSubject, len(g.Subjects))
		for j, s := range g.Subjects {
			subjects[j] = generated.PracticeSubject{
				NodeId: mustUUID(s.Node.ID), Names: generated.LocalizedNames(s.Node.Names), OnPath: s.OnPath,
				ItemCount: s.ItemCount, CurrentRung: generated.PracticeRung(s.CurrentRung),
			}
			if s.Level != nil {
				level := generated.KnowledgeLevel(*s.Level)
				subjects[j].Level = &level
			}
		}
		groups[i] = generated.PracticeSubjectGroup{AreaNodeId: mustUUID(g.Area.ID), Names: generated.LocalizedNames(g.Area.Names), Subjects: subjects}
	}
	out := generated.PracticeSubjectList{Groups: groups}
	if list.InstrumentID != nil {
		id := mustUUID(*list.InstrumentID)
		out.InstrumentId = &id
	}
	return out
}

// notFoundMessage is a domain.ErrNotFound's own explanation, without the
// sentinel's prefix.
func notFoundMessage(err error) string {
	return strings.TrimPrefix(err.Error(), domain.ErrNotFound.Error()+": ")
}

// toGeneratedPracticeSessionPlan maps plan, naming each diagram shape option
// in locale, the student's language.
func toGeneratedPracticeSessionPlan(plan domain.PracticeSessionPlan, names userNames, locale string) generated.PracticeSessionPlan {
	items := make([]generated.PracticeSessionItem, len(plan.Items))
	for i, item := range plan.Items {
		items[i] = toGeneratedPracticeSessionItem(item, names, locale)
	}
	focus := make([]generated.PracticeFocusNode, len(plan.FocusNodes))
	for i, n := range plan.FocusNodes {
		focus[i] = generated.PracticeFocusNode{NodeId: mustUUID(n.ID), Names: generated.LocalizedNames(n.Names)}
	}
	return generated.PracticeSessionPlan{
		PracticeSessionId: mustUUID(plan.ID),
		InstrumentId:      uuidPtrFromStringPtr(plan.InstrumentID),
		Minutes:           plan.Minutes,
		FocusSource:       generated.PracticeFocusSource(plan.FocusSource),
		FocusNodes:        focus,
		Items:             items,
		FeltQuestions:     plan.FeltQuestions,
		TapCheckDue:       plan.TapCheckDue,
	}
}

func toGeneratedPracticeSessionItem(item domain.PracticeSessionItem, names userNames, locale string) generated.PracticeSessionItem {
	out := generated.PracticeSessionItem{
		ItemKey:          item.ItemKey,
		Kind:             generated.PracticeItemKind(item.Kind),
		Reason:           generated.PracticePickReason(item.Reason),
		Rung:             generated.PracticeRung(item.Rung),
		NodeId:           uuidPtrFromStringPtr(item.NodeID),
		Level:            generated.KnowledgeLevel(item.Level),
		EstimatedSeconds: item.EstimatedSeconds,
	}
	if p := item.PlayAlong; p != nil {
		var bestCleanNoteValue *generated.NoteValue2
		if v := p.BestCleanNoteValue; v != nil {
			noteValue := generated.NoteValue2(*v)
			bestCleanNoteValue = &noteValue
		}
		out.PlayAlong = &struct {
			BestCleanNoteValue *generated.NoteValue2 `json:"best_clean_note_value"`
			BestCleanTempoBpm  *int                  `json:"best_clean_tempo_bpm"`
			DiagramId          openapi_types.UUID    `json:"diagram_id"`
			StartNoteValue     generated.NoteValue2  `json:"start_note_value"`
			StartTempoBpm      int                   `json:"start_tempo_bpm"`
			TargetTempoBpm     int                   `json:"target_tempo_bpm"`
		}{
			BestCleanNoteValue: bestCleanNoteValue,
			BestCleanTempoBpm:  p.BestCleanTempoBPM,
			DiagramId:          mustUUID(p.DiagramID),
			StartNoteValue:     generated.NoteValue2(p.StartNoteValue),
			StartTempoBpm:      p.StartTempoBPM,
			TargetTempoBpm:     p.TargetTempoBPM,
		}
	}
	if e := item.Exercise; e != nil {
		exercise := toExercise(*e, names)
		out.Exercise = &exercise
	}
	if c := item.FretboardCell; c != nil {
		out.FretboardCell = &struct {
			Drill              generated.FretboardDrill `json:"drill"`
			Fret               int                      `json:"fret"`
			FretWindow         generated.FretWindow     `json:"fret_window"`
			LayoutInstrumentId openapi_types.UUID       `json:"layout_instrument_id"`
			String             int                      `json:"string"`
		}{
			Drill:              generated.FretboardDrill(c.Drill),
			Fret:               c.Fret,
			FretWindow:         generated.FretWindow{FromFret: generated.FretWindowFromFret(c.Window.FromFret)},
			LayoutInstrumentId: mustUUID(c.LayoutInstrumentID),
			String:             c.String,
		}
	}
	if d := item.DiagramShape; d != nil {
		out.DiagramShape = toGeneratedDiagramShape(*d, locale)
	}
	if q := item.GeneratedChoice; q != nil {
		out.GeneratedChoice = toGeneratedChoiceQuestion(*q, locale)
	}
	return out
}

// toGeneratedChoiceQuestion maps a planned harmony or ear question, its names in
// locale. What the way doesn't give is null, and every list is a list,
// empty when unused.
func toGeneratedChoiceQuestion(q domain.PlannedGeneratedChoice, locale string) *generated.GeneratedChoiceQuestion {
	out := &generated.GeneratedChoiceQuestion{
		Way:      generated.GeneratedChoiceWay(q.Way()),
		MusicKey: generated.MusicKey(q.MusicKey),
		Chart:    nonNilStrings(q.Chart),
		DrawnAs:  generated.GeneratedChordDrawing(q.DrawnAs),
	}
	if q.Numeral != "" {
		numeral := generated.ChordNumeral(q.Numeral)
		out.Numeral = &numeral
	}
	if q.Chord != "" {
		chord := q.Chord
		out.Chord = &chord
	}
	if q.Function != "" {
		function := generated.ChordFunction(q.Function)
		out.Function = &function
	}
	if p := q.Progression; p != nil {
		out.Progression = &struct {
			Key  string `json:"key"`
			Name string `json:"name"`
		}{Key: p.Key, Name: p.Names.Resolve(locale)}
	}
	out.Options = make([]struct {
		Chart []string `json:"chart"`
		Name  *string  `json:"name"`
		Value string   `json:"value"`
	}, len(q.Options))
	for i, o := range q.Options {
		out.Options[i].Value = o.Value
		out.Options[i].Chart = nonNilStrings(o.Chart)
		if o.Names != nil {
			name := o.Names.Resolve(locale)
			out.Options[i].Name = &name
		}
	}
	out.Voicings = make([]struct {
		Chord     string             `json:"chord"`
		DiagramId openapi_types.UUID `json:"diagram_id"`
	}, len(q.Voicings))
	for i, box := range q.Voicings {
		out.Voicings[i].Chord = box.Chord
		out.Voicings[i].DiagramId = mustUUID(box.DiagramID)
	}
	if q.Playback != nil {
		out.Playback = toGeneratedChoicePlayback(*q.Playback)
	}
	return out
}

// toGeneratedChoicePlayback maps what a question asked by ear plays; a bar
// with no numeral shown has a null numeral.
func toGeneratedChoicePlayback(p domain.GeneratedPlayback) *generated.GeneratedChoicePlayback {
	out := &generated.GeneratedChoicePlayback{TempoBpm: p.TempoBPM, BassLine: p.BassLine}
	out.Bars = make([]struct {
		Chord   string                  `json:"chord"`
		Numeral *generated.ChordNumeral `json:"numeral"`
	}, len(p.Bars))
	for i, bar := range p.Bars {
		out.Bars[i].Chord = bar.Chord
		if bar.Numeral != "" {
			numeral := generated.ChordNumeral(bar.Numeral)
			out.Bars[i].Numeral = &numeral
		}
	}
	return out
}

// nonNilStrings is values, or an empty list in place of nil.
func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// generatedDiagramShape is the generated PracticeSessionItem's diagram_shape.
type generatedDiagramShape = struct {
	AskedInterval      *generated.ShapeInterval    `json:"asked_interval"`
	DiagramId          openapi_types.UUID          `json:"diagram_id"`
	Drill              generated.DiagramShapeDrill `json:"drill"`
	LayoutInstrumentId openapi_types.UUID          `json:"layout_instrument_id"`
	Options            []struct {
		Name  string `json:"name"`
		Shape string `json:"shape"`
	} `json:"options"`
	Shape       string `json:"shape"`
	ShapeFamily string `json:"shape_family"`
}

// toGeneratedDiagramShape maps a planned shape, its options named in locale
// and always a list, empty for a degree to find.
func toGeneratedDiagramShape(d domain.PlannedDiagramShape, locale string) *generatedDiagramShape {
	out := &generatedDiagramShape{
		DiagramId:          mustUUID(d.DiagramID),
		Drill:              generated.DiagramShapeDrill(d.Drill),
		LayoutInstrumentId: mustUUID(d.LayoutInstrumentID),
		Shape:              d.Shape,
		ShapeFamily:        d.Family,
		Options: make([]struct {
			Name  string `json:"name"`
			Shape string `json:"shape"`
		}, len(d.Options)),
	}
	for i, m := range d.Options {
		out.Options[i].Name = m.Names.Resolve(locale)
		out.Options[i].Shape = m.Shape
	}
	if d.AskedInterval != nil {
		asked := generated.ShapeInterval(*d.AskedInterval)
		out.AskedInterval = &asked
	}
	return out
}

// ListProgressions lists the progression catalog to any signed-in user.
func (h *Handler) ListProgressions(ctx context.Context, _ generated.ListProgressionsRequestObject) (generated.ListProgressionsResponseObject, error) {
	if _, ok := h.resolveCaller(ctx); !ok {
		return generated.ListProgressions401JSONResponse(unauthorizedError()), nil
	}
	progressions, err := h.progressionCatalog.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make(generated.ListProgressions200JSONResponse, len(progressions))
	for i, p := range progressions {
		out[i] = toGeneratedProgression(p)
	}
	return out, nil
}

// toGeneratedProgression maps a catalog progression; its lists are never
// null.
func toGeneratedProgression(p domain.Progression) generated.Progression {
	return generated.Progression{
		ProgressionId: mustUUID(p.ID),
		Key:           p.Key,
		Bars:          nonNilStrings(p.Bars),
		Confusable:    nonNilStrings(p.Confusable),
		Names:         generated.LocalizedNames(p.Names),
	}
}
