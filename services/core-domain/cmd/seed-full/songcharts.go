package main

import (
	"context"
	"embed"
	"fmt"
	"log"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// songChartFate is where a seeded song chart ends up.
type songChartFate string

const (
	// publishedThenCorrected is published at revision 1, then its draft is
	// changed, so the learner read and the admin preview differ.
	publishedThenCorrected songChartFate = "published_then_corrected"
	publishedThenWithdrawn songChartFate = "published_then_withdrawn"
	draftOnly              songChartFate = "draft_only"
)

// songChartSpec is a seeded chart, written as ChordPro. Every song is in the
// public domain, so seeded data never carries lyrics someone holds rights to.
type songChartSpec struct {
	title           string
	language        string
	rightsConfirmed bool
	chordPro        string
	fate            songChartFate
	// correction is the draft's text after publishing, for
	// publishedThenCorrected.
	correction string
}

//go:embed songcharts/*.cho
var songChartFiles embed.FS

// chordProFile reads one of the seeded ChordPro texts. They are embedded,
// so a missing one fails the build, never a seeding run.
func chordProFile(name string) string {
	text, err := songChartFiles.ReadFile("songcharts/" + name)
	if err != nil {
		panic(err)
	}
	return string(text)
}

func songChartSpecs() []songChartSpec {
	return []songChartSpec{
		{
			title:           "Amazing Grace",
			language:        "en",
			rightsConfirmed: true,
			chordPro:        chordProFile("amazing-grace.cho"),
			fate:            publishedThenCorrected,
			correction:      chordProFile("amazing-grace-corrected.cho"),
		},
		{
			title:           "Oh! Susanna",
			language:        "en",
			rightsConfirmed: true,
			chordPro:        chordProFile("oh-susanna.cho"),
			fate:            publishedThenWithdrawn,
		},
		{
			// A draft with every kind of chord that doesn't fully resolve:
			// a no-chord marking, a slash chord whose bass the catalog
			// lacks, and a symbol that isn't a chord.
			title:    "Ciranda, Cirandinha",
			language: "pt_BR",
			chordPro: chordProFile("ciranda-cirandinha.cho"),
			fate:     draftOnly,
		},
	}
}

// seedSongCharts authors every song chart as author and takes each to its
// fate. A chart starts empty and gets its text through a ChordPro import,
// as an admin bringing in a chart from elsewhere would.
func seedSongCharts(ctx context.Context, charts *application.SongChartService, author domain.User) ([]domain.SongChart, error) {
	seeded := make([]domain.SongChart, 0, len(songChartSpecs()))
	for _, spec := range songChartSpecs() {
		chart, err := seedSongChart(ctx, charts, author, spec)
		if err != nil {
			return nil, fmt.Errorf("song chart %q: %w", spec.title, err)
		}
		seeded = append(seeded, chart)
	}
	return seeded, nil
}

func seedSongChart(ctx context.Context, charts *application.SongChartService, author domain.User, spec songChartSpec) (domain.SongChart, error) {
	chart, err := charts.Create(ctx, author, application.SongChartInput{
		Title: spec.title, Artist: "—", Language: spec.language, RightsConfirmed: spec.rightsConfirmed,
		Body: domain.ImportChordPro("[C]—\n").Body,
	})
	if err != nil {
		return domain.SongChart{}, fmt.Errorf("create: %w", err)
	}
	imported, err := charts.ImportChordPro(ctx, author, chart.ID, spec.chordPro)
	if err != nil {
		return domain.SongChart{}, fmt.Errorf("import: %w", err)
	}
	chart = imported.Chart
	if spec.fate == draftOnly {
		return chart, nil
	}
	if _, err := charts.Publish(ctx, author, chart.ID); err != nil {
		return domain.SongChart{}, fmt.Errorf("publish: %w", err)
	}
	switch spec.fate {
	case publishedThenCorrected:
		corrected, err := charts.ImportChordPro(ctx, author, chart.ID, spec.correction)
		if err != nil {
			return domain.SongChart{}, fmt.Errorf("correct: %w", err)
		}
		return corrected.Chart, nil
	case publishedThenWithdrawn:
		return charts.Withdraw(ctx, author, chart.ID, "Seeded as withdrawn, to show a chart learners can no longer read.")
	case draftOnly:
	}
	return charts.Get(ctx, author, chart.ID)
}

// logSongChartPreviews prints where each seeded chart's preview is, since
// the web app has no list of song charts to reach them from yet.
func logSongChartPreviews(charts []domain.SongChart) {
	for _, c := range charts {
		log.Printf("  song chart %q (%s): /admin/song-charts/%s/preview", c.Draft.Title, c.Status, c.ID)
	}
}
