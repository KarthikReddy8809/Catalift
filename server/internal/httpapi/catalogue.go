package httpapi

import (
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/KarthikReddy8809/catalift/server/internal/auth"
	"github.com/KarthikReddy8809/catalift/server/internal/catalogue"
	"github.com/KarthikReddy8809/catalift/server/internal/generation"
	"github.com/KarthikReddy8809/catalift/server/internal/store"
)

// Upload caps (T-11, T-16): the whole multipart body, and photos per request.
const (
	maxCSVRequest    = catalogue.MaxCSVBytes + 64<<10
	maxImagesRequest = 20*catalogue.MaxImageBytes + 1<<20
	maxImagesPerCall = 20
)

func brandJSON(b *store.Brand) map[string]any {
	var note any
	if b.VoiceNote.Valid {
		note = b.VoiceNote.String
	}
	return map[string]any{"id": idString(b.ID), "name": b.Name, "voice_note": note, "words_to_avoid": nonNil(b.WordsToAvoid),
		"created_at": ts(b.CreatedAt.Time), "updated_at": ts(b.UpdatedAt.Time)}
}

func (a *api) listBrands(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	limit, key, ok := pageArgs(w, r)
	if !ok {
		return
	}
	uploadID, ok := uploadFilter(w, r)
	if !ok {
		return
	}
	rows, err := a.Catalogue.Brands(r.Context(), keyAt(key, 0), limit+1, uploadID)
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
		data[i] = brandJSON(&rows[i])
		next = cursorOf(rows[i].Name)
	}
	WriteJSON(w, http.StatusOK, map[string]any{"data": data, "page": page(more, next)})
}

func (a *api) updateBrand(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	id, ok := pathID(w, r, "brand_id")
	if !ok {
		return
	}
	var in struct {
		VoiceNote    *string  `json:"voice_note"`
		WordsToAvoid []string `json:"words_to_avoid"`
	}
	if !decode(w, r, &in) || !requireLen(w, r, "voice_note", in.VoiceNote, 2000) {
		return
	}
	var words []string // nil keeps the stored list
	if in.WordsToAvoid != nil {
		clean, reason := catalogue.CleanAvoidWords(in.WordsToAvoid)
		if reason != "" {
			badRequest(w, r, "words_to_avoid", reason)
			return
		}
		words = clean
	}
	if in.VoiceNote != nil {
		t := strings.TrimSpace(*in.VoiceNote)
		in.VoiceNote = &t
		if t == "" {
			in.VoiceNote = nil
		}
	}
	b, err := a.Catalogue.SetBrandVoice(r.Context(), id, in.VoiceNote, words)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if words != nil {
		// The brand's listings are checked against the new list now, as a
		// channel's are after its rules change.
		counts, err := a.Listings.RecheckBrand(r.Context(), b.ID)
		if err != nil {
			a.fail(w, r, err)
			return
		}
		a.log.InfoContext(r.Context(), "brand words to avoid changed", "brand_id", b.ID,
			"rechecked", counts.Rechecked, "approvals_cleared", counts.ApprovalsCleared)
	}
	WriteJSON(w, http.StatusOK, brandJSON(&b))
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

// getLatestUpload returns the newest upload: every screen shows only its
// records, while older ones stay in the database.
func (a *api) getLatestUpload(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	id, err := a.Catalogue.LatestUpload(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.writeUpload(w, r, http.StatusOK, id)
}

func (a *api) getUpload(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	if id, ok := pathID(w, r, "upload_id"); ok {
		a.writeUpload(w, r, http.StatusOK, id)
	}
}

// readPhotos reads the "files" parts of a multipart body; photos over 10 MB
// are named, not read. It writes the error response itself and says ok=false.
func (a *api) readPhotos(w http.ResponseWriter, r *http.Request) (files []catalogue.ImageFile, tooLarge []string, ok bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImagesRequest)
	mr, err := r.MultipartReader()
	if err != nil {
		badRequest(w, r, "body", "must be multipart/form-data with files parts")
		return nil, nil, false
	}
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			a.fail(w, r, err)
			return nil, nil, false
		}
		if part.FormName() != "files" {
			continue
		}
		if len(files)+len(tooLarge) >= maxImagesPerCall {
			badRequest(w, r, "files", "may hold at most "+strconv.Itoa(maxImagesPerCall)+" photos per request")
			return nil, nil, false
		}
		data, err := io.ReadAll(io.LimitReader(part, catalogue.MaxImageBytes+1))
		if err != nil {
			a.fail(w, r, err)
			return nil, nil, false
		}
		if len(data) > catalogue.MaxImageBytes {
			tooLarge = append(tooLarge, part.FileName())
			continue
		}
		files = append(files, catalogue.ImageFile{Name: part.FileName(), Data: data})
	}
	if len(files)+len(tooLarge) == 0 {
		badRequest(w, r, "files", "must hold at least one photo")
		return nil, nil, false
	}
	return files, tooLarge, true
}

func photosJSON(res catalogue.ImagesResult, tooLarge []string) map[string]any {
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
	return map[string]any{
		"attached": attached, "unmatched_files": nonNil(res.UnmatchedFiles), "rejected_files": rejected,
		"products_missing_image": nonNil(res.ProductsMissingImage),
	}
}

func (a *api) createImages(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	id, ok := pathID(w, r, "upload_id")
	if !ok {
		return
	}
	files, tooLarge, ok := a.readPhotos(w, r)
	if !ok {
		return
	}
	res, err := a.Catalogue.AttachImages(r.Context(), id, files)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusCreated, photosJSON(res, tooLarge))
}

// createProductImages adds photos to one product the seller picked, whatever
// the files are called: the fix for a product left without a photo.
func (a *api) createProductImages(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	id, ok := pathID(w, r, "product_id")
	if !ok {
		return
	}
	files, tooLarge, ok := a.readPhotos(w, r)
	if !ok {
		return
	}
	res, err := a.Catalogue.AttachToProduct(r.Context(), id, files)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusCreated, photosJSON(res, tooLarge))
}

// listRowErrors lists the rejected CSV rows not yet fixed, with their typed
// values and the categories a fix may use.
func (a *api) listRowErrors(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	var upload int64
	if v := r.URL.Query().Get("filter[upload_id]"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			badRequest(w, r, "filter[upload_id]", "must be a positive integer")
			return
		}
		upload = n
	}
	rows, err := a.Catalogue.OpenRowErrors(r.Context(), upload)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(rows))
	for i := range rows {
		e := &rows[i]
		data[i] = map[string]any{
			"upload_id": idString(e.UploadID), "file_name": e.FileName, "row_number": e.RowNumber,
			"sku": textOrNil(e.Sku), "reason": e.Reason, "category": textOrNil(e.RawCategory),
			"brand": textOrNil(e.RawBrand), "price": textOrNil(e.RawPrice),
		}
	}
	WriteJSON(w, http.StatusOK, map[string]any{"data": data, "categories": a.Catalogue.CategoryNames()})
}

// updateRow re-checks a seller's correction of a rejected row: a valid row is
// loaded as a product, an invalid one keeps its error with the new reason.
func (a *api) updateRow(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	upload, ok := pathID(w, r, "upload_id")
	if !ok {
		return
	}
	rowNumber, err := strconv.ParseInt(r.PathValue("row_number"), 10, 32)
	if err != nil || rowNumber < 2 {
		badRequest(w, r, "row_number", "must be a CSV row number, 2 or more")
		return
	}
	var in struct {
		SKU      string `json:"sku"`
		Category string `json:"category"`
		Brand    string `json:"brand"`
		Price    string `json:"price"`
	}
	if !decode(w, r, &in) {
		return
	}
	productID, reason, err := a.Catalogue.FixRow(r.Context(), upload, int32(rowNumber), catalogue.RowFix{
		SKU: in.SKU, Category: in.Category, Brand: in.Brand, Price: in.Price,
	})
	if errors.Is(err, catalogue.ErrRowNotFound) {
		apiError(w, r, http.StatusNotFound, "not_found", "That row has no open error; it may be loaded already.")
		return
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if reason != "" {
		WriteJSON(w, http.StatusOK, map[string]any{"status": "rejected", "reason": reason, "product_id": nil})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"status": "loaded", "reason": nil, "product_id": idString(productID)})
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
	var confidence any
	if p.DetectionConfidence.Valid {
		confidence = p.DetectionConfidence.Float32
	}
	return map[string]any{
		"upload_id": idString(p.UploadID),
		"status":    catalogue.ProductStatus(p),
		"listings": map[string]int32{
			"total": p.ListingsTotal, "failing_rules": p.ListingsFailing, "approved": p.ListingsApproved,
		},
		"id": idString(p.ID), "sku": p.Sku, "brand": map[string]string{"id": idString(p.BrandID), "name": p.BrandName},
		"category": p.Category, "price_minor": p.PriceMinor, "currency": p.Currency, "image_count": p.ImageCount,
		"attributes": map[string]any{
			"detection_status": p.DetectionStatus, "revision": p.Revision,
			"colour": textOrNil(p.Colour), "pattern": textOrNil(p.Pattern), "sleeve": textOrNil(p.Sleeve),
			"neckline": textOrNil(p.Neckline), "fit": textOrNil(p.Fit), "detection_error": textOrNil(p.DetectionError),
			"confidence": confidence,
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

func (a *api) getProductImage(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	id, ok := pathID(w, r, "product_id")
	if !ok {
		return
	}
	f, err := a.Catalogue.Thumbnail(r.Context(), id)
	if errors.Is(err, catalogue.ErrNoImage) {
		apiError(w, r, http.StatusNotFound, "not_found", "The product has no photo yet.")
		return
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	defer func() { _ = f.Close() }() // read-only file; nothing to recover on close
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err := io.Copy(w, f); err != nil {
		a.log.WarnContext(r.Context(), "thumbnail interrupted", "product_id", id, "err", err)
	}
}

// updateProduct corrects a loaded product's SKU, category, brand and price
// with the upload's rules; a refused change says why, as a row fix does.
func (a *api) updateProduct(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	id, ok := pathID(w, r, "product_id")
	if !ok {
		return
	}
	var in struct {
		SKU      string `json:"sku"`
		Category string `json:"category"`
		Brand    string `json:"brand"`
		Price    string `json:"price"`
	}
	if !decode(w, r, &in) {
		return
	}
	reason, err := a.Catalogue.UpdateProduct(r.Context(), id, catalogue.RowFix{SKU: in.SKU, Category: in.Category, Brand: in.Brand, Price: in.Price})
	if errors.Is(err, catalogue.ErrHasApprovals) {
		apiError(w, r, http.StatusUnprocessableEntity, "unprocessable",
			"A reviewer has approved this product's listings, so its details can no longer change.")
		return
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if reason != "" {
		WriteJSON(w, http.StatusOK, map[string]any{"status": "rejected", "reason": reason})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"status": "saved", "reason": nil})
}

// enrichProduct runs the vision call again for one product the seller fixed.
func (a *api) enrichProduct(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	id, ok := pathID(w, r, "product_id")
	if !ok {
		return
	}
	run, err := a.Generation.EnrichOne(r.Context(), p.UserID, id)
	if errors.Is(err, generation.ErrNoPhoto) {
		apiError(w, r, http.StatusUnprocessableEntity, "unprocessable", "Add a photo first; the AI reads the attributes from it.")
		return
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.log.InfoContext(r.Context(), "product enrichment queued", "product_id", id, "run_id", run.ID, "user_id", p.UserID)
	WriteJSON(w, http.StatusCreated, runJSON(run))
}

// discardRow drops one rejected row the seller does not want to load.
func (a *api) discardRow(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	upload, ok := pathID(w, r, "upload_id")
	if !ok {
		return
	}
	rowNumber, err := strconv.ParseInt(r.PathValue("row_number"), 10, 32)
	if err != nil || rowNumber < 2 {
		badRequest(w, r, "row_number", "must be a CSV row number, 2 or more")
		return
	}
	err = a.Catalogue.DiscardRow(r.Context(), upload, int32(rowNumber))
	if errors.Is(err, catalogue.ErrRowNotFound) {
		apiError(w, r, http.StatusNotFound, "not_found", "That row has no open error.")
		return
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// discardRows drops every open rejected row, or one upload's.
func (a *api) discardRows(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	var upload int64
	if v := r.URL.Query().Get("filter[upload_id]"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			badRequest(w, r, "filter[upload_id]", "must be a positive integer")
			return
		}
		upload = n
	}
	n, err := a.Catalogue.DiscardRows(r.Context(), upload)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"discarded": n})
}
