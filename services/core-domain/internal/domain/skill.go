package domain

// Skill is a self-referential tree node naming an observable, practicable
// skill a ContentNode or Exercise can be classified under. ParentID nil
// means a root skill; any skill may have children, to any depth. Name is
// unique among siblings sharing the same ParentID — including among other
// roots — not globally; enforcing that uniqueness requires a repository
// round trip, so it is an application-layer concern, not checked here.
type Skill struct {
	ID       string
	Name     string
	ParentID *string
}

// NewSkill validates and constructs a Skill. Whether ParentID references a
// skill that actually exists, and whether Name collides with a sibling's,
// are application-layer concerns — both require a repository round trip
// this constructor can't perform.
func NewSkill(id, name string, parentID *string) (Skill, error) {
	if name == "" {
		return Skill{}, NewValidationError("name", "must not be empty")
	}
	return Skill{ID: id, Name: name, ParentID: parentID}, nil
}
