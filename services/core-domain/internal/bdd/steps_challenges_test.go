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

	sc.Step(`^"([^"]+)" creates a challenge for "([^"]+)" with subject skill "([^"]+)" and pass threshold (\d+)$`, w.createsChallengeWithSubjectSkill)
	sc.Step(`^"([^"]+)" creates a challenge for "([^"]+)" with subject skill "([^"]+)", pass threshold (\d+), and time threshold (\d+) ms$`, w.createsChallengeWithTimeThreshold)
	sc.Step(`^"([^"]+)" creates a challenge for "([^"]+)" with subject skill "([^"]+)", pass threshold (\d+), shuffled exercises, and shuffled options$`, w.createsChallengeWithShuffle)
	sc.Step(`^"([^"]+)" retrieves the challenge "([^"]+)"$`, w.retrievesChallenge)
	sc.Step(`^"([^"]+)" lists the challenges for content node "([^"]+)"$`, w.listsContentNodeChallenges)
	sc.Step(`^"([^"]+)" lists the challenges for a content node ID that does not exist$`, w.listsContentNodeChallengesMissing)
	sc.Step(`^an unauthenticated request attempts to list the challenges for content node "([^"]+)"$`, w.unauthListsContentNodeChallenges)
	sc.Step(`^"([^"]+)" submits a create challenge request with neither subject_skill_id nor subject_concept_id set$`, w.submitsChallengeMissingSubject)
	sc.Step(`^"([^"]+)" submits a create challenge request with both subject_skill_id and subject_concept_id set$`, w.submitsChallengeBothSubjects)
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
	sc.Step(`^the response returns the challenge's subject skill, threshold, and parent content node$`, w.challengeResponseComplete)

	sc.Step(`^a challenge "([^"]+)" exists for content node "([^"]+)" with no time threshold set$`, w.putChallengeNoShuffle)
	sc.Step(`^a challenge "([^"]+)" exists for content node "([^"]+)" with time threshold (\d+) ms$`, w.putChallengeWithTimeThreshold)
	sc.Step(`^the response reports time_threshold_ms (\d+)$`, w.responseReportsTimeThreshold)
	sc.Step(`^the response does not report a time_threshold_ms$`, w.responseDoesNotReportTimeThreshold)

	sc.Step(`^content node "([^"]+)" is also classified under skill "([^"]+)"$`, w.nodeAlsoClassifiedUnderSkill)

	sc.Step(`^"([^"]+)" updates challenge "([^"]+)" with subject skill "([^"]+)" and pass threshold (\d+)$`, w.updatesChallengeWithSubjectSkill)
	sc.Step(`^"([^"]+)" updates challenge "([^"]+)" with subject concept "([^"]+)" and pass threshold (\d+)$`, w.updatesChallengeWithSubjectConcept)
	sc.Step(`^"([^"]+)" updates challenge "([^"]+)" with subject skill "([^"]+)" and pass threshold (\d+) and time threshold (\d+) ms$`, w.updatesChallengeWithTimeThreshold)
	sc.Step(`^"([^"]+)" updates challenge "([^"]+)" with subject skill "([^"]+)" and pass threshold (\d+) and the time_threshold_ms field omitted$`, w.updatesChallengeWithSubjectSkill)
	sc.Step(`^"([^"]+)" updates challenge "([^"]+)" with subject skill "([^"]+)", pass threshold (\d+), shuffled exercises, and shuffled options$`, w.updatesChallengeWithShuffle)
	sc.Step(`^"([^"]+)" submits an update challenge request for "([^"]+)" with neither subject_skill_id nor subject_concept_id set$`, w.submitsUpdateChallengeMissingSubject)
	sc.Step(`^"([^"]+)" submits an update challenge request for "([^"]+)" with both subject_skill_id and subject_concept_id set$`, w.submitsUpdateChallengeBothSubjects)
	sc.Step(`^"([^"]+)" submits an update challenge request for "([^"]+)" with pass_threshold (\d+)$`, w.submitsUpdateChallengeWithPassThreshold)
	sc.Step(`^"([^"]+)" attempts to update a challenge with an ID that does not exist$`, w.attemptsUpdateMissingChallenge)
	sc.Step(`^"([^"]+)" attempts to update challenge "([^"]+)" with subject skill "([^"]+)"$`, w.attemptsUpdateChallenge)
	sc.Step(`^an unauthenticated request attempts to update challenge "([^"]+)" with subject skill "([^"]+)"$`, w.unauthUpdatesChallenge)

	sc.Step(`^the challenge's subject skill is "([^"]+)"$`, w.challengeSubjectSkillIs)
	sc.Step(`^the challenge's subject concept is "([^"]+)"$`, w.challengeSubjectConceptIs)
	sc.Step(`^the challenge's pass threshold is (\d+)$`, w.challengePassThresholdIs)
	sc.Step(`^the challenge's time_threshold_ms is (\d+)$`, w.challengeTimeThresholdIs)
}

// nodeAlsoClassifiedUnderSkill appends skillName (auto-created as a root
// skill if not already known) to slug's classification.skill_ids, so a
// later challenge subject-membership check against that skill passes.
func (w *world) nodeAlsoClassifiedUnderSkill(slug, skillName string) error {
	node, err := w.nodes.GetByID(w.ctx(), nodeID(slug).String())
	if err != nil {
		return err
	}
	id := w.skillIDFor(skillName)
	node.Classification.Skills = append(node.Classification.Skills, domain.Skill{ID: id.String(), Name: skillName})
	w.nodes.put(node)
	return nil
}

func (w *world) putChallengeWithTimeThreshold(slug, nodeSlug, msStr string) error {
	ms, err := parseInt(msStr)
	if err != nil {
		return err
	}
	subjectSkillID := w.skillIDFor("subject-" + slug).String()
	w.challenges.put(domain.Challenge{
		ID:              challengeID(slug).String(),
		ContentNodeID:   nodeID(nodeSlug).String(),
		SubjectSkillID:  &subjectSkillID,
		PassThreshold:   70,
		TimeThresholdMS: &ms,
		CreatedAt:       fixedNow,
	})
	return nil
}

func (w *world) responseReportsTimeThreshold(wantStr string) error {
	want, err := parseInt(wantStr)
	if err != nil {
		return err
	}
	resp, ok := w.lastResp.(generated.GetChallenge200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.TimeThresholdMs == nil || *resp.TimeThresholdMs != want {
		return fmt.Errorf("expected time_threshold_ms %d, got %+v", want, resp.TimeThresholdMs)
	}
	return nil
}

func (w *world) responseDoesNotReportTimeThreshold() error {
	resp, ok := w.lastResp.(generated.GetChallenge200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.TimeThresholdMs != nil {
		return fmt.Errorf("expected no time_threshold_ms, got %d", *resp.TimeThresholdMs)
	}
	return nil
}

func (w *world) updatesChallengeWithSubjectSkill(name, challengeSlug, skillName, passThresholdStr string) error {
	passThreshold, err := parseInt(passThresholdStr)
	if err != nil {
		return err
	}
	skillID := w.skillIDFor(skillName)
	resp, err := w.handler.UpdateChallenge(w.ctx(), generated.UpdateChallengeRequestObject{
		ChallengeId: challengeID(challengeSlug),
		Body:        &generated.UpdateChallengeRequest{SubjectSkillId: &skillID, PassThreshold: passThreshold},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) updatesChallengeWithSubjectConcept(name, challengeSlug, conceptName, passThresholdStr string) error {
	passThreshold, err := parseInt(passThresholdStr)
	if err != nil {
		return err
	}
	conceptID := w.conceptIDFor(conceptName)
	resp, err := w.handler.UpdateChallenge(w.ctx(), generated.UpdateChallengeRequestObject{
		ChallengeId: challengeID(challengeSlug),
		Body:        &generated.UpdateChallengeRequest{SubjectConceptId: &conceptID, PassThreshold: passThreshold},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) updatesChallengeWithTimeThreshold(name, challengeSlug, skillName, passThresholdStr, timeThresholdStr string) error {
	passThreshold, err := parseInt(passThresholdStr)
	if err != nil {
		return err
	}
	timeThreshold, err := parseInt(timeThresholdStr)
	if err != nil {
		return err
	}
	skillID := w.skillIDFor(skillName)
	resp, err := w.handler.UpdateChallenge(w.ctx(), generated.UpdateChallengeRequestObject{
		ChallengeId: challengeID(challengeSlug),
		Body: &generated.UpdateChallengeRequest{
			SubjectSkillId: &skillID, PassThreshold: passThreshold,
			TimeThresholdMs: &timeThreshold,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) updatesChallengeWithShuffle(name, challengeSlug, skillName, passThresholdStr string) error {
	passThreshold, err := parseInt(passThresholdStr)
	if err != nil {
		return err
	}
	skillID := w.skillIDFor(skillName)
	shuffle := true
	resp, err := w.handler.UpdateChallenge(w.ctx(), generated.UpdateChallengeRequestObject{
		ChallengeId: challengeID(challengeSlug),
		Body: &generated.UpdateChallengeRequest{
			SubjectSkillId: &skillID, PassThreshold: passThreshold,
			ShuffleExercises: &shuffle, ShuffleOptions: &shuffle,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsUpdateChallengeMissingSubject(name, challengeSlug string) error {
	resp, err := w.handler.UpdateChallenge(w.ctx(), generated.UpdateChallengeRequestObject{
		ChallengeId: challengeID(challengeSlug),
		Body:        &generated.UpdateChallengeRequest{PassThreshold: 70},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsUpdateChallengeBothSubjects(name, challengeSlug string) error {
	skillID := w.skillIDFor("triad-shapes")
	conceptID := w.conceptIDFor("chord-theory")
	resp, err := w.handler.UpdateChallenge(w.ctx(), generated.UpdateChallengeRequestObject{
		ChallengeId: challengeID(challengeSlug),
		Body:        &generated.UpdateChallengeRequest{SubjectSkillId: &skillID, SubjectConceptId: &conceptID, PassThreshold: 70},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsUpdateChallengeWithPassThreshold(name, challengeSlug, passThresholdStr string) error {
	passThreshold, err := parseInt(passThresholdStr)
	if err != nil {
		return err
	}
	skillID := w.skillIDFor("triad-shapes")
	resp, err := w.handler.UpdateChallenge(w.ctx(), generated.UpdateChallengeRequestObject{
		ChallengeId: challengeID(challengeSlug),
		Body:        &generated.UpdateChallengeRequest{SubjectSkillId: &skillID, PassThreshold: passThreshold},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) attemptsUpdateMissingChallenge(string) error {
	skillID := w.skillIDFor("triad-shapes")
	resp, err := w.handler.UpdateChallenge(w.ctx(), generated.UpdateChallengeRequestObject{
		ChallengeId: deterministicUUID("challenge", "does-not-exist"),
		Body:        &generated.UpdateChallengeRequest{SubjectSkillId: &skillID, PassThreshold: 70},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) attemptsUpdateChallenge(name, challengeSlug, skillName string) error {
	skillID := w.skillIDFor(skillName)
	resp, err := w.handler.UpdateChallenge(w.ctx(), generated.UpdateChallengeRequestObject{
		ChallengeId: challengeID(challengeSlug),
		Body:        &generated.UpdateChallengeRequest{SubjectSkillId: &skillID, PassThreshold: 70},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthUpdatesChallenge(challengeSlug, skillName string) error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.attemptsUpdateChallenge("", challengeSlug, skillName)
}

func (w *world) challengeSubjectSkillIs(want string) error {
	resp, ok := w.lastResp.(generated.UpdateChallenge200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	wantID := w.skillIDFor(want)
	if resp.SubjectSkillId == nil || *resp.SubjectSkillId != wantID {
		return fmt.Errorf("expected subject_skill_id %s, got %#v", wantID, resp.SubjectSkillId)
	}
	return nil
}

func (w *world) challengeSubjectConceptIs(want string) error {
	resp, ok := w.lastResp.(generated.UpdateChallenge200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	wantID := w.conceptIDFor(want)
	if resp.SubjectConceptId == nil || *resp.SubjectConceptId != wantID {
		return fmt.Errorf("expected subject_concept_id %s, got %#v", wantID, resp.SubjectConceptId)
	}
	return nil
}

func (w *world) challengePassThresholdIs(wantStr string) error {
	want, err := parseInt(wantStr)
	if err != nil {
		return err
	}
	resp, ok := w.lastResp.(generated.UpdateChallenge200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.PassThreshold != want {
		return fmt.Errorf("expected pass_threshold %d, got %d", want, resp.PassThreshold)
	}
	return nil
}

func (w *world) challengeTimeThresholdIs(wantStr string) error {
	want, err := parseInt(wantStr)
	if err != nil {
		return err
	}
	switch resp := w.lastResp.(type) {
	case generated.UpdateChallenge200JSONResponse:
		if resp.TimeThresholdMs == nil || *resp.TimeThresholdMs != want {
			return fmt.Errorf("expected time_threshold_ms %d, got %+v", want, resp.TimeThresholdMs)
		}
	case generated.CreateChallenge201JSONResponse:
		if resp.TimeThresholdMs == nil || *resp.TimeThresholdMs != want {
			return fmt.Errorf("expected time_threshold_ms %d, got %+v", want, resp.TimeThresholdMs)
		}
	default:
		return fmt.Errorf("expected a 200 or 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}

// putChallenge seeds a challenge whose subject skill is "triad-shapes" —
// the skill every challenges.feature/exercises.feature scenario expects
// "intro-to-triads" (this suite's default content node fixture) to already
// carry, per putContentNode's classification.
func (w *world) putChallenge(slug, nodeSlug string) error {
	subjectSkillID := w.skillIDFor("triad-shapes").String()
	w.challenges.put(domain.Challenge{
		ID:             challengeID(slug).String(),
		ContentNodeID:  nodeID(nodeSlug).String(),
		SubjectSkillID: &subjectSkillID,
		PassThreshold:  70,
		CreatedAt:      fixedNow,
	})
	return nil
}

func (w *world) createsChallengeWithSubjectSkill(name, nodeSlug, skillName, passThresholdStr string) error {
	passThreshold, err := parseInt(passThresholdStr)
	if err != nil {
		return err
	}
	skillID := w.skillIDFor(skillName)
	resp, err := w.handler.CreateChallenge(w.ctx(), generated.CreateChallengeRequestObject{
		ContentNodeId: nodeID(nodeSlug),
		Body:          &generated.CreateChallengeRequest{SubjectSkillId: &skillID, PassThreshold: passThreshold},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) createsChallengeWithTimeThreshold(name, nodeSlug, skillName, passThresholdStr, timeThresholdStr string) error {
	passThreshold, err := parseInt(passThresholdStr)
	if err != nil {
		return err
	}
	timeThreshold, err := parseInt(timeThresholdStr)
	if err != nil {
		return err
	}
	skillID := w.skillIDFor(skillName)
	resp, err := w.handler.CreateChallenge(w.ctx(), generated.CreateChallengeRequestObject{
		ContentNodeId: nodeID(nodeSlug),
		Body: &generated.CreateChallengeRequest{
			SubjectSkillId: &skillID, PassThreshold: passThreshold,
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

func (w *world) submitsChallengeMissingSubject(string) error {
	resp, err := w.handler.CreateChallenge(w.ctx(), generated.CreateChallengeRequestObject{
		ContentNodeId: nodeID("intro-to-triads"),
		Body:          &generated.CreateChallengeRequest{PassThreshold: 70},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsChallengeBothSubjects(string) error {
	skillID := w.skillIDFor("triad-shapes")
	conceptID := w.conceptIDFor("chord-theory")
	resp, err := w.handler.CreateChallenge(w.ctx(), generated.CreateChallengeRequestObject{
		ContentNodeId: nodeID("intro-to-triads"),
		Body:          &generated.CreateChallengeRequest{SubjectSkillId: &skillID, SubjectConceptId: &conceptID, PassThreshold: 70},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsChallengeMissingPassThreshold(string) error {
	skillID := w.skillIDFor("triad-shapes")
	resp, err := w.handler.CreateChallenge(w.ctx(), generated.CreateChallengeRequestObject{
		ContentNodeId: nodeID("intro-to-triads"),
		Body:          &generated.CreateChallengeRequest{SubjectSkillId: &skillID},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsChallengeWithPassThreshold(name, passThresholdStr string) error {
	passThreshold, err := parseInt(passThresholdStr)
	if err != nil {
		return err
	}
	skillID := w.skillIDFor("triad-shapes")
	resp, err := w.handler.CreateChallenge(w.ctx(), generated.CreateChallengeRequestObject{
		ContentNodeId: nodeID("intro-to-triads"),
		Body:          &generated.CreateChallengeRequest{SubjectSkillId: &skillID, PassThreshold: passThreshold},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) createsChallengeForMissingNode(string) error {
	skillID := w.skillIDFor("triad-shapes")
	resp, err := w.handler.CreateChallenge(w.ctx(), generated.CreateChallengeRequestObject{
		ContentNodeId: deterministicUUID("node", "does-not-exist"),
		Body:          &generated.CreateChallengeRequest{SubjectSkillId: &skillID, PassThreshold: 70},
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
	skillID := w.skillIDFor("triad-shapes")
	resp, err := w.handler.CreateChallenge(w.ctx(), generated.CreateChallengeRequestObject{
		ContentNodeId: nodeID(nodeSlug),
		Body:          &generated.CreateChallengeRequest{SubjectSkillId: &skillID, PassThreshold: 70},
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
	if resp.SubjectSkillId == nil || resp.PassThreshold == 0 || resp.ContentNodeId.String() == "" {
		return fmt.Errorf("expected a fully populated challenge, got %+v", resp)
	}
	return nil
}

func (w *world) createsChallengeWithShuffle(name, nodeSlug, skillName, passThresholdStr string) error {
	passThreshold, err := parseInt(passThresholdStr)
	if err != nil {
		return err
	}
	skillID := w.skillIDFor(skillName)
	shuffle := true
	resp, err := w.handler.CreateChallenge(w.ctx(), generated.CreateChallengeRequestObject{
		ContentNodeId: nodeID(nodeSlug),
		Body: &generated.CreateChallengeRequest{
			SubjectSkillId: &skillID, PassThreshold: passThreshold,
			ShuffleExercises: &shuffle, ShuffleOptions: &shuffle,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) challengeShuffleFlags() (shuffleExercises, shuffleOptions bool, err error) {
	switch resp := w.lastResp.(type) {
	case generated.CreateChallenge201JSONResponse:
		return resp.ShuffleExercises, resp.ShuffleOptions, nil
	case generated.UpdateChallenge200JSONResponse:
		return resp.ShuffleExercises, resp.ShuffleOptions, nil
	default:
		return false, false, fmt.Errorf("expected a 201 or 200 response, got %#v", w.lastResp)
	}
}

func (w *world) challengeShuffleEnabled() error {
	shuffleExercises, shuffleOptions, err := w.challengeShuffleFlags()
	if err != nil {
		return err
	}
	if !shuffleExercises || !shuffleOptions {
		return fmt.Errorf("expected both shuffle flags enabled, got exercises=%v options=%v", shuffleExercises, shuffleOptions)
	}
	return nil
}

func (w *world) challengeShuffleDisabled() error {
	shuffleExercises, shuffleOptions, err := w.challengeShuffleFlags()
	if err != nil {
		return err
	}
	if shuffleExercises || shuffleOptions {
		return fmt.Errorf("expected both shuffle flags disabled, got exercises=%v options=%v", shuffleExercises, shuffleOptions)
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
