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
	sc.Step(`^"([^"]+)" creates a challenge for "([^"]+)" with subject tag "([^"]+)",\s+pass threshold (\d+), and remediation target "([^"]+)"$`, w.createsChallengeWithRemediation)
	sc.Step(`^"([^"]+)" retrieves the challenge "([^"]+)"$`, w.retrievesChallenge)
	sc.Step(`^"([^"]+)" submits a create challenge request with the subject_tag field omitted$`, w.submitsChallengeMissingSubjectTag)
	sc.Step(`^"([^"]+)" submits a create challenge request with the pass_threshold field omitted$`, w.submitsChallengeMissingPassThreshold)
	sc.Step(`^"([^"]+)" submits a create challenge request with pass_threshold (\d+)$`, w.submitsChallengeWithPassThreshold)
	sc.Step(`^"([^"]+)" creates a challenge for a content node ID that does not exist$`, w.createsChallengeForMissingNode)
	sc.Step(`^"([^"]+)" retrieves a challenge with an ID that does not exist$`, w.retrievesMissingChallenge)
	sc.Step(`^"([^"]+)" attempts to create a challenge for "([^"]+)"$`, w.attemptsCreateChallenge)
	sc.Step(`^an unauthenticated request attempts to create a challenge$`, w.unauthCreatesChallenge)

	sc.Step(`^the challenge is created and assigned a stable identifier$`, w.challengeCreated)
	sc.Step(`^the challenge records "([^"]+)" as its parent content node$`, w.challengeRecordsParent)
	sc.Step(`^the challenge is created with the remediation target recorded$`, w.challengeRecordsRemediation)
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

func (w *world) createsChallengeWithRemediation(name, nodeSlug, subjectTag, passThresholdStr, remediationSlug string) error {
	passThreshold, err := parseInt(passThresholdStr)
	if err != nil {
		return err
	}
	target := nodeID(remediationSlug)
	resp, err := w.handler.CreateChallenge(w.ctx(), generated.CreateChallengeRequestObject{
		ContentNodeId: nodeID(nodeSlug),
		Body: &generated.CreateChallengeRequest{
			SubjectTag: subjectTag, PassThreshold: passThreshold,
			RemediationTargetContentNodeId: &target,
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

func (w *world) challengeRecordsRemediation() error {
	resp, ok := w.lastResp.(generated.CreateChallenge201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v", w.lastResp)
	}
	if resp.RemediationTargetContentNodeId == nil {
		return fmt.Errorf("expected remediation_target_content_node_id to be set")
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
