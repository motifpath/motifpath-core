//go:build integration

package bdd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

// goldenCaseFile is one golden/practice-graders/<grader>.v<n>.json file. The cases
// are shared with the web client's instant-feedback graders, so a student never
// sees feedback the record contradicts.
type goldenCaseFile struct {
	Grader    string `json:"grader"`
	Reference struct {
		Diagrams map[string]struct {
			Name string `json:"name"`
		} `json:"diagrams"`
		Exercises map[string]struct {
			OptionIDs        []string `json:"option_ids"`
			CorrectOptionIDs []string `json:"correct_option_ids"`
		} `json:"exercises"`
	} `json:"reference"`
	Cases []struct {
		Name     string          `json:"name"`
		ItemKey  string          `json:"item_key"`
		Response json.RawMessage `json:"response"`
		Expected struct {
			Result   string `json:"result"`
			Reason   string `json:"reason"`
			Evidence struct {
				Source           string `json:"source"`
				Correct          *bool  `json:"correct"`
				LatencyMs        *int   `json:"latency_ms"`
				AudioMs          *int   `json:"audio_ms"`
				Rating           string `json:"rating"`
				TempoBPM         *int   `json:"tempo_bpm"`
				ChangesPerMinute *int   `json:"changes_per_minute"`
			} `json:"evidence"`
		} `json:"expected"`
	} `json:"cases"`
}

// TestGraderGoldenCases runs every golden case of each grader this worker has.
// It lives with the BDD suite because both read the sibling motifpath-specs
// checkout, which CI provides only to the BDD job.
func TestGraderGoldenCases(t *testing.T) {
	for _, grader := range domain.Graders() {
		t.Run(grader.ID(), func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(specsDir, "golden", "practice-graders", grader.ID()+".json"))
			require.NoError(t, err, "every grader needs its golden cases in motifpath-specs")

			var file goldenCaseFile
			require.NoError(t, json.Unmarshal(raw, &file))
			require.Equal(t, grader.ID(), file.Grader)

			ref := domain.PracticeReference{Diagrams: map[string]domain.DiagramReference{}, Exercises: map[string]domain.ExerciseReference{}}
			for id := range file.Reference.Diagrams {
				ref.Diagrams[id] = domain.DiagramReference{ID: id}
			}
			for id, e := range file.Reference.Exercises {
				ref.Exercises[id] = domain.ExerciseReference{ID: id, OptionIDs: e.OptionIDs, CorrectOptionIDs: e.CorrectOptionIDs}
			}

			for _, c := range file.Cases {
				t.Run(c.Name, func(t *testing.T) {
					key, err := domain.ParsePracticeItemKey(c.ItemKey)
					require.NoError(t, err)
					response, err := decodeGoldenResponse(c.Response)
					require.NoError(t, err)

					got := grader.Grade(key, response, ref)

					if c.Expected.Result == "rejected" {
						assert.Equal(t, domain.GradeRejection(c.Expected.Reason), got.Rejection)
						return
					}
					require.Empty(t, got.Rejection)
					want := c.Expected.Evidence
					assert.Equal(t, domain.EvidenceSource(want.Source), got.Evidence.Source)
					assert.Equal(t, want.Correct, got.Evidence.Correct)
					assert.Equal(t, want.LatencyMs, got.Evidence.LatencyMs)
					assert.Equal(t, want.AudioMs, got.Evidence.AudioMs)
					assert.Equal(t, domain.SelfRating(want.Rating), got.Evidence.Rating)
					assert.Equal(t, want.TempoBPM, got.Evidence.TempoBPM)
					assert.Equal(t, want.ChangesPerMinute, got.Evidence.ChangesPerMinute)
				})
			}
		})
	}
}

// decodeGoldenResponse reads a case's raw response the way the Kafka adapter
// reads one off the wire.
func decodeGoldenResponse(raw json.RawMessage) (domain.PracticeResponse, error) {
	var w struct {
		ResponseType     string   `json:"response_type"`
		NoteName         string   `json:"note_name"`
		String           *int     `json:"string"`
		Fret             *int     `json:"fret"`
		OptionIDs        []string `json:"option_ids"`
		LatencyMs        *int     `json:"latency_ms"`
		AudioMs          *int     `json:"audio_ms"`
		Rating           string   `json:"rating"`
		TempoBPM         *int     `json:"tempo_bpm"`
		ChangesPerMinute *int     `json:"changes_per_minute"`
	}
	if err := json.Unmarshal(raw, &w); err != nil {
		return domain.PracticeResponse{}, err
	}
	return domain.PracticeResponse{
		Type:             domain.PracticeResponseType(w.ResponseType),
		NoteName:         w.NoteName,
		String:           w.String,
		Fret:             w.Fret,
		OptionIDs:        w.OptionIDs,
		LatencyMs:        w.LatencyMs,
		AudioMs:          w.AudioMs,
		Rating:           domain.SelfRating(w.Rating),
		TempoBPM:         w.TempoBPM,
		ChangesPerMinute: w.ChangesPerMinute,
	}, nil
}
