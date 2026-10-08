package catalogue

import "github.com/KarthikReddy8809/catalift/server/internal/store"

// Product states the seller watches (seller flow step 5), derived on read
// from the attributes and listing counts, never stored.
const (
	StatusNeedsPhoto      = "needs_photo"
	StatusUploaded        = "uploaded"
	StatusEnriching       = "enriching"
	StatusBudgetExhausted = "budget_exhausted"
	StatusFailed          = "failed"
	StatusReadyForReview  = "ready_for_review"
	StatusApproved        = "approved"
)

// ProductStatus says where a product is in the flow. A budget stop wins over
// a failure, so the seller sees the cause they can act on (raise the cap).
func ProductStatus(p *store.ListProductsRow) string {
	switch {
	case p.ImageCount == 0:
		return StatusNeedsPhoto
	case !p.EnrichmentStarted:
		return StatusUploaded
	case p.DetectionStatus == store.DetectionStatusStoppedBudget || p.ListingsStopped > 0:
		return StatusBudgetExhausted
	case p.DetectionStatus == store.DetectionStatusFailed || p.ListingsFailed > 0:
		return StatusFailed
	case p.DetectionStatus == store.DetectionStatusPending || p.ListingsQueued > 0:
		return StatusEnriching
	case p.ListingsTotal > 0 && p.ListingsApproved == p.ListingsTotal:
		return StatusApproved
	}
	return StatusReadyForReview
}
