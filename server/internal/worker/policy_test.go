package worker

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/KarthikReddy8809/catalift/internal/ai"
)

func TestDecide(t *testing.T) {
	start := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	boom := errors.New("provider error")
	cases := []struct {
		name     string
		err      error
		attempts int16
		now      time.Time
		want     Decision
	}{
		{"first failure retries after 10 s", boom, 0, start, Decision{Retry, 10 * time.Second, "provider error"}},
		{"second failure retries after 60 s", boom, 1, start, Decision{Retry, 60 * time.Second, "provider error"}},
		{"third failure ends the job", boom, 2, start, Decision{Fail, 0, "provider error"}},
		{"budget refusal stops the job", fmt.Errorf("reserve: %w", ai.ErrBudgetBlocked), 0, start, Decision{StopBudget, 0, "stopped: the AI budget is used up"}},
		{"429 waits for Retry-After without an attempt", &ai.RateLimitedError{RetryAfter: 45 * time.Second}, 2, start.Add(time.Minute), Decision{Wait, 45 * time.Second, "rate limited"}},
		{"429 with no Retry-After waits 30 s", &ai.RateLimitedError{}, 0, start, Decision{Wait, 30 * time.Second, "rate limited"}},
		{"a permanent failure ends the job at once", permanent("the product has no photo"), 0, start, Decision{Fail, 0, "the product has no photo"}},
		{"429 an hour after the first try ends the job", &ai.RateLimitedError{RetryAfter: time.Second}, 0, start.Add(time.Hour), Decision{Fail, 0, "rate limited"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Decide(c.err, c.attempts, start, c.now)

			if got != c.want {
				t.Fatalf("got %+v, want %+v", got, c.want)
			}
		})
	}
}
