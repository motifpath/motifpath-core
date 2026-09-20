package domain

// Concept is a self-referential tree node naming an intellectual concept a
// ContentNode or Exercise can be classified under. It forms a tree the same
// way Skill does — see Skill's doc comment for the shared shape and
// sibling-uniqueness rule.
type Concept struct {
	ID       string
	Name     string
	ParentID *string
}

// NewConcept validates and constructs a Concept. Whether ParentID
// references a concept that actually exists, and whether Name collides
// with a sibling's, are application-layer concerns — both require a
// repository round trip this constructor can't perform.
func NewConcept(id, name string, parentID *string) (Concept, error) {
	if name == "" {
		return Concept{}, NewValidationError("name", "must not be empty")
	}
	return Concept{ID: id, Name: name, ParentID: parentID}, nil
}
