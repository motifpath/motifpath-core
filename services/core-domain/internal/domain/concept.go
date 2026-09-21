package domain

// Concept is a self-referential tree node naming an intellectual concept a
// ContentNode or Exercise can be classified under. See TaxonomyNode's doc
// comment for the shared shape and sibling-uniqueness rule — Concept and
// Skill are otherwise independent trees.
type Concept TaxonomyNode

// NewConcept validates and constructs a Concept — see newTaxonomyNode.
func NewConcept(id, name string, parentID *string) (Concept, error) {
	n, err := newTaxonomyNode(id, name, parentID)
	return Concept(n), err
}
