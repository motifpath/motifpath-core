package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

func newKnowledgeNodeService(nodes *fakeKnowledgeNodeRepository) *application.KnowledgeNodeService {
	instruments := seededInstrumentRepository()
	instruments.byID["bass"] = domain.Instrument{ID: "bass", Names: domain.LocalizedText{"en": "bass"}, Family: domain.InstrumentFamilyFretted, DefaultVoiceID: "electric-bass"}
	return application.NewKnowledgeNodeService(nodes, instruments, newFakeLanguageRepository(), idSequence())
}

var bilingualOpenChords = map[string]string{"en": "Play open chords", "pt_BR": "Tocar acordes abertos"}

func strRef(s string) *string { return &s }

func TestKnowledgeNodeService_Create(t *testing.T) {
	ctx := context.Background()
	concept := domain.KnowledgeNode{ID: "chords", Kind: domain.KnowledgeNodeKindConcept, Key: "chords", Names: domain.LocalizedText{"en": "Chords", "pt_BR": "Acordes"}}

	t.Run("an admin creates a skill under a skill, for an instrument", func(t *testing.T) {
		nodes := newFakeKnowledgeNodeRepository()
		parent := domain.KnowledgeNode{ID: "fretting", Kind: domain.KnowledgeNodeKindSkill, Key: "fretting"}
		nodes.put(parent)
		svc := newKnowledgeNodeService(nodes)

		got, err := svc.Create(ctx, adminCaller(), application.CreateKnowledgeNodeInput{
			Kind: domain.KnowledgeNodeKindSkill, Key: "play-open-chords", Names: bilingualOpenChords,
			ParentID: &parent.ID, InstrumentIDs: []string{"guitar"},
		})

		require.NoError(t, err)
		assert.NotEmpty(t, got.ID)
		assert.Equal(t, "play-open-chords", got.Key)
		assert.Equal(t, &parent.ID, got.ParentID)
		assert.Equal(t, []string{"guitar"}, got.InstrumentIDs)
		stored, err := nodes.GetByID(ctx, got.ID)
		require.NoError(t, err)
		assert.Equal(t, got, stored)
	})

	t.Run("a key already in use is a conflict", func(t *testing.T) {
		nodes := newFakeKnowledgeNodeRepository()
		nodes.put(concept)
		svc := newKnowledgeNodeService(nodes)

		_, err := svc.Create(ctx, adminCaller(), application.CreateKnowledgeNodeInput{Kind: domain.KnowledgeNodeKindSkill, Key: "chords", Names: bilingualOpenChords})

		assert.ErrorIs(t, err, domain.ErrConflict)
	})

	for _, caller := range []domain.User{teacherCaller(), studentCaller()} {
		t.Run(string(caller.Role)+" may not create a node", func(t *testing.T) {
			svc := newKnowledgeNodeService(newFakeKnowledgeNodeRepository())

			_, err := svc.Create(ctx, caller, application.CreateKnowledgeNodeInput{Kind: domain.KnowledgeNodeKindSkill, Key: "play-open-chords", Names: bilingualOpenChords})

			assert.ErrorIs(t, err, domain.ErrForbidden)
		})
	}

	rejected := []struct {
		name      string
		input     application.CreateKnowledgeNodeInput
		wantField string
	}{
		{name: "a name in only one language", input: application.CreateKnowledgeNodeInput{Kind: domain.KnowledgeNodeKindSkill, Key: "play-open-chords", Names: map[string]string{"en": "Play open chords"}}, wantField: "names"},
		{name: "a key that is not kebab-case", input: application.CreateKnowledgeNodeInput{Kind: domain.KnowledgeNodeKindSkill, Key: "Play open", Names: bilingualOpenChords}, wantField: "key"},
		{name: "a missing kind", input: application.CreateKnowledgeNodeInput{Key: "play-open-chords", Names: bilingualOpenChords}, wantField: "kind"},
		{name: "a parent that does not exist", input: application.CreateKnowledgeNodeInput{Kind: domain.KnowledgeNodeKindSkill, Key: "play-open-chords", Names: bilingualOpenChords, ParentID: strRef("ghost")}, wantField: "parent_id"},
		{name: "a parent of the other kind", input: application.CreateKnowledgeNodeInput{Kind: domain.KnowledgeNodeKindSkill, Key: "play-open-chords", Names: bilingualOpenChords, ParentID: &concept.ID}, wantField: "parent_id"},
		{name: "an instrument that does not exist", input: application.CreateKnowledgeNodeInput{Kind: domain.KnowledgeNodeKindSkill, Key: "play-open-chords", Names: bilingualOpenChords, InstrumentIDs: []string{"banjo"}}, wantField: "instrument_ids"},
	}
	for _, tt := range rejected {
		t.Run(tt.name+" is rejected without persisting", func(t *testing.T) {
			nodes := newFakeKnowledgeNodeRepository()
			nodes.put(concept)
			svc := newKnowledgeNodeService(nodes)

			_, err := svc.Create(ctx, adminCaller(), tt.input)

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, tt.wantField, valErr.Fields[0].Field)
			all, listErr := nodes.List(ctx, ports.KnowledgeNodeFilter{})
			require.NoError(t, listErr)
			assert.Len(t, all, 1)
		})
	}
}

func TestKnowledgeNodeService_Read(t *testing.T) {
	ctx := context.Background()
	nodes := newFakeKnowledgeNodeRepository()
	nodes.put(domain.KnowledgeNode{ID: "s", Kind: domain.KnowledgeNodeKindSkill, Key: "palm-muting", InstrumentIDs: []string{"guitar"}})
	nodes.put(domain.KnowledgeNode{ID: "c", Kind: domain.KnowledgeNodeKindConcept, Key: "major-scale"})
	svc := newKnowledgeNodeService(nodes)

	t.Run("lists by kind and instrument", func(t *testing.T) {
		skill := domain.KnowledgeNodeKindSkill
		got, err := svc.List(ctx, ports.KnowledgeNodeFilter{Kind: &skill})
		require.NoError(t, err)
		assert.Len(t, got, 1)

		got, err = svc.List(ctx, ports.KnowledgeNodeFilter{InstrumentIDs: []string{"piano"}})
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "major-scale", got[0].Key)
	})

	t.Run("gets one node, or not found", func(t *testing.T) {
		got, err := svc.Get(ctx, "c")
		require.NoError(t, err)
		assert.Equal(t, "major-scale", got.Key)

		_, err = svc.Get(ctx, "ghost")
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestKnowledgeNodeService_Update(t *testing.T) {
	ctx := context.Background()
	fretting := domain.KnowledgeNode{ID: "fretting", Kind: domain.KnowledgeNodeKindSkill, Key: "fretting", Names: domain.LocalizedText{"en": "Fretting", "pt_BR": "Digitação"}}
	chordPlaying := domain.KnowledgeNode{ID: "chord-playing", Kind: domain.KnowledgeNodeKindSkill, Key: "chord-playing", Names: domain.LocalizedText{"en": "Chords", "pt_BR": "Acordes"}}
	barre := domain.KnowledgeNode{ID: "barre", Kind: domain.KnowledgeNodeKindSkill, Key: "barre-chords", Names: domain.LocalizedText{"en": "Barre", "pt_BR": "Pestana"}, ParentID: &fretting.ID, InstrumentIDs: []string{"guitar", "bass"}}
	eShape := domain.KnowledgeNode{ID: "e-shape", Kind: domain.KnowledgeNodeKindSkill, Key: "e-shape-barre", Names: domain.LocalizedText{"en": "E", "pt_BR": "Mi"}, ParentID: &barre.ID, InstrumentIDs: []string{"guitar"}}
	blues := domain.KnowledgeNode{ID: "blues", Kind: domain.KnowledgeNodeKindConcept, Key: "blues-form", Names: domain.LocalizedText{"en": "Blues", "pt_BR": "Blues"}, Descriptions: domain.LocalizedText{"en": "12 bars", "pt_BR": "12 compassos"}}
	setup := func() (*fakeKnowledgeNodeRepository, *application.KnowledgeNodeService) {
		nodes := newFakeKnowledgeNodeRepository()
		for _, n := range []domain.KnowledgeNode{fretting, chordPlaying, barre, eShape, blues} {
			nodes.put(n)
		}
		return nodes, newKnowledgeNodeService(nodes)
	}
	null := application.Nullable[string]{Set: true}
	under := func(id string) application.Nullable[string] {
		return application.Nullable[string]{Set: true, Value: &id}
	}

	t.Run("renames, keeping everything else", func(t *testing.T) {
		_, svc := setup()

		got, err := svc.Update(ctx, adminCaller(), barre.ID, application.UpdateKnowledgeNodeInput{Names: map[string]string{"en": "Barre chords", "pt_BR": "Acordes com pestana"}})

		require.NoError(t, err)
		assert.Equal(t, domain.LocalizedText{"en": "Barre chords", "pt_BR": "Acordes com pestana"}, got.Names)
		assert.Equal(t, barre.Key, got.Key)
		assert.Equal(t, barre.ParentID, got.ParentID)
		assert.Equal(t, barre.InstrumentIDs, got.InstrumentIDs)
	})

	t.Run("removes and replaces descriptions", func(t *testing.T) {
		_, svc := setup()

		got, err := svc.Update(ctx, adminCaller(), blues.ID, application.UpdateKnowledgeNodeInput{Descriptions: application.Nullable[map[string]string]{Set: true}})
		require.NoError(t, err)
		assert.Nil(t, got.Descriptions)

		replaced := map[string]string{"en": "A form", "pt_BR": "Uma forma"}
		got, err = svc.Update(ctx, adminCaller(), blues.ID, application.UpdateKnowledgeNodeInput{Descriptions: application.Nullable[map[string]string]{Set: true, Value: &replaced}})
		require.NoError(t, err)
		assert.Equal(t, domain.LocalizedText(replaced), got.Descriptions)
	})

	t.Run("moves under another parent of the same kind, and to the root", func(t *testing.T) {
		_, svc := setup()

		got, err := svc.Update(ctx, adminCaller(), barre.ID, application.UpdateKnowledgeNodeInput{ParentID: under(chordPlaying.ID)})
		require.NoError(t, err)
		assert.Equal(t, &chordPlaying.ID, got.ParentID)

		got, err = svc.Update(ctx, adminCaller(), barre.ID, application.UpdateKnowledgeNodeInput{ParentID: null})
		require.NoError(t, err)
		assert.Nil(t, got.ParentID)
	})

	t.Run("widens instruments to every instrument", func(t *testing.T) {
		_, svc := setup()
		every := []string{}

		got, err := svc.Update(ctx, adminCaller(), barre.ID, application.UpdateKnowledgeNodeInput{InstrumentIDs: &every})

		require.NoError(t, err)
		assert.Empty(t, got.InstrumentIDs)
	})

	t.Run("narrows instruments when every classified item still suits the node", func(t *testing.T) {
		nodes, svc := setup()
		nodes.classified[barre.ID] = [][]string{{"guitar"}, {"guitar", "piano"}}
		guitarOnly := []string{"guitar"}

		got, err := svc.Update(ctx, adminCaller(), barre.ID, application.UpdateKnowledgeNodeInput{InstrumentIDs: &guitarOnly})

		require.NoError(t, err)
		assert.Equal(t, guitarOnly, got.InstrumentIDs)
	})

	conflicts := []struct {
		name     string
		id       string
		input    application.UpdateKnowledgeNodeInput
		classify [][]string
	}{
		{name: "moving a node under itself", id: fretting.ID, input: application.UpdateKnowledgeNodeInput{ParentID: under(fretting.ID)}},
		{name: "moving a node under a descendant", id: fretting.ID, input: application.UpdateKnowledgeNodeInput{ParentID: under(eShape.ID)}},
		{name: "narrowing instruments while bass content uses the node", id: barre.ID, input: application.UpdateKnowledgeNodeInput{InstrumentIDs: &[]string{"guitar"}}, classify: [][]string{{"bass"}}},
		{name: "narrowing instruments while every-instrument content uses the node", id: blues.ID, input: application.UpdateKnowledgeNodeInput{InstrumentIDs: &[]string{"guitar"}}, classify: [][]string{nil}},
	}
	for _, tt := range conflicts {
		t.Run(tt.name+" is a conflict", func(t *testing.T) {
			nodes, svc := setup()
			nodes.classified[tt.id] = tt.classify
			before, err := nodes.GetByID(ctx, tt.id)
			require.NoError(t, err)

			_, err = svc.Update(ctx, adminCaller(), tt.id, tt.input)

			assert.ErrorIs(t, err, domain.ErrConflict)
			after, err := nodes.GetByID(ctx, tt.id)
			require.NoError(t, err)
			assert.Equal(t, before, after)
		})
	}

	rejected := []struct {
		name      string
		input     application.UpdateKnowledgeNodeInput
		wantField string
	}{
		{name: "names missing a language", input: application.UpdateKnowledgeNodeInput{Names: map[string]string{"en": "Barre"}}, wantField: "names"},
		{name: "descriptions missing a language", input: application.UpdateKnowledgeNodeInput{Descriptions: application.Nullable[map[string]string]{Set: true, Value: &map[string]string{"en": "x"}}}, wantField: "descriptions"},
		{name: "a parent of the other kind", input: application.UpdateKnowledgeNodeInput{ParentID: under(blues.ID)}, wantField: "parent_id"},
		{name: "a parent that does not exist", input: application.UpdateKnowledgeNodeInput{ParentID: under("ghost")}, wantField: "parent_id"},
		{name: "an instrument that does not exist", input: application.UpdateKnowledgeNodeInput{InstrumentIDs: &[]string{"banjo"}}, wantField: "instrument_ids"},
	}
	for _, tt := range rejected {
		t.Run(tt.name+" is rejected", func(t *testing.T) {
			_, svc := setup()

			_, err := svc.Update(ctx, adminCaller(), barre.ID, tt.input)

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, tt.wantField, valErr.Fields[0].Field)
		})
	}

	t.Run("an unknown node is not found", func(t *testing.T) {
		_, svc := setup()

		_, err := svc.Update(ctx, adminCaller(), "ghost", application.UpdateKnowledgeNodeInput{Names: bilingualOpenChords})

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a teacher may not update a node", func(t *testing.T) {
		_, svc := setup()

		_, err := svc.Update(ctx, teacherCaller(), barre.ID, application.UpdateKnowledgeNodeInput{Names: bilingualOpenChords})

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestKnowledgeNodeService_Delete(t *testing.T) {
	ctx := context.Background()
	node := domain.KnowledgeNode{ID: "fretting", Kind: domain.KnowledgeNodeKindSkill, Key: "fretting"}

	t.Run("deletes a node nothing uses", func(t *testing.T) {
		nodes := newFakeKnowledgeNodeRepository()
		nodes.put(node)
		svc := newKnowledgeNodeService(nodes)

		require.NoError(t, svc.Delete(ctx, adminCaller(), node.ID))

		_, err := nodes.GetByID(ctx, node.ID)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	inUse := []struct {
		name    string
		usage   ports.KnowledgeNodeUsage
		message string
	}{
		{name: "children", usage: ports.KnowledgeNodeUsage{Children: 1}, message: "child"},
		{name: "edges", usage: ports.KnowledgeNodeUsage{Edges: 2}, message: "knowledge edge"},
		{name: "content", usage: ports.KnowledgeNodeUsage{ContentNodes: 1}, message: "content"},
		{name: "exercises", usage: ports.KnowledgeNodeUsage{Exercises: 1}, message: "exercise"},
		{name: "diagrams", usage: ports.KnowledgeNodeUsage{Diagrams: 1}, message: "diagram"},
		{name: "challenges", usage: ports.KnowledgeNodeUsage{Challenges: 1}, message: "challenge"},
	}
	for _, tt := range inUse {
		t.Run("a node with "+tt.name+" is a conflict that says so", func(t *testing.T) {
			nodes := newFakeKnowledgeNodeRepository()
			nodes.put(node)
			nodes.usage[node.ID] = tt.usage
			svc := newKnowledgeNodeService(nodes)

			err := svc.Delete(ctx, adminCaller(), node.ID)

			assert.ErrorIs(t, err, domain.ErrConflict)
			assert.Contains(t, err.Error(), tt.message)
			_, getErr := nodes.GetByID(ctx, node.ID)
			assert.NoError(t, getErr)
		})
	}

	t.Run("an unknown node is not found", func(t *testing.T) {
		svc := newKnowledgeNodeService(newFakeKnowledgeNodeRepository())

		assert.ErrorIs(t, svc.Delete(ctx, adminCaller(), "ghost"), domain.ErrNotFound)
	})

	t.Run("a teacher may not delete a node", func(t *testing.T) {
		nodes := newFakeKnowledgeNodeRepository()
		nodes.put(node)
		svc := newKnowledgeNodeService(nodes)

		assert.ErrorIs(t, svc.Delete(ctx, teacherCaller(), node.ID), domain.ErrForbidden)
	})
}

func TestKnowledgeNodeService_ChildNeverWiderThanParent(t *testing.T) {
	ctx := context.Background()
	palmMuting := domain.KnowledgeNode{ID: "palm-muting", Kind: domain.KnowledgeNodeKindSkill, Key: "palm-muting", Names: domain.LocalizedText{"en": "Palm", "pt_BR": "Palm"}, InstrumentIDs: []string{"guitar"}}
	chugs := domain.KnowledgeNode{ID: "chugs", Kind: domain.KnowledgeNodeKindSkill, Key: "palm-mute-chugs", Names: domain.LocalizedText{"en": "Chugs", "pt_BR": "Chugs"}, ParentID: &palmMuting.ID, InstrumentIDs: []string{"guitar"}}
	charts := domain.KnowledgeNode{ID: "charts", Kind: domain.KnowledgeNodeKindSkill, Key: "read-chord-charts", Names: domain.LocalizedText{"en": "Charts", "pt_BR": "Cifras"}}
	hammerOns := domain.KnowledgeNode{ID: "hammer-ons", Kind: domain.KnowledgeNodeKindSkill, Key: "hammer-ons", Names: domain.LocalizedText{"en": "Hammer", "pt_BR": "Hammer"}, InstrumentIDs: []string{"guitar", "bass"}}
	bassGrooves := domain.KnowledgeNode{ID: "bass-grooves", Kind: domain.KnowledgeNodeKindSkill, Key: "bass-hammer-on-grooves", Names: domain.LocalizedText{"en": "Grooves", "pt_BR": "Grooves"}, ParentID: &hammerOns.ID, InstrumentIDs: []string{"bass"}}
	setup := func() (*fakeKnowledgeNodeRepository, *application.KnowledgeNodeService) {
		nodes := newFakeKnowledgeNodeRepository()
		for _, n := range []domain.KnowledgeNode{palmMuting, chugs, charts, hammerOns, bassGrooves} {
			nodes.put(n)
		}
		return nodes, newKnowledgeNodeService(nodes)
	}
	under := func(id string) application.Nullable[string] { return application.Nullable[string]{Set: true, Value: &id} }

	rejected := []struct {
		name      string
		do        func(*application.KnowledgeNodeService) error
		wantField string
	}{
		{name: "creating an every-instrument child under a guitar parent", wantField: "instrument_ids", do: func(svc *application.KnowledgeNodeService) error {
			_, err := svc.Create(ctx, adminCaller(), application.CreateKnowledgeNodeInput{Kind: domain.KnowledgeNodeKindSkill, Key: "chug-riffs", Names: bilingualOpenChords, ParentID: &palmMuting.ID})
			return err
		}},
		{name: "creating a guitar-and-bass child under a guitar parent", wantField: "instrument_ids", do: func(svc *application.KnowledgeNodeService) error {
			_, err := svc.Create(ctx, adminCaller(), application.CreateKnowledgeNodeInput{Kind: domain.KnowledgeNodeKindSkill, Key: "chug-riffs", Names: bilingualOpenChords, ParentID: &palmMuting.ID, InstrumentIDs: []string{"guitar", "bass"}})
			return err
		}},
		{name: "widening a child beyond its parent", wantField: "instrument_ids", do: func(svc *application.KnowledgeNodeService) error {
			_, err := svc.Update(ctx, adminCaller(), chugs.ID, application.UpdateKnowledgeNodeInput{InstrumentIDs: &[]string{}})
			return err
		}},
		{name: "moving an every-instrument node under a guitar parent", wantField: "parent_id", do: func(svc *application.KnowledgeNodeService) error {
			_, err := svc.Update(ctx, adminCaller(), charts.ID, application.UpdateKnowledgeNodeInput{ParentID: under(palmMuting.ID)})
			return err
		}},
		{name: "moving and widening in one request", wantField: "instrument_ids", do: func(svc *application.KnowledgeNodeService) error {
			_, err := svc.Update(ctx, adminCaller(), bassGrooves.ID, application.UpdateKnowledgeNodeInput{ParentID: under(palmMuting.ID), InstrumentIDs: &[]string{"bass"}})
			return err
		}},
	}
	for _, tt := range rejected {
		t.Run(tt.name+" is rejected", func(t *testing.T) {
			_, svc := setup()

			err := tt.do(svc)

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, tt.wantField, valErr.Fields[0].Field)
		})
	}

	t.Run("moving a node and narrowing it to fit its new parent in one request is accepted", func(t *testing.T) {
		_, svc := setup()

		got, err := svc.Update(ctx, adminCaller(), charts.ID, application.UpdateKnowledgeNodeInput{ParentID: under(palmMuting.ID), InstrumentIDs: &[]string{"guitar"}})

		require.NoError(t, err)
		assert.Equal(t, &palmMuting.ID, got.ParentID)
	})

	t.Run("narrowing a node while a child is for an instrument outside the new scope is a conflict", func(t *testing.T) {
		nodes, svc := setup()

		_, err := svc.Update(ctx, adminCaller(), hammerOns.ID, application.UpdateKnowledgeNodeInput{InstrumentIDs: &[]string{"guitar"}})

		assert.ErrorIs(t, err, domain.ErrConflict)
		stored, getErr := nodes.GetByID(ctx, hammerOns.ID)
		require.NoError(t, getErr)
		assert.Equal(t, hammerOns.InstrumentIDs, stored.InstrumentIDs)
	})

	t.Run("narrowing a node its children still fit inside is accepted", func(t *testing.T) {
		_, svc := setup()

		got, err := svc.Update(ctx, adminCaller(), hammerOns.ID, application.UpdateKnowledgeNodeInput{InstrumentIDs: &[]string{"bass"}})

		require.NoError(t, err)
		assert.Equal(t, []string{"bass"}, got.InstrumentIDs)
	})
}

func TestKnowledgeNodeService_ListRejectsAnUnknownKind(t *testing.T) {
	svc := newKnowledgeNodeService(newFakeKnowledgeNodeRepository())
	topic := domain.KnowledgeNodeKind("topic")

	_, err := svc.List(context.Background(), ports.KnowledgeNodeFilter{Kind: &topic})

	var valErr *domain.ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "kind", valErr.Fields[0].Field)
}
