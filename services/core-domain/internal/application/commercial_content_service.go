package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// SaleState is whether new acquisitions of an item are free, paid or
// withdrawn.
type SaleState string

// SaleStateFree is the sale state of every item until selling exists.
const SaleStateFree SaleState = "free"

// Sale blocker codes core can evaluate.
const (
	BlockerUnresolvedOwnership  = "unresolved_ownership"
	BlockerConfigurationMissing = "configuration_missing"
	BlockerNewSalesPaused       = "new_sales_paused"
)

// SaleBlocker is one reason core would refuse a paid sale state or a new sale
// now. Content names the dependency concerned, or is nil when the blocker is
// about the item or its seller.
type SaleBlocker struct {
	Code    string
	Content *domain.ContentRef
}

// CommercialContent is an item's authoritative commercial state: its
// commercial revision and latest release, if any. Until selling exists every
// item is free and lists the blockers core can already evaluate.
type CommercialContent struct {
	Ownership    domain.ContentOwnership
	Revision     int
	SaleState    SaleState
	Release      *domain.CommercialRelease
	SaleBlockers []SaleBlocker
}

// OwnershipCorrectionRequest is an administrator's reviewed change of an
// item's rights holder, against the ownership revision they observed.
type OwnershipCorrectionRequest struct {
	ExpectedRevision int    `json:"expected_revision"`
	OwnerKind        string `json:"owner_kind"`
	OwnerID          string `json:"owner_id"`
	Reason           string `json:"reason"`
}

// CommandReceipt is the durable receipt of a commercial command.
type CommandReceipt struct {
	OperationID     string `json:"operation_id"`
	ResourceID      string `json:"resource_id"`
	Status          string `json:"status"`
	EvidenceEntryID string `json:"evidence_entry_id"`
}

const ownershipCorrectionIntentVersion = 1

// CommercialContentService reads an item's commercial state and corrects its
// ownership. Only an item's owner and administrators can tell it exists.
type CommercialContentService struct {
	store       ports.CommercialStore
	commands    *CommercialCommands
	rollout     *CommercialRolloutService
	environment domain.Environment
}

func NewCommercialContentService(store ports.CommercialStore, commands *CommercialCommands, rollout *CommercialRolloutService, environment domain.Environment) *CommercialContentService {
	return &CommercialContentService{store: store, commands: commands, rollout: rollout, environment: environment}
}

// Get returns an item's commercial state for its owner or an administrator.
// Anyone else, and an item with no ownership record, is domain.ErrNotFound.
func (s *CommercialContentService) Get(ctx context.Context, caller domain.User, kind, id string) (CommercialContent, error) {
	content, err := parseContentRef(kind, id)
	if err != nil {
		return CommercialContent{}, err
	}
	ownership, err := s.store.CurrentOwnership(ctx, content)
	if err != nil {
		return CommercialContent{}, err
	}
	if !canReadCommercialContent(caller, ownership) {
		return CommercialContent{}, domain.ErrNotFound
	}
	blockers, err := s.saleBlockers(ctx, caller, ownership)
	if err != nil {
		return CommercialContent{}, err
	}
	state, err := s.store.CommercialState(ctx, content)
	if err != nil {
		return CommercialContent{}, err
	}
	release, err := latestRelease(ctx, s.store, state)
	if err != nil {
		return CommercialContent{}, err
	}
	return CommercialContent{Ownership: ownership, Revision: state.Revision, SaleState: SaleStateFree, Release: release, SaleBlockers: blockers}, nil
}

// latestRelease is the item's latest release, or nil when it has none. A
// recorded release that cannot be read makes the item's state unavailable.
func latestRelease(ctx context.Context, releases releaseReader, state domain.CommercialState) (*domain.CommercialRelease, error) {
	if state.LatestReleaseID == nil {
		return nil, nil
	}
	release, err := releases.Release(ctx, *state.LatestReleaseID)
	if err != nil {
		return nil, fmt.Errorf("%w: latest release %s: %w", domain.ErrUnavailable, *state.LatestReleaseID, err)
	}
	return &release, nil
}

// releaseReader reads a stored release, in or outside a transaction.
type releaseReader interface {
	Release(ctx context.Context, releaseID string) (domain.CommercialRelease, error)
}

func canReadCommercialContent(caller domain.User, ownership domain.ContentOwnership) bool {
	if caller.Role == domain.RoleAdmin {
		return true
	}
	return ownership.OwnerKind == domain.OwnerTeacher && ownership.OwnerID != nil && *ownership.OwnerID == caller.ID
}

// saleBlockers lists what would block a sale today, for the caller: no
// reviewed rights holder, no selling configuration (none exists yet), and new
// sales paused by the rollout flag that governs the owner's kind of sale.
func (s *CommercialContentService) saleBlockers(ctx context.Context, caller domain.User, ownership domain.ContentOwnership) ([]SaleBlocker, error) {
	blockers := []SaleBlocker{}
	if ownership.OwnerKind == domain.OwnerUnresolved {
		return append(blockers, SaleBlocker{Code: BlockerUnresolvedOwnership}, SaleBlocker{Code: BlockerConfigurationMissing}), nil
	}
	blockers = append(blockers, SaleBlocker{Code: BlockerConfigurationMissing})
	allowed, err := s.rollout.Allowed(ctx, salesFlag(ownership), domain.RolloutSubject{Kind: domain.RolloutSubjectUser, ID: caller.ID})
	if err != nil {
		return nil, err
	}
	if !allowed {
		blockers = append(blockers, SaleBlocker{Code: BlockerNewSalesPaused})
	}
	return blockers, nil
}

// salesFlag is the rollout flag that governs new sales of an item: a
// teacher's own paid content, or the platform's.
func salesFlag(ownership domain.ContentOwnership) domain.FlagName {
	if ownership.OwnerKind == domain.OwnerTeacher {
		return domain.FlagName("teacher_paid_paths")
	}
	return domain.FlagName("motifpath_purchase")
}

// CorrectOwnership appends an administrator's correction of an item's rights
// holder. The creator, source and earlier revisions are kept. It is refused
// for anyone else, while the content-ownership rollout is OFF, and against a
// stale ownership revision; a same-intent replay returns the original
// receipt.
func (s *CommercialContentService) CorrectOwnership(ctx context.Context, caller domain.User, kind, id, idempotencyKey string, request OwnershipCorrectionRequest) (CommandReceipt, error) {
	content, err := parseContentRef(kind, id)
	if err != nil {
		return CommandReceipt{}, err
	}
	correction := domain.OwnershipCorrection{
		ExpectedRevision: request.ExpectedRevision, OwnerKind: domain.OwnerKind(request.OwnerKind), OwnerID: request.OwnerID,
	}
	if err := correction.Validate(); err != nil {
		return CommandReceipt{}, err
	}
	if strings.TrimSpace(request.Reason) == "" {
		return CommandReceipt{}, domain.NewValidationError("reason", "a correction states its rights evidence")
	}
	body, err := json.Marshal(request)
	if err != nil {
		return CommandReceipt{}, domain.NewValidationError("body", err.Error())
	}
	intent := CommercialIntent{
		Key: domain.OperationKey{
			Environment: s.environment, ActorKind: domain.AuditActorUser, ActorID: caller.ID,
			Action: ActionCorrectContentOwnership, ResourceKind: string(content.Kind), ResourceID: content.ID,
			CallerKey: idempotencyKey,
		},
		Version: ownershipCorrectionIntentVersion, Body: body, Reason: request.Reason, PolicyRevision: ownershipPolicyRevision,
	}
	authorize := func(context.Context) error {
		if caller.Role != domain.RoleAdmin {
			return fmt.Errorf("%w: only an administrator corrects ownership", domain.ErrForbidden)
		}
		return nil
	}
	receipt, err := s.commands.Run(ctx, intent, authorize, s.correctionEffect(caller, content, correction))
	if err != nil {
		return CommandReceipt{}, err
	}
	var result CommandReceipt
	if err := json.Unmarshal(receipt.Result, &result); err != nil {
		return CommandReceipt{}, fmt.Errorf("%w: recorded receipt is unreadable: %w", domain.ErrUnavailable, err)
	}
	return result, nil
}

func (s *CommercialContentService) correctionEffect(caller domain.User, content domain.ContentRef, correction domain.OwnershipCorrection) CommercialEffect {
	return func(ctx context.Context, tx ports.CommercialTx, run CommercialRun) (CommercialEffectResult, error) {
		revision, err := s.rollout.AuthorizeIntent(ctx, tx, domain.FlagName("content_ownership"), domain.RolloutSubject{Kind: domain.RolloutSubjectUser, ID: caller.ID})
		if err != nil {
			return CommercialEffectResult{}, err
		}
		current, err := tx.Ownership().LockCurrent(ctx, content)
		if err != nil {
			return CommercialEffectResult{}, err
		}
		next, err := current.Corrected(correction, run.EvidenceEntryID)
		if err != nil {
			return CommercialEffectResult{}, err
		}
		if err := checkRightsHolder(ctx, tx, correction); err != nil {
			return CommercialEffectResult{}, err
		}
		if err := tx.Ownership().Append(ctx, next); err != nil {
			return CommercialEffectResult{}, err
		}
		result, err := json.Marshal(CommandReceipt{
			OperationID: run.OperationID, ResourceID: content.ID, Status: string(domain.AuditOutcomeApplied), EvidenceEntryID: run.EvidenceEntryID,
		})
		if err != nil {
			return CommercialEffectResult{}, err
		}
		return CommercialEffectResult{
			Result: result, AuthorizedRevision: revision, ConfigurationRevision: ownershipConfigurationRevision,
			Before: ownershipState(current), After: ownershipState(next),
		}, nil
	}
}

// checkRightsHolder refuses a correction to a teacher-kind owner unless that
// owner is a core user whose role is teacher or admin now: a student, or an id
// core does not know, cannot hold rights.
func checkRightsHolder(ctx context.Context, tx ports.CommercialTx, correction domain.OwnershipCorrection) error {
	if correction.OwnerKind != domain.OwnerTeacher {
		return nil
	}
	role, err := tx.Ownership().UserRole(ctx, correction.OwnerID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.NewValidationError("owner_id", "the rights holder is not a core user")
	}
	if err != nil {
		return err
	}
	if role != domain.RoleTeacher && role != domain.RoleAdmin {
		return domain.NewValidationError("owner_id", "a rights holder is a teacher or an administrator")
	}
	return nil
}

func parseContentRef(kind, id string) (domain.ContentRef, error) {
	contentKind, err := domain.ParseContentKind(kind)
	if err != nil {
		return domain.ContentRef{}, err
	}
	if id == "" {
		return domain.ContentRef{}, domain.NewValidationError("content_id", "is required")
	}
	return domain.ContentRef{Kind: contentKind, ID: id}, nil
}
