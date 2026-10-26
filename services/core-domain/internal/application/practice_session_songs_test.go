package application_test

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

const (
	commonProgressions = "common-progressions"
	cadences           = "cadences"
	popProgression     = "pop-progression"
)

var songProgressionCatalog = fakeProgressions{
	{Key: "I-IV-I-V", Bars: []string{"I", "IV", "I", "V"}, NodeIDs: []string{commonProgressions}},
	{Key: "I-IV-V-I", Bars: []string{"I", "IV", "V", "I"}, NodeIDs: []string{commonProgressions, cadences}},
}

// fakeLastInHandSession holds the items of each student's last in-hand session.
type fakeLastInHandSession map[string][]string

func (f fakeLastInHandSession) LastInHandSessionItemKeys(_ context.Context, studentID string) ([]string, error) {
	return f[studentID], nil
}

func songItemKey(layout, chartID string) string { return "song:" + layout + ":" + chartID }

// songFixture is a practice fixture with the songs installed, on the guitar and
// bass layouts, and common-progressions on both instruments' paths with a
// play-along, pop-progression, at the play-in-time rung below them.
type songFixture struct {
	*practiceFixture
	charts *fakeSongChartRepository
	last   fakeLastInHandSession
}

func newSongFixture(t *testing.T) *songFixture {
	f := &songFixture{practiceFixture: newPracticeFixture(t), charts: newFakeSongChartRepository(), last: fakeLastInHandSession{}}
	f.instruments.put(domain.Instrument{ID: practiceGuitar, Tuning: []string{"E2", "A2", "D3", "G3", "B3", "E4"}})
	f.instruments.put(domain.Instrument{ID: practiceBass, Tuning: []string{"E1", "A1", "D2", "G2"}})
	f.skill(cadences)
	f.playAlong(popProgression, "Pop progression", practiceGuitar, commonProgressions, 90)
	f.onPathFor([]string{practiceGuitar, practiceBass}, commonProgressions, cadences)
	f.svc.WithHarmonyDrills(songProgressionCatalog, harmonyChordCatalog(), fakeFretboardLayouts{
		practiceGuitar: {{ID: practiceGuitar}},
		practiceBass:   {{ID: practiceBass}},
	})
	f.svc.WithSongs(f.charts, f.last)
	return f
}

// song publishes a chart whose first section is tagged with progressions and
// classifies its songs on both layouts under their nodes.
func (f *songFixture) song(id, title string, progressions ...string) {
	f.songOfSections(id, title, progressions)
}

// songOfSections publishes a chart with a section per entry of sections, each
// tagged with its progressions, and classifies its songs on both layouts
// under their nodes.
func (f *songFixture) songOfSections(id, title string, sections ...[]string) {
	var body domain.SongChartDocument
	var progressions []string
	for _, tags := range sections {
		body.Sections = append(body.Sections, domain.SongChartSection{Kind: domain.SectionVerse, ProgressionKeys: tags})
		progressions = append(progressions, tags...)
	}
	chart := domain.SongChart{ID: id, Status: domain.SongChartDraftStatus, Draft: domain.SongChartDraft{
		Title: title, Artist: "Traditional", Body: body,
		RightsConfirmation: &domain.RightsConfirmation{ConfirmedBy: "admin-1", ConfirmedAt: f.now},
	}}
	require.NoError(f.t, f.charts.Create(context.Background(), chart))
	published, rev, err := chart.Publish("admin-1", f.now)
	require.NoError(f.t, err)
	require.NoError(f.t, f.charts.Publish(context.Background(), published, rev))
	var nodes []string
	for _, p := range songProgressionCatalog {
		for _, tag := range progressions {
			if p.Key == tag {
				for _, n := range p.NodeIDs {
					if !slices.Contains(nodes, n) {
						nodes = append(nodes, n)
					}
				}
			}
		}
	}
	for _, layout := range []string{practiceGuitar, practiceBass} {
		f.cells[layout] = append(f.cells[layout], domain.ClassifiedItem{ItemKey: songItemKey(layout, id), NodeIDs: nodes})
	}
}

// atUseRung makes the play-along below the songs accurate, so the subject's
// new items come from the use rung.
func (f *songFixture) atUseRung() {
	f.state(popProgression, domain.PracticeItemState{Level: domain.KnowledgeLevelAccurate, Counted: 3, Accuracy: 1, Box: 2, DueAt: f.inDays(5)})
}

// rated makes a song rated days ago, not yet due.
func (f *songFixture) rated(itemKey string, days int) {
	f.stateOf(itemKey, domain.PracticeItemState{Level: domain.KnowledgeLevelLearning, Counted: 1, Box: 1, LastAt: f.daysAgo(days), DueAt: f.inDays(1)})
}

func lastItem(t *testing.T, plan domain.PracticeSessionPlan) domain.PracticeSessionItem {
	t.Helper()
	require.NotEmpty(t, plan.Items)
	return plan.Items[len(plan.Items)-1]
}

func TestPracticeSessionService_ComposePlanSongs(t *testing.T) {
	t.Run("a subject at the use rung ends the session with a song that plays it", func(t *testing.T) {
		f := newSongFixture(t)
		f.song("amazing-grace", "Amazing Grace", "I-IV-I-V")
		f.atUseRung()

		plan := f.choose(t, ptrTo(practiceGuitar), commonProgressions, 20)

		last := lastItem(t, plan)
		assert.Equal(t, songItemKey(practiceGuitar, "amazing-grace"), last.ItemKey)
		assert.Equal(t, domain.PracticeItemKindSong, last.Kind)
		assert.Equal(t, domain.PracticePickApplication, last.Reason)
		assert.Equal(t, domain.PracticeRungUse, last.Rung)
		assert.Equal(t, 130, last.EstimatedSeconds)
		require.NotNil(t, last.Song)
		assert.Equal(t, domain.PlannedSong{
			SongChartID: "amazing-grace", LayoutInstrumentID: practiceGuitar, Title: "Amazing Grace", Artist: "Traditional",
			ProgressionKey: "I-IV-I-V", SectionIndex: 0, Plays: domain.SongPlaysChords,
		}, *last.Song)
	})

	t.Run("the song names the first section that plays the subject's progression", func(t *testing.T) {
		f := newSongFixture(t)
		f.songOfSections("home-on-the-range", "Home on the Range", nil, []string{"I-IV-I-V"}, []string{"I-IV-I-V"})
		f.atUseRung()

		plan := f.choose(t, ptrTo(practiceGuitar), commonProgressions, 20)

		last := lastItem(t, plan)
		require.NotNil(t, last.Song)
		assert.Equal(t, "I-IV-I-V", last.Song.ProgressionKey)
		assert.Equal(t, 1, last.Song.SectionIndex)
	})

	t.Run("today's mix ends with a song of its main subject at the use rung", func(t *testing.T) {
		f := newSongFixture(t)
		f.song("amazing-grace", "Amazing Grace", "I-IV-I-V")
		f.atUseRung()
		f.exercise("ex-1", commonProgressions, 60, practiceGuitar)
		f.accurate(domain.ExerciseItemKey("ex-1"))

		plan := f.compose(t, practiceGuitar, 20)

		last := lastItem(t, plan)
		assert.Equal(t, songItemKey(practiceGuitar, "amazing-grace"), last.ItemKey)
		assert.Equal(t, domain.PracticePickApplication, last.Reason)
	})

	t.Run("below the use rung, the ending is a play-along, not a song", func(t *testing.T) {
		f := newSongFixture(t)
		f.song("amazing-grace", "Amazing Grace", "I-IV-I-V")
		// The current rung is know, and the next one with material play in time.
		f.exercise("ex-1", commonProgressions, 60, practiceGuitar)

		plan := f.choose(t, ptrTo(practiceGuitar), commonProgressions, 20)

		assert.Empty(t, itemsOfKind(plan, domain.PracticeItemKindSong))
		last := lastItem(t, plan)
		assert.Equal(t, domain.PlayAlongItemKey(popProgression), last.ItemKey)
		assert.Equal(t, domain.PracticePickApplication, last.Reason)
	})

	t.Run("a chosen session with a song but no play-along has an ending", func(t *testing.T) {
		f := newSongFixture(t)
		f.song("oh-susanna", "Oh! Susanna", "I-IV-V-I")
		f.exercise("ex-1", cadences, 60, practiceGuitar)
		f.accurate(domain.ExerciseItemKey("ex-1"))

		plan := f.choose(t, ptrTo(practiceGuitar), cadences, 20)

		assert.Equal(t, songItemKey(practiceGuitar, "oh-susanna"), lastItem(t, plan).ItemKey)
	})

	t.Run("a song's nodes are its progressions' nodes", func(t *testing.T) {
		f := newSongFixture(t)
		f.song("amazing-grace", "Amazing Grace", "I-IV-I-V")
		f.song("oh-susanna", "Oh! Susanna", "I-IV-V-I")
		f.exercise("ex-1", cadences, 60, practiceGuitar)
		f.accurate(domain.ExerciseItemKey("ex-1"))

		plan := f.choose(t, ptrTo(practiceGuitar), cadences, 20)

		last := lastItem(t, plan)
		assert.Equal(t, songItemKey(practiceGuitar, "oh-susanna"), last.ItemKey)
		assert.Equal(t, "I-IV-V-I", last.Song.ProgressionKey)
		assert.Len(t, itemsOfKind(plan, domain.PracticeItemKindSong), 1)
	})

	t.Run("a subject whose next rung with material is use can end with a song", func(t *testing.T) {
		f := newSongFixture(t)
		f.song("amazing-grace", "Amazing Grace", "I-IV-I-V")

		plan := f.choose(t, ptrTo(practiceGuitar), commonProgressions, 20)

		assert.Equal(t, songItemKey(practiceGuitar, "amazing-grace"), lastItem(t, plan).ItemKey)
	})

	t.Run("a session plays at most one song, last, and never as a focus or review item", func(t *testing.T) {
		f := newSongFixture(t)
		for _, id := range []string{"s1", "s2", "s3", "s4", "s5"} {
			f.song(id, id, "I-IV-I-V")
		}
		f.atUseRung()
		f.dueSince(2, songItemKey(practiceGuitar, "s1"), songItemKey(practiceGuitar, "s2"))

		plan := f.choose(t, ptrTo(practiceGuitar), commonProgressions, 30)

		songs := itemsOfKind(plan, domain.PracticeItemKindSong)
		require.Len(t, songs, 1)
		assert.Equal(t, songs[0], lastItem(t, plan))
		assert.Equal(t, domain.PracticePickApplication, songs[0].Reason)
	})

	t.Run("a song never rated on the layout comes before one rated before", func(t *testing.T) {
		f := newSongFixture(t)
		f.song("amazing-grace", "Amazing Grace", "I-IV-I-V")
		f.song("oh-susanna", "Oh! Susanna", "I-IV-I-V")
		f.atUseRung()
		f.rated(songItemKey(practiceGuitar, "amazing-grace"), 3)
		f.rated(songItemKey(practiceBass, "oh-susanna"), 3)

		plan := f.choose(t, ptrTo(practiceGuitar), commonProgressions, 20)

		assert.Equal(t, songItemKey(practiceGuitar, "oh-susanna"), lastItem(t, plan).ItemKey, "rated on bass is not rated on guitar")
	})

	t.Run("among songs rated before, the one rated longest ago comes first", func(t *testing.T) {
		f := newSongFixture(t)
		f.song("amazing-grace", "Amazing Grace", "I-IV-I-V")
		f.song("oh-susanna", "Oh! Susanna", "I-IV-I-V")
		f.song("home-on-the-range", "Home on the Range", "I-IV-I-V")
		f.atUseRung()
		f.rated(songItemKey(practiceGuitar, "amazing-grace"), 2)
		f.rated(songItemKey(practiceGuitar, "oh-susanna"), 9)
		f.rated(songItemKey(practiceGuitar, "home-on-the-range"), 5)

		plan := f.choose(t, ptrTo(practiceGuitar), commonProgressions, 20)

		assert.Equal(t, songItemKey(practiceGuitar, "oh-susanna"), lastItem(t, plan).ItemKey)
	})

	t.Run("the song of the last in-hand session isn't played again while another fits", func(t *testing.T) {
		f := newSongFixture(t)
		f.song("amazing-grace", "Amazing Grace", "I-IV-I-V")
		f.song("oh-susanna", "Oh! Susanna", "I-IV-I-V")
		f.atUseRung()
		f.rated(songItemKey(practiceGuitar, "amazing-grace"), 3)
		f.last[studentCaller().ID] = []string{domain.PlayAlongItemKey(popProgression), songItemKey(practiceGuitar, "oh-susanna")}

		plan := f.choose(t, ptrTo(practiceGuitar), commonProgressions, 20)

		assert.Equal(t, songItemKey(practiceGuitar, "amazing-grace"), lastItem(t, plan).ItemKey)
	})

	t.Run("the song of the last in-hand session comes back when it is the only one", func(t *testing.T) {
		f := newSongFixture(t)
		f.song("amazing-grace", "Amazing Grace", "I-IV-I-V")
		f.atUseRung()
		f.last[studentCaller().ID] = []string{songItemKey(practiceGuitar, "amazing-grace")}

		plan := f.choose(t, ptrTo(practiceGuitar), commonProgressions, 20)

		assert.Equal(t, songItemKey(practiceGuitar, "amazing-grace"), lastItem(t, plan).ItemKey)
	})

	t.Run("on bass, the song is played as the roots of its chords", func(t *testing.T) {
		f := newSongFixture(t)
		f.song("amazing-grace", "Amazing Grace", "I-IV-I-V")
		f.exercise("ex-1", commonProgressions, 60, practiceBass)
		f.accurate(domain.ExerciseItemKey("ex-1"))

		plan := f.choose(t, ptrTo(practiceBass), commonProgressions, 20)

		last := lastItem(t, plan)
		assert.Equal(t, songItemKey(practiceBass, "amazing-grace"), last.ItemKey)
		assert.Equal(t, domain.SongPlaysRoots, last.Song.Plays)
	})

	t.Run("a session under 10 minutes has no song", func(t *testing.T) {
		f := newSongFixture(t)
		f.song("amazing-grace", "Amazing Grace", "I-IV-I-V")
		f.atUseRung()
		f.exercise("ex-1", commonProgressions, 60, practiceGuitar)

		plan := f.choose(t, ptrTo(practiceGuitar), commonProgressions, 5)

		assert.Empty(t, itemsOfKind(plan, domain.PracticeItemKindSong))
	})

	t.Run("a session in the head never plays a song", func(t *testing.T) {
		f := newSongFixture(t)
		f.song("amazing-grace", "Amazing Grace", "I-IV-I-V")
		f.exercise("ex-1", commonProgressions, 60)

		plan := f.choose(t, nil, commonProgressions, 20)

		assert.Empty(t, itemsOfKind(plan, domain.PracticeItemKindSong))
	})
}

// Songs are authored, so there can be many: composing reads them once, however
// many songs there are and however many skills each serves.
func TestPracticeSessionService_ComposePlanReadsTheSongsOnce(t *testing.T) {
	f := newSongFixture(t)
	for _, id := range []string{"s1", "s2", "s3", "s4", "s5"} {
		f.song(id, id, "I-IV-V-I")
	}
	f.atUseRung()
	f.exercise("ex-1", cadences, 60, practiceGuitar)
	f.accurate(domain.ExerciseItemKey("ex-1"))

	plan := f.compose(t, practiceGuitar, 20)

	require.Len(t, itemsOfKind(plan, domain.PracticeItemKindSong), 1)
	assert.Equal(t, 1, f.charts.publishedReads)
	assert.Zero(t, f.charts.chartReads)

	f.compose(t, practiceGuitar, 20)

	assert.Equal(t, 2, f.charts.publishedReads, "a later session reads them again, so a newly published song is found")
}

func ptrTo(s string) *string { return &s }
