package ai

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KarthikReddy8809/catalift/server/internal/store"
)

// Estimates reserved before a call, in micro-USD (Q-018 estimate). The
// reservation counts toward spend until the call is finalised.
var estimate = map[Purpose]int64{
	PurposeDetect:     4_200,
	PurposeGenerate:   5_000,
	PurposeRegenerate: 1_100,
	PurposeEvalDetect: 4_200,
}

// CallTimeout bounds one provider call (HLD section 6).
const CallTimeout = 60 * time.Second

// Gateway is the only way to reach a model.
type Gateway struct {
	pool     *pgxpool.Pool
	provider Provider
	model    string
}

// NewGateway wraps a provider with the budget and the ledger.
func NewGateway(pool *pgxpool.Pool, provider Provider, model string) *Gateway {
	return &Gateway{pool: pool, provider: provider, model: model}
}

// Provider names the provider in use, for logs.
func (g *Gateway) Provider() string { return g.provider.Name() }

// Call checks the budget under its lock, reserves the estimate, calls the
// provider and records the outcome. productID is 0 only for eval calls.
func (g *Gateway) Call(ctx context.Context, jobID, productID int64, req Request) (Response, error) {
	callID, err := g.reserve(ctx, jobID, productID, req)
	if err != nil {
		return Response{}, err
	}

	cctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	resp, callErr := g.provider.Complete(cctx, req)

	// The outcome is recorded even when the caller's context is done, so the
	// ledger never keeps a stale reservation for a call that finished.
	rec := context.WithoutCancel(ctx)
	var rl *RateLimitedError
	switch {
	case callErr == nil:
		err := g.finish(rec, callID, "succeeded", &resp, "")
		return resp, err
	case errors.As(callErr, &rl):
		if err := g.finish(rec, callID, "failed", &Response{}, callErr.Error()); err != nil {
			return Response{}, err
		}
		return Response{}, callErr
	case errors.Is(callErr, ErrUncertain) || errors.Is(callErr, context.DeadlineExceeded):
		if err := g.finish(rec, callID, "possibly_charged", nil, callErr.Error()); err != nil {
			return Response{}, err
		}
		return Response{}, fmt.Errorf("%w: %w", ErrUncertain, callErr)
	default:
		if err := g.finish(rec, callID, "failed", nil, callErr.Error()); err != nil {
			return Response{}, err
		}
		return Response{}, callErr
	}
}

func (g *Gateway) reserve(ctx context.Context, jobID, productID int64, req Request) (int64, error) {
	est := estimate[req.Purpose]
	if est == 0 {
		est = 5_000
	}
	tx, err := g.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin reserve: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // a no-op after Commit
	q := store.New(tx)
	b, err := q.LockBudget(ctx)
	if err != nil {
		return 0, fmt.Errorf("lock budget: %w", err)
	}
	if b.BlockedAt.Valid {
		return 0, ErrBudgetBlocked
	}
	spent, err := q.TotalSpend(ctx)
	if err != nil {
		return 0, fmt.Errorf("total spend: %w", err)
	}
	if spent+est > b.LimitMicroUsd {
		if err := q.SetBudgetBlocked(ctx); err != nil {
			return 0, fmt.Errorf("set budget blocked: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return 0, fmt.Errorf("commit block: %w", err)
		}
		return 0, ErrBudgetBlocked
	}
	id, err := q.ReserveCall(ctx, store.ReserveCallParams{
		JobID:            optInt(jobID),
		ProductID:        optInt(productID),
		Purpose:          store.AiCallPurpose(req.Purpose),
		Model:            g.model,
		PromptTemplateID: req.TemplateID,
		Prompt:           req.Prompt,
		ReservedMicroUsd: est,
	})
	if err != nil {
		return 0, fmt.Errorf("reserve call: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit reserve: %w", err)
	}
	return id, nil
}

func (g *Gateway) finish(ctx context.Context, id int64, status string, resp *Response, errText string) error {
	p := store.FinishCallParams{ID: id, Status: store.AiCallStatus(status)}
	if resp != nil {
		p.CostMicroUsd = pgtype.Int8{Int64: resp.CostMicroUSD, Valid: true}
		p.InputTokens = pgtype.Int4{Int32: int32(resp.InputTokens), Valid: true}   //nolint:gosec // US-00-011: token counts are far below 2^31.
		p.OutputTokens = pgtype.Int4{Int32: int32(resp.OutputTokens), Valid: true} //nolint:gosec // US-00-011: token counts are far below 2^31.
		if resp.GenerationID != "" {
			p.ProviderGenerationID = pgtype.Text{String: resp.GenerationID, Valid: true}
		}
	}
	if errText != "" {
		if len(errText) > 2000 {
			errText = errText[:2000]
		}
		p.Error = pgtype.Text{String: errText, Valid: true}
	}
	if err := store.New(g.pool).FinishCall(ctx, p); err != nil {
		return fmt.Errorf("record AI call %d: %w", id, err)
	}
	return nil
}

func optInt(v int64) pgtype.Int8 { return pgtype.Int8{Int64: v, Valid: v != 0} }
