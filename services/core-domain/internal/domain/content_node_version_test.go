package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestNewContentNodeVersionSnapshot(t *testing.T) {
	mediaURL := "https://cdn.example.com/video.mp4"
	node := domain.ContentNode{
		ID:          "node-1",
		Title:       "Open chords",
		ContentType: domain.ContentTypeVideo,
		MediaURL:    &mediaURL,
	}
	publishedAt := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)

	version := domain.NewContentNodeVersionSnapshot("version-1", node, 1, "teacher-1", publishedAt)

	assert.Equal(t, "version-1", version.ID)
	assert.Equal(t, "node-1", version.ContentNodeID)
	assert.Equal(t, 1, version.VersionNumber)
	assert.Equal(t, "Open chords", version.Title)
	assert.Equal(t, domain.ContentTypeVideo, version.ContentType)
	assert.Equal(t, &mediaURL, version.MediaURL)
	assert.Equal(t, "teacher-1", version.PublishedBy)
	assert.Equal(t, publishedAt, version.PublishedAt)

	t.Run("a later snapshot of an edited node does not mutate the earlier one", func(t *testing.T) {
		edited := node
		edited.Title = "Open chords (revised)"

		second := domain.NewContentNodeVersionSnapshot("version-2", edited, 2, "teacher-1", publishedAt.Add(time.Hour))

		assert.Equal(t, "Open chords", version.Title, "first snapshot must stay unchanged")
		assert.Equal(t, "Open chords (revised)", second.Title)
		assert.Equal(t, 2, second.VersionNumber)
	})
}
