//go:build integration

package bdd

import (
	"context"
	"fmt"
	"slices"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerItemInstrumentSteps(sc *godog.ScenarioContext, w *world) {
	// Courses.
	sc.Step(`^"([^"]+)" creates a course titled "([^"]+)" for instruments ((?:"[^"]+"(?:, )?)+) with checkpoints in order: "([^"]+)"$`, func(_, title, instruments, pathSlug string) error {
		return w.createCourseFor(title, instrumentIDsNamed(quotedValues(instruments)), pathSlug)
	})
	sc.Step(`^"([^"]+)" creates a course titled "([^"]+)" for every instrument with checkpoints in order: "([^"]+)"$`, func(_, title, pathSlug string) error {
		return w.createCourseFor(title, []uuid.UUID{}, pathSlug)
	})
	sc.Step(`^the course is for instruments ((?:"[^"]+"(?:, )?)+)$`, func(instruments string) error {
		return w.lastItemIsFor(instrumentIDsNamed(quotedValues(instruments)))
	})
	sc.Step(`^the course is for every instrument$`, func() error { return w.lastItemIsFor(nil) })
	sc.Step(`^a course "([^"]+)" exists, published for instruments ((?:"[^"]+"(?:, )?)+), with checkpoints "([^"]+)"$`, func(courseSlug, instruments, pathSlug string) error {
		return w.seedCourseFor(courseSlug, instrumentIDsNamed(quotedValues(instruments)), "bob", pathSlug)
	})
	sc.Step(`^a course "([^"]+)" exists, published for instruments ((?:"[^"]+"(?:, )?)+), created by "([^"]+)", with checkpoints "([^"]+)"$`, func(courseSlug, instruments, creator, pathSlug string) error {
		return w.seedCourseFor(courseSlug, instrumentIDsNamed(quotedValues(instruments)), creator, pathSlug)
	})
	sc.Step(`^a course "([^"]+)" exists, published for every instrument, with checkpoints "([^"]+)"$`, func(courseSlug, pathSlug string) error {
		return w.seedCourseFor(courseSlug, []uuid.UUID{}, "bob", pathSlug)
	})
	sc.Step(`^"([^"]+)" replaces course "([^"]+)" setting its instruments to ((?:"[^"]+"(?:, )?)+)$`, func(_, courseSlug, instruments string) error {
		w.lastCourseSlug = courseSlug
		return w.replaceCourseInstruments(courseSlug, instrumentIDsNamed(quotedValues(instruments)))
	})

	// Learning paths.
	sc.Step(`^"([^"]+)" creates a learning path titled "([^"]+)" for instruments ((?:"[^"]+"(?:, )?)+) with items in order: "([^"]+)"$`, func(_, title, instruments, nodeSlug string) error {
		ids := instrumentIDsNamed(quotedValues(instruments))
		resp, err := w.handler.CreateLearningPath(w.ctx(), generated.CreateLearningPathRequestObject{
			Body: pathRequestWithInstruments(title, nodeSlug, ids),
		})
		w.lastResp, w.lastErr = resp, err
		return err
	})
	sc.Step(`^the learning path is for instruments ((?:"[^"]+"(?:, )?)+)$`, func(instruments string) error {
		return w.lastItemIsFor(instrumentIDsNamed(quotedValues(instruments)))
	})
	sc.Step(`^a learning path "([^"]+)" exists with items "([^"]+)", for instruments ((?:"[^"]+"(?:, )?)+)$`, func(slug, nodeSlug, instruments string) error {
		ids := instrumentIDStrings(quotedValues(instruments))
		return w.putLibraryPath(slug, nodeSlug, func(p *domain.LearningPath) { p.InstrumentIDs = ids })
	})
	sc.Step(`^a learning path "([^"]+)" exists with items "([^"]+)", for every instrument$`, func(slug, nodeSlug string) error {
		return w.putLibraryPath(slug, nodeSlug, func(*domain.LearningPath) {})
	})

	// Content nodes.
	sc.Step(`^"([^"]+)" creates an article content node titled "([^"]+)" for instruments ((?:"[^"]+"(?:, )?)+)$`, func(_, title, instruments string) error {
		return w.createArticleFor(title, instrumentIDsNamed(quotedValues(instruments)))
	})
	sc.Step(`^"([^"]+)" creates an article content node titled "([^"]+)" for every instrument$`, func(_, title string) error {
		return w.createArticleFor(title, []uuid.UUID{})
	})
	sc.Step(`^the content node is for instruments ((?:"[^"]+"(?:, )?)+)$`, func(instruments string) error {
		return w.lastItemIsFor(instrumentIDsNamed(quotedValues(instruments)))
	})
	sc.Step(`^the content node is for every instrument$`, func() error { return w.lastItemIsFor(nil) })
	sc.Step(`^a content node "([^"]+)" exists for instruments ((?:"[^"]+"(?:, )?)+)$`, func(slug, instruments string) error {
		return w.putNodeFor(slug, instrumentIDStrings(quotedValues(instruments)))
	})
	sc.Step(`^a content node "([^"]+)" exists for every instrument$`, func(slug string) error {
		return w.putNodeFor(slug, nil)
	})
	sc.Step(`^"([^"]+)" lists content nodes filtered by instrument "([^"]+)"$`, func(_, instrument string) error {
		id := instrumentID(instrument)
		resp, err := w.handler.ListContentNodes(w.ctx(), generated.ListContentNodesRequestObject{Params: generated.ListContentNodesParams{InstrumentId: &id}})
		w.lastResp, w.lastErr = resp, err
		return err
	})
}

// instrumentIDsNamed maps instrument names to the ids the world seeds them
// under; an unseeded name maps to an id no instrument has.
func instrumentIDsNamed(names []string) []uuid.UUID {
	ids := make([]uuid.UUID, len(names))
	for i, name := range names {
		ids[i] = instrumentID(name)
	}
	return ids
}

func instrumentIDStrings(names []string) []string {
	ids := make([]string, len(names))
	for i, id := range instrumentIDsNamed(names) {
		ids[i] = id.String()
	}
	return ids
}

func (w *world) createCourseFor(title string, instruments []uuid.UUID, pathSlug string) error {
	resp, err := w.handler.CreateCourse(w.ctx(), generated.CreateCourseRequestObject{
		Body: &generated.CreateCourseRequest{
			Title: title, Summary: "A summary", Level: generated.CreateCourseRequestLevelBeginner, Language: "en",
			InstrumentIds: instruments, Checkpoints: toCourseCheckpointBody([]courseCheckpointSpec{{slug: pathSlug}}),
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) seedCourseFor(courseSlug string, instruments []uuid.UUID, creator, pathSlug string) error {
	_, err := w.seedCourseWith(courseSlug, []string{pathSlug}, creator, true, func(body *generated.CreateCourseRequest) {
		body.InstrumentIds = instruments
	})
	return err
}

// replaceCourseInstruments replaces the course's live draft with the same
// content for other instruments.
func (w *world) replaceCourseInstruments(courseSlug string, instruments []uuid.UUID) error {
	current, err := w.courses.GetByID(context.Background(), w.courseIDBySlug[courseSlug].String())
	if err != nil {
		return err
	}
	checkpoints := make([]courseCheckpointBody, len(current.Checkpoints))
	for i, cp := range current.Checkpoints {
		checkpoints[i].LearningPathId = uuid.MustParse(cp.LearningPathID)
		checkpoints[i].Title = cp.Title
	}
	resp, err := w.handler.ReplaceCourse(w.ctx(), generated.ReplaceCourseRequestObject{
		CourseId: w.courseIDBySlug[courseSlug],
		Body: &generated.ReplaceCourseRequest{
			Title: current.Title, Summary: current.Summary, Level: generated.ReplaceCourseRequestLevel(current.Level),
			Language: current.Language, InstrumentIds: instruments, Checkpoints: checkpoints,
		},
	})
	if err != nil {
		return err
	}
	if _, ok := resp.(generated.ReplaceCourse200JSONResponse); !ok {
		return fmt.Errorf("setup: expected the course replace to succeed, got %#v", resp)
	}
	return nil
}

func pathRequestWithInstruments(title, nodeSlug string, instruments []uuid.UUID) *generated.CreateLearningPathRequest {
	body := &generated.CreateLearningPathRequest{Title: title, Level: generated.CreateLearningPathRequestLevelBeginner, InstrumentIds: instruments}
	body.Items = append(body.Items, struct {
		ContentNodeId uuid.UUID `json:"content_node_id"`
		SectionLabel  *string   `json:"section_label,omitempty"`
	}{ContentNodeId: nodeID(nodeSlug)})
	return body
}

func (w *world) createArticleFor(title string, instruments []uuid.UUID) error {
	doc := promptDocFor("An article.")
	resp, err := w.handler.CreateContentNode(w.ctx(), generated.CreateContentNodeRequestObject{
		Body: &generated.CreateContentNodeRequest{
			Title: title, ContentType: generated.CreateContentNodeRequestContentTypeArticle,
			Classification: w.classificationInputFor("s", "c", "beginner"), RichContent: &doc,
			LanguageCodes: []string{"en"}, InstrumentIds: instruments,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) putNodeFor(slug string, instruments []string) error {
	if err := w.putContentNode(slug, domain.ContentTypeArticle); err != nil {
		return err
	}
	node, err := w.nodes.GetByID(context.Background(), nodeID(slug).String())
	if err != nil {
		return err
	}
	node.InstrumentIDs = instruments
	w.nodes.put(node)
	return nil
}

// lastItemIsFor checks the instruments of the course, learning path or
// content node the last step created; want nil means every instrument.
func (w *world) lastItemIsFor(want []uuid.UUID) error {
	var got []uuid.UUID
	switch resp := w.lastResp.(type) {
	case generated.CreateCourse201JSONResponse:
		got = resp.InstrumentIds
	case generated.CreateLearningPath201JSONResponse:
		got = resp.InstrumentIds
	case generated.CreateContentNode201JSONResponse:
		got = resp.InstrumentIds
	default:
		return fmt.Errorf("expected a created course, learning path or content node, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	sort := func(ids []uuid.UUID) []string {
		out := make([]string, len(ids))
		for i, id := range ids {
			out[i] = id.String()
		}
		slices.Sort(out)
		return out
	}
	if fmt.Sprint(sort(got)) != fmt.Sprint(sort(want)) {
		return fmt.Errorf("expected instruments %v, got %v", sort(want), sort(got))
	}
	return nil
}
