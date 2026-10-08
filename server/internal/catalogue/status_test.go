package catalogue

import (
	"testing"

	"github.com/KarthikReddy8809/catalift/server/internal/store"
)

func TestProductStatus(t *testing.T) {
	done := store.DetectionStatusDone
	cases := []struct {
		name string
		row  store.ListProductsRow
		want string
	}{
		{"no photo", store.ListProductsRow{ImageCount: 0}, StatusNeedsPhoto},
		{"photo, never run", store.ListProductsRow{ImageCount: 1}, StatusUploaded},
		{"detection running", store.ListProductsRow{ImageCount: 1, EnrichmentStarted: true, DetectionStatus: store.DetectionStatusPending}, StatusEnriching},
		{"a listing still queued", store.ListProductsRow{ImageCount: 1, EnrichmentStarted: true, DetectionStatus: done, ListingsQueued: 1, ListingsTotal: 2}, StatusEnriching},
		{"budget stopped detection", store.ListProductsRow{ImageCount: 1, EnrichmentStarted: true, DetectionStatus: store.DetectionStatusStoppedBudget}, StatusBudgetExhausted},
		{"budget stopped a listing", store.ListProductsRow{ImageCount: 1, EnrichmentStarted: true, DetectionStatus: done, ListingsStopped: 1, ListingsFailed: 1}, StatusBudgetExhausted},
		{"detection failed", store.ListProductsRow{ImageCount: 1, EnrichmentStarted: true, DetectionStatus: store.DetectionStatusFailed}, StatusFailed},
		{"written, not approved", store.ListProductsRow{ImageCount: 1, EnrichmentStarted: true, DetectionStatus: done, ListingsTotal: 2, ListingsApproved: 1}, StatusReadyForReview},
		{"every channel approved", store.ListProductsRow{ImageCount: 1, EnrichmentStarted: true, DetectionStatus: done, ListingsTotal: 2, ListingsApproved: 2}, StatusApproved},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ProductStatus(&c.row)

			if got != c.want {
				t.Fatalf("got %s, want %s", got, c.want)
			}
		})
	}
}
