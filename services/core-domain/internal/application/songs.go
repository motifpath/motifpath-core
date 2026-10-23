package application

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// WithSongs installs the songs: each published song chart tagged with the
// progressions it plays, played whole on a fretboard layout as a session's
// application ending. It reads the progressions and the chord catalog the
// harmony drills were installed with, so it does nothing without them.
// lastInHand tells the song of the student's last in-hand session, which isn't
// played again while another fits.
func (s *PracticeSessionService) WithSongs(charts ports.SongChartRepository, lastInHand ports.LastInHandSessionReader) *PracticeSessionService {
	if s.harmony == nil {
		return s
	}
	s.lastInHand = lastInHand
	return s.WithDrillSources(songSource{harmony: s.harmony, charts: charts})
}

// songSource finds the songs the knowledge map classifies under a skill. They
// are played, so a session in the head finds none.
type songSource struct {
	harmony *harmonyDrills
	charts  ports.SongChartRepository
}

func (src songSource) SkillItems(ctx context.Context, skillID, instrumentID string, inHand bool, knowledge KnowledgeMap) ([]domain.DrillItem, error) {
	return src.forComposition().SkillItems(ctx, skillID, instrumentID, inHand, knowledge)
}

func (src songSource) Item(ctx context.Context, itemKey string) (domain.DrillItem, bool, error) {
	return src.forComposition().Item(ctx, itemKey)
}

// forComposition reads the songs once for a composition, however many of them
// its skills find: they are authored, so there can be many.
func (src songSource) forComposition() DrillSource {
	return &composingSongs{source: src}
}

// composingSongs is the songs one composition plays, read on first need.
type composingSongs struct {
	source  songSource
	once    sync.Once
	library songLibrary
	err     error
}

// songLibrary is what a song is played from: the latest published revision of
// every published chart, by chart, the progression catalog, and the guitar
// layout.
type songLibrary struct {
	revisions map[string]domain.SongChartRevision
	catalog   []domain.Progression
	guitar    string
}

func (c *composingSongs) read(ctx context.Context) (songLibrary, error) {
	c.once.Do(func() {
		var revisions []domain.SongChartRevision
		if revisions, c.err = c.source.charts.PublishedRevisions(ctx); c.err != nil {
			return
		}
		c.library.revisions = make(map[string]domain.SongChartRevision, len(revisions))
		for _, rev := range revisions {
			c.library.revisions[rev.SongChartID] = rev
		}
		if c.library.catalog, c.err = c.source.harmony.progressions.List(ctx); c.err != nil {
			return
		}
		c.library.guitar, c.err = c.source.harmony.guitarLayoutID(ctx)
	})
	return c.library, c.err
}

func (c *composingSongs) SkillItems(ctx context.Context, skillID, _ string, inHand bool, knowledge KnowledgeMap) ([]domain.DrillItem, error) {
	if !inHand {
		return nil, nil
	}
	var items []domain.DrillItem
	for _, classified := range knowledge.Items {
		if !slices.Contains(classified.NodeIDs, skillID) {
			continue
		}
		item, ok, err := c.Item(ctx, classified.ItemKey)
		if err != nil {
			return nil, err
		}
		if ok {
			items = append(items, item)
		}
	}
	return items, nil
}

// Item reads itemKey as a song: its layout and the latest published revision
// of its chart, with the progressions it is tagged with. A chart no longer
// published, or tagged with no progression the catalog has, is no song. The
// layout the chord catalog's voicings are on is the guitar one; any other is a
// bass, whose student plays the roots.
func (c *composingSongs) Item(ctx context.Context, itemKey string) (domain.DrillItem, bool, error) {
	rest, ok := strings.CutPrefix(itemKey, string(domain.PracticeItemKindSong)+":")
	if !ok {
		return nil, false, nil
	}
	layoutID, chartID, ok := strings.Cut(rest, ":")
	if !ok {
		return nil, false, nil
	}
	library, err := c.read(ctx)
	if err != nil {
		return nil, false, err
	}
	rev, published := library.revisions[chartID]
	if !published {
		return nil, false, nil
	}
	var progressions []domain.Progression
	for _, key := range rev.ProgressionKeys {
		if i := slices.IndexFunc(library.catalog, func(p domain.Progression) bool { return p.Key == key }); i >= 0 {
			progressions = append(progressions, library.catalog[i])
		}
	}
	if len(progressions) == 0 {
		return nil, false, nil
	}
	return domain.SongItem{
		LayoutInstrumentID: layoutID, Bass: layoutID != library.guitar, ChartID: chartID,
		Title: rev.Title, Artist: rev.Artist, Progressions: progressions,
	}, true, nil
}

// endsSessionOnly reports whether c is only ever a session's application
// ending, such as a song: never its warm-up, a focus item, a review or a
// stretch.
func (c practiceCandidate) endsSessionOnly() bool {
	return c.drill != nil && domain.EndsSessionOnly(c.drill.Kind())
}

// rungSubjects are the subjects whose ladder c is material for. In Today's mix
// a song is material for every node its progressions practise, though it is
// offered for one; a chosen session counts everything under its one subject.
func (c *composer) rungSubjects(cand practiceCandidate) []string {
	if song, ok := cand.drill.(domain.SongItem); ok && c.chosenNode == "" {
		return song.NodeIDs()
	}
	return []string{c.subjectOf(cand)}
}

// songEnding finds the song that ends a session on subject, when subject takes
// new items from the use rung: one of candidates, not yet picked, that
// practises serves ("" for any). One never rated on its layout comes first,
// then the one rated longest ago, at random among equals; the song of the
// student's last in-hand session only when no other fits. Its node is subject.
func (c *composer) songEnding(candidates []practiceCandidate, serves, subject string) *practiceCandidate {
	if !slices.Contains(c.newRungs[subject], domain.PracticeRungUse) {
		return nil
	}
	var songs, fresh []practiceCandidate
	for _, cand := range candidates {
		song, ok := cand.drill.(domain.SongItem)
		if !ok || c.picked[cand.key()] || serves != "" && !song.Serves(serves) {
			continue
		}
		songs = append(songs, cand)
		if !slices.Contains(c.lastInHand, cand.key()) {
			fresh = append(fresh, cand)
		}
	}
	if len(fresh) > 0 {
		songs = fresh
	}
	if len(songs) == 0 {
		return nil
	}
	c.shuffleCandidates(songs)
	slices.SortStableFunc(songs, func(a, b practiceCandidate) int { return ratedAt(a).Compare(ratedAt(b)) })
	pick := songs[0]
	pick.drill = pick.drill.(domain.SongItem).ForSubject(subject)
	pick.nodeID = &subject
	return &pick
}

// ratedAt is when cand was last rated; the zero time, first of all, when it
// never was.
func ratedAt(cand practiceCandidate) time.Time {
	if !cand.practised() || cand.state.LastAt == nil {
		return time.Time{}
	}
	return *cand.state.LastAt
}
