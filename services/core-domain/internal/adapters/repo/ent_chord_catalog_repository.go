package repo

import (
	"context"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/chorddefinition"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/chordvoicing"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntChordCatalogRepository reads the chord_definitions and chord_voicings
// the chord catalog's migrations install.
type EntChordCatalogRepository struct {
	client *ent.Client
}

func NewEntChordCatalogRepository(client *ent.Client) *EntChordCatalogRepository {
	return &EntChordCatalogRepository{client: client}
}

func (r *EntChordCatalogRepository) GetChord(ctx context.Context, id string) (domain.ChordDefinition, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.ChordDefinition{}, domain.ErrNotFound
	}
	return r.first(ctx, r.client.ChordDefinition.Query().Where(chorddefinition.ID(parsed)))
}

func (r *EntChordCatalogRepository) FindChord(ctx context.Context, rootPitchClass int, quality domain.ChordQuality, bassPitchClass *int) (domain.ChordDefinition, error) {
	query := r.client.ChordDefinition.Query().Where(
		chorddefinition.RootPitchClass(rootPitchClass),
		chorddefinition.Quality(string(quality)),
	)
	if bassPitchClass == nil {
		query = query.Where(chorddefinition.BassPitchClassIsNil())
	} else {
		query = query.Where(chorddefinition.BassPitchClass(*bassPitchClass))
	}
	return r.first(ctx, query)
}

func (r *EntChordCatalogRepository) GetChords(ctx context.Context, ids []string) (map[string]domain.ChordDefinition, error) {
	rows, err := r.client.ChordDefinition.Query().Where(chorddefinition.IDIn(parseUUIDsSkippingInvalid(ids)...)).
		WithVoicings(activeVoicingsBestFirst).All(ctx)
	if err != nil {
		return nil, err
	}
	found := make(map[string]domain.ChordDefinition, len(rows))
	for _, row := range rows {
		found[row.ID.String()] = toDomainChord(row)
	}
	return found, nil
}

func (r *EntChordCatalogRepository) GetVoicings(ctx context.Context, ids []string) (map[string]domain.ChordVoicing, error) {
	rows, err := r.client.ChordVoicing.Query().Where(chordvoicing.IDIn(parseUUIDsSkippingInvalid(ids)...)).All(ctx)
	if err != nil {
		return nil, err
	}
	found := make(map[string]domain.ChordVoicing, len(rows))
	for _, row := range rows {
		found[row.ID.String()] = toDomainVoicing(row)
	}
	return found, nil
}

func (r *EntChordCatalogRepository) GetVoicingByDiagramID(ctx context.Context, diagramID string) (domain.ChordVoicing, error) {
	id, err := uuid.Parse(diagramID)
	if err != nil {
		return domain.ChordVoicing{}, domain.ErrNotFound
	}
	row, err := r.client.ChordVoicing.Query().Where(chordvoicing.DiagramID(id)).Only(ctx)
	if ent.IsNotFound(err) {
		return domain.ChordVoicing{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.ChordVoicing{}, err
	}
	return toDomainVoicing(row), nil
}

func activeVoicingsBestFirst(q *ent.ChordVoicingQuery) {
	q.Where(chordvoicing.StatusEQ(chordvoicing.StatusActive)).
		Order(ent.Asc(chordvoicing.FieldRecommendedRank), ent.Asc(chordvoicing.FieldID))
}

// first returns the one chord query selects, with its active voicings best
// first, or domain.ErrNotFound.
func (r *EntChordCatalogRepository) first(ctx context.Context, query *ent.ChordDefinitionQuery) (domain.ChordDefinition, error) {
	row, err := query.WithVoicings(activeVoicingsBestFirst).Only(ctx)
	if ent.IsNotFound(err) {
		return domain.ChordDefinition{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.ChordDefinition{}, err
	}
	return toDomainChord(row), nil
}

func toDomainChord(row *ent.ChordDefinition) domain.ChordDefinition {
	voicings := make([]domain.ChordVoicing, len(row.Edges.Voicings))
	for i, v := range row.Edges.Voicings {
		voicings[i] = toDomainVoicing(v)
	}
	return domain.ChordDefinition{
		ID:              row.ID.String(),
		CanonicalSymbol: row.CanonicalSymbol,
		Root:            row.Root,
		RootPitchClass:  row.RootPitchClass,
		Quality:         domain.ChordQuality(row.Quality),
		Formula:         row.Formula,
		Omittable:       row.Omittable,
		Bass:            row.Bass,
		BassPitchClass:  row.BassPitchClass,
		Aliases:         row.Aliases,
		Voicings:        voicings,
	}
}

func toDomainVoicing(row *ent.ChordVoicing) domain.ChordVoicing {
	fingering := make([]domain.VoicingFinger, len(row.Fingering))
	for i, f := range row.Fingering {
		fingering[i] = domain.VoicingFinger{PositionID: f.PositionID, Finger: f.Finger}
	}
	var shapeFamily *string
	if row.ShapeFamily != nil {
		family := string(*row.ShapeFamily)
		shapeFamily = &family
	}
	return domain.ChordVoicing{
		ID:                row.ID.String(),
		ChordDefinitionID: row.ChordDefinitionID.String(),
		DiagramID:         row.DiagramID.String(),
		InstrumentID:      row.InstrumentID.String(),
		TuningFingerprint: row.TuningFingerprint,
		LowestFret:        row.LowestFret,
		HighestFret:       row.HighestFret,
		Fingering:         fingering,
		MutedStrings:      row.MutedStrings,
		OmittedIntervals:  row.OmittedIntervals,
		Difficulty:        string(row.Difficulty),
		TechniqueTags:     row.TechniqueTags,
		ShapeFamily:       shapeFamily,
		IsMovable:         row.IsMovable,
		RecommendedRank:   row.RecommendedRank,
		Status:            domain.ChordVoicingStatus(row.Status),
		TemplateKey:       row.TemplateKey,
	}
}
