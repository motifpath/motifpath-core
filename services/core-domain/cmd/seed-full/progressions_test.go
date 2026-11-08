package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestSeededCreatedProgressions(t *testing.T) {
	specs := createdProgressionSpecs()
	var drills, chartOnly []domain.Progression
	for _, s := range specs {
		p, err := domain.NewCreatedProgression(s.mode, s.bars, s.nicknames, []string{"en", "pt_BR"})
		require.NoError(t, err, "%v reads as a progression", s.bars)
		if !s.inDrills {
			chartOnly = append(chartOnly, p)
			continue
		}
		added, err := p.AddedToDrills([]string{"concept"}, drills)
		require.NoError(t, err, "%s enters the drills", p.Key)
		drills = append(drills, added)
	}

	t.Run("one progression is for song charts only", func(t *testing.T) {
		assert.Len(t, chartOnly, 1)
	})
	t.Run("the drills meet a minor progression and a length other than four bars", func(t *testing.T) {
		var minor, otherLength bool
		for _, p := range drills {
			minor = minor || p.Mode == string(domain.ProgressionModeMinor)
			otherLength = otherLength || len(p.Bars) != 4
		}
		assert.True(t, minor, "a minor progression is in the drills")
		assert.True(t, otherLength, "a progression of other than four bars is in the drills")
	})
}
