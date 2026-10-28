package repo

import (
	"context"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/emailsend"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/predicate"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntEmailSendLog keeps the record of sent engagement emails in Postgres.
// The unique index on recipient, kind and period makes a claim atomic: of
// any number of concurrent claims for the same email, one insert wins.
type EntEmailSendLog struct {
	client *ent.Client
}

func NewEntEmailSendLog(client *ent.Client) *EntEmailSendLog {
	return &EntEmailSendLog{client: client}
}

func (r *EntEmailSendLog) Claim(ctx context.Context, claim domain.EmailSendClaim) (bool, error) {
	_, err := r.client.EmailSend.Create().
		SetRecipient(claim.Recipient).
		SetKind(emailsend.Kind(claim.Kind)).
		SetPeriodKey(claim.PeriodKey).
		SetSentAt(claim.At).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *EntEmailSendLog) Release(ctx context.Context, claim domain.EmailSendClaim) error {
	_, err := r.client.EmailSend.Delete().Where(matching(claim)).Exec(ctx)
	return err
}

func (r *EntEmailSendLog) Claimed(ctx context.Context, claim domain.EmailSendClaim) (bool, error) {
	return r.client.EmailSend.Query().Where(matching(claim)).Exist(ctx)
}

// matching selects the row of claim's recipient, kind and period, whenever
// it was claimed.
func matching(claim domain.EmailSendClaim) predicate.EmailSend {
	return emailsend.And(
		emailsend.Recipient(claim.Recipient),
		emailsend.KindEQ(emailsend.Kind(claim.Kind)),
		emailsend.PeriodKey(claim.PeriodKey),
	)
}
