//go:build integration

package bdd

import (
	"fmt"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerChallengeSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^a challenge "([^"]+)" exists for content node "([^"]+)"$`, w.putChallenge)

	sc.Step(`^"([^"]+)" creates a challenge for "([^"]+)" with subject tag "([^"]+)"\s+and pass threshold (\d+)$`, w.createsChallenge)
	sc.Step(`^"([^"]+)" creates a challenge for "([^"]+)" with subject tag "([^"]+)",\s+pass threshold (\d+), and time threshold (\d+) ms$`, w.createsChallengeWithTimeThreshold)
	sc.Step(`^"([^"]+)" creates a challenge for "([^"]+)" with subject tag "([^"]+)",\s+pass threshold (\d+), shuffled exercises, and shuffled options$`, w.createsChallengeWithShuffle)
	sc.Step(`^"([^"]+)" retrieves the challenge "([^"]+)"$`, w.retrievesChallenge)
	sc.Step(`^"([^"]+)" lists the challenges for content node "([^"]+)"$`, w.listsContentNodeChallenges)
	sc.Step(`^"([^"]+)" lists the challenges for a content node ID that does not exist$`, w.listsContentNodeChallengesMissing)
	sc.Step(`^an unauthenticated request attempts to list the challenges for content node "([^"]+)"$`, w.unauthListsContentNodeChallenges)
	sc.Step(`^"([^"]+)" submits a create challenge request with the subject_tag field omitted$`, w.submitsChallengeMissingSubjectTag)
	sc.Step(`^"([^"]+)" submits a create challenge request with the pass_threshold field omitted$`, w.submitsChallengeMissingPassThreshold)
	sc.Step(`^"([^"]+)" submits a create challenge request with pass_threshold (\d+)$`, w.submitsChallengeWithPassThreshold)
	sc.Step(`^"([^"]+)" creates a challenge for a content node ID that does not exist$`, w.createsChallengeForMissingNode)
	sc.Step(`^"([^"]+)" retrieves a challenge with an ID that does not exist$`, w.retrievesMissingChallenge)
	sc.Step(`^"([^"]+)" attempts to create a challenge for "([^"]+)"$`, w.attemptsCreateChallenge)
	sc.Step(`^an unauthenticated request attempts to create a challenge$`, w.unauthCreatesChallenge)

	sc.Step(`^the challenge is created and assigned a stable identifier$`, w.challengeCreated)
	sc.Step(`^the challenge records "([^"]+)" as its parent content node$`, w.challengeRecordsParent)
	sc.Step(`^the challenge is created with time_threshold_ms (\d+)$`, w.challengeRecordsTimeThreshold)
	sc.Step(`^the challenge is created with exercise shuffling and option shuffling both enabled$`, w.challengeShuffleEnabled)
	sc.Step(`^the challenge is created with exercise shuffling and option shuffling both disabled$`, w.challengeShuffleDisabled)
	sc.Step(`^the response returns the challenge's subject tag, threshold, and parent content node$`, w.challengeResponseComplete)
}

func (w *world) putChallenge(slug, nodeSlug string) error {
	w.challenges.put(domain.Challenge{
		ID:            challengeID(slug).String(),
		ContentNodeID: nodeID(nodeSlug).String(),
		SubjectTag:    "subject-" + slug,
		PassThreshold: 70,
		CreatedAt:     fixedNow,
	})
	return nil
}

func (w *world) createsChallenge(name, nodeSlug, subjectTag, passThresholdStr string) error {
	passThreshold, err := parseInt(passThresholdStr)
	if err != nil {
		return err
	}
	resp, err := w.handler.CreateChallenge(w.ctx(), generated.CreateChallengeRequestObject{
		ContentNodeId: nodeID(nodeSlug),
		Body:          &generated.CreateChallengeRequest{SubjectTag: subjectTag, PassThreshold: passThreshold},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) createsChallengeWithTimeThreshold(name, nodeSlug, subjectTag, passThresholdStr, timeThresholdStr string) error {
	passThreshold, err := parseInt(passThresholdStr)
	if err != nil {
		return err
	}
	timeThreshold, err := parseInt(timeThresholdStr)
	if err != nil {
		return err
	}
	resp, err := w.handler.CreateChallenge(w.ctx(), generated.CreateChallengeRequestObject{
		ContentNodeId: nodeID(nodeSlug),
		Body: &generated.CreateChallengeRequest{
			SubjectTag: subjectTag, PassThreshold: passThreshold,
			TimeThresholdMs: &timeThreshold,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) retrievesChallenge(name, slug string) error {
	resp, err := w.handler.GetChallenge(w.ctx(), generated.GetChallengeRequestObject{ChallengeId: challengeID(slug)})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsChallengeMissingSubjectTag(string) error {
	resp, err := w.handler.CreateChallenge(w.ctx(), generated.CreateChallengeRequestObject{
		ContentNodeId: nodeID("intro-to-triads"),
		Body:          &generated.CreateChallengeRequest{PassThreshold: 70},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsChallengeMissingPassThreshold(string) error {
	resp, err := w.handler.CreateChallenge(w.ctx(), generated.CreateChallengeRequestObject{
		ContentNodeId: nodeID("intro-to-triads"),
		Body:          &generated.CreateChallengeRequest{SubjectTag: "triad-shapes"},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsChallengeWithPassThreshold(name, passThresholdStr string) error {
	passThreshold, err := parseInt(passThresholdStr)
	if err != nil {
		return err
	}
	resp, err := w.handler.CreateChallenge(w.ctx(), generated.CreateChallengeRequestObject{
		ContentNodeId: nodeID("intro-to-triads"),
		Body:          &generated.CreateChallengeRequest{SubjectTag: "triad-shapes", PassThreshold: passThreshold},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) createsChallengeForMissingNode(string) error {
	resp, err := w.handler.CreateChallenge(w.ctx(), generated.CreateChallengeRequestObject{
		ContentNodeId: deterministicUUID("node", "does-not-exist"),
		Body:          &generated.CreateChallengeRequest{SubjectTag: "triad-shapes", PassThreshold: 70},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) retrievesMissingChallenge(string) error {
	resp, err := w.handler.GetChallenge(w.ctx(), generated.GetChallengeRequestObject{ChallengeId: deterministicUUID("challenge", "does-not-exist")})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) attemptsCreateChallenge(name, nodeSlug string) error {
	resp, err := w.handler.CreateChallenge(w.ctx(), generated.CreateChallengeRequestObject{
		ContentNodeId: nodeID(nodeSlug),
		Body:          &generated.CreateChallengeRequest{SubjectTag: "triad-shapes", PassThreshold: 70},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthCreatesChallenge() error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.attemptsCreateChallenge("", "intro-to-triads")
}

func (w *world) challengeCreated() error {
	if _, ok := w.lastResp.(generated.CreateChallenge201JSONResponse); !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}

func (w *world) challengeRecordsParent(nodeSlug string) error {
	resp, ok := w.lastResp.(generated.CreateChallenge201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v", w.lastResp)
	}
	if resp.ContentNodeId != nodeID(nodeSlug) {
		return fmt.Errorf("expected content_node_id %s, got %s", nodeID(nodeSlug), resp.ContentNodeId)
	}
	return nil
}

func (w *world) challengeRecordsTimeThreshold(wantStr string) error {
	want, err := parseInt(wantStr)
	if err != nil {
		return err
	}
	resp, ok := w.lastResp.(generated.CreateChallenge201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v", w.lastResp)
	}
	if resp.TimeThresholdMs == nil || *resp.TimeThresholdMs != want {
		return fmt.Errorf("expected time_threshold_ms %d, got %+v", want, resp.TimeThresholdMs)
	}
	return nil
}

func (w *world) challengeResponseComplete() error {
	resp, ok := w.lastResp.(generated.GetChallenge200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.SubjectTag == "" || resp.PassThreshold == 0 || resp.ContentNodeId.String() == "" {
		return fmt.Errorf("expected a fully populated challenge, got %+v", resp)
	}
	return nil
}

func (w *world) createsChallengeWithShuffle(name, nodeSlug, subjectTag, passThresholdStr string) error {
	passThreshold, err := parseInt(passThresholdStr)
	if err != nil {
		return err
	}
	shuffle := true
	resp, err := w.handler.CreateChallenge(w.ctx(), generated.CreateChallengeRequestObject{
		ContentNodeId: nodeID(nodeSlug),
		Body: &generated.CreateChallengeRequest{
			SubjectTag: subjectTag, PassThreshold: passThreshold,
			ShuffleExercises: &shuffle, ShuffleOptions: &shuffle,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) challengeShuffleEnabled() error {
	resp, ok := w.lastResp.(generated.CreateChallenge201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v", w.lastResp)
	}
	if !resp.ShuffleExercises || !resp.ShuffleOptions {
		return fmt.Errorf("expected both shuffle flags enabled, got %+v", resp)
	}
	return nil
}

func (w *world) challengeShuffleDisabled() error {
	resp, ok := w.lastResp.(generated.CreateChallenge201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v", w.lastResp)
	}
	if resp.ShuffleExercises || resp.ShuffleOptions {
		return fmt.Errorf("expected both shuffle flags disabled, got %+v", resp)
	}
	return nil
}

func (w *world) listsContentNodeChallenges(name, nodeSlug string) error {
	resp, err := w.handler.ListContentNodeChallenges(w.ctx(), generated.ListContentNodeChallengesRequestObject{ContentNodeId: nodeID(nodeSlug)})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) listsContentNodeChallengesMissing(string) error {
	resp, err := w.handler.ListContentNodeChallenges(w.ctx(), generated.ListContentNodeChallengesRequestObject{ContentNodeId: deterministicUUID("node", "does-not-exist")})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthListsContentNodeChallenges(nodeSlug string) error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.listsContentNodeChallenges("", nodeSlug)
}
