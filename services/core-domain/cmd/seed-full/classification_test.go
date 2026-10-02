package main

import (
	"testing"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestRootSkillIDPrefersTheExistingRoot(t *testing.T) {
	root, err := domain.NewSkill("root", "Scales", nil)
	if err != nil {
		t.Fatal(err)
	}
	parent := "parent"
	child, err := domain.NewSkill("child", "Scales", &parent)
	if err != nil {
		t.Fatal(err)
	}

	id, found := rootSkillID([]domain.Skill{child, root}, "Scales")
	if !found || id != root.ID {
		t.Fatalf("got (%q, %t), want (%q, true)", id, found, root.ID)
	}
}

func TestRootConceptIDPrefersTheExistingRoot(t *testing.T) {
	root, err := domain.NewConcept("root", "Fretboard patterns", nil)
	if err != nil {
		t.Fatal(err)
	}

	id, found := rootConceptID([]domain.Concept{root}, "Fretboard patterns")
	if !found || id != root.ID {
		t.Fatalf("got (%q, %t), want (%q, true)", id, found, root.ID)
	}
}
