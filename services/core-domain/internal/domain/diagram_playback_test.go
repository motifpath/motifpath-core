package domain_test

import (
	"fmt"
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
	return newSequencedDiagramNamed(namesOf("Lick"), opts)
}

// newSequencedDiagramNamed is newSequencedDiagram with the given names.
func newSequencedDiagramNamed(names map[string]string, opts domain.DiagramOptions) (domain.Diagram, error) {
	instrument, err := domain.NewInstrument("guitar", map[string]string{"en": "Guitar"}, []string{"en"}, domain.InstrumentFamilyFretted, intPtr(6), []string{"E2", "A2", "D3", "G3", "B3", "E4"}, nil, guitarVoice)
	if err != nil {
		return domain.Diagram{}, err
	}
	positions := []domain.Position{
		fretted("p1", "R", "A", 6, 5),
		fretted("p2", "b3", "C", 6, 8),
		fretted("p3", "4", "D", 5, 5),
	}
	return domain.NewDiagram("diagram-1", "user-1", instrument, names, offered, positions, []string{"s"}, []string{"c"}, opts, time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC))
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

// playbackOf is a playback with the given id and English name, at 90 BPM,
// of one eighth note on each of ids.
func playbackOf(id, name string, ids ...string) domain.DiagramPlayback {
	steps := make([]domain.SequenceStep, len(ids))
	for i, positionID := range ids {
		steps[i] = step(eighth, positionID)
	}
	return domain.DiagramPlayback{ID: id, Names: domain.LocalizedText{"en": name}, TempoBPM: 90, Steps: steps}
}

func TestNewDiagram_Playbacks(t *testing.T) {
	t.Run("without playbacks the diagram doesn't play and has no default", func(t *testing.T) {
		got, err := newSequencedDiagram(domain.DiagramOptions{})

		require.NoError(t, err)
		assert.Empty(t, got.Playbacks)
		assert.Nil(t, got.DefaultPlaybackID)
	})

	t.Run("single notes, chords, repeats, rests and tuplets are kept as given", func(t *testing.T) {
		steps := []domain.SequenceStep{
			step(eighth, "p1"),
			{PositionIDs: []string{"p1", "p2", "p3"}, Value: quarter, Strum: domain.StrumDown},
			step(domain.NoteValue{Num: 1, Den: 12}, "p2"),
			step(domain.NoteValue{Num: 1, Den: 12}, "p1"),
			step(domain.NoteValue{Num: 1, Den: 12}, "p3"),
			step(quarter),
		}
		playback := domain.DiagramPlayback{ID: "pb-1", Names: namesOf("Lick"), TempoBPM: 90, TimeSignature: domain.TimeSignature{Beats: 6, BeatValue: 8}, Steps: steps}

		got, err := newSequencedDiagram(domain.DiagramOptions{Playbacks: []domain.DiagramPlayback{playback}})

		require.NoError(t, err)
		require.Len(t, got.Playbacks, 1)
		played := got.Playbacks[0]
		require.Len(t, played.Steps, 6)
		assert.Equal(t, domain.StrumNone, played.Steps[0].Strum, "an unset strum defaults to none")
		assert.Equal(t, domain.StrumDown, played.Steps[1].Strum)
		assert.Equal(t, []string{"p1", "p2", "p3"}, played.Steps[1].PositionIDs)
		assert.Equal(t, domain.NoteValue{Num: 1, Den: 12}, played.Steps[2].Value)
		assert.Empty(t, played.Steps[5].PositionIDs, "a step with no positions is a rest")
		assert.Equal(t, domain.TimeSignature{Beats: 6, BeatValue: 8}, played.TimeSignature)
		assert.Equal(t, 90, played.TempoBPM)
		assert.Equal(t, domain.Strum(""), steps[0].Strum, "the caller's steps are untouched by normalization")
	})

	t.Run("a playback without a time signature is in 4/4", func(t *testing.T) {
		got, err := newSequencedDiagram(domain.DiagramOptions{Playbacks: []domain.DiagramPlayback{playbackOf("pb-1", "Run", "p1")}})

		require.NoError(t, err)
		assert.Equal(t, domain.TimeSignature{Beats: 4, BeatValue: 4}, got.Playbacks[0].TimeSignature)
	})

	t.Run("playbacks keep their order and their names are trimmed", func(t *testing.T) {
		strum := playbackOf("pb-strum", "Strum", "p1")
		strum.Names = domain.LocalizedText{"en": "  Strum "}

		got, err := newSequencedDiagram(domain.DiagramOptions{Playbacks: []domain.DiagramPlayback{strum, playbackOf("pb-arp", "Arpeggio", "p1", "p2")}})

		require.NoError(t, err)
		require.Len(t, got.Playbacks, 2)
		assert.Equal(t, "pb-strum", got.Playbacks[0].ID)
		assert.Equal(t, domain.LocalizedText{"en": "Strum"}, got.Playbacks[0].Names)
		assert.Equal(t, "pb-arp", got.Playbacks[1].ID)
	})

	t.Run("the first playback is the default when none is chosen", func(t *testing.T) {
		got, err := newSequencedDiagram(domain.DiagramOptions{Playbacks: []domain.DiagramPlayback{playbackOf("pb-strum", "Strum", "p1"), playbackOf("pb-arp", "Arpeggio", "p2")}})

		require.NoError(t, err)
		require.NotNil(t, got.DefaultPlaybackID)
		assert.Equal(t, "pb-strum", *got.DefaultPlaybackID)
	})

	t.Run("a chosen default is kept", func(t *testing.T) {
		got, err := newSequencedDiagram(domain.DiagramOptions{
			Playbacks:         []domain.DiagramPlayback{playbackOf("pb-strum", "Strum", "p1"), playbackOf("pb-arp", "Arpeggio", "p2")},
			DefaultPlaybackID: strPtr("pb-arp"),
		})

		require.NoError(t, err)
		require.NotNil(t, got.DefaultPlaybackID)
		assert.Equal(t, "pb-arp", *got.DefaultPlaybackID)
	})

	t.Run("the boundary tempos, the longest playback and the most playbacks are accepted", func(t *testing.T) {
		longest := playbackOf("pb-long", "Long")
		longest.Steps = make([]domain.SequenceStep, domain.MaxSequenceSteps)
		for i := range longest.Steps {
			longest.Steps[i] = step(eighth, "p1")
		}
		slowest, fastest := playbackOf("pb-slow", "Slow", "p1"), playbackOf("pb-fast", "Fast", "p1")
		slowest.TempoBPM, fastest.TempoBPM = 20, 300
		most := []domain.DiagramPlayback{longest, slowest, fastest}
		for i := len(most); i < domain.MaxDiagramPlaybacks; i++ {
			most = append(most, playbackOf(fmt.Sprintf("pb-%d", i), fmt.Sprintf("Take %d", i), "p1"))
		}

		got, err := newSequencedDiagram(domain.DiagramOptions{Playbacks: most})

		require.NoError(t, err)
		assert.Len(t, got.Playbacks, domain.MaxDiagramPlaybacks)
	})

	withPlayback := func(change func(p *domain.DiagramPlayback)) domain.DiagramOptions {
		p := playbackOf("pb-1", "Run", "p1")
		change(&p)
		return domain.DiagramOptions{Playbacks: []domain.DiagramPlayback{p}}
	}
	withStep := func(s domain.SequenceStep) domain.DiagramOptions {
		return withPlayback(func(p *domain.DiagramPlayback) { p.Steps = []domain.SequenceStep{s} })
	}
	tooManySteps := make([]domain.SequenceStep, domain.MaxSequenceSteps+1)
	for i := range tooManySteps {
		tooManySteps[i] = step(eighth, "p1")
	}
	tooManyPlaybacks := make([]domain.DiagramPlayback, domain.MaxDiagramPlaybacks+1)
	for i := range tooManyPlaybacks {
		tooManyPlaybacks[i] = playbackOf(fmt.Sprintf("pb-%d", i), fmt.Sprintf("Take %d", i), "p1")
	}
	tests := []struct {
		name      string
		opts      domain.DiagramOptions
		wantField string
	}{
		{name: "a step naming a position not in the diagram", opts: withStep(step(eighth, "p9")), wantField: "playbacks"},
		{name: "a step naming the same position twice", opts: withStep(step(eighth, "p1", "p1")), wantField: "playbacks"},
		{name: "a note value of 0/4", opts: withStep(step(domain.NoteValue{Num: 0, Den: 4}, "p1")), wantField: "playbacks"},
		{name: "a note value of 1/0", opts: withStep(step(domain.NoteValue{Num: 1, Den: 0}, "p1")), wantField: "playbacks"},
		{name: "a note value numerator above 64", opts: withStep(step(domain.NoteValue{Num: 65, Den: 4}, "p1")), wantField: "playbacks"},
		{name: "a note value denominator above 128", opts: withStep(step(domain.NoteValue{Num: 1, Den: 129}, "p1")), wantField: "playbacks"},
		{name: "an unrecognised strum", opts: withStep(domain.SequenceStep{PositionIDs: []string{"p1"}, Value: eighth, Strum: "sideways"}), wantField: "playbacks"},
		{name: "a playback with no steps", opts: withPlayback(func(p *domain.DiagramPlayback) { p.Steps = nil }), wantField: "playbacks"},
		{name: "a playback with more steps than allowed", opts: withPlayback(func(p *domain.DiagramPlayback) { p.Steps = tooManySteps }), wantField: "playbacks"},
		{name: "a playback with no tempo", opts: withPlayback(func(p *domain.DiagramPlayback) { p.TempoBPM = 0 }), wantField: "playbacks"},
		{name: "a playback at 19 BPM", opts: withPlayback(func(p *domain.DiagramPlayback) { p.TempoBPM = 19 }), wantField: "playbacks"},
		{name: "a playback at 301 BPM", opts: withPlayback(func(p *domain.DiagramPlayback) { p.TempoBPM = 301 }), wantField: "playbacks"},
		{name: "a time signature of 4/3", opts: withPlayback(func(p *domain.DiagramPlayback) { p.TimeSignature = domain.TimeSignature{Beats: 4, BeatValue: 3} }), wantField: "playbacks"},
		{name: "a time signature of 17/4", opts: withPlayback(func(p *domain.DiagramPlayback) { p.TimeSignature = domain.TimeSignature{Beats: 17, BeatValue: 4} }), wantField: "playbacks"},
		{name: "a time signature of 0/4", opts: withPlayback(func(p *domain.DiagramPlayback) { p.TimeSignature = domain.TimeSignature{Beats: 0, BeatValue: 4} }), wantField: "playbacks"},
		{name: "a playback without an id", opts: withPlayback(func(p *domain.DiagramPlayback) { p.ID = "" }), wantField: "playbacks"},
		{name: "two playbacks with the same id",
			opts: domain.DiagramOptions{Playbacks: []domain.DiagramPlayback{playbackOf("pb-1", "Strum", "p1"), playbackOf("pb-1", "Arpeggio", "p2")}}, wantField: "playbacks"},
		{name: "two playbacks with the same name in one language",
			opts: domain.DiagramOptions{Playbacks: []domain.DiagramPlayback{playbackOf("pb-1", "Strum", "p1"), playbackOf("pb-2", "Strum", "p2")}}, wantField: "playbacks"},
		{name: "a playback named in a language the diagram isn't",
			opts: withPlayback(func(p *domain.DiagramPlayback) {
				p.Names = domain.LocalizedText{"en": "Run", "pt_BR": "Corrida"}
			}), wantField: "playbacks"},
		{name: "a playback with a blank name", opts: withPlayback(func(p *domain.DiagramPlayback) { p.Names = domain.LocalizedText{"en": " "} }), wantField: "playbacks"},
		{name: "more playbacks than allowed", opts: domain.DiagramOptions{Playbacks: tooManyPlaybacks}, wantField: "playbacks"},
		{name: "a default that is none of the playbacks",
			opts: domain.DiagramOptions{Playbacks: []domain.DiagramPlayback{playbackOf("pb-1", "Run", "p1")}, DefaultPlaybackID: strPtr("pb-9")}, wantField: "default_playback_id"},
		{name: "a default and no playbacks", opts: domain.DiagramOptions{DefaultPlaybackID: strPtr("pb-1")}, wantField: "default_playback_id"},
	}
	for _, tt := range tests {
		t.Run(tt.name+" is rejected", func(t *testing.T) {
			_, err := newSequencedDiagram(tt.opts)

			requireFieldError(t, err, tt.wantField)
		})
	}

	t.Run("a playback named in fewer languages than the diagram is rejected", func(t *testing.T) {
		_, err := newSequencedDiagramNamed(map[string]string{"en": "Lick", "pt_BR": "Frase"}, withPlayback(func(*domain.DiagramPlayback) {}))

		requireFieldError(t, err, "playbacks")
	})

	t.Run("playbacks are named in every language of the diagram", func(t *testing.T) {
		playback := playbackOf("pb-1", "Strum", "p1")
		playback.Names = domain.LocalizedText{"en": "Strum", "pt_BR": "Batida"}

		got, err := newSequencedDiagramNamed(map[string]string{"en": "Lick", "pt_BR": "Frase"}, domain.DiagramOptions{Playbacks: []domain.DiagramPlayback{playback}})

		require.NoError(t, err)
		assert.Equal(t, domain.LocalizedText{"en": "Strum", "pt_BR": "Batida"}, got.Playbacks[0].Names)
	})
}

func TestDiagram_DefaultPlayback(t *testing.T) {
	t.Run("is the playback the default id names", func(t *testing.T) {
		d, err := newSequencedDiagram(domain.DiagramOptions{
			Playbacks:         []domain.DiagramPlayback{playbackOf("pb-strum", "Strum", "p1"), playbackOf("pb-arp", "Arpeggio", "p2")},
			DefaultPlaybackID: strPtr("pb-arp"),
		})
		require.NoError(t, err)

		got, ok := d.DefaultPlayback()

		require.True(t, ok)
		assert.Equal(t, "pb-arp", got.ID)
	})

	t.Run("a diagram without playbacks has none", func(t *testing.T) {
		_, ok := domain.Diagram{}.DefaultPlayback()

		assert.False(t, ok)
	})
}

func TestDiagram_HasPlayback(t *testing.T) {
	d, err := newSequencedDiagram(domain.DiagramOptions{Playbacks: []domain.DiagramPlayback{playbackOf("pb-strum", "Strum", "p1")}})
	require.NoError(t, err)

	assert.True(t, d.HasPlayback("pb-strum"))
	assert.False(t, d.HasPlayback("pb-gone"))
}

func TestKeptDefaultPlaybackID(t *testing.T) {
	playbacks := []domain.DiagramPlayback{playbackOf("pb-new", "Fingerstyle", "p1"), playbackOf("pb-arp", "Arpeggio", "p2")}
	tests := []struct {
		name    string
		current *string
		want    *string
	}{
		{name: "a default still among the playbacks is kept", current: strPtr("pb-arp"), want: strPtr("pb-arp")},
		{name: "a default no longer among them is dropped, so the first becomes the default", current: strPtr("pb-strum"), want: nil},
		{name: "no current default stays none", current: nil, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, domain.KeptDefaultPlaybackID(tt.current, playbacks))
		})
	}
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
