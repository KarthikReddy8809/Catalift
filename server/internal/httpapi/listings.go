package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/KarthikReddy8809/catalift/server/internal/auth"
	"github.com/KarthikReddy8809/catalift/server/internal/listings"
	"github.com/KarthikReddy8809/catalift/server/internal/store"
)

const maxApprovalsPerCall = 500

func listingJSON(l *listings.GridRow) map[string]any {
	var bullets any
	if l.Title.Valid {
		bullets = []string{l.Bullet1.String, l.Bullet2.String, l.Bullet3.String, l.Bullet4.String, l.Bullet5.String}
	}
	fails := make([]map[string]string, len(l.Failures))
	for i, f := range l.Failures {
		fails[i] = map[string]string{"rule": f.Rule, "field": f.Field, "message": f.Message}
	}
	var approvedAt, regen any
	if l.ApprovedAt.Valid {
		approvedAt = ts(l.ApprovedAt.Time)
	}
	if l.Regeneration != nil {
		regen = map[string]any{"field": l.Regeneration.Field, "status": l.Regeneration.Status}
	}
	return map[string]any{
		"id": idString(l.ID), "product_id": idString(l.ProductID), "sku": l.Sku, "channel": l.Channel,
		"status": l.Status, "failure_reason": textOrNil(l.FailureReason), "version": l.Version,
		"title": textOrNil(l.Title), "bullets": bullets, "description": textOrNil(l.Description),
		"rule_status": l.RuleStatus, "rule_failures": fails,
		"approved": l.ApprovedAt.Valid, "approved_at": approvedAt, "approved_by": textOrNil(l.ApprovedBy),
		"attributes": map[string]any{
			"colour": textOrNil(l.Colour), "pattern": textOrNil(l.Pattern), "sleeve": textOrNil(l.Sleeve),
			"neckline": textOrNil(l.Neckline), "fit": textOrNil(l.Fit),
		},
		"attributes_revision": l.AttributesRevision, "latest_regeneration": regen,
	}
}

func (a *api) listListings(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	limit, key, ok := pageArgs(w, r)
	if !ok {
		return
	}
	params := store.ListGridParams{AfterSku: keyAt(key, 0), AfterChannel: keyAt(key, 1), PageSize: limit + 1}
	q := r.URL.Query()
	if v := q.Get("filter[channel]"); v != "" {
		params.Channel = pgtype.Text{String: v, Valid: true}
	}
	if v := q.Get("filter[rule_status]"); v != "" {
		if !oneOf(v, "unchecked", "passing", "failing") {
			badRequest(w, r, "filter[rule_status]", "must be unchecked, passing or failing")
			return
		}
		params.RuleStatus = store.NullRuleStatus{RuleStatus: store.RuleStatus(v), Valid: true}
	}
	if v := q.Get("filter[approved]"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			badRequest(w, r, "filter[approved]", "must be true or false")
			return
		}
		params.Approved = pgtype.Bool{Bool: b, Valid: true}
	}
	uploadID, ok := uploadFilter(w, r)
	if !ok {
		return
	}
	if uploadID != 0 {
		params.UploadID = pgtype.Int8{Int64: uploadID, Valid: true}
	}
	rows, err := a.Listings.Grid(r.Context(), params)
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
		data[i] = listingJSON(&rows[i])
		next = cursorOf(strings.ToLower(rows[i].Sku), rows[i].Channel)
	}
	WriteJSON(w, http.StatusOK, map[string]any{"data": data, "page": page(more, next)})
}

func (a *api) loadListing(w http.ResponseWriter, r *http.Request, id int64) (listings.GridRow, bool) {
	rows, err := a.Listings.Grid(r.Context(), store.ListGridParams{ListingID: pgtype.Int8{Int64: id, Valid: true}, PageSize: 1})
	if err != nil {
		a.fail(w, r, err)
		return listings.GridRow{}, false
	}
	if len(rows) == 0 {
		a.fail(w, r, listings.ErrNotFound)
		return listings.GridRow{}, false
	}
	return rows[0], true
}

func (a *api) getListing(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	id, ok := pathID(w, r, "listing_id")
	if !ok {
		return
	}
	if l, ok := a.loadListing(w, r, id); ok {
		WriteJSON(w, http.StatusOK, listingJSON(&l))
	}
}

type listingUpdate struct {
	Version     int32   `json:"version"`
	Title       *string `json:"title"`
	Bullet1     *string `json:"bullet_1"`
	Bullet2     *string `json:"bullet_2"`
	Bullet3     *string `json:"bullet_3"`
	Bullet4     *string `json:"bullet_4"`
	Bullet5     *string `json:"bullet_5"`
	Description *string `json:"description"`
}

// validate checks the version and every given field; it names the first problem.
func (in listingUpdate) validate() (field, reason string) {
	if in.Version < 1 {
		return "version", "is required"
	}
	fields := []struct {
		name string
		v    *string
		max  int
	}{
		{"title", in.Title, 500}, {"bullet_1", in.Bullet1, 1000}, {"bullet_2", in.Bullet2, 1000}, {"bullet_3", in.Bullet3, 1000},
		{"bullet_4", in.Bullet4, 1000}, {"bullet_5", in.Bullet5, 1000}, {"description", in.Description, 10000},
	}
	given := 0
	for _, f := range fields {
		if f.v == nil {
			continue
		}
		given++
		if strings.TrimSpace(*f.v) == "" {
			return f.name, "must not be empty"
		}
		if len([]rune(*f.v)) > f.max {
			return f.name, "must be at most " + strconv.Itoa(f.max) + " characters"
		}
	}
	if given == 0 {
		return "body", "must change at least one field"
	}
	return "", ""
}

func (a *api) updateListing(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	id, ok := pathID(w, r, "listing_id")
	if !ok {
		return
	}
	var in listingUpdate
	if !decode(w, r, &in) {
		return
	}
	if field, reason := in.validate(); field != "" {
		badRequest(w, r, field, reason)
		return
	}
	err := a.Listings.UpdateText(r.Context(), id, listings.Edit{
		Version: in.Version, Title: in.Title, Description: in.Description,
		Bullets: [5]*string{in.Bullet1, in.Bullet2, in.Bullet3, in.Bullet4, in.Bullet5},
	})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if l, ok := a.loadListing(w, r, id); ok {
		WriteJSON(w, http.StatusOK, listingJSON(&l))
	}
}

func (a *api) updateAttributes(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	id, ok := pathID(w, r, "product_id")
	if !ok {
		return
	}
	var in struct {
		Revision *int32  `json:"revision"`
		Colour   *string `json:"colour"`
		Pattern  *string `json:"pattern"`
		Sleeve   *string `json:"sleeve"`
		Neckline *string `json:"neckline"`
		Fit      *string `json:"fit"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Revision == nil {
		badRequest(w, r, "revision", "is required")
		return
	}
	for _, f := range []struct {
		name string
		v    *string
	}{{"colour", in.Colour}, {"pattern", in.Pattern}, {"sleeve", in.Sleeve}, {"neckline", in.Neckline}, {"fit", in.Fit}} {
		if f.v != nil && strings.TrimSpace(*f.v) == "" {
			badRequest(w, r, f.name, "must not be empty")
			return
		}
		if !requireLen(w, r, f.name, f.v, 100) {
			return
		}
	}
	row, err := a.Listings.CorrectAttributes(r.Context(), id, p.UserID, listings.AttributesEdit{
		Revision: *in.Revision, Colour: in.Colour, Pattern: in.Pattern, Sleeve: in.Sleeve, Neckline: in.Neckline, Fit: in.Fit,
	})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"detection_status": row.DetectionStatus, "revision": row.Revision,
		"colour": textOrNil(row.Colour), "pattern": textOrNil(row.Pattern), "sleeve": textOrNil(row.Sleeve),
		"neckline": textOrNil(row.Neckline), "fit": textOrNil(row.Fit), "detection_error": textOrNil(row.DetectionError),
	})
}

func regenJSON(id, listingID int64, field, instruction string, status store.RegenerationStatus, created pgtype.Timestamptz) map[string]any {
	return map[string]any{"id": idString(id), "listing_id": idString(listingID), "field": field,
		"instruction": instruction, "status": status, "created_at": ts(created.Time)}
}

func (a *api) listRegenerations(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	id, ok := pathID(w, r, "listing_id")
	if !ok {
		return
	}
	limit, key, ok := pageArgs(w, r)
	if !ok {
		return
	}
	before := int64(1<<63 - 1)
	if k := keyAt(key, 0); k != "" {
		n, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			badRequest(w, r, "cursor", "is not a cursor this API issued")
			return
		}
		before = n
	}
	if _, ok := a.loadListing(w, r, id); !ok {
		return
	}
	rows, err := a.Listings.Regenerations(r.Context(), id, before, limit+1)
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
	for i, g := range rows {
		data[i] = regenJSON(g.ID, g.ListingID, g.Field, g.Instruction, g.Status, g.CreatedAt)
		next = cursorOf(idString(g.ID))
	}
	WriteJSON(w, http.StatusOK, map[string]any{"data": data, "page": page(more, next)})
}

func (a *api) createRegeneration(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	id, ok := pathID(w, r, "listing_id")
	if !ok {
		return
	}
	var in struct {
		Field       string `json:"field"`
		Instruction string `json:"instruction"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !oneOf(in.Field, listings.Fields...) {
		badRequest(w, r, "field", "must be title, bullet_1 to bullet_5 or description")
		return
	}
	in.Instruction = strings.TrimSpace(in.Instruction)
	if in.Instruction == "" {
		badRequest(w, r, "instruction", "is required")
		return
	}
	if !requireLen(w, r, "instruction", &in.Instruction, 1000) {
		return
	}
	row, err := a.Listings.RequestRegeneration(r.Context(), id, p.UserID, in.Field, in.Instruction)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusCreated, regenJSON(row.ID, row.ListingID, row.Field, row.Instruction, row.Status, row.CreatedAt))
}

func (a *api) createApprovals(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var in struct {
		Items []struct {
			ListingID string `json:"listing_id"`
			Version   int32  `json:"version"`
		} `json:"items"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.Items) == 0 || len(in.Items) > maxApprovalsPerCall {
		badRequest(w, r, "items", "must hold 1 to "+strconv.Itoa(maxApprovalsPerCall)+" listings")
		return
	}
	items := make([]listings.ApproveItem, len(in.Items))
	for i, it := range in.Items {
		id, err := strconv.ParseInt(it.ListingID, 10, 64)
		if err != nil || id <= 0 || it.Version < 1 {
			badRequest(w, r, "items["+strconv.Itoa(i)+"]", "needs a listing_id and a version")
			return
		}
		items[i] = listings.ApproveItem{ListingID: id, Version: it.Version}
	}
	ok, skipped, err := a.Listings.Approve(r.Context(), p.UserID, items)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	approved := make([]map[string]any, len(ok))
	for i, o := range ok {
		approved[i] = map[string]any{"listing_id": idString(o.ListingID), "version": o.Version, "approved_at": ts(o.ApprovedAt)}
	}
	skip := make([]map[string]any, len(skipped))
	for i, s := range skipped {
		skip[i] = map[string]any{"listing_id": idString(s.ListingID), "reason": s.Reason}
	}
	a.log.InfoContext(r.Context(), "approvals", "user_id", p.UserID, "approved", len(ok), "skipped", len(skipped))
	WriteJSON(w, http.StatusCreated, map[string]any{"approved": approved, "skipped": skip})
}

// createRuleCheck runs a listing's channel rules on draft text without saving
// it, so the editor's badge updates as the reviewer types. It changes nothing,
// so it needs no Idempotency-Key.
func (a *api) createRuleCheck(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	var in struct {
		ListingID string `json:"listing_id"`
		listingUpdate
	}
	if !decode(w, r, &in) {
		return
	}
	id, err := strconv.ParseInt(in.ListingID, 10, 64)
	if err != nil || id <= 0 {
		badRequest(w, r, "listing_id", "must be a positive integer")
		return
	}
	in.Version = 1 // validate() needs a version; a check is not tied to one
	if field, reason := in.validate(); field != "" && field != "body" {
		badRequest(w, r, field, reason)
		return
	}
	fails, err := a.Listings.CheckDraft(r.Context(), id, listings.Edit{
		Title: in.Title, Description: in.Description,
		Bullets: [5]*string{in.Bullet1, in.Bullet2, in.Bullet3, in.Bullet4, in.Bullet5},
	})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	out := make([]map[string]string, len(fails))
	for i, f := range fails {
		out[i] = map[string]string{"rule": f.Rule, "field": f.Field, "message": f.Message}
	}
	status := "passing"
	if len(fails) > 0 {
		status = "failing"
	}
	WriteJSON(w, http.StatusOK, map[string]any{"rule_status": status, "rule_failures": out})
}
