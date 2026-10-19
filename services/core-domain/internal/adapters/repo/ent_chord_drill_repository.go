package repo

import (
	"context"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/changetarget"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/chordchangepair"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/chorddrill"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntChordDrillRepository reads the chord_drills, chord_change_pairs and
// change_targets the drill catalog's migrations install.
type EntChordDrillRepository struct {
	client *ent.Client
}

func NewEntChordDrillRepository(client *ent.Client) *EntChordDrillRepository {
	return &EntChordDrillRepository{client: client}
}

func (r *EntChordDrillRepository) Catalog(ctx context.Context) (domain.ChordDrillCatalog, error) {
	drills, err := r.client.ChordDrill.Query().Order(ent.Asc(chorddrill.FieldOrdinal)).All(ctx)
	if err != nil {
		return domain.ChordDrillCatalog{}, err
	}
	pairs, err := r.client.ChordChangePair.Query().Order(ent.Asc(chordchangepair.FieldOrdinal)).All(ctx)
	if err != nil {
		return domain.ChordDrillCatalog{}, err
	}
	targets, err := r.client.ChangeTarget.Query().Order(ent.Asc(changetarget.FieldVersion)).All(ctx)
	if err != nil {
		return domain.ChordDrillCatalog{}, err
	}
	catalog := domain.ChordDrillCatalog{
		Drills:  make([]domain.ChordDrill, len(drills)),
		Changes: make([]domain.ChordChangePair, len(pairs)),
		Targets: make([]domain.ChangeTarget, len(targets)),
	}
	for i, d := range drills {
		catalog.Drills[i] = domain.ChordDrill{ID: d.ID.String(), DiagramID: d.DiagramID.String(), Chord: d.Chord, Ordinal: d.Ordinal}
	}
	for i, p := range pairs {
		catalog.Changes[i] = domain.ChordChangePair{ID: p.ID.String(), FromChordDrillID: p.FromChordDrillID.String(), ToChordDrillID: p.ToChordDrillID.String(), Ordinal: p.Ordinal}
	}
	for i, t := range targets {
		catalog.Targets[i] = domain.ChangeTarget{
			ID: t.ID.String(), Version: t.Version, EffectiveFrom: t.EffectiveFrom.UTC(), ChangesPerMinute: t.ChangesPerMinute,
			Source: domain.DrillThresholdSource(t.Source),
		}
	}
	return catalog, nil
}
