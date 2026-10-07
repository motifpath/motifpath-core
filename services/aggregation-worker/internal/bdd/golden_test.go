//go:build integration

package bdd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
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
			Name               string `json:"name"`
			LayoutInstrumentID string `json:"layout_instrument_id"`
			ShapeFamily        string `json:"shape_family"`
			Shape              string `json:"shape"`
			Positions          []struct {
				String   int    `json:"string"`
				Fret     int    `json:"fret"`
				Interval string `json:"interval"`
			} `json:"positions"`
		} `json:"diagrams"`
		ShapeFamilies map[string]struct {
			Members []string `json:"members"`
		} `json:"shape_families"`
		Exercises map[string]struct {
			OptionIDs        []string `json:"option_ids"`
			CorrectOptionIDs []string `json:"correct_option_ids"`
		} `json:"exercises"`
		Instruments map[string]struct {
			Tuning []string `json:"tuning"`
		} `json:"instruments"`
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
				AnswerKey        *struct {
					String   *int   `json:"string"`
					Fret     *int   `json:"fret"`
					NoteName string `json:"note_name"`
					Options  []struct {
						OptionID  string `json:"option_id"`
						IsCorrect bool   `json:"is_correct"`
					} `json:"options"`
					ShapeFamily string `json:"shape_family"`
					Shape       string `json:"shape"`
					Interval    string `json:"interval"`
					Cells       []struct {
						String int `json:"string"`
						Fret   int `json:"fret"`
					} `json:"cells"`
				} `json:"answer_key"`
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

			ref := domain.PracticeReference{Diagrams: map[string]domain.DiagramReference{}, Exercises: map[string]domain.ExerciseReference{}, Instruments: map[string]domain.InstrumentReference{}}
			for id, d := range file.Reference.Diagrams {
				diagram := domain.DiagramReference{ID: id, LayoutInstrumentID: d.LayoutInstrumentID, ShapeFamily: d.ShapeFamily, Shape: d.Shape}
				if d.ShapeFamily != "" {
					diagram.FamilyMembers = file.Reference.ShapeFamilies[d.ShapeFamily].Members
				}
				for _, p := range d.Positions {
					diagram.Positions = append(diagram.Positions, domain.DiagramPosition{String: p.String, Fret: p.Fret, Interval: p.Interval})
				}
				ref.Diagrams[id] = diagram
			}
			for id, e := range file.Reference.Exercises {
				options := make([]domain.AnswerOption, len(e.OptionIDs))
				for i, o := range e.OptionIDs {
					options[i] = domain.AnswerOption{OptionID: o, IsCorrect: slices.Contains(e.CorrectOptionIDs, o)}
				}
				ref.Exercises[id] = domain.ExerciseReference{ID: id, OptionIDs: e.OptionIDs, CorrectOptionIDs: e.CorrectOptionIDs, Options: options}
			}
			for id, i := range file.Reference.Instruments {
				ref.Instruments[id] = domain.InstrumentReference{ID: id, Tuning: i.Tuning}
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
					if want.AnswerKey == nil {
						assert.Nil(t, got.Evidence.AnswerKey)
						return
					}
					require.NotNil(t, got.Evidence.AnswerKey, "auto-graded evidence keeps its answer key")
					assert.Equal(t, want.AnswerKey.String, got.Evidence.AnswerKey.String)
					assert.Equal(t, want.AnswerKey.Fret, got.Evidence.AnswerKey.Fret)
					assert.Equal(t, want.AnswerKey.NoteName, got.Evidence.AnswerKey.NoteName)
					require.Len(t, got.Evidence.AnswerKey.Options, len(want.AnswerKey.Options))
					for i, o := range want.AnswerKey.Options {
						assert.Equal(t, o.OptionID, got.Evidence.AnswerKey.Options[i].OptionID)
						assert.Equal(t, o.IsCorrect, got.Evidence.AnswerKey.Options[i].IsCorrect)
					}
					assert.Equal(t, want.AnswerKey.ShapeFamily, got.Evidence.AnswerKey.ShapeFamily)
					assert.Equal(t, want.AnswerKey.Shape, got.Evidence.AnswerKey.Shape)
					assert.Equal(t, want.AnswerKey.Interval, got.Evidence.AnswerKey.Interval)
					require.Len(t, got.Evidence.AnswerKey.Cells, len(want.AnswerKey.Cells))
					for i, c := range want.AnswerKey.Cells {
						assert.Equal(t, domain.AnswerCell{String: c.String, Fret: c.Fret}, got.Evidence.AnswerKey.Cells[i])
					}
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
		Shape            string   `json:"shape"`
		Interval         string   `json:"interval"`
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
		Shape:            w.Shape,
		Interval:         w.Interval,
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
