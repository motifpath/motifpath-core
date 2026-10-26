package application

import (
	"context"
	"log/slog"
	"slices"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// practiceSubjectCatalogFormat is the version of how a catalog's items are
// tallied: what suits a session, the rung each item, or each way of a drill,
// trains. A change to any of them bumps it, so catalogs built by older code
// are rebuilt on their next read.
const practiceSubjectCatalogFormat = 1

// The practice subject catalog is the part of a subjects list that is the
// same for every student: which items a scope has, under which nodes, what
// sessions they suit and what rungs they train. A content change raises the
// content version; the first list that finds its scope's catalog built from
// an older version rebuilds it from the content and stores it, so the lists
// after it read one row per scope instead of every item. The knowledge nodes,
// the student's states and their path are always read live.

// SubjectCatalogInvalidator marks the practice subject catalog out of date
// after a content change.
type SubjectCatalogInvalidator interface {
	InvalidateSubjectCatalogs(ctx context.Context)
}

// WithSubjectCatalogs makes ListSubjects read, and keep current, the
// precomputed catalog in catalogs.
func (s *PracticeSessionService) WithSubjectCatalogs(catalogs ports.PracticeSubjectCatalogRepository) *PracticeSessionService {
	s.subjectCatalogs = catalogs
	return s
}

// InvalidateSubjectCatalogs raises the content version, so the next list of
// each scope rebuilds its catalog. A failure is logged, not returned: the
// change is already committed, and the invalidation on the next start
// catches up. The write outlives the caller's cancellation, as a practice
// reference write does.
func (s *PracticeSessionService) InvalidateSubjectCatalogs(ctx context.Context) {
	if s.subjectCatalogs == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), referenceWriteTimeout)
	defer cancel()
	if err := s.subjectCatalogs.BumpContentVersion(ctx); err != nil {
		slog.ErrorContext(ctx, "invalidate the practice subject catalog", "error", err)
	}
}

// subjectCatalogItems is the catalog items of each of scopes: read from the
// precomputed catalog when it was built from the current content in the
// current format, built from the content otherwise, and then stored for the
// lists after it. s is composing.
func (s *PracticeSessionService) subjectCatalogItems(ctx context.Context, scopes []string) (map[string][]domain.PracticeSubjectCatalogItem, error) {
	items, version, err := s.currentCatalogItems(ctx, scopes)
	if err != nil {
		return nil, err
	}
	var missing []string
	for _, scope := range scopes {
		if _, ok := items[scope]; !ok {
			missing = append(missing, scope)
		}
	}
	if len(missing) == 0 {
		return items, nil
	}
	built, err := s.buildCatalogs(ctx, missing, version)
	if err != nil {
		return nil, err
	}
	for _, c := range built {
		items[c.Scope] = c.Items
	}
	if s.subjectCatalogs != nil {
		// A catalog not stored is built again by the next list; this one is
		// answered either way.
		if _, err := s.subjectCatalogs.Store(ctx, version, built); err != nil {
			slog.ErrorContext(ctx, "store the practice subject catalog", "error", err)
		}
	}
	return items, nil
}

// currentCatalogItems is the catalog items of those of scopes whose stored
// catalog was built from the current content version in the current format,
// and that version; none without a catalog installed.
func (s *PracticeSessionService) currentCatalogItems(ctx context.Context, scopes []string) (map[string][]domain.PracticeSubjectCatalogItem, int64, error) {
	items := map[string][]domain.PracticeSubjectCatalogItem{}
	if s.subjectCatalogs == nil {
		return items, 0, nil
	}
	rows, version, err := s.subjectCatalogs.Catalogs(ctx, scopes)
	if err != nil {
		return nil, 0, err
	}
	for scope, row := range rows {
		if row.ContentVersion == version && row.FormatVersion == practiceSubjectCatalogFormat {
			items[scope] = row.Items
		}
	}
	return items, version, nil
}

// buildCatalogs builds the catalog of each of scopes from content version
// version, reading the diagrams and exercises all of them name in one read
// each. s is composing.
func (s *PracticeSessionService) buildCatalogs(ctx context.Context, scopes []string, version int64) ([]domain.PracticeSubjectCatalog, error) {
	templates, err := s.drillTemplates(ctx)
	if err != nil {
		return nil, err
	}
	classified := make([][]domain.ClassifiedItem, len(scopes))
	var all []domain.ClassifiedItem
	for i, scope := range scopes {
		if classified[i], err = s.rollup.ClassifiedItems(ctx, scope); err != nil {
			return nil, err
		}
		all = append(all, classified[i]...)
	}
	if err := s.composingDiagrams.readAll(ctx, all); err != nil {
		return nil, err
	}
	if err := s.composingExercises.readAll(ctx, all); err != nil {
		return nil, err
	}
	now := s.now()
	c := newComposer(now, 0, s.shuffle, templates)
	loaded := map[string]*practiceCandidate{}
	catalogs := make([]domain.PracticeSubjectCatalog, len(scopes))
	for i, scope := range scopes {
		catalog := domain.PracticeSubjectCatalog{
			Scope: scope, ContentVersion: version, FormatVersion: practiceSubjectCatalogFormat, BuiltAt: now,
			Items: []domain.PracticeSubjectCatalogItem{},
		}
		for _, item := range classified[i] {
			cand, err := s.loadOnce(ctx, loaded, item.ItemKey)
			if err != nil {
				return nil, err
			}
			if cand != nil {
				catalog.Items = append(catalog.Items, c.catalogItem(*cand, item))
			}
		}
		catalogs[i] = catalog
	}
	return catalogs, nil
}

// catalogItem is cand's catalog item, classified as item: a drill item has
// the rung of each of its ways, any other item the one rung it trains.
func (c *composer) catalogItem(cand practiceCandidate, item domain.ClassifiedItem) domain.PracticeSubjectCatalogItem {
	out := domain.PracticeSubjectCatalogItem{
		ItemKey: item.ItemKey, NodeIDs: slices.Clone(item.NodeIDs),
		InHand: cand.suitsSession(true), InHead: cand.suitsSession(false),
	}
	if cand.drill == nil {
		out.Rung = c.rungOf(cand)
		return out
	}
	for _, way := range cand.drill.Ways() {
		rung := domain.ItemRung(domain.PracticeSessionItem{Kind: cand.drill.Kind(), Way: way}, c.templates)
		out.Ways = append(out.Ways, domain.PracticeSubjectCatalogWay{Way: way, Rung: rung})
	}
	return out
}

// CatalogInvalidatingReferences is references, invalidating the practice
// subject catalog through catalogs after each diagram, exercise, instrument
// or song chart reference it writes: those are written after every save that
// changes what a subjects list holds.
func CatalogInvalidatingReferences(references ports.PracticeReferenceWriter, catalogs SubjectCatalogInvalidator) ports.PracticeReferenceWriter {
	return catalogInvalidatingReferences{PracticeReferenceWriter: references, catalogs: catalogs}
}

type catalogInvalidatingReferences struct {
	ports.PracticeReferenceWriter
	catalogs SubjectCatalogInvalidator
}

func (r catalogInvalidatingReferences) PutDiagrams(ctx context.Context, refs []domain.DiagramReference) error {
	if err := r.PracticeReferenceWriter.PutDiagrams(ctx, refs); err != nil {
		return err
	}
	r.catalogs.InvalidateSubjectCatalogs(ctx)
	return nil
}

func (r catalogInvalidatingReferences) PutExercises(ctx context.Context, refs []domain.ExerciseReference) error {
	if err := r.PracticeReferenceWriter.PutExercises(ctx, refs); err != nil {
		return err
	}
	r.catalogs.InvalidateSubjectCatalogs(ctx)
	return nil
}

func (r catalogInvalidatingReferences) PutInstruments(ctx context.Context, refs []domain.InstrumentReference) error {
	if err := r.PracticeReferenceWriter.PutInstruments(ctx, refs); err != nil {
		return err
	}
	r.catalogs.InvalidateSubjectCatalogs(ctx)
	return nil
}

func (r catalogInvalidatingReferences) PutSongCharts(ctx context.Context, refs []domain.SongChartReference) error {
	if err := r.PracticeReferenceWriter.PutSongCharts(ctx, refs); err != nil {
		return err
	}
	r.catalogs.InvalidateSubjectCatalogs(ctx)
	return nil
}
