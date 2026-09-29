//go:build integration

package bdd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// defaultVoiceOf is the voice an instrument of each family is created with
// when a scenario doesn't name one: the platform's first voice of it.
var defaultVoiceOf = map[domain.InstrumentFamily]string{
	domain.InstrumentFamilyFretted:  "acoustic-guitar",
	domain.InstrumentFamilyKeyboard: "piano",
}

// voiceSamplesBaseURL is the public media address voice samples are listed
// under in these scenarios.
const voiceSamplesBaseURL = "https://media.motifpath.test"

func registerVoiceSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the platform provides a fretted voice "([^"]+)" and a keyboard voice "([^"]+)"$`, w.platformProvidesVoices)
	sc.Step(`^the platform provides a fretted voice "([^"]+)"$`, w.platformProvidesFrettedVoice)
	sc.Step(`^"([^"]+)" lists the voices$`, w.listsVoices)
	sc.Step(`^an unauthenticated request attempts to list the voices$`, w.unauthListsVoices)
	sc.Step(`^the voices include "([^"]+)" for (fretted|keyboard) instruments$`, w.voicesInclude)
	sc.Step(`^the voices are ordered "([^"]+)"$`, w.voicesAreOrdered)
	sc.Step(`^every sample of voice "([^"]+)" has a download address$`, w.everySampleHasAddress)
	sc.Step(`^the samples of voice "([^"]+)" are in ascending pitch$`, w.samplesAscend)
	sc.Step(`^every voice has a non-empty attribution$`, w.everyVoiceHasAttribution)
	sc.Step(`^every voice has a name in "([^"]+)" and in "([^"]+)"$`, w.everyVoiceNamedIn)

	sc.Step(`^"([^"]+)" creates a fretted instrument named "([^"]+)" in English and "([^"]+)" in Portuguese with (\d+) strings tuned "([^"]+)" and default voice "([^"]+)"$`, w.createsFrettedInstrumentWithVoice)
	sc.Step(`^"([^"]+)" submits a create instrument request for a fretted instrument with the default_voice_id field omitted$`, w.submitsInstrumentWithoutVoice)
	sc.Step(`^"([^"]+)" renames instrument "([^"]+)" to "([^"]+)" in English and "([^"]+)" in Portuguese with default voice "([^"]+)"$`, w.renamesInstrumentWithVoice)
	sc.Step(`^the instrument's default voice is "([^"]+)"$`, w.instrumentDefaultVoiceIs)
	sc.Step(`^the instrument's tuning is "([^"]+)"$`, w.instrumentTuningIs)
	sc.Step(`^the instrument's tuning is unchanged$`, w.instrumentTuningUnchanged)
}

func (w *world) platformProvidesVoices(fretted, keyboard string) error {
	for id, family := range map[string]domain.InstrumentFamily{fretted: domain.InstrumentFamilyFretted, keyboard: domain.InstrumentFamilyKeyboard} {
		voice, err := w.voices.GetByID(w.ctx(), id)
		if err != nil {
			return fmt.Errorf("expected the platform to provide voice %q: %w", id, err)
		}
		if voice.Family != family {
			return fmt.Errorf("expected voice %q to play %s instruments, it plays %s ones", id, family, voice.Family)
		}
	}
	return nil
}

func (w *world) platformProvidesFrettedVoice(id string) error {
	w.voices.put(domain.Voice{ID: id, Names: domain.LocalizedText{"en": id, "pt_BR": id}, Family: domain.InstrumentFamilyFretted, Pitches: []int{40}, Attribution: "Test samples"})
	return nil
}

func (w *world) listsVoices(string) error {
	resp, err := w.handler.ListVoices(w.ctx(), generated.ListVoicesRequestObject{})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthListsVoices() error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.listsVoices("")
}

func (w *world) listedVoices() ([]generated.Voice, error) {
	resp, ok := w.lastResp.(generated.ListVoices200JSONResponse)
	if !ok {
		return nil, fmt.Errorf("expected a 200 voice list, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return resp, nil
}

func (w *world) listedVoice(id string) (generated.Voice, error) {
	voices, err := w.listedVoices()
	if err != nil {
		return generated.Voice{}, err
	}
	for _, v := range voices {
		if v.VoiceId == id {
			return v, nil
		}
	}
	return generated.Voice{}, fmt.Errorf("expected voice %q among %+v", id, voices)
}

func (w *world) voicesInclude(id, family string) error {
	voice, err := w.listedVoice(id)
	if err != nil {
		return err
	}
	if string(voice.Family) != family {
		return fmt.Errorf("expected voice %q to play %s instruments, got %s", id, family, voice.Family)
	}
	return nil
}

func (w *world) voicesAreOrdered(list string) error {
	voices, err := w.listedVoices()
	if err != nil {
		return err
	}
	got := make([]string, len(voices))
	for i, v := range voices {
		got[i] = v.VoiceId
	}
	if want := splitCommaList(list); !slices.Equal(got, want) {
		return fmt.Errorf("expected voices %v, got %v", want, got)
	}
	return nil
}

func (w *world) everySampleHasAddress(id string) error {
	voice, err := w.listedVoice(id)
	if err != nil {
		return err
	}
	if len(voice.Samples) == 0 {
		return fmt.Errorf("expected voice %q to have samples", id)
	}
	for _, sample := range voice.Samples {
		if !strings.HasPrefix(sample.Url, voiceSamplesBaseURL+"/") {
			return fmt.Errorf("expected sample %d of voice %q to be downloadable from %s, got %q", sample.Pitch, id, voiceSamplesBaseURL, sample.Url)
		}
	}
	return nil
}

func (w *world) samplesAscend(id string) error {
	voice, err := w.listedVoice(id)
	if err != nil {
		return err
	}
	pitches := make([]int, len(voice.Samples))
	for i, sample := range voice.Samples {
		pitches[i] = sample.Pitch
	}
	if !slices.IsSorted(pitches) {
		return fmt.Errorf("expected voice %q's samples in ascending pitch, got %v", id, pitches)
	}
	return nil
}

func (w *world) everyVoiceHasAttribution() error {
	voices, err := w.listedVoices()
	if err != nil {
		return err
	}
	for _, v := range voices {
		if strings.TrimSpace(v.Attribution) == "" {
			return fmt.Errorf("expected voice %q to carry an attribution", v.VoiceId)
		}
	}
	return nil
}

func (w *world) everyVoiceNamedIn(first, second string) error {
	voices, err := w.listedVoices()
	if err != nil {
		return err
	}
	for _, v := range voices {
		for _, language := range []string{first, second} {
			if strings.TrimSpace(v.Names[language]) == "" {
				return fmt.Errorf("expected voice %q to be named in %q, got %v", v.VoiceId, language, v.Names)
			}
		}
	}
	return nil
}

func (w *world) createsFrettedInstrumentWithVoice(_, english, portuguese, stringCount, tuning, voice string) error {
	return w.createFrettedInstrumentWithVoice(bilingual(english, portuguese), stringCount, tuning, voice)
}

func (w *world) submitsInstrumentWithoutVoice(string) error {
	return w.createFrettedInstrumentWithVoice(bilingual("Guitar", "Violão"), "6", "E2, A2, D3, G3, B3, E4", "")
}

func (w *world) renamesInstrumentWithVoice(_, name, english, portuguese, voice string) error {
	return w.updateInstrument(instrumentID(name), generated.UpdateInstrumentRequest{Names: bilingual(english, portuguese), DefaultVoiceId: &voice})
}

func (w *world) instrumentDefaultVoiceIs(want string) error {
	instrument, err := w.currentInstrument()
	if err != nil {
		return err
	}
	if instrument.DefaultVoiceId != want {
		return fmt.Errorf("expected default voice %q, got %q", want, instrument.DefaultVoiceId)
	}
	return nil
}

func (w *world) instrumentTuningIs(list string) error {
	instrument, err := w.currentInstrument()
	if err != nil {
		return err
	}
	if instrument.Tuning == nil || !slices.Equal(*instrument.Tuning, splitCommaList(list)) {
		return fmt.Errorf("expected tuning %q, got %v", list, instrument.Tuning)
	}
	return nil
}

// instrumentTuningUnchanged checks the tuning against the one every
// fretted instrument seeded by "a fretted instrument ... exists" has.
func (w *world) instrumentTuningUnchanged() error {
	return w.instrumentTuningIs("E2, A2, D3, G3, B3, E4")
}
