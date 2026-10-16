//go:build integration

package bdd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
)

func registerProgressionCatalogSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)" lists the progression catalog$`, w.listsProgressionCatalog)
	sc.Step(`^someone who is not signed in lists the progression catalog$`, w.anonymousListsProgressionCatalog)
	sc.Step(`^it lists (\d+) progressions, ordered by key$`, w.listsProgressionsByKey)
	sc.Step(`^"([^"]+)" has the bars (.+) and the name "([^"]+)"$`, w.progressionHasBarsAndName)
	sc.Step(`^"([^"]+)" has (\d+) bars, starting (.+)$`, w.progressionHasBarsStarting)
	sc.Step(`^its name is "([^"]+)"$`, w.progressionNameIs)
	sc.Step(`^"([^"]+)" lists (.+) as confusable$`, w.progressionListsConfusable)
	sc.Step(`^the request is refused as unauthenticated$`, w.progressionsRefusedUnauthenticated)
}

func (w *world) listsProgressionCatalog(string) error {
	resp, err := w.handler.ListProgressions(w.ctx(), generated.ListProgressionsRequestObject{})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) anonymousListsProgressionCatalog() error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.listsProgressionCatalog("")
}

func (w *world) listedProgressions() (generated.ListProgressions200JSONResponse, error) {
	list, ok := w.lastResp.(generated.ListProgressions200JSONResponse)
	if !ok {
		return nil, fmt.Errorf("expected the progression catalog, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return list, nil
}

func (w *world) listedProgression(key string) (generated.Progression, error) {
	list, err := w.listedProgressions()
	if err != nil {
		return generated.Progression{}, err
	}
	i := slices.IndexFunc(list, func(p generated.Progression) bool { return p.Key == key })
	if i < 0 {
		return generated.Progression{}, fmt.Errorf("%q is not listed", key)
	}
	w.harmony().listedKey = key
	return list[i], nil
}

// listOf reads "a, b and c" as its items.
func listOf(text string) []string {
	return strings.FieldsFunc(strings.ReplaceAll(text, " and ", ","), func(r rune) bool { return r == ',' || r == ' ' || r == '"' })
}

func (w *world) listsProgressionsByKey(count int) error {
	list, err := w.listedProgressions()
	if err != nil {
		return err
	}
	keys := make([]string, len(list))
	for i, p := range list {
		keys[i] = p.Key
	}
	if len(keys) != count || !slices.IsSorted(keys) {
		return fmt.Errorf("it lists %v", keys)
	}
	return nil
}

func (w *world) progressionHasBarsAndName(key, bars, name string) error {
	p, err := w.listedProgression(key)
	if err != nil {
		return err
	}
	if !slices.Equal(p.Bars, listOf(bars)) || p.Names["en"] != name {
		return fmt.Errorf("%s has bars %v and name %q", key, p.Bars, p.Names["en"])
	}
	return nil
}

func (w *world) progressionHasBarsStarting(key string, count int, start string) error {
	p, err := w.listedProgression(key)
	if err != nil {
		return err
	}
	want := listOf(start)
	if len(p.Bars) != count || !slices.Equal(p.Bars[:len(want)], want) {
		return fmt.Errorf("%s has bars %v", key, p.Bars)
	}
	return nil
}

func (w *world) progressionNameIs(name string) error {
	p, err := w.listedProgression(w.harmony().listedKey)
	if err != nil {
		return err
	}
	if p.Names["en"] != name {
		return fmt.Errorf("its name is %q", p.Names["en"])
	}
	return nil
}

func (w *world) progressionListsConfusable(key, confusable string) error {
	p, err := w.listedProgression(key)
	if err != nil {
		return err
	}
	want := listOf(confusable)
	if !slices.Equal(p.Confusable, want) {
		return fmt.Errorf("%s lists %v as confusable, want %v", key, p.Confusable, want)
	}
	return nil
}

func (w *world) progressionsRefusedUnauthenticated() error {
	if _, ok := w.lastResp.(generated.ListProgressions401JSONResponse); !ok {
		return fmt.Errorf("expected the request refused as unauthenticated, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}
