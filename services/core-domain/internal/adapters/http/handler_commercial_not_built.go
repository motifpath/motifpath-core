package http

import (
	"context"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
)

// The commercial operations below are listed in the API before the runtime
// ticket that owns each one builds it. Until then each answers 501 without
// touching an application service. A runtime ticket replaces a method here
// with its real implementation.

func (h *Handler) ListCommercialAuditEntries(_ context.Context, _ generated.ListCommercialAuditEntriesRequestObject) (generated.ListCommercialAuditEntriesResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) CreateCommercialConfiguration(_ context.Context, _ generated.CreateCommercialConfigurationRequestObject) (generated.CreateCommercialConfigurationResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) GetCurrentCommercialConfiguration(_ context.Context, _ generated.GetCurrentCommercialConfigurationRequestObject) (generated.GetCurrentCommercialConfigurationResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) ListCommercialGroups(_ context.Context, _ generated.ListCommercialGroupsRequestObject) (generated.ListCommercialGroupsResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) SaveCommercialGroup(_ context.Context, _ generated.SaveCommercialGroupRequestObject) (generated.SaveCommercialGroupResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) ChangeCommercialGroupMembers(_ context.Context, _ generated.ChangeCommercialGroupMembersRequestObject) (generated.ChangeCommercialGroupMembersResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) GetCommercialRollout(_ context.Context, _ generated.GetCommercialRolloutRequestObject) (generated.GetCommercialRolloutResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) TurnOffAllCommercialFlags(_ context.Context, _ generated.TurnOffAllCommercialFlagsRequestObject) (generated.TurnOffAllCommercialFlagsResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) ChangeCommercialFlag(_ context.Context, _ generated.ChangeCommercialFlagRequestObject) (generated.ChangeCommercialFlagResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) ExecuteFullPaymentRefund(_ context.Context, _ generated.ExecuteFullPaymentRefundRequestObject) (generated.ExecuteFullPaymentRefundResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) RetryFailedFullRefund(_ context.Context, _ generated.RetryFailedFullRefundRequestObject) (generated.RetryFailedFullRefundResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) ReviewFullRefundEligibility(_ context.Context, _ generated.ReviewFullRefundEligibilityRequestObject) (generated.ReviewFullRefundEligibilityResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) GetCurrentSubscriptionOffer(_ context.Context, _ generated.GetCurrentSubscriptionOfferRequestObject) (generated.GetCurrentSubscriptionOfferResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) CreateSubscriptionOfferRevision(_ context.Context, _ generated.CreateSubscriptionOfferRevisionRequestObject) (generated.CreateSubscriptionOfferRevisionResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) GetCommercialContent(_ context.Context, _ generated.GetCommercialContentRequestObject) (generated.GetCommercialContentResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) CreateCommercialOffer(_ context.Context, _ generated.CreateCommercialOfferRequestObject) (generated.CreateCommercialOfferResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) CorrectCommercialOwnership(_ context.Context, _ generated.CorrectCommercialOwnershipRequestObject) (generated.CorrectCommercialOwnershipResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) PublishCommercialRelease(_ context.Context, _ generated.PublishCommercialReleaseRequestObject) (generated.PublishCommercialReleaseResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) ChangeCommercialSaleState(_ context.Context, _ generated.ChangeCommercialSaleStateRequestObject) (generated.ChangeCommercialSaleStateResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) ListCommercialOffers(_ context.Context, _ generated.ListCommercialOffersRequestObject) (generated.ListCommercialOffersResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) GetCommercialRelease(_ context.Context, _ generated.GetCommercialReleaseRequestObject) (generated.GetCommercialReleaseResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) ReceiveCMSCommercialHint(_ context.Context, _ generated.ReceiveCMSCommercialHintRequestObject) (generated.ReceiveCMSCommercialHintResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) ReceiveStripePaymentEvent(_ context.Context, _ generated.ReceiveStripePaymentEventRequestObject) (generated.ReceiveStripePaymentEventResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) GetRefundRequest(_ context.Context, _ generated.GetRefundRequestRequestObject) (generated.GetRefundRequestResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) GetMyLessonCommercialAccess(_ context.Context, _ generated.GetMyLessonCommercialAccessRequestObject) (generated.GetMyLessonCommercialAccessResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) ListSubscriptionOffers(_ context.Context, _ generated.ListSubscriptionOffersRequestObject) (generated.ListSubscriptionOffersResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) ListMyCommercialBalances(_ context.Context, _ generated.ListMyCommercialBalancesRequestObject) (generated.ListMyCommercialBalancesResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) ListMyTeacherSales(_ context.Context, _ generated.ListMyTeacherSalesRequestObject) (generated.ListMyTeacherSalesResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) ListMyCommercialSettlements(_ context.Context, _ generated.ListMyCommercialSettlementsRequestObject) (generated.ListMyCommercialSettlementsResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) GetMyTeacherSellingTerms(_ context.Context, _ generated.GetMyTeacherSellingTermsRequestObject) (generated.GetMyTeacherSellingTermsResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) GetMyPayoutAccount(_ context.Context, _ generated.GetMyPayoutAccountRequestObject) (generated.GetMyPayoutAccountResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) CreateMyPayoutDashboardLink(_ context.Context, _ generated.CreateMyPayoutDashboardLinkRequestObject) (generated.CreateMyPayoutDashboardLinkResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) CreateMyPayoutOnboardingLink(_ context.Context, _ generated.CreateMyPayoutOnboardingLinkRequestObject) (generated.CreateMyPayoutOnboardingLinkResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) CreateMyCommercialCheckout(_ context.Context, _ generated.CreateMyCommercialCheckoutRequestObject) (generated.CreateMyCommercialCheckoutResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) GetMyCommercialCheckout(_ context.Context, _ generated.GetMyCommercialCheckoutRequestObject) (generated.GetMyCommercialCheckoutResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) ListMyCommercialHoldings(_ context.Context, _ generated.ListMyCommercialHoldingsRequestObject) (generated.ListMyCommercialHoldingsResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) ListMyCommercialPayments(_ context.Context, _ generated.ListMyCommercialPaymentsRequestObject) (generated.ListMyCommercialPaymentsResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) RequestMyPaymentRefund(_ context.Context, _ generated.RequestMyPaymentRefundRequestObject) (generated.RequestMyPaymentRefundResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) CreateMySubscriptionCheckout(_ context.Context, _ generated.CreateMySubscriptionCheckoutRequestObject) (generated.CreateMySubscriptionCheckoutResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) GetMySubscriptionCheckout(_ context.Context, _ generated.GetMySubscriptionCheckoutRequestObject) (generated.GetMySubscriptionCheckoutResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) ListMySubscriptions(_ context.Context, _ generated.ListMySubscriptionsRequestObject) (generated.ListMySubscriptionsResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) CancelMySubscriptionRenewal(_ context.Context, _ generated.CancelMySubscriptionRenewalRequestObject) (generated.CancelMySubscriptionRenewalResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) CreateMyPaymentMethodSession(_ context.Context, _ generated.CreateMyPaymentMethodSessionRequestObject) (generated.CreateMyPaymentMethodSessionResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) ResumeMySubscriptionRenewal(_ context.Context, _ generated.ResumeMySubscriptionRenewalRequestObject) (generated.ResumeMySubscriptionRenewalResponseObject, error) {
	return nil, errOperationNotBuilt
}

func (h *Handler) ScheduleMyAnnualSubscription(_ context.Context, _ generated.ScheduleMyAnnualSubscriptionRequestObject) (generated.ScheduleMyAnnualSubscriptionResponseObject, error) {
	return nil, errOperationNotBuilt
}
