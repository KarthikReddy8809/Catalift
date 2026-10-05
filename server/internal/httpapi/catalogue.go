package httpapi

import (
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/KarthikReddy8809/catalift/internal/auth"
	"github.com/KarthikReddy8809/catalift/internal/catalogue"
	"github.com/KarthikReddy8809/catalift/internal/store"
)

// Upload caps (T-11, T-16): the whole multipart body, and photos per request.
const (
	maxCSVRequest    = catalogue.MaxCSVBytes + 64<<10
	maxImagesRequest = 20*catalogue.MaxImageBytes + 1<<20
	maxImagesPerCall = 20
)

func brandJSON(b store.Brand) map[string]any {
	var note any
	if b.VoiceNote.Valid {
		note = b.VoiceNote.String
	}
	return map[string]any{"id": idString(b.ID), "name": b.Name, "voice_note": note,
		"created_at": ts(b.CreatedAt.Time), "updated_at": ts(b.UpdatedAt.Time)}
}

func (a *api) listBrands(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	limit, key, ok := pageArgs(w, r)
	if !ok {
		return
	}
	rows, err := a.Catalogue.Brands(r.Context(), keyAt(key, 0), limit+1)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	more := len(rows) > int(limit)
	if more {
		rows = rows[:limit]
	}
	data := make([]map[string]any, len(rows))
	next := ""
	for i, b := range rows {
		data[i] = brandJSON(b)
		next = cursorOf(b.Name)
	}
	WriteJSON(w, http.StatusOK, map[string]any{"data": data, "page": page(more, next)})
}

func (a *api) updateBrand(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	id, ok := pathID(w, r, "brand_id")
	if !ok {
		return
	}
	var in struct {
		VoiceNote *string `json:"voice_note"`
	}
	if !decode(w, r, &in) || !requireLen(w, r, "voice_note", in.VoiceNote, 2000) {
		return
	}
	if in.VoiceNote != nil {
		t := strings.TrimSpace(*in.VoiceNote)
		in.VoiceNote = &t
		if t == "" {
			in.VoiceNote = nil
		}
	}
	b, err := a.Catalogue.SetVoiceNote(r.Context(), id, in.VoiceNote)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, brandJSON(b))
}

func (a *api) writeUpload(w http.ResponseWriter, r *http.Request, status int, id int64) {
	u, errs, err := a.Catalogue.Upload(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	rowErrs := make([]map[string]any, len(errs))
	for i, e := range errs {
		var sku any
		if e.Sku.Valid {
			sku = e.Sku.String
		}
		rowErrs[i] = map[string]any{"row_number": e.RowNumber, "sku": sku, "reason": e.Reason}
	}
	WriteJSON(w, status, map[string]any{
		"id": idString(u.ID), "file_name": u.FileName, "rows_total": u.RowsTotal, "rows_accepted": u.RowsAccepted,
		"rows_rejected": u.RowsRejected, "row_errors": rowErrs, "ai_cost_micro_usd": u.AiCostMicroUsd, "created_at": ts(u.CreatedAt.Time),
	})
}

func (a *api) createUpload(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCSVRequest)
	mr, err := r.MultipartReader()
	if err != nil {
		badRequest(w, r, "body", "must be multipart/form-data with a file part")
		return
	}
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			badRequest(w, r, "file", "is required")
			return
		}
		if err != nil {
			a.fail(w, r, err)
			return
		}
		if part.FormName() != "file" {
			continue
		}
		name := part.FileName()
		ctype := part.Header.Get("Content-Type")
		if !strings.EqualFold(filepath.Ext(name), ".csv") && !strings.HasPrefix(ctype, "text/csv") {
			apiError(w, r, http.StatusUnsupportedMediaType, "unsupported_media_type", "The product list must be a .csv file.")
			return
		}
		id, err := a.Catalogue.UploadCSV(r.Context(), p.UserID, name, part)
		if err != nil {
			a.fail(w, r, err)
			return
		}
		a.writeUpload(w, r, http.StatusCreated, id)
		return
	}
}

func (a *api) getUpload(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	if id, ok := pathID(w, r, "upload_id"); ok {
		a.writeUpload(w, r, http.StatusOK, id)
	}
}

func (a *api) createImages(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	id, ok := pathID(w, r, "upload_id")
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxImagesRequest)
	mr, err := r.MultipartReader()
	if err != nil {
		badRequest(w, r, "body", "must be multipart/form-data with files parts")
		return
	}
	var files []catalogue.ImageFile
	var tooLarge []string
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			a.fail(w, r, err)
			return
		}
		if part.FormName() != "files" {
			continue
		}
		if len(files)+len(tooLarge) >= maxImagesPerCall {
			badRequest(w, r, "files", "may hold at most "+strconv.Itoa(maxImagesPerCall)+" photos per request")
			return
		}
		data, err := io.ReadAll(io.LimitReader(part, catalogue.MaxImageBytes+1))
		if err != nil {
			a.fail(w, r, err)
			return
		}
		if len(data) > catalogue.MaxImageBytes {
			tooLarge = append(tooLarge, part.FileName())
			continue
		}
		files = append(files, catalogue.ImageFile{Name: part.FileName(), Data: data})
	}
	if len(files)+len(tooLarge) == 0 {
		badRequest(w, r, "files", "must hold at least one photo")
		return
	}
	res, err := a.Catalogue.AttachImages(r.Context(), id, files)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	attached := make([]map[string]any, len(res.Attached))
	for i, at := range res.Attached {
		attached[i] = map[string]any{"file_name": at.FileName, "sku": at.SKU, "position": at.Position}
	}
	rejected := make([]map[string]any, 0, len(res.Rejected)+len(tooLarge))
	for _, n := range tooLarge {
		rejected = append(rejected, map[string]any{"file_name": n, "reason": "larger than 10 MB"})
	}
	for _, rj := range res.Rejected {
		rejected = append(rejected, map[string]any{"file_name": rj.FileName, "reason": rj.Reason})
	}
	WriteJSON(w, http.StatusCreated, map[string]any{
		"attached": attached, "unmatched_files": nonNil(res.UnmatchedFiles), "rejected_files": rejected,
		"products_missing_image": nonNil(res.ProductsMissingImage),
	})
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func textOrNil(t pgtype.Text) any {
	if t.Valid {
		return t.String
	}
	return nil
}

func productJSON(p *store.ListProductsRow) map[string]any {
	return map[string]any{
		"id": idString(p.ID), "sku": p.Sku, "brand": map[string]string{"id": idString(p.BrandID), "name": p.BrandName},
		"category": p.Category, "price_minor": p.PriceMinor, "currency": p.Currency, "image_count": p.ImageCount,
		"attributes": map[string]any{
			"detection_status": p.DetectionStatus, "revision": p.Revision,
			"colour": textOrNil(p.Colour), "pattern": textOrNil(p.Pattern), "sleeve": textOrNil(p.Sleeve),
			"neckline": textOrNil(p.Neckline), "fit": textOrNil(p.Fit), "detection_error": textOrNil(p.DetectionError),
		},
		"ai_cost_micro_usd": p.AiCostMicroUsd, "created_at": ts(p.CreatedAt.Time),
	}
}

func (a *api) listProducts(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	limit, key, ok := pageArgs(w, r)
	if !ok {
		return
	}
	params := store.ListProductsParams{AfterSku: keyAt(key, 0), PageSize: limit + 1}
	q := r.URL.Query()
	if v := q.Get("filter[upload_id]"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			badRequest(w, r, "filter[upload_id]", "must be a positive integer")
			return
		}
		params.UploadID = pgtype.Int8{Int64: id, Valid: true}
	}
	if v := q.Get("filter[detection_status]"); v != "" {
		ds := store.DetectionStatus(v)
		if !oneOf(v, "pending", "done", "failed", "stopped_budget") {
			badRequest(w, r, "filter[detection_status]", "must be pending, done, failed or stopped_budget")
			return
		}
		params.DetectionStatus = store.NullDetectionStatus{DetectionStatus: ds, Valid: true}
	}
	rows, err := a.Catalogue.Products(r.Context(), params)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	more := len(rows) > int(limit)
	if more {
		rows = rows[:limit]
	}
	data := make([]map[string]any, len(rows))
	next := ""
	for i := range rows {
		data[i] = productJSON(&rows[i])
		next = cursorOf(strings.ToLower(rows[i].Sku))
	}
	WriteJSON(w, http.StatusOK, map[string]any{"data": data, "page": page(more, next)})
}

func (a *api) getProduct(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	id, ok := pathID(w, r, "product_id")
	if !ok {
		return
	}
	p, err := a.Catalogue.Product(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, productJSON(&p))
}

func oneOf(v string, options ...string) bool {
	for _, o := range options {
		if v == o {
			return true
		}
	}
	return false
}
