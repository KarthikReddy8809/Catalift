//go:build integration

package httpapi_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/KarthikReddy8809/catalift/server/internal/ai"
	"github.com/KarthikReddy8809/catalift/server/internal/auth"
	"github.com/KarthikReddy8809/catalift/server/internal/catalogue"
	"github.com/KarthikReddy8809/catalift/server/internal/channels"
	"github.com/KarthikReddy8809/catalift/server/internal/exports"
	"github.com/KarthikReddy8809/catalift/server/internal/generation"
	"github.com/KarthikReddy8809/catalift/server/internal/httpapi"
	"github.com/KarthikReddy8809/catalift/server/internal/listings"
	"github.com/KarthikReddy8809/catalift/server/internal/store"
	"github.com/KarthikReddy8809/catalift/server/internal/testdb"
	"github.com/KarthikReddy8809/catalift/server/internal/worker"
)

const testPassword = "correct-horse-battery"

type env struct {
	t    *testing.T
	pool *pgxpool.Pool
	srv  *httptest.Server
	wk   *worker.Worker
}

func newEnv(t *testing.T) *env {
	t.Helper()
	pool := testdb.New(t)
	set, err := channels.LoadDir("../../config/channels")
	if err != nil {
		t.Fatalf("channels: %v", err)
	}
	q := store.New(pool)
	for email, role := range map[string]store.UserRole{"seller@example.com": store.UserRoleSeller, "reviewer@example.com": store.UserRoleReviewer} {
		hash, err := auth.HashPassword(testPassword)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := q.CreateUser(context.Background(), store.CreateUserParams{Email: email, Role: role, PasswordHash: hash}); err != nil {
			t.Fatalf("create user: %v", err)
		}
	}
	dataDir := t.TempDir()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	deps := &httpapi.Deps{
		Auth: auth.NewService(q), SignInLimiter: auth.NewLimiter(10, 15*time.Minute, time.Now),
		Catalogue: catalogue.NewService(pool, dataDir), Generation: generation.NewService(pool, set),
		Listings: listings.NewService(pool, set), Exports: exports.NewService(pool, set, dataDir), Channels: set,
	}
	srv := httptest.NewServer(httpapi.NewWithAPI(log, "test", prometheus.NewRegistry(), deps))
	t.Cleanup(srv.Close)
	gw := ai.NewGateway(pool, ai.Local{}, "local")
	return &env{t: t, pool: pool, srv: srv, wk: worker.New(pool, gw, set, log, "test")}
}

type client struct {
	e    *env
	http *http.Client
	csrf string
}

func (e *env) signIn(email string) *client {
	e.t.Helper()
	jar, _ := cookiejar.New(nil) // cookiejar.New never fails without options
	c := &client{e: e, http: &http.Client{Jar: jar}}
	var s struct {
		CSRF string `json:"csrf_token"`
	}
	c.must("POST", "/v1/sessions", map[string]string{"email": email, "password": testPassword}, http.StatusOK, &s)
	c.csrf = s.CSRF
	return c
}

func (c *client) do(method, path string, body any, contentType string) *http.Response {
	c.e.t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case []byte:
		rd = bytes.NewReader(b)
	default:
		raw, _ := json.Marshal(b) // test bodies always marshal
		rd = bytes.NewReader(raw)
		contentType = "application/json"
	}
	req, _ := http.NewRequestWithContext(context.Background(), method, c.e.srv.URL+path, rd)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if method != http.MethodGet {
		req.Header.Set("X-CSRF-Token", c.csrf)
		req.Header.Set("Idempotency-Key", newUUID())
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.e.t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

func (c *client) must(method, path string, body any, want int, out any) {
	c.e.t.Helper()
	resp := c.do(method, path, body, "")
	c.expect(resp, want, out)
}

func (c *client) expect(resp *http.Response, want int, out any) {
	c.e.t.Helper()
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != want {
		c.e.t.Fatalf("%s %s: status %d, want %d: %s", resp.Request.Method, resp.Request.URL.Path, resp.StatusCode, want, raw)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			c.e.t.Fatalf("decode %s: %v", raw, err)
		}
	}
}

func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:]) // a weak key only risks a test collision
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func multipartBody(field string, files map[string][]byte) ([]byte, string) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for name, data := range files {
		w, _ := mw.CreateFormFile(field, name) // writes to memory
		_, _ = w.Write(data)
	}
	_ = mw.Close()
	return buf.Bytes(), mw.FormDataContentType()
}

func navyPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for y := range 40 {
		for x := range 40 {
			img.Set(x, y, color.RGBA{R: 22, G: 32, B: 85, A: 255})
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}

func (e *env) drainJobs() {
	e.t.Helper()
	for range 200 {
		ran, err := e.wk.RunOne(context.Background())
		if err != nil {
			e.t.Fatalf("worker: %v", err)
		}
		if !ran {
			return
		}
	}
	e.t.Fatal("jobs did not drain in 200 runs")
}

type listingPage struct {
	Data []struct {
		ID         string `json:"id"`
		SKU        string `json:"sku"`
		Channel    string `json:"channel"`
		Status     string `json:"status"`
		Version    int32  `json:"version"`
		Title      string `json:"title"`
		RuleStatus string `json:"rule_status"`
		Approved   bool   `json:"approved"`
		Attributes struct {
			Colour string `json:"colour"`
		} `json:"attributes"`
	} `json:"data"`
}

// uploadAndGenerate loads two products with photos and runs every job.
func (e *env) uploadAndGenerate(seller *client) listingPage {
	e.t.Helper()
	csv := "sku,category,brand,price\nKU-101,kurta,Indigo Loom,149900\nKU-102,kurta,Indigo Loom,99900\nKU-101,kurta,Indigo Loom,1\n"
	body, ctype := multipartBody("file", map[string][]byte{"products.csv": []byte(csv)})
	var up struct {
		ID           string `json:"id"`
		RowsAccepted int    `json:"rows_accepted"`
		RowsRejected int    `json:"rows_rejected"`
	}
	seller.expect(seller.do("POST", "/v1/uploads", body, ctype), http.StatusCreated, &up)
	if up.RowsAccepted != 2 || up.RowsRejected != 1 {
		e.t.Fatalf("upload counts: %+v", up)
	}
	body, ctype = multipartBody("files", map[string][]byte{"KU-101_front.png": navyPNG(), "KU-102-1.png": navyPNG(), "XX-9.png": navyPNG()})
	var imgs struct {
		Attached       []any    `json:"attached"`
		UnmatchedFiles []string `json:"unmatched_files"`
	}
	seller.expect(seller.do("POST", "/v1/uploads/"+up.ID+"/images", body, ctype), http.StatusCreated, &imgs)
	if len(imgs.Attached) != 2 || len(imgs.UnmatchedFiles) != 1 {
		e.t.Fatalf("images: %+v", imgs)
	}
	seller.must("POST", "/v1/generation-runs", map[string]any{"neutral_voice_confirmed": true, "upload_id": up.ID}, http.StatusCreated, nil)
	e.drainJobs()
	var page listingPage
	seller.must("GET", "/v1/listings", nil, http.StatusOK, &page)
	return page
}

func TestUploadGenerateReviewExport(t *testing.T) {
	e := newEnv(t)
	seller := e.signIn("seller@example.com")
	reviewer := e.signIn("reviewer@example.com")

	page := e.uploadAndGenerate(seller)

	if len(page.Data) != 4 {
		t.Fatalf("want 2 products x 2 channels, got %d", len(page.Data))
	}
	for _, l := range page.Data {
		if l.Status != "generated" || l.Attributes.Colour != "navy" || l.RuleStatus == "unchecked" {
			t.Fatalf("listing not generated and checked: %+v", l)
		}
	}
	var items []map[string]any
	for _, l := range page.Data {
		items = append(items, map[string]any{"listing_id": l.ID, "version": l.Version})
	}
	var res struct {
		Approved []any `json:"approved"`
		Skipped  []struct {
			Reason string `json:"reason"`
		} `json:"skipped"`
	}
	reviewer.must("POST", "/v1/approvals", map[string]any{"items": items}, http.StatusCreated, &res)
	if len(res.Approved) == 0 {
		t.Fatalf("nothing approved: %+v", res)
	}
	var exp struct {
		Files []struct {
			Channel     string `json:"channel"`
			DownloadURL string `json:"download_url"`
		} `json:"files"`
	}
	reviewer.must("POST", "/v1/exports", nil, http.StatusCreated, &exp)
	if len(exp.Files) == 0 {
		t.Fatal("export wrote no files")
	}
	resp := reviewer.do("GET", exp.Files[0].DownloadURL, nil, "")
	defer func() { _ = resp.Body.Close() }()
	csvBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(string(csvBody), "sku,") {
		t.Fatalf("download: %d %s", resp.StatusCode, csvBody)
	}
}

func TestEditAfterViewSkipsApprovalAsVersionChanged(t *testing.T) {
	e := newEnv(t)
	seller := e.signIn("seller@example.com")
	reviewer := e.signIn("reviewer@example.com")
	l := e.uploadAndGenerate(seller).Data[0]
	reviewer.must("PATCH", "/v1/listings/"+l.ID, map[string]any{"version": l.Version, "description": "Edited by another reviewer."}, http.StatusOK, nil)

	var res struct {
		Skipped []struct {
			Reason string `json:"reason"`
		} `json:"skipped"`
	}
	reviewer.must("POST", "/v1/approvals", map[string]any{"items": []map[string]any{{"listing_id": l.ID, "version": l.Version}}}, http.StatusCreated, &res)

	if len(res.Skipped) != 1 || res.Skipped[0].Reason != "version_changed" {
		t.Fatalf("skipped: %+v", res.Skipped)
	}
}

func TestStaleEditIsAVersionConflict(t *testing.T) {
	e := newEnv(t)
	seller := e.signIn("seller@example.com")
	reviewer := e.signIn("reviewer@example.com")
	l := e.uploadAndGenerate(seller).Data[0]
	reviewer.must("PATCH", "/v1/listings/"+l.ID, map[string]any{"version": l.Version, "description": "First edit."}, http.StatusOK, nil)

	resp := reviewer.do("PATCH", "/v1/listings/"+l.ID, map[string]any{"version": l.Version, "description": "Second edit."}, "")

	reviewer.expect(resp, http.StatusConflict, nil)
}

func TestSellerCannotApprove(t *testing.T) {
	e := newEnv(t)
	seller := e.signIn("seller@example.com")
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}

	seller.must("POST", "/v1/approvals", map[string]any{"items": []map[string]any{{"listing_id": "1", "version": 1}}}, http.StatusForbidden, &body)

	if body.Error.Code != "forbidden_role" {
		t.Fatalf("code = %q", body.Error.Code)
	}
}

func TestWriteWithoutCSRFTokenIsRefused(t *testing.T) {
	e := newEnv(t)
	reviewer := e.signIn("reviewer@example.com")
	reviewer.csrf = ""
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}

	reviewer.must("POST", "/v1/exports", nil, http.StatusForbidden, &body)

	if body.Error.Code != "csrf_failed" {
		t.Fatalf("code = %q", body.Error.Code)
	}
}

func TestCorrectingAttributesClearsApprovalAndRegenerates(t *testing.T) {
	e := newEnv(t)
	seller := e.signIn("seller@example.com")
	reviewer := e.signIn("reviewer@example.com")
	page := e.uploadAndGenerate(seller)
	var items []map[string]any
	for _, l := range page.Data {
		items = append(items, map[string]any{"listing_id": l.ID, "version": l.Version})
	}
	reviewer.must("POST", "/v1/approvals", map[string]any{"items": items}, http.StatusCreated, nil)
	var products struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				Revision int32 `json:"revision"`
			} `json:"attributes"`
		} `json:"data"`
	}
	seller.must("GET", "/v1/products", nil, http.StatusOK, &products)
	p := products.Data[0]

	reviewer.must("PATCH", "/v1/products/"+p.ID+"/attributes", map[string]any{"revision": p.Attributes.Revision, "colour": "maroon"}, http.StatusOK, nil)

	var after listingPage
	reviewer.must("GET", "/v1/listings?filter[approved]=true", nil, http.StatusOK, &after)
	for _, l := range after.Data {
		if l.SKU == "KU-101" {
			t.Fatalf("listing of the corrected product is still approved: %+v", l)
		}
	}
}

func TestBudgetBlockStopsGeneration(t *testing.T) {
	e := newEnv(t)
	seller := e.signIn("seller@example.com")
	if _, err := e.pool.Exec(context.Background(), "UPDATE budget SET limit_micro_usd = 1 WHERE id = 1"); err != nil {
		t.Fatal(err)
	}

	page := e.uploadAndGenerate(seller)

	var products struct {
		Data []struct {
			Attributes struct {
				DetectionStatus string `json:"detection_status"`
			} `json:"attributes"`
		} `json:"data"`
	}
	seller.must("GET", "/v1/products", nil, http.StatusOK, &products)
	for _, p := range products.Data {
		if p.Attributes.DetectionStatus != "stopped_budget" {
			t.Fatalf("detection = %q, want stopped_budget", p.Attributes.DetectionStatus)
		}
	}
	for _, l := range page.Data {
		if l.Status != "stopped_budget" {
			t.Fatalf("listing status = %q, want stopped_budget", l.Status)
		}
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	seller.must("POST", "/v1/generation-runs", map[string]any{"neutral_voice_confirmed": true}, http.StatusUnprocessableEntity, &body)
	if body.Error.Code != "budget_blocked" {
		t.Fatalf("code = %q", body.Error.Code)
	}
}

type regenPage struct {
	Data []struct {
		Status string `json:"status"`
	} `json:"data"`
}

func TestRegenerationAppliesToAnUnchangedField(t *testing.T) {
	e := newEnv(t)
	seller := e.signIn("seller@example.com")
	reviewer := e.signIn("reviewer@example.com")
	l := e.uploadAndGenerate(seller).Data[0]
	reviewer.must("POST", "/v1/listings/"+l.ID+"/regeneration-requests", map[string]any{"field": "title", "instruction": "Make it shorter."}, http.StatusCreated, nil)

	e.drainJobs()

	var got regenPage
	reviewer.must("GET", "/v1/listings/"+l.ID+"/regeneration-requests", nil, http.StatusOK, &got)
	if len(got.Data) != 1 || got.Data[0].Status != "applied" {
		t.Fatalf("regenerations: %+v", got.Data)
	}
}

func TestRegenerationIsSupersededByAnEditToTheSameField(t *testing.T) {
	e := newEnv(t)
	seller := e.signIn("seller@example.com")
	reviewer := e.signIn("reviewer@example.com")
	l := e.uploadAndGenerate(seller).Data[0]
	reviewer.must("POST", "/v1/listings/"+l.ID+"/regeneration-requests", map[string]any{"field": "title", "instruction": "Make it shorter."}, http.StatusCreated, nil)
	reviewer.must("PATCH", "/v1/listings/"+l.ID, map[string]any{"version": l.Version, "title": "Navy Kurta"}, http.StatusOK, nil)

	e.drainJobs()

	var got regenPage
	reviewer.must("GET", "/v1/listings/"+l.ID+"/regeneration-requests", nil, http.StatusOK, &got)
	if len(got.Data) != 1 || got.Data[0].Status != "superseded" {
		t.Fatalf("regenerations: %+v", got.Data)
	}
}
