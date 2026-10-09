//go:build integration

package bdd

import (
	"context"
	"fmt"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/domain"
)

// SongChartCompletions reads the played marks a scenario gave the student,
// raw, as the worker keeps them.
func (f *fakePracticeActivity) SongChartCompletions(_ context.Context, studentID string) ([]domain.SongChartCompletion, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.songs[studentID], nil
}

func (f *fakePracticeActivity) addSongChartCompletion(studentID string, c domain.SongChartCompletion) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.songs == nil {
		f.songs = map[string][]domain.SongChartCompletion{}
	}
	f.songs[studentID] = append(f.songs[studentID], c)
}

func registerSongsPlayedSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)" marked the song charts "([^"]+)" and "([^"]+)" as played$`, func(name, first, second string) error {
		if err := w.marksPlayed(name, first, 0); err != nil {
			return err
		}
		return w.marksPlayed(name, second, 0)
	})
	sc.Step(`^"([^"]+)" marked the song chart "([^"]+)" as played$`, func(name, title string) error { return w.marksPlayed(name, title, 0) })
	sc.Step(`^"([^"]+)" first marked "([^"]+)" as played (\d+) days ago$`, w.marksPlayed)
	sc.Step(`^"([^"]+)" first marked "([^"]+)" as played yesterday$`, func(name, title string) error { return w.marksPlayed(name, title, 1) })
	sc.Step(`^"([^"]+)" marked "([^"]+)" as played again today$`, func(name, title string) error { return w.marksPlayed(name, title, 0) })
	sc.Step(`^"([^"]+)" was withdrawn afterwards$`, w.withdrawnAfterPlayed)
	sc.Step(`^a song_chart\.completed event from "([^"]+)" names a song chart that doesn't exist$`, w.marksMissingChartPlayed)

	sc.Step(`^the overview shows (\d+) songs? played$`, w.overviewShowsSongsPlayed)
	sc.Step(`^the overview shows (\d+) songs? played in the last 7 days$`, w.overviewShowsSongsPlayedLast7)
}

// ── Givens ───────────────────────────────────────────────────────────────

// marksPlayed records name marking title as played daysAgo days before the
// summary's today, in the evening, publishing the chart first if the
// scenario has none by that title.
func (w *world) marksPlayed(name, title string, daysAgo int) error {
	chartID, err := w.playableChart(title)
	if err != nil {
		return err
	}
	w.practiceActivity.addSongChartCompletion(w.ensureRegistered(name, domain.RoleStudent).String(),
		domain.SongChartCompletion{SongChartID: chartID, CompletedAt: w.localTime(daysAgo, 20, 0)})
	return nil
}

// playableChart is the id of the published chart title, stored straight in
// the repository: what a chart holds is beside the point for counting it.
func (w *world) playableChart(title string) (string, error) {
	if id, ok := w.charts().idByTitle[title]; ok {
		return id, nil
	}
	id := deterministicUUID("played-song-chart", title).String()
	chart := domain.SongChart{ID: id, Status: domain.SongChartPublished, Draft: domain.SongChartDraft{Title: title}}
	if err := w.songChartRepo.Create(context.Background(), chart); err != nil {
		return "", err
	}
	w.charts().idByTitle[title] = id
	return id, nil
}

func (w *world) withdrawnAfterPlayed(title string) error {
	id, err := w.chartID(title)
	if err != nil {
		return err
	}
	chart, err := w.songChartRepo.GetByID(context.Background(), id.String())
	if err != nil {
		return err
	}
	chart.Status = domain.SongChartWithdrawn
	return w.songChartRepo.Save(context.Background(), chart)
}

func (w *world) marksMissingChartPlayed(name string) error {
	w.practiceActivity.addSongChartCompletion(w.ensureRegistered(name, domain.RoleStudent).String(),
		domain.SongChartCompletion{SongChartID: uuid.NewString(), CompletedAt: w.localTime(0, 20, 0)})
	return nil
}

// ── Thens ────────────────────────────────────────────────────────────────

func (w *world) overviewShowsSongsPlayed(songs int) error {
	o, err := w.overview()
	if err != nil {
		return err
	}
	if o.SongsPlayedTotal != songs {
		return fmt.Errorf("expected %d songs played, got %d", songs, o.SongsPlayedTotal)
	}
	return nil
}

func (w *world) overviewShowsSongsPlayedLast7(songs int) error {
	o, err := w.overview()
	if err != nil {
		return err
	}
	if o.SongsPlayedLast7 != songs {
		return fmt.Errorf("expected %d songs played in the last 7 days, got %d", songs, o.SongsPlayedLast7)
	}
	return nil
}
