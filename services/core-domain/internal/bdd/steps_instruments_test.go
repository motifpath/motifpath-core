//go:build integration

package bdd

import (
	"fmt"
	"strconv"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerInstrumentSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^a fretted instrument "([^"]+)" exists in the system$`, w.aFrettedInstrumentExists)
	sc.Step(`^a keyboard instrument "([^"]+)" exists in the system$`, w.aKeyboardInstrumentExists)

	sc.Step(`^"([^"]+)" creates a fretted instrument named "([^"]+)" with (\d+) strings tuned "([^"]+)"$`, w.createsFrettedInstrument)
	sc.Step(`^"([^"]+)" creates a keyboard instrument named "([^"]+)" with key range "([^"]+)" to "([^"]+)"$`, w.createsKeyboardInstrument)
	sc.Step(`^"([^"]+)" attempts to create an instrument$`, w.attemptsCreateInstrument)
	sc.Step(`^an unauthenticated request attempts to create an instrument$`, w.unauthCreatesInstrument)
	sc.Step(`^"([^"]+)" submits a create instrument request for a fretted instrument with the tuning field omitted$`, w.submitsFrettedInstrumentWithoutTuning)
	sc.Step(`^"([^"]+)" submits a create instrument request for a keyboard instrument carrying tuning$`, w.submitsKeyboardInstrumentWithTuning)
	sc.Step(`^"([^"]+)" submits a create instrument request with family "([^"]+)"$`, w.submitsInstrumentWithFamily)

	sc.Step(`^the instrument is created and assigned a stable identifier$`, w.instrumentCreated)
	sc.Step(`^the instrument's family is "([^"]+)"$`, w.instrumentFamilyIs)

	sc.Step(`^"([^"]+)" lists all known instruments$`, w.listsAllInstruments)
	sc.Step(`^the response includes instrument "([^"]+)" and instrument "([^"]+)"$`, w.responseIncludesTwoInstruments)
}

func (w *world) aFrettedInstrumentExists(name string) error {
	six := 6
	w.instruments.put(domain.Instrument{
		ID: instrumentID(name).String(), Name: name, Family: domain.InstrumentFamilyFretted,
		StringCount: &six, Tuning: []string{"E", "A", "D", "G", "B", "E"},
	})
	return nil
}

func (w *world) aKeyboardInstrumentExists(name string) error {
	w.instruments.put(domain.Instrument{
		ID: instrumentID(name).String(), Name: name, Family: domain.InstrumentFamilyKeyboard,
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

func (w *world) createsFrettedInstrument(_, name, stringCount, tuning string) error {
	count, err := strconv.Atoi(stringCount)
	if err != nil {
		return fmt.Errorf("string count %q is not a number: %w", stringCount, err)
	}
	notes := splitCommaList(tuning)
	return w.createInstrument(generated.CreateInstrumentRequest{
		Name: name, Family: generated.CreateInstrumentRequestFamily(domain.InstrumentFamilyFretted),
		StringCount: &count, Tuning: &notes,
	})
}

func (w *world) createsKeyboardInstrument(_, name, lowest, highest string) error {
	return w.createInstrument(generated.CreateInstrumentRequest{
		Name: name, Family: generated.CreateInstrumentRequestFamily(domain.InstrumentFamilyKeyboard),
		KeyRange: keyRangeBody(lowest, highest),
	})
}

func (w *world) attemptsCreateInstrument(string) error {
	return w.createsKeyboardInstrument("", "attempted-instrument", "A0", "C8")
}

func (w *world) unauthCreatesInstrument() error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.attemptsCreateInstrument("")
}

func (w *world) submitsFrettedInstrumentWithoutTuning(string) error {
	count := 6
	return w.createInstrument(generated.CreateInstrumentRequest{
		Name: "No tuning", Family: generated.CreateInstrumentRequestFamily(domain.InstrumentFamilyFretted), StringCount: &count,
	})
}

func (w *world) submitsKeyboardInstrumentWithTuning(string) error {
	tuning := []string{"E", "A", "D", "G", "B", "E"}
	return w.createInstrument(generated.CreateInstrumentRequest{
		Name: "Tuned keyboard", Family: generated.CreateInstrumentRequestFamily(domain.InstrumentFamilyKeyboard),
		KeyRange: keyRangeBody("A0", "C8"), Tuning: &tuning,
	})
}

func (w *world) submitsInstrumentWithFamily(_, family string) error {
	return w.createInstrument(generated.CreateInstrumentRequest{
		Name: "Odd instrument", Family: generated.CreateInstrumentRequestFamily(family),
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

func (w *world) instrumentFamilyIs(family string) error {
	resp, ok := w.lastResp.(generated.CreateInstrument201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if string(resp.Family) != family {
		return fmt.Errorf("expected family %q, got %q", family, resp.Family)
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
