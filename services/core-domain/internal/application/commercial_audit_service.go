package application

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

const (
	defaultAuditPageSize = 20
	maxAuditPageSize     = 100
	maxAuditResourceKind = 100
	maxAuditResourceID   = 255

	auditReadAction       = "commercial_audit.read"
	auditReadResourceKind = "commercial_audit"
	auditCursorVersion    = 1
)

// AuditQuery is one page request for commercial evidence. Continuations
// repeat the original filters and limit with the cursor they were given.
type AuditQuery struct {
	OperationID  string
	ResourceKind string
	ResourceID   string
	Cursor       string
	// Limit is the page size; 0 means the default.
	Limit int
}

// AuditPage is one page of a fixed snapshot. NextCursor is nil once the
// traversal is complete.
type AuditPage struct {
	Items      []domain.AuditEntry
	NextCursor *string
}

// CommercialAuditService lets administrators trace commercial operations.
// Every read is itself recorded as evidence.
type CommercialAuditService struct {
	store       ports.CommercialStore
	environment domain.Environment
	secret      []byte
	now         func() time.Time
	newID       func() string
}

func NewCommercialAuditService(store ports.CommercialStore, environment domain.Environment, cursorSecret []byte, now func() time.Time, newID func() string) *CommercialAuditService {
	return &CommercialAuditService{store: store, environment: environment, secret: cursorSecret, now: now, newID: newID}
}

// List returns one page of the evidence matching the query, in append order
// within a snapshot fixed by the first page. Entries appended later never
// enter that traversal. Storage failure is domain.ErrUnavailable, never an
// empty page; a cursor that does not match its query is a validation error.
func (s *CommercialAuditService) List(ctx context.Context, caller domain.User, query AuditQuery) (AuditPage, error) {
	if caller.Role != domain.RoleAdmin {
		return AuditPage{}, domain.ErrForbidden
	}
	filter, limit, err := validateAuditQuery(query)
	if err != nil {
		return AuditPage{}, err
	}

	position, err := s.position(ctx, caller, query, filter, limit)
	if err != nil {
		return AuditPage{}, err
	}
	records, err := s.store.Audit().List(ctx, s.environment, filter, position.After, position.Snapshot, limit+1)
	if err != nil {
		return AuditPage{}, fmt.Errorf("%w: read commercial evidence: %w", domain.ErrUnavailable, err)
	}

	page := AuditPage{Items: make([]domain.AuditEntry, 0, limit)}
	for i, record := range records {
		if i == limit {
			next := position
			next.After = records[limit-1].OrderKey
			cursor := s.encodeCursor(next)
			page.NextCursor = &cursor
			break
		}
		page.Items = append(page.Items, record.Entry)
	}

	if err := s.recordRead(ctx, caller, filter, position.Snapshot); err != nil {
		return AuditPage{}, err
	}
	return page, nil
}

func validateAuditQuery(query AuditQuery) (ports.AuditFilter, int, error) {
	filter := ports.AuditFilter{OperationID: query.OperationID, ResourceKind: query.ResourceKind, ResourceID: query.ResourceID}
	hasResource := query.ResourceKind != "" || query.ResourceID != ""
	switch {
	case query.OperationID == "" && !hasResource:
		return filter, 0, domain.NewValidationError("operation_id", "operation_id or resource_kind and resource_id is required")
	case query.OperationID != "" && uuid.Validate(query.OperationID) != nil:
		return filter, 0, domain.NewValidationError("operation_id", "must be a UUID")
	case hasResource && (query.ResourceKind == "" || query.ResourceID == ""):
		return filter, 0, domain.NewValidationError("resource_kind", "resource_kind and resource_id go together")
	case len(query.ResourceKind) > maxAuditResourceKind:
		return filter, 0, domain.NewValidationError("resource_kind", "is too long")
	case len(query.ResourceID) > maxAuditResourceID:
		return filter, 0, domain.NewValidationError("resource_id", "is too long")
	}
	limit := query.Limit
	if limit == 0 {
		limit = defaultAuditPageSize
	}
	if limit < 1 || limit > maxAuditPageSize {
		return filter, 0, domain.NewValidationError("limit", fmt.Sprintf("must be between 1 and %d", maxAuditPageSize))
	}
	return filter, limit, nil
}

// auditCursor is a traversal position, bound to everything that defines the
// traversal so a continuation cannot change its scope.
type auditCursor struct {
	Version     int    `json:"v"`
	Snapshot    int64  `json:"s"`
	After       int64  `json:"a"`
	Environment string `json:"e"`
	Admin       string `json:"u"`
	Scope       string `json:"f"`
	Limit       int    `json:"l"`
}

func (s *CommercialAuditService) position(ctx context.Context, caller domain.User, query AuditQuery, filter ports.AuditFilter, limit int) (auditCursor, error) {
	expected := auditCursor{
		Version: auditCursorVersion, Environment: s.environment.String(), Admin: caller.ID,
		Scope: auditScopeDigest(filter), Limit: limit,
	}
	if query.Cursor == "" {
		snapshot, err := s.store.Audit().LastOrderKey(ctx, s.environment)
		if err != nil {
			return auditCursor{}, fmt.Errorf("%w: read commercial evidence: %w", domain.ErrUnavailable, err)
		}
		expected.Snapshot = snapshot
		return expected, nil
	}

	cursor, err := s.decodeCursor(query.Cursor)
	if err != nil {
		return auditCursor{}, err
	}
	bound := cursor
	bound.Snapshot, bound.After = 0, 0
	if bound != expected || cursor.After > cursor.Snapshot {
		return auditCursor{}, invalidAuditCursor()
	}
	return cursor, nil
}

func (s *CommercialAuditService) encodeCursor(cursor auditCursor) string {
	payload, err := json.Marshal(cursor)
	if err != nil {
		// A struct of strings and integers always marshals.
		panic(fmt.Sprintf("marshal audit cursor: %v", err))
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	return encoded + "." + s.sign(encoded)
}

func (s *CommercialAuditService) decodeCursor(raw string) (auditCursor, error) {
	encoded, signature, found := strings.Cut(raw, ".")
	if !found || !hmac.Equal([]byte(signature), []byte(s.sign(encoded))) {
		return auditCursor{}, invalidAuditCursor()
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return auditCursor{}, invalidAuditCursor()
	}
	var cursor auditCursor
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return auditCursor{}, invalidAuditCursor()
	}
	return cursor, nil
}

func (s *CommercialAuditService) sign(encoded string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(encoded))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func invalidAuditCursor() error {
	return domain.NewValidationError("cursor", "is invalid or does not belong to this query; start a new traversal")
}

// auditScopeDigest identifies a filter compactly: it names the access
// evidence's resource and binds cursors to their filter.
func auditScopeDigest(filter ports.AuditFilter) string {
	sum := sha256.Sum256([]byte(filter.OperationID + "\x00" + filter.ResourceKind + "\x00" + filter.ResourceID))
	return hex.EncodeToString(sum[:])
}

// recordRead appends the access evidence for a read. It is appended after
// the snapshot was fixed, so it is never part of the snapshot it describes.
func (s *CommercialAuditService) recordRead(ctx context.Context, caller domain.User, filter ports.AuditFilter, snapshot int64) error {
	scope, err := json.Marshal(map[string]string{
		"operation_id": filter.OperationID, "resource_kind": filter.ResourceKind, "resource_id": filter.ResourceID,
		"snapshot": fmt.Sprint(snapshot),
	})
	if err != nil {
		return fmt.Errorf("%w: %w", domain.ErrUnavailable, err)
	}
	now := s.now()
	entry := domain.AuditEntry{
		AuditEntryID: s.newID(),
		Operation: domain.OperationKey{
			Environment: s.environment, ActorKind: domain.AuditActorUser, ActorID: caller.ID,
			Action: auditReadAction, ResourceKind: auditReadResourceKind, ResourceID: auditScopeDigest(filter),
			CallerKey: s.newID(),
		},
		AttemptID: s.newID(), OccurredAt: now, RecordedAt: now,
		Reason: "administrator inspected commercial evidence", Outcome: domain.AuditOutcomeApplied,
		AfterState: scope, PolicyRevision: domain.CommercialRegistryRevision,
		ConfigurationRevision: unresolvedConfiguration, CorrelationID: s.newID(),
		CausedByEntryIDs: []string{}, ProviderReferences: []string{},
	}
	err = s.store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
		return tx.Audit().Append(ctx, entry)
	})
	if err != nil {
		return fmt.Errorf("%w: the read could not be evidenced: %w", domain.ErrUnavailable, err)
	}
	return nil
}
