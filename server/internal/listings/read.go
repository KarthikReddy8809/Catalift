package listings

import (
	"context"
	"fmt"

	"github.com/KarthikReddy8809/catalift/server/internal/store"
)

// GridRow is one listing as the review grid shows it.
type GridRow struct {
	store.ListGridRow
	Failures     []store.ListRuleResultsRow
	Regeneration *store.LatestRegenerationsRow
}

// Grid lists listings by SKU then channel, with their rule failures and
// latest regeneration request (AC-US-00-007-1).
func (s *Service) Grid(ctx context.Context, p store.ListGridParams) ([]GridRow, error) {
	q := store.New(s.pool)
	rows, err := q.ListGrid(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("list grid: %w", err)
	}
	ids := make([]int64, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}
	fails, err := q.ListRuleResults(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("rule results: %w", err)
	}
	regens, err := q.LatestRegenerations(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("latest regenerations: %w", err)
	}
	byID := map[int64]*GridRow{}
	out := make([]GridRow, len(rows))
	for i := range rows {
		out[i] = GridRow{ListGridRow: rows[i]}
		byID[rows[i].ID] = &out[i]
	}
	for _, f := range fails {
		byID[f.ListingID].Failures = append(byID[f.ListingID].Failures, f)
	}
	for i := range regens {
		byID[regens[i].ListingID].Regeneration = &regens[i]
	}
	return out, nil
}

// Regenerations lists a listing's regeneration requests, newest first.
func (s *Service) Regenerations(ctx context.Context, listingID, beforeID int64, limit int32) ([]store.ListRegenerationsRow, error) {
	rows, err := store.New(s.pool).ListRegenerations(ctx, store.ListRegenerationsParams{ListingID: listingID, BeforeID: beforeID, PageSize: limit})
	if err != nil {
		return nil, fmt.Errorf("list regenerations: %w", err)
	}
	return rows, nil
}

// Rechecks is the latest config re-check per channel (D7).
func (s *Service) Rechecks(ctx context.Context) ([]store.LatestRechecksRow, error) {
	rows, err := store.New(s.pool).LatestRechecks(ctx)
	if err != nil {
		return nil, fmt.Errorf("latest rechecks: %w", err)
	}
	return rows, nil
}

// Budget reads the limit, the spend and the blocked state (US-00-012).
func (s *Service) Budget(ctx context.Context) (store.GetBudgetRow, error) {
	b, err := store.New(s.pool).GetBudget(ctx)
	if err != nil {
		return store.GetBudgetRow{}, fmt.Errorf("read budget: %w", err)
	}
	return b, nil
}
