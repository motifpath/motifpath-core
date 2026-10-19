//go:build integration

package bdd

import (
	"context"
	"fmt"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// registerClassificationInstrumentSteps covers the rules that tie a
// classification to its item: each node must be of the field's kind, and a
// content node's or diagram's nodes must suit its instruments. Exercises
// carry instruments too.
func registerClassificationInstrumentSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)" submits a create content node request with (skill|concept) "([^"]+)" in its (skill|concept) ids$`, w.submitsContentNodeWithNodeInField)
	sc.Step(`^"([^"]+)" submits a create exercise request with (skill|concept) "([^"]+)" in its (skill|concept) ids$`, w.submitsExerciseWithNodeInField)
	sc.Step(`^"([^"]+)" creates an article content node titled "([^"]+)" for instruments ((?:"[^"]+"(?:, )?)+) classified under skill "([^"]+)"$`, func(_, title, instruments, skill string) error {
		return w.createArticleClassified(title, instrumentIDsNamed(quotedValues(instruments)), skill)
	})
	sc.Step(`^"([^"]+)" creates an article content node titled "([^"]+)" for every instrument classified under skill "([^"]+)"$`, func(_, title, skill string) error {
		return w.createArticleClassified(title, []uuid.UUID{}, skill)
	})
	sc.Step(`^a content node "([^"]+)" for instruments ((?:"[^"]+"(?:, )?)+) classified under skill "([^"]+)" exists in the system$`, w.putContentNodeClassified)
	sc.Step(`^"([^"]+)" updates content node "([^"]+)" to be for instruments ((?:"[^"]+"(?:, )?)+)$`, w.updatesContentNodeInstruments)
	sc.Step(`^"([^"]+)" submits a create diagram request on instrument "([^"]+)" classified under skill "([^"]+)"$`, w.submitsDiagramClassifiedUnder)

	sc.Step(`^"([^"]+)" creates an exercise titled "([^"]+)" for instruments ((?:"[^"]+"(?:, )?)+)$`, func(_, title, instruments string) error {
		ids := generated.InstrumentIds(instrumentIDsNamed(quotedValues(instruments)))
		return w.createExerciseFor(title, &ids)
	})
	sc.Step(`^"([^"]+)" creates an exercise titled "([^"]+)" for every instrument$`, func(_, title string) error {
		return w.createExerciseFor(title, nil)
	})
	sc.Step(`^an exercise "([^"]+)" for instruments ((?:"[^"]+"(?:, )?)+) exists in the system$`, w.putExerciseFor)
	sc.Step(`^"([^"]+)" updates exercise "([^"]+)" with title "([^"]+)" and the instrument_ids field omitted$`, w.updatesExerciseKeepingInstruments)
	sc.Step(`^the exercise is for instruments ((?:"[^"]+"(?:, )?)+)$`, func(instruments string) error {
		return w.lastExerciseIsFor(instrumentIDsNamed(quotedValues(instruments)))
	})
	sc.Step(`^the exercise is for every instrument$`, func() error { return w.lastExerciseIsFor(nil) })
}

// idsInFields puts the node a scenario names into the skill or concept ids
// field, and a matching default into the other.
func (w *world) idsInFields(kind, name, field string) (skillIDs, conceptIDs []uuid.UUID) {
	id := w.nodeIDFor(kind, name)
	if field == string(domain.KnowledgeNodeKindSkill) {
		return []uuid.UUID{id}, w.conceptIDsFor("c")
	}
	return w.skillIDsFor("s"), []uuid.UUID{id}
}

func (w *world) submitsContentNodeWithNodeInField(_, kind, name, field string) error {
	skillIDs, conceptIDs := w.idsInFields(kind, name, field)
	resp, err := w.handler.CreateContentNode(w.ctx(), generated.CreateContentNodeRequestObject{
		Body: &generated.CreateContentNodeRequest{
			Title: "Title", ContentType: generated.CreateContentNodeRequestContentTypeVideo, MediaUrl: defaultMediaURL(),
			Classification: generated.ClassificationInput{SkillIds: skillIDs, ConceptIds: conceptIDs, DifficultyLevel: generated.ClassificationInputDifficultyLevelBeginner},
			LanguageCodes:  []string{"en"},
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsExerciseWithNodeInField(_, kind, name, field string) error {
	skillIDs, conceptIDs := w.idsInFields(kind, name, field)
	body := w.exerciseBody("title")
	body.SkillIds, body.ConceptIds = skillIDs, conceptIDs
	resp, err := w.handler.CreateExercise(w.ctx(), generated.CreateExerciseRequestObject{Body: &body})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) createArticleClassified(title string, instruments []uuid.UUID, skill string) error {
	doc := promptDocFor("An article.")
	resp, err := w.handler.CreateContentNode(w.ctx(), generated.CreateContentNodeRequestObject{
		Body: &generated.CreateContentNodeRequest{
			Title: title, ContentType: generated.CreateContentNodeRequestContentTypeArticle,
			Classification: w.classificationInputFor(skill, "c", "beginner"), RichContent: &doc,
			LanguageCodes: []string{"en"}, InstrumentIds: instruments,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) putContentNodeClassified(slug, instruments, skill string) error {
	if err := w.putContentNode(slug, domain.ContentTypeArticle); err != nil {
		return err
	}
	node, err := w.nodes.GetByID(context.Background(), nodeID(slug).String())
	if err != nil {
		return err
	}
	node.Classification.Skills = []domain.KnowledgeNode{{ID: w.skillIDFor(skill).String()}}
	node.Classification.Concepts = []domain.KnowledgeNode{{ID: w.conceptIDFor("c").String()}}
	node.InstrumentIDs = instrumentIDStrings(quotedValues(instruments))
	w.nodes.put(node)
	return nil
}

func (w *world) updatesContentNodeInstruments(_, slug, instruments string) error {
	existing, err := w.nodes.GetByID(w.ctx(), nodeID(slug).String())
	if err != nil {
		return err
	}
	mediaURL, richContent := bodyFor(existing)
	resp, err := w.handler.UpdateContentNode(w.ctx(), generated.UpdateContentNodeRequestObject{
		ContentNodeId: nodeID(slug),
		Body: &generated.UpdateContentNodeRequest{
			Title: existing.Title, MediaUrl: mediaURL, RichContent: richContent,
			Classification: generated.ClassificationInput{
				SkillIds:        idsOfSkills(existing.Classification.Skills),
				ConceptIds:      idsOfConcepts(existing.Classification.Concepts),
				DifficultyLevel: generated.ClassificationInputDifficultyLevel(existing.Classification.DifficultyLevel),
			},
			LanguageCodes: []string{"en"},
			InstrumentIds: instrumentIDsNamed(quotedValues(instruments)),
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsDiagramClassifiedUnder(_, instrument, skill string) error {
	return w.createDiagram(generated.CreateDiagramRequest{
		InstrumentIds: []openapi_types.UUID{instrumentID(instrument)}, Names: english("Palm-muted riff"), Positions: onePositionOnGuitar(),
		Classification: w.diagramClassification(skill, "scale-construction"),
	})
}

// exerciseBody is a valid text-response exercise titled title, classified
// under the default skill and concept.
func (w *world) exerciseBody(title string) generated.CreateExerciseRequest {
	label := "correct option"
	return generated.CreateExerciseRequest{
		SkillIds: w.skillIDsFor("skill-1"), ConceptIds: w.conceptIDsFor("concept-1"),
		Title: title, Prompt: promptDocFor("prompt"), ExerciseType: generated.CreateExerciseRequestExerciseTypeTextResponse,
		Options:       opts([]generated.Option{{OptionId: uuid.New(), IsCorrect: true, Label: &label}}),
		LanguageCodes: []string{"en"},
	}
}

func (w *world) createExerciseFor(title string, instruments *generated.InstrumentIds) error {
	body := w.exerciseBody(title)
	body.InstrumentIds = instruments
	resp, err := w.handler.CreateExercise(w.ctx(), generated.CreateExerciseRequestObject{Body: &body})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) putExerciseFor(slug, instruments string) error {
	label := "option-" + slug
	w.exercises.put(domain.Exercise{
		ID: exerciseID(slug).String(), Title: slug, Prompt: domain.NewPlainTextPrompt("prompt-" + slug), ExerciseType: domain.ExerciseTypeTextResponse,
		Skills:        []domain.KnowledgeNode{{ID: w.skillIDFor("skill-1").String()}},
		Concepts:      []domain.KnowledgeNode{{ID: w.conceptIDFor("concept-1").String()}},
		Options:       []domain.Option{{ID: uuid.NewString(), IsCorrect: true, Label: &label}},
		Languages:     []domain.Language{{Code: "en"}},
		InstrumentIDs: instrumentIDStrings(quotedValues(instruments)),
		ChallengeIDs:  []string{}, CreatedBy: w.ensureRegistered("bob", domain.RoleTeacher).String(), CreatedAt: fixedNow,
	})
	return nil
}

func (w *world) updatesExerciseKeepingInstruments(_, slug, title string) error {
	label := "correct option"
	resp, err := w.handler.UpdateExercise(w.ctx(), generated.UpdateExerciseRequestObject{
		ExerciseId: exerciseID(slug),
		Body: &generated.UpdateExerciseRequest{
			SkillIds: w.skillIDsFor("skill-1"), ConceptIds: w.conceptIDsFor("concept-1"),
			Title: title, Prompt: promptDocFor("prompt"),
			Options:       opts([]generated.Option{{OptionId: uuid.New(), IsCorrect: true, Label: &label}}),
			LanguageCodes: []string{"en"},
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

// lastExerciseIsFor checks the instruments of the exercise the last step
// created or updated; want nil means every instrument.
func (w *world) lastExerciseIsFor(want []uuid.UUID) error {
	var got generated.InstrumentIds
	switch resp := w.lastResp.(type) {
	case generated.CreateExercise201JSONResponse:
		got = resp.InstrumentIds
	case generated.UpdateExercise200JSONResponse:
		got = resp.InstrumentIds
	default:
		return fmt.Errorf("expected a created or updated exercise, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if fmt.Sprint([]uuid.UUID(got)) != fmt.Sprint(append([]uuid.UUID{}, want...)) {
		return fmt.Errorf("expected instruments %v, got %v", want, got)
	}
	return nil
}
