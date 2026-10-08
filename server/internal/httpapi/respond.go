package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/KarthikReddy8809/catalift/server/internal/ai"
	"github.com/KarthikReddy8809/catalift/server/internal/catalogue"
	"github.com/KarthikReddy8809/catalift/server/internal/exports"
	"github.com/KarthikReddy8809/catalift/server/internal/generation"
	"github.com/KarthikReddy8809/catalift/server/internal/listings"
	"github.com/KarthikReddy8809/catalift/server/internal/middleware"
)

// detail names the field a validation error is about.
type detail struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// apiError writes the error envelope with the request id (api/openapi.yaml Error).
func apiError(w http.ResponseWriter, r *http.Request, status int, code, message string, details ...detail) {
	body := map[string]any{"code": code, "message": message, "request_id": middleware.RequestIDFrom(r.Context())}
	if len(details) > 0 {
		body["details"] = details
	}
	WriteJSON(w, status, map[string]any{"error": body})
}

func badRequest(w http.ResponseWriter, r *http.Request, field, reason string) {
	apiError(w, r, http.StatusBadRequest, "validation_failed", "The request is not valid: "+field+" "+reason+".", detail{Field: field, Reason: reason})
}

// fail maps a service error to its status and code, once, at the edge.
func (a *api) fail(w http.ResponseWriter, r *http.Request, err error) {
	var voice *generation.VoiceNoteRequiredError
	var tooBig *http.MaxBytesError
	switch {
	case errors.As(err, &voice):
		ds := make([]detail, 0, len(voice.Brands))
		for _, b := range voice.Brands {
			ds = append(ds, detail{Field: "neutral_voice_confirmed", Reason: "brand " + b + " has no voice note"})
		}
		apiError(w, r, http.StatusUnprocessableEntity, "voice_note_required", "A brand has no voice note; add one or confirm a neutral voice.", ds...)
	case errors.As(err, &tooBig):
		apiError(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "The file is too large: a CSV may be 5 MB and an image 10 MB.")
	case errors.Is(err, ai.ErrBudgetBlocked):
		apiError(w, r, http.StatusUnprocessableEntity, "budget_blocked", "AI work is stopped because the budget is used up. The owner must clear the block.")
	case errors.Is(err, listings.ErrNotFound), errors.Is(err, catalogue.ErrUploadNotFound), errors.Is(err, catalogue.ErrBrandNotFound),
		errors.Is(err, catalogue.ErrProductNotFound), errors.Is(err, generation.ErrRunNotFound), errors.Is(err, exports.ErrNotFound),
		errors.Is(err, catalogue.ErrNoUpload):
		apiError(w, r, http.StatusNotFound, "not_found", "No such record.")
	case errors.Is(err, listings.ErrVersionConflict):
		apiError(w, r, http.StatusConflict, "version_conflict", "It changed since you loaded it. Reload and try again.")
	case errors.Is(err, listings.ErrNotGenerated):
		apiError(w, r, http.StatusUnprocessableEntity, "listing_not_generated", "The listing has no generated text yet.")
	case errors.Is(err, listings.ErrNotDetected):
		apiError(w, r, http.StatusUnprocessableEntity, "unprocessable", "The product's attributes are not detected yet.")
	case errors.Is(err, exports.ErrNothingApproved):
		apiError(w, r, http.StatusUnprocessableEntity, "no_approved_listings", "No channel has an approved listing, so there is nothing to export.")
	case errors.Is(err, catalogue.ErrBadHeader):
		apiError(w, r, http.StatusUnprocessableEntity, "unprocessable", catalogue.ErrBadHeader.Error()+".", detail{Field: "file", Reason: "bad header"})
	default:
		a.log.ErrorContext(r.Context(), "request failed", "err", err, "method", r.Method, "path", r.URL.Path,
			"request_id", middleware.RequestIDFrom(r.Context()))
		apiError(w, r, http.StatusInternalServerError, "internal", "Something failed on our side. Quote the request id if it keeps happening.")
	}
}

// decode reads a JSON body of at most 1 MB, refusing unknown fields.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		badRequest(w, r, "body", "is not valid JSON for this request ("+err.Error()+")")
		return false
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		badRequest(w, r, "body", "has data after the JSON object")
		return false
	}
	return true
}

// uploadFilter reads the optional filter[upload_id]; 0 means every upload.
func uploadFilter(w http.ResponseWriter, r *http.Request) (int64, bool) {
	v := r.URL.Query().Get("filter[upload_id]")
	if v == "" {
		return 0, true
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		badRequest(w, r, "filter[upload_id]", "must be a positive integer")
		return 0, false
	}
	return n, true
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		badRequest(w, r, name, "must be a positive integer")
		return 0, false
	}
	return id, true
}

func idString(id int64) string { return strconv.FormatInt(id, 10) }

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// pageArgs reads limit (1 to 100, default 50) and the opaque cursor.
func pageArgs(w http.ResponseWriter, r *http.Request) (limit int32, key []string, ok bool) {
	limit = 50
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			badRequest(w, r, "limit", "must be between 1 and 100")
			return 0, nil, false
		}
		limit = int32(n) //nolint:gosec // US-00-001: bounded to 100 above.
	}
	if c := r.URL.Query().Get("cursor"); c != "" {
		raw, err := base64.RawURLEncoding.DecodeString(c)
		if err != nil || json.Unmarshal(raw, &key) != nil {
			badRequest(w, r, "cursor", "is not a cursor this API issued")
			return 0, nil, false
		}
	}
	return limit, key, true
}

func cursorOf(key ...string) string {
	b, _ := json.Marshal(key) // a string slice always marshals
	return base64.RawURLEncoding.EncodeToString(b)
}

// page is the Page schema; next is the cursor after the last row shown.
func page(hasMore bool, next string) map[string]any {
	p := map[string]any{"has_more": hasMore, "next_cursor": nil}
	if hasMore {
		p["next_cursor"] = next
	}
	return p
}

func keyAt(key []string, i int) string {
	if i < len(key) {
		return key[i]
	}
	return ""
}

func requireLen(w http.ResponseWriter, r *http.Request, field string, v *string, maxLen int) bool {
	if v != nil && len([]rune(*v)) > maxLen {
		badRequest(w, r, field, fmt.Sprintf("must be at most %d characters", maxLen))
		return false
	}
	return true
}
