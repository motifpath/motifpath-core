package domain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

var openChordNames = map[string]string{"en": "Play open chords", "pt_BR": "Tocar acordes abertos"}

func TestNewKnowledgeNode(t *testing.T) {
	t.Run("a root skill named in every language", func(t *testing.T) {
		node, err := domain.NewKnowledgeNode("node-1", domain.KnowledgeNodeKindSkill, "play-open-chords", openChordNames, nil, nil, nil, offeredLanguages)

		require.NoError(t, err)
		assert.Equal(t, "node-1", node.ID)
		assert.Equal(t, domain.KnowledgeNodeKindSkill, node.Kind)
		assert.Equal(t, "play-open-chords", node.Key)
		assert.Equal(t, domain.LocalizedText{"en": "Play open chords", "pt_BR": "Tocar acordes abertos"}, node.Names)
		assert.Nil(t, node.Descriptions)
		assert.Nil(t, node.ParentID)
		assert.Empty(t, node.InstrumentIDs)
	})

	t.Run("a child concept with descriptions and instruments", func(t *testing.T) {
		parentID := "node-0"
		descriptions := map[string]string{"en": " A 12-bar progression ", "pt_BR": "Uma progressão de 12 compassos"}

		node, err := domain.NewKnowledgeNode("node-1", domain.KnowledgeNodeKindConcept, "blues-form", map[string]string{"en": "Blues form", "pt_BR": "Forma do blues"}, descriptions, &parentID, []string{"guitar", "bass"}, offeredLanguages)

		require.NoError(t, err)
		assert.Equal(t, domain.KnowledgeNodeKindConcept, node.Kind)
		assert.Equal(t, domain.LocalizedText{"en": "A 12-bar progression", "pt_BR": "Uma progressão de 12 compassos"}, node.Descriptions)
		require.NotNil(t, node.ParentID)
		assert.Equal(t, "node-0", *node.ParentID)
		assert.Equal(t, []string{"guitar", "bass"}, node.InstrumentIDs)
	})

	tests := []struct {
		name         string
		kind         domain.KnowledgeNodeKind
		key          string
		names        map[string]string
		descriptions map[string]string
		instruments  []string
		wantField    string
	}{
		{name: "unknown kind", kind: "topic", key: "play-open-chords", names: openChordNames, wantField: "kind"},
		{name: "empty kind", kind: "", key: "play-open-chords", names: openChordNames, wantField: "kind"},
		{name: "empty key", kind: domain.KnowledgeNodeKindSkill, key: "", names: openChordNames, wantField: "key"},
		{name: "uppercase key", kind: domain.KnowledgeNodeKindSkill, key: "Play-Open-Chords", names: openChordNames, wantField: "key"},
		{name: "key with spaces", kind: domain.KnowledgeNodeKindSkill, key: "play open chords", names: openChordNames, wantField: "key"},
		{name: "key with a double hyphen", kind: domain.KnowledgeNodeKindSkill, key: "play--open", names: openChordNames, wantField: "key"},
		{name: "key with a leading hyphen", kind: domain.KnowledgeNodeKindSkill, key: "-play", names: openChordNames, wantField: "key"},
		{name: "key with a trailing hyphen", kind: domain.KnowledgeNodeKindSkill, key: "play-", names: openChordNames, wantField: "key"},
		{name: "key longer than 100 characters", kind: domain.KnowledgeNodeKindSkill, key: strings.Repeat("a", 101), names: openChordNames, wantField: "key"},
		{name: "names in only one language", kind: domain.KnowledgeNodeKindSkill, key: "play-open-chords", names: map[string]string{"en": "Play open chords"}, wantField: "names"},
		{name: "names in an unknown language", kind: domain.KnowledgeNodeKindSkill, key: "play-open-chords", names: map[string]string{"en": "a", "pt_BR": "b", "fr": "c"}, wantField: "names"},
		{name: "name longer than 200 characters", kind: domain.KnowledgeNodeKindSkill, key: "play-open-chords", names: map[string]string{"en": strings.Repeat("a", 201), "pt_BR": "b"}, wantField: "names"},
		{name: "descriptions in only one language", kind: domain.KnowledgeNodeKindConcept, key: "blues-form", names: openChordNames, descriptions: map[string]string{"en": "A 12-bar progression"}, wantField: "descriptions"},
		{name: "description longer than 1000 characters", kind: domain.KnowledgeNodeKindConcept, key: "blues-form", names: openChordNames, descriptions: map[string]string{"en": strings.Repeat("a", 1001), "pt_BR": "b"}, wantField: "descriptions"},
		{name: "a repeated instrument", kind: domain.KnowledgeNodeKindSkill, key: "palm-muting", names: openChordNames, instruments: []string{"guitar", "guitar"}, wantField: "instrument_ids"},
	}
	for _, tt := range tests {
		t.Run(tt.name+" is rejected", func(t *testing.T) {
			_, err := domain.NewKnowledgeNode("node-1", tt.kind, tt.key, tt.names, tt.descriptions, nil, tt.instruments, offeredLanguages)

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			require.Len(t, valErr.Fields, 1)
			assert.Equal(t, tt.wantField, valErr.Fields[0].Field)
		})
	}

	t.Run("a 100-character key is accepted", func(t *testing.T) {
		_, err := domain.NewKnowledgeNode("node-1", domain.KnowledgeNodeKindSkill, strings.Repeat("a", 100), openChordNames, nil, nil, nil, offeredLanguages)

		require.NoError(t, err)
	})
}

func TestKnowledgeNodeSuits(t *testing.T) {
	everyInstrument := domain.KnowledgeNode{}
	guitarAndBass := domain.KnowledgeNode{InstrumentIDs: []string{"guitar", "bass"}}

	tests := []struct {
		name        string
		node        domain.KnowledgeNode
		instruments []string
		want        bool
	}{
		{name: "an every-instrument node suits every-instrument content", node: everyInstrument, want: true},
		{name: "an every-instrument node suits guitar content", node: everyInstrument, instruments: []string{"guitar"}, want: true},
		{name: "a guitar-and-bass node suits guitar content", node: guitarAndBass, instruments: []string{"guitar"}, want: true},
		{name: "a guitar-and-bass node suits guitar-and-bass content", node: guitarAndBass, instruments: []string{"bass", "guitar"}, want: true},
		{name: "a guitar-and-bass node does not suit piano content", node: guitarAndBass, instruments: []string{"piano"}, want: false},
		{name: "a guitar-and-bass node suits guitar-and-piano content", node: guitarAndBass, instruments: []string{"guitar", "piano"}, want: true},
		{name: "a guitar-and-bass node does not suit every-instrument content", node: guitarAndBass, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.node.Suits(tt.instruments))
		})
	}
}

func TestKnowledgeNodeWithin(t *testing.T) {
	everyInstrument := domain.KnowledgeNode{}
	guitarAndBass := domain.KnowledgeNode{InstrumentIDs: []string{"guitar", "bass"}}
	guitar := domain.KnowledgeNode{InstrumentIDs: []string{"guitar"}}
	piano := domain.KnowledgeNode{InstrumentIDs: []string{"piano"}}

	tests := []struct {
		name          string
		child, parent domain.KnowledgeNode
		want          bool
	}{
		{name: "any child fits under an every-instrument parent", child: guitar, parent: everyInstrument, want: true},
		{name: "an every-instrument child fits under an every-instrument parent", child: everyInstrument, parent: everyInstrument, want: true},
		{name: "a guitar child fits under a guitar-and-bass parent", child: guitar, parent: guitarAndBass, want: true},
		{name: "an equal scope fits", child: guitarAndBass, parent: guitarAndBass, want: true},
		{name: "an every-instrument child is wider than a guitar parent", child: everyInstrument, parent: guitar, want: false},
		{name: "a guitar-and-bass child is wider than a guitar parent", child: guitarAndBass, parent: guitar, want: false},
		{name: "a piano child is outside a guitar parent", child: piano, parent: guitar, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.child.Within(tt.parent))
		})
	}
}

func TestKnowledgeEnumsValid(t *testing.T) {
	assert.True(t, domain.KnowledgeNodeKindSkill.Valid())
	assert.True(t, domain.KnowledgeNodeKindConcept.Valid())
	assert.False(t, domain.KnowledgeNodeKind("topic").Valid())
	assert.True(t, domain.KnowledgeEdgeTypeApplies.Valid())
	assert.True(t, domain.KnowledgeEdgeTypeRequires.Valid())
	assert.False(t, domain.KnowledgeEdgeType("prerequisite_of").Valid())
}
