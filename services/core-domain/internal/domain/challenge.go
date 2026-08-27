package domain

import "time"

// Challenge is the assessment unit for a content node: it groups exercises
// and carries the subject tag, pass threshold, and optional remediation
// target used by the rules-based recommendation engine.
type Challenge struct {
	ID                             string
	ContentNodeID                  string
	SubjectTag                     string
	PassThreshold                  int
	RemediationTargetContentNodeID *string
	CreatedAt                      time.Time
}

// NewChallenge validates and constructs a Challenge. Whether ContentNodeID
// and RemediationTargetContentNodeID refer to content nodes that actually
// exist is an application-layer concern — it requires a repository
// round-trip this constructor can't perform.
func NewChallenge(id, contentNodeID, subjectTag string, passThreshold int, remediationTarget *string, createdAt time.Time) (Challenge, error) {
	var errs []FieldError

	if subjectTag == "" {
		errs = append(errs, FieldError{Field: "subject_tag", Reason: "must not be empty"})
	}
	if passThreshold < 1 || passThreshold > 100 {
		errs = append(errs, FieldError{Field: "pass_threshold", Reason: "must be between 1 and 100"})
	}

	if len(errs) > 0 {
		return Challenge{}, &ValidationError{Fields: errs}
	}

	return Challenge{
		ID:                             id,
		ContentNodeID:                  contentNodeID,
		SubjectTag:                     subjectTag,
		PassThreshold:                  passThreshold,
		RemediationTargetContentNodeID: remediationTarget,
		CreatedAt:                      createdAt,
	}, nil
}
