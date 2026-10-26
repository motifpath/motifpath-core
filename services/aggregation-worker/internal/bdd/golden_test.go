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
		Progressions map[string]struct {
			Bars []string `json:"bars"`
		} `json:"progressions"`
		SongCharts map[string]struct {
			Name string `json:"name"`
		} `json:"song_charts"`
	} `json:"reference"`
	Cases []struct {
		Name     string          `json:"name"`
		ItemKey  string          `json:"item_key"`
		Response json.RawMessage `json:"response"`
		Expected struct {
			Result         string         `json:"result"`
			Reason         string         `json:"reason"`
			Evidence       goldenEvidence `json:"evidence"`
			EvidenceByItem []struct {
				ItemKey  string         `json:"item_key"`
				Evidence goldenEvidence `json:"evidence"`
			} `json:"evidence_by_item"`
		} `json:"expected"`
	} `json:"cases"`
}

// goldenEvidence is the evidence a golden case expects a grader to conclude.
type goldenEvidence struct {
	Source           string  `json:"source"`
	Correct          *bool   `json:"correct"`
	LatencyMs        *int    `json:"latency_ms"`
	AudioMs          *int    `json:"audio_ms"`
	Rating           string  `json:"rating"`
	TempoBPM         *int    `json:"tempo_bpm"`
	NoteValue        *string `json:"note_value"`
	ChangesPerMinute *int    `json:"changes_per_minute"`
	MusicKey         string  `json:"music_key"`
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
		Choices    []string `json:"choices"`
		FretWindow *struct {
			FromFret int `json:"from_fret"`
		} `json:"fret_window"`
		Way      string   `json:"way"`
		MusicKey string   `json:"music_key"`
		Answer   string   `json:"answer"`
		Offered  []string `json:"offered"`
		Tone     string   `json:"tone"`
		Bars     []struct {
			Chord    string `json:"chord"`
			NoteName string `json:"note_name"`
			Hit      bool   `json:"hit"`
		} `json:"bars"`
	} `json:"answer_key"`
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

			ref := domain.PracticeReference{Diagrams: map[string]domain.DiagramReference{}, Exercises: map[string]domain.ExerciseReference{},
				Instruments: map[string]domain.InstrumentReference{}, Progressions: map[string]domain.ProgressionReference{},
				SongCharts: map[string]domain.SongChartReference{}}
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
			for key, p := range file.Reference.Progressions {
				ref.Progressions[key] = domain.ProgressionReference{Key: key, Bars: p.Bars}
			}
			for id := range file.Reference.SongCharts {
				ref.SongCharts[id] = domain.SongChartReference{ID: id}
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
					if c.Expected.EvidenceByItem != nil {
						require.Len(t, got.EvidenceByItem, len(c.Expected.EvidenceByItem))
						for i, want := range c.Expected.EvidenceByItem {
							assert.Equal(t, want.ItemKey, got.EvidenceByItem[i].ItemKey)
							assertGoldenEvidence(t, want.Evidence, got.EvidenceByItem[i].Evidence)
						}
						return
					}
					assert.Empty(t, got.EvidenceByItem, "an answer about one item is graded on that item")
					assertGoldenEvidence(t, c.Expected.Evidence, got.Evidence)
				})
			}
		})
	}
}

// assertGoldenEvidence checks one piece of graded evidence against a golden case.
func assertGoldenEvidence(t *testing.T, want goldenEvidence, got domain.GradedEvidence) {
	t.Helper()
	assert.Equal(t, domain.EvidenceSource(want.Source), got.Source)
	assert.Equal(t, want.Correct, got.Correct)
	assert.Equal(t, want.LatencyMs, got.LatencyMs)
	assert.Equal(t, want.AudioMs, got.AudioMs)
	assert.Equal(t, domain.SelfRating(want.Rating), got.Rating)
	assert.Equal(t, want.TempoBPM, got.TempoBPM)
	assert.Equal(t, noteValueOf(want.NoteValue), got.NoteValue)
	assert.Equal(t, want.ChangesPerMinute, got.ChangesPerMinute)
	assert.Equal(t, want.MusicKey, got.MusicKey)
	if want.AnswerKey == nil {
		assert.Nil(t, got.AnswerKey)
		return
	}
	require.NotNil(t, got.AnswerKey, "auto-graded evidence keeps its answer key")
	assert.Equal(t, want.AnswerKey.String, got.AnswerKey.String)
	assert.Equal(t, want.AnswerKey.Fret, got.AnswerKey.Fret)
	assert.Equal(t, want.AnswerKey.NoteName, got.AnswerKey.NoteName)
	require.Len(t, got.AnswerKey.Options, len(want.AnswerKey.Options))
	for i, o := range want.AnswerKey.Options {
		assert.Equal(t, o.OptionID, got.AnswerKey.Options[i].OptionID)
		assert.Equal(t, o.IsCorrect, got.AnswerKey.Options[i].IsCorrect)
	}
	assert.Equal(t, want.AnswerKey.ShapeFamily, got.AnswerKey.ShapeFamily)
	assert.Equal(t, want.AnswerKey.Shape, got.AnswerKey.Shape)
	assert.Equal(t, want.AnswerKey.Interval, got.AnswerKey.Interval)
	require.Len(t, got.AnswerKey.Cells, len(want.AnswerKey.Cells))
	for i, c := range want.AnswerKey.Cells {
		assert.Equal(t, domain.AnswerCell{String: c.String, Fret: c.Fret}, got.AnswerKey.Cells[i])
	}
	assert.Equal(t, want.AnswerKey.Choices, got.AnswerKey.Choices)
	if want.AnswerKey.FretWindow == nil {
		assert.Nil(t, got.AnswerKey.FretWindowFrom)
	} else {
		assert.Equal(t, &want.AnswerKey.FretWindow.FromFret, got.AnswerKey.FretWindowFrom)
	}
	assert.Equal(t, want.AnswerKey.Way, got.AnswerKey.Way)
	assert.Equal(t, want.AnswerKey.MusicKey, got.AnswerKey.MusicKey)
	assert.Equal(t, want.AnswerKey.Answer, got.AnswerKey.Answer)
	assert.Equal(t, want.AnswerKey.Offered, got.AnswerKey.Offered)
	assert.Equal(t, want.AnswerKey.Tone, got.AnswerKey.Tone)
	require.Len(t, got.AnswerKey.Bars, len(want.AnswerKey.Bars))
	for i, b := range want.AnswerKey.Bars {
		assert.Equal(t, domain.AnswerBar{Chord: b.Chord, NoteName: b.NoteName, Hit: b.Hit}, got.AnswerKey.Bars[i])
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
		NoteValue        *string  `json:"note_value"`
		ChangesPerMinute *int     `json:"changes_per_minute"`
		Choices          []string `json:"choices"`
		Reason           string   `json:"reason"`
		Way              string   `json:"way"`
		MusicKey         string   `json:"music_key"`
		Options          []string `json:"options"`
		Choice           string   `json:"choice"`
		Tone             string   `json:"tone"`
		Taps             []struct {
			AtMs   int `json:"at_ms"`
			String int `json:"string"`
			Fret   int `json:"fret"`
		} `json:"taps"`
		FretWindow *struct {
			FromFret int `json:"from_fret"`
		} `json:"fret_window"`
		Cells []struct {
			String int `json:"string"`
			Fret   int `json:"fret"`
		} `json:"cells"`
	}
	if err := json.Unmarshal(raw, &w); err != nil {
		return domain.PracticeResponse{}, err
	}
	r := domain.PracticeResponse{
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
		NoteValue:        noteValueOf(w.NoteValue),
		ChangesPerMinute: w.ChangesPerMinute,
		Choices:          w.Choices,
		Reason:           domain.NotAnsweredReason(w.Reason),
		Way:              w.Way,
		MusicKey:         w.MusicKey,
		Options:          w.Options,
		Choice:           w.Choice,
		Tone:             w.Tone,
	}
	for _, t := range w.Taps {
		r.Taps = append(r.Taps, domain.TimedTap{AtMs: t.AtMs, String: t.String, Fret: t.Fret})
	}
	if w.FretWindow != nil {
		r.FretWindowFrom = &w.FretWindow.FromFret
	}
	for _, c := range w.Cells {
		r.Cells = append(r.Cells, domain.AnswerCell{String: c.String, Fret: c.Fret})
	}
	return r, nil
}

// noteValueOf is a golden case's note value, or nil when the case has none.
func noteValueOf(s *string) *domain.NoteValue {
	if s == nil {
		return nil
	}
	v := domain.NoteValue(*s)
	return &v
}
