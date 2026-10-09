package domain

import "time"

// SongChartCompletion is one time a student marked a song chart as played
// in its reader. Marking the same chart again is another completion.
type SongChartCompletion struct {
	SongChartID string
	CompletedAt time.Time
}

// SongsPlayed is how many distinct song charts a student has marked as
// played, and how many of them were first marked on the last 7 calendar
// days.
type SongsPlayed struct {
	Total int
	Last7 int
}

// CountSongsPlayed counts the distinct charts among completions that still
// exist, each from the first time it was marked: a chart marked again
// counts once and is new this week only if its first mark falls on the
// last 7 calendar days at now in loc. A mark dated after today, by a
// device clock running ahead, still counts as played but never as this
// week's. A chart withdrawn since still exists.
func CountSongsPlayed(completions []SongChartCompletion, exists map[string]bool, now time.Time, loc *time.Location) SongsPlayed {
	firstPlayed := map[string]time.Time{}
	for _, c := range completions {
		if !exists[c.SongChartID] {
			continue
		}
		if first, ok := firstPlayed[c.SongChartID]; !ok || c.CompletedAt.Before(first) {
			firstPlayed[c.SongChartID] = c.CompletedAt
		}
	}
	start := Last7DaysStart(now, loc)
	y, m, d := now.In(loc).Date()
	end := time.Date(y, m, d+1, 0, 0, 0, 0, loc)
	played := SongsPlayed{Total: len(firstPlayed)}
	for _, first := range firstPlayed {
		if !first.Before(start) && first.Before(end) {
			played.Last7++
		}
	}
	return played
}
