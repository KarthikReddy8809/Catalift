package httpapi

import (
	"io"
	"net/http"
	"strconv"

	"github.com/KarthikReddy8809/catalift/server/internal/auth"
	"github.com/KarthikReddy8809/catalift/server/internal/exports"
	"github.com/KarthikReddy8809/catalift/server/internal/generation"
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
	rechecks, err := a.Listings.Rechecks(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	counts, err := a.Exports.Readiness(r.Context())
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
	for i := range a.Channels.Channels {
		c := &a.Channels.Channels[i]
		headers := make([]string, len(c.ExportColumns))
		for i, col := range c.ExportColumns {
			headers[i] = col.Header
		}
		var lr any
		if v, ok := last[c.ID]; ok {
			lr = v
		}
		data = append(data, map[string]any{
			"id": c.ID, "name": c.Name, "enabled": true, "load_error": nil, "title_max_length": c.TitleMaxLength,
			"required_attributes": nonNil(c.RequiredAttributes), "banned_words": nonNil(c.BannedWords), "export_headers": headers,
			"last_recheck": lr, "listings_total": byCh[c.ID].total, "listings_approved": byCh[c.ID].approved,
		})
	}
	for _, e := range a.Channels.Errors {
		id := e.ID
		if id == "" {
			id = e.File
		}
		data = append(data, map[string]any{
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
	return map[string]any{"id": idString(e.ID), "created_at": ts(e.CreatedAt), "files": files, "skipped_channels": skipped}
}

func (a *api) createExport(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	e, err := a.Exports.Create(r.Context(), p.UserID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.log.InfoContext(r.Context(), "export written", "export_id", e.ID, "user_id", p.UserID, "files", len(e.Files))
	WriteJSON(w, http.StatusCreated, exportJSON(e))
}

func (a *api) getExport(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	id, ok := pathID(w, r, "export_id")
	if !ok {
		return
	}
	e, err := a.Exports.Get(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, exportJSON(e))
}

func (a *api) getExportFile(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	id, ok := pathID(w, r, "export_id")
	if !ok {
		return
	}
	ch := r.PathValue("channel")
	f, err := a.Exports.Open(r.Context(), id, ch)
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
