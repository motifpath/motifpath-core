//go:build integration

package bdd

import (
	"fmt"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerKnowledgeEdgeSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^skill "([^"]+)" applies concept "([^"]+)"$`, w.skillAppliesConcept)
	sc.Step(`^(skill|concept) "([^"]+)" requires (skill|concept) "([^"]+)" at level "([^"]+)"$`, w.nodeRequiresNode)

	sc.Step(`^"([^"]+)" links (skill|concept) "([^"]+)" to (skill|concept) "([^"]+)" with "([^"]+)"$`, w.linksNodes)
	sc.Step(`^"([^"]+)" links (skill|concept) "([^"]+)" to (skill|concept) "([^"]+)" with "([^"]+)" at level "([^"]+)"$`, w.linksNodesAtLevel)
	sc.Step(`^"([^"]+)" submits a create knowledge edge request whose to_id does not exist$`, w.linksToUnknownNode)
	sc.Step(`^"([^"]+)" changes that (?:applies|requires) edge to level "([^"]+)"$`, w.changesEdgeLevel)
	sc.Step(`^"([^"]+)" deletes that knowledge edge$`, w.deletesEdge)
	sc.Step(`^"([^"]+)" lists the "([^"]+)" edges from (skill|concept) "([^"]+)"$`, w.listsEdgesFrom)
	sc.Step(`^"([^"]+)" lists the edges to (skill|concept) "([^"]+)"$`, w.listsEdgesTo)
	sc.Step(`^an unauthenticated request attempts to list knowledge edges$`, w.unauthListsEdges)

	sc.Step(`^the knowledge edge is created and assigned a stable identifier$`, w.edgeCreated)
	sc.Step(`^the knowledge edge's type is "([^"]+)"$`, w.edgeTypeIs)
	sc.Step(`^the knowledge edge has no level$`, w.edgeHasNoLevel)
	sc.Step(`^the knowledge edge's level is "([^"]+)"$`, w.edgeLevelIs)
	sc.Step(`^the knowledge edge is deleted$`, w.edgeDeleted)
	sc.Step(`^(skill|concept) "([^"]+)" can now be deleted$`, w.nodeCanNowBeDeleted)
	sc.Step(`^the response includes an? "([^"]+)" edge to (skill|concept) "([^"]+)" at level "([^"]+)"$`, w.responseIncludesEdgeTo)
	sc.Step(`^the response includes an? "([^"]+)" edge from (skill|concept) "([^"]+)"$`, w.responseIncludesEdgeFrom)
	sc.Step(`^the response does not include an? "([^"]+)" edge$`, w.responseExcludesEdgeType)
}

// putEdge seeds an edge directly into w.knowledgeEdges and makes it "that"
// edge later steps refer to.
func (w *world) putEdge(fromID, toID string, edgeType domain.KnowledgeEdgeType, level *domain.MasteryLevel) error {
	id := deterministicUUID("edge", fromID, toID, string(edgeType))
	edge := domain.KnowledgeEdge{ID: id.String(), FromID: fromID, ToID: toID, Type: edgeType, Level: level}
	if err := w.knowledgeEdges.Create(w.ctx(), edge); err != nil {
		return err
	}
	w.lastEdgeID = id
	return nil
}

func (w *world) skillAppliesConcept(skill, concept string) error {
	return w.putEdge(w.skillIDFor(skill).String(), w.conceptIDFor(concept).String(), domain.KnowledgeEdgeTypeApplies, nil)
}

func (w *world) nodeRequiresNode(fromKind, from, toKind, to, level string) error {
	l := domain.MasteryLevel(level)
	return w.putEdge(w.nodeIDFor(fromKind, from).String(), w.nodeIDFor(toKind, to).String(), domain.KnowledgeEdgeTypeRequires, &l)
}

func (w *world) createEdge(body generated.CreateKnowledgeEdgeRequest) error {
	resp, err := w.handler.CreateKnowledgeEdge(w.ctx(), generated.CreateKnowledgeEdgeRequestObject{Body: &body})
	w.lastResp, w.lastErr = resp, err
	if created, ok := resp.(generated.CreateKnowledgeEdge201JSONResponse); ok {
		w.lastEdgeID = created.EdgeId
	}
	return err
}

func (w *world) linksNodes(_, fromKind, from, toKind, to, edgeType string) error {
	return w.createEdge(generated.CreateKnowledgeEdgeRequest{FromId: w.nodeIDFor(fromKind, from), ToId: w.nodeIDFor(toKind, to), Type: generated.KnowledgeEdgeType(edgeType)})
}

func (w *world) linksNodesAtLevel(_, fromKind, from, toKind, to, edgeType, level string) error {
	l := generated.MasteryLevel(level)
	return w.createEdge(generated.CreateKnowledgeEdgeRequest{FromId: w.nodeIDFor(fromKind, from), ToId: w.nodeIDFor(toKind, to), Type: generated.KnowledgeEdgeType(edgeType), Level: &l})
}

func (w *world) linksToUnknownNode(string) error {
	return w.createEdge(generated.CreateKnowledgeEdgeRequest{FromId: w.skillIDFor("improvise-over-a-blues"), ToId: deterministicUUID("knowledge-node", "does-not-exist"), Type: generated.Applies})
}

func (w *world) changesEdgeLevel(_, level string) error {
	resp, err := w.handler.UpdateKnowledgeEdge(w.ctx(), generated.UpdateKnowledgeEdgeRequestObject{
		EdgeId: w.lastEdgeID, Body: &generated.UpdateKnowledgeEdgeRequest{Level: generated.MasteryLevel(level)},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) deletesEdge(string) error {
	resp, err := w.handler.DeleteKnowledgeEdge(w.ctx(), generated.DeleteKnowledgeEdgeRequestObject{EdgeId: w.lastEdgeID})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) listEdges(params generated.ListKnowledgeEdgesParams) error {
	resp, err := w.handler.ListKnowledgeEdges(w.ctx(), generated.ListKnowledgeEdgesRequestObject{Params: params})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) listsEdgesFrom(_, edgeType, kind, name string) error {
	t := generated.KnowledgeEdgeType(edgeType)
	from := w.nodeIDFor(kind, name)
	return w.listEdges(generated.ListKnowledgeEdgesParams{Type: &t, FromId: &from})
}

func (w *world) listsEdgesTo(_, kind, name string) error {
	to := w.nodeIDFor(kind, name)
	return w.listEdges(generated.ListKnowledgeEdgesParams{ToId: &to})
}

func (w *world) unauthListsEdges() error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.listEdges(generated.ListKnowledgeEdgesParams{})
}

// lastEdge is the knowledge edge the last create or update returned.
func (w *world) lastEdge() (generated.KnowledgeEdge, error) {
	switch resp := w.lastResp.(type) {
	case generated.CreateKnowledgeEdge201JSONResponse:
		return generated.KnowledgeEdge(resp), nil
	case generated.UpdateKnowledgeEdge200JSONResponse:
		return generated.KnowledgeEdge(resp), nil
	default:
		return generated.KnowledgeEdge{}, fmt.Errorf("expected a knowledge edge response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
}

func (w *world) edgeCreated() error {
	if _, ok := w.lastResp.(generated.CreateKnowledgeEdge201JSONResponse); !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}

func (w *world) edgeTypeIs(edgeType string) error {
	edge, err := w.lastEdge()
	if err != nil {
		return err
	}
	if string(edge.Type) != edgeType {
		return fmt.Errorf("expected type %q, got %q", edgeType, edge.Type)
	}
	return nil
}

func (w *world) edgeHasNoLevel() error {
	edge, err := w.lastEdge()
	if err != nil {
		return err
	}
	if edge.Level != nil {
		return fmt.Errorf("expected no level, got %q", *edge.Level)
	}
	return nil
}

func (w *world) edgeLevelIs(level string) error {
	edge, err := w.lastEdge()
	if err != nil {
		return err
	}
	if edge.Level == nil || string(*edge.Level) != level {
		return fmt.Errorf("expected level %q, got %v", level, edge.Level)
	}
	return nil
}

func (w *world) edgeDeleted() error {
	if _, ok := w.lastResp.(generated.DeleteKnowledgeEdge204Response); !ok {
		return fmt.Errorf("expected a 204 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}

func (w *world) nodeCanNowBeDeleted(kind, name string) error {
	if err := w.deletesNode("", kind, name); err != nil {
		return err
	}
	return w.nodeDeleted()
}

func (w *world) listedEdges() (generated.ListKnowledgeEdges200JSONResponse, error) {
	resp, ok := w.lastResp.(generated.ListKnowledgeEdges200JSONResponse)
	if !ok {
		return nil, fmt.Errorf("expected a knowledge edge list, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return resp, nil
}

func (w *world) responseIncludesEdgeTo(edgeType, kind, name, level string) error {
	edges, err := w.listedEdges()
	if err != nil {
		return err
	}
	to := w.nodeIDFor(kind, name)
	for _, e := range edges {
		if string(e.Type) == edgeType && e.ToId == to && e.Level != nil && string(*e.Level) == level {
			return nil
		}
	}
	return fmt.Errorf("expected a %q edge to %s at %q, got %+v", edgeType, to, level, edges)
}

func (w *world) responseIncludesEdgeFrom(edgeType, kind, name string) error {
	edges, err := w.listedEdges()
	if err != nil {
		return err
	}
	from := w.nodeIDFor(kind, name)
	for _, e := range edges {
		if string(e.Type) == edgeType && e.FromId == from {
			return nil
		}
	}
	return fmt.Errorf("expected a %q edge from %s, got %+v", edgeType, from, edges)
}

func (w *world) responseExcludesEdgeType(edgeType string) error {
	edges, err := w.listedEdges()
	if err != nil {
		return err
	}
	for _, e := range edges {
		if string(e.Type) == edgeType {
			return fmt.Errorf("expected no %q edge, got %+v", edgeType, edges)
		}
	}
	return nil
}
