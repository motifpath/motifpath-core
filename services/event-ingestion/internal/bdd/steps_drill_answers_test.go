//go:build integration

package bdd

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"

	"github.com/cucumber/godog"

	"github.com/motifpath/event-ingestion/internal/domain"
)

// Steps for the answers whose shape carries more than one value: a note named
// among choices, an item skipped, and the drill v2 answers.
func registerDrillAnswerSteps(sc *godog.ScenarioContext, w *world) {
	list := `((?:"[^"]*"(?:, | and )?)+)`
	sc.Step(`^"([^"]+)" submits a practice\.item_answered event naming the note "([^"]+)" among `+list+` for the guitar cell on string (\d+) at fret (\d+)$`, w.submitNameTheNoteAmong)
	sc.Step(`^"([^"]+)" submits a practice\.item_answered event skipping exercise "([^"]+)" because its sound failed to load$`, w.submitSkippedExercise)
	sc.Step(`^"([^"]+)" submits a practice\.item_answered event picking "([^"]+)" among `+list+` for the chord on (\S+) in (\S+)$`, w.submitGeneratedChoice)
	sc.Step(`^"([^"]+)" submits a practice\.item_answered event for a guide-tone take of "([^"]+)" in (\S+) at (\d+) BPM with no taps$`, w.submitEmptyGuideToneTake)
	sc.Step(`^"([^"]+)" submits a practice\.item_answered event for a guide-tone take of "([^"]+)" in (\S+) at (\d+) BPM in (\w+) with no taps$`, w.submitEmptyGuideToneTakeInNoteValue)
	sc.Step(`^"([^"]+)" submits a practice\.item_answered event tapping ((?:string \d+ at fret \d+(?:, | and )?)+) for the note "([^"]+)" in the window from fret (\d+)$`, w.submitMultiTap)

	sc.Step(`^the stored and published practice\.item_answered event keeps the choices `+list+`$`, w.answerKeepsTheChoices)
	sc.Step(`^the stored and published practice\.item_answered event is not answered, with the reason "([^"]+)"$`, w.answerIsNotAnswered)
	sc.Step(`^the stored and published practice\.item_answered event keeps an empty list of taps$`, w.answerKeepsAnEmptyListOfTaps)
}

type timedTap struct {
	AtMs   int `json:"at_ms"`
	String int `json:"string"`
	Fret   int `json:"fret"`
}

type fretWindow struct {
	FromFret int `json:"from_fret"`
}

type tappedCell struct {
	String int `json:"string"`
	Fret   int `json:"fret"`
}

var (
	quotedValue = regexp.MustCompile(`"([^"]*)"`)
	stringFret  = regexp.MustCompile(`string (\d+) at fret (\d+)`)
)

// quotedValues reads a list written as "a", "b" and "c".
func quotedValues(list string) []string {
	var values []string
	for _, m := range quotedValue.FindAllStringSubmatch(list, -1) {
		values = append(values, m[1])
	}
	return values
}

// ── When ───────────────────────────────────────────────────────────────────

func (w *world) submitNameTheNoteAmong(name, note, choices, stringNumber, fret string) error {
	response := nameTheNote(note)
	response.Choices = quotedValues(choices)
	return w.submitPractice(w.answerEvent(name, guitarCellKey(stringNumber, fret), response))
}

func (w *world) submitSkippedExercise(name, exerciseName string) error {
	return w.submitPractice(w.answerEvent(name, exerciseKey(exerciseName), &practiceResponse{ResponseType: "not_answered", Reason: "failed_to_load"}))
}

func (w *world) submitGeneratedChoice(name, choice, options, numeral, key string) error {
	latency := 2100
	return w.submitPractice(w.answerEvent(name, "diatonic_chord:"+key+":"+numeral, &practiceResponse{
		ResponseType: "generated_choice", Way: "numeral_to_chord", Options: quotedValues(options), Choice: choice, LatencyMs: &latency,
	}))
}

func (w *world) submitEmptyGuideToneTake(name, progression, key, tempoStr string) error {
	tempo, err := strconv.Atoi(tempoStr)
	if err != nil {
		return err
	}
	return w.submitPractice(w.answerEvent(name, fmt.Sprintf("guide_tone:%s:%s", guitarID, progression), &practiceResponse{
		ResponseType: "timed_taps", MusicKey: key, Tone: "3", TempoBPM: tempo, Taps: &[]timedTap{},
	}))
}

func (w *world) submitEmptyGuideToneTakeInNoteValue(name, progression, key, tempoStr, noteValue string) error {
	tempo, err := strconv.Atoi(tempoStr)
	if err != nil {
		return err
	}
	return w.submitPractice(w.answerEvent(name, fmt.Sprintf("guide_tone:%s:%s", guitarID, progression), &practiceResponse{
		ResponseType: "timed_taps", MusicKey: key, Tone: "3", TempoBPM: tempo, NoteValue: noteValue, Taps: &[]timedTap{},
	}))
}

func (w *world) submitMultiTap(name, places, note, fromStr string) error {
	from, err := strconv.Atoi(fromStr)
	if err != nil {
		return err
	}
	cells := []tappedCell{}
	for _, m := range stringFret.FindAllStringSubmatch(places, -1) {
		str, err := strconv.Atoi(m[1])
		if err != nil {
			return err
		}
		fret, err := strconv.Atoi(m[2])
		if err != nil {
			return err
		}
		cells = append(cells, tappedCell{String: str, Fret: fret})
	}
	// Finding every instance is answered under the planned cell's key: the first
	// place tapped, which must be the note named.
	if len(cells) == 0 || guitarNoteAt(cells[0].String, cells[0].Fret) != note {
		return fmt.Errorf("the first place tapped must be a %s, the cell asked", note)
	}
	latency := 5200
	return w.submitPractice(w.answerEvent(name, fmt.Sprintf("fretboard_cell:%s:%d:%d", guitarID, cells[0].String, cells[0].Fret), &practiceResponse{
		ResponseType: "multi_tap", FretWindow: &fretWindow{FromFret: from}, Cells: &cells, LatencyMs: &latency,
	}))
}

// ── Then ───────────────────────────────────────────────────────────────────

// storedAndPublishedResponses are the answer's response as stored and as published.
func (w *world) storedAndPublishedResponses() (map[string]domain.PracticeResponse, error) {
	stored, published, err := w.storedAndPublishedAnswer()
	if err != nil {
		return nil, err
	}
	return map[string]domain.PracticeResponse{"stored": stored.Response, "published": published.Response}, nil
}

func (w *world) answerKeepsTheChoices(choices string) error {
	responses, err := w.storedAndPublishedResponses()
	if err != nil {
		return err
	}
	want := quotedValues(choices)
	for label, r := range responses {
		if !slices.Equal(r.Choices, want) {
			return fmt.Errorf("the %s answer keeps the choices %v, want %v", label, r.Choices, want)
		}
	}
	return nil
}

func (w *world) answerIsNotAnswered(reason string) error {
	responses, err := w.storedAndPublishedResponses()
	if err != nil {
		return err
	}
	for label, r := range responses {
		if r.Type != domain.PracticeResponseNotAnswered || string(r.Reason) != reason {
			return fmt.Errorf("the %s answer is %s with reason %q, want not_answered with %q", label, r.Type, r.Reason, reason)
		}
	}
	return nil
}

func (w *world) answerKeepsAnEmptyListOfTaps() error {
	responses, err := w.storedAndPublishedResponses()
	if err != nil {
		return err
	}
	for label, r := range responses {
		if r.Taps == nil || len(r.Taps) != 0 {
			return fmt.Errorf("the %s answer's taps are %v, want an empty list", label, r.Taps)
		}
	}
	return nil
}

// guitarNoteAt is the note of a standard-tuned guitar's string at fret, spelled
// with sharps; string 1 is the highest-pitched.
func guitarNoteAt(str, fret int) string {
	notes := []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}
	openByString := map[int]int{1: 4, 2: 11, 3: 7, 4: 2, 5: 9, 6: 4}
	return notes[(openByString[str]+fret)%12]
}
