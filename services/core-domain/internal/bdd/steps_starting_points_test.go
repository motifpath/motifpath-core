//go:build integration

package bdd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// startingPointsWorld is what the starting points scenarios remember between steps.
type startingPointsWorld struct {
	// kindBySlug says whether a slug the scenario seeded is a path or a course.
	kindBySlug map[string]string
	// numbered are the paths a "N published paths" step seeded, in order.
	numbered []string
	// before is the stored starting_points value just before the last write, to tell whether a
	// refused write left it unchanged.
	before json.RawMessage
	// result is the last where-to-start answer.
	result generated.StartingPoints
}

func spCourseID(slug string) uuid.UUID { return deterministicUUID("course", slug) }

func registerStartingPointsSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^an instrument "([^"]+)" exists$`, w.spInstrumentExists)
	sc.Step(`^a learning path "([^"]+)" exists, published, level "([^"]+)", language "([^"]+)", for "([^"]+)"$`, w.spPublishedPath)
	sc.Step(`^a course "([^"]+)" exists, published, level "([^"]+)", language "([^"]+)", for "([^"]+)"$`, w.spPublishedCourse)
	sc.Step(`^the course "([^"]+)" is retired$`, w.spRetireCourse)
	sc.Step(`^(\d+) published paths in "([^"]+)" exist for "([^"]+)"$`, w.spNumberedPaths)
	sc.Step(`^(\d+) more published beginner paths in "([^"]+)" exist for "([^"]+)"$`, w.spNumberedPaths)
	sc.Step(`^user "([^"]+)" is registered as an admin$`, func(name string) error { w.ensureRegistered(name, domain.RoleAdmin); return nil })
	sc.Step(`^teacher "([^"]+)" is registered and authenticated as a teacher$`, func(name string) error { w.authenticateAs(name, domain.RoleTeacher); return nil })

	sc.Step(`^"([^"]+)" lists the app's settings$`, w.spListSettings)
	sc.Step(`^the settings include "([^"]+)", not set yet$`, w.spSettingsIncludeUnset)
	sc.Step(`^"([^"]+)" reads the setting "([^"]+)"$`, w.spReadSetting)
	sc.Step(`^"([^"]+)" sets "([^"]+)" to one list for "([^"]+)", "([^"]+)", "([^"]+)" with (.+)$`, w.spSetOneList)
	sc.Step(`^"([^"]+)" sets "([^"]+)" to one list for an instrument that does not exist$`, w.spSetUnknownInstrument)
	sc.Step(`^"([^"]+)" sets "([^"]+)" to one list without "([^"]+)"$`, w.spSetListWithout)
	sc.Step(`^"([^"]+)" sets "([^"]+)" to two lists for "([^"]+)", "([^"]+)", "([^"]+)"$`, w.spSetTwoListsSameCombo)
	sc.Step(`^"([^"]+)" sets "([^"]+)" to two lists, one without "language" and one with no entries$`, w.spSetTwoBrokenLists)
	sc.Step(`^"([^"]+)" sets "([^"]+)" to only the list for "([^"]+)", "([^"]+)", "([^"]+)"$`, w.spSetOnlyList)
	sc.Step(`^"([^"]+)" sets "([^"]+)" to any value$`, func(name, key string) error { return w.spSet(name, key, json.RawMessage(`{}`)) })
	sc.Step(`^"starting_points" holds lists for "([^"]+)", "([^"]+)", "([^"]+)" and "([^"]+)", "([^"]+)", "([^"]+)"$`, w.spHoldsTwoLists)
	sc.Step(`^"starting_points" has a list for "([^"]+)", "([^"]+)", "([^"]+)" with (.+)$`, w.spHasList)
	sc.Step(`^"starting_points" has no list for "([^"]+)", "([^"]+)", "([^"]+)"$`, w.spHasNoList)

	sc.Step(`^the setting holds that list, with (.+)$`, w.spSettingHoldsItems)
	sc.Step(`^the setting records "([^"]+)" as the last to change it$`, w.spSettingRecords)
	sc.Step(`^the setting is stored$`, w.spSettingStored)
	sc.Step(`^"starting_points" holds only the list for "([^"]+)", "([^"]+)", "([^"]+)"$`, w.spHoldsOnly)
	sc.Step(`^"starting_points" is unchanged$`, w.spUnchanged)

	sc.Step(`^"([^"]+)" asks where to start on "([^"]+)", "([^"]+)", "([^"]+)"$`, w.spAsk)
	sc.Step(`^the starting points are (.+)$`, w.spResultIs)
	sc.Step(`^the source is "([^"]+)"$`, w.spSourceIs)
	sc.Step(`^(\d+) starting points are returned, in the list's order$`, w.spNumberedInOrder)
	sc.Step(`^(\d+) starting points are returned, all of them paths$`, w.spCountAllPaths)

	sc.Step(`^the request is rejected with a validation error$`, w.requestRejectedInvalid)
	sc.Step(`^the request is rejected with a validation error naming "([^"]+)"$`, func(field string) error { return w.spRejectionNames(".", field) })
	sc.Step(`^the request is rejected with a validation error naming the list's (\w+)$`, func(field string) error { return w.spRejectionNames("lists[0].", field) })
	sc.Step(`^the request is rejected with a validation error naming the entry$`, func() error { return w.spRejectionNamesPrefix("lists[0].items[0]") })
	sc.Step(`^the request is rejected with a validation error naming the repeated entry$`, func() error { return w.rejectionIdentifiesField("lists[0].items[1]") })
	sc.Step(`^the request is rejected with a validation error naming the second list$`, func() error { return w.rejectionIdentifiesField("lists[1]") })
	sc.Step(`^the request is rejected with a validation error naming both problems$`, func() error {
		if err := w.rejectionIdentifiesField("lists[0].language"); err != nil {
			return err
		}
		return w.rejectionIdentifiesField("lists[1].items")
	})
	sc.Step(`^the request is rejected as not found$`, w.requestRefusedNotFound)
	sc.Step(`^the request is forbidden$`, w.requestRefusedForbidden)
}

// ── Seeding ────────────────────────────────────────────────────────────────

func (w *world) spInstrumentExists(name string) error {
	w.instruments.put(domain.Instrument{ID: instrumentID(name).String(), Names: domain.LocalizedText{"en": name}, Family: domain.InstrumentFamilyFretted})
	return nil
}

func (w *world) spPublishedPath(slug, level, language, instrument string) error {
	l := domain.DifficultyLevel(level)
	w.paths.put(domain.LearningPath{
		ID: pathID(slug).String(), Title: slug, Status: domain.LearningPathStatusPublished, Level: &l,
		Language: &language, InstrumentIDs: []string{instrumentID(instrument).String()},
		TeacherID: w.ensureRegistered("sp-author", domain.RoleTeacher).String(),
	})
	w.sp.kindBySlug[slug] = "path"
	return nil
}

func (w *world) spPublishedCourse(slug, level, language, instrument string) error {
	id := spCourseID(slug).String()
	instruments := []string{instrumentID(instrument).String()}
	w.courses.put(domain.Course{ID: id, Title: slug, Level: domain.DifficultyLevel(level), Language: language, InstrumentIDs: instruments,
		Status: domain.CourseStatusPublished, CreatedBy: w.ensureRegistered("sp-author", domain.RoleTeacher).String()})
	if err := w.courseVersions.Create(context.Background(), domain.CourseVersion{
		ID: deterministicUUID("course-version", slug).String(), CourseID: id, VersionNumber: 1, TitleSnapshot: slug,
		LevelSnapshot: domain.DifficultyLevel(level), LanguageSnapshot: language, InstrumentIDsSnapshot: instruments,
		PublishedAt: fixedNow, AvailableForNewEnrollments: true,
	}); err != nil {
		return err
	}
	w.sp.kindBySlug[slug] = "course"
	return nil
}

func (w *world) spRetireCourse(slug string) error {
	return w.courses.UpdateStatus(context.Background(), spCourseID(slug).String(), domain.CourseStatusRetired, time.Now())
}

func (w *world) spNumberedPaths(count int, language, instrument string) error {
	start := len(w.sp.numbered)
	for i := range count {
		slug := fmt.Sprintf("a-numbered-path-%02d", start+i)
		if err := w.spPublishedPath(slug, "beginner", language, instrument); err != nil {
			return err
		}
		w.sp.numbered = append(w.sp.numbered, slug)
	}
	return nil
}

// ── Building values ────────────────────────────────────────────────────────

type spItem struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type spListValue struct {
	InstrumentID string   `json:"instrument_id"`
	Experience   string   `json:"experience"`
	Language     string   `json:"language"`
	Items        []spItem `json:"items"`
}

var (
	spQuoted       = regexp.MustCompile(`(path|course) "([^"]+)"`)
	spKindOnly     = regexp.MustCompile(`^an entry of kind "([^"]+)"$`)
	spCarrying     = regexp.MustCompile(`^a course carrying the id of path "([^"]+)"$`)
	spTwice        = regexp.MustCompile(`^path "([^"]+)" twice$`)
	spThoseNumbers = regexp.MustCompile(`^those (\d+) paths$`)
)

func (w *world) spItemID(kind, slug string) string {
	if kind == "course" {
		return spCourseID(slug).String()
	}
	return pathID(slug).String()
}

// spItems reads a list's entries from a step's words: `course "a" then path "b"`, `no entries`,
// `path "a" twice`, `those 7 paths`, `an entry of kind "song"`, `a course carrying the id of path "a"`.
func (w *world) spItems(words string) ([]spItem, error) {
	switch {
	case words == "no entries":
		return []spItem{}, nil
	case spKindOnly.MatchString(words):
		return []spItem{{Kind: spKindOnly.FindStringSubmatch(words)[1], ID: pathID("anything").String()}}, nil
	case spCarrying.MatchString(words):
		return []spItem{{Kind: "course", ID: pathID(spCarrying.FindStringSubmatch(words)[1]).String()}}, nil
	case spTwice.MatchString(words):
		id := pathID(spTwice.FindStringSubmatch(words)[1]).String()
		return []spItem{{Kind: "path", ID: id}, {Kind: "path", ID: id}}, nil
	case spThoseNumbers.MatchString(words):
		n, err := strconv.Atoi(spThoseNumbers.FindStringSubmatch(words)[1])
		if err != nil {
			return nil, err
		}
		if n != len(w.sp.numbered) {
			return nil, fmt.Errorf("asked for those %d paths, but %d were seeded", n, len(w.sp.numbered))
		}
		items := make([]spItem, n)
		for i, slug := range w.sp.numbered {
			items[i] = spItem{Kind: "path", ID: pathID(slug).String()}
		}
		return items, nil
	}
	matches := spQuoted.FindAllStringSubmatch(words, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("can't read entries from %q", words)
	}
	items := make([]spItem, len(matches))
	for i, m := range matches {
		items[i] = spItem{Kind: m[1], ID: w.spItemID(m[1], m[2])}
	}
	return items, nil
}

func spListOf(instrument, experience, language string, items []spItem) spListValue {
	return spListValue{InstrumentID: instrumentID(instrument).String(), Experience: experience, Language: language, Items: items}
}

func spDocument(lists ...spListValue) (json.RawMessage, error) {
	return json.Marshal(map[string][]spListValue{"lists": lists})
}

// ── Settings API ───────────────────────────────────────────────────────────

func (w *world) spStored() json.RawMessage {
	entry, err := w.appConfig.Get(context.Background(), "starting_points")
	if err != nil {
		return nil
	}
	return entry.Value
}

func (w *world) spSet(name, key string, value json.RawMessage) error {
	w.sp.before = w.spStored()
	body := generated.AppConfigValue{}
	if err := body.UnmarshalJSON(value); err != nil {
		return err
	}
	w.lastResp, w.lastErr = w.handler.SetAppConfig(w.identityCtx(name), generated.SetAppConfigRequestObject{Key: key, Body: &body})
	return nil
}

func (w *world) spSetOneList(name, key, instrument, experience, language, words string) error {
	items, err := w.spItems(words)
	if err != nil {
		return err
	}
	value, err := spDocument(spListOf(instrument, experience, language, items))
	if err != nil {
		return err
	}
	return w.spSet(name, key, value)
}

func (w *world) spSetUnknownInstrument(name, key string) error {
	return w.spSetOneList(name, key, "no-such-instrument", "never_played", "en", `path "eg-from-zero"`)
}

func (w *world) spSetListWithout(name, key, field string) error {
	list := map[string]json.RawMessage{
		"instrument_id": json.RawMessage(strconv.Quote(instrumentID("electric-guitar").String())),
		"experience":    json.RawMessage(`"never_played"`),
		"language":      json.RawMessage(`"en"`),
		"items":         json.RawMessage(fmt.Sprintf(`[{"kind":"path","id":%q}]`, pathID("eg-from-zero").String())),
	}
	if _, ok := list[field]; !ok {
		return fmt.Errorf("a list has no field %q", field)
	}
	delete(list, field)
	value, err := json.Marshal(map[string][]map[string]json.RawMessage{"lists": {list}})
	if err != nil {
		return err
	}
	return w.spSet(name, key, value)
}

func (w *world) spSetTwoListsSameCombo(name, key, instrument, experience, language string) error {
	value, err := spDocument(
		spListOf(instrument, experience, language, []spItem{{Kind: "path", ID: pathID("eg-from-zero").String()}}),
		spListOf(instrument, experience, language, []spItem{{Kind: "course", ID: spCourseID("eg-basics").String()}}))
	if err != nil {
		return err
	}
	return w.spSet(name, key, value)
}

func (w *world) spSetTwoBrokenLists(name, key string) error {
	value := json.RawMessage(fmt.Sprintf(`{"lists":[{"instrument_id":%q,"experience":"never_played","items":[{"kind":"path","id":%q}]},`+
		`{"instrument_id":%q,"experience":"plays_well","language":"en","items":[]}]}`,
		instrumentID("electric-guitar").String(), pathID("eg-from-zero").String(), instrumentID("electric-guitar").String()))
	return w.spSet(name, key, value)
}

func (w *world) spSetOnlyList(name, key, instrument, experience, language string) error {
	return w.spSetOneList(name, key, instrument, experience, language, `path "eg-from-zero"`)
}

// spSeed stores lists through the admin API, as an admin of its own, so a Given step starts from a
// value the validator accepted.
func (w *world) spSeed(lists ...spListValue) error {
	w.ensureRegistered("sp-seeding-admin", domain.RoleAdmin)
	value, err := spDocument(lists...)
	if err != nil {
		return err
	}
	if err := w.spSet("sp-seeding-admin", "starting_points", value); err != nil {
		return err
	}
	if _, ok := w.lastResp.(generated.SetAppConfig200JSONResponse); !ok {
		return fmt.Errorf("setup: seeding starting points was refused: %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}

func (w *world) spHoldsTwoLists(i1, e1, l1, i2, e2, l2 string) error {
	return w.spSeed(
		spListOf(i1, e1, l1, []spItem{{Kind: "path", ID: pathID("eg-from-zero").String()}}),
		spListOf(i2, e2, l2, []spItem{{Kind: "course", ID: spCourseID("eg-basics").String()}}))
}

func (w *world) spHasList(instrument, experience, language, words string) error {
	items, err := w.spItems(words)
	if err != nil {
		return err
	}
	return w.spSeed(spListOf(instrument, experience, language, items))
}

func (w *world) spHasNoList(_, _, _ string) error {
	if stored := w.spStored(); stored != nil {
		return fmt.Errorf("expected no starting points yet, found %s", stored)
	}
	return nil
}

func (w *world) spListSettings(name string) error {
	w.lastResp, w.lastErr = w.handler.ListAppConfig(w.identityCtx(name), generated.ListAppConfigRequestObject{})
	return nil
}

func (w *world) spSettingsIncludeUnset(key string) error {
	resp, ok := w.lastResp.(generated.ListAppConfig200JSONResponse)
	if !ok {
		return fmt.Errorf("expected the settings, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	for _, s := range resp {
		if s.Key == key {
			if s.IsSet || s.UpdatedAt != nil || s.UpdatedBy != nil {
				return fmt.Errorf("%q is listed as set: %+v", key, s)
			}
			return nil
		}
	}
	return fmt.Errorf("%q is not among the settings %+v", key, resp)
}

func (w *world) spReadSetting(name, key string) error {
	w.lastResp, w.lastErr = w.handler.GetAppConfig(w.identityCtx(name), generated.GetAppConfigRequestObject{Key: key})
	return nil
}

func (w *world) spReadEntry() (generated.AppConfigEntry, error) {
	switch resp := w.lastResp.(type) {
	case generated.GetAppConfig200JSONResponse:
		return generated.AppConfigEntry(resp), nil
	case generated.SetAppConfig200JSONResponse:
		return generated.AppConfigEntry(resp), nil
	}
	return generated.AppConfigEntry{}, fmt.Errorf("expected a setting, got %#v (err=%v)", w.lastResp, w.lastErr)
}

func (w *world) spSettingHoldsItems(words string) error {
	entry, err := w.spReadEntry()
	if err != nil {
		return err
	}
	want, err := w.spItems(words)
	if err != nil {
		return err
	}
	if entry.Value == nil {
		return fmt.Errorf("the setting has no value")
	}
	config, err := entry.Value.AsStartingPointsConfig()
	if err != nil {
		return err
	}
	if len(config.Lists) != 1 {
		return fmt.Errorf("expected one list, got %+v", config.Lists)
	}
	got := make([]spItem, len(config.Lists[0].Items))
	for i, item := range config.Lists[0].Items {
		got[i] = spItem{Kind: string(item.Kind), ID: item.Id.String()}
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		return fmt.Errorf("the list holds %v, want %v", got, want)
	}
	return nil
}

func (w *world) spSettingRecords(name string) error {
	entry, err := w.spReadEntry()
	if err != nil {
		return err
	}
	if entry.UpdatedBy == nil || entry.UpdatedBy.UserId != w.userMotifID[name] {
		return fmt.Errorf("the setting records %+v as the last to change it, want %q", entry.UpdatedBy, name)
	}
	if entry.UpdatedAt == nil {
		return fmt.Errorf("the setting records no time it was changed")
	}
	return nil
}

func (w *world) spSettingStored() error {
	if _, ok := w.lastResp.(generated.SetAppConfig200JSONResponse); !ok {
		return fmt.Errorf("expected the setting to be stored, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}

func (w *world) spHoldsOnly(instrument, experience, language string) error {
	var stored struct {
		Lists []spListValue `json:"lists"`
	}
	if err := json.Unmarshal(w.spStored(), &stored); err != nil {
		return err
	}
	if len(stored.Lists) != 1 {
		return fmt.Errorf("expected one list, got %+v", stored.Lists)
	}
	l := stored.Lists[0]
	if l.InstrumentID != instrumentID(instrument).String() || l.Experience != experience || l.Language != language {
		return fmt.Errorf("the only list is %+v", l)
	}
	return nil
}

func (w *world) spUnchanged() error {
	if !bytes.Equal(w.spStored(), w.sp.before) {
		return fmt.Errorf("starting_points changed from %s to %s", w.sp.before, w.spStored())
	}
	return nil
}

func (w *world) spRejectionNames(prefix, field string) error {
	if prefix == "." {
		return w.spRejectionNamesSuffix("." + field)
	}
	return w.rejectionIdentifiesField(prefix + field)
}

func (w *world) spRejectionNamesSuffix(suffix string) error {
	errs, err := w.validationErrors()
	if err != nil {
		return err
	}
	for _, e := range errs {
		if strings.HasSuffix(e.Field, suffix) {
			return nil
		}
	}
	return fmt.Errorf("no validation error named a field ending in %q; got %+v", suffix, errs)
}

func (w *world) spRejectionNamesPrefix(prefix string) error {
	errs, err := w.validationErrors()
	if err != nil {
		return err
	}
	for _, e := range errs {
		if strings.HasPrefix(e.Field, prefix) {
			return nil
		}
	}
	return fmt.Errorf("no validation error named a field under %q; got %+v", prefix, errs)
}

// ── Where to start ─────────────────────────────────────────────────────────

func (w *world) spAsk(name, instrument, experience, language string) error {
	w.ensureRegistered(name, domain.RoleStudent)
	w.lastResp, w.lastErr = w.handler.ListStartingPoints(w.identityCtx(name), generated.ListStartingPointsRequestObject{
		Params: generated.ListStartingPointsParams{
			InstrumentId: instrumentID(instrument), Experience: generated.StartingPointExperience(experience), Language: language,
		},
	})
	if resp, ok := w.lastResp.(generated.ListStartingPoints200JSONResponse); ok {
		w.sp.result = generated.StartingPoints(resp)
	}
	return nil
}

func (w *world) spResultItems() ([]spItem, error) {
	if _, ok := w.lastResp.(generated.ListStartingPoints200JSONResponse); !ok {
		return nil, fmt.Errorf("expected where to start, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	items := make([]spItem, len(w.sp.result.Items))
	for i, item := range w.sp.result.Items {
		switch {
		case item.Kind == generated.StartingPointKindPath && item.Path != nil:
			items[i] = spItem{Kind: "path", ID: item.Path.LearningPathId.String()}
		case item.Kind == generated.StartingPointKindCourse && item.Course != nil:
			items[i] = spItem{Kind: "course", ID: item.Course.CourseId.String()}
		default:
			return nil, fmt.Errorf("starting point %d is a %q without its entry: %+v", i, item.Kind, item)
		}
	}
	return items, nil
}

func (w *world) spResultIs(words string) error {
	got, err := w.spResultItems()
	if err != nil {
		return err
	}
	want, err := w.spItems(strings.TrimSuffix(words, " only"))
	if err != nil {
		return err
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		return fmt.Errorf("the starting points are %v, want %v", got, want)
	}
	return nil
}

func (w *world) spSourceIs(source string) error {
	if _, err := w.spResultItems(); err != nil {
		return err
	}
	if string(w.sp.result.Source) != source {
		return fmt.Errorf("the source is %q, want %q", w.sp.result.Source, source)
	}
	return nil
}

func (w *world) spNumberedInOrder(count int) error {
	got, err := w.spResultItems()
	if err != nil {
		return err
	}
	want, err := w.spItems(fmt.Sprintf("those %d paths", count))
	if err != nil {
		return err
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		return fmt.Errorf("the starting points are %v, want %v", got, want)
	}
	return nil
}

func (w *world) spCountAllPaths(count int) error {
	got, err := w.spResultItems()
	if err != nil {
		return err
	}
	if len(got) != count {
		return fmt.Errorf("%d starting points returned, want %d", len(got), count)
	}
	for _, item := range got {
		if item.Kind != "path" {
			return fmt.Errorf("a %s is among them: %v", item.Kind, got)
		}
	}
	return nil
}
