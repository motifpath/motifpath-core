package ports

import (
	"context"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
)

// EmailSendLog records every engagement email the platform has sent, so
// each one goes out at most once however often the mailer runs.
type EmailSendLog interface {
	// Claim records claim, and reports false without recording anything
	// when the same recipient, kind and period were already claimed. Only a
	// successful claim may be followed by a send.
	Claim(ctx context.Context, claim domain.EmailSendClaim) (bool, error)

	// Release removes a claim whose email could not be sent, so a later run
	// can try again.
	Release(ctx context.Context, claim domain.EmailSendClaim) error

	// Claimed reports whether the recipient, kind and period of claim were
	// already claimed.
	Claimed(ctx context.Context, claim domain.EmailSendClaim) (bool, error)
}

// EmailComposer writes each kind of engagement email in its recipient's
// language.
type EmailComposer interface {
	ProgressReport(message domain.ProgressReportMessage) (domain.Email, error)
	Reminder(message domain.ReminderMessage) (domain.Email, error)
	ConciergeSummary(message domain.ConciergeSummaryMessage) (domain.Email, error)
}

// EmailSender delivers an email. An error means it was not delivered; one
// that wraps domain.ErrEmailRejected means the server refused it for good,
// so sending it again would fail the same way.
type EmailSender interface {
	Send(ctx context.Context, email domain.Email) error
}

// LastActivityReader tells when students were last active: the time of
// their latest event of any kind.
type LastActivityReader interface {
	// LastActive returns when each of studentIDs was last active; a student
	// with no activity at all is absent.
	LastActive(ctx context.Context, studentIDs []string) (map[string]time.Time, error)
}
