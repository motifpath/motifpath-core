package domain

// TaxonomyNode is the shared shape of Skill and Concept: a self-referential
// tree node with a name, unique among siblings sharing the same ParentID
// (nil meaning a root node) — including among other roots — not globally;
// enforcing that uniqueness requires a repository round trip, so it is an
// application-layer concern, not checked here. Skill and Concept are
// distinct named types over this shape, not aliases of it or each other:
// they classify a ContentNode/Exercise along independent trees, and keeping
// them nominally distinct lets the compiler catch a Skill id passed where a
// Concept id is expected, and vice versa.
type TaxonomyNode struct {
	ID       string
	Name     string
	ParentID *string
}

// newTaxonomyNode validates and constructs a TaxonomyNode. Whether ParentID
// references a node that actually exists, and whether Name collides with a
// sibling's, are application-layer concerns — both require a repository
// round trip this constructor can't perform.
func newTaxonomyNode(id, name string, parentID *string) (TaxonomyNode, error) {
	if name == "" {
		return TaxonomyNode{}, NewValidationError("name", "must not be empty")
	}
	return TaxonomyNode{ID: id, Name: name, ParentID: parentID}, nil
}
