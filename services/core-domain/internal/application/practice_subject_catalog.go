package application

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// practiceSubjectCatalogFormat is the version of how a catalog's items are
// tallied: what suits a session, the rung each item, or each way of a drill,
// trains. A change to any of them bumps it, so catalogs built by older code
// are listed live until they are rebuilt.
const practiceSubjectCatalogFormat = 1

// The practice subject catalog is the part of a subjects list that is the
// same for every student: which items a scope has, under which nodes, what
// sessions they suit and what rungs they train. It is built ahead of time
// and rebuilt after content changes, so a list reads one row per scope
// instead of every item. The knowledge nodes, the student's states and their
// path are still read live.

// CatalogSignaller queues a rebuild of the practice subject catalog.
type CatalogSignaller interface {
	Signal(ctx context.Context, reason domain.PracticeCatalogSignalReason)
}

// WithSubjectCatalogs makes ListSubjects read the precomputed catalog in
// catalogs, signalling through signals when a scope has none it can read.
func (s *PracticeSessionService) WithSubjectCatalogs(catalogs ports.PracticeSubjectCatalogRepository, signals CatalogSignaller) *PracticeSessionService {
	s.subjectCatalogs = catalogs
	s.catalogSignals = signals
	return s
}

// BuildSubjectCatalogs builds the catalog of every instrument, and of none,
// from the content as it is now.
func (s *PracticeSessionService) BuildSubjectCatalogs(ctx context.Context) ([]domain.PracticeSubjectCatalog, error) {
	instruments, err := s.instruments.List(ctx)
	if err != nil {
		return nil, err
	}
	scopes := make([]string, 0, len(instruments)+1)
	for _, i := range instruments {
		scopes = append(scopes, i.ID)
	}
	return s.forComposition().buildCatalogs(ctx, append(scopes, ""))
}

// subjectCatalogItems is the catalog items of each of scopes: read from the
// precomputed catalog when it has them in the current format, built live
// otherwise, which queues a rebuild. s is composing.
func (s *PracticeSessionService) subjectCatalogItems(ctx context.Context, scopes []string) (map[string][]domain.PracticeSubjectCatalogItem, error) {
	items := map[string][]domain.PracticeSubjectCatalogItem{}
	if s.subjectCatalogs != nil {
		rows, err := s.subjectCatalogs.Catalogs(ctx, scopes)
		if err != nil {
			return nil, err
		}
		for scope, row := range rows {
			if row.FormatVersion == practiceSubjectCatalogFormat {
				items[scope] = row.Items
			}
		}
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
	built, err := s.buildCatalogs(ctx, missing)
	if err != nil {
		return nil, err
	}
	for _, c := range built {
		items[c.Scope] = c.Items
	}
	if s.subjectCatalogs != nil {
		s.catalogSignals.Signal(ctx, domain.PracticeCatalogSignalMissing)
	}
	return items, nil
}

// buildCatalogs builds the catalog of each of scopes, reading the diagrams
// and exercises all of them name in one read each. s is composing.
func (s *PracticeSessionService) buildCatalogs(ctx context.Context, scopes []string) ([]domain.PracticeSubjectCatalog, error) {
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
		catalog := domain.PracticeSubjectCatalog{Scope: scope, FormatVersion: practiceSubjectCatalogFormat, BuiltAt: now, Items: []domain.PracticeSubjectCatalogItem{}}
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

// PracticeSubjectCatalogService keeps the practice subject catalog rebuilt:
// content changes queue signals, and it rebuilds once for every signal
// pending, after they settle.
type PracticeSubjectCatalogService struct {
	practice *PracticeSessionService
	catalogs ports.PracticeSubjectCatalogRepository
	// settle is how long a burst of signals is waited out before a rebuild;
	// poll is how often it looks for signals queued by another instance.
	settle, poll time.Duration
	wake         chan struct{}
}

func NewPracticeSubjectCatalogService(practice *PracticeSessionService, catalogs ports.PracticeSubjectCatalogRepository, settle, poll time.Duration) *PracticeSubjectCatalogService {
	return &PracticeSubjectCatalogService{practice: practice, catalogs: catalogs, settle: settle, poll: poll, wake: make(chan struct{}, 1)}
}

// Signal queues a rebuild and wakes Run. A failure is logged, not returned:
// the content is already committed, and the rebuild on the next start
// catches up. The write outlives the caller's cancellation, as a practice
// reference write does.
func (s *PracticeSubjectCatalogService) Signal(ctx context.Context, reason domain.PracticeCatalogSignalReason) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), referenceWriteTimeout)
	defer cancel()
	if err := s.catalogs.Signal(ctx, reason); err != nil {
		slog.ErrorContext(ctx, "queue a practice subject catalog rebuild", "reason", reason, "error", err)
		return
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// RebuildPending rebuilds the catalog when signals are pending, reporting
// whether it did.
func (s *PracticeSubjectCatalogService) RebuildPending(ctx context.Context) (bool, error) {
	return s.catalogs.Rebuild(ctx, s.practice.BuildSubjectCatalogs)
}

// Run queues the start signal, then rebuilds whenever it is woken by a
// signal or its poll comes round, until ctx is done. A rebuild that fails is
// logged; its signals stay for the next one.
func (s *PracticeSubjectCatalogService) Run(ctx context.Context) {
	s.Signal(ctx, domain.PracticeCatalogSignalStart)
	poll := time.NewTicker(s.poll)
	defer poll.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
		case <-poll.C:
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(s.settle):
		}
		// The signals of the burst are all pending now; this rebuild is theirs.
		select {
		case <-s.wake:
		default:
		}
		if _, err := s.RebuildPending(ctx); err != nil && ctx.Err() == nil {
			slog.ErrorContext(ctx, "rebuild the practice subject catalog", "error", err)
		}
	}
}

// CatalogSignallingReferences is references, queueing a catalog rebuild
// through signals after each diagram, exercise, instrument or song chart
// reference it writes: those are written after every save that changes
// what a subjects list holds.
func CatalogSignallingReferences(references ports.PracticeReferenceWriter, signals CatalogSignaller) ports.PracticeReferenceWriter {
	return catalogSignallingReferences{PracticeReferenceWriter: references, signals: signals}
}

type catalogSignallingReferences struct {
	ports.PracticeReferenceWriter
	signals CatalogSignaller
}

func (r catalogSignallingReferences) PutDiagrams(ctx context.Context, refs []domain.DiagramReference) error {
	if err := r.PracticeReferenceWriter.PutDiagrams(ctx, refs); err != nil {
		return err
	}
	r.signals.Signal(ctx, domain.PracticeCatalogSignalDiagram)
	return nil
}

func (r catalogSignallingReferences) PutExercises(ctx context.Context, refs []domain.ExerciseReference) error {
	if err := r.PracticeReferenceWriter.PutExercises(ctx, refs); err != nil {
		return err
	}
	r.signals.Signal(ctx, domain.PracticeCatalogSignalExercise)
	return nil
}

func (r catalogSignallingReferences) PutInstruments(ctx context.Context, refs []domain.InstrumentReference) error {
	if err := r.PracticeReferenceWriter.PutInstruments(ctx, refs); err != nil {
		return err
	}
	r.signals.Signal(ctx, domain.PracticeCatalogSignalInstrument)
	return nil
}

func (r catalogSignallingReferences) PutSongCharts(ctx context.Context, refs []domain.SongChartReference) error {
	if err := r.PracticeReferenceWriter.PutSongCharts(ctx, refs); err != nil {
		return err
	}
	r.signals.Signal(ctx, domain.PracticeCatalogSignalSongChart)
	return nil
}
