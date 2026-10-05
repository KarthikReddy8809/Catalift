// Package listings owns everything that must agree about a listing: its
// text, version, rule results, approvals, attributes and regeneration
// requests (eng review D3, D5), so approving, editing and re-checking are
// each one transaction here.
package listings

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KarthikReddy8809/catalift/server/internal/ai"
	"github.com/KarthikReddy8809/catalift/server/internal/channels"
	"github.com/KarthikReddy8809/catalift/server/internal/store"
)

// Errors the handlers map to statuses.
var (
	ErrNotFound        = errors.New("no such listing")
	ErrVersionConflict = errors.New("the listing or attributes changed since the version sent")
	ErrNotGenerated    = errors.New("the listing has no generated text yet")
	ErrNotDetected     = errors.New("the product's attributes are not detected yet")
)

// Fields a reviewer may edit or regenerate.
var Fields = []string{"title", "bullet_1", "bullet_2", "bullet_3", "bullet_4", "bullet_5", "description"}

// Service runs listing operations over the database and the loaded channels.
type Service struct {
	pool     *pgxpool.Pool
	channels channels.Set
}

// NewService builds the service.
func NewService(pool *pgxpool.Pool, set channels.Set) *Service {
	return &Service{pool: pool, channels: set}
}

// Channels returns the loaded channel set.
func (s *Service) Channels() channels.Set { return s.channels }

func txt(t pgtype.Text) string { return t.String }

func text(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// Revalidate runs the rules engine on a listing's current version and stores
// the result, inside the caller's transaction (REQ-008). A listing whose
// channel is switched off stays unchecked (D8).
func Revalidate(ctx context.Context, q *store.Queries, set channels.Set, listingID int64) (store.RuleStatus, []channels.Failure, error) {
	l, err := q.GetListing(ctx, listingID)
	if err != nil {
		return "", nil, fmt.Errorf("load listing %d: %w", listingID, err)
	}
	ch, ok := set.Get(l.Channel)
	if !ok || l.Status != store.ListingStatusGenerated {
		return l.RuleStatus, nil, nil
	}
	attrs := map[string]string{}
	if a, err := q.GetAttributes(ctx, l.ProductID); err == nil {
		attrs = map[string]string{"colour": txt(a.Colour), "pattern": txt(a.Pattern), "sleeve": txt(a.Sleeve), "neckline": txt(a.Neckline), "fit": txt(a.Fit)}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return "", nil, fmt.Errorf("load attributes: %w", err)
	}
	failures := channels.Validate(ch, channels.Listing{
		Title:       txt(l.Title),
		Bullets:     [5]string{txt(l.Bullet1), txt(l.Bullet2), txt(l.Bullet3), txt(l.Bullet4), txt(l.Bullet5)},
		Description: txt(l.Description),
		Attributes:  attrs,
	})
	if err := q.DeleteRuleResults(ctx, listingID); err != nil {
		return "", nil, fmt.Errorf("clear rule results: %w", err)
	}
	ruleNames := []string{}
	for _, f := range failures {
		ruleNames = append(ruleNames, f.Rule)
		if err := q.InsertRuleResult(ctx, store.InsertRuleResultParams{
			ListingID: listingID, ListingVersion: l.Version, Rule: f.Rule, Field: f.Field, Message: f.Message, ConfigHash: ch.Hash,
		}); err != nil {
			return "", nil, fmt.Errorf("store rule result: %w", err)
		}
	}
	status := store.RuleStatusPassing
	if len(failures) > 0 {
		status = store.RuleStatusFailing
	}
	if err := q.SetRuleStatus(ctx, store.SetRuleStatusParams{
		ID: listingID, RuleStatus: status, RuleConfigHash: pgtype.Text{String: ch.Hash, Valid: true},
		FirstPassPassed: len(failures) == 0, FirstPassFailedRules: ruleNames,
	}); err != nil {
		return "", nil, fmt.Errorf("store rule status: %w", err)
	}
	return status, failures, nil
}

// InTx runs fn in one transaction.
func InTx(ctx context.Context, pool *pgxpool.Pool, fn func(q *store.Queries) error) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // a no-op after Commit
	if err := fn(store.New(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// Edit is a reviewer's change to some fields of one listing.
type Edit struct {
	Version     int32
	Title       *string
	Bullets     [5]*string
	Description *string
}

// UpdateText saves an edit made against a version: the version rises, the
// approval no longer matches (tenet 5) and the rules run again
// (AC-US-00-004-6). A stale version is ErrVersionConflict.
func (s *Service) UpdateText(ctx context.Context, listingID int64, e Edit) error {
	return InTx(ctx, s.pool, func(q *store.Queries) error {
		_, err := q.UpdateListingText(ctx, store.UpdateListingTextParams{
			ID: listingID, ExpectedVersion: e.Version,
			Title: text(e.Title), Description: text(e.Description),
			Bullet1: text(e.Bullets[0]), Bullet2: text(e.Bullets[1]), Bullet3: text(e.Bullets[2]),
			Bullet4: text(e.Bullets[3]), Bullet5: text(e.Bullets[4]),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return s.explainMiss(ctx, q, listingID)
		}
		if err != nil {
			return fmt.Errorf("update listing: %w", err)
		}
		_, _, err = Revalidate(ctx, q, s.channels, listingID)
		return err
	})
}

func (s *Service) explainMiss(ctx context.Context, q *store.Queries, listingID int64) error {
	l, err := q.GetListing(ctx, listingID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return ErrNotFound
	case err != nil:
		return fmt.Errorf("load listing: %w", err)
	case l.Status != store.ListingStatusGenerated:
		return ErrNotGenerated
	default:
		return ErrVersionConflict
	}
}

// AttributesEdit is a reviewer's correction of detected attributes.
type AttributesEdit struct {
	Revision                               int32
	Colour, Pattern, Sleeve, Neckline, Fit *string
}

// CorrectAttributes saves a correction and, in the same transaction, raises
// every listing of the product, which clears their approvals, and re-checks
// them (D5).
func (s *Service) CorrectAttributes(ctx context.Context, productID, userID int64, e AttributesEdit) (store.CorrectAttributesRow, error) {
	var out store.CorrectAttributesRow
	err := InTx(ctx, s.pool, func(q *store.Queries) error {
		row, err := q.CorrectAttributes(ctx, store.CorrectAttributesParams{
			ProductID: productID, ExpectedRevision: e.Revision, CorrectedBy: pgtype.Int8{Int64: userID, Valid: true},
			Colour: text(e.Colour), Pattern: text(e.Pattern), Sleeve: text(e.Sleeve), Neckline: text(e.Neckline), Fit: text(e.Fit),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			a, gerr := q.GetAttributes(ctx, productID)
			switch {
			case errors.Is(gerr, pgx.ErrNoRows):
				return ErrNotFound
			case gerr != nil:
				return fmt.Errorf("load attributes: %w", gerr)
			case a.DetectionStatus != store.DetectionStatusDone:
				return ErrNotDetected
			default:
				return ErrVersionConflict
			}
		}
		if err != nil {
			return fmt.Errorf("correct attributes: %w", err)
		}
		out = row
		ids, err := q.BumpProductListings(ctx, productID)
		if err != nil {
			return fmt.Errorf("bump listings: %w", err)
		}
		for _, id := range ids {
			if _, _, err := Revalidate(ctx, q, s.channels, id); err != nil {
				return err
			}
		}
		return nil
	})
	return out, err
}

// ApproveItem is one listing at the version the reviewer saw.
type ApproveItem struct {
	ListingID int64
	Version   int32
}

// Approved is one listing approved now.
type Approved struct {
	ListingID  int64
	Version    int32
	ApprovedAt time.Time
}

// Skipped is a listing not approved, with the reason the reviewer sees.
type Skipped struct {
	ListingID int64
	Reason    string
}

// Approve approves each listing still at its version and passing its rules,
// one statement per listing, so an edit in between is never approved
// (AC-US-00-009-2, -3; Q-009). The rest are skipped with a reason.
func (s *Service) Approve(ctx context.Context, userID int64, items []ApproveItem) ([]Approved, []Skipped, error) {
	q := store.New(s.pool)
	var ok []Approved
	var skipped []Skipped
	for _, it := range items {
		at, err := q.ApproveIfCurrent(ctx, store.ApproveIfCurrentParams{ApprovedBy: userID, ListingID: it.ListingID, ListingVersion: it.Version})
		if err == nil {
			ok = append(ok, Approved{ListingID: it.ListingID, Version: it.Version, ApprovedAt: at.Time})
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, fmt.Errorf("approve listing %d: %w", it.ListingID, err)
		}
		l, err := q.GetListing(ctx, it.ListingID)
		reason := "not_found"
		switch {
		case errors.Is(err, pgx.ErrNoRows):
		case err != nil:
			return nil, nil, fmt.Errorf("load listing %d: %w", it.ListingID, err)
		case l.Status != store.ListingStatusGenerated:
			reason = "not_generated"
		case l.Version != it.Version:
			reason = "version_changed"
		default:
			reason = "failing_rules"
		}
		skipped = append(skipped, Skipped{ListingID: it.ListingID, Reason: reason})
	}
	return ok, skipped, nil
}

// RequestRegeneration records a reviewer's instruction and queues the job,
// in one transaction. The job carries the field's value now, so the result
// applies only if the reviewer has not changed that field since (D12).
func (s *Service) RequestRegeneration(ctx context.Context, listingID, userID int64, field, instruction string) (store.CreateRegenerationRow, error) {
	var out store.CreateRegenerationRow
	err := InTx(ctx, s.pool, func(q *store.Queries) error {
		b, err := q.GetBudget(ctx)
		if err != nil {
			return fmt.Errorf("read budget: %w", err)
		}
		if b.BlockedAt.Valid {
			return ai.ErrBudgetBlocked
		}
		l, err := q.GetListingForUpdate(ctx, listingID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("load listing: %w", err)
		}
		if l.Status != store.ListingStatusGenerated {
			return ErrNotGenerated
		}
		base := map[string]pgtype.Text{"title": l.Title, "bullet_1": l.Bullet1, "bullet_2": l.Bullet2, "bullet_3": l.Bullet3,
			"bullet_4": l.Bullet4, "bullet_5": l.Bullet5, "description": l.Description}[field]
		row, err := q.CreateRegeneration(ctx, store.CreateRegenerationParams{
			ListingID: listingID, Field: field, Instruction: instruction, BaseValue: base.String, RequestedBy: userID,
		})
		if err != nil {
			return fmt.Errorf("create regeneration: %w", err)
		}
		a, err := q.GetAttributes(ctx, l.ProductID)
		if err != nil {
			return fmt.Errorf("load attributes: %w", err)
		}
		if _, err := q.EnqueueJob(ctx, store.EnqueueJobParams{
			Type: store.JobTypeRegenerateField, DedupeKey: fmt.Sprintf("regen:l%d:%s:r%d", listingID, field, row.ID),
			ProductID: l.ProductID, ListingID: pgtype.Int8{Int64: listingID, Valid: true},
			RegenerationRequestID: pgtype.Int8{Int64: row.ID, Valid: true},
			AttributesRevision:    pgtype.Int4{Int32: a.Revision, Valid: true},
		}); err != nil {
			return fmt.Errorf("enqueue regeneration: %w", err)
		}
		out = row
		return nil
	})
	return out, err
}

// Recheck re-checks every generated listing whose channel config changed
// since it was last checked (D7). An approved listing that now fails gets a
// new version, which clears its approval. One config_rechecks row per
// channel records the counts the grid shows.
func (s *Service) Recheck(ctx context.Context) error {
	q := store.New(s.pool)
	for i := range s.channels.Channels {
		ch := &s.channels.Channels[i]
		ids, err := q.ListingsForRecheck(ctx, store.ListingsForRecheckParams{Channel: ch.ID, ConfigHash: ch.Hash})
		if err != nil {
			return fmt.Errorf("list listings for %s: %w", ch.ID, err)
		}
		if len(ids) == 0 {
			continue
		}
		cleared := 0
		for _, id := range ids {
			err := InTx(ctx, s.pool, func(tq *store.Queries) error {
				wasApproved, err := tq.IsApprovedNow(ctx, id)
				if err != nil {
					return fmt.Errorf("approval of %d: %w", id, err)
				}
				status, _, err := Revalidate(ctx, tq, s.channels, id)
				if err != nil {
					return err
				}
				if wasApproved && status == store.RuleStatusFailing {
					cleared++
					return tq.BumpListing(ctx, id)
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		if err := q.InsertConfigRecheck(ctx, store.InsertConfigRecheckParams{
			Channel: ch.ID, ConfigHash: ch.Hash, ListingsRechecked: int32(len(ids)), ApprovalsCleared: int32(cleared), //nolint:gosec // US-00-004: counts are bounded by the table size.
		}); err != nil {
			return fmt.Errorf("record recheck for %s: %w", ch.ID, err)
		}
	}
	return nil
}
