//go:build integration

package bdd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// The formulas of the qualities the chord scenarios use; the full table
// lives in the chord catalog and reaches the service as installed rows.
var scenarioFormulas = map[domain.ChordQuality][]string{
	domain.ChordQualityMajor:  {"R", "3", "5"},
	domain.ChordQualityMinor:  {"R", "b3", "5"},
	domain.ChordQualityMajor7: {"R", "3", "5", "7"},
}

func registerChordSteps(sc *godog.ScenarioContext, w *world) {
	// The chord catalog, installed as the catalog's migrations would.
	sc.Step(`^the chord catalog has a voicing "([^"]+)" on instrument "([^"]+)"$`, w.chordCatalogHasVoicing)
	sc.Step(`^the chord catalog has the chord "([^"]+)" with active voicings ranked:$`, w.chordCatalogHasChord)
	sc.Step(`^voicing "([^"]+)" has been withdrawn from the chord catalog$`, w.voicingWithdrawn)

	// Chord voicing diagrams in the diagram library.
	sc.Step(`^the (?:new )?diagram's purpose is "([^"]+)"$`, w.diagramPurposeIs)
	sc.Step(`^"([^"]+)" lists diagrams of purpose "([^"]+)"$`, w.listsDiagramsOfPurpose)
	sc.Step(`^the response includes the diagram of voicing "([^"]+)"$`, func(key string) error { return w.responseIncludes(voicingSlug(key)) })
	sc.Step(`^the response does not include the diagram of voicing "([^"]+)"$`, func(key string) error { return w.responseDoesNotInclude(voicingSlug(key)) })
	sc.Step(`^the response includes "([^"]+)" and the diagram of voicing "([^"]+)"$`, w.responseIncludesDiagramAndVoicing)
	sc.Step(`^the diagram of voicing "([^"]+)" has purpose "([^"]+)"$`, w.listedVoicingDiagramHasPurpose)
	sc.Step(`^"([^"]+)" updates the diagram of voicing "([^"]+)" setting root note "([^"]+)" and label display "([^"]+)"$`, func(caller, key, root, display string) error {
		return w.updatesDiagramRootAndLabelDisplay(caller, voicingSlug(key), root, display)
	})
	sc.Step(`^the request is refused because only the chord catalog can change that diagram$`, w.refusedAsChordVoicing)
	sc.Step(`^the diagram of voicing "([^"]+)" is unchanged$`, w.voicingDiagramUnchanged)
	sc.Step(`^"([^"]+)" saves a copy of the diagram of voicing "([^"]+)" named "([^"]+)"$`, func(caller, key, name string) error {
		return w.savesCopyOfDiagram(caller, voicingSlug(key), name)
	})
	sc.Step(`^a new diagram is created with the same positions as the diagram of voicing "([^"]+)"$`, func(key string) error {
		return w.newDiagramCopiesPositionsOf(voicingSlug(key))
	})
	sc.Step(`^"([^"]+)" can update the new diagram$`, w.canUpdateNewDiagram)
	sc.Step(`^"([^"]+)" retrieves the diagram of voicing "([^"]+)"$`, func(caller, key string) error {
		return w.retrievesDiagram(caller, voicingSlug(key))
	})
	sc.Step(`^the response is the diagram of voicing "([^"]+)", with its positions and playbacks$`, w.responseIsVoicingDiagram)
	sc.Step(`^"([^"]+)" retrieves the new diagram$`, w.retrievesNewDiagram)
	sc.Step(`^the diagram carries voicing "([^"]+)", with its fingering and fret window$`, w.diagramCarriesVoicing)
	sc.Step(`^the diagram carries no voicing$`, w.diagramCarriesNoVoicing)

	// Searching and reading the catalog.
	sc.Step(`^"([^"]+)" searches the chord catalog for "([^"]*)"$`, w.searchesChords)
	sc.Step(`^"([^"]+)" searches the chord catalog for a symbol of (\d+) characters$`, func(caller string, n int) error {
		return w.searchesChords(caller, strings.Repeat("C", n))
	})
	sc.Step(`^an unauthenticated request searches the chord catalog for "([^"]+)"$`, func(symbol string) error {
		return w.searchesChords("", symbol)
	})
	sc.Step(`^the symbol is read as parsed$`, func() error { return w.symbolReadAs("parsed", "") })
	sc.Step(`^the symbol is read as no chord$`, func() error { return w.symbolReadAs("no_chord", "") })
	sc.Step(`^the symbol is read as unparsed, with the warning "([^"]+)"$`, func(warning string) error { return w.symbolReadAs("unparsed", warning) })
	sc.Step(`^the chord found is "([^"]+)"$`, w.chordFoundIs)
	sc.Step(`^no chord is found$`, w.noChordFound)
	sc.Step(`^the chord offered without the bass is "([^"]+)"$`, w.chordWithoutBassIs)
	sc.Step(`^no chord without the bass is offered$`, w.noChordWithoutBass)
	sc.Step(`^the search result keeps the symbol as written, "([^"]+)"$`, w.searchKeepsSymbol)
	sc.Step(`^the parsed root is "([^"]+)"$`, w.parsedRootIs)
	sc.Step(`^its voicings are "([^"]+)" then "([^"]+)"$`, func(first, second string) error { return w.voicingsAre(first, second) })
	sc.Step(`^its voicings are "([^"]+)" only$`, func(only string) error { return w.voicingsAre(only) })
	sc.Step(`^"([^"]+)" retrieves the chord "([^"]+)"$`, w.retrievesChord)
	sc.Step(`^"([^"]+)" retrieves a chord with an ID that does not exist$`, w.retrievesMissingChord)
	sc.Step(`^the chord's formula is "([^"]+)", "([^"]+)", "([^"]+)", "([^"]+)"$`, w.chordFormulaIs)
	sc.Step(`^each voicing names its chord_voicing diagram$`, w.eachVoicingNamesItsDiagram)
}

func voicingSlug(key string) string   { return "voicing:" + key }
func chordID(symbol string) uuid.UUID { return deterministicUUID("chord", symbol) }
func voicingID(key string) uuid.UUID  { return deterministicUUID("chord-voicing", key) }

// seedVoicingDiagram installs the chord_voicing diagram of voicing key: one
// position per string that sounds, and a down-strum playback of them all.
func (w *world) seedVoicingDiagram(key, instrumentName string, frets map[int]int, root string) (domain.Diagram, error) {
	instrument, err := w.ensureInstrumentSeeded(instrumentName)
	if err != nil {
		return domain.Diagram{}, err
	}
	slug := voicingSlug(key)
	var positions []domain.Position
	var ids []string
	for s := 6; s >= 1; s-- {
		fret, ok := frets[s]
		if !ok {
			continue
		}
		str, fr := s, fret
		id := deterministicUUID("position", slug, fmt.Sprint(s)).String()
		positions = append(positions, domain.Position{ID: id, Interval: "R", NoteName: root, String: &str, Fret: &fr})
		ids = append(ids, id)
	}
	playback := domain.DiagramPlayback{ID: deterministicUUID("playback", slug).String(), Names: domain.LocalizedText{"en": "Strum down", "pt_BR": "Batida para baixo"},
		TempoBPM: 60, Steps: []domain.SequenceStep{{PositionIDs: ids, Value: domain.NoteValue{Num: 1, Den: 1}, Strum: domain.StrumDown}}}
	skillID, conceptID := w.skillIDFor("chord-diagrams"), w.conceptIDFor("chords")
	diagram, err := domain.NewDiagram(diagramID(slug).String(), w.curatorID(), instrument, map[string]string{"en": key + " — open", "pt_BR": key + " — aberto"}, offeredLanguages,
		positions, []string{skillID.String()}, []string{conceptID.String()}, domain.DiagramOptions{Kind: domain.DiagramKindBasic, RootNote: &root, Playbacks: []domain.DiagramPlayback{playback}}, fixedNow)
	if err != nil {
		return domain.Diagram{}, fmt.Errorf("seeding the diagram of voicing %q: %w", key, err)
	}
	diagram.Purpose = domain.DiagramPurposeChordVoicing
	w.diagrams.put(diagram)
	return diagram, nil
}

// fingeringOf fingers a seeded voicing's fretted positions one finger per
// fret above the lowest, and gives the frets they span; open strings take none.
func fingeringOf(diagram domain.Diagram) (lowest, highest int, fingering []domain.VoicingFinger) {
	for _, p := range diagram.Positions {
		if p.Fret == nil || *p.Fret == 0 {
			continue
		}
		if lowest == 0 || *p.Fret < lowest {
			lowest = *p.Fret
		}
		highest = max(highest, *p.Fret)
	}
	for _, p := range diagram.Positions {
		if p.Fret != nil && *p.Fret > 0 {
			fingering = append(fingering, domain.VoicingFinger{PositionID: p.ID, Finger: fmt.Sprint(*p.Fret - lowest + 1)})
		}
	}
	return lowest, highest, fingering
}

func (w *world) chordCatalogHasVoicing(key, instrumentName string) error {
	_, err := w.seedVoicingDiagram(key, instrumentName, map[int]int{5: 0, 4: 2, 3: 2, 2: 1, 1: 0}, "A")
	return err
}

func (w *world) chordCatalogHasChord(symbol string, table *godog.Table) error {
	var keys []string
	for row := 1; row < len(table.Rows); row++ {
		key, err := cell(table, row, "voicing")
		if err != nil {
			return err
		}
		rankCell, err := cell(table, row, "rank")
		if err != nil {
			return err
		}
		var rank int
		if _, err := fmt.Sscan(rankCell, &rank); err != nil {
			return fmt.Errorf("rank %q: %w", rankCell, err)
		}
		if rank != len(keys)+1 {
			return fmt.Errorf("voicings must be listed in rank order, got rank %d in row %d", rank, row)
		}
		keys = append(keys, key)
	}
	return w.seedCatalogChord(symbol, keys...)
}

// seedCatalogChord installs the chord symbol in the catalog with the voicings keys,
// ranked in the order given, as the catalog's migrations would.
func (w *world) seedCatalogChord(symbol string, keys ...string) error {
	reading := domain.ParseChordSymbol(symbol)
	if reading.Parsed == nil {
		return fmt.Errorf("%q is not a chord symbol", symbol)
	}
	p := reading.Parsed
	chord := domain.ChordDefinition{ID: chordID(symbol).String(), CanonicalSymbol: p.CanonicalSymbol, Root: p.Root, RootPitchClass: p.RootPitchClass,
		Quality: p.Quality, Formula: scenarioFormulas[p.Quality], Bass: p.Bass, BassPitchClass: p.BassPitchClass}
	for i, key := range keys {
		diagram, err := w.seedVoicingDiagram(key, "guitar", map[int]int{5: 0, 4: 2}, p.Root)
		if err != nil {
			return err
		}
		lowest, highest, fingering := fingeringOf(diagram)
		chord.Voicings = append(chord.Voicings, domain.ChordVoicing{ID: voicingID(key).String(), ChordDefinitionID: chord.ID, DiagramID: diagram.ID,
			InstrumentID: diagram.InstrumentID, TuningFingerprint: "E2-A2-D3-G3-B3-E4", LowestFret: lowest, HighestFret: highest, Fingering: fingering,
			Difficulty: "beginner", RecommendedRank: i + 1, Status: domain.ChordVoicingActive})
	}
	w.chords.put(chord)
	return nil
}

func (w *world) voicingWithdrawn(key string) error {
	for _, id := range w.chords.order {
		chord := w.chords.chords[id]
		for i, v := range chord.Voicings {
			if v.ID == voicingID(key).String() {
				chord.Voicings[i].Status = domain.ChordVoicingWithdrawn
				w.chords.put(chord)
				return nil
			}
		}
	}
	return fmt.Errorf("no voicing %q in the chord catalog", key)
}

func (w *world) diagramPurposeIs(want string) error {
	diagram, err := w.currentDiagram()
	if err != nil {
		return err
	}
	if string(diagram.Purpose) != want {
		return fmt.Errorf("expected purpose %q, got %q", want, diagram.Purpose)
	}
	return nil
}

func (w *world) listsDiagramsOfPurpose(_, purpose string) error {
	p := generated.ListDiagramsParamsPurpose(purpose)
	return w.listDiagrams(generated.ListDiagramsParams{Purpose: &p})
}

func (w *world) responseIncludesDiagramAndVoicing(slug, key string) error {
	if err := w.responseIncludes(slug); err != nil {
		return err
	}
	return w.responseIncludes(voicingSlug(key))
}

func (w *world) listedVoicingDiagramHasPurpose(key, want string) error {
	resp, ok := w.lastResp.(generated.ListDiagrams200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a diagram list, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	for _, d := range resp.Items {
		if d.DiagramId == diagramID(voicingSlug(key)) {
			if string(d.Purpose) != want {
				return fmt.Errorf("expected purpose %q, got %q", want, d.Purpose)
			}
			return nil
		}
	}
	return fmt.Errorf("the diagram of voicing %q is not in the list", key)
}

func (w *world) refusedAsChordVoicing() error {
	if _, ok := w.lastResp.(generated.UpdateDiagram409JSONResponse); !ok {
		return fmt.Errorf("expected the chord catalog's refusal, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}

func (w *world) voicingDiagramUnchanged(key string) error {
	stored, err := w.diagrams.GetByID(w.ctx(), diagramID(voicingSlug(key)).String())
	if err != nil {
		return err
	}
	if stored.RootNote == nil || *stored.RootNote != "A" || stored.LabelDisplay != domain.LabelDisplayInterval {
		return fmt.Errorf("expected the diagram of voicing %q unchanged, got root %v and label display %q", key, stored.RootNote, stored.LabelDisplay)
	}
	return nil
}

func (w *world) canUpdateNewDiagram(string) error {
	created, ok := w.lastResp.(generated.CreateDiagram201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a new diagram, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	root := "G"
	resp, err := w.handler.UpdateDiagram(w.ctx(), generated.UpdateDiagramRequestObject{DiagramId: created.DiagramId, Body: &generated.UpdateDiagramRequest{RootNote: &root}})
	if err != nil {
		return err
	}
	if _, ok := resp.(generated.UpdateDiagram200JSONResponse); !ok {
		return fmt.Errorf("expected the new diagram to be updated, got %#v", resp)
	}
	return nil
}

func (w *world) responseIsVoicingDiagram(key string) error {
	if err := w.responseIsDiagram(voicingSlug(key)); err != nil {
		return err
	}
	resp := w.lastResp.(generated.GetDiagram200JSONResponse)
	if len(resp.Positions) == 0 || len(resp.Playbacks) == 0 {
		return fmt.Errorf("expected the diagram of voicing %q with its positions and playbacks, got %d and %d", key, len(resp.Positions), len(resp.Playbacks))
	}
	return nil
}

func (w *world) searchesChords(_, symbol string) error {
	resp, err := w.handler.SearchChords(w.ctx(), generated.SearchChordsRequestObject{Params: generated.SearchChordsParams{Symbol: symbol}})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) chordSearch() (generated.ChordSearchResult, error) {
	resp, ok := w.lastResp.(generated.SearchChords200JSONResponse)
	if !ok {
		return generated.ChordSearchResult{}, fmt.Errorf("expected a chord search result, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return generated.ChordSearchResult(resp), nil
}

func (w *world) symbolReadAs(status, warning string) error {
	search, err := w.chordSearch()
	if err != nil {
		return err
	}
	if string(search.Status) != status {
		return fmt.Errorf("expected the symbol read as %q, got %q", status, search.Status)
	}
	got := ""
	if search.Warning != nil {
		got = string(*search.Warning)
	}
	if got != warning {
		return fmt.Errorf("expected warning %q, got %q", warning, got)
	}
	return nil
}

// currentChord is the chord the last search found or the last read returned.
func (w *world) currentChord() (generated.ChordDefinition, error) {
	switch resp := w.lastResp.(type) {
	case generated.SearchChords200JSONResponse:
		if resp.Chord == nil {
			return generated.ChordDefinition{}, fmt.Errorf("expected a chord, found none for %q", resp.WrittenSymbol)
		}
		return *resp.Chord, nil
	case generated.GetChord200JSONResponse:
		return generated.ChordDefinition(resp), nil
	default:
		return generated.ChordDefinition{}, fmt.Errorf("expected a chord, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
}

func (w *world) chordFoundIs(symbol string) error {
	chord, err := w.currentChord()
	if err != nil {
		return err
	}
	if chord.CanonicalSymbol != symbol {
		return fmt.Errorf("expected chord %q, got %q", symbol, chord.CanonicalSymbol)
	}
	return nil
}

func (w *world) noChordFound() error {
	search, err := w.chordSearch()
	if err != nil {
		return err
	}
	if search.Chord != nil {
		return fmt.Errorf("expected no chord, got %q", search.Chord.CanonicalSymbol)
	}
	return nil
}

func (w *world) chordWithoutBassIs(symbol string) error {
	search, err := w.chordSearch()
	if err != nil {
		return err
	}
	if search.ChordWithoutBass == nil || search.ChordWithoutBass.CanonicalSymbol != symbol {
		return fmt.Errorf("expected %q offered without the bass, got %+v", symbol, search.ChordWithoutBass)
	}
	return nil
}

func (w *world) noChordWithoutBass() error {
	search, err := w.chordSearch()
	if err != nil {
		return err
	}
	if search.ChordWithoutBass != nil {
		return fmt.Errorf("expected no chord without the bass, got %q", search.ChordWithoutBass.CanonicalSymbol)
	}
	return nil
}

func (w *world) searchKeepsSymbol(written string) error {
	search, err := w.chordSearch()
	if err != nil {
		return err
	}
	if search.WrittenSymbol != written {
		return fmt.Errorf("expected the written symbol %q, got %q", written, search.WrittenSymbol)
	}
	return nil
}

func (w *world) parsedRootIs(root string) error {
	search, err := w.chordSearch()
	if err != nil {
		return err
	}
	if search.Parsed == nil || search.Parsed.Root != root {
		return fmt.Errorf("expected parsed root %q, got %+v", root, search.Parsed)
	}
	return nil
}

func (w *world) voicingsAre(keys ...string) error {
	chord, err := w.currentChord()
	if err != nil {
		return err
	}
	got := make([]uuid.UUID, len(chord.Voicings))
	for i, v := range chord.Voicings {
		got[i] = v.ChordVoicingId
	}
	want := make([]uuid.UUID, len(keys))
	for i, k := range keys {
		want[i] = voicingID(k)
	}
	if !slices.Equal(want, got) {
		return fmt.Errorf("expected voicings %v, got %v", keys, got)
	}
	return nil
}

func (w *world) retrievesChord(_, symbol string) error {
	resp, err := w.handler.GetChord(w.ctx(), generated.GetChordRequestObject{ChordDefinitionId: chordID(symbol)})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) retrievesMissingChord(string) error {
	resp, err := w.handler.GetChord(w.ctx(), generated.GetChordRequestObject{ChordDefinitionId: uuid.New()})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) chordFormulaIs(a, b, c, d string) error {
	chord, err := w.currentChord()
	if err != nil {
		return err
	}
	want := []generated.ChordInterval{generated.ChordInterval(a), generated.ChordInterval(b), generated.ChordInterval(c), generated.ChordInterval(d)}
	if !slices.Equal(want, chord.Formula) {
		return fmt.Errorf("expected formula %v, got %v", want, chord.Formula)
	}
	return nil
}

func (w *world) eachVoicingNamesItsDiagram() error {
	chord, err := w.currentChord()
	if err != nil {
		return err
	}
	for _, v := range chord.Voicings {
		d, err := w.diagrams.GetByID(w.ctx(), v.DiagramId.String())
		if err != nil {
			return fmt.Errorf("voicing %s names no diagram: %w", v.ChordVoicingId, err)
		}
		if d.Purpose != domain.DiagramPurposeChordVoicing {
			return fmt.Errorf("voicing %s names a %q diagram", v.ChordVoicingId, d.Purpose)
		}
	}
	return nil
}

func (w *world) retrievesNewDiagram(caller string) error {
	created, ok := w.lastResp.(generated.CreateDiagram201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a new diagram, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	resp, err := w.handler.GetDiagram(w.ctx(), generated.GetDiagramRequestObject{DiagramId: created.DiagramId})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) diagramCarriesVoicing(key string) error {
	diagram, err := w.currentDiagram()
	if err != nil {
		return err
	}
	voicing := diagram.ChordVoicing
	if voicing == nil {
		return fmt.Errorf("expected the diagram to carry voicing %q, got none", key)
	}
	if voicing.ChordVoicingId != voicingID(key) {
		return fmt.Errorf("expected voicing %q, got %s", key, voicing.ChordVoicingId)
	}
	if len(voicing.Fingering) == 0 || voicing.FretWindow.HighestFret == 0 {
		return fmt.Errorf("expected voicing %q with its fingering and fret window, got %+v", key, *voicing)
	}
	return nil
}

func (w *world) diagramCarriesNoVoicing() error {
	diagram, err := w.currentDiagram()
	if err != nil {
		return err
	}
	if diagram.ChordVoicing != nil {
		return fmt.Errorf("expected no voicing, got %s", diagram.ChordVoicing.ChordVoicingId)
	}
	return nil
}
