package keeper

import (
	"context"

	govsynthtypes "github.com/zerone-chain/zerone/x/governance_synthesis/types"
	"github.com/zerone-chain/zerone/x/inquiry/types"
)

// GovernanceSynthesisAdapter exposes the per-domain open-inquiry
// count used by the Frontier synthesizer.
type GovernanceSynthesisAdapter struct {
	k Keeper
}

func NewGovernanceSynthesisAdapter(k Keeper) *GovernanceSynthesisAdapter {
	return &GovernanceSynthesisAdapter{k: k}
}

// CountOpenInquiriesByDomain returns the number of inquiries in
// (OPEN, ANSWERED) status for the given domain. Walks the by-domain
// index. Bounded by domain size.
func (a *GovernanceSynthesisAdapter) CountOpenInquiriesByDomain(ctx context.Context, domain string) uint64 {
	count := uint64(0)
	_ = a.k.IterateInquiriesByDomain(ctx, domain, func(q *types.Inquiry) bool {
		if q.Status == types.InquiryStatus_INQUIRY_STATUS_OPEN ||
			q.Status == types.InquiryStatus_INQUIRY_STATUS_ANSWERED {
			count++
		}
		return false
	})
	return count
}

var _ govsynthtypes.FrontierInquiryKeeper = (*GovernanceSynthesisAdapter)(nil)
