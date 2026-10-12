//go:build integration

package bdd

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerExerciseSoundSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^a diagram "([^"]+)" exists on instrument "([^"]+)" with no playbacks$`, w.aDiagramWithNoPlaybacks)

	sc.Step(`^"([^"]+)" creates an audio_recognition exercise titled "([^"]+)" with options "([^"]+)" and "([^"]+)" correct, sounding (.+)$`, w.createsRecognitionSounding)
	sc.Step(`^"([^"]+)" creates an audio_recognition exercise titled "([^"]+)" with options "([^"]+)" and "([^"]+)" correct, an uploaded clip and diagram "([^"]+)" playing playback "([^"]+)"$`, w.createsRecognitionWithClipAndDiagram)
	sc.Step(`^"([^"]+)" creates an audio_recognition exercise titled "([^"]+)" with options "([^"]+)" and "([^"]+)" correct, and no sound$`, w.createsRecognitionWithNoSound)
	sc.Step(`^"([^"]+)" creates an audio_selection exercise titled "([^"]+)" with options:$`, w.createsSelectionWithSounds)
	sc.Step(`^"([^"]+)" has an audio_recognition exercise "([^"]+)" whose sound is an uploaded clip$`, w.hasRecognitionWithClip)
	sc.Step(`^"([^"]+)" updates the exercise's sound to diagram "([^"]+)" playing playback "([^"]+)", with no audio_url$`, w.updatesSoundToDiagram)

	sc.Step(`^the exercise's sound is diagram "([^"]+)" playing playback "([^"]+)"(?: at (\d+) BPM, (reversed))?$`, w.exerciseSoundIs)
	sc.Step(`^the exercise has no audio_url$`, w.exerciseHasNoAudioURL)
	sc.Step(`^option "([^"]+)" sounds diagram "([^"]+)" playing playback "([^"]+)"$`, w.optionSounds)
	sc.Step(`^option "([^"]+)" is marked correct$`, w.labelledOptionIsCorrect)
	sc.Step(`^option "([^"]+)" has an audio_url and no diagram$`, w.optionHasClipOnly)

	sc.Step(`^an audio_recognition exercise "([^"]+)" sounds diagram "([^"]+)" playing playback "([^"]+)"$`, w.seedsRecognitionSounding)
	sc.Step(`^an audio_selection exercise "([^"]+)" has an option that sounds diagram "([^"]+)" playing playback "([^"]+)"$`, w.seedsSelectionSounding)
	sc.Step(`^the error names playback "([^"]+)" and the exercise "([^"]+)"$`, w.conflictNamesPlaybackAndExercise)
	sc.Step(`^diagram "([^"]+)" still has playbacks "([^"]+)"$`, w.diagramHasPlaybacksNamed)
	sc.Step(`^diagram "([^"]+)" has playbacks "([^"]+)"$`, w.diagramHasPlaybacksNamed)
	sc.Step(`^the playback "([^"]+)" of diagram "([^"]+)" is changed to (\d+) BPM$`, w.playbackTempoChanged)
	sc.Step(`^the default playback of diagram "([^"]+)" is changed to "([^"]+)"$`, w.defaultPlaybackChanged)
}

const clipURL = "https://cdn.example.com/listening/clip.mp3"

var (
	soundWithNoPlayback      = regexp.MustCompile(`^diagram "([^"]+)" with no playback$`)
	soundWithDefaultPlayback = regexp.MustCompile(`^diagram "([^"]+)" playing its default playback$`)
	soundPlaying             = regexp.MustCompile(`^diagram "([^"]+)" playing playback "([^"]+)"(?: at (\d+) BPM, (reversed))?(, looping)?$`)
)

func (w *world) aDiagramWithNoPlaybacks(slug, instrument string) error {
	return w.seedDiagram(slug, bothLanguages(slug), instrument, domain.DiagramKindBasic, w.curatorID(), nil)
}

// soundRef is a diagram_ref of slug that plays the playback named name once
// (no playback chosen when name is empty).
func soundRef(slug, name string) generated.DiagramRef {
	ref := generated.DiagramRef{DiagramId: diagramID(slug)}
	direction := generated.DiagramRefPlaybackDirection("as_authored")
	ref.Playback = &diagramRefPlayback{Direction: &direction}
	if name != "" {
		id := playbackID(diagramID(slug).String(), name)
		ref.Playback.PlaybackId = &id
	}
	return ref
}

// soundFrom reads a sound phrase of the "sounding ..." step.
func soundFrom(phrase string) (generated.DiagramRef, error) {
	switch {
	case soundWithNoPlayback.MatchString(phrase):
		return generated.DiagramRef{DiagramId: diagramID(soundWithNoPlayback.FindStringSubmatch(phrase)[1])}, nil
	case soundWithDefaultPlayback.MatchString(phrase):
		return soundRef(soundWithDefaultPlayback.FindStringSubmatch(phrase)[1], ""), nil
	case soundPlaying.MatchString(phrase):
		match := soundPlaying.FindStringSubmatch(phrase)
		ref := soundRef(match[1], match[2])
		if match[3] != "" {
			tempo, err := strconv.Atoi(match[3])
			if err != nil {
				return ref, err
			}
			direction := generated.DiagramRefPlaybackDirection(match[4])
			ref.Playback.TempoBpm, ref.Playback.Direction = &tempo, &direction
		}
		if match[5] != "" {
			loop := true
			ref.Playback.Loop = &loop
		}
		return ref, nil
	default:
		return generated.DiagramRef{}, fmt.Errorf("unknown sound %q", phrase)
	}
}

// labelOptions are text options labelled labelsCSV, correct is the right one.
func labelOptions(labelsCSV, correct string) []generated.Option {
	labels := splitCommaList(labelsCSV)
	options := make([]generated.Option, len(labels))
	for i, label := range labels {
		options[i] = generated.Option{OptionId: uuid.New(), IsCorrect: label == correct, Label: &label}
	}
	return options
}

func (w *world) createRecognition(title, labelsCSV, correct string, audioURL *string, sound *generated.DiagramRef) error {
	resp, err := w.handler.CreateExercise(w.ctx(), generated.CreateExerciseRequestObject{
		Body: &generated.CreateExerciseRequest{
			SkillIds: w.skillIDsFor("skill-1"), ConceptIds: w.conceptIDsFor("concept-1"),
			Title: title, Prompt: promptDocFor("Is this chord major or minor?"), ExerciseType: generated.CreateExerciseRequestExerciseTypeAudioRecognition,
			AudioUrl: audioURL, DiagramRef: sound,
			Options:       opts(labelOptions(labelsCSV, correct)),
			LanguageCodes: []string{"en"},
		},
	})
	w.lastResp, w.lastErr = resp, err
	if created, ok := resp.(generated.CreateExercise201JSONResponse); ok {
		w.soundExerciseID = created.ExerciseId
	}
	return err
}

func (w *world) createsRecognitionSounding(_, title, labelsCSV, correct, phrase string) error {
	sound, err := soundFrom(phrase)
	if err != nil {
		return err
	}
	return w.createRecognition(title, labelsCSV, correct, nil, &sound)
}

func (w *world) createsRecognitionWithClipAndDiagram(_, title, labelsCSV, correct, slug, playback string) error {
	sound, clip := soundRef(slug, playback), clipURL
	return w.createRecognition(title, labelsCSV, correct, &clip, &sound)
}

func (w *world) createsRecognitionWithNoSound(_, title, labelsCSV, correct string) error {
	return w.createRecognition(title, labelsCSV, correct, nil, nil)
}

// createsSelectionWithSounds reads a | label | sound | correct | table whose
// sound is "uploaded clip", "<diagram> · <playback>", or both joined by " + ".
func (w *world) createsSelectionWithSounds(_, title string, table *godog.Table) error {
	var options []generated.Option
	for _, row := range table.Rows[1:] {
		label, sound, correct := row.Cells[0].Value, row.Cells[1].Value, row.Cells[2].Value == "yes"
		option := generated.Option{OptionId: uuid.New(), IsCorrect: correct, Label: &label}
		for _, part := range strings.Split(sound, " + ") {
			if part == "uploaded clip" {
				clip := clipURL
				option.AudioUrl = &clip
				continue
			}
			slug, playback, ok := strings.Cut(part, " · ")
			if !ok {
				return fmt.Errorf("unknown sound %q", part)
			}
			ref := soundRef(slug, playback)
			option.DiagramRef = &ref
		}
		options = append(options, option)
	}
	resp, err := w.handler.CreateExercise(w.ctx(), generated.CreateExerciseRequestObject{
		Body: &generated.CreateExerciseRequest{
			SkillIds: w.skillIDsFor("skill-1"), ConceptIds: w.conceptIDsFor("concept-1"),
			Title: title, Prompt: promptDocFor("Which of these sounds is a minor chord?"), ExerciseType: generated.CreateExerciseRequestExerciseTypeAudioSelection,
			Options:       opts(options),
			LanguageCodes: []string{"en"},
		},
	})
	w.lastResp, w.lastErr = resp, err
	if created, ok := resp.(generated.CreateExercise201JSONResponse); ok {
		w.soundExerciseID = created.ExerciseId
	}
	return err
}

func (w *world) hasRecognitionWithClip(_, title string) error {
	clip := clipURL
	if err := w.createRecognition(title, "Major, Minor", "Minor", &clip, nil); err != nil {
		return err
	}
	return w.exerciseCreated()
}

func (w *world) updatesSoundToDiagram(_, slug, playback string) error {
	current, err := w.soundExercise()
	if err != nil {
		return err
	}
	sound := soundRef(slug, playback)
	resp, err := w.handler.UpdateExercise(w.ctx(), generated.UpdateExerciseRequestObject{
		ExerciseId: current.ExerciseId,
		Body: &generated.UpdateExerciseRequest{
			SkillIds: w.skillIDsFor("skill-1"), ConceptIds: w.conceptIDsFor("concept-1"),
			Title: current.Title, Prompt: current.Prompt, DiagramRef: &sound,
			Options:       &current.Options,
			LanguageCodes: []string{"en"},
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

// soundExercise reads back the scenario's listening exercise.
func (w *world) soundExercise() (generated.Exercise, error) {
	resp, err := w.handler.GetExercise(w.ctx(), generated.GetExerciseRequestObject{ExerciseId: w.soundExerciseID})
	if err != nil {
		return generated.Exercise{}, err
	}
	exercise, ok := resp.(generated.GetExercise200JSONResponse)
	if !ok {
		return generated.Exercise{}, fmt.Errorf("expected to read the exercise, got %#v", resp)
	}
	return generated.Exercise(exercise), nil
}

// describeSound renders a sound ref the way the steps phrase it.
func describeSound(ref *generated.DiagramRef) string {
	if ref == nil {
		return "no diagram"
	}
	if ref.Playback == nil || ref.Playback.PlaybackId == nil {
		return fmt.Sprintf("diagram %s with no named playback", ref.DiagramId)
	}
	s := fmt.Sprintf("diagram %s playing %s", ref.DiagramId, *ref.Playback.PlaybackId)
	if ref.Playback.TempoBpm != nil {
		s += fmt.Sprintf(" at %d BPM", *ref.Playback.TempoBpm)
	}
	if ref.Playback.Direction != nil && *ref.Playback.Direction != "as_authored" {
		s += ", " + string(*ref.Playback.Direction)
	}
	return s
}

func (w *world) exerciseSoundIs(slug, playback, tempo, direction string) error {
	exercise, err := w.soundExercise()
	if err != nil {
		return err
	}
	want := fmt.Sprintf("diagram %s playing %s", diagramID(slug), playbackID(diagramID(slug).String(), playback))
	if tempo != "" {
		want += fmt.Sprintf(" at %s BPM, %s", tempo, direction)
	}
	if got := describeSound(exercise.DiagramRef); got != want {
		return fmt.Errorf("expected the sound %s, got %s", want, got)
	}
	return nil
}

func (w *world) exerciseHasNoAudioURL() error {
	exercise, err := w.soundExercise()
	if err != nil {
		return err
	}
	if exercise.AudioUrl != nil {
		return fmt.Errorf("expected no audio_url, got %q", *exercise.AudioUrl)
	}
	return nil
}

// labelledOption is the scenario exercise's option labelled label.
func (w *world) labelledOption(label string) (generated.Option, error) {
	exercise, err := w.soundExercise()
	if err != nil {
		return generated.Option{}, err
	}
	for _, option := range exercise.Options {
		if option.Label != nil && *option.Label == label {
			return option, nil
		}
	}
	return generated.Option{}, fmt.Errorf("no option labelled %q", label)
}

func (w *world) optionSounds(label, slug, playback string) error {
	option, err := w.labelledOption(label)
	if err != nil {
		return err
	}
	want := fmt.Sprintf("diagram %s playing %s", diagramID(slug), playbackID(diagramID(slug).String(), playback))
	if got := describeSound(option.DiagramRef); got != want {
		return fmt.Errorf("expected option %q to sound %s, got %s", label, want, got)
	}
	return nil
}

func (w *world) labelledOptionIsCorrect(label string) error {
	option, err := w.labelledOption(label)
	if err != nil {
		return err
	}
	if !option.IsCorrect {
		return fmt.Errorf("expected option %q to be correct", label)
	}
	return nil
}

func (w *world) optionHasClipOnly(label string) error {
	option, err := w.labelledOption(label)
	if err != nil {
		return err
	}
	if option.AudioUrl == nil || option.DiagramRef != nil {
		return fmt.Errorf("expected option %q to carry only an audio_url, got audio_url=%v diagram_ref=%s", label, option.AudioUrl, describeSound(option.DiagramRef))
	}
	return nil
}

// domainSound is a stored diagram_ref of slug playing the playback named name.
func domainSound(slug, name string) *domain.DiagramRef {
	id := playbackID(diagramID(slug).String(), name).String()
	return &domain.DiagramRef{DiagramID: diagramID(slug).String(), Playback: &domain.DiagramRefPlayback{Direction: domain.DiagramPlaybackDirectionAsAuthored, PlaybackID: &id}}
}

// putSoundExercise stores e as the scenario's listening exercise.
func (w *world) putSoundExercise(e domain.Exercise) error {
	e.Prompt = domain.NewPlainTextPrompt("Listen")
	e.Languages = []domain.Language{{Code: "en"}}
	e.CreatedAt = fixedNow
	if err := w.exercises.Create(w.ctx(), e); err != nil {
		return err
	}
	w.soundExerciseID = uuid.MustParse(e.ID)
	return nil
}

func (w *world) seedsRecognitionSounding(title, slug, playback string) error {
	minor := "Minor"
	return w.putSoundExercise(domain.Exercise{
		ID: exerciseID(title).String(), Title: title, ExerciseType: domain.ExerciseTypeAudioRecognition,
		DiagramRef: domainSound(slug, playback),
		Options:    []domain.Option{{ID: uuid.NewString(), IsCorrect: true, Label: &minor}},
	})
}

func (w *world) seedsSelectionSounding(title, slug, playback string) error {
	return w.putSoundExercise(domain.Exercise{
		ID: exerciseID(title).String(), Title: title, ExerciseType: domain.ExerciseTypeAudioSelection,
		Options: []domain.Option{{ID: uuid.NewString(), IsCorrect: true, DiagramRef: domainSound(slug, playback)}},
	})
}

func (w *world) conflictNamesPlaybackAndExercise(playback, title string) error {
	resp, ok := w.lastResp.(generated.UpdateDiagram409JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 409 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	for _, want := range []string{strconv.Quote(playback), strconv.Quote(title)} {
		if !strings.Contains(resp.Message, want) {
			return fmt.Errorf("expected the error to name %s, got %q", want, resp.Message)
		}
	}
	return nil
}

func (w *world) diagramHasPlaybacksNamed(slug, namesCSV string) error {
	diagram, err := w.storedDiagram(slug)
	if err != nil {
		return err
	}
	got := make([]string, len(diagram.Playbacks))
	for i, p := range diagram.Playbacks {
		got[i] = p.Names["en"]
	}
	if want := splitCommaList(namesCSV); !slices.Equal(got, want) {
		return fmt.Errorf("expected playbacks %v, got %v", want, got)
	}
	return nil
}

// updateDiagramPlaybacks sends body as an update of slug, as admin.
func (w *world) updateDiagramPlaybacks(slug string, body generated.UpdateDiagramRequest) error {
	w.curatorID()
	resp, err := w.handler.UpdateDiagram(w.identityCtx("admin"), generated.UpdateDiagramRequestObject{DiagramId: diagramID(slug), Body: &body})
	w.lastResp, w.lastErr = resp, err
	if err != nil {
		return err
	}
	if _, ok := resp.(generated.UpdateDiagram200JSONResponse); !ok {
		return fmt.Errorf("expected the diagram update to succeed, got %#v", resp)
	}
	return nil
}

func (w *world) playbackTempoChanged(name, slug string, tempo int) error {
	stored, err := w.storedDiagram(slug)
	if err != nil {
		return err
	}
	playbacks := make([]generated.DiagramPlaybackInput, len(stored.Playbacks))
	for i, p := range stored.Playbacks {
		playbacks[i] = resent(p)
		if p.Names["en"] == name {
			playbacks[i].TempoBpm = tempo
		}
	}
	return w.updateDiagramPlaybacks(slug, generated.UpdateDiagramRequest{Playbacks: &playbacks})
}

func (w *world) defaultPlaybackChanged(slug, name string) error {
	id := openapi_types.UUID(playbackID(diagramID(slug).String(), name))
	return w.updateDiagramPlaybacks(slug, generated.UpdateDiagramRequest{DefaultPlaybackId: nullable.NewNullableWithValue(id)})
}
