//go:build integration

package bdd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

// generatedChoiceGoldenFile is golden/practice-graders/generated_choice.v1.json.
// The worker grades harmony answers by these cases; the planner here must
// compute the same right answers, or it would offer questions whose right
// answer the grader doesn't find among the options.
type generatedChoiceGoldenFile struct {
	Reference struct {
		Progressions map[string]struct {
			Bars []string `json:"bars"`
		} `json:"progressions"`
	} `json:"reference"`
	Cases []struct {
		Name     string `json:"name"`
		ItemKey  string `json:"item_key"`
		Expected struct {
			Result   string `json:"result"`
			Evidence struct {
				AnswerKey struct {
					Way      string `json:"way"`
					MusicKey string `json:"music_key"`
					Answer   string `json:"answer"`
				} `json:"answer_key"`
			} `json:"evidence"`
		} `json:"expected"`
	} `json:"cases"`
}

func TestGeneratedChoiceGoldenAnswers(t *testing.T) {
	specsDir := strings.TrimSuffix(featuresBase, "/features")
	raw, err := os.ReadFile(filepath.Join(specsDir, "golden", "practice-graders", "generated_choice.v1.json"))
	require.NoError(t, err, "the harmony planner needs the grader's golden cases in motifpath-specs")
	var golden generatedChoiceGoldenFile
	require.NoError(t, json.Unmarshal(raw, &golden))

	checked := 0
	for _, c := range golden.Cases {
		if c.Expected.Result != "graded" {
			continue
		}
		key := c.Expected.Evidence.AnswerKey
		parts := strings.Split(c.ItemKey, ":")
		var want string
		switch key.Way {
		case "numeral_to_chord", "function_to_chord":
			want = domain.NumeralChord(parts[1], parts[2])
		case "chord_to_function":
			want = string(domain.NumeralFunction(parts[2]))
		case "numbers_to_chart":
			want = strings.Join(domain.ProgressionChart(golden.Reference.Progressions[parts[1]].Bars, key.MusicKey), " ")
		default:
			continue
		}
		checked++
		t.Run(c.Name, func(t *testing.T) {
			assert.Equal(t, key.Answer, want, "the planner's right answer for %s as %s", c.ItemKey, key.Way)
		})
	}
	require.Positive(t, checked, "no golden case computed a harmony answer")
}
