package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestDiagramListFilter_Matches(t *testing.T) {
	rootA := "A"
	basic := domain.Diagram{ID: "b", Names: domain.LocalizedText{"en": "Scale", "pt_BR": "Escala Jônica"}, InstrumentID: "guitar", Kind: domain.DiagramKindBasic, CreatedBy: "admin", RootNote: &rootA, Skills: []domain.KnowledgeNode{{ID: "s1"}}, Concepts: []domain.KnowledgeNode{{ID: "c1"}}}
	mine := domain.Diagram{ID: "m", Names: domain.LocalizedText{"en": "Box shape"}, InstrumentID: "guitar", Kind: domain.DiagramKindCustom, CreatedBy: "me", Skills: []domain.KnowledgeNode{{ID: "s2"}}, Concepts: []domain.KnowledgeNode{{ID: "c2"}}}
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
		{"Language keeps diagrams named in that language", domain.DiagramListFilter{Language: "pt_BR"}, map[string]bool{"b": true, "m": false, "t": false}},
		{"every set field must match", domain.DiagramListFilter{VisibleTo: "me", Kind: domain.DiagramKindCustom}, map[string]bool{"b": false, "m": true, "t": false}},
		{"CreatedBy narrows within VisibleTo, never widens it", domain.DiagramListFilter{VisibleTo: "me", CreatedBy: "them"}, map[string]bool{"b": false, "m": false, "t": false}},
		{"RootNote matches the recorded root exactly, never an unrecorded one", domain.DiagramListFilter{RootNote: "A"}, map[string]bool{"b": true, "m": false, "t": false}},
		{"Name matches part of a name in any language, ignoring case and accents", domain.DiagramListFilter{Name: "JONICA"}, map[string]bool{"b": true, "m": false, "t": false}},
		{"Name matches the English name too", domain.DiagramListFilter{Name: "box"}, map[string]bool{"b": false, "m": true, "t": false}},
		{"Name never matches a diagram without names", domain.DiagramListFilter{Name: "x"}, map[string]bool{"b": false, "m": true, "t": false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, d := range []domain.Diagram{basic, mine, theirs} {
				assert.Equal(t, tt.want[d.ID], tt.filter.Matches(d), "diagram %s", d.ID)
			}
		})
	}
}

func TestDiagramListFilter_MatchesPurpose(t *testing.T) {
	general := domain.Diagram{ID: "g", Kind: domain.DiagramKindBasic, Purpose: domain.DiagramPurposeGeneral}
	voicing := domain.Diagram{ID: "v", Kind: domain.DiagramKindBasic, Purpose: domain.DiagramPurposeChordVoicing}

	tests := []struct {
		name   string
		filter domain.DiagramPurposeFilter
		want   map[string]bool
	}{
		{"no purpose filter matches every purpose", "", map[string]bool{"g": true, "v": true}},
		{"any matches every purpose", domain.DiagramPurposeFilterAny, map[string]bool{"g": true, "v": true}},
		{"general keeps only general diagrams", domain.DiagramPurposeFilterGeneral, map[string]bool{"g": true, "v": false}},
		{"chord_voicing keeps only chord voicing diagrams", domain.DiagramPurposeFilterChordVoicing, map[string]bool{"g": false, "v": true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := domain.DiagramListFilter{Purpose: tt.filter}
			for _, d := range []domain.Diagram{general, voicing} {
				assert.Equal(t, tt.want[d.ID], filter.Matches(d), "diagram %s", d.ID)
			}
		})
	}
}

func TestDiagramPurposeFilter_Valid(t *testing.T) {
	for _, f := range []domain.DiagramPurposeFilter{"", "general", "chord_voicing", "any"} {
		assert.True(t, f.Valid(), "%q", f)
	}
	assert.False(t, domain.DiagramPurposeFilter("scale").Valid())
}
