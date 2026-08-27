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
}

func (w *world) putContentNode(slug string, contentType domain.ContentType) error {
	w.lastNodeSlug = slug
	w.nodes.put(domain.ContentNode{
		ID:          nodeID(slug).String(),
		TeacherID:   deterministicUUID("motif-user", "seed-teacher").String(),
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
