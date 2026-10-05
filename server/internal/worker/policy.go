package worker

import (
	"errors"
	"time"

	"github.com/KarthikReddy8809/catalift/server/internal/ai"
)

// Retry policy (HLD section 6; eng review D14, D15).
const (
	MaxAttempts      = 3
	rateLimitDefault = 30 * time.Second
	rateLimitCap     = time.Hour
)

// retryDelays are the waits before the second and third attempts (D14).
var retryDelays = [...]time.Duration{10 * time.Second, 60 * time.Second}

// Outcome says what happens to a job after a failed try.
type Outcome int

// Outcomes of a failed try.
const (
	// Retry queues the job again after Delay, using one attempt.
	Retry Outcome = iota
	// Wait queues the job again after Delay without using an attempt (a 429).
	Wait
	// Fail ends the job and its work as failed.
	Fail
	// StopBudget ends the job and its work as stopped by the budget.
	StopBudget
)

// Decision is the policy's answer for one failed try.
type Decision struct {
	Outcome Outcome
	Delay   time.Duration
	Reason  string
}

// Decide applies D14 and D15 to a failed try. attempts is the number of
// attempts used before this one; firstTry is when the job was first run.
func Decide(err error, attempts int16, firstTry, now time.Time) Decision {
	if errors.Is(err, ai.ErrBudgetBlocked) {
		return Decision{Outcome: StopBudget, Reason: "stopped: the AI budget is used up"}
	}
	var pe permanentError
	if errors.As(err, &pe) {
		return Decision{Outcome: Fail, Reason: pe.reason}
	}
	var rl *ai.RateLimitedError
	if errors.As(err, &rl) {
		if now.Sub(firstTry) >= rateLimitCap {
			return Decision{Outcome: Fail, Reason: "rate limited"}
		}
		d := rl.RetryAfter
		if d <= 0 {
			d = rateLimitDefault
		}
		return Decision{Outcome: Wait, Delay: d, Reason: "rate limited"}
	}
	used := int(attempts) + 1
	if used >= MaxAttempts {
		return Decision{Outcome: Fail, Reason: err.Error()}
	}
	return Decision{Outcome: Retry, Delay: retryDelays[used-1], Reason: err.Error()}
}
