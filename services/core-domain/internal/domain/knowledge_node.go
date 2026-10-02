package domain

import (
	"regexp"
	"slices"
)

// KnowledgeNodeKind says what a KnowledgeNode names: something a student
// can do, or something true or known.
type KnowledgeNodeKind string

const (
	// KnowledgeNodeKindSkill is something a student can do, named as an
	// action ("Play open chords").
	KnowledgeNodeKindSkill KnowledgeNodeKind = "skill"
	// KnowledgeNodeKindConcept is something true or known ("Open chord
	// shapes").
	KnowledgeNodeKindConcept KnowledgeNodeKind = "concept"
)

const (
	knowledgeNodeKeyMaxLength         = 100
	knowledgeNodeNameMaxLength        = 200
	knowledgeNodeDescriptionMaxLength = 1000
)

var knowledgeNodeKeyPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// KnowledgeNode is a skill or concept in the knowledge graph. Each kind
// forms a strict tree through ParentID — at most one parent, of the same
// kind — so a rollup over a subtree never counts a node twice. What a node
// uses or needs across the tree is a KnowledgeEdge. Whether ParentID exists
// and is of the same kind, whether the move would create a cycle, and
// whether Key is unique all need a repository round trip, so they are
// application-layer concerns.
type KnowledgeNode struct {
	ID   string
	Kind KnowledgeNodeKind
	// Key is the node's stable handle for code and seed scripts —
	// lowercase kebab-case, unique across both kinds, never changed after
	// creation. It is not for display.
	Key   string
	Names LocalizedText
	// Descriptions is nil when the node has no description.
	Descriptions LocalizedText
	ParentID     *string
	// InstrumentIDs lists the instruments the node is for; empty means
	// every instrument, including ones added later.
	InstrumentIDs []string
}

// NewKnowledgeNode validates and constructs a KnowledgeNode. names, and
// descriptions when given, must cover exactly languages — every language
// MotifPath offers.
func NewKnowledgeNode(id string, kind KnowledgeNodeKind, key string, names, descriptions map[string]string, parentID *string, instrumentIDs []string, languages []string) (KnowledgeNode, error) {
	if kind != KnowledgeNodeKindSkill && kind != KnowledgeNodeKindConcept {
		return KnowledgeNode{}, NewValidationError("kind", `must be "skill" or "concept"`)
	}
	if len(key) > knowledgeNodeKeyMaxLength || !knowledgeNodeKeyPattern.MatchString(key) {
		return KnowledgeNode{}, NewValidationError("key", "must be lowercase kebab-case, at most 100 characters")
	}
	node := KnowledgeNode{ID: id, Kind: kind, Key: key, ParentID: parentID}
	if err := node.setNames(names, languages); err != nil {
		return KnowledgeNode{}, err
	}
	if err := node.setDescriptions(descriptions, languages); err != nil {
		return KnowledgeNode{}, err
	}
	if err := node.setInstrumentIDs(instrumentIDs); err != nil {
		return KnowledgeNode{}, err
	}
	return node, nil
}

// Rename replaces the node's names — one for every language in languages.
func (n KnowledgeNode) Rename(names map[string]string, languages []string) (KnowledgeNode, error) {
	err := n.setNames(names, languages)
	return n, err
}

// Describe replaces the node's descriptions — one for every language in
// languages — or removes them when descriptions is nil.
func (n KnowledgeNode) Describe(descriptions map[string]string, languages []string) (KnowledgeNode, error) {
	err := n.setDescriptions(descriptions, languages)
	return n, err
}

// ForInstruments replaces the instruments the node is for; empty means
// every instrument.
func (n KnowledgeNode) ForInstruments(instrumentIDs []string) (KnowledgeNode, error) {
	err := n.setInstrumentIDs(instrumentIDs)
	return n, err
}

func (n *KnowledgeNode) setNames(names map[string]string, languages []string) error {
	localized, err := NewLocalizedText("names", names, knowledgeNodeNameMaxLength, languages)
	if err != nil {
		return err
	}
	n.Names = localized
	return nil
}

func (n *KnowledgeNode) setDescriptions(descriptions map[string]string, languages []string) error {
	if descriptions == nil {
		n.Descriptions = nil
		return nil
	}
	localized, err := NewLocalizedText("descriptions", descriptions, knowledgeNodeDescriptionMaxLength, languages)
	if err != nil {
		return err
	}
	n.Descriptions = localized
	return nil
}

func (n *KnowledgeNode) setInstrumentIDs(instrumentIDs []string) error {
	if problem := instrumentIDsProblem(instrumentIDs); problem != "" {
		return NewValidationError("instrument_ids", problem)
	}
	n.InstrumentIDs = instrumentIDs
	return nil
}

// Suits reports whether content for instrumentIDs (empty meaning every
// instrument) may be classified under the node: a node for every
// instrument suits any content; a node for specific instruments suits
// content for at least one of them, never content for every instrument.
func (n KnowledgeNode) Suits(instrumentIDs []string) bool {
	if len(n.InstrumentIDs) == 0 {
		return true
	}
	for _, id := range instrumentIDs {
		if slices.Contains(n.InstrumentIDs, id) {
			return true
		}
	}
	return false
}
