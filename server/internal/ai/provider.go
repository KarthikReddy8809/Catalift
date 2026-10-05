// Package ai is the one path to a language model (tenet 2): the gateway
// checks the budget, reserves an estimate, calls the provider and records
// tokens and cost (REQ-020, REQ-021). Only the worker uses it.
package ai

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Purpose is what a call is for; it is recorded on the ledger.
type Purpose string

// Purposes of a call, recorded on the ledger (AC-US-00-011-1).
const (
	PurposeDetect     Purpose = "detect"
	PurposeGenerate   Purpose = "generate"
	PurposeRegenerate Purpose = "regenerate"
	PurposeEvalDetect Purpose = "eval_detect"
)

// Request is one model call. Image is optional (detection sends the 1024 px copy, D23).
type Request struct {
	Purpose    Purpose
	TemplateID string
	Prompt     string
	Image      []byte
	ImageType  string
}

// Response is the model's text and what it cost.
type Response struct {
	Text         string
	InputTokens  int
	OutputTokens int
	// CostMicroUSD is the provider's reported cost in millionths of a dollar.
	CostMicroUSD int64
	GenerationID string
	Model        string
}

// Provider calls a model.
type Provider interface {
	Name() string
	Complete(ctx context.Context, req Request) (Response, error)
}

// RateLimitedError is a 429: the call was refused before any work, so it is
// not charged and does not use one of a job's attempts (D15).
type RateLimitedError struct{ RetryAfter time.Duration }

func (e *RateLimitedError) Error() string {
	return fmt.Sprintf("provider rate limited; retry after %s", e.RetryAfter)
}

// ErrBudgetBlocked means the USD 8 block is on (REQ-021, D13); no call was made.
var ErrBudgetBlocked = errors.New("AI calls are blocked by the budget")

// ErrUncertain means the call may have reached the provider and been charged
// (a timeout or a dropped connection); it is recorded as possibly charged.
var ErrUncertain = errors.New("the AI call ended without a clear answer")
