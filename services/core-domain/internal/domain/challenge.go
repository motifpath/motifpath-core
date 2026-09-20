package domain

import "time"

// Challenge is the assessment unit for a content node: it groups exercises
// and carries the subject (a reference to exactly one Skill or Concept tree
// node — SubjectSkillID/SubjectConceptID are mutually exclusive, exactly one
// always set) and pass threshold used by the rules-based recommendation
// engine, plus an optional, purely informational time threshold.
// Remediation targets are not modeled here — see Exercise.RemediationTargets,
// which attaches remediation to the specific exercise a student struggled
// with rather than the whole challenge.
type Challenge struct {
	ID            string
	ContentNodeID string
	// SubjectSkillID/SubjectConceptID reference a node in the shared
	// Skill/Concept tree — exactly one is set. Whether the referenced id
	// actually belongs to the parent ContentNode's own classification is an
	// application-layer concern, requiring a repository round trip this
	// constructor can't perform.
	SubjectSkillID   *string
	SubjectConceptID *string
	PassThreshold    int
	// TimeThresholdMS is the teacher's explicit time-expectation override,
	// in milliseconds, or nil if none was set. It is never enforced — never
	// gates submission or affects scoring. A nil value does not mean "no
	// threshold" to a caller: the application layer resolves the value
	// actually shown to a caller (this override if set, otherwise computed
	// from linked exercises' estimated durations) — this field alone is not
	// the full picture, see ChallengeService's resolution logic.
	TimeThresholdMS *int
	// ShuffleExercises and ShuffleOptions control whether this challenge's
	// exercises, and each exercise's options, are returned in a fresh random
	// order on every ListChallengeExercises call, or always in link order.
	ShuffleExercises bool
	ShuffleOptions   bool
	CreatedAt        time.Time
}

// NewChallenge validates and constructs a Challenge. Whether ContentNodeID
// refers to a content node that actually exists, and whether the subject id
// belongs to that node's own classification, are application-layer
// concerns — both require a repository round trip this constructor can't
// perform.
func NewChallenge(id, contentNodeID string, subjectSkillID, subjectConceptID *string, passThreshold int, timeThresholdMS *int, shuffleExercises, shuffleOptions bool, createdAt time.Time) (Challenge, error) {
	errs := validateChallengeContent(subjectSkillID, subjectConceptID, passThreshold, timeThresholdMS)
	if len(errs) > 0 {
		return Challenge{}, &ValidationError{Fields: errs}
	}

	return Challenge{
		ID:               id,
		ContentNodeID:    contentNodeID,
		SubjectSkillID:   subjectSkillID,
		SubjectConceptID: subjectConceptID,
		PassThreshold:    passThreshold,
		TimeThresholdMS:  timeThresholdMS,
		ShuffleExercises: shuffleExercises,
		ShuffleOptions:   shuffleOptions,
		CreatedAt:        createdAt,
	}, nil
}

// Update validates and returns a copy of c with its subject, pass
// threshold, time threshold override, and shuffle flags replaced. ID,
// ContentNodeID, and CreatedAt carry over unchanged — a challenge's parent
// content node cannot change after creation, and its linked exercises are
// managed exclusively through ExerciseService's Link/Unlink operations, not
// through this update.
func (c Challenge) Update(subjectSkillID, subjectConceptID *string, passThreshold int, timeThresholdMS *int, shuffleExercises, shuffleOptions bool) (Challenge, error) {
	errs := validateChallengeContent(subjectSkillID, subjectConceptID, passThreshold, timeThresholdMS)
	if len(errs) > 0 {
		return Challenge{}, &ValidationError{Fields: errs}
	}

	updated := c
	updated.SubjectSkillID = subjectSkillID
	updated.SubjectConceptID = subjectConceptID
	updated.PassThreshold = passThreshold
	updated.TimeThresholdMS = timeThresholdMS
	updated.ShuffleExercises = shuffleExercises
	updated.ShuffleOptions = shuffleOptions
	return updated, nil
}

// validateChallengeContent checks that exactly one of subjectSkillID/
// subjectConceptID is set (non-nil and non-empty), plus passThreshold and
// timeThresholdMS. When neither is set, the failure is reported against
// subject_skill_id; when both are set, against subject_concept_id — the
// shape the merged Gherkin scenarios pin for each case.
func validateChallengeContent(subjectSkillID, subjectConceptID *string, passThreshold int, timeThresholdMS *int) []FieldError {
	var errs []FieldError

	hasSkill := subjectSkillID != nil && *subjectSkillID != ""
	hasConcept := subjectConceptID != nil && *subjectConceptID != ""
	switch {
	case !hasSkill && !hasConcept:
		errs = append(errs, FieldError{Field: "subject_skill_id", Reason: "exactly one of subject_skill_id or subject_concept_id must be set"})
	case hasSkill && hasConcept:
		errs = append(errs, FieldError{Field: "subject_concept_id", Reason: "exactly one of subject_skill_id or subject_concept_id must be set"})
	}

	if passThreshold < 1 || passThreshold > 100 {
		errs = append(errs, FieldError{Field: "pass_threshold", Reason: "must be between 1 and 100"})
	}
	if timeThresholdMS != nil && *timeThresholdMS < 1 {
		errs = append(errs, FieldError{Field: "time_threshold_ms", Reason: "must be at least 1 when present"})
	}

	return errs
}
