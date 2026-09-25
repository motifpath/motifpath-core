//go:build integration

package bdd

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerInstrumentSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^a fretted instrument "([^"]+)" exists in the system$`, w.aFrettedInstrumentExists)
	sc.Step(`^a keyboard instrument "([^"]+)" exists in the system$`, w.aKeyboardInstrumentExists)

	sc.Step(`^"([^"]+)" creates a fretted instrument named "([^"]+)" in English and "([^"]+)" in Portuguese with (\d+) strings tuned "([^"]+)"$`, w.createsBilingualFrettedInstrument)
	sc.Step(`^"([^"]+)" creates a fretted instrument named only "([^"]+)" in English with (\d+) strings tuned "([^"]+)"$`, w.createsEnglishOnlyFrettedInstrument)
	sc.Step(`^"([^"]+)" creates a fretted instrument with names "([^"]+)" "([^"]+)", "([^"]+)" "([^"]+)" and "([^"]+)" "([^"]+)" with (\d+) strings tuned "([^"]+)"$`, w.createsFrettedInstrumentWithThreeNames)
	sc.Step(`^"([^"]+)" creates a keyboard instrument named "([^"]+)" in English and "([^"]+)" in Portuguese with key range "([^"]+)" to "([^"]+)"$`, w.createsKeyboardInstrument)
	sc.Step(`^"([^"]+)" renames instrument "([^"]+)" to "([^"]+)" in English and "([^"]+)" in Portuguese$`, w.renamesInstrument)
	sc.Step(`^"([^"]+)" renames instrument "([^"]+)" to only "([^"]+)" in English$`, w.renamesInstrumentEnglishOnly)
	sc.Step(`^"([^"]+)" renames an instrument with an ID that does not exist$`, w.renamesMissingInstrument)
	sc.Step(`^an unauthenticated request attempts to rename instrument "([^"]+)"$`, w.unauthRenamesInstrument)
	sc.Step(`^"([^"]+)" attempts to create an instrument$`, w.attemptsCreateInstrument)
	sc.Step(`^an unauthenticated request attempts to create an instrument$`, w.unauthCreatesInstrument)
	sc.Step(`^"([^"]+)" submits a create instrument request for a fretted instrument with the tuning field omitted$`, w.submitsFrettedInstrumentWithoutTuning)
	sc.Step(`^"([^"]+)" submits a create instrument request for a keyboard instrument carrying tuning$`, w.submitsKeyboardInstrumentWithTuning)
	sc.Step(`^"([^"]+)" submits a create instrument request with family "([^"]+)"$`, w.submitsInstrumentWithFamily)

	sc.Step(`^the instrument is created and assigned a stable identifier$`, w.instrumentCreated)
	sc.Step(`^the instrument's family is "([^"]+)"$`, w.instrumentFamilyIs)
	sc.Step(`^the instrument's name in "([^"]+)" is "([^"]+)"$`, w.instrumentNameIs)
	sc.Step(`^the instrument's languages are "([^"]+)"$`, w.instrumentLanguagesAre)

	sc.Step(`^"([^"]+)" lists all known instruments$`, w.listsAllInstruments)
	sc.Step(`^the response includes instrument "([^"]+)" and instrument "([^"]+)"$`, w.responseIncludesTwoInstruments)
}

func (w *world) aFrettedInstrumentExists(name string) error {
	six := 6
	w.instruments.put(domain.Instrument{
		ID: instrumentID(name).String(), Names: domain.LocalizedText{"en": name}, Family: domain.InstrumentFamilyFretted,
		StringCount: &six, Tuning: []string{"E", "A", "D", "G", "B", "E"},
	})
	return nil
}

func (w *world) aKeyboardInstrumentExists(name string) error {
	w.instruments.put(domain.Instrument{
		ID: instrumentID(name).String(), Names: domain.LocalizedText{"en": name}, Family: domain.InstrumentFamilyKeyboard,
		KeyRange: &domain.KeyRange{Lowest: "A0", Highest: "C8"},
	})
	return nil
}

func (w *world) createInstrument(body generated.CreateInstrumentRequest) error {
	resp, err := w.handler.CreateInstrument(w.ctx(), generated.CreateInstrumentRequestObject{Body: &body})
	w.lastResp, w.lastErr = resp, err
	return err
}

func keyRangeBody(lowest, highest string) *struct {
	Highest string `json:"highest"`
	Lowest  string `json:"lowest"`
} {
	return &struct {
		Highest string `json:"highest"`
		Lowest  string `json:"lowest"`
	}{Highest: highest, Lowest: lowest}
}

// bilingual is the names map for an instrument named in English and
// Portuguese, the two languages MotifPath offers.
func bilingual(english, portuguese string) map[string]string {
	return map[string]string{"en": english, "pt_BR": portuguese}
}

func (w *world) createFrettedInstrument(names map[string]string, stringCount, tuning string) error {
	count, err := strconv.Atoi(stringCount)
	if err != nil {
		return fmt.Errorf("string count %q is not a number: %w", stringCount, err)
	}
	notes := splitCommaList(tuning)
	return w.createInstrument(generated.CreateInstrumentRequest{
		Names: names, Family: generated.CreateInstrumentRequestFamily(domain.InstrumentFamilyFretted),
		StringCount: &count, Tuning: &notes,
	})
}

func (w *world) createsBilingualFrettedInstrument(_, english, portuguese, stringCount, tuning string) error {
	return w.createFrettedInstrument(bilingual(english, portuguese), stringCount, tuning)
}

func (w *world) createsEnglishOnlyFrettedInstrument(_, english, stringCount, tuning string) error {
	return w.createFrettedInstrument(map[string]string{"en": english}, stringCount, tuning)
}

func (w *world) createsFrettedInstrumentWithThreeNames(_, lang1, name1, lang2, name2, lang3, name3, stringCount, tuning string) error {
	return w.createFrettedInstrument(map[string]string{lang1: name1, lang2: name2, lang3: name3}, stringCount, tuning)
}

func (w *world) createsKeyboardInstrument(_, english, portuguese, lowest, highest string) error {
	return w.createInstrument(generated.CreateInstrumentRequest{
		Names: bilingual(english, portuguese), Family: generated.CreateInstrumentRequestFamily(domain.InstrumentFamilyKeyboard),
		KeyRange: keyRangeBody(lowest, highest),
	})
}

func (w *world) renameInstrument(id uuid.UUID, names map[string]string) error {
	resp, err := w.handler.UpdateInstrument(w.ctx(), generated.UpdateInstrumentRequestObject{
		InstrumentId: id, Body: &generated.UpdateInstrumentRequest{Names: names},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) renamesInstrument(_, name, english, portuguese string) error {
	return w.renameInstrument(instrumentID(name), bilingual(english, portuguese))
}

func (w *world) renamesInstrumentEnglishOnly(_, name, english string) error {
	return w.renameInstrument(instrumentID(name), map[string]string{"en": english})
}

func (w *world) renamesMissingInstrument(string) error {
	return w.renameInstrument(instrumentID("does-not-exist"), bilingual("Ghost", "Fantasma"))
}

func (w *world) unauthRenamesInstrument(name string) error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.renameInstrument(instrumentID(name), bilingual("Guitar", "Violão"))
}

func (w *world) attemptsCreateInstrument(string) error {
	return w.createsKeyboardInstrument("", "Attempted", "Tentativa", "A0", "C8")
}

func (w *world) unauthCreatesInstrument() error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.attemptsCreateInstrument("")
}

func (w *world) submitsFrettedInstrumentWithoutTuning(string) error {
	count := 6
	return w.createInstrument(generated.CreateInstrumentRequest{
		Names: bilingual("No tuning", "Sem afinação"), Family: generated.CreateInstrumentRequestFamily(domain.InstrumentFamilyFretted), StringCount: &count,
	})
}

func (w *world) submitsKeyboardInstrumentWithTuning(string) error {
	tuning := []string{"E", "A", "D", "G", "B", "E"}
	return w.createInstrument(generated.CreateInstrumentRequest{
		Names: bilingual("Tuned keyboard", "Teclado afinado"), Family: generated.CreateInstrumentRequestFamily(domain.InstrumentFamilyKeyboard),
		KeyRange: keyRangeBody("A0", "C8"), Tuning: &tuning,
	})
}

func (w *world) submitsInstrumentWithFamily(_, family string) error {
	return w.createInstrument(generated.CreateInstrumentRequest{
		Names: bilingual("Odd instrument", "Instrumento estranho"), Family: generated.CreateInstrumentRequestFamily(family),
	})
}

func (w *world) instrumentCreated() error {
	resp, ok := w.lastResp.(generated.CreateInstrument201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.InstrumentId.String() == "" {
		return fmt.Errorf("expected an instrument_id in the response")
	}
	return nil
}

// currentInstrument returns the Instrument carried by whichever create or
// update response the last step produced.
func (w *world) currentInstrument() (generated.Instrument, error) {
	switch resp := w.lastResp.(type) {
	case generated.CreateInstrument201JSONResponse:
		return generated.Instrument(resp), nil
	case generated.UpdateInstrument200JSONResponse:
		return generated.Instrument(resp), nil
	default:
		return generated.Instrument{}, fmt.Errorf("expected an instrument response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
}

func (w *world) instrumentFamilyIs(family string) error {
	instrument, err := w.currentInstrument()
	if err != nil {
		return err
	}
	if string(instrument.Family) != family {
		return fmt.Errorf("expected family %q, got %q", family, instrument.Family)
	}
	return nil
}

func (w *world) instrumentNameIs(language, want string) error {
	instrument, err := w.currentInstrument()
	if err != nil {
		return err
	}
	if got, ok := instrument.Names[language]; !ok || got != want {
		return fmt.Errorf("expected the %q name to be %q, got names %v", language, want, instrument.Names)
	}
	return nil
}

func (w *world) instrumentLanguagesAre(list string) error {
	instrument, err := w.currentInstrument()
	if err != nil {
		return err
	}
	if want := splitCommaList(list); !slices.Equal(instrument.Languages, want) {
		return fmt.Errorf("expected languages %v, got %v", want, instrument.Languages)
	}
	return nil
}

func (w *world) listsAllInstruments(string) error {
	resp, err := w.handler.ListInstruments(w.ctx(), generated.ListInstrumentsRequestObject{})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) responseIncludesTwoInstruments(nameA, nameB string) error {
	resp, ok := w.lastResp.(generated.ListInstruments200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	for _, name := range []string{nameA, nameB} {
		want := instrumentID(name)
		found := false
		for _, i := range resp {
			if i.InstrumentId == want {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("expected instruments to include %q (%s), got %+v", name, want, resp)
		}
	}
	return nil
}
