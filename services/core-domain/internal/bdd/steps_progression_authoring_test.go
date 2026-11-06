//go:build integration

package bdd

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

const quotedList = `((?:"[^"]+"(?:, | and )?)+)`

var quotedItem = regexp.MustCompile(`"([^"]+)"`)

func quotedItems(text string) []string {
	matches := quotedItem.FindAllStringSubmatch(text, -1)
	items := make([]string, len(matches))
	for i, m := range matches {
		items[i] = m[1]
	}
	return items
}

// Admins create progressions for song charts, name them and remove them.
func registerProgressionAuthoringSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)" creates a (major|minor) progression with the bars `+quotedList+`$`, func(admin, mode, bars string) error {
		return w.createsProgression(admin, mode, quotedItems(bars), nil)
	})
	sc.Step(`^"([^"]+)" creates a (major|minor) progression with the bars `+quotedList+` and the nicknames "([^"]+)" in "([^"]+)" and "([^"]+)" in "([^"]+)"$`,
		func(admin, mode, bars, one, oneLang, other, otherLang string) error {
			return w.createsProgression(admin, mode, quotedItems(bars), map[string]string{oneLang: one, otherLang: other})
		})
	sc.Step(`^"([^"]+)" creates a major progression of (\d+) bars of "([^"]+)"$`, func(admin string, count int, numeral string) error {
		return w.createsProgression(admin, "major", slices.Repeat([]string{numeral}, count), nil)
	})
	sc.Step(`^"([^"]+)" can create a major progression with the bars `+quotedList+` again$`, func(admin, bars string) error {
		if err := w.createsProgression(admin, "major", quotedItems(bars), nil); err != nil {
			return err
		}
		_, err := w.createdProgression()
		return err
	})
	sc.Step(`^"([^"]+)" has created the (major|minor) progression "([^"]+)"$`, func(admin, mode, numerals string) error {
		return w.hasCreatedProgression(admin, mode, numerals, nil)
	})
	sc.Step(`^"([^"]+)" has created the major progression "([^"]+)" with the nickname "([^"]+)" in "([^"]+)"$`, func(admin, numerals, nickname, language string) error {
		return w.hasCreatedProgression(admin, "major", numerals, map[string]string{language: nickname})
	})

	sc.Step(`^the progression "([^"]+)" exists with the bars `+quotedList+`$`, w.progressionKeyAndBars)
	sc.Step(`^the progression "([^"]+)" has the bars `+quotedList+`$`, w.progressionKeyAndBars)
	sc.Step(`^the progression "([^"]+)" is created$`, func(key string) error {
		_, err := w.storedProgression(key)
		return err
	})
	sc.Step(`^the progression is created with (\d+) bars$`, func(count int) error {
		created, err := w.createdProgression()
		if err != nil {
			return err
		}
		if len(created.Bars) != count {
			return fmt.Errorf("it has %d bars", len(created.Bars))
		}
		return nil
	})
	sc.Step(`^it is not in the drills$`, func() error {
		created, err := w.createdProgression()
		if err != nil {
			return err
		}
		if created.InDrills {
			return fmt.Errorf("%s is in the drills", created.Key)
		}
		return nil
	})
	sc.Step(`^its name is "([^"]+)" in every language$`, func(name string) error {
		created, err := w.createdProgression()
		if err != nil {
			return err
		}
		for _, language := range []string{"en", "pt_BR"} {
			if created.Names[language] != name {
				return fmt.Errorf("its names are %v", created.Names)
			}
		}
		return nil
	})
	sc.Step(`^its name is "([^"]+)" in "([^"]+)" and "([^"]+)" in "([^"]+)"$`, func(one, oneLang, other, otherLang string) error {
		return w.currentProgressionNamed(map[string]string{oneLang: one, otherLang: other})
	})
	sc.Step(`^its name is "([^"]+)" in "([^"]+)"$`, func(name, language string) error {
		return w.currentProgressionNamed(map[string]string{language: name})
	})
	sc.Step(`^its bar (\d+) is borrowed and no other bar is$`, func(bar int) error {
		created, err := w.createdProgression()
		if err != nil {
			return err
		}
		if !slices.Equal(created.BorrowedBars, []int{bar - 1}) {
			return fmt.Errorf("the borrowed bars are %v", created.BorrowedBars)
		}
		return nil
	})

	sc.Step(`^"([^"]+)" changes its nickname to "([^"]+)" in "([^"]+)"$`, func(admin, nickname, language string) error {
		return w.changesNickname(admin, w.authoredProgressionKey(), nickname, language)
	})
	sc.Step(`^"([^"]+)" changes the nickname of "([^"]+)" to "([^"]+)" in "([^"]+)"$`, w.changesNickname)
	sc.Step(`^its key is still "([^"]+)"$`, func(key string) error {
		updated, ok := w.lastResp.(generated.UpdateProgression200JSONResponse)
		if !ok {
			return fmt.Errorf("expected the changed progression, got %#v (err=%v)", w.lastResp, w.lastErr)
		}
		if updated.Key != key {
			return fmt.Errorf("its key is %s", updated.Key)
		}
		return nil
	})
	sc.Step(`^the change is refused because the progression is the catalog's$`, func() error {
		if _, ok := w.lastResp.(generated.UpdateProgression409JSONResponse); !ok {
			return fmt.Errorf("expected a conflict, got %#v (err=%v)", w.lastResp, w.lastErr)
		}
		return nil
	})

	sc.Step(`^a student lists the progressions$`, func() error {
		w.authenticateAs("student-lister", domain.RoleStudent)
		return w.listsProgressionCatalog("")
	})
	sc.Step(`^the list has the catalog's (\d+) progressions in the drills$`, func(count int) error {
		list, err := w.listedProgressions()
		if err != nil {
			return err
		}
		inDrills := 0
		for _, p := range list {
			if p.InDrills {
				inDrills++
			}
		}
		if inDrills != count {
			return fmt.Errorf("%d progressions are in the drills", inDrills)
		}
		return nil
	})
	sc.Step(`^it has "([^"]+)", not in the drills, with no wrong answers$`, func(key string) error {
		p, err := w.listedProgression(key)
		if err != nil {
			return err
		}
		if p.InDrills || len(p.Confusable) != 0 {
			return fmt.Errorf("%s: in drills %v, wrong answers %v", key, p.InDrills, p.Confusable)
		}
		return nil
	})

	sc.Step(`^the creation is refused because the progression exists$`, func() error {
		_, err := w.existingProgression()
		return err
	})
	sc.Step(`^the refusal returns the progression "([^"]+)"$`, func(key string) error {
		existing, err := w.existingProgression()
		if err != nil {
			return err
		}
		if existing.Key != key {
			return fmt.Errorf("it returned %s", existing.Key)
		}
		return nil
	})
	sc.Step(`^the creation is refused as invalid, naming bar (\d+)$`, func(bar int) error {
		return w.creationRefusedAsInvalid(fmt.Sprintf("bars/%d", bar-1))
	})
	sc.Step(`^the creation is refused as invalid$`, func() error { return w.creationRefusedAsInvalid("bars") })
	sc.Step(`^no progression is created$`, func() error {
		if len(w.progressions.created) != 0 {
			return fmt.Errorf("created %d progressions", len(w.progressions.created))
		}
		return nil
	})
	sc.Step(`^the request is refused because only admins create progressions$`, func() error {
		if _, ok := w.lastResp.(generated.CreateProgression403JSONResponse); !ok {
			return fmt.Errorf("expected a refusal, got %#v (err=%v)", w.lastResp, w.lastErr)
		}
		return nil
	})

	sc.Step(`^a published song chart "([^"]+)" whose verse is tagged with "([^"]+)"$`, w.publishedChartTagged)
	sc.Step(`^student "([^"]+)" starts a practice session$`, func(name string) error {
		if err := w.studentPlaysOne(name, "guitar"); err != nil {
			return err
		}
		return w.listSubjects(nil)
	})
	sc.Step(`^no practice subject or song is on "([^"]+)"$`, func(key string) error {
		if _, err := w.subjectList(); err != nil {
			return err
		}
		drills, err := w.progressions.ListInDrills(context.Background())
		if err != nil {
			return err
		}
		if slices.ContainsFunc(drills, func(p domain.Progression) bool { return p.Key == key }) {
			return fmt.Errorf("%s reaches the drills", key)
		}
		return nil
	})

	sc.Step(`^the verse of the draft of "([^"]+)" is tagged with "([^"]+)"$`, func(title, key string) error {
		return w.draftWithTags(title, key, false)
	})
	sc.Step(`^the verse and the chorus of the draft of "([^"]+)" are tagged with "([^"]+)"$`, func(title, key string) error {
		return w.draftWithTags(title, key, true)
	})
	sc.Step(`^"([^"]+)" untags the verse of "([^"]+)"$`, func(admin, title string) error {
		if err := w.tagsVerse(admin, title); err != nil {
			return err
		}
		return w.mustSucceed("untagging " + title)
	})
	sc.Step(`^the latest published revision of "([^"]+)" tags its verse with "([^"]+)"$`, w.publishedChartTagged)
	sc.Step(`^the draft of "([^"]+)" no longer tags it$`, func(title string) error {
		if err := w.tagsVerse(songChartSeeder, title); err != nil {
			return err
		}
		return w.mustSucceed("untagging " + title)
	})
	sc.Step(`^revision 1 of "([^"]+)" tags its verse with "([^"]+)"$`, w.publishedChartTagged)
	sc.Step(`^revision 2, the latest, and the draft of "([^"]+)" don't tag it$`, func(title string) error {
		if err := w.tagsVerse(songChartSeeder, title); err != nil {
			return err
		}
		if err := w.mustSucceed("untagging " + title); err != nil {
			return err
		}
		return w.seedPublication(title)
	})
	sc.Step(`^revision 1 of "([^"]+)" still tags its verse with "([^"]+)"$`, func(title, key string) error {
		rev, err := w.songChartRepo.GetRevision(context.Background(), w.songCharts.idByTitle[title], 1)
		if err != nil {
			return err
		}
		if !slices.ContainsFunc(rev.SectionProgressions, func(t domain.SectionProgression) bool { return t.ProgressionKey == key }) {
			return fmt.Errorf("revision 1 tags %v", rev.SectionProgressions)
		}
		return nil
	})

	sc.Step(`^"([^"]+)" removes the progression "([^"]+)"$`, w.removesProgression)
	sc.Step(`^it is no longer listed$`, func() error { return w.removedProgressionIsGone() })
	sc.Step(`^the progression is removed$`, func() error { return w.removedProgressionIsGone() })
	sc.Step(`^the progression "([^"]+)" is still listed$`, func(key string) error {
		_, err := w.storedProgression(key)
		return err
	})
	sc.Step(`^removing is refused because it is in use$`, func() error {
		_, err := w.removalRefused(generated.InUse)
		return err
	})
	sc.Step(`^removing is refused because it is in the drills$`, func() error {
		_, err := w.removalRefused(generated.InDrills)
		return err
	})
	sc.Step(`^the refusal lists the verse and the chorus of the draft of "([^"]+)"$`, func(title string) error {
		return w.refusalLists(title, generated.ProgressionUseRevisionDraft, 0, 1)
	})
	sc.Step(`^the refusal lists the verse of the published revision of "([^"]+)"$`, func(title string) error {
		return w.refusalLists(title, generated.ProgressionUseRevisionPublished, 0)
	})
	sc.Step(`^the request is refused because only admins remove progressions$`, func() error {
		if _, ok := w.lastResp.(generated.DeleteProgression403JSONResponse); !ok {
			return fmt.Errorf("expected a refusal, got %#v (err=%v)", w.lastResp, w.lastErr)
		}
		return nil
	})
}

func (w *world) createsProgression(admin, mode string, bars []string, nicknames map[string]string) error {
	input := generated.ProgressionInput{Mode: generated.ProgressionInputMode(mode), Bars: bars}
	if nicknames != nil {
		input.Nicknames = &nicknames
	}
	w.authenticateAs(admin, domain.RoleAdmin)
	resp, err := w.handler.CreateProgression(w.ctx(), generated.CreateProgressionRequestObject{Body: &input})
	w.lastResp, w.lastErr = resp, err
	if created, ok := resp.(generated.CreateProgression201JSONResponse); ok {
		w.authoredKey = created.Key
	}
	return nil
}

// hasCreatedProgression creates it from numerals joined by "-", as a key spells them.
func (w *world) hasCreatedProgression(admin, mode, numerals string, nicknames map[string]string) error {
	if err := w.createsProgression(admin, mode, strings.Split(numerals, "-"), nicknames); err != nil {
		return err
	}
	_, err := w.createdProgression()
	return err
}

func (w *world) createdProgression() (generated.Progression, error) {
	created, ok := w.lastResp.(generated.CreateProgression201JSONResponse)
	if !ok {
		return generated.Progression{}, fmt.Errorf("expected a created progression, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return generated.Progression(created), nil
}

func (w *world) authoredProgressionKey() string { return w.authoredKey }

func (w *world) storedProgression(key string) (domain.Progression, error) {
	return w.progressions.find(context.Background(), func(p domain.Progression) bool { return p.Key == key })
}

func (w *world) progressionKeyAndBars(key, bars string) error {
	stored, err := w.storedProgression(key)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	if want := quotedItems(bars); !slices.Equal(stored.Bars, want) {
		return fmt.Errorf("its bars are %v, not %v", stored.Bars, want)
	}
	return nil
}

func (w *world) currentProgressionNamed(want map[string]string) error {
	var names generated.LocalizedNames
	switch resp := w.lastResp.(type) {
	case generated.CreateProgression201JSONResponse:
		names = resp.Names
	case generated.UpdateProgression200JSONResponse:
		names = resp.Names
	default:
		return fmt.Errorf("expected a progression, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	for language, name := range want {
		if names[language] != name {
			return fmt.Errorf("its names are %v", names)
		}
	}
	return nil
}

func (w *world) changesNickname(admin, key, nickname, language string) error {
	stored, err := w.storedProgression(key)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	w.authenticateAs(admin, domain.RoleAdmin)
	resp, err := w.handler.UpdateProgression(w.ctx(), generated.UpdateProgressionRequestObject{
		ProgressionId: uuid.MustParse(stored.ID),
		Body:          &generated.ProgressionNicknamesInput{Nicknames: map[string]string{language: nickname}},
	})
	w.lastResp, w.lastErr = resp, err
	return nil
}

func (w *world) existingProgression() (generated.Progression, error) {
	refused, ok := w.lastResp.(generated.CreateProgression409JSONResponse)
	if !ok {
		return generated.Progression{}, fmt.Errorf("expected a refusal, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	exists, err := refused.AsProgressionExistsError()
	if err != nil {
		return generated.Progression{}, err
	}
	return exists.Progression, nil
}

func (w *world) creationRefusedAsInvalid(field string) error {
	refused, ok := w.lastResp.(generated.CreateProgression400JSONResponse)
	if !ok {
		return fmt.Errorf("expected a validation error, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	for _, f := range refused.Errors {
		if f.Field == field {
			return nil
		}
	}
	return fmt.Errorf("the refusal names %+v, not %s", refused.Errors, field)
}

// chartPlaying is a chart in D whose verse and chorus sound I–♭VII–IV–I.
func (w *world) chartPlaying(title string) error {
	for _, symbol := range []string{"D", "C", "G"} {
		if err := w.seedCatalogChord(symbol, openVoicingKey(symbol)); err != nil {
			return err
		}
	}
	key := "D"
	return w.hasChartWithChords(songChartSeeder, title, &key, 0, true, "D C G D", "D C G D")
}

func (w *world) draftWithTags(title, key string, chorusToo bool) error {
	if err := w.chartPlaying(title); err != nil {
		return err
	}
	err := w.changeDraft(songChartSeeder, title, func(_ *generated.SongChartDraftInput, doc *domain.SongChartDocument) {
		doc.Sections[0].ProgressionKeys = []string{key}
		if chorusToo {
			doc.Sections[1].ProgressionKeys = []string{key}
		}
	})
	if err != nil {
		return err
	}
	return w.mustSucceed("tagging " + title)
}

func (w *world) publishedChartTagged(title, key string) error {
	if err := w.draftWithTags(title, key, false); err != nil {
		return err
	}
	return w.seedPublication(title)
}

func (w *world) removesProgression(admin, key string) error {
	stored, err := w.storedProgression(key)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	w.authoredKey = key
	w.authenticateAs(admin, domain.RoleAdmin)
	resp, err := w.handler.DeleteProgression(w.ctx(), generated.DeleteProgressionRequestObject{ProgressionId: uuid.MustParse(stored.ID)})
	w.lastResp, w.lastErr = resp, err
	return nil
}

func (w *world) removedProgressionIsGone() error {
	if _, ok := w.lastResp.(generated.DeleteProgression204Response); !ok {
		return fmt.Errorf("expected a removal, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if _, err := w.storedProgression(w.authoredKey); err == nil {
		return fmt.Errorf("%s is still stored", w.authoredKey)
	}
	return nil
}

func (w *world) removalRefused(reason generated.ProgressionInUseErrorReason) (generated.DeleteProgression409JSONResponse, error) {
	refused, ok := w.lastResp.(generated.DeleteProgression409JSONResponse)
	if !ok {
		return refused, fmt.Errorf("expected a refusal, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if refused.Reason != reason {
		return refused, fmt.Errorf("the reason is %s", refused.Reason)
	}
	return refused, nil
}

func (w *world) refusalLists(title string, revision generated.ProgressionUseRevision, sections ...int) error {
	refused, err := w.removalRefused(generated.InUse)
	if err != nil {
		return err
	}
	var got []int
	for _, use := range refused.Uses {
		if use.Title != title || use.Revision != revision {
			return fmt.Errorf("the refusal lists %+v", refused.Uses)
		}
		got = append(got, use.SectionIndex)
	}
	slices.Sort(got)
	if !slices.Equal(got, sections) {
		return fmt.Errorf("the refusal lists sections %v", got)
	}
	return nil
}
