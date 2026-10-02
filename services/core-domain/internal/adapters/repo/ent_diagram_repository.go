package repo

import (
	"context"
	"regexp"
	"slices"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqljson"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/concept"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/diagram"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/diagramregion"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/instrument"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/position"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/predicate"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/schema"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/skill"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntDiagramRepository persists Diagram records, with their Positions,
// Regions and skill/concept links, via ent/Postgres.
type EntDiagramRepository struct {
	client *ent.Client
}

func NewEntDiagramRepository(client *ent.Client) *EntDiagramRepository {
	return &EntDiagramRepository{client: client}
}

func (r *EntDiagramRepository) Create(ctx context.Context, d domain.Diagram) error {
	id, err := uuid.Parse(d.ID)
	if err != nil {
		return err
	}
	instrumentID, err := uuid.Parse(d.InstrumentID)
	if err != nil {
		return err
	}
	compatibleInstrumentIDs, err := parseUUIDs(d.InstrumentIDs)
	if err != nil {
		return err
	}
	createdBy, err := uuid.Parse(d.CreatedBy)
	if err != nil {
		return err
	}
	skillIDs, err := parseUUIDs(d.SkillIDs())
	if err != nil {
		return err
	}
	conceptIDs, err := parseUUIDs(d.ConceptIDs())
	if err != nil {
		return err
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Diagram.Create().
		SetID(id).
		SetInstrumentID(instrumentID).
		SetNames(d.Names).
		SetKind(diagram.Kind(d.Kind)).
		SetCreatedBy(createdBy).
		SetNillableRootNote(d.RootNote).
		SetNillableColor(d.Color).
		SetLabelDisplay(diagram.LabelDisplay(d.LabelDisplay)).
		SetNillableMode(entMode(d.Mode)).
		SetNillableTempoBpm(d.TempoBPM).
		SetTimeSignatureBeats(d.TimeSignature.Beats).
		SetTimeSignatureBeatValue(d.TimeSignature.BeatValue).
		SetSequence(entSequence(d.Sequence)).
		SetCreatedAt(d.CreatedAt).
		AddCompatibleInstrumentIDs(compatibleInstrumentIDs...).
		AddSkillIDs(skillIDs...).
		AddConceptIDs(conceptIDs...).
		Save(ctx); err != nil {
		return rollback(tx, err)
	}
	if err := createPositions(ctx, tx, id, d.Positions); err != nil {
		return rollback(tx, err)
	}
	if err := createRegions(ctx, tx, id, d.Regions); err != nil {
		return rollback(tx, err)
	}
	return tx.Commit()
}

func (r *EntDiagramRepository) GetByID(ctx context.Context, id string) (domain.Diagram, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.Diagram{}, domain.ErrNotFound
	}
	row, err := withDiagramEdges(r.client.Diagram.Query()).Where(diagram.ID(parsed)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.Diagram{}, domain.ErrNotFound
		}
		return domain.Diagram{}, err
	}
	return toDomainDiagram(row), nil
}

func (r *EntDiagramRepository) List(ctx context.Context, filter domain.DiagramListFilter, page domain.PageRequest) (domain.Page[domain.Diagram], error) {
	query, err := r.filteredQuery(ctx, filter)
	if err != nil {
		return domain.Page[domain.Diagram]{}, err
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return domain.Page[domain.Diagram]{}, err
	}
	rows, err := withDiagramEdges(query).
		Order(byResolvedName(filter.Locale), ent.Asc(diagram.FieldID)).
		Limit(page.Limit).
		Offset(page.Offset).
		All(ctx)
	if err != nil {
		return domain.Page[domain.Diagram]{}, err
	}
	items := make([]domain.Diagram, len(rows))
	for i, row := range rows {
		items[i] = toDomainDiagram(row)
	}
	return domain.Page[domain.Diagram]{Items: items, Total: total}, nil
}

// ListCreatorIDs returns the distinct created_by of every diagram matching
// filter, using the same predicates as List.
func (r *EntDiagramRepository) ListCreatorIDs(ctx context.Context, filter domain.DiagramListFilter) ([]string, error) {
	query, err := r.filteredQuery(ctx, filter)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		CreatedBy uuid.UUID `json:"created_by"`
	}
	if err := query.Unique(true).Select(diagram.FieldCreatedBy).Scan(ctx, &rows); err != nil {
		return nil, err
	}
	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.CreatedBy.String()
	}
	return ids, nil
}

// filteredQuery selects the diagrams matching filter. Every field but Name
// is a SQL predicate. Name is matched here, in Go, over the rows the other
// predicates leave — the same case- and accent-blind matching as
// domain.ContainsLoosely, which a plain SQL comparison can't express — and
// the query is then narrowed to those ids, so ordering and paging stay in
// SQL. The library is small enough for that pass to stay cheap.
func (r *EntDiagramRepository) filteredQuery(ctx context.Context, filter domain.DiagramListFilter) (*ent.DiagramQuery, error) {
	predicates, err := diagramListPredicates(filter)
	if err != nil {
		return nil, err
	}
	query := r.client.Diagram.Query().Where(predicates...)
	if filter.Name == "" {
		return query, nil
	}
	candidates, err := query.Clone().Select(diagram.FieldID, diagram.FieldNames).All(ctx)
	if err != nil {
		return nil, err
	}
	matched := []uuid.UUID{}
	for _, c := range candidates {
		for _, name := range c.Names {
			if domain.ContainsLoosely(name, filter.Name) {
				matched = append(matched, c.ID)
				break
			}
		}
	}
	return query.Where(diagram.IDIn(matched...)), nil
}

// byResolvedName orders diagrams by the name a reader of locale sees — the
// name in locale, else the English one, else the one in the alphabetically
// first language — the SQL form of domain.LocalizedText.Resolve.
//
// locale is written into the SQL as a literal rather than a bind argument:
// ent numbers ORDER BY arguments apart from WHERE arguments, so a bound one
// collides with the filters'. It is only ever a language code, and anything
// that isn't one falls back to "en", so nothing unvalidated reaches the SQL.
func byResolvedName(locale string) func(*sql.Selector) {
	if !languageCodePattern.MatchString(locale) {
		locale = "en"
	}
	return func(s *sql.Selector) {
		names := s.C(diagram.FieldNames)
		s.OrderExpr(sql.Expr(
			"COALESCE(" + names + " ->> '" + locale + "', " + names + " ->> 'en', " +
				"(SELECT value FROM jsonb_each_text(" + names + ") ORDER BY key LIMIT 1))",
		))
	}
}

// languageCodePattern matches a Language.code such as "en" or "pt_BR".
var languageCodePattern = regexp.MustCompile(`^[A-Za-z_]+$`)

// diagramListPredicates translates filter into ent predicates, one per set
// field; domain.DiagramListFilter.Matches states the same rules in memory.
func diagramListPredicates(filter domain.DiagramListFilter) ([]predicate.Diagram, error) {
	var predicates []predicate.Diagram
	if filter.Kind != "" {
		predicates = append(predicates, diagram.KindEQ(diagram.Kind(filter.Kind)))
	}
	byID := []struct {
		value string
		match func(uuid.UUID) predicate.Diagram
	}{
		{filter.VisibleTo, func(viewer uuid.UUID) predicate.Diagram {
			return diagram.Or(diagram.KindEQ(diagram.KindBasic), diagram.CreatedBy(viewer))
		}},
		{filter.CreatedBy, diagram.CreatedBy},
		{filter.InstrumentID, func(id uuid.UUID) predicate.Diagram { return diagram.HasCompatibleInstrumentsWith(instrument.ID(id)) }},
		{filter.SkillID, func(id uuid.UUID) predicate.Diagram { return diagram.HasSkillsWith(skill.ID(id)) }},
		{filter.ConceptID, func(id uuid.UUID) predicate.Diagram { return diagram.HasConceptsWith(concept.ID(id)) }},
	}
	if filter.RootNote != "" {
		predicates = append(predicates, diagram.RootNoteEQ(filter.RootNote))
	}
	if filter.Language != "" {
		language := filter.Language
		predicates = append(predicates, func(s *sql.Selector) {
			s.Where(sqljson.HasKey(s.C(diagram.FieldNames), sqljson.Path(language)))
		})
	}
	for _, f := range byID {
		if f.value == "" {
			continue
		}
		id, err := uuid.Parse(f.value)
		if err != nil {
			return nil, err
		}
		predicates = append(predicates, f.match(id))
	}
	return predicates, nil
}

func (r *EntDiagramRepository) Update(ctx context.Context, d domain.Diagram) error {
	id, err := uuid.Parse(d.ID)
	if err != nil {
		return domain.ErrNotFound
	}
	compatibleInstrumentIDs, err := parseUUIDs(d.InstrumentIDs)
	if err != nil {
		return err
	}
	skillIDs, err := parseUUIDs(d.SkillIDs())
	if err != nil {
		return err
	}
	conceptIDs, err := parseUUIDs(d.ConceptIDs())
	if err != nil {
		return err
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	// Checked up front: replacing the join rows for an id with no diagram
	// trips their foreign key before UpdateOneID can report not-found.
	exists, err := tx.Diagram.Query().Where(diagram.ID(id)).Exist(ctx)
	if err != nil {
		return rollback(tx, err)
	}
	if !exists {
		return rollback(tx, domain.ErrNotFound)
	}
	update := tx.Diagram.UpdateOneID(id).
		SetNames(d.Names).
		SetNillableRootNote(d.RootNote).
		SetNillableColor(d.Color).
		SetLabelDisplay(diagram.LabelDisplay(d.LabelDisplay)).
		SetTimeSignatureBeats(d.TimeSignature.Beats).
		SetTimeSignatureBeatValue(d.TimeSignature.BeatValue).
		SetSequence(entSequence(d.Sequence))
	// Unlike root note and color, mode and tempo can be cleared.
	if d.Mode != nil {
		update.SetMode(*entMode(d.Mode))
	} else {
		update.ClearMode()
	}
	if d.TempoBPM != nil {
		update.SetTempoBpm(*d.TempoBPM)
	} else {
		update.ClearTempoBpm()
	}
	if _, err := update.
		ClearCompatibleInstruments().
		AddCompatibleInstrumentIDs(compatibleInstrumentIDs...).
		ClearSkills().
		AddSkillIDs(skillIDs...).
		ClearConcepts().
		AddConceptIDs(conceptIDs...).
		Save(ctx); err != nil {
		return rollback(tx, err)
	}
	if _, err := tx.Position.Delete().Where(position.DiagramID(id)).Exec(ctx); err != nil {
		return rollback(tx, err)
	}
	if _, err := tx.DiagramRegion.Delete().Where(diagramregion.DiagramID(id)).Exec(ctx); err != nil {
		return rollback(tx, err)
	}
	if err := createPositions(ctx, tx, id, d.Positions); err != nil {
		return rollback(tx, err)
	}
	if err := createRegions(ctx, tx, id, d.Regions); err != nil {
		return rollback(tx, err)
	}
	return tx.Commit()
}

// withDiagramEdges eager-loads everything toDomainDiagram reads, with
// positions in the order the author listed them and regions in drawing
// order.
func withDiagramEdges(q *ent.DiagramQuery) *ent.DiagramQuery {
	return q.
		WithCompatibleInstruments().
		WithPositions(func(pq *ent.PositionQuery) { pq.Order(ent.Asc(position.FieldOrdinal)) }).
		WithRegions(func(rq *ent.DiagramRegionQuery) { rq.Order(ent.Asc(diagramregion.FieldOrdinal)) }).
		WithSkills().
		WithConcepts()
}

func createPositions(ctx context.Context, tx *ent.Tx, diagramID uuid.UUID, positions []domain.Position) error {
	builders := make([]*ent.PositionCreate, len(positions))
	for i, p := range positions {
		id, err := uuid.Parse(p.ID)
		if err != nil {
			return err
		}
		builders[i] = tx.Position.Create().
			SetID(id).
			SetDiagramID(diagramID).
			SetOrdinal(i).
			SetInterval(p.Interval).
			SetNoteName(p.NoteName).
			SetShape(position.Shape(p.Shape)).
			SetNillableColor(p.Color).
			SetNillableStringNumber(p.String).
			SetNillableFret(p.Fret).
			SetNillableKey(p.Key)
		if p.CustomLabel != nil {
			builders[i].SetCustomLabel(p.CustomLabel)
		}
		if p.Note != nil {
			builders[i].SetNote(p.Note)
		}
	}
	_, err := tx.Position.CreateBulk(builders...).Save(ctx)
	if ent.IsConstraintError(err) {
		// The diagram row exists and ordinals are generated 0..n-1, so the
		// only constraint a caller can trip here is the global uniqueness of
		// a client-supplied position id already owned by another diagram.
		return domain.NewValidationError("positions", "a position_id is already used by another diagram")
	}
	return err
}

// createRegions stores regions under diagramID in drawing order.
func createRegions(ctx context.Context, tx *ent.Tx, diagramID uuid.UUID, regions []domain.Region) error {
	if len(regions) == 0 {
		return nil
	}
	builders := make([]*ent.DiagramRegionCreate, len(regions))
	for i, r := range regions {
		id, err := uuid.Parse(r.ID)
		if err != nil {
			return err
		}
		builders[i] = tx.DiagramRegion.Create().
			SetID(id).
			SetDiagramID(diagramID).
			SetOrdinal(i).
			SetNillableFretStart(r.FretStart).
			SetNillableFretEnd(r.FretEnd).
			SetNillableStringStart(r.StringStart).
			SetNillableStringEnd(r.StringEnd).
			SetNillableKeyStart(r.KeyStart).
			SetNillableKeyEnd(r.KeyEnd).
			SetDescription(r.Description).
			SetNillableColor(r.Color)
	}
	_, err := tx.DiagramRegion.CreateBulk(builders...).Save(ctx)
	if ent.IsConstraintError(err) {
		// As with positions, the only constraint a caller can trip is a
		// client-supplied region id already owned by another diagram.
		return domain.NewValidationError("regions", "a region_id is already used by another diagram")
	}
	return err
}

func toDomainDiagram(row *ent.Diagram) domain.Diagram {
	positions := make([]domain.Position, len(row.Edges.Positions))
	for i, p := range row.Edges.Positions {
		positions[i] = domain.Position{
			ID:          p.ID.String(),
			Interval:    p.Interval,
			NoteName:    p.NoteName,
			Shape:       domain.PositionShape(p.Shape),
			Color:       p.Color,
			String:      p.StringNumber,
			Fret:        p.Fret,
			Key:         p.Key,
			CustomLabel: localizedTextOrNil(p.CustomLabel),
			Note:        localizedTextOrNil(p.Note),
		}
	}
	var regions []domain.Region
	for _, r := range row.Edges.Regions {
		regions = append(regions, domain.Region{
			ID:          r.ID.String(),
			FretStart:   r.FretStart,
			FretEnd:     r.FretEnd,
			StringStart: r.StringStart,
			StringEnd:   r.StringEnd,
			KeyStart:    r.KeyStart,
			KeyEnd:      r.KeyEnd,
			Description: domain.LocalizedText(r.Description),
			Color:       r.Color,
		})
	}
	instrumentIDs := make([]string, len(row.Edges.CompatibleInstruments))
	for i, instrument := range row.Edges.CompatibleInstruments {
		instrumentIDs[i] = instrument.ID.String()
	}
	slices.Sort(instrumentIDs)
	return domain.Diagram{
		ID:            row.ID.String(),
		InstrumentID:  row.InstrumentID.String(),
		InstrumentIDs: instrumentIDs,
		Names:         domain.LocalizedText(row.Names),
		Kind:          domain.DiagramKind(row.Kind),
		CreatedBy:     row.CreatedBy.String(),
		RootNote:      row.RootNote,
		LabelDisplay:  domain.LabelDisplay(row.LabelDisplay),
		Color:         row.Color,
		Positions:     positions,
		Regions:       regions,
		Mode:          domainMode(row.Mode),
		TimeSignature: domain.TimeSignature{
			Beats: row.TimeSignatureBeats, BeatValue: row.TimeSignatureBeatValue,
		},
		Sequence:  domainSequence(row.Sequence),
		TempoBPM:  row.TempoBpm,
		Skills:    domainSkillsFromEdges(row.Edges.Skills),
		Concepts:  domainConceptsFromEdges(row.Edges.Concepts),
		CreatedAt: row.CreatedAt,
	}
}

func entMode(mode *domain.DiagramMode) *diagram.Mode {
	if mode == nil {
		return nil
	}
	m := diagram.Mode(*mode)
	return &m
}

func domainMode(mode *diagram.Mode) *domain.DiagramMode {
	if mode == nil {
		return nil
	}
	m := domain.DiagramMode(*mode)
	return &m
}

// entSequence is steps as stored; no steps is an empty list, never NULL.
func entSequence(steps []domain.SequenceStep) []schema.SequenceStep {
	out := make([]schema.SequenceStep, len(steps))
	for i, s := range steps {
		out[i] = schema.SequenceStep{
			PositionIDs: s.PositionIDs,
			Value:       schema.NoteValue{Num: s.Value.Num, Den: s.Value.Den},
			Strum:       string(s.Strum),
		}
		if out[i].PositionIDs == nil {
			out[i].PositionIDs = []string{}
		}
	}
	return out
}

// domainSequence is stored steps as the domain holds them: nil for none, a
// rest's positions as an empty list.
func domainSequence(steps []schema.SequenceStep) []domain.SequenceStep {
	if len(steps) == 0 {
		return nil
	}
	out := make([]domain.SequenceStep, len(steps))
	for i, s := range steps {
		out[i] = domain.SequenceStep{
			PositionIDs: s.PositionIDs,
			Value:       domain.NoteValue{Num: s.Value.Num, Den: s.Value.Den},
			Strum:       domain.Strum(s.Strum),
		}
	}
	return out
}

// localizedTextOrNil is text as a domain.LocalizedText, keeping an absent
// (NULL) column as nil rather than an empty map.
func localizedTextOrNil(text map[string]string) domain.LocalizedText {
	if len(text) == 0 {
		return nil
	}
	return domain.LocalizedText(text)
}
