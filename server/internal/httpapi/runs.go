package httpapi

import (
	"io"
	"net/http"
	"strconv"

	"github.com/KarthikReddy8809/catalift/server/internal/auth"
	"github.com/KarthikReddy8809/catalift/server/internal/channels"
	"github.com/KarthikReddy8809/catalift/server/internal/exports"
	"github.com/KarthikReddy8809/catalift/server/internal/generation"
	"github.com/KarthikReddy8809/catalift/server/internal/listings"
)

func runJSON(p generation.Progress) map[string]any {
	return map[string]any{"id": idString(p.ID), "created_at": ts(p.CreatedAt), "detection": p.Detection, "listings": p.Listings}
}

func (a *api) createRun(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in struct {
		NeutralVoiceConfirmed bool   `json:"neutral_voice_confirmed"`
		UploadID              string `json:"upload_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	var upload int64
	if in.UploadID != "" {
		n, err := strconv.ParseInt(in.UploadID, 10, 64)
		if err != nil || n <= 0 {
			badRequest(w, r, "upload_id", "must be a positive integer")
			return
		}
		upload = n
	}
	run, err := a.Generation.Start(r.Context(), p.UserID, in.NeutralVoiceConfirmed, upload)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.log.InfoContext(r.Context(), "generation run started", "run_id", run.ID, "user_id", p.UserID)
	WriteJSON(w, http.StatusCreated, runJSON(run))
}

func (a *api) getRun(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	id, ok := pathID(w, r, "run_id")
	if !ok {
		return
	}
	run, err := a.Generation.Progress(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, runJSON(run))
}

func (a *api) resumeRun(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	id, ok := pathID(w, r, "run_id")
	if !ok {
		return
	}
	run, err := a.Generation.Resume(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, runJSON(run))
}

func (a *api) listChannels(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	// Pick up an edit saved by another API process or a direct database fix.
	if err := a.Listings.RefreshRules(r.Context()); err != nil {
		a.fail(w, r, err)
		return
	}
	set := a.Channels.Current()
	rechecks, err := a.Listings.Rechecks(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	uploadID, ok := uploadFilter(w, r)
	if !ok {
		return
	}
	counts, err := a.Exports.Readiness(r.Context(), uploadID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	last := map[string]map[string]any{}
	for _, rc := range rechecks {
		last[rc.Channel] = map[string]any{"config_hash": rc.ConfigHash, "listings_rechecked": rc.ListingsRechecked,
			"approvals_cleared": rc.ApprovalsCleared, "created_at": ts(rc.CreatedAt.Time)}
	}
	type count struct{ total, approved int32 }
	byCh := map[string]count{}
	for _, c := range counts {
		byCh[c.Channel] = count{c.Total, c.Approved}
	}
	data := []map[string]any{}
	for i := range set.Channels {
		c := &set.Channels[i]
		headers := make([]string, len(c.ExportColumns))
		for i, col := range c.ExportColumns {
			headers[i] = col.Header
		}
		var lr any
		if v, ok := last[c.ID]; ok {
			lr = v
		}
		var edited any
		if e, ok := a.Channels.LastEdit(c.ID); ok {
			edited = map[string]string{"by": e.By, "at": ts(e.At)}
		}
		data = append(data, map[string]any{
			"config_hash": c.Hash, "last_edit": edited,
			"id": c.ID, "name": c.Name, "enabled": true, "load_error": nil, "title_max_length": c.TitleMaxLength,
			"required_attributes": nonNil(c.RequiredAttributes), "banned_words": nonNil(c.BannedWords), "export_headers": headers,
			"last_recheck": lr, "listings_total": byCh[c.ID].total, "listings_approved": byCh[c.ID].approved,
		})
	}
	for _, e := range set.Errors {
		id := e.ID
		if id == "" {
			id = e.File
		}
		data = append(data, map[string]any{
			"config_hash": nil, "last_edit": nil,
			"id": id, "name": id, "enabled": false, "load_error": e.Error, "title_max_length": nil,
			"required_attributes": []string{}, "banned_words": []string{}, "export_headers": []string{},
			"last_recheck": nil, "listings_total": 0, "listings_approved": 0,
		})
	}
	WriteJSON(w, http.StatusOK, map[string]any{"data": data, "page": page(false, "")})
}

func exportJSON(e exports.Export) map[string]any {
	files := make([]map[string]any, len(e.Files))
	for i, f := range e.Files {
		files[i] = map[string]any{"channel": f.Channel, "row_count": f.RowCount,
			"download_url": "/v1/exports/" + idString(e.ID) + "/files/" + f.Channel}
	}
	skipped := make([]map[string]string, len(e.Skipped))
	for i, s := range e.Skipped {
		skipped[i] = map[string]string{"channel": s.Channel, "reason": s.Reason}
	}
	var sentAt, sentBy any
	if e.Sent() {
		sentAt, sentBy = ts(e.SentAt), e.SentBy
	}
	return map[string]any{"id": idString(e.ID), "created_at": ts(e.CreatedAt), "files": files, "skipped_channels": skipped,
		"sent_at": sentAt, "sent_by": sentBy}
}

// sellerSees is true for a seller, who sees only exports sent to them.
func sellerSees(p auth.Principal) bool { return p.Role == auth.RoleSeller }

// listExports lists exports newest first: a reviewer sees every export, a
// seller only those a reviewer sent.
func (a *api) listExports(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	limit, key, ok := pageArgs(w, r)
	if !ok {
		return
	}
	var before int64
	if k := keyAt(key, 0); k != "" {
		n, err := strconv.ParseInt(k, 10, 64)
		if err != nil || n <= 0 {
			badRequest(w, r, "cursor", "is not a cursor this API issued")
			return
		}
		before = n
	}
	uploadID, ok := uploadFilter(w, r)
	if !ok {
		return
	}
	sentOnly := sellerSees(p) || r.URL.Query().Get("filter[sent]") == "true"
	rows, err := a.Exports.List(r.Context(), sentOnly, uploadID, before, limit+1)
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
		data[i] = exportJSON(rows[i])
		next = cursorOf(idString(rows[i].ID))
	}
	WriteJSON(w, http.StatusOK, map[string]any{"data": data, "page": page(more, next)})
}

// sendExport hands an export to the seller; sending again is harmless.
func (a *api) sendExport(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	id, ok := pathID(w, r, "export_id")
	if !ok {
		return
	}
	e, err := a.Exports.Send(r.Context(), id, p.UserID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.log.InfoContext(r.Context(), "export sent to seller", "export_id", e.ID, "user_id", p.UserID)
	WriteJSON(w, http.StatusOK, exportJSON(e))
}

func (a *api) createExport(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	// The body is optional: {"upload_id": "12"} exports one upload, no body
	// exports every upload as before.
	var in struct {
		UploadID string `json:"upload_id"`
	}
	if r.ContentLength != 0 && !decode(w, r, &in) {
		return
	}
	var uploadID int64
	if in.UploadID != "" {
		n, err := strconv.ParseInt(in.UploadID, 10, 64)
		if err != nil || n <= 0 {
			badRequest(w, r, "upload_id", "must be a positive integer")
			return
		}
		uploadID = n
	}
	e, err := a.Exports.Create(r.Context(), p.UserID, uploadID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.log.InfoContext(r.Context(), "export written", "export_id", e.ID, "user_id", p.UserID, "files", len(e.Files))
	WriteJSON(w, http.StatusCreated, exportJSON(e))
}

func (a *api) getExport(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	id, ok := pathID(w, r, "export_id")
	if !ok {
		return
	}
	e, err := a.Exports.Get(r.Context(), id, sellerSees(p))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, exportJSON(e))
}

func (a *api) getExportFile(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	id, ok := pathID(w, r, "export_id")
	if !ok {
		return
	}
	ch := r.PathValue("channel")
	f, err := a.Exports.Open(r.Context(), id, ch, sellerSees(p))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	defer func() { _ = f.Close() }() // read-only file; nothing to recover on close
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+ch+"-"+idString(id)+`.csv"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err := io.Copy(w, f); err != nil {
		a.log.WarnContext(r.Context(), "export download interrupted", "export_id", id, "err", err)
	}
}

func (a *api) getBudget(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	b, err := a.Listings.Budget(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	var blocked any
	if b.BlockedAt.Valid {
		blocked = ts(b.BlockedAt.Time)
	}
	WriteJSON(w, http.StatusOK, map[string]any{"limit_micro_usd": b.LimitMicroUsd, "spent_micro_usd": b.SpentMicroUsd, "blocked_at": blocked})
}

// updateChannelRules saves a reviewer's change to one channel's rules and
// re-checks its listings. The body carries the config_hash the reviewer saw,
// so a change made by someone else meanwhile is a version conflict.
func (a *api) updateChannelRules(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in struct {
		ConfigHash         string   `json:"config_hash"`
		TitleMaxLength     int      `json:"title_max_length"`
		RequiredAttributes []string `json:"required_attributes"`
		BannedWords        []string `json:"banned_words"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.ConfigHash == "" {
		badRequest(w, r, "config_hash", "is required")
		return
	}
	channel := r.PathValue("channel")
	counts, err := a.Listings.SaveRules(r.Context(), p.UserID, channel, in.ConfigHash, channels.Rules{
		TitleMaxLength: in.TitleMaxLength, RequiredAttributes: in.RequiredAttributes, BannedWords: in.BannedWords,
	})
	if bad, ok := listings.IsInvalidRules(err); ok {
		apiError(w, r, http.StatusUnprocessableEntity, "unprocessable", "The rules were not saved: "+bad.Reason+".")
		return
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.log.InfoContext(r.Context(), "channel rules changed", "channel", channel, "user_id", p.UserID,
		"rechecked", counts.Rechecked, "approvals_cleared", counts.ApprovalsCleared)
	WriteJSON(w, http.StatusOK, map[string]any{
		"channel": channel, "listings_rechecked": counts.Rechecked, "approvals_cleared": counts.ApprovalsCleared,
	})
}
