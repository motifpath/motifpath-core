package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func modePtr(m domain.DiagramMode) *domain.DiagramMode { return &m }

func step(value domain.NoteValue, ids ...string) domain.SequenceStep {
	return domain.SequenceStep{PositionIDs: ids, Value: value}
}

var (
	eighth  = domain.NoteValue{Num: 1, Den: 8}
	quarter = domain.NoteValue{Num: 1, Den: 4}
)

func newSequencedDiagram(opts domain.DiagramOptions) (domain.Diagram, error) {
	instrument, err := domain.NewInstrument("guitar", map[string]string{"en": "Guitar"}, []string{"en"}, domain.InstrumentFamilyFretted, intPtr(6), []string{"E2", "A2", "D3", "G3", "B3", "E4"}, nil, guitarVoice)
	if err != nil {
		return domain.Diagram{}, err
	}
	positions := []domain.Position{
		fretted("p1", "R", "A", 6, 5),
		fretted("p2", "b3", "C", 6, 8),
		fretted("p3", "4", "D", 5, 5),
	}
	return domain.NewDiagram("diagram-1", "user-1", instrument, namesOf("Lick"), offered, positions, []string{"s"}, []string{"c"}, opts, time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC))
}

func requireFieldError(t *testing.T, err error, field string) {
	t.Helper()
	var valErr *domain.ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, field, valErr.Fields[0].Field)
}

func TestNewDiagram_Mode(t *testing.T) {
	tests := []struct {
		name      string
		rootNote  *string
		mode      *domain.DiagramMode
		wantField string // empty means the constructor must succeed
	}{
		{name: "a root note with a mode names the key", rootNote: strPtr("A"), mode: modePtr(domain.DiagramModeMinor)},
		{name: "every church mode is accepted", rootNote: strPtr("D"), mode: modePtr(domain.DiagramModeDorian)},
		{name: "no mode means no key", rootNote: strPtr("A")},
		{name: "a mode without a root note is rejected", mode: modePtr(domain.DiagramModeMinor), wantField: "mode"},
		{name: "an unrecognised mode is rejected", rootNote: strPtr("A"), mode: modePtr("harmonic"), wantField: "mode"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newSequencedDiagram(domain.DiagramOptions{RootNote: tt.rootNote, Mode: tt.mode})

			if tt.wantField != "" {
				requireFieldError(t, err, tt.wantField)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.mode, got.Mode)
		})
	}
}

func TestNewDiagram_Sequence(t *testing.T) {
	t.Run("without a sequence the diagram has no playback and a 4/4 time signature", func(t *testing.T) {
		got, err := newSequencedDiagram(domain.DiagramOptions{})

		require.NoError(t, err)
		assert.Empty(t, got.Sequence)
		assert.Nil(t, got.TempoBPM)
		assert.Equal(t, domain.TimeSignature{Beats: 4, BeatValue: 4}, got.TimeSignature)
	})

	t.Run("single notes, chords, repeats, rests and tuplets are kept as given", func(t *testing.T) {
		sequence := []domain.SequenceStep{
			step(eighth, "p1"),
			{PositionIDs: []string{"p1", "p2", "p3"}, Value: quarter, Strum: domain.StrumDown},
			step(domain.NoteValue{Num: 1, Den: 12}, "p2"),
			step(domain.NoteValue{Num: 1, Den: 12}, "p1"),
			step(domain.NoteValue{Num: 1, Den: 12}, "p3"),
			step(quarter),
		}

		got, err := newSequencedDiagram(domain.DiagramOptions{TempoBPM: intPtr(90), TimeSignature: domain.TimeSignature{Beats: 6, BeatValue: 8}, Sequence: sequence})

		require.NoError(t, err)
		require.Len(t, got.Sequence, 6)
		assert.Equal(t, domain.StrumNone, got.Sequence[0].Strum, "an unset strum defaults to none")
		assert.Equal(t, domain.StrumDown, got.Sequence[1].Strum)
		assert.Equal(t, []string{"p1", "p2", "p3"}, got.Sequence[1].PositionIDs)
		assert.Equal(t, domain.NoteValue{Num: 1, Den: 12}, got.Sequence[2].Value)
		assert.Empty(t, got.Sequence[5].PositionIDs, "a step with no positions is a rest")
		assert.Equal(t, domain.TimeSignature{Beats: 6, BeatValue: 8}, got.TimeSignature)
		require.NotNil(t, got.TempoBPM)
		assert.Equal(t, 90, *got.TempoBPM)
		assert.Equal(t, domain.Strum(""), sequence[0].Strum, "the caller's steps are untouched by normalization")
	})

	tooMany := make([]domain.SequenceStep, domain.MaxSequenceSteps+1)
	for i := range tooMany {
		tooMany[i] = step(eighth, "p1")
	}
	tests := []struct {
		name      string
		opts      domain.DiagramOptions
		wantField string
	}{
		{name: "a step naming a position not in the diagram",
			opts: domain.DiagramOptions{TempoBPM: intPtr(90), Sequence: []domain.SequenceStep{step(eighth, "p9")}}, wantField: "sequence"},
		{name: "a step naming the same position twice",
			opts: domain.DiagramOptions{TempoBPM: intPtr(90), Sequence: []domain.SequenceStep{step(eighth, "p1", "p1")}}, wantField: "sequence"},
		{name: "a note value of 0/4",
			opts: domain.DiagramOptions{TempoBPM: intPtr(90), Sequence: []domain.SequenceStep{step(domain.NoteValue{Num: 0, Den: 4}, "p1")}}, wantField: "sequence"},
		{name: "a note value of 1/0",
			opts: domain.DiagramOptions{TempoBPM: intPtr(90), Sequence: []domain.SequenceStep{step(domain.NoteValue{Num: 1, Den: 0}, "p1")}}, wantField: "sequence"},
		{name: "a note value numerator above 64",
			opts: domain.DiagramOptions{TempoBPM: intPtr(90), Sequence: []domain.SequenceStep{step(domain.NoteValue{Num: 65, Den: 4}, "p1")}}, wantField: "sequence"},
		{name: "a note value denominator above 128",
			opts: domain.DiagramOptions{TempoBPM: intPtr(90), Sequence: []domain.SequenceStep{step(domain.NoteValue{Num: 1, Den: 129}, "p1")}}, wantField: "sequence"},
		{name: "an unrecognised strum",
			opts: domain.DiagramOptions{TempoBPM: intPtr(90), Sequence: []domain.SequenceStep{{PositionIDs: []string{"p1"}, Value: eighth, Strum: "sideways"}}}, wantField: "sequence"},
		{name: "more steps than allowed",
			opts: domain.DiagramOptions{TempoBPM: intPtr(90), Sequence: tooMany}, wantField: "sequence"},
		{name: "a sequence and no tempo",
			opts: domain.DiagramOptions{Sequence: []domain.SequenceStep{step(eighth, "p1")}}, wantField: "tempo_bpm"},
		{name: "a tempo and an empty sequence",
			opts: domain.DiagramOptions{TempoBPM: intPtr(90)}, wantField: "tempo_bpm"},
		{name: "a sequence at 19 BPM",
			opts: domain.DiagramOptions{TempoBPM: intPtr(19), Sequence: []domain.SequenceStep{step(eighth, "p1")}}, wantField: "tempo_bpm"},
		{name: "a sequence at 301 BPM",
			opts: domain.DiagramOptions{TempoBPM: intPtr(301), Sequence: []domain.SequenceStep{step(eighth, "p1")}}, wantField: "tempo_bpm"},
		{name: "a time signature of 4/3",
			opts: domain.DiagramOptions{TimeSignature: domain.TimeSignature{Beats: 4, BeatValue: 3}}, wantField: "time_signature"},
		{name: "a time signature of 17/4",
			opts: domain.DiagramOptions{TimeSignature: domain.TimeSignature{Beats: 17, BeatValue: 4}}, wantField: "time_signature"},
		{name: "a time signature of 0/4",
			opts: domain.DiagramOptions{TimeSignature: domain.TimeSignature{Beats: 0, BeatValue: 4}}, wantField: "time_signature"},
	}
	for _, tt := range tests {
		t.Run(tt.name+" is rejected", func(t *testing.T) {
			_, err := newSequencedDiagram(tt.opts)

			requireFieldError(t, err, tt.wantField)
		})
	}

	t.Run("the boundary tempos and the longest sequence are accepted", func(t *testing.T) {
		longest := tooMany[:domain.MaxSequenceSteps]
		for _, bpm := range []int{20, 300} {
			_, err := newSequencedDiagram(domain.DiagramOptions{TempoBPM: intPtr(bpm), Sequence: longest})
			require.NoError(t, err, "at %d BPM", bpm)
		}
	})
}

func TestParsePitch(t *testing.T) {
	tests := []struct {
		pitch    string
		wantMIDI int
		wantOK   bool
	}{
		{pitch: "C4", wantMIDI: 60, wantOK: true},
		{pitch: "E2", wantMIDI: 40, wantOK: true},
		{pitch: "F#3", wantMIDI: 54, wantOK: true},
		{pitch: "Bb4", wantMIDI: 70, wantOK: true},
		{pitch: "Cbb4", wantMIDI: 58, wantOK: true},
		{pitch: "C-1", wantMIDI: 0, wantOK: true},
		{pitch: "G9", wantMIDI: 127, wantOK: true},
		{pitch: "G#9", wantOK: false},
		{pitch: "Cb-1", wantOK: false},
		{pitch: "E", wantOK: false},
		{pitch: "H2", wantOK: false},
		{pitch: "e2", wantOK: false},
		{pitch: "E10", wantOK: false},
		{pitch: "", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.pitch, func(t *testing.T) {
			got, ok := domain.ParsePitch(tt.pitch)

			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantMIDI, got)
			}
		})
	}
}
