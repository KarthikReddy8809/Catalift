package auth

import (
	"testing"
	"time"
)

func TestPasswordHashVerifies(t *testing.T) {
	hash, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}

	ok := VerifyPassword(hash, "correct horse battery")
	bad := VerifyPassword(hash, "wrong")

	if !ok || bad {
		t.Fatalf("verify: right=%v wrong=%v", ok, bad)
	}
}

func TestPasswordHashIsSalted(t *testing.T) {
	a, _ := HashPassword("same")
	b, _ := HashPassword("same")

	if a == b {
		t.Fatal("two hashes of one password are equal; the salt is missing")
	}
}

func TestVerifyRejectsMalformedHash(t *testing.T) {
	if VerifyPassword("not-a-hash", "x") {
		t.Fatal("a malformed hash verified")
	}
}

func TestTokenHashIsStable(t *testing.T) {
	tok, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}

	first, second := HashToken(tok), HashToken(tok)

	if first != second || first == tok {
		t.Fatal("token hash must be stable and differ from the token")
	}
}

func TestLimiterAllowsTenThenBlocks(t *testing.T) {
	now := time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC)
	l := NewLimiter(10, 15*time.Minute, func() time.Time { return now })

	for i := range 10 {
		if ok, _ := l.Allow("203.0.113.9"); !ok {
			t.Fatalf("attempt %d refused", i+1)
		}
	}
	ok, retry := l.Allow("203.0.113.9")

	if ok || retry != 15*time.Minute {
		t.Fatalf("11th attempt: ok=%v retry=%v", ok, retry)
	}
}

func TestLimiterResetsAfterTheWindow(t *testing.T) {
	now := time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC)
	l := NewLimiter(1, 15*time.Minute, func() time.Time { return now })
	l.Allow("203.0.113.9")

	now = now.Add(16 * time.Minute)
	ok, _ := l.Allow("203.0.113.9")

	if !ok {
		t.Fatal("still blocked after the window")
	}
}
