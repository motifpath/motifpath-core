package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// KnowledgeNodeService manages the skills and concepts of the knowledge
// graph. Any authenticated user may read them; only an admin may change
// them, since every author classifies against the same shared graph.
type KnowledgeNodeService struct {
	nodes       ports.KnowledgeNodeRepository
	instruments ports.InstrumentRepository
	languages   ports.LanguageRepository
	newID       func() string
}

func NewKnowledgeNodeService(nodes ports.KnowledgeNodeRepository, instruments ports.InstrumentRepository, languages ports.LanguageRepository, newID func() string) *KnowledgeNodeService {
	return &KnowledgeNodeService{nodes: nodes, instruments: instruments, languages: languages, newID: newID}
}

// CreateKnowledgeNodeInput is a new node. A nil Descriptions means none; a
// nil ParentID makes a root; an empty InstrumentIDs means every instrument.
type CreateKnowledgeNodeInput struct {
	Kind          domain.KnowledgeNodeKind
	Key           string
	Names         map[string]string
	Descriptions  map[string]string
	ParentID      *string
	InstrumentIDs []string
}

// UpdateKnowledgeNodeInput changes a node; a field left at its zero value
// (nil, or Set false) is unchanged.
type UpdateKnowledgeNodeInput struct {
	Names        map[string]string
	Descriptions Nullable[map[string]string]
	ParentID     Nullable[string]
	// InstrumentIDs replaces the instruments when non-nil; empty means
	// every instrument.
	InstrumentIDs *[]string
}

// Create adds a node. A key already in use is a domain.ErrConflict.
func (s *KnowledgeNodeService) Create(ctx context.Context, caller domain.User, input CreateKnowledgeNodeInput) (domain.KnowledgeNode, error) {
	if caller.Role != domain.RoleAdmin {
		return domain.KnowledgeNode{}, domain.ErrForbidden
	}
	offered, err := offeredLanguages(ctx, s.languages)
	if err != nil {
		return domain.KnowledgeNode{}, err
	}
	node, err := domain.NewKnowledgeNode(s.newID(), input.Kind, input.Key, input.Names, input.Descriptions, input.ParentID, input.InstrumentIDs, offered)
	if err != nil {
		return domain.KnowledgeNode{}, err
	}
	if err := s.checkParent(ctx, node.Kind, node.ParentID); err != nil {
		return domain.KnowledgeNode{}, err
	}
	if err := checkInstrumentsExist(ctx, s.instruments, node.InstrumentIDs); err != nil {
		return domain.KnowledgeNode{}, err
	}
	if err := s.nodes.Create(ctx, node); err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			return domain.KnowledgeNode{}, fmt.Errorf("%w: another knowledge node already has the key %q", domain.ErrConflict, node.Key)
		}
		return domain.KnowledgeNode{}, err
	}
	return node, nil
}

// Get returns the node with id, or domain.ErrNotFound.
func (s *KnowledgeNodeService) Get(ctx context.Context, id string) (domain.KnowledgeNode, error) {
	return s.nodes.GetByID(ctx, id)
}

// List returns the nodes matching filter, sorted by key.
func (s *KnowledgeNodeService) List(ctx context.Context, filter ports.KnowledgeNodeFilter) ([]domain.KnowledgeNode, error) {
	return s.nodes.List(ctx, filter)
}

// Update renames, describes, moves or re-scopes a node; kind and key never
// change. Moving a node under itself or a descendant, or narrowing its
// instruments so content or a diagram classified under it no longer suits
// it, is a domain.ErrConflict.
func (s *KnowledgeNodeService) Update(ctx context.Context, caller domain.User, id string, input UpdateKnowledgeNodeInput) (domain.KnowledgeNode, error) {
	if caller.Role != domain.RoleAdmin {
		return domain.KnowledgeNode{}, domain.ErrForbidden
	}
	node, err := s.nodes.GetByID(ctx, id)
	if err != nil {
		return domain.KnowledgeNode{}, err
	}
	offered, err := offeredLanguages(ctx, s.languages)
	if err != nil {
		return domain.KnowledgeNode{}, err
	}
	if input.Names != nil {
		if node, err = node.Rename(input.Names, offered); err != nil {
			return domain.KnowledgeNode{}, err
		}
	}
	if input.Descriptions.Set {
		var descriptions map[string]string
		if input.Descriptions.Value != nil {
			descriptions = *input.Descriptions.Value
		}
		if node, err = node.Describe(descriptions, offered); err != nil {
			return domain.KnowledgeNode{}, err
		}
	}
	if input.ParentID.Set {
		if err := s.checkMove(ctx, node, input.ParentID.Value); err != nil {
			return domain.KnowledgeNode{}, err
		}
		node.ParentID = input.ParentID.Value
	}
	if input.InstrumentIDs != nil {
		if node, err = s.rescope(ctx, node, *input.InstrumentIDs); err != nil {
			return domain.KnowledgeNode{}, err
		}
	}
	if err := s.nodes.Update(ctx, node); err != nil {
		return domain.KnowledgeNode{}, err
	}
	return node, nil
}

// checkParent returns a validation error on "parent_id" unless parentID is
// nil or an existing node of kind.
func (s *KnowledgeNodeService) checkParent(ctx context.Context, kind domain.KnowledgeNodeKind, parentID *string) error {
	if parentID == nil {
		return nil
	}
	parent, err := s.nodes.GetByID(ctx, *parentID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.NewValidationError("parent_id", "does not reference an existing knowledge node")
	}
	if err != nil {
		return err
	}
	if parent.Kind != kind {
		return domain.NewValidationError("parent_id", fmt.Sprintf("must be a %s, like the node itself", kind))
	}
	return nil
}

// checkMove checks that node may move under parentID (nil for the root).
func (s *KnowledgeNodeService) checkMove(ctx context.Context, node domain.KnowledgeNode, parentID *string) error {
	if err := s.checkParent(ctx, node.Kind, parentID); err != nil {
		return err
	}
	if parentID == nil {
		return nil
	}
	cycle, err := s.nodes.InSubtree(ctx, node.ID, *parentID)
	if err != nil {
		return err
	}
	if cycle {
		return fmt.Errorf("%w: a node cannot move under itself or one of its descendants", domain.ErrConflict)
	}
	return nil
}

// rescope returns node for instrumentIDs, refusing a scope that content or a
// diagram classified under the node would no longer suit.
func (s *KnowledgeNodeService) rescope(ctx context.Context, node domain.KnowledgeNode, instrumentIDs []string) (domain.KnowledgeNode, error) {
	node, err := node.ForInstruments(instrumentIDs)
	if err != nil {
		return domain.KnowledgeNode{}, err
	}
	if err := checkInstrumentsExist(ctx, s.instruments, instrumentIDs); err != nil {
		return domain.KnowledgeNode{}, err
	}
	sets, err := s.nodes.ClassifiedInstrumentSets(ctx, node.ID)
	if err != nil {
		return domain.KnowledgeNode{}, err
	}
	for _, set := range sets {
		if !node.Suits(set) {
			return domain.KnowledgeNode{}, fmt.Errorf("%w: content or a diagram classified under this node is for none of the new instruments", domain.ErrConflict)
		}
	}
	return node, nil
}

// Delete removes a node nothing refers to; a node still in use is a
// domain.ErrConflict whose message says what uses it.
func (s *KnowledgeNodeService) Delete(ctx context.Context, caller domain.User, id string) error {
	if caller.Role != domain.RoleAdmin {
		return domain.ErrForbidden
	}
	if _, err := s.nodes.GetByID(ctx, id); err != nil {
		return err
	}
	usage, err := s.nodes.Usage(ctx, id)
	if err != nil {
		return err
	}
	var uses []string
	for _, u := range []struct {
		count int
		noun  string
	}{
		{usage.Children, "child node"},
		{usage.Edges, "knowledge edge"},
		{usage.ContentNodes, "content node"},
		{usage.Exercises, "exercise"},
		{usage.Diagrams, "diagram"},
		{usage.Challenges, "challenge"},
	} {
		if u.count > 0 {
			uses = append(uses, fmt.Sprintf("%d %s(s)", u.count, u.noun))
		}
	}
	if len(uses) > 0 {
		return fmt.Errorf("%w: the node is still used by %s", domain.ErrConflict, strings.Join(uses, ", "))
	}
	return s.nodes.Delete(ctx, id)
}
