//go:build integration

package bdd

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// A song is a published chart tagged with the progressions it plays, played
// whole as the application ending of an in-hand session on a progression.
func registerSongSteps(sc *godog.ScenarioContext, w *world) {
	tagged := func(title, progression string) error { return w.publishedTaggedChart(title, progression) }
	sc.Step(`^the published song chart "([^"]+)" is tagged with the progression "([^"]+)"$`, tagged)
	sc.Step(`^the published song chart "([^"]+)" is tagged with "([^"]+)" too$`, tagged)
	sc.Step(`^the published song charts "([^"]+)" and "([^"]+)" are tagged with "([^"]+)" too$`, func(a, b, progression string) error {
		if err := w.publishedTaggedChart(a, progression); err != nil {
			return err
		}
		return w.publishedTaggedChart(b, progression)
	})
	sc.Step(`^(\d+) published song charts are tagged with "([^"]+)"$`, func(n int, progression string) error {
		for i := range n {
			if err := w.publishedTaggedChart(fmt.Sprintf("Song %d", i+1), progression); err != nil {
				return err
			}
		}
		return nil
	})
	sc.Step(`^the published song chart "([^"]+)" has no progressions$`, func(title string) error { return w.publishedTaggedChart(title) })
	sc.Step(`^"([^"]+)" rolls up to "([^"]+)" in the progression catalog$`, func(progression, node string) error {
		return w.progressionRollsUpTo(progression, node)
	})
	sc.Step(`^"([^"]+)" rolls up to "([^"]+)" and "([^"]+)"$`, func(progression, a, b string) error {
		return w.progressionRollsUpTo(progression, a, b)
	})
	sc.Step(`^no other published song chart plays a progression of "([^"]+)"$`, w.noOtherSongPractises)

	sc.Step(`^"([^"]+)" is accurate on every rung of "([^"]+)" with material on guitar below use$`, func(name, node string) error {
		return w.atUseRung(name, node, "guitar")
	})
	sc.Step(`^"([^"]+)" is at the use rung on "([^"]+)" on guitar$`, func(name, node string) error { return w.atUseRung(name, node, "guitar") })
	sc.Step(`^"([^"]+)" is at the use rung on "([^"]+)" on electric bass$`, func(name, node string) error {
		return w.atUseRung(name, node, "electric-bass")
	})
	sc.Step(`^"([^"]+)"'s current rung on "([^"]+)" on guitar is "move", and "[^"]+" has no target items in hand$`, w.atTheMoveRung)
	sc.Step(`^"([^"]+)"'s current rung on "([^"]+)" on guitar is "know", with play_in_time material next$`, w.atTheKnowRung)
	sc.Step(`^"([^"]+)"'s main subject in Today's mix is "([^"]+)", at the use rung on guitar$`, func(name, node string) error {
		w.onChordPath(name, node)
		return w.atUseRung(name, node, "guitar")
	})

	sc.Step(`^"([^"]+)" rated the song "([^"]+)" on guitar and it is due$`, func(name, title string) error {
		// Its subject has more to practise, or a session couldn't pick it.
		w.onChordPath(name, "common-progressions")
		if err := w.atUseRung(name, "common-progressions", "guitar"); err != nil {
			return err
		}
		w.dueItems(name, songKey("guitar", title))
		return nil
	})
	sc.Step(`^"([^"]+)" rated "([^"]+)" on guitar, and never "[^"]+"$`, func(name, title string) error {
		w.ratedSongDaysAgo(name, title, 3)
		return nil
	})
	sc.Step(`^"([^"]+)" rated "([^"]+)" on guitar (\d+) days ago$`, func(name, title string, days int) error {
		w.ratedSongDaysAgo(name, title, days)
		return nil
	})
	sc.Step(`^"([^"]+)" rated "([^"]+)" on guitar (\d+) days ago, "([^"]+)" (\d+) days ago and "([^"]+)" (\d+) days ago$`, func(name, a string, da int, b string, db int, c string, dc int) error {
		w.ratedSongDaysAgo(name, a, da)
		w.ratedSongDaysAgo(name, b, db)
		w.ratedSongDaysAgo(name, c, dc)
		return nil
	})
	sc.Step(`^"([^"]+)"'s last in-hand session on guitar ended with "([^"]+)", which "[^"]+" left without rating it$`, w.lastInHandSessionEndedWith)
	sc.Step(`^"([^"]+)"'s last in-hand session ended with "([^"]+)"$`, w.lastInHandSessionEndedWith)
	sc.Step(`^"([^"]+)" was withdrawn after "([^"]+)" rated it on guitar$`, w.withdrawnAfterRating)

	sc.Step(`^the last item is the song "([^"]+)" on guitar with the reason application$`, func(title string) error {
		return w.lastItemIsSong(title, "guitar", true)
	})
	sc.Step(`^the last item is the song "([^"]+)" with the reason application$`, func(title string) error { return w.lastItemIsSong(title, "", true) })
	sc.Step(`^the last item is the song "([^"]+)"$`, func(title string) error { return w.lastItemIsSong(title, "", false) })
	sc.Step(`^the last item is the song "([^"]+)" on the electric-bass layout, playing its roots$`, func(title string) error {
		return w.songPlays(title, "electric-bass", generated.Roots, true)
	})
	sc.Step(`^the song "([^"]+)" on guitar plays its chords$`, func(title string) error {
		return w.songPlays(title, "guitar", generated.Chords, false)
	})
	sc.Step(`^the song is at the use rung, for the progression "([^"]+)"$`, w.songIsAtTheUseRungFor)
	sc.Step(`^the song is estimated at (\d+) seconds, outside the focus time$`, w.songEstimatedAt)
	sc.Step(`^"([^"]+)", whose progression doesn't roll up to "[^"]+", is not in the session$`, w.songNotInSession)
	sc.Step(`^no item in the session is the song "([^"]+)"$`, w.songNotInSession)
	sc.Step(`^no item in the session is a song$`, w.noSongs)
	sc.Step(`^the session has exactly one song, and it is the last item$`, w.exactlyOneSongLast)
	sc.Step(`^no song in the session has a reason other than application$`, w.songsOnlyEnd)
	sc.Step(`^"([^"]+)" still has the evidence of the song "([^"]+)" on guitar$`, w.stillHasSongEvidence)
}

func songChartID(title string) string { return deterministicUUID("song-chart", title).String() }

func songKey(instrument, title string) string {
	return domain.SongItemKey(instrumentID(instrument).String(), songChartID(title))
}

// publishedTaggedChart publishes a chart tagged with progressions and, as the
// classifier does, makes it a song on the guitar and bass layouts under the
// progressions' nodes; an untagged chart is no song.
func (w *world) publishedTaggedChart(title string, progressions ...string) error {
	chart := domain.SongChart{
		ID: songChartID(title), Status: domain.SongChartDraftStatus, CreatedBy: w.curatorID(), CreatedAt: fixedNow,
		Draft: domain.SongChartDraft{
			Title: title, Artist: "Traditional", Language: "en", ProgressionKeys: progressions,
			RightsConfirmation: &domain.RightsConfirmation{ConfirmedBy: w.curatorID(), ConfirmedAt: fixedNow},
			UpdatedBy:          w.curatorID(), UpdatedAt: fixedNow,
		},
	}
	if err := w.songChartRepo.Create(w.ctx(), chart); err != nil {
		return err
	}
	published, rev, err := chart.Publish(w.curatorID(), fixedNow)
	if err != nil {
		return err
	}
	if err := w.songChartRepo.Publish(w.ctx(), published, rev); err != nil {
		return err
	}
	var nodes []string
	for _, key := range progressions {
		p, err := catalogProgression(key)
		if err != nil {
			return err
		}
		for _, n := range p.NodeIDs {
			if !slices.Contains(nodes, n) {
				nodes = append(nodes, n)
			}
		}
	}
	if len(nodes) == 0 {
		return nil
	}
	for _, instrument := range []string{"guitar", "electric-bass"} {
		if _, err := w.ensureInstrumentSeeded(instrument); err != nil {
			return err
		}
		id := instrumentID(instrument).String()
		w.practiceItems.chords[id] = append(w.practiceItems.chords[id], domain.ClassifiedItem{ItemKey: songKey(instrument, title), NodeIDs: nodes})
	}
	return nil
}

func (w *world) progressionRollsUpTo(progression string, nodes ...string) error {
	p, err := catalogProgression(progression)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if !slices.Contains(p.NodeIDs, w.skillIDFor(node).String()) {
			return fmt.Errorf("%s doesn't roll up to %s in the catalog", progression, node)
		}
	}
	return nil
}

// noOtherSongPractises checks that only one song is classified on guitar.
func (w *world) noOtherSongPractises(string) error {
	n := 0
	for _, item := range w.practiceItems.chords[instrumentID("guitar").String()] {
		if strings.HasPrefix(item.ItemKey, string(domain.PracticeItemKindSong)+":") {
			n++
		}
	}
	return expectChord(n == 1, "%d songs are classified on guitar", n)
}

// rungMaterial classifies node's progression key tour of I-IV-I-V (the move
// rung) and an exercise for every instrument (the know rung) on instrument.
func (w *world) rungMaterial(node, instrument string) (tour, exercise string, err error) {
	if instrument == "electric-bass" {
		bass, err := w.ensureInstrumentSeeded(instrument)
		if err != nil {
			return "", "", err
		}
		four := 4
		bass.StringCount, bass.Tuning = &four, []string{"E1", "A1", "D2", "G2"}
		w.instruments.put(bass)
	}
	tour = domain.ProgressionKeysItemKey(instrumentID(instrument).String(), "I-IV-I-V")
	w.classifyChords(instrument, node, tour)
	exercise = w.putPracticeExercise("know-"+node, node)
	return tour, exercise, nil
}

// atUseRung makes every item of node below use accurate on instrument, the
// material it adds and any a step put before, so its new items come from the
// use rung.
func (w *world) atUseRung(name, node, instrument string) error {
	if _, _, err := w.rungMaterial(node, instrument); err != nil {
		return err
	}
	w.putItemStates(name, domain.KnowledgeLevelAccurate, 3, 2, 5, w.itemsBelowUse(node, instrument)...)
	return nil
}

// itemsBelowUse are the keys of node's exercises and of its generated items on
// instrument other than songs.
func (w *world) itemsBelowUse(node, instrument string) []string {
	id := w.skillIDFor(node).String()
	var keys []string
	w.exercises.mu.Lock()
	for _, e := range w.exercises.byID {
		if slices.ContainsFunc(e.Skills, func(n domain.KnowledgeNode) bool { return n.ID == id }) {
			keys = append(keys, domain.ExerciseItemKey(e.ID))
		}
	}
	w.exercises.mu.Unlock()
	for _, item := range w.practiceItems.chords[instrumentID(instrument).String()] {
		if slices.Contains(item.NodeIDs, id) && !strings.HasPrefix(item.ItemKey, string(domain.PracticeItemKindSong)+":") {
			keys = append(keys, item.ItemKey)
		}
	}
	return keys
}

// atTheMoveRung leaves the tour new, with no target material, so use is the
// next rung with material.
func (w *world) atTheMoveRung(name, node string) error {
	_, exercise, err := w.rungMaterial(node, "guitar")
	if err != nil {
		return err
	}
	w.putItemStates(name, domain.KnowledgeLevelAccurate, 3, 2, 5, exercise)
	return nil
}

// atTheKnowRung leaves the exercise new, so the session's new items come from
// know and the play-along's play in time.
func (w *world) atTheKnowRung(_, node string) error {
	w.putPracticeExercise("know-"+node, node, "guitar")
	return nil
}

func (w *world) ratedSongDaysAgo(name, title string, days int) {
	at, dueAt := fixedNow.AddDate(0, 0, -days), fixedNow.AddDate(0, 0, 5)
	w.practiceStates.put(w.ensureRegistered(name, domain.RoleStudent).String(), domain.PracticeItemState{
		ItemKey: songKey("guitar", title), RulesVersion: domain.PracticeRulesVersion, Level: domain.KnowledgeLevelLearning,
		Counted: 1, Box: 1, DueAt: &dueAt, LastAt: &at,
	})
}

func (w *world) lastInHandSessionEndedWith(name, title string) error {
	started, guitar := fixedNow.Add(-24*time.Hour), instrumentID("guitar").String()
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	w.practiceActivity.mu.Lock()
	defer w.practiceActivity.mu.Unlock()
	w.practiceActivity.sessions[studentID] = append(w.practiceActivity.sessions[studentID], fakePracticeSession{
		instrumentID: &guitar, startedAt: &started, lastEventAt: started, itemKeys: []string{songKey("guitar", title)},
	})
	return nil
}

func (w *world) withdrawnAfterRating(title, name string) error {
	w.ratedSongDaysAgo(name, title, 3)
	chart, err := w.songChartRepo.GetByID(w.ctx(), songChartID(title))
	if err != nil {
		return err
	}
	withdrawn, err := chart.Withdraw(w.curatorID(), fixedNow, "Rights disputed")
	if err != nil {
		return err
	}
	return w.songChartRepo.Save(w.ctx(), withdrawn)
}

// lastItemIsSong checks the last item is title's song, on instrument's layout
// unless instrument is "", and with the reason application when application.
func (w *world) lastItemIsSong(title, instrument string, application bool) error {
	last, err := w.lastItem()
	if err != nil {
		return err
	}
	if last.Kind != generated.PracticeItemKindSong || last.Song == nil || last.Song.SongChartId.String() != songChartID(title) {
		return fmt.Errorf("the last item is %s, not the song %q", last.ItemKey, title)
	}
	if instrument != "" && last.ItemKey != songKey(instrument, title) {
		return fmt.Errorf("the last item is %s, not on the %s layout", last.ItemKey, instrument)
	}
	return expectChord(!application || last.Reason == generated.PracticePickReasonApplication, "the song has the reason %s", last.Reason)
}

func (w *world) songPlays(title, instrument string, plays generated.PlannedSongPlays, last bool) error {
	if last {
		if err := w.lastItemIsSong(title, instrument, false); err != nil {
			return err
		}
	}
	songs, err := w.planItemsOfKind(generated.PracticeItemKindSong)
	if err != nil {
		return err
	}
	for _, item := range songs {
		if item.ItemKey == songKey(instrument, title) {
			return expectChord(item.Song.Plays == plays, "the song plays %s", item.Song.Plays)
		}
	}
	return fmt.Errorf("the session has no song %q on %s", title, instrument)
}

func (w *world) songIsAtTheUseRungFor(progression string) error {
	last, err := w.lastItem()
	if err != nil {
		return err
	}
	if last.Song == nil {
		return fmt.Errorf("the last item %s is no song", last.ItemKey)
	}
	return expectChord(last.Rung == generated.Use && last.Song.ProgressionKey == progression,
		"the song is at the %s rung, for %s", last.Rung, last.Song.ProgressionKey)
}

func (w *world) songEstimatedAt(seconds int) error {
	last, err := w.lastItem()
	if err != nil {
		return err
	}
	return expectChord(last.Kind == generated.PracticeItemKindSong && last.EstimatedSeconds == seconds && last.Reason == generated.PracticePickReasonApplication,
		"the last item %s is estimated at %d seconds, reason %s", last.ItemKey, last.EstimatedSeconds, last.Reason)
}

func (w *world) songNotInSession(title string) error {
	songs, err := w.planItemsOfKind(generated.PracticeItemKindSong)
	if err != nil {
		return err
	}
	for _, item := range songs {
		if item.Song != nil && item.Song.SongChartId.String() == songChartID(title) {
			return fmt.Errorf("the session has the song %q", title)
		}
	}
	return nil
}

func (w *world) noSongs() error {
	songs, err := w.planItemsOfKind(generated.PracticeItemKindSong)
	if err != nil {
		return err
	}
	return expectChord(len(songs) == 0, "the session has %d songs", len(songs))
}

func (w *world) exactlyOneSongLast() error {
	songs, err := w.planItemsOfKind(generated.PracticeItemKindSong)
	if err != nil {
		return err
	}
	last, err := w.lastItem()
	if err != nil {
		return err
	}
	return expectChord(len(songs) == 1 && songs[0].ItemKey == last.ItemKey, "the session has %d songs, the last item is %s", len(songs), last.ItemKey)
}

func (w *world) songsOnlyEnd() error {
	songs, err := w.planItemsOfKind(generated.PracticeItemKindSong)
	if err != nil {
		return err
	}
	for _, item := range songs {
		if item.Reason != generated.PracticePickReasonApplication {
			return fmt.Errorf("%s has the reason %s", item.ItemKey, item.Reason)
		}
	}
	return nil
}

func (w *world) stillHasSongEvidence(name, title string) error {
	key := songKey("guitar", title)
	states, err := w.practiceStates.GetStates(w.ctx(), w.ensureRegistered(name, domain.RoleStudent).String(), []string{key})
	if err != nil {
		return err
	}
	state, ok := states[key]
	return expectChord(ok && state.Counted > 0, "%s has no evidence of the song %q on guitar", name, title)
}
