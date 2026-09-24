package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestDiagramListFilter_Matches(t *testing.T) {
	basic := domain.Diagram{ID: "b", InstrumentID: "guitar", Kind: domain.DiagramKindBasic, CreatedBy: "admin", Skills: []domain.Skill{{ID: "s1"}}, Concepts: []domain.Concept{{ID: "c1"}}}
	mine := domain.Diagram{ID: "m", InstrumentID: "guitar", Kind: domain.DiagramKindCustom, CreatedBy: "me", Skills: []domain.Skill{{ID: "s2"}}, Concepts: []domain.Concept{{ID: "c2"}}}
	theirs := domain.Diagram{ID: "t", InstrumentID: "piano", Kind: domain.DiagramKindCustom, CreatedBy: "them"}

	tests := []struct {
		name   string
		filter domain.DiagramListFilter
		want   map[string]bool
	}{
		{"the zero filter matches everything", domain.DiagramListFilter{}, map[string]bool{"b": true, "m": true, "t": true}},
		{"VisibleTo keeps basic diagrams and the viewer's own custom ones", domain.DiagramListFilter{VisibleTo: "me"}, map[string]bool{"b": true, "m": true, "t": false}},
		{"Kind narrows to one kind", domain.DiagramListFilter{Kind: domain.DiagramKindCustom}, map[string]bool{"b": false, "m": true, "t": true}},
		{"CreatedBy narrows to one creator", domain.DiagramListFilter{CreatedBy: "them"}, map[string]bool{"b": false, "m": false, "t": true}},
		{"InstrumentID narrows to one instrument", domain.DiagramListFilter{InstrumentID: "piano"}, map[string]bool{"b": false, "m": false, "t": true}},
		{"SkillID matches an exact linked skill", domain.DiagramListFilter{SkillID: "s2"}, map[string]bool{"b": false, "m": true, "t": false}},
		{"ConceptID matches an exact linked concept", domain.DiagramListFilter{ConceptID: "c1"}, map[string]bool{"b": true, "m": false, "t": false}},
		{"every set field must match", domain.DiagramListFilter{VisibleTo: "me", Kind: domain.DiagramKindCustom}, map[string]bool{"b": false, "m": true, "t": false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, d := range []domain.Diagram{basic, mine, theirs} {
				assert.Equal(t, tt.want[d.ID], tt.filter.Matches(d), "diagram %s", d.ID)
			}
		})
	}
}
