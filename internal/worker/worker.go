// Package worker runs the AI jobs from the Postgres job table (ADR-0005):
// attribute detection, listing generation and field regeneration. Each job
// is claimed with a token and a lease, so a job whose worker died is picked
// up again by the sweeper and a late finish from the old claim is ignored.
package worker

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KarthikReddy8809/catalift/internal/ai"
	"github.com/KarthikReddy8809/catalift/internal/channels"
	"github.com/KarthikReddy8809/catalift/internal/generation"
	"github.com/KarthikReddy8809/catalift/internal/listings"
	"github.com/KarthikReddy8809/catalift/internal/store"
)

// Tuning (HLD section 6).
const (
	Concurrency   = 4
	LeaseSeconds  = 120
	idlePoll      = 2 * time.Second
	sweepInterval = time.Minute
)

// Field length caps from the listings table's CHECK constraint.
var fieldMax = map[string]int{
	"title": 500, "bullet_1": 1000, "bullet_2": 1000, "bullet_3": 1000,
	"bullet_4": 1000, "bullet_5": 1000, "description": 10000,
}

// Worker claims and runs jobs.
type Worker struct {
	pool     *pgxpool.Pool
	gw       *ai.Gateway
	channels channels.Set
	log      *slog.Logger
	name     string
	now      func() time.Time
}

// New builds a worker. name identifies it on claimed jobs.
func New(pool *pgxpool.Pool, gw *ai.Gateway, set channels.Set, log *slog.Logger, name string) *Worker {
	return &Worker{pool: pool, gw: gw, channels: set, log: log, name: name, now: time.Now}
}

// Run claims jobs on Concurrency loops and sweeps stale work every minute,
// until ctx is done; then it waits for the jobs in hand to finish.
func (w *Worker) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for range Concurrency {
		wg.Go(func() { w.loop(ctx) })
	}
	wg.Go(func() { w.sweep(ctx) })
	wg.Wait()
}

func (w *Worker) loop(ctx context.Context) {
	for ctx.Err() == nil {
		ran, err := w.RunOne(ctx)
		if err != nil {
			w.log.Error("job loop", "err", err)
		}
		if !ran || err != nil {
			select {
			case <-ctx.Done():
			case <-time.After(idlePoll):
			}
		}
	}
}

func (w *Worker) sweep(ctx context.Context) {
	t := time.NewTicker(sweepInterval)
	defer t.Stop()
	for {
		q := store.New(w.pool)
		if n, err := q.SweepStaleJobs(ctx); err != nil && ctx.Err() == nil {
			w.log.Error("sweep stale jobs", "err", err)
		} else if n > 0 {
			w.log.Warn("requeued stale jobs", "count", n)
		}
		// A reservation older than two leases belongs to a call whose worker
		// died; it is counted as possibly charged (data model section 4).
		if n, err := q.SweepReservations(ctx, 2*LeaseSeconds); err != nil && ctx.Err() == nil {
			w.log.Error("sweep reservations", "err", err)
		} else if n > 0 {
			w.log.Warn("closed stale reservations", "count", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// RunOne claims one job and runs it. It reports whether a job was claimed.
// The job runs to the end even if ctx is cancelled meanwhile (graceful stop).
func (w *Worker) RunOne(ctx context.Context) (bool, error) {
	var token pgtype.UUID
	if _, err := rand.Read(token.Bytes[:]); err != nil {
		return false, fmt.Errorf("claim token: %w", err)
	}
	token.Valid = true
	q := store.New(w.pool)
	job, err := q.ClaimJob(ctx, store.ClaimJobParams{
		Worker: pgtype.Text{String: w.name, Valid: true}, ClaimToken: token, LeaseSeconds: LeaseSeconds,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim job: %w", err)
	}
	jctx := context.WithoutCancel(ctx)
	log := w.log.With("job_id", job.ID, "type", job.Type, "product_id", job.ProductID)
	var runErr error
	switch job.Type {
	case store.JobTypeDetectAttributes:
		runErr = w.detect(jctx, job)
	case store.JobTypeGenerateListing:
		runErr = w.generate(jctx, job)
	case store.JobTypeRegenerateField:
		runErr = w.regenerate(jctx, job)
	default:
		runErr = fmt.Errorf("unknown job type %q", job.Type)
	}
	if runErr == nil {
		return true, w.finish(jctx, job.ID, token, store.JobStatusDone, "")
	}
	d := Decide(runErr, job.Attempts, job.FirstAttemptAt.Time, w.now())
	log.Warn("job try failed", "err", runErr, "outcome", d.Outcome, "attempts_before", job.Attempts)
	switch d.Outcome {
	case Retry, Wait:
		add := int16(0)
		if d.Outcome == Retry {
			add = 1
		}
		if _, err := q.RetryJob(jctx, store.RetryJobParams{
			AddAttempt: add, DelaySeconds: int32(d.Delay / time.Second), //nolint:gosec // US-00-003: delays are at most an hour.
			LastError: pgtype.Text{String: clipReason(d.Reason), Valid: true}, ID: job.ID, ClaimToken: token,
		}); err != nil {
			return true, fmt.Errorf("retry job %d: %w", job.ID, err)
		}
		return true, nil
	case StopBudget:
		if err := w.endWork(jctx, job, store.DetectionStatusStoppedBudget, store.ListingStatusStoppedBudget, store.RegenerationStatusStoppedBudget, d.Reason); err != nil {
			return true, err
		}
		return true, w.finish(jctx, job.ID, token, store.JobStatusStoppedBudget, d.Reason)
	case Fail:
		if err := w.endWork(jctx, job, store.DetectionStatusFailed, store.ListingStatusFailed, store.RegenerationStatusFailed, d.Reason); err != nil {
			return true, err
		}
		return true, w.finish(jctx, job.ID, token, store.JobStatusFailed, d.Reason)
	}
	return true, fmt.Errorf("job %d: unknown outcome %d", job.ID, d.Outcome)
}

func (w *Worker) finish(ctx context.Context, id int64, token pgtype.UUID, status store.JobStatus, reason string) error {
	n, err := store.New(w.pool).FinishJob(ctx, store.FinishJobParams{
		Status: status, LastError: pgtype.Text{String: clipReason(reason), Valid: reason != ""}, ID: id, ClaimToken: token,
	})
	if err != nil {
		return fmt.Errorf("finish job %d: %w", id, err)
	}
	if n == 0 {
		w.log.Warn("job claim lost before finish; the sweeper requeued it", "job_id", id)
	}
	return nil
}

// endWork marks what the job was producing as ended, so the seller and the
// reviewer see why (AC-US-00-003-3, US-00-012).
func (w *Worker) endWork(ctx context.Context, job store.ClaimJobRow, ds store.DetectionStatus, ls store.ListingStatus, rs store.RegenerationStatus, reason string) error {
	q := store.New(w.pool)
	// Only a failure carries its reason; a budget stop is its own status
	// (chk_product_attributes_error_iff_failed, chk_listings_failure_reason).
	failed := ds == store.DetectionStatusFailed
	r := pgtype.Text{String: clipReason(reason), Valid: failed}
	switch job.Type {
	case store.JobTypeDetectAttributes:
		if err := q.SetDetectionEnded(ctx, store.SetDetectionEndedParams{ProductID: job.ProductID, DetectionStatus: ds, DetectionError: r}); err != nil {
			return fmt.Errorf("end detection: %w", err)
		}
		why := pgtype.Text{String: clipReason("attribute detection failed: " + reason), Valid: failed}
		if err := q.EndQueuedListingsForProduct(ctx, store.EndQueuedListingsForProductParams{ProductID: job.ProductID, Status: ls, FailureReason: why}); err != nil {
			return fmt.Errorf("end listings: %w", err)
		}
	case store.JobTypeGenerateListing:
		if err := q.SetListingEnded(ctx, store.SetListingEndedParams{ID: job.ListingID.Int64, Status: ls, FailureReason: r}); err != nil {
			return fmt.Errorf("end listing: %w", err)
		}
	case store.JobTypeRegenerateField:
		if err := q.SetRegenerationStatus(ctx, store.SetRegenerationStatusParams{ID: job.RegenerationRequestID.Int64, Status: rs}); err != nil {
			return fmt.Errorf("end regeneration: %w", err)
		}
	}
	return nil
}

func (w *Worker) product(ctx context.Context, q *store.Queries, id int64) (ai.Product, error) {
	p, err := q.GetProductForPrompt(ctx, id)
	if err != nil {
		return ai.Product{}, fmt.Errorf("load product %d: %w", id, err)
	}
	return ai.Product{SKU: p.Sku, Brand: p.BrandName, Category: p.Category, VoiceNote: p.VoiceNote.String}, nil
}

func attrsOf(a store.GetAttributesRow) ai.Attributes {
	return ai.Attributes{Colour: a.Colour.String, Pattern: a.Pattern.String, Sleeve: a.Sleeve.String, Neckline: a.Neckline.String, Fit: a.Fit.String}
}

func brief(c channels.Channel) ai.ChannelBrief {
	return ai.ChannelBrief{Name: c.Name, TitleMaxLength: c.TitleMaxLength, BannedWords: c.BannedWords}
}

// detect reads the five attributes from the product's first photo (D23),
// then queues a listing per queued channel at the new revision.
func (w *Worker) detect(ctx context.Context, job store.ClaimJobRow) error {
	q := store.New(w.pool)
	p, err := w.product(ctx, q, job.ProductID)
	if err != nil {
		return err
	}
	img, err := q.FirstImage(ctx, job.ProductID)
	if errors.Is(err, pgx.ErrNoRows) {
		return permanent("the product has no photo")
	}
	if err != nil {
		return fmt.Errorf("first image: %w", err)
	}
	data, err := os.ReadFile(img.DetectionPath)
	if err != nil {
		return fmt.Errorf("read detection copy: %w", err)
	}
	resp, err := w.gw.Call(ctx, job.ID, job.ProductID, ai.Request{
		Purpose: ai.PurposeDetect, TemplateID: ai.TemplateDetect, Prompt: ai.DetectPrompt(p),
		Image: data, ImageType: "image/jpeg",
	})
	if err != nil {
		return err
	}
	a, err := ai.ParseAttributes(resp.Text)
	if err != nil {
		return err
	}
	t := func(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }
	return listings.InTx(ctx, w.pool, func(tq *store.Queries) error {
		rev, err := tq.SetDetectionDone(ctx, store.SetDetectionDoneParams{
			ProductID: job.ProductID, Colour: t(a.Colour), Pattern: t(a.Pattern), Sleeve: t(a.Sleeve), Neckline: t(a.Neckline), Fit: t(a.Fit),
		})
		if err != nil {
			return fmt.Errorf("store attributes: %w", err)
		}
		queued, err := tq.QueuedListingsForProduct(ctx, job.ProductID)
		if err != nil {
			return fmt.Errorf("queued listings: %w", err)
		}
		for _, l := range queued {
			if err := generation.EnqueueGenerate(ctx, tq, job.RunID, job.ProductID, l.ID, rev); err != nil {
				return err
			}
		}
		return nil
	})
}

// generate writes one channel's listing from the attribute revision the job
// was queued with; if a correction raised the revision meanwhile, the job
// re-queues itself at the new one instead of writing stale text (D5).
func (w *Worker) generate(ctx context.Context, job store.ClaimJobRow) error {
	q := store.New(w.pool)
	l, err := q.GetListing(ctx, job.ListingID.Int64)
	if err != nil {
		return fmt.Errorf("load listing: %w", err)
	}
	ch, ok := w.channels.Get(l.Channel)
	if !ok {
		return permanent("the channel " + l.Channel + " is switched off")
	}
	a, err := q.GetAttributes(ctx, job.ProductID)
	if err != nil {
		return fmt.Errorf("load attributes: %w", err)
	}
	if a.Revision != job.AttributesRevision.Int32 {
		return generation.EnqueueGenerate(ctx, q, job.RunID, job.ProductID, l.ID, a.Revision)
	}
	p, err := w.product(ctx, q, job.ProductID)
	if err != nil {
		return err
	}
	resp, err := w.gw.Call(ctx, job.ID, job.ProductID, ai.Request{
		Purpose: ai.PurposeGenerate, TemplateID: ai.TemplateGenerate, Prompt: ai.GeneratePrompt(p, attrsOf(a), brief(ch)),
	})
	if err != nil {
		return err
	}
	text, err := ai.ParseListing(resp.Text)
	if err != nil {
		return err
	}
	t := func(s string, n int) pgtype.Text { return pgtype.Text{String: clip(s, n), Valid: true} }
	return listings.InTx(ctx, w.pool, func(tq *store.Queries) error {
		cur, err := tq.GetAttributes(ctx, job.ProductID)
		if err != nil {
			return fmt.Errorf("recheck attributes: %w", err)
		}
		if cur.Revision != job.AttributesRevision.Int32 {
			return generation.EnqueueGenerate(ctx, tq, job.RunID, job.ProductID, l.ID, cur.Revision)
		}
		if _, err := tq.WriteGenerated(ctx, store.WriteGeneratedParams{
			ID: l.ID, Title: t(text.Title, fieldMax["title"]),
			Bullet1: t(text.Bullets[0], 1000), Bullet2: t(text.Bullets[1], 1000), Bullet3: t(text.Bullets[2], 1000),
			Bullet4: t(text.Bullets[3], 1000), Bullet5: t(text.Bullets[4], 1000), Description: t(text.Description, fieldMax["description"]),
		}); err != nil {
			return fmt.Errorf("write listing: %w", err)
		}
		_, _, err = listings.Revalidate(ctx, tq, w.channels, l.ID)
		return err
	})
}

// regenerate rewrites one field; the result applies only if the field still
// holds the value the reviewer saw when asking (D12), else it is superseded.
func (w *Worker) regenerate(ctx context.Context, job store.ClaimJobRow) error {
	q := store.New(w.pool)
	rg, err := q.GetRegeneration(ctx, job.RegenerationRequestID.Int64)
	if err != nil {
		return fmt.Errorf("load regeneration: %w", err)
	}
	l, err := q.GetListing(ctx, rg.ListingID)
	if err != nil {
		return fmt.Errorf("load listing: %w", err)
	}
	ch, ok := w.channels.Get(l.Channel)
	if !ok {
		return permanent("the channel " + l.Channel + " is switched off")
	}
	a, err := q.GetAttributes(ctx, job.ProductID)
	if err != nil {
		return fmt.Errorf("load attributes: %w", err)
	}
	p, err := w.product(ctx, q, job.ProductID)
	if err != nil {
		return err
	}
	resp, err := w.gw.Call(ctx, job.ID, job.ProductID, ai.Request{
		Purpose: ai.PurposeRegenerate, TemplateID: ai.TemplateRegenerate,
		Prompt: ai.RegeneratePrompt(p, attrsOf(a), brief(ch), rg.Field, rg.BaseValue, rg.Instruction),
	})
	if err != nil {
		return err
	}
	value, err := ai.ParseField(resp.Text, fieldMax[rg.Field])
	if err != nil {
		return err
	}
	return listings.InTx(ctx, w.pool, func(tq *store.Queries) error {
		_, err := tq.SetFieldIfUnchanged(ctx, store.SetFieldIfUnchangedParams{Field: rg.Field, Value: value, ID: rg.ListingID, BaseValue: rg.BaseValue})
		status := store.RegenerationStatusApplied
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			status = store.RegenerationStatusSuperseded
		case err != nil:
			return fmt.Errorf("apply regeneration: %w", err)
		default:
			if _, _, err := listings.Revalidate(ctx, tq, w.channels, rg.ListingID); err != nil {
				return err
			}
		}
		if err := tq.SetRegenerationStatus(ctx, store.SetRegenerationStatusParams{ID: rg.ID, Status: status}); err != nil {
			return fmt.Errorf("regeneration status: %w", err)
		}
		return nil
	})
}

// permanentError is a failure no retry can fix.
type permanentError struct{ reason string }

func (e permanentError) Error() string { return e.reason }

func permanent(reason string) error { return permanentError{reason: reason} }

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

func clipReason(s string) string { return clip(s, 500) }
