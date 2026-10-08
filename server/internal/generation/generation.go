// Package generation starts and resumes generation runs (US-00-003, D6):
// it decides which products need detection and which (product, channel)
// pairs need a listing, and queues that work in the same transaction as the
// state change (ADR-0005).
package generation

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
	"github.com/KarthikReddy8809/catalift/server/internal/listings"
	"github.com/KarthikReddy8809/catalift/server/internal/store"
)

// VoiceNoteRequiredError names the brands that have no voice note
// (AC-US-00-003-5); the seller adds one or confirms a neutral voice.
type VoiceNoteRequiredError struct{ Brands []string }

func (e *VoiceNoteRequiredError) Error() string { return "a brand in scope has no voice note" }

// ErrRunNotFound is an unknown run id.
var ErrRunNotFound = errors.New("no such generation run")

// Progress is a run's counts (GenerationRun schema).
type Progress struct {
	ID        int64
	CreatedAt time.Time
	Detection map[string]int32
	Listings  map[string]int32
}

// Service queues generation work.
type Service struct {
	pool  *pgxpool.Pool
	rules *channels.Registry
}

// NewService builds the service.
func NewService(pool *pgxpool.Pool, rules *channels.Registry) *Service {
	return &Service{pool: pool, rules: rules}
}

// Start creates a run and queues every (product, enabled channel) with no
// listing or a failed or stopped one, and detection where it is not done (D6).
func (s *Service) Start(ctx context.Context, userID int64, neutralConfirmed bool, uploadID int64) (Progress, error) {
	q := store.New(s.pool)
	b, err := q.GetBudget(ctx)
	if err != nil {
		return Progress{}, fmt.Errorf("read budget: %w", err)
	}
	if b.BlockedAt.Valid {
		return Progress{}, ai.ErrBudgetBlocked
	}
	upload := pgtype.Int8{Int64: uploadID, Valid: uploadID != 0}
	if !neutralConfirmed {
		missing, err := q.BrandsWithoutVoice(ctx, upload)
		if err != nil {
			return Progress{}, fmt.Errorf("brands without voice: %w", err)
		}
		if len(missing) > 0 {
			return Progress{}, &VoiceNoteRequiredError{Brands: missing}
		}
	}
	var runID int64
	err = listings.InTx(ctx, s.pool, func(tq *store.Queries) error {
		run, err := tq.CreateRun(ctx, store.CreateRunParams{StartedBy: userID, NeutralVoiceConfirmed: neutralConfirmed})
		if err != nil {
			return fmt.Errorf("create run: %w", err)
		}
		runID = run.ID
		ids, err := tq.ProductIDsForRun(ctx, upload)
		if err != nil {
			return fmt.Errorf("products for run: %w", err)
		}
		return s.queue(ctx, tq, runID, ids)
	})
	if err != nil {
		return Progress{}, err
	}
	return s.Progress(ctx, runID)
}

// Resume queues a run's failed and stopped work again (D6); live work is
// not queued twice, so a repeat is harmless.
func (s *Service) Resume(ctx context.Context, runID int64) (Progress, error) {
	q := store.New(s.pool)
	if _, err := q.GetRun(ctx, runID); errors.Is(err, pgx.ErrNoRows) {
		return Progress{}, ErrRunNotFound
	} else if err != nil {
		return Progress{}, fmt.Errorf("load run: %w", err)
	}
	b, err := q.GetBudget(ctx)
	if err != nil {
		return Progress{}, fmt.Errorf("read budget: %w", err)
	}
	if b.BlockedAt.Valid {
		return Progress{}, ai.ErrBudgetBlocked
	}
	err = listings.InTx(ctx, s.pool, func(tq *store.Queries) error {
		ids, err := tq.RunProductIDs(ctx, pgtype.Int8{Int64: runID, Valid: true})
		if err != nil {
			return fmt.Errorf("run products: %w", err)
		}
		return s.queue(ctx, tq, runID, ids)
	})
	if err != nil {
		return Progress{}, err
	}
	return s.Progress(ctx, runID)
}

func (s *Service) queue(ctx context.Context, q *store.Queries, runID int64, productIDs []int64) error {
	run := pgtype.Int8{Int64: runID, Valid: true}
	for _, pid := range productIDs {
		if err := q.EnsureAttributes(ctx, pid); err != nil {
			return fmt.Errorf("ensure attributes %d: %w", pid, err)
		}
		for _, ch := range s.rules.Current().IDs() {
			if _, err := q.QueueListing(ctx, store.QueueListingParams{ProductID: pid, Channel: ch}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("queue listing %d %s: %w", pid, ch, err)
			}
		}
		a, err := q.GetAttributes(ctx, pid)
		if err != nil {
			return fmt.Errorf("attributes %d: %w", pid, err)
		}
		if a.DetectionStatus != store.DetectionStatusDone {
			if err := q.ResetDetectionPending(ctx, pid); err != nil {
				return fmt.Errorf("reset detection %d: %w", pid, err)
			}
			if _, err := q.EnqueueJob(ctx, store.EnqueueJobParams{
				Type: store.JobTypeDetectAttributes, DedupeKey: fmt.Sprintf("detect:p%d", pid), RunID: run, ProductID: pid,
			}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("enqueue detection %d: %w", pid, err)
			}
			continue
		}
		queued, err := q.QueuedListingsForProduct(ctx, pid)
		if err != nil {
			return fmt.Errorf("queued listings %d: %w", pid, err)
		}
		for _, l := range queued {
			if err := EnqueueGenerate(ctx, q, run, pid, l.ID, a.Revision); err != nil {
				return err
			}
		}
	}
	return nil
}

// EnqueueGenerate queues writing one listing from one attribute revision.
func EnqueueGenerate(ctx context.Context, q *store.Queries, run pgtype.Int8, productID, listingID int64, revision int32) error {
	if _, err := q.EnqueueJob(ctx, store.EnqueueJobParams{
		Type: store.JobTypeGenerateListing, DedupeKey: fmt.Sprintf("generate:l%d:r%d", listingID, revision),
		RunID: run, ProductID: productID, ListingID: pgtype.Int8{Int64: listingID, Valid: true},
		AttributesRevision: pgtype.Int4{Int32: revision, Valid: true},
	}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("enqueue generation %d: %w", listingID, err)
	}
	return nil
}

// Progress reads a run's counts.
func (s *Service) Progress(ctx context.Context, runID int64) (Progress, error) {
	q := store.New(s.pool)
	run, err := q.GetRun(ctx, runID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Progress{}, ErrRunNotFound
	}
	if err != nil {
		return Progress{}, fmt.Errorf("load run: %w", err)
	}
	id := pgtype.Int8{Int64: runID, Valid: true}
	d, err := q.RunProgress(ctx, id)
	if err != nil {
		return Progress{}, fmt.Errorf("detection progress: %w", err)
	}
	l, err := q.RunListingProgress(ctx, id)
	if err != nil {
		return Progress{}, fmt.Errorf("listing progress: %w", err)
	}
	return Progress{
		ID: run.ID, CreatedAt: run.CreatedAt.Time,
		Detection: map[string]int32{"pending": d.DetectionPending, "done": d.DetectionDone, "failed": d.DetectionFailed, "stopped_budget": d.DetectionStopped},
		Listings:  map[string]int32{"queued": l.Queued, "generated": l.Generated, "failed": l.Failed, "stopped_budget": l.Stopped},
	}, nil
}

// ErrNoPhoto means the product has no photo, so there is nothing to look at.
var ErrNoPhoto = errors.New("the product has no photo")

// EnrichOne runs the one-call enrichment again for a single product after
// the seller fixed it (seller flow step 3): detection resets, every channel's
// listing is queued again, and one vision call writes them all. It counts
// toward the budget like any other call and is refused while it is blocked.
func (s *Service) EnrichOne(ctx context.Context, userID, productID int64) (Progress, error) {
	q := store.New(s.pool)
	b, err := q.GetBudget(ctx)
	if err != nil {
		return Progress{}, fmt.Errorf("read budget: %w", err)
	}
	if b.BlockedAt.Valid {
		return Progress{}, ai.ErrBudgetBlocked
	}
	if _, err := q.FirstImage(ctx, productID); errors.Is(err, pgx.ErrNoRows) {
		return Progress{}, ErrNoPhoto
	} else if err != nil {
		return Progress{}, fmt.Errorf("first image: %w", err)
	}
	var runID int64
	err = listings.InTx(ctx, s.pool, func(tq *store.Queries) error {
		run, err := tq.CreateRun(ctx, store.CreateRunParams{StartedBy: userID, NeutralVoiceConfirmed: true})
		if err != nil {
			return fmt.Errorf("create run: %w", err)
		}
		runID = run.ID
		if err := tq.EnsureAttributes(ctx, productID); err != nil {
			return fmt.Errorf("ensure attributes: %w", err)
		}
		if err := tq.ForceDetectionPending(ctx, productID); err != nil {
			return fmt.Errorf("reset detection: %w", err)
		}
		for _, ch := range s.rules.Current().IDs() {
			if _, err := tq.QueueListing(ctx, store.QueueListingParams{ProductID: productID, Channel: ch}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("queue listing %s: %w", ch, err)
			}
		}
		if err := tq.RequeueProductListings(ctx, productID); err != nil {
			return fmt.Errorf("requeue listings: %w", err)
		}
		if _, err := tq.EnqueueJob(ctx, store.EnqueueJobParams{
			Type: store.JobTypeDetectAttributes, DedupeKey: fmt.Sprintf("detect:p%d", productID),
			RunID: pgtype.Int8{Int64: runID, Valid: true}, ProductID: productID,
		}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("enqueue enrichment: %w", err)
		}
		return nil
	})
	if err != nil {
		return Progress{}, err
	}
	return s.Progress(ctx, runID)
}
