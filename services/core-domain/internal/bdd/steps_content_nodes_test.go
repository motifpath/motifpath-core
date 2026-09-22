//go:build integration

package bdd

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerContentNodeSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)" is authenticated as a teacher$`, func(name string) error { w.authenticateAs(name, domain.RoleTeacher); return nil })
	sc.Step(`^"([^"]+)" is authenticated as a student$`, func(name string) error { w.authenticateAs(name, domain.RoleStudent); return nil })
	sc.Step(`^"([^"]+)" is authenticated as an admin$`, func(name string) error { w.authenticateAs(name, domain.RoleAdmin); return nil })

	sc.Step(`^a content node "([^"]+)" exists in the system$`, func(slug string) error { return w.putContentNode(slug, domain.ContentTypeVideo) })
	sc.Step(`^a video content node "([^"]+)" exists in the system$`, func(slug string) error { return w.putContentNode(slug, domain.ContentTypeVideo) })
	sc.Step(`^an article content node "([^"]+)" exists in the system$`, func(slug string) error { return w.putContentNode(slug, domain.ContentTypeArticle) })
	sc.Step(`^content nodes "([^"]+)", "([^"]+)", "([^"]+)" exist in the system$`, w.contentNodesExist)
	sc.Step(`^content nodes "([^"]+)", "([^"]+)", and "([^"]+)" exist in the system$`, w.contentNodesExist)
	sc.Step(`^a content node "([^"]+)" exists in the system with skills "([^"]+)"$`, w.putContentNodeWithSkills)
	sc.Step(`^a content node "([^"]+)" exists in the system with concepts "([^"]+)"$`, w.putContentNodeWithConcepts)

	sc.Step(`^"([^"]+)" creates a video content node titled "([^"]+)" with skills "([^"]+)", concepts "([^"]+)", and difficulty "([^"]+)"$`, w.createsContentNode(domain.ContentTypeVideo))
	sc.Step(`^"([^"]+)" creates an article content node titled "([^"]+)" with skills "([^"]+)", concepts "([^"]+)", and difficulty "([^"]+)"$`, w.createsContentNode(domain.ContentTypeArticle))
	sc.Step(`^"([^"]+)" creates a video content node titled "([^"]+)" with media url "([^"]+)", skills "([^"]+)", concepts "([^"]+)", and difficulty "([^"]+)"$`, w.createsVideoContentNodeWithMediaURL)
	sc.Step(`^"([^"]+)" creates an article content node titled "([^"]+)" with article body "([^"]+)", skills "([^"]+)", concepts "([^"]+)", and difficulty "([^"]+)"$`, w.createsArticleContentNodeWithBody)
	sc.Step(`^"([^"]+)" creates an article content node titled "([^"]+)" with article body containing an inline diagram embed of "([^"]+)", skills "([^"]+)", concepts "([^"]+)", and difficulty "([^"]+)"$`, w.createsArticleContentNodeWithDiagramEmbed)
	sc.Step(`^"([^"]+)" submits a create video content node request with the media_url field omitted$`, w.submitsVideoWithoutMediaURL)
	sc.Step(`^"([^"]+)" submits a create video content node request with media url "([^"]*)"$`, w.submitsVideoWithMediaURL)
	sc.Step(`^"([^"]+)" submits a create article content node request with the rich_content field omitted$`, w.submitsArticleWithoutBody)
	sc.Step(`^"([^"]+)" submits a create video content node request carrying rich_content$`, w.submitsVideoCarryingRichContent)
	sc.Step(`^"([^"]+)" submits a create article content node request carrying media_url$`, w.submitsArticleCarryingMediaURL)
	sc.Step(`^"([^"]+)" retrieves the content node "([^"]+)"$`, w.retrievesContentNode)
	sc.Step(`^"([^"]+)" submits a create content node request with the title field omitted$`, w.submitsContentNodeMissingTitle)
	sc.Step(`^"([^"]+)" submits a create content node request with the classification field omitted$`, w.submitsContentNodeMissingClassification)
	sc.Step(`^"([^"]+)" submits a create content node request with difficulty level "([^"]+)"$`, w.submitsContentNodeBadDifficulty)
	sc.Step(`^"([^"]+)" submits a create content node request with content_type "([^"]+)"$`, w.submitsContentNodeWithContentType)
	sc.Step(`^"([^"]+)" submits a create content node request with an empty skills list$`, w.submitsContentNodeEmptySkills)
	sc.Step(`^"([^"]+)" submits a create content node request with an empty concepts list$`, w.submitsContentNodeEmptyConcepts)
	sc.Step(`^"([^"]+)" submits a create content node request with a skill id that does not exist$`, w.submitsContentNodeBadSkillID)
	sc.Step(`^"([^"]+)" submits a create content node request with a concept id that does not exist$`, w.submitsContentNodeBadConceptID)
	sc.Step(`^"([^"]+)" attempts to create a content node$`, w.attemptsCreateContentNode)
	sc.Step(`^an unauthenticated request attempts to create a content node$`, w.unauthCreatesContentNode)
	sc.Step(`^"([^"]+)" retrieves a content node with an ID that does not exist$`, w.retrievesMissingContentNode)

	sc.Step(`^the content node is created and assigned a stable identifier$`, w.contentNodeCreated)
	sc.Step(`^the classification review state is "([^"]+)"$`, w.classificationReviewStateIs)
	sc.Step(`^the content node records "([^"]+)" as the owner$`, w.contentNodeRecordsOwner)
	sc.Step(`^the content node's media url is "([^"]+)"$`, w.contentNodeMediaURLIs)
	sc.Step(`^the response returns the content node's title, type, and classification$`, w.contentNodeResponseComplete)
	sc.Step(`^the content node's classification carries skills "([^"]+)"$`, w.contentNodeClassificationCarriesSkills)
	sc.Step(`^the content node's rich content contains a diagram node$`, w.contentNodeRichContentHasDiagramNode)

	sc.Step(`^an admin has confirmed the classification of content node "([^"]+)"$`, w.adminConfirmsClassification)

	sc.Step(`^"([^"]+)" lists all content nodes$`, w.listsAllContentNodes)
	sc.Step(`^"([^"]+)" attempts to list all content nodes$`, w.listsAllContentNodes)
	sc.Step(`^an unauthenticated request attempts to list all content nodes$`, w.unauthListsAllContentNodes)
	sc.Step(`^"([^"]+)" lists content nodes filtered by content_type "([^"]+)"$`, w.listsContentNodesByType)
	sc.Step(`^"([^"]+)" lists content nodes filtered by skill "([^"]+)"$`, w.listsContentNodesBySkill)
	sc.Step(`^"([^"]+)" lists content nodes filtered by concept "([^"]+)"$`, w.listsContentNodesByConcept)

	sc.Step(`^"([^"]+)" updates content node "([^"]+)" with title "([^"]+)" and skills "([^"]+)", concepts "([^"]+)", and difficulty "([^"]+)"$`, w.updatesContentNodeFull)
	sc.Step(`^"([^"]+)" updates content node "([^"]+)" with skills "([^"]+)"$`, w.updatesContentNodeSkillsOnly)
	sc.Step(`^"([^"]+)" updates content node "([^"]+)" with title "([^"]+)"$`, w.updatesContentNodeTitleOnly)
	sc.Step(`^"([^"]+)" attempts to update content node "([^"]+)" with title "([^"]+)"$`, w.updatesContentNodeTitleOnly)
	sc.Step(`^an unauthenticated request attempts to update content node "([^"]+)" with title "([^"]+)"$`, w.unauthUpdatesContentNode)
	sc.Step(`^"([^"]+)" submits an update content node request for "([^"]+)" with the title field omitted$`, w.submitsUpdateContentNodeMissingTitle)
	sc.Step(`^"([^"]+)" submits an update content node request for "([^"]+)" with media url "([^"]*)"$`, w.submitsUpdateContentNodeWithMediaURL)
	sc.Step(`^"([^"]+)" submits an update content node request for "([^"]+)" with the classification field omitted$`, w.submitsUpdateContentNodeMissingClassification)
	sc.Step(`^"([^"]+)" attempts to update a content node with an ID that does not exist$`, w.attemptsUpdateMissingContentNode)

	sc.Step(`^the content node's title is "([^"]+)"$`, w.contentNodeTitleIs)
	sc.Step(`^the content node's classification difficulty is "([^"]+)"$`, w.contentNodeDifficultyIs)
	sc.Step(`^the content node's type is still "([^"]+)"$`, w.contentNodeTypeIs)
	sc.Step(`^the classification review state is still "([^"]+)"$`, w.classificationReviewStateStillIs)
}

// classificationInputFor builds a ClassificationInput resolving comma-listed
// skill/concept names to ids via the world's shared registry, auto-creating
// any name not already known as a root node.
func (w *world) classificationInputFor(skills, concepts, difficulty string) generated.ClassificationInput {
	return generated.ClassificationInput{
		SkillIds:        w.skillIDsFor(skills),
		ConceptIds:      w.conceptIDsFor(concepts),
		DifficultyLevel: generated.ClassificationInputDifficultyLevel(difficulty),
	}
}

func (w *world) putContentNodeWithSkills(slug, skills string) error {
	w.lastNodeSlug = slug
	w.nodes.put(domain.ContentNode{
		ID:          nodeID(slug).String(),
		TeacherID:   w.ensureRegistered("bob", domain.RoleTeacher).String(),
		Title:       slug,
		ContentType: domain.ContentTypeVideo,
		Classification: domain.Classification{
			Skills:          skillsFromUUIDs(w.skillIDsFor(skills)),
			Concepts:        []domain.Concept{{ID: w.conceptIDFor("concept-" + slug).String(), Name: "concept-" + slug}},
			DifficultyLevel: domain.DifficultyLevelBeginner, ReviewState: domain.ReviewStatePending,
		},
		CreatedAt: fixedNow,
	})
	return nil
}

func (w *world) putContentNodeWithConcepts(slug, concepts string) error {
	w.lastNodeSlug = slug
	w.nodes.put(domain.ContentNode{
		ID:          nodeID(slug).String(),
		TeacherID:   w.ensureRegistered("bob", domain.RoleTeacher).String(),
		Title:       slug,
		ContentType: domain.ContentTypeVideo,
		Classification: domain.Classification{
			Skills:          []domain.Skill{{ID: w.skillIDFor("skill-" + slug).String(), Name: "skill-" + slug}},
			Concepts:        conceptsFromUUIDs(w.conceptIDsFor(concepts)),
			DifficultyLevel: domain.DifficultyLevelBeginner, ReviewState: domain.ReviewStatePending,
		},
		CreatedAt: fixedNow,
	})
	return nil
}

func skillsFromUUIDs(ids []uuid.UUID) []domain.Skill {
	result := make([]domain.Skill, len(ids))
	for i, id := range ids {
		result[i] = domain.Skill{ID: id.String()}
	}
	return result
}

func conceptsFromUUIDs(ids []uuid.UUID) []domain.Concept {
	result := make([]domain.Concept, len(ids))
	for i, id := range ids {
		result[i] = domain.Concept{ID: id.String()}
	}
	return result
}

func (w *world) adminConfirmsClassification(slug string) error {
	node, err := w.nodes.GetByID(w.ctx(), nodeID(slug).String())
	if err != nil {
		return err
	}
	node.Classification.ReviewState = domain.ReviewStateConfirmed
	w.nodes.put(node)
	return nil
}

func (w *world) listsAllContentNodes(name string) error {
	resp, err := w.handler.ListContentNodes(w.ctx(), generated.ListContentNodesRequestObject{})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthListsAllContentNodes() error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.listsAllContentNodes("")
}

func (w *world) listsContentNodesByType(name, contentType string) error {
	ct := generated.ListContentNodesParamsContentType(contentType)
	resp, err := w.handler.ListContentNodes(w.ctx(), generated.ListContentNodesRequestObject{
		Params: generated.ListContentNodesParams{ContentType: &ct},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) listsContentNodesBySkill(name, skillName string) error {
	id := w.skillIDFor(skillName)
	resp, err := w.handler.ListContentNodes(w.ctx(), generated.ListContentNodesRequestObject{
		Params: generated.ListContentNodesParams{SkillId: &id},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) listsContentNodesByConcept(name, conceptName string) error {
	id := w.conceptIDFor(conceptName)
	resp, err := w.handler.ListContentNodes(w.ctx(), generated.ListContentNodesRequestObject{
		Params: generated.ListContentNodesParams{ConceptId: &id},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) updatesContentNodeFull(name, slug, title, skills, concepts, difficulty string) error {
	existing, err := w.nodes.GetByID(w.ctx(), nodeID(slug).String())
	if err != nil {
		return err
	}
	mediaURL, richContent := bodyFor(existing)
	resp, err := w.handler.UpdateContentNode(w.ctx(), generated.UpdateContentNodeRequestObject{
		ContentNodeId: nodeID(slug),
		Body: &generated.UpdateContentNodeRequest{
			Title:          title,
			Classification: w.classificationInputFor(skills, concepts, difficulty),
			MediaUrl:       mediaURL,
			RichContent:    richContent,
			LanguageCodes:  []string{"en"},
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

// updatesContentNodeSkillsOnly replaces slug's skill_ids while resubmitting
// its current title, concept_ids, and difficulty unchanged — a real client
// always resends the whole classification (ClassificationInput has no
// partial-update shape), so this step reads the node's current values for
// everything the Gherkin text doesn't mention.
func (w *world) updatesContentNodeSkillsOnly(name, slug, skills string) error {
	existing, err := w.nodes.GetByID(w.ctx(), nodeID(slug).String())
	if err != nil {
		return err
	}
	mediaURL, richContent := bodyFor(existing)
	resp, err := w.handler.UpdateContentNode(w.ctx(), generated.UpdateContentNodeRequestObject{
		ContentNodeId: nodeID(slug),
		Body: &generated.UpdateContentNodeRequest{
			Title:       existing.Title,
			MediaUrl:    mediaURL,
			RichContent: richContent,
			Classification: generated.ClassificationInput{
				SkillIds:        w.skillIDsFor(skills),
				ConceptIds:      idsOfConcepts(existing.Classification.Concepts),
				DifficultyLevel: generated.ClassificationInputDifficultyLevel(existing.Classification.DifficultyLevel),
			},
			LanguageCodes: []string{"en"},
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func idsOfSkills(skills []domain.Skill) []uuid.UUID {
	ids := make([]uuid.UUID, len(skills))
	for i, s := range skills {
		ids[i] = uuid.MustParse(s.ID)
	}
	return ids
}

func idsOfConcepts(concepts []domain.Concept) []uuid.UUID {
	ids := make([]uuid.UUID, len(concepts))
	for i, c := range concepts {
		ids[i] = uuid.MustParse(c.ID)
	}
	return ids
}

// updatesContentNodeTitleOnly changes slug's title while resubmitting its
// current classification unchanged — see updatesContentNodeSkillsOnly's doc
// comment for why the whole classification is always resent.
func (w *world) updatesContentNodeTitleOnly(name, slug, title string) error {
	existing, err := w.nodes.GetByID(w.ctx(), nodeID(slug).String())
	if err != nil {
		return err
	}
	mediaURL, richContent := bodyFor(existing)
	resp, err := w.handler.UpdateContentNode(w.ctx(), generated.UpdateContentNodeRequestObject{
		ContentNodeId: nodeID(slug),
		Body: &generated.UpdateContentNodeRequest{
			Title:       title,
			MediaUrl:    mediaURL,
			RichContent: richContent,
			Classification: generated.ClassificationInput{
				SkillIds:        idsOfSkills(existing.Classification.Skills),
				ConceptIds:      idsOfConcepts(existing.Classification.Concepts),
				DifficultyLevel: generated.ClassificationInputDifficultyLevel(existing.Classification.DifficultyLevel),
			},
			LanguageCodes: []string{"en"},
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthUpdatesContentNode(slug, title string) error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.updatesContentNodeTitleOnly("", slug, title)
}

func (w *world) submitsUpdateContentNodeMissingTitle(name, slug string) error {
	resp, err := w.handler.UpdateContentNode(w.ctx(), generated.UpdateContentNodeRequestObject{
		ContentNodeId: nodeID(slug),
		Body: &generated.UpdateContentNodeRequest{
			Classification: w.classificationInputFor("s", "c", "beginner"),
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsUpdateContentNodeWithMediaURL(_, slug, mediaURL string) error {
	resp, err := w.handler.UpdateContentNode(w.ctx(), generated.UpdateContentNodeRequestObject{
		ContentNodeId: nodeID(slug),
		Body: &generated.UpdateContentNodeRequest{
			Title:          "Title",
			Classification: w.classificationInputFor("s", "c", "beginner"),
			MediaUrl:       &mediaURL,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsUpdateContentNodeMissingClassification(name, slug string) error {
	resp, err := w.handler.UpdateContentNode(w.ctx(), generated.UpdateContentNodeRequestObject{
		ContentNodeId: nodeID(slug),
		Body:          &generated.UpdateContentNodeRequest{Title: "Title"},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) attemptsUpdateMissingContentNode(string) error {
	resp, err := w.handler.UpdateContentNode(w.ctx(), generated.UpdateContentNodeRequestObject{
		ContentNodeId: deterministicUUID("node", "does-not-exist"),
		Body: &generated.UpdateContentNodeRequest{
			Title:          "Title",
			Classification: w.classificationInputFor("s", "c", "beginner"),
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) contentNodeTitleIs(want string) error {
	resp, ok := w.lastResp.(generated.UpdateContentNode200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.Title != want {
		return fmt.Errorf("expected title %q, got %q", want, resp.Title)
	}
	return nil
}

func (w *world) contentNodeDifficultyIs(want string) error {
	resp, ok := w.lastResp.(generated.UpdateContentNode200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if string(resp.Classification.DifficultyLevel) != want {
		return fmt.Errorf("expected difficulty_level %q, got %q", want, resp.Classification.DifficultyLevel)
	}
	return nil
}

func (w *world) contentNodeTypeIs(want string) error {
	resp, ok := w.lastResp.(generated.UpdateContentNode200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if string(resp.ContentType) != want {
		return fmt.Errorf("expected content_type %q, got %q", want, resp.ContentType)
	}
	return nil
}

func (w *world) classificationReviewStateStillIs(want string) error {
	resp, ok := w.lastResp.(generated.UpdateContentNode200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if string(resp.Classification.ReviewState) != want {
		return fmt.Errorf("expected review_state %q, got %q", want, resp.Classification.ReviewState)
	}
	return nil
}

func (w *world) contentNodeClassificationCarriesSkills(names string) error {
	var skills []generated.Skill
	switch resp := w.lastResp.(type) {
	case generated.CreateContentNode201JSONResponse:
		skills = resp.Classification.Skills
	case generated.UpdateContentNode200JSONResponse:
		skills = resp.Classification.Skills
	default:
		return fmt.Errorf("expected a content node response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	for _, want := range splitCommaList(names) {
		found := false
		for _, s := range skills {
			if s.Name == want {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("expected classification to carry skill %q, got %+v", want, skills)
		}
	}
	return nil
}

// putContentNode's default classification includes "triad-shapes" and
// "chord-theory" (as both a skill and a concept) alongside a node-specific
// skill/concept — challenges.feature and exercises.feature scenarios
// reference "intro-to-triads" (this suite's default node) with a challenge
// subject of "triad-shapes" or "chord-theory" without a separate
// classification-setup step, so those names must already be part of every
// content node's classification for the subject-membership check to pass.
func (w *world) putContentNode(slug string, contentType domain.ContentType) error {
	w.lastNodeSlug = slug
	node := domain.ContentNode{
		ID:          nodeID(slug).String(),
		TeacherID:   w.ensureRegistered("bob", domain.RoleTeacher).String(),
		Title:       slug,
		ContentType: contentType,
		Classification: domain.Classification{
			Skills: []domain.Skill{
				{ID: w.skillIDFor("skill-" + slug).String(), Name: "skill-" + slug},
				{ID: w.skillIDFor("triad-shapes").String(), Name: "triad-shapes"},
				{ID: w.skillIDFor("chord-theory").String(), Name: "chord-theory"},
			},
			Concepts: []domain.Concept{
				{ID: w.conceptIDFor("concept-" + slug).String(), Name: "concept-" + slug},
				{ID: w.conceptIDFor("chord-theory").String(), Name: "chord-theory"},
			},
			DifficultyLevel: domain.DifficultyLevelBeginner, ReviewState: domain.ReviewStatePending,
		},
		// Matches the "en" default a registered user's locale resolves to
		// (see RegisterUser), so student-path-view scenarios that don't
		// specifically exercise language locking see this node's
		// prerequisite-based lock state unaffected by it.
		Languages: []domain.Language{{Code: "en"}},
		CreatedAt: fixedNow,
	}
	if contentType == domain.ContentTypeArticle {
		body := domain.NewPlainTextPrompt("Default article body.")
		node.RichContent = &body
	} else {
		node.MediaURL = defaultMediaURL()
	}
	w.nodes.put(node)
	return nil
}

func (w *world) contentNodesExist(a, b, c string) error {
	for _, slug := range []string{a, b, c} {
		if err := w.putContentNode(slug, domain.ContentTypeVideo); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) createsContentNode(contentType domain.ContentType) func(name, title, skills, concepts, difficulty string) error {
	return func(name, title, skills, concepts, difficulty string) error {
		if contentType == domain.ContentTypeVideo {
			return w.createsContentNodeWithBody(contentType, title, skills, concepts, difficulty, defaultMediaURL(), nil)
		}
		return w.createsContentNodeWithBody(contentType, title, skills, concepts, difficulty, nil, defaultArticleBody())
	}
}

func (w *world) createsVideoContentNodeWithMediaURL(name, title, mediaURL, skills, concepts, difficulty string) error {
	return w.createsContentNodeWithBody(domain.ContentTypeVideo, title, skills, concepts, difficulty, &mediaURL, nil)
}

func (w *world) createsArticleContentNodeWithBody(name, title, body, skills, concepts, difficulty string) error {
	doc := promptDocFor(body)
	return w.createsContentNodeWithBody(domain.ContentTypeArticle, title, skills, concepts, difficulty, nil, &doc)
}

// diagramPromptNode builds a "diagram" PromptNode embedding diagramSlug via
// diagramRef — the generated.PromptNode.Attrs field is a generic map
// (validated by the authoring editor, not this schema), so the key names
// here must match domain.PromptNodeAttrs.DiagramRef's own json tag
// ("diagramRef", camelCase, ProseMirror's attrs convention) and
// domain.DiagramRef's own tags (snake_case) for the generic
// toDomainPromptDocument JSON round trip to decode it correctly.
func diagramPromptNode(diagramSlug string) generated.PromptNode {
	attrs := map[string]interface{}{
		"diagramRef": map[string]interface{}{
			"diagram_id": diagramID(diagramSlug).String(),
			"layers":     map[string]interface{}{"intervals": true},
		},
	}
	return generated.PromptNode{Type: generated.PromptNodeTypeDiagram, Attrs: &attrs}
}

func (w *world) createsArticleContentNodeWithDiagramEmbed(name, title, diagramSlug, skills, concepts, difficulty string) error {
	doc := generated.PromptDocument{Type: generated.Doc, Content: []generated.PromptNode{diagramPromptNode(diagramSlug)}}
	return w.createsContentNodeWithBody(domain.ContentTypeArticle, title, skills, concepts, difficulty, nil, &doc)
}

func (w *world) contentNodeRichContentHasDiagramNode() error {
	resp, ok := w.lastResp.(generated.CreateContentNode201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.RichContent == nil {
		return fmt.Errorf("expected rich_content to be set, got %+v", resp)
	}
	for _, node := range resp.RichContent.Content {
		if node.Type == generated.PromptNodeTypeDiagram {
			return nil
		}
	}
	return fmt.Errorf("expected rich_content to contain a diagram node, got %+v", resp.RichContent)
}

func (w *world) createsContentNodeWithBody(contentType domain.ContentType, title, skills, concepts, difficulty string, mediaURL *string, richContent *generated.PromptDocument) error {
	resp, err := w.handler.CreateContentNode(w.ctx(), generated.CreateContentNodeRequestObject{
		Body: &generated.CreateContentNodeRequest{
			Title:          title,
			ContentType:    generated.CreateContentNodeRequestContentType(contentType),
			Classification: w.classificationInputFor(skills, concepts, difficulty),
			MediaUrl:       mediaURL,
			RichContent:    richContent,
			LanguageCodes:  []string{"en"},
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

// submitsContentNodeWithBody submits a create request of contentType whose
// body fields are exactly mediaURL/richContent, for scenarios that break the
// video-xor-article body rule on purpose.
func (w *world) submitsContentNodeWithBody(contentType domain.ContentType, mediaURL *string, richContent *generated.PromptDocument) error {
	return w.createsContentNodeWithBody(contentType, "Title", "s", "c", "beginner", mediaURL, richContent)
}

func (w *world) submitsVideoWithoutMediaURL(string) error {
	return w.submitsContentNodeWithBody(domain.ContentTypeVideo, nil, nil)
}

func (w *world) submitsVideoWithMediaURL(_, mediaURL string) error {
	return w.submitsContentNodeWithBody(domain.ContentTypeVideo, &mediaURL, nil)
}

func (w *world) submitsArticleWithoutBody(string) error {
	return w.submitsContentNodeWithBody(domain.ContentTypeArticle, nil, nil)
}

func (w *world) submitsVideoCarryingRichContent(string) error {
	return w.submitsContentNodeWithBody(domain.ContentTypeVideo, defaultMediaURL(), defaultArticleBody())
}

func (w *world) submitsArticleCarryingMediaURL(string) error {
	return w.submitsContentNodeWithBody(domain.ContentTypeArticle, defaultMediaURL(), defaultArticleBody())
}

func (w *world) contentNodeMediaURLIs(want string) error {
	resp, ok := w.lastResp.(generated.CreateContentNode201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v", w.lastResp)
	}
	if resp.MediaUrl == nil || *resp.MediaUrl != want {
		return fmt.Errorf("expected media_url %q, got %v", want, resp.MediaUrl)
	}
	return nil
}

// defaultMediaURL is the video body steps that don't care about the body
// submit, so a fully valid create/update request carries one.
func defaultMediaURL() *string {
	url := "https://cdn.motifpath.io/videos/default.mp4"
	return &url
}

// defaultArticleBody is defaultMediaURL's counterpart for article nodes.
func defaultArticleBody() *generated.PromptDocument {
	doc := promptDocFor("Default article body.")
	return &doc
}

// bodyFor returns the request body fields a full-replace update of existing
// must resend: its own media_url for a video, and a default document for an
// article (the fake repository does not round-trip the wire-format document).
func bodyFor(existing domain.ContentNode) (*string, *generated.PromptDocument) {
	if existing.ContentType == domain.ContentTypeArticle {
		return nil, defaultArticleBody()
	}
	return defaultMediaURL(), nil
}

func (w *world) retrievesContentNode(name, slug string) error {
	resp, err := w.handler.GetContentNode(w.ctx(), generated.GetContentNodeRequestObject{ContentNodeId: nodeID(slug)})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsContentNodeMissingTitle(string) error {
	resp, err := w.handler.CreateContentNode(w.ctx(), generated.CreateContentNodeRequestObject{
		Body: &generated.CreateContentNodeRequest{
			ContentType:    generated.CreateContentNodeRequestContentTypeVideo,
			MediaUrl:       defaultMediaURL(),
			Classification: w.classificationInputFor("s", "c", "beginner"),
			LanguageCodes:  []string{"en"},
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsContentNodeMissingClassification(string) error {
	resp, err := w.handler.CreateContentNode(w.ctx(), generated.CreateContentNodeRequestObject{
		Body: &generated.CreateContentNodeRequest{
			Title:         "Title",
			ContentType:   generated.CreateContentNodeRequestContentTypeVideo,
			LanguageCodes: []string{"en"},
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsContentNodeBadDifficulty(name, difficulty string) error {
	resp, err := w.handler.CreateContentNode(w.ctx(), generated.CreateContentNodeRequestObject{
		Body: &generated.CreateContentNodeRequest{
			Title:          "Title",
			ContentType:    generated.CreateContentNodeRequestContentTypeVideo,
			MediaUrl:       defaultMediaURL(),
			Classification: w.classificationInputFor("s", "c", difficulty),
			LanguageCodes:  []string{"en"},
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

// submitsContentNodeWithContentType submits a create request with an
// arbitrary content_type string, for scenarios proving a value outside
// video/article (e.g. "diagram" — never a ContentNode's own content_type,
// only ever an inline PromptNode embed within one) is rejected. The
// generated request type only defines Video/Article constants, so an
// out-of-enum value needs an explicit string conversion rather than one of
// those constants.
func (w *world) submitsContentNodeWithContentType(name, contentType string) error {
	resp, err := w.handler.CreateContentNode(w.ctx(), generated.CreateContentNodeRequestObject{
		Body: &generated.CreateContentNodeRequest{
			Title:          "Title",
			ContentType:    generated.CreateContentNodeRequestContentType(contentType),
			MediaUrl:       defaultMediaURL(),
			Classification: w.classificationInputFor("s", "c", "beginner"),
			LanguageCodes:  []string{"en"},
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsContentNodeEmptySkills(string) error {
	resp, err := w.handler.CreateContentNode(w.ctx(), generated.CreateContentNodeRequestObject{
		Body: &generated.CreateContentNodeRequest{
			Title:       "Title",
			ContentType: generated.CreateContentNodeRequestContentTypeVideo,
			MediaUrl:    defaultMediaURL(),
			Classification: generated.ClassificationInput{
				SkillIds: nil, ConceptIds: w.conceptIDsFor("c"), DifficultyLevel: generated.ClassificationInputDifficultyLevelBeginner,
			},
			LanguageCodes: []string{"en"},
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsContentNodeEmptyConcepts(string) error {
	resp, err := w.handler.CreateContentNode(w.ctx(), generated.CreateContentNodeRequestObject{
		Body: &generated.CreateContentNodeRequest{
			Title:       "Title",
			ContentType: generated.CreateContentNodeRequestContentTypeVideo,
			MediaUrl:    defaultMediaURL(),
			Classification: generated.ClassificationInput{
				SkillIds: w.skillIDsFor("s"), ConceptIds: nil, DifficultyLevel: generated.ClassificationInputDifficultyLevelBeginner,
			},
			LanguageCodes: []string{"en"},
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsContentNodeBadSkillID(string) error {
	missing := deterministicUUID("skill", "does-not-exist")
	resp, err := w.handler.CreateContentNode(w.ctx(), generated.CreateContentNodeRequestObject{
		Body: &generated.CreateContentNodeRequest{
			Title:       "Title",
			ContentType: generated.CreateContentNodeRequestContentTypeVideo,
			MediaUrl:    defaultMediaURL(),
			Classification: generated.ClassificationInput{
				SkillIds: []uuid.UUID{missing}, ConceptIds: w.conceptIDsFor("c"), DifficultyLevel: generated.ClassificationInputDifficultyLevelBeginner,
			},
			LanguageCodes: []string{"en"},
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsContentNodeBadConceptID(string) error {
	missing := deterministicUUID("concept", "does-not-exist")
	resp, err := w.handler.CreateContentNode(w.ctx(), generated.CreateContentNodeRequestObject{
		Body: &generated.CreateContentNodeRequest{
			Title:       "Title",
			ContentType: generated.CreateContentNodeRequestContentTypeVideo,
			MediaUrl:    defaultMediaURL(),
			Classification: generated.ClassificationInput{
				SkillIds: w.skillIDsFor("s"), ConceptIds: []uuid.UUID{missing}, DifficultyLevel: generated.ClassificationInputDifficultyLevelBeginner,
			},
			LanguageCodes: []string{"en"},
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) attemptsCreateContentNode(string) error {
	resp, err := w.handler.CreateContentNode(w.ctx(), generated.CreateContentNodeRequestObject{
		Body: &generated.CreateContentNodeRequest{
			Title:          "Title",
			ContentType:    generated.CreateContentNodeRequestContentTypeVideo,
			MediaUrl:       defaultMediaURL(),
			Classification: w.classificationInputFor("s", "c", "beginner"),
			LanguageCodes:  []string{"en"},
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthCreatesContentNode() error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.attemptsCreateContentNode("")
}

func (w *world) retrievesMissingContentNode(string) error {
	resp, err := w.handler.GetContentNode(w.ctx(), generated.GetContentNodeRequestObject{ContentNodeId: deterministicUUID("node", "does-not-exist")})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) contentNodeCreated() error {
	if _, ok := w.lastResp.(generated.CreateContentNode201JSONResponse); !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}

func (w *world) classificationReviewStateIs(state string) error {
	resp, ok := w.lastResp.(generated.CreateContentNode201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v", w.lastResp)
	}
	if string(resp.Classification.ReviewState) != state {
		return fmt.Errorf("expected review_state %q, got %q", state, resp.Classification.ReviewState)
	}
	return nil
}

func (w *world) contentNodeRecordsOwner(name string) error {
	resp, ok := w.lastResp.(generated.CreateContentNode201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v", w.lastResp)
	}
	if resp.TeacherId != w.userMotifID[name] {
		return fmt.Errorf("expected teacher_id %s for %q, got %s", w.userMotifID[name], name, resp.TeacherId)
	}
	return nil
}

func (w *world) contentNodeResponseComplete() error {
	resp, ok := w.lastResp.(generated.GetContentNode200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.Title == "" || resp.ContentType == "" || len(resp.Classification.Skills) == 0 {
		return fmt.Errorf("expected a fully populated content node, got %+v", resp)
	}
	return nil
}
