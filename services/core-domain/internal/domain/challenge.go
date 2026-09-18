package domain

import "time"

// Challenge is the assessment unit for a content node: it groups exercises
// and carries the subject tag and pass threshold used by the rules-based
// recommendation engine, plus an optional, purely informational time
// threshold. Remediation targets are not modeled here — see
// Exercise.RemediationTargets, which attaches remediation to the specific
// exercise a student struggled with rather than the whole challenge.
type Challenge struct {
	ID            string
	ContentNodeID string
	SubjectTag    string
	PassThreshold int
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
// refers to a content node that actually exists is an application-layer
// concern — it requires a repository round-trip this constructor can't
// perform.
func NewChallenge(id, contentNodeID, subjectTag string, passThreshold int, timeThresholdMS *int, shuffleExercises, shuffleOptions bool, createdAt time.Time) (Challenge, error) {
	errs := validateChallengeContent(subjectTag, passThreshold, timeThresholdMS)
	if len(errs) > 0 {
		return Challenge{}, &ValidationError{Fields: errs}
	}

	return Challenge{
		ID:               id,
		ContentNodeID:    contentNodeID,
		SubjectTag:       subjectTag,
		PassThreshold:    passThreshold,
		TimeThresholdMS:  timeThresholdMS,
		ShuffleExercises: shuffleExercises,
		ShuffleOptions:   shuffleOptions,
		CreatedAt:        createdAt,
	}, nil
}

// Update validates and returns a copy of c with its subject tag, pass
// threshold, time threshold override, and shuffle flags replaced. ID,
// ContentNodeID, and CreatedAt carry over unchanged — a challenge's parent
// content node cannot change after creation, and its linked exercises are
// managed exclusively through ExerciseService's Link/Unlink operations, not
// through this update.
func (c Challenge) Update(subjectTag string, passThreshold int, timeThresholdMS *int, shuffleExercises, shuffleOptions bool) (Challenge, error) {
	errs := validateChallengeContent(subjectTag, passThreshold, timeThresholdMS)
	if len(errs) > 0 {
		return Challenge{}, &ValidationError{Fields: errs}
	}

	updated := c
	updated.SubjectTag = subjectTag
	updated.PassThreshold = passThreshold
	updated.TimeThresholdMS = timeThresholdMS
	updated.ShuffleExercises = shuffleExercises
	updated.ShuffleOptions = shuffleOptions
	return updated, nil
}

func validateChallengeContent(subjectTag string, passThreshold int, timeThresholdMS *int) []FieldError {
	var errs []FieldError

	if subjectTag == "" {
		errs = append(errs, FieldError{Field: "subject_tag", Reason: "must not be empty"})
	}
	if passThreshold < 1 || passThreshold > 100 {
		errs = append(errs, FieldError{Field: "pass_threshold", Reason: "must be between 1 and 100"})
	}
	if timeThresholdMS != nil && *timeThresholdMS < 1 {
		errs = append(errs, FieldError{Field: "time_threshold_ms", Reason: "must be at least 1 when present"})
	}

	return errs
}
