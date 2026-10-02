package domain

// KnowledgeEdgeType says what a KnowledgeEdge records.
type KnowledgeEdgeType string

const (
	// KnowledgeEdgeTypeApplies records that a skill uses a concept — only
	// skill → concept, never with a level. It never implies a requirement.
	KnowledgeEdgeTypeApplies KnowledgeEdgeType = "applies"
	// KnowledgeEdgeTypeRequires records that one node needs another at a
	// mastery level — any kind to any kind. It informs practice and
	// recommendations and never gates content.
	KnowledgeEdgeTypeRequires KnowledgeEdgeType = "requires"
)

// Valid reports whether t is a known edge type.
func (t KnowledgeEdgeType) Valid() bool {
	return t == KnowledgeEdgeTypeApplies || t == KnowledgeEdgeTypeRequires
}

// MasteryLevel is the practice mastery scale, ordered accurate < fluent <
// retained.
type MasteryLevel string

const (
	MasteryLevelAccurate MasteryLevel = "accurate"
	MasteryLevelFluent   MasteryLevel = "fluent"
	MasteryLevelRetained MasteryLevel = "retained"
)

func (l MasteryLevel) valid() bool {
	return l == MasteryLevelAccurate || l == MasteryLevelFluent || l == MasteryLevelRetained
}

// KnowledgeEdge is a typed link between two KnowledgeNodes. That at most
// one edge of each type links the same two nodes in the same direction, and
// that requires edges never form a cycle, need a repository round trip, so
// they are application-layer concerns.
type KnowledgeEdge struct {
	ID     string
	FromID string
	ToID   string
	Type   KnowledgeEdgeType
	// Level is the level a requires edge asks for; nil for applies.
	Level *MasteryLevel
}

// NewKnowledgeEdge validates and constructs a KnowledgeEdge from from to to.
// It takes the nodes rather than their ids because which types are allowed
// depends on their kinds.
func NewKnowledgeEdge(id string, from, to KnowledgeNode, edgeType KnowledgeEdgeType, level *MasteryLevel) (KnowledgeEdge, error) {
	switch edgeType {
	case KnowledgeEdgeTypeApplies:
		if from.Kind != KnowledgeNodeKindSkill || to.Kind != KnowledgeNodeKindConcept {
			return KnowledgeEdge{}, NewValidationError("type", "applies links a skill to a concept")
		}
		if level != nil {
			return KnowledgeEdge{}, NewValidationError("level", "must be absent for applies")
		}
	case KnowledgeEdgeTypeRequires:
		if level == nil {
			return KnowledgeEdge{}, NewValidationError("level", "is required for requires")
		}
		if !level.valid() {
			return KnowledgeEdge{}, NewValidationError("level", `must be "accurate", "fluent" or "retained"`)
		}
	default:
		return KnowledgeEdge{}, NewValidationError("type", `must be "applies" or "requires"`)
	}
	if from.ID == to.ID {
		return KnowledgeEdge{}, NewValidationError("to_id", "must not be the same node as from_id")
	}
	return KnowledgeEdge{ID: id, FromID: from.ID, ToID: to.ID, Type: edgeType, Level: level}, nil
}

// WithLevel returns the edge asking for level instead. Only a requires edge
// has a level.
func (e KnowledgeEdge) WithLevel(level MasteryLevel) (KnowledgeEdge, error) {
	if e.Type != KnowledgeEdgeTypeRequires {
		return KnowledgeEdge{}, NewValidationError("level", "only a requires edge has a level")
	}
	if !level.valid() {
		return KnowledgeEdge{}, NewValidationError("level", `must be "accurate", "fluent" or "retained"`)
	}
	e.Level = &level
	return e, nil
}
