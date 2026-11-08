package ports

import (
	"context"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
)

// CommercialStore is the unit of work for commercial changes. Everything a
// commercial command changes — its business records, its operation record and
// its audit evidence — commits together or not at all, so a change can never
// exist without its evidence.
type CommercialStore interface {
	// InTx runs fn in one transaction and commits only if fn returns nil.
	// Any error from fn, or from the commit, leaves nothing changed.
	InTx(ctx context.Context, fn func(ctx context.Context, tx CommercialTx) error) error
	// ReadActivation reads the current activation outside any command. A
	// read failure is returned as an error, never as an empty activation.
	ReadActivation(ctx context.Context) (domain.CommercialActivation, error)
	// Audit reads evidence outside any command.
	Audit() CommercialAuditReader
}

// CommercialTx is what a command may read and change inside its transaction.
type CommercialTx interface {
	Activation() CommercialActivationTx
	Groups() CommercialGroupsTx
	Operations() CommercialOperationsTx
	Audit() CommercialAuditWriter
}

// CommercialActivationTx guards the environment-wide activation revision.
// Intents hold the shared lock while they are authorized; activation and group
// changes hold the exclusive lock. So once an OFF change commits, no new
// intent can be authorized under the revision before it.
type CommercialActivationTx interface {
	// LockShared takes the shared lock and returns the current revision.
	LockShared(ctx context.Context) (int64, error)
	// LockExclusive takes the exclusive lock and returns the current revision.
	LockExclusive(ctx context.Context) (int64, error)
	// Read returns the activation as seen inside this transaction.
	Read(ctx context.Context) (domain.CommercialActivation, error)
	// SaveFlag replaces one flag's switch and audience.
	SaveFlag(ctx context.Context, flag domain.FlagName, setting domain.FlagSetting, at time.Time, by string) error
	// BumpRevision adds one to the revision and returns the new value. The
	// exclusive lock must be held.
	BumpRevision(ctx context.Context) (int64, error)
}

// CommercialGroupsTx reads and changes rollout groups.
type CommercialGroupsTx interface {
	// List returns every group ordered by slug.
	List(ctx context.Context) ([]domain.CommercialGroup, error)
	// Find returns one group, or domain.ErrNotFound.
	Find(ctx context.Context, slug string) (domain.CommercialGroup, error)
	// Save creates a group or renames it; members are unchanged.
	Save(ctx context.Context, slug, name string, at time.Time, by string) error
	// SetMembers replaces a group's members.
	SetMembers(ctx context.Context, slug string, memberIDs []string, at time.Time, by string) error
	// MissingUsers returns the ids, of those given, that are not core users.
	MissingUsers(ctx context.Context, userIDs []string) ([]string, error)
}

// CommercialOperationsTx stores operation records and their attempts.
type CommercialOperationsTx interface {
	// Find returns the operation recorded under key, or domain.ErrNotFound.
	Find(ctx context.Context, key domain.OperationKey) (domain.CommercialOperation, error)
	// Insert records a new operation. A key that is already recorded returns
	// domain.ErrConflict.
	Insert(ctx context.Context, operation domain.CommercialOperation) error
	// RecordAttempt appends one attempt.
	RecordAttempt(ctx context.Context, attempt domain.OperationAttempt) error
}

// CommercialAuditWriter appends evidence. Evidence is never updated or deleted.
type CommercialAuditWriter interface {
	Append(ctx context.Context, entry domain.AuditEntry) error
}

// AuditFilter selects evidence by operation, or by resource. Exactly one of
// OperationID or the ResourceKind and ResourceID pair is set.
type AuditFilter struct {
	OperationID  string
	ResourceKind string
	ResourceID   string
}

// AuditRecord is one stored entry with its position in append order.
type AuditRecord struct {
	OrderKey int64
	Entry    domain.AuditEntry
}

// CommercialAuditReader traverses evidence in append order within a
// snapshot, so entries appended during a traversal never shift it.
type CommercialAuditReader interface {
	// LastOrderKey returns the highest order key in this environment, or 0.
	LastOrderKey(ctx context.Context, environment domain.Environment) (int64, error)
	// List returns up to limit entries matching filter in this environment,
	// with afterKey < order key <= snapshotKey, in order.
	List(ctx context.Context, environment domain.Environment, filter AuditFilter, afterKey, snapshotKey int64, limit int) ([]AuditRecord, error)
}
