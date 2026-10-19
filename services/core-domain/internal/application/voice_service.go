package application

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// VoiceService lists the platform's Voices with the address of every sample
// a player downloads to play them.
type VoiceService struct {
	voices ports.VoiceRepository
	// samplesBaseURL is the public media address samples are served under.
	samplesBaseURL string
}

func NewVoiceService(voices ports.VoiceRepository, samplesBaseURL string) *VoiceService {
	return &VoiceService{voices: voices, samplesBaseURL: strings.TrimSuffix(samplesBaseURL, "/")}
}

// VoiceSample is one recording of a voice: its MIDI pitch and where to
// download it.
type VoiceSample struct {
	Pitch int
	URL   string
}

// PlayableVoice is a Voice with its samples, lowest pitch first.
type PlayableVoice struct {
	Voice   domain.Voice
	Samples []VoiceSample
}

// ListVoices returns every voice in id order. Any authenticated user may
// list them, since students need them to play diagrams.
func (s *VoiceService) ListVoices(ctx context.Context) ([]PlayableVoice, error) {
	voices, err := s.voices.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]PlayableVoice, len(voices))
	for i, voice := range voices {
		pitches := slices.Sorted(slices.Values(voice.Pitches))
		samples := make([]VoiceSample, len(pitches))
		for j, pitch := range pitches {
			samples[j] = VoiceSample{Pitch: pitch, URL: fmt.Sprintf("%s/audio/voices/%s/%d.mp3", s.samplesBaseURL, voice.ID, pitch)}
		}
		result[i] = PlayableVoice{Voice: voice, Samples: samples}
	}
	return result, nil
}
