// Package auth owns seeded users, server-side sessions and roles (ADR-0006).
// Passwords are argon2id hashes; session and CSRF tokens are random and
// stored only as SHA-256 hashes, so a database read hands out no session.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/argon2"

	"github.com/KarthikReddy8809/catalift/internal/store"
)

// Role is seller or reviewer (ADR-0006).
type Role string

// The two roles (ADR-0006).
const (
	RoleSeller   Role = "seller"
	RoleReviewer Role = "reviewer"
)

// Principal is the signed-in user a request runs as.
type Principal struct {
	UserID       int64
	Email        string
	Role         Role
	CSRFHash     string
	SessionToken string
	ExpiresAt    time.Time
}

// SessionLength is how long a session lasts (HLD assumption: 7 days; retention UNDEFINED).
const SessionLength = 7 * 24 * time.Hour

// ErrBadCredentials is the one answer for a wrong email or password, so the
// response does not reveal which (T-02).
var ErrBadCredentials = errors.New("email or password is wrong")

// argon2id parameters (OWASP minimum: 19 MiB, 2 iterations).
const (
	argonTime    = 2
	argonMemory  = 19 * 1024
	argonThreads = 1
	argonKeyLen  = 32
)

// HashPassword returns an encoded argon2id hash with its salt and parameters.
func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// VerifyPassword checks a password against an encoded hash in constant time.
func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) == 0 || len(want) > 64 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, t, m, p, uint32(len(want))) //nolint:gosec // ADR-0006: len(want) is checked to be at most 64 just above.
	return subtle.ConstantTimeCompare(got, want) == 1
}

// NewToken returns 32 random bytes, URL-safe encoded.
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken is how a session or CSRF token is stored and looked up.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Service signs people in and resolves sessions.
type Service struct {
	q   *store.Queries
	now func() time.Time
	// dummyHash is verified when the email is unknown, so a wrong email takes
	// as long as a wrong password (T-02).
	dummyHash string
}

// NewService builds the service over the database.
func NewService(q *store.Queries) *Service {
	h, _ := HashPassword("timing-equaliser") // the error only comes from the OS random source; an empty hash just fails to verify
	return &Service{q: q, now: time.Now, dummyHash: h}
}

// Login checks the credentials and creates a session. It returns the session
// token and the CSRF token, both shown to the client once.
func (s *Service) Login(ctx context.Context, email, password string) (Principal, string, error) {
	u, err := s.q.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		VerifyPassword(s.dummyHash, password)
		return Principal{}, "", ErrBadCredentials
	}
	if err != nil {
		return Principal{}, "", fmt.Errorf("look up user: %w", err)
	}
	if !VerifyPassword(u.PasswordHash, password) {
		return Principal{}, "", ErrBadCredentials
	}
	token, err := NewToken()
	if err != nil {
		return Principal{}, "", err
	}
	csrf := CSRFFor(token)
	expires := s.now().Add(SessionLength)
	if err := s.q.CreateSession(ctx, store.CreateSessionParams{
		TokenHash: HashToken(token), UserID: u.ID, CsrfTokenHash: HashToken(csrf),
		ExpiresAt: tsz(expires),
	}); err != nil {
		return Principal{}, "", fmt.Errorf("create session: %w", err)
	}
	return Principal{UserID: u.ID, Email: u.Email, Role: Role(u.Role), CSRFHash: HashToken(csrf), SessionToken: token, ExpiresAt: expires}, csrf, nil
}

// CSRFFor derives a session's CSRF token from its session token, so the
// current session can hand it out again after a page reload without storing
// it. The session token is HttpOnly and the derivation is one-way (ADR-0006).
func CSRFFor(sessionToken string) string {
	sum := sha256.Sum256([]byte("csrf\x00" + sessionToken))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// Resolve returns the principal for a session token, or ok=false.
func (s *Service) Resolve(ctx context.Context, token string) (Principal, bool, error) {
	if token == "" {
		return Principal{}, false, nil
	}
	row, err := s.q.GetSession(ctx, HashToken(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, false, nil
	}
	if err != nil {
		return Principal{}, false, fmt.Errorf("look up session: %w", err)
	}
	return Principal{
		UserID: row.UserID, Email: row.Email, Role: Role(row.Role), CSRFHash: row.CsrfTokenHash,
		SessionToken: token, ExpiresAt: row.ExpiresAt.Time,
	}, true, nil
}

// Logout ends a session.
func (s *Service) Logout(ctx context.Context, token string) error {
	if err := s.q.DeleteSession(ctx, HashToken(token)); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// CheckCSRF compares a request's CSRF token with the session's, in constant time.
func CheckCSRF(p Principal, token string) bool {
	return token != "" && subtle.ConstantTimeCompare([]byte(HashToken(token)), []byte(p.CSRFHash)) == 1
}

// Limiter is a fixed-window attempt counter per key (sign-in: 10 per 15
// minutes per IP, T-01). One process serves the API (ADR-0002), so memory is enough.
type Limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	now    func() time.Time
	seen   map[string]window
}

type window struct {
	start time.Time
	count int
}

// NewLimiter builds a limiter; now is injected for tests.
func NewLimiter(maxAttempts int, w time.Duration, now func() time.Time) *Limiter {
	return &Limiter{max: maxAttempts, window: w, now: now, seen: map[string]window{}}
}

// Allow counts an attempt and says whether it may proceed, and if not, how
// long until it may.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	w := l.seen[key]
	if now.Sub(w.start) >= l.window {
		w = window{start: now}
	}
	if w.count >= l.max {
		return false, w.start.Add(l.window).Sub(now)
	}
	w.count++
	l.seen[key] = w
	return true, 0
}
