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
	files, err := channels.LoadDir("../../config/channels")
	set := channels.NewRegistry(files)
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
		Catalogue: catalogue.NewService(pool, dataDir, catalogue.NewCategories("kurta", "saree")), Generation: generation.NewService(pool, set),
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
	csv := "sku,category,brand,price\nKU-101,kurta,Indigo Loom,149900\nKU-102,kurta,Indigo Loom,99900\nKU-101,kurta,Indigo Loom,1\nJN-1,jeans,Indigo Loom,999\n"
	body, ctype := multipartBody("file", map[string][]byte{"products.csv": []byte(csv)})
	var up struct {
		ID           string `json:"id"`
		RowsAccepted int    `json:"rows_accepted"`
		RowsRejected int    `json:"rows_rejected"`
	}
	seller.expect(seller.do("POST", "/v1/uploads", body, ctype), http.StatusCreated, &up)
	if up.RowsAccepted != 2 || up.RowsRejected != 2 {
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
	var prods productPage
	seller.must("GET", "/v1/products", nil, http.StatusOK, &prods)
	for _, p := range prods.Data {
		if p.Status != "budget_exhausted" {
			t.Fatalf("product status = %q, want budget_exhausted", p.Status)
		}
	}
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

type productPage struct {
	Data []struct {
		ID         string `json:"id"`
		SKU        string `json:"sku"`
		Status     string `json:"status"`
		Attributes struct {
			Confidence *float64 `json:"confidence"`
		} `json:"attributes"`
	} `json:"data"`
}

func TestUnknownCategoryIsRejectedBeforeAnyAICall(t *testing.T) {
	e := newEnv(t)
	seller := e.signIn("seller@example.com")
	body, ctype := multipartBody("file", map[string][]byte{"p.csv": []byte("sku,category,brand,price\nJN-1,jeans,Indigo Loom,999\n")})
	var up struct {
		RowErrors []struct {
			Reason string `json:"reason"`
		} `json:"row_errors"`
		RowsAccepted int `json:"rows_accepted"`
	}

	seller.expect(seller.do("POST", "/v1/uploads", body, ctype), http.StatusCreated, &up)

	if up.RowsAccepted != 0 || len(up.RowErrors) != 1 || !strings.Contains(up.RowErrors[0].Reason, "kurta, saree") {
		t.Fatalf("upload: %+v", up)
	}
}

func TestEnrichmentMakesOneAICallPerProductAndWritesEveryChannel(t *testing.T) {
	e := newEnv(t)
	seller := e.signIn("seller@example.com")

	page := e.uploadAndGenerate(seller)

	var calls, products int
	if err := e.pool.QueryRow(context.Background(),
		"SELECT count(*), count(DISTINCT product_id) FROM ai_calls WHERE prompt_template_id = 'enrich-v1'").Scan(&calls, &products); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || products != 2 || len(page.Data) != 4 {
		t.Fatalf("calls=%d products=%d listings=%d, want 2 calls for 2 products and 4 listings", calls, products, len(page.Data))
	}
	var prods productPage
	seller.must("GET", "/v1/products", nil, http.StatusOK, &prods)
	for _, p := range prods.Data {
		if p.Status != "ready_for_review" || p.Attributes.Confidence == nil {
			t.Fatalf("product %s: status=%s confidence=%v", p.SKU, p.Status, p.Attributes.Confidence)
		}
	}
}

func TestProductStatusMovesFromUploadedToApproved(t *testing.T) {
	e := newEnv(t)
	seller := e.signIn("seller@example.com")
	reviewer := e.signIn("reviewer@example.com")
	body, ctype := multipartBody("file", map[string][]byte{"p.csv": []byte("sku,category,brand,price\nKU-201,kurta,Indigo Loom,999\n")})
	var up struct {
		ID string `json:"id"`
	}
	seller.expect(seller.do("POST", "/v1/uploads", body, ctype), http.StatusCreated, &up)
	var prods productPage
	seller.must("GET", "/v1/products", nil, http.StatusOK, &prods)
	if prods.Data[0].Status != "needs_photo" {
		t.Fatalf("before photo: %s", prods.Data[0].Status)
	}
	body, ctype = multipartBody("files", map[string][]byte{"KU-201.png": navyPNG()})
	seller.expect(seller.do("POST", "/v1/uploads/"+up.ID+"/images", body, ctype), http.StatusCreated, nil)
	seller.must("GET", "/v1/products", nil, http.StatusOK, &prods)
	if prods.Data[0].Status != "uploaded" {
		t.Fatalf("before run: %s", prods.Data[0].Status)
	}
	seller.must("POST", "/v1/generation-runs", map[string]any{"neutral_voice_confirmed": true}, http.StatusCreated, nil)
	seller.must("GET", "/v1/products", nil, http.StatusOK, &prods)
	if prods.Data[0].Status != "enriching" {
		t.Fatalf("queued: %s", prods.Data[0].Status)
	}
	e.drainJobs()
	var page listingPage
	reviewer.must("GET", "/v1/listings", nil, http.StatusOK, &page)
	var items []map[string]any
	for _, l := range page.Data {
		if l.RuleStatus == "passing" {
			items = append(items, map[string]any{"listing_id": l.ID, "version": l.Version})
		}
	}

	reviewer.must("POST", "/v1/approvals", map[string]any{"items": items}, http.StatusCreated, nil)

	seller.must("GET", "/v1/products", nil, http.StatusOK, &prods)
	want := "ready_for_review"
	if len(items) == len(page.Data) {
		want = "approved"
	}
	if prods.Data[0].Status != want {
		t.Fatalf("after approval of %d of %d: %s, want %s", len(items), len(page.Data), prods.Data[0].Status, want)
	}
}

func TestProductImageServesTheThumbnail(t *testing.T) {
	e := newEnv(t)
	reviewer := e.signIn("reviewer@example.com")
	e.uploadAndGenerate(e.signIn("seller@example.com"))
	var prods productPage
	reviewer.must("GET", "/v1/products", nil, http.StatusOK, &prods)

	resp := reviewer.do("GET", "/v1/products/"+prods.Data[0].ID+"/image", nil, "")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "image/jpeg" {
		t.Fatalf("image: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
}

func TestRuleCheckReportsADraftWithoutSavingIt(t *testing.T) {
	e := newEnv(t)
	reviewer := e.signIn("reviewer@example.com")
	l := e.uploadAndGenerate(e.signIn("seller@example.com")).Data[0]
	var check struct {
		RuleStatus   string `json:"rule_status"`
		RuleFailures []struct {
			Rule string `json:"rule"`
		} `json:"rule_failures"`
	}

	reviewer.must("POST", "/v1/rule-checks", map[string]any{"listing_id": l.ID, "bullet_1": "Guaranteed to fit."}, http.StatusOK, &check)

	if check.RuleStatus != "failing" || len(check.RuleFailures) == 0 || check.RuleFailures[0].Rule != "banned_word" {
		t.Fatalf("check: %+v", check)
	}
	var after listingPage
	reviewer.must("GET", "/v1/listings", nil, http.StatusOK, &after)
	for _, a := range after.Data {
		if a.ID == l.ID && (a.Version != l.Version || a.RuleStatus != l.RuleStatus) {
			t.Fatalf("the check changed the listing: before %+v after %+v", l, a)
		}
	}
}

func TestReviewerCannotUpload(t *testing.T) {
	e := newEnv(t)
	reviewer := e.signIn("reviewer@example.com")
	body, ctype := multipartBody("file", map[string][]byte{"p.csv": []byte("sku,category,brand,price\nKU-1,kurta,B,9\n")})
	var out struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}

	reviewer.expect(reviewer.do("POST", "/v1/uploads", body, ctype), http.StatusForbidden, &out)

	if out.Error.Code != "forbidden_role" {
		t.Fatalf("code = %q", out.Error.Code)
	}
}

type channelPage struct {
	Data []struct {
		ID                 string   `json:"id"`
		ConfigHash         string   `json:"config_hash"`
		TitleMaxLength     int      `json:"title_max_length"`
		RequiredAttributes []string `json:"required_attributes"`
		BannedWords        []string `json:"banned_words"`
		LastEdit           *struct {
			By string `json:"by"`
		} `json:"last_edit"`
	} `json:"data"`
}

func TestReviewerRuleEditRechecksAndClearsApprovals(t *testing.T) {
	e := newEnv(t)
	reviewer := e.signIn("reviewer@example.com")
	page := e.uploadAndGenerate(e.signIn("seller@example.com"))
	var items []map[string]any
	for _, l := range page.Data {
		if l.Channel == "own_website" && l.RuleStatus == "passing" {
			items = append(items, map[string]any{"listing_id": l.ID, "version": l.Version})
		}
	}
	reviewer.must("POST", "/v1/approvals", map[string]any{"items": items}, http.StatusCreated, nil)
	var chans channelPage
	reviewer.must("GET", "/v1/channels", nil, http.StatusOK, &chans)
	var web = chans.Data[0]
	for _, c := range chans.Data {
		if c.ID == "own_website" {
			web = c
		}
	}
	var res struct {
		Rechecked int `json:"listings_rechecked"`
		Cleared   int `json:"approvals_cleared"`
	}

	// Every stand-in listing says "Made by Indigo Loom."; banning "loom" fails them all.
	reviewer.must("PATCH", "/v1/channels/own_website", map[string]any{
		"config_hash": web.ConfigHash, "title_max_length": web.TitleMaxLength,
		"required_attributes": web.RequiredAttributes, "banned_words": append(web.BannedWords, "loom"),
	}, http.StatusOK, &res)

	if res.Rechecked != 2 || res.Cleared != len(items) || len(items) == 0 {
		t.Fatalf("recheck: %+v, approved before: %d", res, len(items))
	}
	var after listingPage
	reviewer.must("GET", "/v1/listings?filter[channel]=own_website", nil, http.StatusOK, &after)
	for _, l := range after.Data {
		if l.RuleStatus != "failing" || l.Approved {
			t.Fatalf("listing after the edit: %+v", l)
		}
	}
	reviewer.must("GET", "/v1/channels", nil, http.StatusOK, &chans)
	for _, c := range chans.Data {
		if c.ID == "own_website" && (c.LastEdit == nil || c.LastEdit.By != "reviewer@example.com") {
			t.Fatalf("last edit: %+v", c.LastEdit)
		}
	}
}

func TestRuleEditAgainstStaleRulesIsAConflict(t *testing.T) {
	e := newEnv(t)
	reviewer := e.signIn("reviewer@example.com")
	body := map[string]any{"config_hash": "not-the-current-hash", "title_max_length": 100, "required_attributes": []string{}, "banned_words": []string{}}

	reviewer.must("PATCH", "/v1/channels/own_website", body, http.StatusConflict, nil)
}

func TestSellerCannotEditRules(t *testing.T) {
	e := newEnv(t)
	seller := e.signIn("seller@example.com")
	body := map[string]any{"config_hash": "x", "title_max_length": 100, "required_attributes": []string{}, "banned_words": []string{}}

	seller.must("PATCH", "/v1/channels/own_website", body, http.StatusForbidden, nil)
}

type rowErrorPage struct {
	Data []struct {
		UploadID  string  `json:"upload_id"`
		RowNumber int     `json:"row_number"`
		SKU       *string `json:"sku"`
		Category  *string `json:"category"`
		Price     *string `json:"price"`
		Reason    string  `json:"reason"`
	} `json:"data"`
	Categories []string `json:"categories"`
}

type rowFixResult struct {
	Status    string  `json:"status"`
	Reason    *string `json:"reason"`
	ProductID *string `json:"product_id"`
}

func TestSellerFixesARejectedRowAndAddsItsPhoto(t *testing.T) {
	e := newEnv(t)
	seller := e.signIn("seller@example.com")
	csv := "sku,category,brand,price\nKU-301,kurta,Indigo Loom,999\nKU-302,jeans,Indigo Loom,abc\n"
	body, ctype := multipartBody("file", map[string][]byte{"p.csv": []byte(csv)})
	seller.expect(seller.do("POST", "/v1/uploads", body, ctype), http.StatusCreated, nil)
	var open rowErrorPage
	seller.must("GET", "/v1/row-errors", nil, http.StatusOK, &open)
	if len(open.Data) != 1 || open.Data[0].Price == nil || *open.Data[0].Price != "abc" || len(open.Categories) == 0 {
		t.Fatalf("open rows: %+v", open)
	}
	row := open.Data[0]
	path := fmt.Sprintf("/v1/uploads/%s/rows/%d", row.UploadID, row.RowNumber)
	var res rowFixResult

	// Still wrong: the category is unknown, so the row stays open with that reason.
	seller.must("PUT", path, map[string]string{"sku": "KU-302", "category": "jeans", "brand": "Indigo Loom", "price": "1299"}, http.StatusOK, &res)
	if res.Status != "rejected" || res.Reason == nil || !strings.Contains(*res.Reason, "category") {
		t.Fatalf("first fix: %+v", res)
	}
	// A SKU that already exists is refused too.
	seller.must("PUT", path, map[string]string{"sku": "KU-301", "category": "kurta", "brand": "Indigo Loom", "price": "1299"}, http.StatusOK, &res)
	if res.Status != "rejected" || *res.Reason != "SKU already exists" {
		t.Fatalf("duplicate fix: %+v", res)
	}
	seller.must("PUT", path, map[string]string{"sku": "KU-302", "category": "kurta", "brand": "Indigo Loom", "price": "1299"}, http.StatusOK, &res)

	if res.Status != "loaded" || res.ProductID == nil {
		t.Fatalf("fix: %+v", res)
	}
	seller.must("GET", "/v1/row-errors", nil, http.StatusOK, &open)
	if len(open.Data) != 0 {
		t.Fatalf("row still open: %+v", open.Data)
	}
	var prods productPage
	seller.must("GET", "/v1/products/"+*res.ProductID, nil, http.StatusOK, nil)
	body, ctype = multipartBody("files", map[string][]byte{"any-name-at-all.png": navyPNG()})
	seller.expect(seller.do("POST", "/v1/products/"+*res.ProductID+"/images", body, ctype), http.StatusCreated, nil)
	seller.must("GET", "/v1/products", nil, http.StatusOK, &prods)
	for _, p := range prods.Data {
		if p.SKU == "KU-302" && p.Status != "uploaded" {
			t.Fatalf("KU-302 after its photo: %s", p.Status)
		}
	}
}

func TestReviewerCannotFixRows(t *testing.T) {
	e := newEnv(t)
	reviewer := e.signIn("reviewer@example.com")

	reviewer.must("PUT", "/v1/uploads/1/rows/2", map[string]string{"sku": "A", "category": "kurta", "brand": "B", "price": "1"}, http.StatusForbidden, nil)
}

func TestSellerFixesAProductAndRerunsItsVisionCall(t *testing.T) {
	e := newEnv(t)
	seller := e.signIn("seller@example.com")
	ctx := context.Background()
	if _, err := e.pool.Exec(ctx, "UPDATE budget SET limit_micro_usd = 1 WHERE id = 1"); err != nil {
		t.Fatal(err)
	}
	e.uploadAndGenerate(seller) // the budget stops both products before any AI call
	if _, err := e.pool.Exec(ctx, "UPDATE budget SET limit_micro_usd = 8000000, blocked_at = NULL WHERE id = 1"); err != nil {
		t.Fatal(err)
	}
	var prods productPage
	seller.must("GET", "/v1/products", nil, http.StatusOK, &prods)
	p := prods.Data[0]
	var saved struct {
		Status string  `json:"status"`
		Reason *string `json:"reason"`
	}

	seller.must("PATCH", "/v1/products/"+p.ID, map[string]string{"sku": p.SKU, "category": "kurta", "brand": "Indigo Loom", "price": "abc"}, http.StatusOK, &saved)
	if saved.Status != "rejected" || saved.Reason == nil || !strings.Contains(*saved.Reason, "price") {
		t.Fatalf("bad price: %+v", saved)
	}
	seller.must("PATCH", "/v1/products/"+p.ID, map[string]string{"sku": p.SKU + "-B", "category": "saree", "brand": "Indigo Loom", "price": "2499"}, http.StatusOK, &saved)
	if saved.Status != "saved" {
		t.Fatalf("fix: %+v", saved)
	}
	seller.must("POST", "/v1/products/"+p.ID+"/enrich", nil, http.StatusCreated, nil)
	e.drainJobs()

	var calls int
	if err := e.pool.QueryRow(ctx, "SELECT count(*) FROM ai_calls WHERE product_id = $1 AND prompt_template_id = 'enrich-v1'", p.ID).Scan(&calls); err != nil {
		t.Fatal(err)
	}
	seller.must("GET", "/v1/products", nil, http.StatusOK, &prods)
	for _, q := range prods.Data {
		if q.ID == p.ID && (q.SKU != p.SKU+"-B" || q.Status != "ready_for_review") {
			t.Fatalf("after the re-run: %+v", q)
		}
	}
	if calls != 1 {
		t.Fatalf("vision calls for the product = %d, want 1", calls)
	}
}

func TestRerunNeedsAPhoto(t *testing.T) {
	e := newEnv(t)
	seller := e.signIn("seller@example.com")
	body, ctype := multipartBody("file", map[string][]byte{"p.csv": []byte("sku,category,brand,price\nKU-401,kurta,B,999\n")})
	seller.expect(seller.do("POST", "/v1/uploads", body, ctype), http.StatusCreated, nil)
	var prods productPage
	seller.must("GET", "/v1/products", nil, http.StatusOK, &prods)

	seller.must("POST", "/v1/products/"+prods.Data[0].ID+"/enrich", nil, http.StatusUnprocessableEntity, nil)
}

func TestReviewerCannotEditOrRerunProducts(t *testing.T) {
	e := newEnv(t)
	reviewer := e.signIn("reviewer@example.com")

	reviewer.must("PATCH", "/v1/products/1", map[string]string{"sku": "A", "category": "kurta", "brand": "B", "price": "1"}, http.StatusForbidden, nil)
	reviewer.must("POST", "/v1/products/1/enrich", nil, http.StatusForbidden, nil)
}

func TestSellerDiscardsRejectedRows(t *testing.T) {
	e := newEnv(t)
	seller := e.signIn("seller@example.com")
	csv := "sku,category,brand,price\nKU-501,jeans,B,9\nKU-502,jeans,B,9\nKU-503,jeans,B,9\n"
	body, ctype := multipartBody("file", map[string][]byte{"p.csv": []byte(csv)})
	var up struct {
		ID string `json:"id"`
	}
	seller.expect(seller.do("POST", "/v1/uploads", body, ctype), http.StatusCreated, &up)

	seller.must("DELETE", "/v1/uploads/"+up.ID+"/rows/2", nil, http.StatusNoContent, nil)
	var all struct {
		Discarded int `json:"discarded"`
	}
	seller.must("DELETE", "/v1/row-errors", nil, http.StatusOK, &all)

	var open rowErrorPage
	seller.must("GET", "/v1/row-errors", nil, http.StatusOK, &open)
	if all.Discarded != 2 || len(open.Data) != 0 {
		t.Fatalf("discarded %d, still open %d", all.Discarded, len(open.Data))
	}
}

type exportBody struct {
	ID     string  `json:"id"`
	SentAt *string `json:"sent_at"`
	SentBy *string `json:"sent_by"`
	Files  []struct {
		Channel     string `json:"channel"`
		DownloadURL string `json:"download_url"`
	} `json:"files"`
}

func (e *env) approveAllAndExport(reviewer *client) exportBody {
	e.t.Helper()
	page := e.uploadAndGenerate(e.signIn("seller@example.com"))
	var items []map[string]any
	for _, l := range page.Data {
		items = append(items, map[string]any{"listing_id": l.ID, "version": l.Version})
	}
	reviewer.must("POST", "/v1/approvals", map[string]any{"items": items}, http.StatusCreated, nil)
	var exp exportBody
	reviewer.must("POST", "/v1/exports", nil, http.StatusCreated, &exp)
	if len(exp.Files) == 0 {
		e.t.Fatal("export wrote no files")
	}
	return exp
}

func TestSellerGetsOnlyExportsAReviewerSent(t *testing.T) {
	e := newEnv(t)
	reviewer := e.signIn("reviewer@example.com")
	seller := e.signIn("seller@example.com")
	exp := e.approveAllAndExport(reviewer)
	var list struct {
		Data []exportBody `json:"data"`
	}

	seller.must("GET", "/v1/exports", nil, http.StatusOK, &list)
	if len(list.Data) != 0 {
		t.Fatalf("seller saw an unsent export: %+v", list.Data)
	}
	seller.must("GET", "/v1/exports/"+exp.ID, nil, http.StatusNotFound, nil)
	seller.must("GET", exp.Files[0].DownloadURL, nil, http.StatusNotFound, nil)

	var sent exportBody
	reviewer.must("POST", "/v1/exports/"+exp.ID+"/send", nil, http.StatusOK, &sent)
	if sent.SentAt == nil || sent.SentBy == nil || *sent.SentBy != "reviewer@example.com" {
		t.Fatalf("send: %+v", sent)
	}
	seller.must("GET", "/v1/exports", nil, http.StatusOK, &list)
	if len(list.Data) != 1 || list.Data[0].ID != exp.ID {
		t.Fatalf("seller's received exports: %+v", list.Data)
	}
	resp := seller.do("GET", exp.Files[0].DownloadURL, nil, "")
	defer func() { _ = resp.Body.Close() }()
	csvBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(string(csvBody), "sku,") {
		t.Fatalf("seller download: %d %s", resp.StatusCode, csvBody)
	}
}

func TestSendingAnExportTwiceKeepsTheFirstSend(t *testing.T) {
	e := newEnv(t)
	reviewer := e.signIn("reviewer@example.com")
	exp := e.approveAllAndExport(reviewer)
	var first, second exportBody

	reviewer.must("POST", "/v1/exports/"+exp.ID+"/send", nil, http.StatusOK, &first)
	reviewer.must("POST", "/v1/exports/"+exp.ID+"/send", nil, http.StatusOK, &second)

	if first.SentAt == nil || second.SentAt == nil || *first.SentAt != *second.SentAt {
		t.Fatalf("first %v, second %v", first.SentAt, second.SentAt)
	}
}

func TestSellerCannotSendAnExport(t *testing.T) {
	e := newEnv(t)
	seller := e.signIn("seller@example.com")

	seller.must("POST", "/v1/exports/1/send", nil, http.StatusForbidden, nil)
}

func TestSendingAMissingExportIsNotFound(t *testing.T) {
	e := newEnv(t)
	reviewer := e.signIn("reviewer@example.com")

	reviewer.must("POST", "/v1/exports/999999/send", nil, http.StatusNotFound, nil)
}

type brandPage struct {
	Data []struct {
		ID           string   `json:"id"`
		Name         string   `json:"name"`
		VoiceNote    *string  `json:"voice_note"`
		WordsToAvoid []string `json:"words_to_avoid"`
	} `json:"data"`
}

func TestBrandWordsToAvoidFailTheBrandsListings(t *testing.T) {
	e := newEnv(t)
	seller := e.signIn("seller@example.com")
	e.uploadAndGenerate(seller)
	var brands brandPage
	seller.must("GET", "/v1/brands", nil, http.StatusOK, &brands)
	var saved struct {
		VoiceNote    *string  `json:"voice_note"`
		WordsToAvoid []string `json:"words_to_avoid"`
	}

	// Every stand-in listing says "Made by Indigo Loom."; avoiding "loom" fails them all.
	seller.must("PATCH", "/v1/brands/"+brands.Data[0].ID, map[string]any{
		"voice_note": "warm and concise", "words_to_avoid": []string{" loom ", "LOOM", ""},
	}, http.StatusOK, &saved)

	if saved.VoiceNote == nil || *saved.VoiceNote != "warm and concise" || len(saved.WordsToAvoid) != 1 || saved.WordsToAvoid[0] != "loom" {
		t.Fatalf("saved brand: %+v", saved)
	}
	var after listingPage
	seller.must("GET", "/v1/listings", nil, http.StatusOK, &after)
	for _, l := range after.Data {
		if l.RuleStatus != "failing" {
			t.Fatalf("listing not failing after the brand's words changed: %+v", l)
		}
	}
}

func TestBrandWordsToAvoidHaveALimit(t *testing.T) {
	e := newEnv(t)
	seller := e.signIn("seller@example.com")
	e.uploadAndGenerate(seller)
	var brands brandPage
	seller.must("GET", "/v1/brands", nil, http.StatusOK, &brands)

	seller.must("PATCH", "/v1/brands/"+brands.Data[0].ID, map[string]any{
		"voice_note": "warm", "words_to_avoid": []string{strings.Repeat("a", 61)},
	}, http.StatusBadRequest, nil)
}

func TestReviewerCannotEditBrandVoice(t *testing.T) {
	e := newEnv(t)
	reviewer := e.signIn("reviewer@example.com")

	reviewer.must("PATCH", "/v1/brands/1", map[string]any{"voice_note": "loud"}, http.StatusForbidden, nil)
}

// uploadNewThread uploads a second launch (one product, its photo) and
// generates it; it returns the upload id.
func (e *env) uploadNewThread(seller *client) string {
	e.t.Helper()
	body, ctype := multipartBody("file", map[string][]byte{"diwali.csv": []byte("sku,category,brand,price\nNT-1,kurta,New Thread,999\n")})
	var up struct {
		ID string `json:"id"`
	}
	seller.expect(seller.do("POST", "/v1/uploads", body, ctype), http.StatusCreated, &up)
	body, ctype = multipartBody("files", map[string][]byte{"NT-1_front.png": navyPNG()})
	seller.expect(seller.do("POST", "/v1/uploads/"+up.ID+"/images", body, ctype), http.StatusCreated, nil)
	seller.must("POST", "/v1/generation-runs", map[string]any{"neutral_voice_confirmed": true, "upload_id": up.ID}, http.StatusCreated, nil)
	e.drainJobs()
	return up.ID
}

func TestEveryListCanShowOnlyTheLatestUpload(t *testing.T) {
	e := newEnv(t)
	seller := e.signIn("seller@example.com")
	reviewer := e.signIn("reviewer@example.com")
	e.uploadAndGenerate(seller)
	newID := e.uploadNewThread(seller)
	var latest struct {
		ID string `json:"id"`
	}

	seller.must("GET", "/v1/uploads/latest", nil, http.StatusOK, &latest)

	if latest.ID != newID {
		t.Fatalf("latest upload %q, want %q", latest.ID, newID)
	}
	var grid listingPage
	reviewer.must("GET", "/v1/listings?filter[upload_id]="+newID, nil, http.StatusOK, &grid)
	if len(grid.Data) != 2 {
		t.Fatalf("want NT-1 on 2 channels, got %+v", grid.Data)
	}
	var items []map[string]any
	for _, l := range grid.Data {
		if l.SKU != "NT-1" {
			t.Fatalf("an older upload's listing leaked in: %+v", l)
		}
		items = append(items, map[string]any{"listing_id": l.ID, "version": l.Version})
	}
	var brands brandPage
	seller.must("GET", "/v1/brands?filter[upload_id]="+newID, nil, http.StatusOK, &brands)
	if len(brands.Data) != 1 || brands.Data[0].Name != "New Thread" {
		t.Fatalf("brands of the latest upload: %+v", brands.Data)
	}

	// Approve every listing of both uploads; the export still holds only the new one.
	var all listingPage
	reviewer.must("GET", "/v1/listings", nil, http.StatusOK, &all)
	for _, l := range all.Data {
		if l.SKU != "NT-1" {
			items = append(items, map[string]any{"listing_id": l.ID, "version": l.Version})
		}
	}
	reviewer.must("POST", "/v1/approvals", map[string]any{"items": items}, http.StatusCreated, nil)
	var exp struct {
		ID    string `json:"id"`
		Files []struct {
			RowCount int `json:"row_count"`
		} `json:"files"`
	}
	reviewer.must("POST", "/v1/exports", map[string]any{"upload_id": newID}, http.StatusCreated, &exp)
	for _, f := range exp.Files {
		if f.RowCount != 1 {
			t.Fatalf("export of the latest upload holds %d rows, want 1: %+v", f.RowCount, exp)
		}
	}
	var list struct {
		Data []exportBody `json:"data"`
	}
	reviewer.must("GET", "/v1/exports?filter[upload_id]="+newID, nil, http.StatusOK, &list)
	if len(list.Data) != 1 || list.Data[0].ID != exp.ID {
		t.Fatalf("exports of the latest upload: %+v", list.Data)
	}
}

func TestLatestUploadIsNotFoundBeforeAnyUpload(t *testing.T) {
	e := newEnv(t)
	seller := e.signIn("seller@example.com")

	seller.must("GET", "/v1/uploads/latest", nil, http.StatusNotFound, nil)
}
