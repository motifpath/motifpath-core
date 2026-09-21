package domain

// Skill is a self-referential tree node naming an observable, practicable
// skill a ContentNode or Exercise can be classified under. See
// TaxonomyNode's doc comment for the shared shape and sibling-uniqueness
// rule.
type Skill TaxonomyNode

// NewSkill validates and constructs a Skill — see newTaxonomyNode.
func NewSkill(id, name string, parentID *string) (Skill, error) {
	n, err := newTaxonomyNode(id, name, parentID)
	return Skill(n), err
}
