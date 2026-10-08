package listings

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/KarthikReddy8809/catalift/server/internal/channels"
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

// CheckDraft runs the channel's rules on a listing with some fields replaced,
// without saving anything, so the editor's badge follows the reviewer's
// typing (reviewer flow step 3). Fields left nil keep the stored text.
func (s *Service) CheckDraft(ctx context.Context, listingID int64, e Edit) ([]channels.Failure, error) {
	q := store.New(s.pool)
	l, err := q.GetListing(ctx, listingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load listing: %w", err)
	}
	ch, ok := s.rules.Current().Get(l.Channel)
	if !ok {
		return nil, ErrNotFound
	}
	attrs := map[string]string{}
	if a, err := q.GetAttributes(ctx, l.ProductID); err == nil {
		attrs = map[string]string{"colour": txt(a.Colour), "pattern": txt(a.Pattern), "sleeve": txt(a.Sleeve), "neckline": txt(a.Neckline), "fit": txt(a.Fit)}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("load attributes: %w", err)
	}
	avoid, err := q.AvoidWordsForProduct(ctx, l.ProductID)
	if err != nil {
		return nil, fmt.Errorf("load brand words to avoid: %w", err)
	}
	pick := func(edit *string, stored pgtype.Text) string {
		if edit != nil {
			return *edit
		}
		return stored.String
	}
	stored := [5]pgtype.Text{l.Bullet1, l.Bullet2, l.Bullet3, l.Bullet4, l.Bullet5}
	var bullets [5]string
	for i := range bullets {
		bullets[i] = pick(e.Bullets[i], stored[i])
	}
	return channels.Validate(ch, channels.Listing{
		Title: pick(e.Title, l.Title), Bullets: bullets, Description: pick(e.Description, l.Description), Attributes: attrs,
		AvoidWords: avoid,
	}), nil
}
