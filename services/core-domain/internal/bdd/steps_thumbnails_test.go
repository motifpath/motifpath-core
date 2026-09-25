//go:build integration

package bdd

import (
	"context"
	"fmt"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	appHTTP "github.com/motifpath/core-domain/internal/adapters/http"
	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerThumbnailSteps(sc *godog.ScenarioContext, w *world) {
	// Courses.
	sc.Step(`^a course "([^"]+)" exists as a draft with thumbnail "([^"]+)" and checkpoints "([^"]+)"$`, func(courseSlug, thumbnail, pathSlug string) error {
		_, err := w.seedCourseWith(courseSlug, []string{pathSlug}, "bob", false, func(body *generated.CreateCourseRequest) {
			body.ThumbnailUrl = &thumbnail
		})
		return err
	})
	sc.Step(`^a course "([^"]+)" exists, published with thumbnail "([^"]+)", with checkpoints "([^"]+)"$`, func(courseSlug, thumbnail, pathSlug string) error {
		_, err := w.seedCourseWith(courseSlug, []string{pathSlug}, "bob", true, func(body *generated.CreateCourseRequest) {
			body.ThumbnailUrl = &thumbnail
		})
		return err
	})
	sc.Step(`^the course "([^"]+)" is republished with thumbnail "([^"]+)"$`, w.republishCourseWithThumbnail)
	sc.Step(`^"([^"]+)" creates a course titled "([^"]+)" with thumbnail "([^"]+)" and checkpoints in order: "([^"]+)"$`, func(_, title, thumbnail, pathSlug string) error {
		resp, err := w.handler.CreateCourse(w.ctx(), generated.CreateCourseRequestObject{
			Body: &generated.CreateCourseRequest{
				Title: title, Summary: "A summary", Level: generated.CreateCourseRequestLevelBeginner, Language: "en",
				ThumbnailUrl: &thumbnail, Checkpoints: toCourseCheckpointBody([]courseCheckpointSpec{{slug: pathSlug}}),
			},
		})
		w.lastResp, w.lastErr = resp, err
		return err
	})
	sc.Step(`^"([^"]+)" replaces course "([^"]+)" without a thumbnail$`, func(_, courseSlug string) error {
		resp, err := w.replaceCourseThumbnail(w.ctx(), courseSlug, nil)
		w.lastResp, w.lastErr = resp, err
		return err
	})
	sc.Step(`^course version (\d+) records the thumbnail "([^"]+)"$`, w.courseVersionRecordsThumbnail)
	sc.Step(`^the course has no thumbnail$`, func() error {
		resp, ok := w.lastResp.(generated.ReplaceCourse200JSONResponse)
		if !ok {
			return fmt.Errorf("expected a replaced course, got %#v (err=%v)", w.lastResp, w.lastErr)
		}
		return noThumbnail(resp.ThumbnailUrl)
	})
	sc.Step(`^the enrollment in "([^"]+)" shows the thumbnail "([^"]+)"$`, w.enrollmentShowsThumbnail)

	// Learning paths.
	sc.Step(`^"([^"]+)" creates a learning path titled "([^"]+)" with thumbnail "([^"]+)" and items in order: "([^"]+)"$`, func(_, title, thumbnail, nodeSlug string) error {
		body := pathRequestWithInstruments(title, nodeSlug, nil)
		body.ThumbnailUrl = &thumbnail
		resp, err := w.handler.CreateLearningPath(w.ctx(), generated.CreateLearningPathRequestObject{Body: body})
		w.lastResp, w.lastErr = resp, err
		return err
	})
	sc.Step(`^the learning path's thumbnail is "([^"]+)"$`, func(want string) error {
		resp, ok := w.lastResp.(generated.CreateLearningPath201JSONResponse)
		if !ok {
			return fmt.Errorf("expected a created learning path, got %#v (err=%v)", w.lastResp, w.lastErr)
		}
		if resp.ThumbnailUrl == nil || *resp.ThumbnailUrl != want {
			return fmt.Errorf("expected the thumbnail %q, got %v", want, resp.ThumbnailUrl)
		}
		return nil
	})
	sc.Step(`^a learning path "([^"]+)" exists with items "([^"]+)" and thumbnail "([^"]+)"$`, func(slug, nodeSlug, thumbnail string) error {
		return w.putLibraryPath(slug, nodeSlug, func(p *domain.LearningPath) { p.ThumbnailURL = &thumbnail })
	})
	sc.Step(`^"([^"]+)" replaces learning path "([^"]+)" without a thumbnail$`, func(_, slug string) error {
		return w.replacesLearningPathAt(generated.ReplaceLearningPathRequestLevelBeginner, slug, []pathItemSpec{{slug: "node-01"}})
	})
	sc.Step(`^the learning path has no thumbnail$`, func() error {
		resp, ok := w.lastResp.(generated.ReplaceLearningPath200JSONResponse)
		if !ok {
			return fmt.Errorf("expected a replaced learning path, got %#v (err=%v)", w.lastResp, w.lastErr)
		}
		return noThumbnail(resp.ThumbnailUrl)
	})

	// Content nodes.
	sc.Step(`^a content node "([^"]+)" exists for instruments ((?:"[^"]+"(?:, )?)+) with thumbnail "([^"]+)"$`, func(slug, instruments, thumbnail string) error {
		return w.putNodeWith(slug, func(n *domain.ContentNode) {
			n.InstrumentIDs = instrumentIDStrings(quotedValues(instruments))
			n.ThumbnailURL = &thumbnail
		})
	})
	sc.Step(`^a content node "([^"]+)" exists with thumbnail "([^"]+)"$`, func(slug, thumbnail string) error {
		return w.putNodeWith(slug, func(n *domain.ContentNode) { n.ThumbnailURL = &thumbnail })
	})
	sc.Step(`^"([^"]+)" updates content node "([^"]+)" without a thumbnail$`, w.updatesNodeWithoutThumbnail)
	sc.Step(`^the new content node version records instruments ((?:"[^"]+"(?:, )?)+) and thumbnail "([^"]+)"$`, w.nodeVersionRecords)
	sc.Step(`^the content node has no thumbnail$`, func() error {
		resp, ok := w.lastResp.(generated.UpdateContentNode200JSONResponse)
		if !ok {
			return fmt.Errorf("expected an updated content node, got %#v (err=%v)", w.lastResp, w.lastErr)
		}
		return noThumbnail(resp.ThumbnailUrl)
	})
}

func noThumbnail(thumbnail *string) error {
	if thumbnail != nil {
		return fmt.Errorf("expected no thumbnail, got %q", *thumbnail)
	}
	return nil
}

// replaceCourseThumbnail replaces the course's live draft with the same
// content and the given thumbnail (nil for none), as the caller in ctx.
func (w *world) replaceCourseThumbnail(ctx context.Context, courseSlug string, thumbnail *string) (generated.ReplaceCourseResponseObject, error) {
	current, err := w.courses.GetByID(context.Background(), w.courseIDBySlug[courseSlug].String())
	if err != nil {
		return nil, err
	}
	checkpoints := make([]courseCheckpointBody, len(current.Checkpoints))
	for i, cp := range current.Checkpoints {
		checkpoints[i].LearningPathId = uuid.MustParse(cp.LearningPathID)
		checkpoints[i].Title = cp.Title
	}
	return w.handler.ReplaceCourse(ctx, generated.ReplaceCourseRequestObject{
		CourseId: w.courseIDBySlug[courseSlug],
		Body: &generated.ReplaceCourseRequest{
			Title: current.Title, Summary: current.Summary, Level: generated.ReplaceCourseRequestLevel(current.Level),
			Language: current.Language, ThumbnailUrl: thumbnail, Checkpoints: checkpoints,
		},
	})
}

// republishCourseWithThumbnail has the course's creator give it a new
// thumbnail and an admin publish that as its next version.
func (w *world) republishCourseWithThumbnail(courseSlug, thumbnail string) error {
	creatorCtx := appHTTP.WithClerkUserID(context.Background(), clerkSub("bob"))
	if resp, err := w.replaceCourseThumbnail(creatorCtx, courseSlug, &thumbnail); err != nil {
		return err
	} else if _, ok := resp.(generated.ReplaceCourse200JSONResponse); !ok {
		return fmt.Errorf("setup: expected the course replace to succeed, got %#v", resp)
	}
	adminName := "seed-admin-for-" + courseSlug
	w.ensureRegistered(adminName, domain.RoleAdmin)
	adminCtx := appHTTP.WithClerkUserID(context.Background(), clerkSub(adminName))
	resp, err := w.handler.PublishCourse(adminCtx, generated.PublishCourseRequestObject{CourseId: w.courseIDBySlug[courseSlug]})
	if err != nil {
		return err
	}
	if _, ok := resp.(generated.PublishCourse201JSONResponse); !ok {
		return fmt.Errorf("setup: expected the republish to succeed, got %#v", resp)
	}
	return nil
}

func (w *world) courseVersionRecordsThumbnail(versionStr, want string) error {
	resp, ok := w.lastResp.(generated.PublishCourse201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	number, err := parseInt(versionStr)
	if err != nil {
		return err
	}
	if resp.VersionNumber != number || resp.ThumbnailUrlSnapshot == nil || *resp.ThumbnailUrlSnapshot != want {
		return fmt.Errorf("expected course version %d to record the thumbnail %q, got version %d with %v", number, want, resp.VersionNumber, resp.ThumbnailUrlSnapshot)
	}
	return nil
}

func (w *world) enrollmentShowsThumbnail(courseSlug, want string) error {
	resp, ok := w.lastResp.(generated.ListMyCourseEnrollments200JSONResponse)
	if !ok {
		return fmt.Errorf("expected the enrollment list, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	for _, e := range resp {
		if e.CourseId != w.courseIDBySlug[courseSlug] {
			continue
		}
		if e.CourseThumbnailUrl == nil || *e.CourseThumbnailUrl != want {
			return fmt.Errorf("expected the enrollment in %q to show %q, got %v", courseSlug, want, e.CourseThumbnailUrl)
		}
		return nil
	}
	return fmt.Errorf("no enrollment in %q", courseSlug)
}

// putNodeWith seeds an article content node named after slug and lets adjust
// change it.
func (w *world) putNodeWith(slug string, adjust func(*domain.ContentNode)) error {
	if err := w.putContentNode(slug, domain.ContentTypeArticle); err != nil {
		return err
	}
	node, err := w.nodes.GetByID(context.Background(), nodeID(slug).String())
	if err != nil {
		return err
	}
	adjust(&node)
	w.nodes.put(node)
	return nil
}

// updatesNodeWithoutThumbnail resubmits the node as it is, minus its
// thumbnail.
func (w *world) updatesNodeWithoutThumbnail(_, slug string) error {
	existing, err := w.nodes.GetByID(w.ctx(), nodeID(slug).String())
	if err != nil {
		return err
	}
	mediaURL, richContent := bodyFor(existing)
	resp, err := w.handler.UpdateContentNode(w.ctx(), generated.UpdateContentNodeRequestObject{
		ContentNodeId: nodeID(slug),
		Body: &generated.UpdateContentNodeRequest{
			Title: existing.Title, MediaUrl: mediaURL, RichContent: richContent,
			Classification: generated.ClassificationInput{
				SkillIds:        idsOfSkills(existing.Classification.Skills),
				ConceptIds:      idsOfConcepts(existing.Classification.Concepts),
				DifficultyLevel: generated.ClassificationInputDifficultyLevel(existing.Classification.DifficultyLevel),
			},
			LanguageCodes: []string{"en"},
			InstrumentIds: toUUIDList(existing.InstrumentIDs),
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func toUUIDList(ids []string) []uuid.UUID {
	out := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		out[i] = uuid.MustParse(id)
	}
	return out
}

func (w *world) nodeVersionRecords(instruments, thumbnail string) error {
	resp, ok := w.lastResp.(generated.PublishContentNode201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a published content node version, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	want := instrumentIDsNamed(quotedValues(instruments))
	if fmt.Sprint(resp.InstrumentIdsSnapshot) != fmt.Sprint(want) {
		return fmt.Errorf("expected the version to record instruments %v, got %v", want, resp.InstrumentIdsSnapshot)
	}
	if resp.ThumbnailUrlSnapshot == nil || *resp.ThumbnailUrlSnapshot != thumbnail {
		return fmt.Errorf("expected the version to record the thumbnail %q, got %v", thumbnail, resp.ThumbnailUrlSnapshot)
	}
	return nil
}
