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

func levelRef(l domain.MasteryLevel) *domain.MasteryLevel { return &l }

func TestKnowledgeEdgeService(t *testing.T) {
	ctx := context.Background()
	improvise := domain.KnowledgeNode{ID: "improvise", Kind: domain.KnowledgeNodeKindSkill, Key: "improvise-over-a-blues"}
	pentatonic := domain.KnowledgeNode{ID: "pentatonic", Kind: domain.KnowledgeNodeKindSkill, Key: "play-pentatonic-positions"}
	blues := domain.KnowledgeNode{ID: "blues", Kind: domain.KnowledgeNodeKindConcept, Key: "blues-form"}
	scale := domain.KnowledgeNode{ID: "scale", Kind: domain.KnowledgeNodeKindConcept, Key: "minor-pentatonic-scale"}
	setup := func() (*fakeKnowledgeEdgeRepository, *application.KnowledgeEdgeService) {
		nodes := newFakeKnowledgeNodeRepository()
		for _, n := range []domain.KnowledgeNode{improvise, pentatonic, blues, scale} {
			nodes.put(n)
		}
		edges := newFakeKnowledgeEdgeRepository()
		return edges, application.NewKnowledgeEdgeService(edges, nodes, idSequence())
	}
	link := func(from, to domain.KnowledgeNode, edgeType domain.KnowledgeEdgeType, level *domain.MasteryLevel) application.CreateKnowledgeEdgeInput {
		return application.CreateKnowledgeEdgeInput{FromID: from.ID, ToID: to.ID, Type: edgeType, Level: level}
	}

	t.Run("an admin links a skill to a concept with applies", func(t *testing.T) {
		edges, svc := setup()

		got, err := svc.Create(ctx, adminCaller(), link(improvise, blues, domain.KnowledgeEdgeTypeApplies, nil))

		require.NoError(t, err)
		assert.NotEmpty(t, got.ID)
		assert.Nil(t, got.Level)
		stored, err := edges.GetByID(ctx, got.ID)
		require.NoError(t, err)
		assert.Equal(t, got, stored)
	})

	t.Run("a node may both apply and require the same concept", func(t *testing.T) {
		_, svc := setup()
		_, err := svc.Create(ctx, adminCaller(), link(improvise, blues, domain.KnowledgeEdgeTypeApplies, nil))
		require.NoError(t, err)

		_, err = svc.Create(ctx, adminCaller(), link(improvise, blues, domain.KnowledgeEdgeTypeRequires, levelRef(domain.MasteryLevelFluent)))

		assert.NoError(t, err)
	})

	conflicts := []struct {
		name     string
		existing []application.CreateKnowledgeEdgeInput
		create   application.CreateKnowledgeEdgeInput
	}{
		{
			name:     "the same edge twice",
			existing: []application.CreateKnowledgeEdgeInput{link(improvise, blues, domain.KnowledgeEdgeTypeApplies, nil)},
			create:   link(improvise, blues, domain.KnowledgeEdgeTypeApplies, nil),
		},
		{
			name:     "a requires edge closing a direct cycle",
			existing: []application.CreateKnowledgeEdgeInput{link(improvise, pentatonic, domain.KnowledgeEdgeTypeRequires, levelRef(domain.MasteryLevelFluent))},
			create:   link(pentatonic, improvise, domain.KnowledgeEdgeTypeRequires, levelRef(domain.MasteryLevelAccurate)),
		},
		{
			name: "a requires edge closing a longer cycle across kinds",
			existing: []application.CreateKnowledgeEdgeInput{
				link(improvise, pentatonic, domain.KnowledgeEdgeTypeRequires, levelRef(domain.MasteryLevelFluent)),
				link(pentatonic, scale, domain.KnowledgeEdgeTypeRequires, levelRef(domain.MasteryLevelAccurate)),
			},
			create: link(scale, improvise, domain.KnowledgeEdgeTypeRequires, levelRef(domain.MasteryLevelAccurate)),
		},
	}
	for _, tt := range conflicts {
		t.Run(tt.name+" is a conflict", func(t *testing.T) {
			edges, svc := setup()
			for _, e := range tt.existing {
				_, err := svc.Create(ctx, adminCaller(), e)
				require.NoError(t, err)
			}

			_, err := svc.Create(ctx, adminCaller(), tt.create)

			assert.ErrorIs(t, err, domain.ErrConflict)
			all, listErr := edges.List(ctx, ports.KnowledgeEdgeFilter{})
			require.NoError(t, listErr)
			assert.Len(t, all, len(tt.existing))
		})
	}

	rejected := []struct {
		name      string
		input     application.CreateKnowledgeEdgeInput
		wantField string
	}{
		{name: "applies from a concept to a skill", input: link(blues, improvise, domain.KnowledgeEdgeTypeApplies, nil), wantField: "type"},
		{name: "requires without a level", input: link(improvise, pentatonic, domain.KnowledgeEdgeTypeRequires, nil), wantField: "level"},
		{name: "a node linked to itself", input: link(improvise, improvise, domain.KnowledgeEdgeTypeRequires, levelRef(domain.MasteryLevelAccurate)), wantField: "to_id"},
		{name: "a to node that does not exist", input: application.CreateKnowledgeEdgeInput{FromID: improvise.ID, ToID: "ghost", Type: domain.KnowledgeEdgeTypeApplies}, wantField: "to_id"},
		{name: "a from node that does not exist", input: application.CreateKnowledgeEdgeInput{FromID: "ghost", ToID: blues.ID, Type: domain.KnowledgeEdgeTypeApplies}, wantField: "from_id"},
	}
	for _, tt := range rejected {
		t.Run(tt.name+" is rejected", func(t *testing.T) {
			_, svc := setup()

			_, err := svc.Create(ctx, adminCaller(), tt.input)

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, tt.wantField, valErr.Fields[0].Field)
		})
	}

	t.Run("an admin changes a requires edge's level", func(t *testing.T) {
		_, svc := setup()
		edge, err := svc.Create(ctx, adminCaller(), link(improvise, pentatonic, domain.KnowledgeEdgeTypeRequires, levelRef(domain.MasteryLevelAccurate)))
		require.NoError(t, err)

		got, err := svc.UpdateLevel(ctx, adminCaller(), edge.ID, domain.MasteryLevelFluent)

		require.NoError(t, err)
		assert.Equal(t, levelRef(domain.MasteryLevelFluent), got.Level)
	})

	t.Run("giving an applies edge a level is rejected", func(t *testing.T) {
		_, svc := setup()
		edge, err := svc.Create(ctx, adminCaller(), link(improvise, blues, domain.KnowledgeEdgeTypeApplies, nil))
		require.NoError(t, err)

		_, err = svc.UpdateLevel(ctx, adminCaller(), edge.ID, domain.MasteryLevelFluent)

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, "level", valErr.Fields[0].Field)
	})

	t.Run("an admin deletes an edge", func(t *testing.T) {
		edges, svc := setup()
		edge, err := svc.Create(ctx, adminCaller(), link(improvise, blues, domain.KnowledgeEdgeTypeApplies, nil))
		require.NoError(t, err)

		require.NoError(t, svc.Delete(ctx, adminCaller(), edge.ID))

		_, err = edges.GetByID(ctx, edge.ID)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("unknown edges are not found", func(t *testing.T) {
		_, svc := setup()

		_, err := svc.UpdateLevel(ctx, adminCaller(), "ghost", domain.MasteryLevelFluent)
		assert.ErrorIs(t, err, domain.ErrNotFound)
		assert.ErrorIs(t, svc.Delete(ctx, adminCaller(), "ghost"), domain.ErrNotFound)
	})

	t.Run("anyone lists edges by type and endpoint", func(t *testing.T) {
		_, svc := setup()
		_, err := svc.Create(ctx, adminCaller(), link(improvise, pentatonic, domain.KnowledgeEdgeTypeRequires, levelRef(domain.MasteryLevelFluent)))
		require.NoError(t, err)
		_, err = svc.Create(ctx, adminCaller(), link(improvise, blues, domain.KnowledgeEdgeTypeApplies, nil))
		require.NoError(t, err)
		requires := domain.KnowledgeEdgeTypeRequires

		got, err := svc.List(ctx, ports.KnowledgeEdgeFilter{Type: &requires, FromID: &improvise.ID})

		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, pentatonic.ID, got[0].ToID)
	})

	for _, caller := range []domain.User{teacherCaller(), studentCaller()} {
		t.Run(string(caller.Role)+" may not write edges", func(t *testing.T) {
			_, svc := setup()
			edge, err := svc.Create(ctx, adminCaller(), link(improvise, pentatonic, domain.KnowledgeEdgeTypeRequires, levelRef(domain.MasteryLevelAccurate)))
			require.NoError(t, err)

			_, err = svc.Create(ctx, caller, link(improvise, blues, domain.KnowledgeEdgeTypeApplies, nil))
			assert.ErrorIs(t, err, domain.ErrForbidden)
			_, err = svc.UpdateLevel(ctx, caller, edge.ID, domain.MasteryLevelFluent)
			assert.ErrorIs(t, err, domain.ErrForbidden)
			assert.ErrorIs(t, svc.Delete(ctx, caller, edge.ID), domain.ErrForbidden)
		})
	}
}
