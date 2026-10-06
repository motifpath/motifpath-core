package repo

import (
	"context"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/diagram"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/exercise"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/instrument"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntNodeItemSource lists practice items with their classification via
// ent/Postgres.
type EntNodeItemSource struct {
	client *ent.Client
}

func NewEntNodeItemSource(client *ent.Client) *EntNodeItemSource {
	return &EntNodeItemSource{client: client}
}

// ClassifiedItems returns the play-alongs and exercises that suit
// instrumentID. A play-along is a basic diagram with playback — a teacher's
// custom diagrams are theirs alone, never offered for practice — and it
// suits every instrument it is linked to. A malformed id has no items.
func (s *EntNodeItemSource) ClassifiedItems(ctx context.Context, instrumentID string) ([]domain.ClassifiedItem, error) {
	parsed, err := uuid.Parse(instrumentID)
	if err != nil {
		return nil, nil
	}
	diagrams, err := s.client.Diagram.Query().
		Where(
			diagram.KindEQ(diagram.KindBasic),
			diagram.TempoBpmNotNil(),
			diagram.HasCompatibleInstrumentsWith(instrument.ID(parsed)),
		).
		WithSkills().
		WithConcepts().
		All(ctx)
	if err != nil {
		return nil, err
	}
	exercises, err := s.client.Exercise.Query().
		Where(exercise.Or(
			exercise.HasInstrumentsWith(instrument.ID(parsed)),
			exercise.Not(exercise.HasInstruments()),
		)).
		WithSkills().
		WithConcepts().
		All(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]domain.ClassifiedItem, 0, len(diagrams)+len(exercises))
	for _, row := range diagrams {
		items = append(items, domain.ClassifiedItem{
			ItemKey: domain.PlayAlongItemKey(row.ID.String()),
			NodeIDs: classifiedNodeIDs(row.Edges.Skills, row.Edges.Concepts),
		})
	}
	for _, row := range exercises {
		items = append(items, domain.ClassifiedItem{
			ItemKey: domain.ExerciseItemKey(row.ID.String()),
			NodeIDs: classifiedNodeIDs(row.Edges.Skills, row.Edges.Concepts),
		})
	}
	return items, nil
}

func classifiedNodeIDs(skills, concepts []*ent.KnowledgeNode) []string {
	ids := make([]string, 0, len(skills)+len(concepts))
	for _, rows := range [][]*ent.KnowledgeNode{skills, concepts} {
		for _, row := range rows {
			ids = append(ids, row.ID.String())
		}
	}
	return ids
}
