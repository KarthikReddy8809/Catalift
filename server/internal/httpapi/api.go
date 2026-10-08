package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/KarthikReddy8809/catalift/server/internal/auth"
	"github.com/KarthikReddy8809/catalift/server/internal/catalogue"
	"github.com/KarthikReddy8809/catalift/server/internal/channels"
	"github.com/KarthikReddy8809/catalift/server/internal/exports"
	"github.com/KarthikReddy8809/catalift/server/internal/generation"
	"github.com/KarthikReddy8809/catalift/server/internal/listings"
)

// SessionCookie names the session cookie (ADR-0006).
const SessionCookie = "catalift_session"

// Deps are the services the /v1 routes call.
type Deps struct {
	Auth          *auth.Service
	SignInLimiter *auth.Limiter
	Catalogue     *catalogue.Service
	Generation    *generation.Service
	Listings      *listings.Service
	Exports       *exports.Service
	Channels      *channels.Registry
	SecureCookies bool
}

type api struct {
	Deps
	log  *slog.Logger
	idem *idempotency
}

type access int

const (
	anyRole access = iota
	reviewerOnly
	// sellerOnly: bringing products in is the seller's job; reviewers work on
	// what is already there (seller flow step 1).
	sellerOnly
)

// authed is a handler that runs with a signed-in principal.
type authed func(w http.ResponseWriter, r *http.Request, p auth.Principal)

func (a *api) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/healthz", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /v1/sessions", a.createSession)
	mux.Handle("GET /v1/sessions/current", a.guard(anyRole, false, false, a.currentSession))
	mux.Handle("DELETE /v1/sessions/current", a.guard(anyRole, true, false, a.deleteSession))

	mux.Handle("GET /v1/brands", a.guard(anyRole, false, false, a.listBrands))
	mux.Handle("PATCH /v1/brands/{brand_id}", a.guard(sellerOnly, true, false, a.updateBrand))
	mux.Handle("POST /v1/uploads", a.guard(sellerOnly, true, true, a.createUpload))
	mux.Handle("GET /v1/uploads/latest", a.guard(anyRole, false, false, a.getLatestUpload))
	mux.Handle("GET /v1/uploads/{upload_id}", a.guard(anyRole, false, false, a.getUpload))
	mux.Handle("POST /v1/uploads/{upload_id}/images", a.guard(sellerOnly, true, true, a.createImages))
	mux.Handle("GET /v1/products", a.guard(anyRole, false, false, a.listProducts))
	mux.Handle("GET /v1/products/{product_id}", a.guard(anyRole, false, false, a.getProduct))
	mux.Handle("GET /v1/products/{product_id}/image", a.guard(anyRole, false, false, a.getProductImage))
	mux.Handle("POST /v1/products/{product_id}/images", a.guard(sellerOnly, true, true, a.createProductImages))
	mux.Handle("PATCH /v1/products/{product_id}", a.guard(sellerOnly, true, false, a.updateProduct))
	mux.Handle("POST /v1/products/{product_id}/enrich", a.guard(sellerOnly, true, true, a.enrichProduct))
	mux.Handle("GET /v1/row-errors", a.guard(anyRole, false, false, a.listRowErrors))
	mux.Handle("PUT /v1/uploads/{upload_id}/rows/{row_number}", a.guard(sellerOnly, true, false, a.updateRow))
	mux.Handle("DELETE /v1/uploads/{upload_id}/rows/{row_number}", a.guard(sellerOnly, true, false, a.discardRow))
	mux.Handle("DELETE /v1/row-errors", a.guard(sellerOnly, true, false, a.discardRows))
	mux.Handle("PATCH /v1/products/{product_id}/attributes", a.guard(reviewerOnly, true, false, a.updateAttributes))

	mux.Handle("POST /v1/generation-runs", a.guard(anyRole, true, true, a.createRun))
	mux.Handle("GET /v1/generation-runs/{run_id}", a.guard(anyRole, false, false, a.getRun))
	mux.Handle("POST /v1/generation-runs/{run_id}/resume", a.guard(anyRole, true, false, a.resumeRun))

	mux.Handle("GET /v1/listings", a.guard(anyRole, false, false, a.listListings))
	mux.Handle("GET /v1/listings/{listing_id}", a.guard(anyRole, false, false, a.getListing))
	mux.Handle("PATCH /v1/listings/{listing_id}", a.guard(reviewerOnly, true, false, a.updateListing))
	mux.Handle("GET /v1/listings/{listing_id}/regeneration-requests", a.guard(anyRole, false, false, a.listRegenerations))
	mux.Handle("POST /v1/listings/{listing_id}/regeneration-requests", a.guard(reviewerOnly, true, true, a.createRegeneration))
	mux.Handle("POST /v1/approvals", a.guard(reviewerOnly, true, true, a.createApprovals))
	mux.Handle("POST /v1/rule-checks", a.guard(reviewerOnly, true, false, a.createRuleCheck))

	mux.Handle("GET /v1/channels", a.guard(anyRole, false, false, a.listChannels))
	mux.Handle("PATCH /v1/channels/{channel}", a.guard(reviewerOnly, true, false, a.updateChannelRules))
	mux.Handle("POST /v1/exports", a.guard(reviewerOnly, true, true, a.createExport))
	// A seller sees and downloads only exports a reviewer sent them (ADR-0012).
	mux.Handle("GET /v1/exports", a.guard(anyRole, false, false, a.listExports))
	mux.Handle("GET /v1/exports/{export_id}", a.guard(anyRole, false, false, a.getExport))
	mux.Handle("POST /v1/exports/{export_id}/send", a.guard(reviewerOnly, true, false, a.sendExport))
	mux.Handle("GET /v1/exports/{export_id}/files/{channel}", a.guard(anyRole, false, false, a.getExportFile))
	mux.Handle("GET /v1/budget", a.guard(anyRole, false, false, a.getBudget))
}

// guard resolves the session (401), checks the CSRF token on writes (403
// csrf_failed, T-08), the role (403 forbidden_role, T-22) and, for creates,
// the Idempotency-Key.
func (a *api) guard(acc access, write, idem bool, h authed) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(SessionCookie)
		if err != nil {
			apiError(w, r, http.StatusUnauthorized, "unauthorized", "Sign in to continue.")
			return
		}
		p, ok, err := a.Auth.Resolve(r.Context(), c.Value)
		if err != nil {
			a.fail(w, r, err)
			return
		}
		if !ok {
			apiError(w, r, http.StatusUnauthorized, "unauthorized", "Your session has ended. Sign in again.")
			return
		}
		if write && !auth.CheckCSRF(p, r.Header.Get("X-CSRF-Token")) {
			apiError(w, r, http.StatusForbidden, "csrf_failed", "The request was refused because its CSRF token is missing or wrong. Reload the page.")
			return
		}
		if acc == reviewerOnly && p.Role != auth.RoleReviewer {
			apiError(w, r, http.StatusForbidden, "forbidden_role", "Only reviewers can do this.")
			return
		}
		if acc == sellerOnly && p.Role != auth.RoleSeller {
			apiError(w, r, http.StatusForbidden, "forbidden_role", "Only sellers can do this.")
			return
		}
		if idem {
			a.idem.serve(w, r, p.UserID, func(w http.ResponseWriter, r *http.Request) { h(w, r, p) })
			return
		}
		h(w, r, p)
	})
}

func (a *api) setSessionCookie(w http.ResponseWriter, value string, expires time.Time) {
	// Secure is off only for a plain-HTTP local run (SECURE_COOKIES=false).
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // US-00-011: T-07, Secure comes from config; HttpOnly and SameSite are set.
		Name: SessionCookie, Value: value, Path: "/", Expires: expires,
		HttpOnly: true, Secure: a.SecureCookies, SameSite: http.SameSiteLaxMode,
	})
}

func sessionBody(p auth.Principal, csrf string) map[string]any {
	return map[string]any{
		"user":       map[string]string{"id": idString(p.UserID), "email": p.Email, "role": string(p.Role)},
		"csrf_token": csrf,
		"expires_at": ts(p.ExpiresAt),
	}
}

// clientIP is the address the sign-in limit counts. Behind the reverse proxy
// (ADR-0002) it is the last X-Forwarded-For entry, the one the proxy wrote.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[len(parts)-1])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (a *api) createSession(w http.ResponseWriter, r *http.Request) {
	if ok, wait := a.SignInLimiter.Allow(clientIP(r)); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
		apiError(w, r, http.StatusTooManyRequests, "rate_limited", "Too many sign-in attempts. Wait and try again.")
		return
	}
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if in.Email == "" || len(in.Email) > 254 || in.Password == "" || len(in.Password) > 200 {
		badRequest(w, r, "email", "and password are required")
		return
	}
	p, csrf, err := a.Auth.Login(r.Context(), in.Email, in.Password)
	if errors.Is(err, auth.ErrBadCredentials) {
		a.log.WarnContext(r.Context(), "sign-in failed", "ip", clientIP(r))
		apiError(w, r, http.StatusUnauthorized, "unauthorized", "The email or password is wrong.")
		return
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.log.InfoContext(r.Context(), "sign-in", "user_id", p.UserID, "role", p.Role)
	a.setSessionCookie(w, p.SessionToken, p.ExpiresAt)
	WriteJSON(w, http.StatusOK, sessionBody(p, csrf))
}

func (a *api) currentSession(w http.ResponseWriter, _ *http.Request, p auth.Principal) {
	WriteJSON(w, http.StatusOK, sessionBody(p, auth.CSRFFor(p.SessionToken)))
}

func (a *api) deleteSession(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if err := a.Auth.Logout(r.Context(), p.SessionToken); err != nil {
		a.fail(w, r, err)
		return
	}
	a.setSessionCookie(w, "", time.Unix(0, 0))
	w.WriteHeader(http.StatusNoContent)
}

// idempotency replays the first response to a create for 24 hours when the
// same user sends the same Idempotency-Key again; a different body is 409.
// One API process serves the demo (ADR-0002), so memory is enough.
type idempotency struct {
	mu      sync.Mutex
	entries map[string]*idemEntry
	now     func() time.Time
}

type idemEntry struct {
	fingerprint string
	done        bool
	at          time.Time
	status      int
	header      http.Header
	body        []byte
}

const idemTTL = 24 * time.Hour

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func newIdempotency(now func() time.Time) *idempotency {
	return &idempotency{entries: map[string]*idemEntry{}, now: now}
}

// fingerprint hashes a JSON body; a multipart body is fingerprinted by its
// length and type, since reading it twice would hold every upload in memory.
func fingerprint(w http.ResponseWriter, r *http.Request) (string, error) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		return r.URL.Path + "|" + strconv.FormatInt(r.ContentLength, 10) + "|" + r.Header.Get("Content-Type"), nil
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	sum := sha256.Sum256(append([]byte(r.URL.Path+"|"), body...))
	return hex.EncodeToString(sum[:]), nil
}

func (s *idempotency) serve(w http.ResponseWriter, r *http.Request, userID int64, h http.HandlerFunc) {
	key := r.Header.Get("Idempotency-Key")
	if !uuidRe.MatchString(key) {
		badRequest(w, r, "Idempotency-Key", "must be a UUID")
		return
	}
	fp, err := fingerprint(w, r)
	if err != nil {
		badRequest(w, r, "body", "could not be read")
		return
	}
	id := idString(userID) + "|" + key
	s.mu.Lock()
	now := s.now()
	for k, e := range s.entries {
		if e.done && now.Sub(e.at) > idemTTL {
			delete(s.entries, k)
		}
	}
	e, seen := s.entries[id]
	switch {
	case seen && e.fingerprint != fp:
		s.mu.Unlock()
		apiError(w, r, http.StatusConflict, "idempotency_conflict", "This Idempotency-Key was used with a different request.")
		return
	case seen && !e.done:
		s.mu.Unlock()
		apiError(w, r, http.StatusConflict, "idempotency_conflict", "The first request with this Idempotency-Key is still running.")
		return
	case seen:
		s.mu.Unlock()
		for k, v := range e.header {
			w.Header()[k] = v
		}
		w.Header().Set("Idempotent-Replayed", "true")
		w.WriteHeader(e.status)
		_, _ = w.Write(e.body) // the client has gone if this fails
		return
	}
	entry := &idemEntry{fingerprint: fp}
	s.entries[id] = entry
	s.mu.Unlock()

	rec := &recorder{ResponseWriter: w, status: http.StatusOK}
	h(rec, r)

	s.mu.Lock()
	defer s.mu.Unlock()
	if rec.status >= 500 {
		delete(s.entries, id) // a server failure may be retried with the same key
		return
	}
	entry.done, entry.at, entry.status, entry.body = true, s.now(), rec.status, rec.buf.Bytes()
	entry.header = w.Header().Clone()
}

// recorder passes a response through and keeps a copy for replay.
type recorder struct {
	http.ResponseWriter
	status int
	buf    bytes.Buffer
}

func (r *recorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *recorder) Write(b []byte) (int, error) {
	_, _ = r.buf.Write(b) // a bytes.Buffer write never returns an error; it panics on out-of-memory
	n, err := r.ResponseWriter.Write(b)
	if err != nil {
		return n, fmt.Errorf("write response: %w", err)
	}
	return n, nil
}
