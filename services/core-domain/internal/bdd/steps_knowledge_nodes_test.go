//go:build integration

package bdd

import (
	"fmt"
	"slices"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerKnowledgeNodeSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^a root (skill|concept) "([^"]+)" exists in the system$`, w.aRootNodeExists)
	sc.Step(`^a (skill|concept) "([^"]+)" exists under (?:skill|concept) "([^"]+)"$`, w.aNodeExistsUnder)
	sc.Step(`^a root concept "([^"]+)" with a description exists in the system$`, w.aRootConceptWithDescriptionExists)
	sc.Step(`^a skill "([^"]+)" for instrument "([^"]+)" exists under skill "([^"]+)"$`, w.aSkillForInstrumentExistsUnder)
	sc.Step(`^a root skill "([^"]+)" for instrument "([^"]+)" exists in the system$`, w.aRootSkillForInstrumentExists)
	sc.Step(`^a root skill "([^"]+)" for instruments "([^"]+)" and "([^"]+)" exists in the system$`, w.aRootSkillForInstrumentsExists)

	sc.Step(`^"([^"]+)" creates a (skill|concept) with key "([^"]*)" named "([^"]+)" in English and "([^"]+)" in Portuguese$`, w.createsNode)
	sc.Step(`^"([^"]+)" creates a (skill|concept) with key "([^"]*)" named "([^"]+)" in English and "([^"]+)" in Portuguese under (skill|concept) "([^"]+)"$`, w.createsNodeUnder)
	sc.Step(`^"([^"]+)" creates a (skill|concept) with key "([^"]*)" named "([^"]+)" in English and "([^"]+)" in Portuguese for instrument "([^"]+)"$`, w.createsNodeForInstrument)
	sc.Step(`^"([^"]+)" creates a (skill|concept) with key "([^"]*)" named "([^"]+)" in English and "([^"]+)" in Portuguese, described as "([^"]+)" in English and "([^"]+)" in Portuguese$`, w.createsNodeDescribed)
	sc.Step(`^"([^"]+)" creates a (skill|concept) with key "([^"]*)" named "([^"]+)" in English and "([^"]+)" in Portuguese, described only as "([^"]+)" in English$`, w.createsNodeDescribedInEnglishOnly)
	sc.Step(`^"([^"]+)" creates a (skill|concept) with key "([^"]*)" named only "([^"]+)" in English$`, w.createsNodeNamedInEnglishOnly)
	sc.Step(`^"([^"]+)" creates a (skill|concept) with key "([^"]*)" and names "([^"]+)" "([^"]+)", "([^"]+)" "([^"]+)" and "([^"]+)" "([^"]+)"$`, w.createsNodeWithThreeNames)
	sc.Step(`^"([^"]+)" attempts to create a knowledge node$`, w.attemptsCreateNode)
	sc.Step(`^"([^"]+)" submits a create knowledge node request with an instrument id that does not exist$`, w.submitsNodeWithUnknownInstrument)
	sc.Step(`^"([^"]+)" submits a create knowledge node request with a parent_id that does not exist$`, w.submitsNodeWithUnknownParent)
	sc.Step(`^"([^"]+)" submits a create knowledge node request with the kind field omitted$`, w.submitsNodeWithoutKind)

	sc.Step(`^"([^"]+)" lists all knowledge nodes$`, w.listsAllNodes)
	sc.Step(`^an unauthenticated request attempts to list all knowledge nodes$`, w.unauthListsNodes)
	sc.Step(`^"([^"]+)" lists the knowledge nodes of kind "([^"]+)"$`, w.listsNodesOfKind)
	sc.Step(`^"([^"]+)" lists the knowledge nodes for instrument "([^"]+)"$`, w.listsNodesForInstrument)
	sc.Step(`^"([^"]+)" lists the knowledge nodes for instruments "([^"]+)" and "([^"]+)"$`, w.listsNodesForInstruments)
	sc.Step(`^"([^"]+)" retrieves (skill|concept) "([^"]+)"$`, w.retrievesNode)
	sc.Step(`^"([^"]+)" attempts to retrieve a knowledge node with an ID that does not exist$`, w.retrievesUnknownNode)

	sc.Step(`^"([^"]+)" renames (skill|concept) "([^"]+)" to "([^"]+)" in English and "([^"]+)" in Portuguese$`, w.renamesNode)
	sc.Step(`^"([^"]+)" moves (skill|concept) "([^"]+)" under (skill|concept) "([^"]+)"$`, w.movesNodeUnder)
	sc.Step(`^"([^"]+)" moves (skill|concept) "([^"]+)" to the root$`, w.movesNodeToRoot)
	sc.Step(`^"([^"]+)" makes (skill|concept) "([^"]+)" for every instrument$`, w.makesNodeForEveryInstrument)
	sc.Step(`^"([^"]+)" makes (skill|concept) "([^"]+)" for instrument "([^"]+)" only$`, w.makesNodeForInstrumentOnly)
	sc.Step(`^"([^"]+)" removes the description of (skill|concept) "([^"]+)"$`, w.removesNodeDescription)
	sc.Step(`^"([^"]+)" deletes (skill|concept) "([^"]+)"$`, w.deletesNode)

	sc.Step(`^the knowledge node is created and assigned a stable identifier$`, w.nodeCreated)
	sc.Step(`^the knowledge node's kind is "([^"]+)"$`, w.nodeKindIs)
	sc.Step(`^the knowledge node's key is "([^"]+)"$`, w.nodeKeyIs)
	sc.Step(`^the knowledge node's name in "([^"]+)" is "([^"]+)"$`, w.nodeNameIs)
	sc.Step(`^the knowledge node's languages are "([^"]+)"$`, w.nodeLanguagesAre)
	sc.Step(`^the knowledge node's description in "([^"]+)" is "([^"]+)"$`, w.nodeDescriptionIs)
	sc.Step(`^the knowledge node has no description$`, w.nodeHasNoDescription)
	sc.Step(`^the knowledge node has no parent$`, w.nodeHasNoParent)
	sc.Step(`^the knowledge node's parent is "([^"]+)"$`, w.nodeParentIs)
	sc.Step(`^the knowledge node is for every instrument$`, w.nodeIsForEveryInstrument)
	sc.Step(`^the knowledge node's instruments are "([^"]+)"$`, w.nodeInstrumentsAre)
	sc.Step(`^the knowledge node is deleted$`, w.nodeDeleted)
	sc.Step(`^retrieving (skill|concept) "([^"]+)" returns not found$`, w.retrievingNodeReturnsNotFound)
	sc.Step(`^(skill|concept) "([^"]+)" still has parent "([^"]+)"$`, w.nodeStillHasParent)
	sc.Step(`^the response includes (skill|concept) "([^"]+)" with no parent$`, w.responseIncludesRootNode)
	sc.Step(`^the response includes (skill|concept) "([^"]+)" with parent "([^"]+)"$`, w.responseIncludesNodeWithParent)
	sc.Step(`^the response does not include (skill|concept) "([^"]+)"$`, w.responseExcludesNode)
}

// nodeIDFor resolves a scenario's "skill X"/"concept X" to its id.
func (w *world) nodeIDFor(kind, name string) uuid.UUID {
	if kind == string(domain.KnowledgeNodeKindConcept) {
		return w.conceptIDFor(name)
	}
	return w.skillIDFor(name)
}

func (w *world) aRootNodeExists(kind, name string) error {
	w.nodeIDFor(kind, name)
	return nil
}

func (w *world) aNodeExistsUnder(kind, name, parentName string) error {
	parentID := w.nodeIDFor(kind, parentName)
	if kind == string(domain.KnowledgeNodeKindConcept) {
		w.putConcept(name, &parentID)
	} else {
		w.putSkill(name, &parentID)
	}
	return nil
}

func (w *world) aSkillForInstrumentExistsUnder(name, instrument, parentName string) error {
	parentID := w.skillIDFor(parentName)
	w.putSkill(name, &parentID, instrumentID(instrument).String())
	return nil
}

func (w *world) aRootConceptWithDescriptionExists(name string) error {
	id := w.putConcept(name, nil)
	node, err := w.knowledge.GetByID(w.ctx(), id.String())
	if err != nil {
		return err
	}
	node.Descriptions = domain.LocalizedText{"en": "About " + name, "pt_BR": "Sobre " + name}
	w.knowledge.put(node)
	return nil
}

func (w *world) aRootSkillForInstrumentExists(name, instrument string) error {
	w.putSkill(name, nil, instrumentID(instrument).String())
	return nil
}

func (w *world) aRootSkillForInstrumentsExists(name, first, second string) error {
	w.putSkill(name, nil, instrumentID(first).String(), instrumentID(second).String())
	return nil
}

func (w *world) createNode(body generated.CreateKnowledgeNodeRequest) error {
	resp, err := w.handler.CreateKnowledgeNode(w.ctx(), generated.CreateKnowledgeNodeRequestObject{Body: &body})
	w.lastResp, w.lastErr = resp, err
	if created, ok := resp.(generated.CreateKnowledgeNode201JSONResponse); ok {
		if body.Kind == generated.Concept {
			w.conceptIDByName[body.Key] = created.NodeId
		} else {
			w.skillIDByName[body.Key] = created.NodeId
		}
	}
	return err
}

func (w *world) createsNode(_, kind, key, en, pt string) error {
	return w.createNode(generated.CreateKnowledgeNodeRequest{Kind: generated.KnowledgeNodeKind(kind), Key: key, Names: bilingual(en, pt)})
}

func (w *world) createsNodeUnder(_, kind, key, en, pt, parentKind, parentName string) error {
	parentID := w.nodeIDFor(parentKind, parentName)
	return w.createNode(generated.CreateKnowledgeNodeRequest{Kind: generated.KnowledgeNodeKind(kind), Key: key, Names: bilingual(en, pt), ParentId: &parentID})
}

func (w *world) createsNodeForInstrument(_, kind, key, en, pt, instrument string) error {
	ids := generated.InstrumentIds{instrumentID(instrument)}
	return w.createNode(generated.CreateKnowledgeNodeRequest{Kind: generated.KnowledgeNodeKind(kind), Key: key, Names: bilingual(en, pt), InstrumentIds: &ids})
}

func (w *world) createsNodeDescribed(_, kind, key, en, pt, descEN, descPT string) error {
	descriptions := generated.LocalizedDescription{"en": descEN, "pt_BR": descPT}
	return w.createNode(generated.CreateKnowledgeNodeRequest{Kind: generated.KnowledgeNodeKind(kind), Key: key, Names: bilingual(en, pt), Descriptions: &descriptions})
}

func (w *world) createsNodeDescribedInEnglishOnly(_, kind, key, en, pt, descEN string) error {
	descriptions := generated.LocalizedDescription{"en": descEN}
	return w.createNode(generated.CreateKnowledgeNodeRequest{Kind: generated.KnowledgeNodeKind(kind), Key: key, Names: bilingual(en, pt), Descriptions: &descriptions})
}

func (w *world) createsNodeNamedInEnglishOnly(_, kind, key, en string) error {
	return w.createNode(generated.CreateKnowledgeNodeRequest{Kind: generated.KnowledgeNodeKind(kind), Key: key, Names: generated.LocalizedNames{"en": en}})
}

func (w *world) createsNodeWithThreeNames(_, kind, key, c1, v1, c2, v2, c3, v3 string) error {
	return w.createNode(generated.CreateKnowledgeNodeRequest{Kind: generated.KnowledgeNodeKind(kind), Key: key, Names: generated.LocalizedNames{c1: v1, c2: v2, c3: v3}})
}

func (w *world) attemptsCreateNode(string) error {
	return w.createNode(generated.CreateKnowledgeNodeRequest{Kind: generated.Skill, Key: "attempted-node", Names: bilingual("Attempted", "Tentativa")})
}

func (w *world) submitsNodeWithUnknownInstrument(string) error {
	ids := generated.InstrumentIds{deterministicUUID("instrument", "does-not-exist")}
	return w.createNode(generated.CreateKnowledgeNodeRequest{Kind: generated.Skill, Key: "orphan", Names: bilingual("Orphan", "Órfão"), InstrumentIds: &ids})
}

func (w *world) submitsNodeWithUnknownParent(string) error {
	missing := deterministicUUID("knowledge-node", "does-not-exist")
	return w.createNode(generated.CreateKnowledgeNodeRequest{Kind: generated.Skill, Key: "orphan", Names: bilingual("Orphan", "Órfão"), ParentId: &missing})
}

func (w *world) submitsNodeWithoutKind(string) error {
	return w.createNode(generated.CreateKnowledgeNodeRequest{Key: "orphan", Names: bilingual("Orphan", "Órfão")})
}

func (w *world) listNodes(params generated.ListKnowledgeNodesParams) error {
	resp, err := w.handler.ListKnowledgeNodes(w.ctx(), generated.ListKnowledgeNodesRequestObject{Params: params})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) listsAllNodes(string) error {
	return w.listNodes(generated.ListKnowledgeNodesParams{})
}

func (w *world) unauthListsNodes() error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.listNodes(generated.ListKnowledgeNodesParams{})
}

func (w *world) listsNodesOfKind(_, kind string) error {
	k := generated.KnowledgeNodeKind(kind)
	return w.listNodes(generated.ListKnowledgeNodesParams{Kind: &k})
}

func (w *world) listsNodesForInstrument(_, instrument string) error {
	ids := []openapi_types.UUID{instrumentID(instrument)}
	return w.listNodes(generated.ListKnowledgeNodesParams{InstrumentId: &ids})
}

func (w *world) listsNodesForInstruments(_, first, second string) error {
	ids := []openapi_types.UUID{instrumentID(first), instrumentID(second)}
	return w.listNodes(generated.ListKnowledgeNodesParams{InstrumentId: &ids})
}

func (w *world) getNode(id uuid.UUID) (generated.GetKnowledgeNodeResponseObject, error) {
	return w.handler.GetKnowledgeNode(w.ctx(), generated.GetKnowledgeNodeRequestObject{NodeId: id})
}

func (w *world) retrievesNode(_, kind, name string) error {
	resp, err := w.getNode(w.nodeIDFor(kind, name))
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) retrievesUnknownNode(string) error {
	resp, err := w.getNode(deterministicUUID("knowledge-node", "does-not-exist"))
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) updateNode(id uuid.UUID, body generated.UpdateKnowledgeNodeRequest) error {
	resp, err := w.handler.UpdateKnowledgeNode(w.ctx(), generated.UpdateKnowledgeNodeRequestObject{NodeId: id, Body: &body})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) renamesNode(_, kind, name, en, pt string) error {
	names := generated.LocalizedNames(bilingual(en, pt))
	return w.updateNode(w.nodeIDFor(kind, name), generated.UpdateKnowledgeNodeRequest{Names: &names})
}

func (w *world) movesNodeUnder(_, kind, name, parentKind, parentName string) error {
	return w.updateNode(w.nodeIDFor(kind, name), generated.UpdateKnowledgeNodeRequest{ParentId: nullable.NewNullableWithValue(w.nodeIDFor(parentKind, parentName))})
}

func (w *world) movesNodeToRoot(_, kind, name string) error {
	return w.updateNode(w.nodeIDFor(kind, name), generated.UpdateKnowledgeNodeRequest{ParentId: nullable.NewNullNullable[openapi_types.UUID]()})
}

func (w *world) makesNodeForEveryInstrument(_, kind, name string) error {
	every := generated.InstrumentIds{}
	return w.updateNode(w.nodeIDFor(kind, name), generated.UpdateKnowledgeNodeRequest{InstrumentIds: &every})
}

func (w *world) makesNodeForInstrumentOnly(_, kind, name, instrument string) error {
	ids := generated.InstrumentIds{instrumentID(instrument)}
	return w.updateNode(w.nodeIDFor(kind, name), generated.UpdateKnowledgeNodeRequest{InstrumentIds: &ids})
}

func (w *world) removesNodeDescription(_, kind, name string) error {
	return w.updateNode(w.nodeIDFor(kind, name), generated.UpdateKnowledgeNodeRequest{Descriptions: nullable.NewNullNullable[generated.LocalizedDescription]()})
}

func (w *world) deletesNode(_, kind, name string) error {
	resp, err := w.handler.DeleteKnowledgeNode(w.ctx(), generated.DeleteKnowledgeNodeRequestObject{NodeId: w.nodeIDFor(kind, name)})
	w.lastResp, w.lastErr = resp, err
	return err
}

// lastNode is the knowledge node the last create, get or update returned.
func (w *world) lastNode() (generated.KnowledgeNode, error) {
	switch resp := w.lastResp.(type) {
	case generated.CreateKnowledgeNode201JSONResponse:
		return generated.KnowledgeNode(resp), nil
	case generated.GetKnowledgeNode200JSONResponse:
		return generated.KnowledgeNode(resp), nil
	case generated.UpdateKnowledgeNode200JSONResponse:
		return generated.KnowledgeNode(resp), nil
	default:
		return generated.KnowledgeNode{}, fmt.Errorf("expected a knowledge node response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
}

func (w *world) nodeCreated() error {
	if _, ok := w.lastResp.(generated.CreateKnowledgeNode201JSONResponse); !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}

func (w *world) assertNode(check func(generated.KnowledgeNode) error) error {
	node, err := w.lastNode()
	if err != nil {
		return err
	}
	return check(node)
}

func (w *world) nodeKindIs(kind string) error {
	return w.assertNode(func(n generated.KnowledgeNode) error {
		if string(n.Kind) != kind {
			return fmt.Errorf("expected kind %q, got %q", kind, n.Kind)
		}
		return nil
	})
}

func (w *world) nodeKeyIs(key string) error {
	return w.assertNode(func(n generated.KnowledgeNode) error {
		if n.Key != key {
			return fmt.Errorf("expected key %q, got %q", key, n.Key)
		}
		return nil
	})
}

func (w *world) nodeNameIs(code, name string) error {
	return w.assertNode(func(n generated.KnowledgeNode) error {
		if n.Names[code] != name {
			return fmt.Errorf("expected the name in %q to be %q, got %+v", code, name, n.Names)
		}
		return nil
	})
}

func (w *world) nodeLanguagesAre(list string) error {
	return w.assertNode(func(n generated.KnowledgeNode) error {
		if want := splitCommaList(list); !slices.Equal(n.Languages, want) {
			return fmt.Errorf("expected languages %v, got %v", want, n.Languages)
		}
		return nil
	})
}

func (w *world) nodeDescriptionIs(code, description string) error {
	return w.assertNode(func(n generated.KnowledgeNode) error {
		if n.Descriptions == nil || (*n.Descriptions)[code] != description {
			return fmt.Errorf("expected the description in %q to be %q, got %+v", code, description, n.Descriptions)
		}
		return nil
	})
}

func (w *world) nodeHasNoDescription() error {
	return w.assertNode(func(n generated.KnowledgeNode) error {
		if n.Descriptions != nil {
			return fmt.Errorf("expected no description, got %+v", *n.Descriptions)
		}
		return nil
	})
}

func (w *world) nodeHasNoParent() error {
	return w.assertNode(func(n generated.KnowledgeNode) error {
		if n.ParentId != nil {
			return fmt.Errorf("expected no parent, got %s", *n.ParentId)
		}
		return nil
	})
}

func (w *world) nodeParentIs(parentName string) error {
	return w.assertNode(func(n generated.KnowledgeNode) error {
		want := w.nodeIDFor(string(n.Kind), parentName)
		if n.ParentId == nil || *n.ParentId != want {
			return fmt.Errorf("expected parent %s, got %v", want, n.ParentId)
		}
		return nil
	})
}

func (w *world) nodeIsForEveryInstrument() error {
	return w.assertNode(func(n generated.KnowledgeNode) error {
		if len(n.InstrumentIds) != 0 {
			return fmt.Errorf("expected no instruments, got %v", n.InstrumentIds)
		}
		return nil
	})
}

func (w *world) nodeInstrumentsAre(list string) error {
	return w.assertNode(func(n generated.KnowledgeNode) error {
		want := instrumentIDsNamed(splitCommaList(list))
		if !slices.Equal(n.InstrumentIds, want) {
			return fmt.Errorf("expected instruments %v, got %v", want, n.InstrumentIds)
		}
		return nil
	})
}

func (w *world) nodeDeleted() error {
	if _, ok := w.lastResp.(generated.DeleteKnowledgeNode204Response); !ok {
		return fmt.Errorf("expected a 204 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}

func (w *world) retrievingNodeReturnsNotFound(kind, name string) error {
	resp, err := w.getNode(w.nodeIDFor(kind, name))
	if err != nil {
		return err
	}
	if _, ok := resp.(generated.GetKnowledgeNode404JSONResponse); !ok {
		return fmt.Errorf("expected a 404 response, got %#v", resp)
	}
	return nil
}

func (w *world) nodeStillHasParent(kind, name, parentName string) error {
	resp, err := w.getNode(w.nodeIDFor(kind, name))
	if err != nil {
		return err
	}
	node, ok := resp.(generated.GetKnowledgeNode200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v", resp)
	}
	want := w.nodeIDFor(kind, parentName)
	if node.ParentId == nil || *node.ParentId != want {
		return fmt.Errorf("expected parent %s, got %v", want, node.ParentId)
	}
	return nil
}

// listedNode finds the node a scenario calls kind "name" in the last list
// response.
func (w *world) listedNode(kind, name string) (generated.KnowledgeNode, bool, error) {
	resp, ok := w.lastResp.(generated.ListKnowledgeNodes200JSONResponse)
	if !ok {
		return generated.KnowledgeNode{}, false, fmt.Errorf("expected a knowledge node list, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	want := w.nodeIDFor(kind, name)
	for _, n := range resp {
		if n.NodeId == want {
			return n, true, nil
		}
	}
	return generated.KnowledgeNode{}, false, nil
}

func (w *world) responseIncludesRootNode(kind, name string) error {
	node, found, err := w.listedNode(kind, name)
	if err != nil {
		return err
	}
	if !found || string(node.Kind) != kind || node.ParentId != nil {
		return fmt.Errorf("expected root %s %q in the list, got %+v (found=%v)", kind, name, node, found)
	}
	return nil
}

func (w *world) responseIncludesNodeWithParent(kind, name, parentName string) error {
	node, found, err := w.listedNode(kind, name)
	if err != nil {
		return err
	}
	want := w.nodeIDFor(kind, parentName)
	if !found || node.ParentId == nil || *node.ParentId != want {
		return fmt.Errorf("expected %s %q under %s in the list, got %+v (found=%v)", kind, name, want, node, found)
	}
	return nil
}

func (w *world) responseExcludesNode(kind, name string) error {
	_, found, err := w.listedNode(kind, name)
	if err != nil {
		return err
	}
	if found {
		return fmt.Errorf("expected %s %q not to be in the list", kind, name)
	}
	return nil
}
