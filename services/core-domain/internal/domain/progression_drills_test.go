package domain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func drillProgression(key string, confusable ...string) domain.Progression {
	_, numerals, created := strings.Cut(key, "/")
	if !created {
		numerals = key
	}
	bars := strings.Split(strings.NewReplacer("b", "♭", "#", "♯").Replace(numerals), "-")
	return domain.Progression{Key: key, Mode: "major", Bars: bars, Confusable: confusable, InDrills: true}
}

func slicesRepeat(bars []string, times int) []string {
	var out []string
	for range times {
		out = append(out, bars...)
	}
	return out
}

func chartOnly(t *testing.T, mode string, bars ...string) domain.Progression {
	t.Helper()
	p, err := domain.NewCreatedProgression(mode, bars, nil, []string{"en"})
	require.NoError(t, err)
	return p
}

func TestAddedToDrills(t *testing.T) {
	nodes := []string{"node-1"}

	t.Run("it enters the drills with its nodes", func(t *testing.T) {
		added, err := chartOnly(t, "major", "I", "iii", "IV", "V").AddedToDrills(nodes, []domain.Progression{drillProgression("I-V-vi-IV")})
		require.NoError(t, err)
		assert.True(t, added.InDrills)
		assert.Equal(t, nodes, added.NodeIDs)
	})

	t.Run("wrong answers are ranked by the chords shared bar by bar", func(t *testing.T) {
		added, err := chartOnly(t, "major", "I", "iii", "IV", "V").AddedToDrills(nodes, []domain.Progression{
			drillProgression("vi-IV-I-V"), // shares V
			drillProgression("I-V-vi-IV"), // shares I
			drillProgression("I-vi-IV-V"), // shares I, IV and V
			drillProgression("ii-iii-IV-I"),
		})
		require.NoError(t, err)
		assert.Equal(t, "I-vi-IV-V", added.Confusable[0])
		assert.Equal(t, "ii-iii-IV-I", added.Confusable[1])
	})

	t.Run("a tie goes to the key that sorts first, so the choice stays put", func(t *testing.T) {
		added, err := chartOnly(t, "major", "I", "iii", "IV", "V").AddedToDrills(nodes, []domain.Progression{
			drillProgression("I-V-vi-ii"), drillProgression("I-ii-vi-ii"),
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"I-V-vi-ii", "I-ii-vi-ii"}, added.Confusable)
	})

	t.Run("another mode is never a wrong answer, however many chords it shares", func(t *testing.T) {
		other := drillProgression("minor/I-iii-IV-vi")
		other.Mode = "minor"
		added, err := chartOnly(t, "major", "I", "iii", "IV", "V").AddedToDrills(nodes, []domain.Progression{other, drillProgression("vi-ii-vi-V")})
		require.NoError(t, err)
		assert.Equal(t, []string{"vi-ii-vi-V"}, added.Confusable)
	})

	t.Run("a minor progression enters the drills with minor wrong answers only", func(t *testing.T) {
		minor := drillProgression("minor/i-iv-v-i")
		minor.Mode = "minor"
		added, err := chartOnly(t, "minor", "i", "♭VII", "♭VI", "V").AddedToDrills(nodes, []domain.Progression{drillProgression("I-V-vi-IV"), minor})
		require.NoError(t, err)
		assert.True(t, added.InDrills)
		assert.Equal(t, []string{"minor/i-iv-v-i"}, added.Confusable)
	})

	t.Run("a progression no drill progression shares length and mode with enters with no wrong answer", func(t *testing.T) {
		added, err := chartOnly(t, "major", "I", "IV", "V").AddedToDrills(nodes, []domain.Progression{drillProgression("I-V-vi-IV")})
		require.NoError(t, err)
		assert.True(t, added.InDrills)
		assert.Empty(t, added.Confusable)
	})

	t.Run("two bars and sixteen bars enter the drills", func(t *testing.T) {
		for _, bars := range [][]string{{"I", "V"}, slicesRepeat([]string{"I", "IV", "V", "I"}, 4)} {
			added, err := chartOnly(t, "major", bars...).AddedToDrills(nodes, nil)
			require.NoError(t, err)
			assert.True(t, added.InDrills)
		}
	})

	t.Run("only a spelled twin to offer: it enters with no wrong answer", func(t *testing.T) {
		added, err := chartOnly(t, "major", "I", "♭VII", "IV", "V").AddedToDrills(nodes, []domain.Progression{drillProgression("major/I-#VI-IV-V")})
		require.NoError(t, err)
		assert.Empty(t, added.Confusable)
	})

	t.Run("only the same bar count can be a wrong answer", func(t *testing.T) {
		blues := drillProgression("12-bar-blues")
		blues.Bars = make([]string, 12)
		for i := range blues.Bars {
			blues.Bars[i] = "I7"
		}
		added, err := chartOnly(t, "major", "I", "iii", "IV", "V").AddedToDrills(nodes, []domain.Progression{blues, drillProgression("I-V-vi-IV")})
		require.NoError(t, err)
		assert.Equal(t, []string{"I-V-vi-IV"}, added.Confusable)
	})

	t.Run("no more wrong answers than a question offers", func(t *testing.T) {
		added, err := chartOnly(t, "major", "I", "iii", "IV", "V").AddedToDrills(nodes, []domain.Progression{
			drillProgression("I-V-vi-IV"), drillProgression("I-vi-IV-V"), drillProgression("vi-IV-I-V"),
			drillProgression("IV-V-I-I"), drillProgression("ii-V-I-I"),
		})
		require.NoError(t, err)
		assert.Len(t, added.Confusable, 3)
	})

	t.Run("a progression of more than four bars keeps one wrong answer, as its questions offer two", func(t *testing.T) {
		long := func(key, last string) domain.Progression {
			p := drillProgression(key)
			p.Bars = []string{"I", "vi", "IV", "V", "I", "vi", "IV", last}
			return p
		}
		added, err := chartOnly(t, "major", "I", "vi", "IV", "V", "I", "vi", "ii", "V").AddedToDrills(nodes, []domain.Progression{long("major/a", "I"), long("major/b", "V7")})
		require.NoError(t, err)
		assert.Len(t, added.Confusable, 1)
	})

	t.Run("a spelled twin is never a wrong answer", func(t *testing.T) {
		added, err := chartOnly(t, "major", "I", "♭VII", "IV", "V").AddedToDrills(nodes, []domain.Progression{
			drillProgression("major/I-#VI-IV-V"), drillProgression("I-V-vi-IV"),
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"I-V-vi-IV"}, added.Confusable)
	})

	t.Run("it isn't its own wrong answer", func(t *testing.T) {
		p := chartOnly(t, "major", "I", "iii", "IV", "V")
		added, err := p.AddedToDrills(nodes, []domain.Progression{{Key: p.Key, Mode: "major", Bars: p.Bars}, drillProgression("I-V-vi-IV")})
		require.NoError(t, err)
		assert.Equal(t, []string{"I-V-vi-IV"}, added.Confusable)
	})

	refusals := []struct {
		name   string
		p      domain.Progression
		drills []domain.Progression
		want   []domain.ProgressionNotAddableReason
	}{
		{
			name: "a single bar, which is a chord", p: chartOnly(t, "major", "I"),
			drills: []domain.Progression{drillProgression("I-V-vi-IV")},
			want:   []domain.ProgressionNotAddableReason{domain.ProgressionNotAddableUnsupportedBarCount},
		},
		{
			name: "a single minor bar", p: chartOnly(t, "minor", "i"),
			drills: nil,
			want:   []domain.ProgressionNotAddableReason{domain.ProgressionNotAddableUnsupportedBarCount},
		},
		{
			name: "every reason at once", p: func() domain.Progression { p := drillProgression("major/I"); return p }(),
			drills: nil,
			want: []domain.ProgressionNotAddableReason{
				domain.ProgressionNotAddableAlreadyInDrills, domain.ProgressionNotAddableUnsupportedBarCount,
			},
		},
		{
			name: "already in the drills", p: drillProgression("major/I-iii-IV-V"),
			drills: []domain.Progression{drillProgression("I-V-vi-IV")},
			want:   []domain.ProgressionNotAddableReason{domain.ProgressionNotAddableAlreadyInDrills},
		},
	}
	for _, tt := range refusals {
		t.Run("refused: "+tt.name, func(t *testing.T) {
			_, err := tt.p.AddedToDrills(nodes, tt.drills)
			var refused *domain.ProgressionNotAddableError
			require.ErrorAs(t, err, &refused)
			assert.Equal(t, tt.want, refused.Reasons)
		})
	}

	t.Run("twelve bars are accepted", func(t *testing.T) {
		bars := []string{"I", "I", "I", "I", "IV", "IV", "I", "I", "V", "IV", "I", "♭VII"}
		other := domain.Progression{Key: "12-bar-blues", Mode: "major", InDrills: true, Bars: []string{"I7", "I7", "I7", "I7", "IV7", "IV7", "I7", "I7", "V7", "IV7", "I7", "V7"}}
		added, err := chartOnly(t, "major", bars...).AddedToDrills(nodes, []domain.Progression{other})
		require.NoError(t, err)
		assert.Equal(t, []string{"12-bar-blues"}, added.Confusable)
	})
}

// A question's options never hold two progressions with one chart, even when a
// twin arrives through a partner's partner, since a partner's list was fixed
// before the twin existed or chosen without regard to this progression.
func TestAQuestionNeverOffersTwoProgressionsWithOneChart(t *testing.T) {
	twin := drillProgression("major/I-#VI-IV-V")
	partner := drillProgression("I-V-vi-IV", twin.Key, "vi-IV-I-V")
	asked := drillProgression("major/I-bVII-IV-V", partner.Key)
	catalog := []domain.Progression{asked, partner, twin, drillProgression("vi-IV-I-V")}
	planning := domain.DrillPlanning{Pick: firstPick, Reading: domain.GeneratedChordSymbols}

	values := func(q *domain.PlannedGeneratedChoice) []string {
		out := make([]string, len(q.Options))
		for i, o := range q.Options {
			out[i] = o.Value
		}
		return out
	}

	t.Run("a chart read as numbers", func(t *testing.T) {
		q, _ := plannedChoice(t, domain.ChartProgressionItem{Progression: asked, Catalog: catalog}, "chart_to_numbers", planning)
		assert.Subset(t, values(q), []string{asked.Key, partner.Key, "vi-IV-I-V"})
		assert.NotContains(t, values(q), twin.Key)
	})

	t.Run("numbers read as a chart", func(t *testing.T) {
		q, _ := plannedChoice(t, domain.ChartProgressionItem{Progression: asked, Catalog: catalog}, "numbers_to_chart", planning)
		assert.Len(t, q.Options, 4)
		seen := map[string]bool{}
		for _, v := range values(q) {
			assert.False(t, seen[v], "%q is offered twice", v)
			seen[v] = true
		}
	})

	t.Run("a progression by ear", func(t *testing.T) {
		q, _ := plannedChoice(t, domain.HeardProgressionItem{Progression: asked, Catalog: catalog}, "progression_by_number", planning)
		assert.Subset(t, values(q), []string{asked.Key, partner.Key, "vi-IV-I-V"})
		assert.NotContains(t, values(q), twin.Key)
	})
}
