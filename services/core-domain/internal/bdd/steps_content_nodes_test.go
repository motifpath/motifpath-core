//go:build integration

package bdd

import (
	"fmt"

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

	sc.Step(`^"([^"]+)" creates a video content node titled "([^"]+)"\s+with skill "([^"]+)", concept "([^"]+)", and difficulty "([^"]+)"$`, w.createsContentNode(domain.ContentTypeVideo))
	sc.Step(`^"([^"]+)" creates an article content node titled "([^"]+)"\s+with skill "([^"]+)", concept "([^"]+)", and difficulty "([^"]+)"$`, w.createsContentNode(domain.ContentTypeArticle))
	sc.Step(`^"([^"]+)" retrieves the content node "([^"]+)"$`, w.retrievesContentNode)
	sc.Step(`^"([^"]+)" submits a create content node request with the title field omitted$`, w.submitsContentNodeMissingTitle)
	sc.Step(`^"([^"]+)" submits a create content node request with the classification field omitted$`, w.submitsContentNodeMissingClassification)
	sc.Step(`^"([^"]+)" submits a create content node request with difficulty level "([^"]+)"$`, w.submitsContentNodeBadDifficulty)
	sc.Step(`^"([^"]+)" attempts to create a content node$`, w.attemptsCreateContentNode)
	sc.Step(`^an unauthenticated request attempts to create a content node$`, w.unauthCreatesContentNode)
	sc.Step(`^"([^"]+)" retrieves a content node with an ID that does not exist$`, w.retrievesMissingContentNode)

	sc.Step(`^the content node is created and assigned a stable identifier$`, w.contentNodeCreated)
	sc.Step(`^the classification review state is "([^"]+)"$`, w.classificationReviewStateIs)
	sc.Step(`^the content node records "([^"]+)" as the owner$`, w.contentNodeRecordsOwner)
	sc.Step(`^the response returns the content node's title, type, and classification$`, w.contentNodeResponseComplete)

	sc.Step(`^a content node "([^"]+)" exists in the system with skill "([^"]+)"$`, w.putContentNodeWithSkill)
	sc.Step(`^an admin has confirmed the classification of content node "([^"]+)"$`, w.adminConfirmsClassification)

	sc.Step(`^"([^"]+)" lists all content nodes$`, w.listsAllContentNodes)
	sc.Step(`^"([^"]+)" attempts to list all content nodes$`, w.listsAllContentNodes)
	sc.Step(`^an unauthenticated request attempts to list all content nodes$`, w.unauthListsAllContentNodes)
	sc.Step(`^"([^"]+)" lists content nodes filtered by content_type "([^"]+)"$`, w.listsContentNodesByType)
	sc.Step(`^"([^"]+)" lists content nodes filtered by skill "([^"]+)"$`, w.listsContentNodesBySkill)

	sc.Step(`^"([^"]+)" updates content node "([^"]+)" with title "([^"]+)" and skill "([^"]+)", concept "([^"]+)", and difficulty "([^"]+)"$`, w.updatesContentNodeFull)
	sc.Step(`^"([^"]+)" updates content node "([^"]+)" with title "([^"]+)"$`, w.updatesContentNodeTitleOnly)
	sc.Step(`^"([^"]+)" attempts to update content node "([^"]+)" with title "([^"]+)"$`, w.updatesContentNodeTitleOnly)
	sc.Step(`^an unauthenticated request attempts to update content node "([^"]+)" with title "([^"]+)"$`, w.unauthUpdatesContentNode)
	sc.Step(`^"([^"]+)" submits an update content node request for "([^"]+)" with the title field omitted$`, w.submitsUpdateContentNodeMissingTitle)
	sc.Step(`^"([^"]+)" submits an update content node request for "([^"]+)" with the classification field omitted$`, w.submitsUpdateContentNodeMissingClassification)
	sc.Step(`^"([^"]+)" attempts to update a content node with an ID that does not exist$`, w.attemptsUpdateMissingContentNode)

	sc.Step(`^the content node's title is "([^"]+)"$`, w.contentNodeTitleIs)
	sc.Step(`^the content node's classification difficulty is "([^"]+)"$`, w.contentNodeDifficultyIs)
	sc.Step(`^the content node's type is still "([^"]+)"$`, w.contentNodeTypeIs)
	sc.Step(`^the classification review state is still "([^"]+)"$`, w.classificationReviewStateStillIs)
}

func (w *world) putContentNodeWithSkill(slug, skill string) error {
	w.lastNodeSlug = slug
	w.nodes.put(domain.ContentNode{
		ID:          nodeID(slug).String(),
		TeacherID:   w.ensureRegistered("bob", domain.RoleTeacher).String(),
		Title:       slug,
		ContentType: domain.ContentTypeVideo,
		Classification: domain.Classification{
			Skill: skill, Concept: "concept-" + slug,
			DifficultyLevel: domain.DifficultyLevelBeginner, ReviewState: domain.ReviewStatePending,
		},
		CreatedAt: fixedNow,
	})
	return nil
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

func (w *world) listsContentNodesBySkill(name, skill string) error {
	resp, err := w.handler.ListContentNodes(w.ctx(), generated.ListContentNodesRequestObject{
		Params: generated.ListContentNodesParams{Skill: &skill},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) updatesContentNodeFull(name, slug, title, skill, concept, difficulty string) error {
	resp, err := w.handler.UpdateContentNode(w.ctx(), generated.UpdateContentNodeRequestObject{
		ContentNodeId: nodeID(slug),
		Body: &generated.UpdateContentNodeRequest{
			Title: title,
			Classification: generated.ClassificationInput{
				Skill: skill, Concept: concept,
				DifficultyLevel: generated.ClassificationInputDifficultyLevel(difficulty),
			},
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) updatesContentNodeTitleOnly(name, slug, title string) error {
	resp, err := w.handler.UpdateContentNode(w.ctx(), generated.UpdateContentNodeRequestObject{
		ContentNodeId: nodeID(slug),
		Body: &generated.UpdateContentNodeRequest{
			Title: title,
			Classification: generated.ClassificationInput{
				Skill: "s", Concept: "c", DifficultyLevel: generated.ClassificationInputDifficultyLevelBeginner,
			},
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
			Classification: generated.ClassificationInput{
				Skill: "s", Concept: "c", DifficultyLevel: generated.ClassificationInputDifficultyLevelBeginner,
			},
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
			Title: "Title",
			Classification: generated.ClassificationInput{
				Skill: "s", Concept: "c", DifficultyLevel: generated.ClassificationInputDifficultyLevelBeginner,
			},
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

func (w *world) putContentNode(slug string, contentType domain.ContentType) error {
	w.lastNodeSlug = slug
	w.nodes.put(domain.ContentNode{
		ID:          nodeID(slug).String(),
		TeacherID:   w.ensureRegistered("bob", domain.RoleTeacher).String(),
		Title:       slug,
		ContentType: contentType,
		Classification: domain.Classification{
			Skill: "skill-" + slug, Concept: "concept-" + slug,
			DifficultyLevel: domain.DifficultyLevelBeginner, ReviewState: domain.ReviewStatePending,
		},
		CreatedAt: fixedNow,
	})
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

func (w *world) createsContentNode(contentType domain.ContentType) func(name, title, skill, concept, difficulty string) error {
	return func(name, title, skill, concept, difficulty string) error {
		resp, err := w.handler.CreateContentNode(w.ctx(), generated.CreateContentNodeRequestObject{
			Body: &generated.CreateContentNodeRequest{
				Title:       title,
				ContentType: generated.CreateContentNodeRequestContentType(contentType),
				Classification: generated.ClassificationInput{
					Skill: skill, Concept: concept,
					DifficultyLevel: generated.ClassificationInputDifficultyLevel(difficulty),
				},
			},
		})
		w.lastResp, w.lastErr = resp, err
		return err
	}
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
			Classification: generated.ClassificationInput{Skill: "s", Concept: "c", DifficultyLevel: generated.ClassificationInputDifficultyLevelBeginner},
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsContentNodeMissingClassification(string) error {
	resp, err := w.handler.CreateContentNode(w.ctx(), generated.CreateContentNodeRequestObject{
		Body: &generated.CreateContentNodeRequest{
			Title:       "Title",
			ContentType: generated.CreateContentNodeRequestContentTypeVideo,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsContentNodeBadDifficulty(name, difficulty string) error {
	resp, err := w.handler.CreateContentNode(w.ctx(), generated.CreateContentNodeRequestObject{
		Body: &generated.CreateContentNodeRequest{
			Title:       "Title",
			ContentType: generated.CreateContentNodeRequestContentTypeVideo,
			Classification: generated.ClassificationInput{
				Skill: "s", Concept: "c", DifficultyLevel: generated.ClassificationInputDifficultyLevel(difficulty),
			},
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) attemptsCreateContentNode(string) error {
	resp, err := w.handler.CreateContentNode(w.ctx(), generated.CreateContentNodeRequestObject{
		Body: &generated.CreateContentNodeRequest{
			Title:       "Title",
			ContentType: generated.CreateContentNodeRequestContentTypeVideo,
			Classification: generated.ClassificationInput{
				Skill: "s", Concept: "c", DifficultyLevel: generated.ClassificationInputDifficultyLevelBeginner,
			},
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
	if resp.Title == "" || resp.ContentType == "" || resp.Classification.Skill == "" {
		return fmt.Errorf("expected a fully populated content node, got %+v", resp)
	}
	return nil
}
