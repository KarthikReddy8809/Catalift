// Package exports writes one CSV per enabled channel from the approved
// listings (US-00-009, REQ-014) and serves the files back for download.
package exports

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KarthikReddy8809/catalift/server/internal/channels"
	"github.com/KarthikReddy8809/catalift/server/internal/store"
)

// ErrNothingApproved means no enabled channel has an approved listing.
var ErrNothingApproved = errors.New("no approved listings to export")

// ErrNotFound is an unknown export or a channel it has no file for.
var ErrNotFound = errors.New("no such export file")

// File is one channel's CSV in an export.
type File struct {
	Channel  string
	RowCount int32
}

// Skip is an enabled channel that got no file, with the reason.
type Skip struct {
	Channel string
	Reason  string
}

// Export is one export and its files.
type Export struct {
	ID        int64
	CreatedAt time.Time
	Files     []File
	Skipped   []Skip
}

// Service writes and reads exports.
type Service struct {
	pool     *pgxpool.Pool
	channels channels.Set
	dataDir  string
}

// NewService builds the service; files go under dataDir/exports.
func NewService(pool *pgxpool.Pool, set channels.Set, dataDir string) *Service {
	return &Service{pool: pool, channels: set, dataDir: dataDir}
}

// Create writes a CSV for every enabled channel with at least one approved
// listing. Columns and headers come from the channel file (REQ-017).
func (s *Service) Create(ctx context.Context, userID int64) (Export, error) {
	q := store.New(s.pool)
	type pending struct {
		ch   channels.Channel
		rows []store.ApprovedForChannelRow
	}
	var work []pending
	var skipped []Skip
	for _, ch := range s.channels.Channels { //nolint:gocritic // US-00-010: two channels; the copy is cheap and kept for readability.
		rows, err := q.ApprovedForChannel(ctx, ch.ID)
		if err != nil {
			return Export{}, fmt.Errorf("approved for %s: %w", ch.ID, err)
		}
		if len(rows) == 0 {
			skipped = append(skipped, Skip{Channel: ch.ID, Reason: "no approved listings"})
			continue
		}
		work = append(work, pending{ch: ch, rows: rows})
	}
	if len(work) == 0 {
		return Export{}, ErrNothingApproved
	}
	out := Export{Skipped: skipped}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Export{}, fmt.Errorf("begin export: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // a no-op after Commit
	tq := store.New(tx)
	e, err := tq.CreateExport(ctx, userID)
	if err != nil {
		return Export{}, fmt.Errorf("create export: %w", err)
	}
	out.ID, out.CreatedAt = e.ID, e.CreatedAt.Time
	dir := filepath.Join(s.dataDir, "exports", fmt.Sprint(e.ID))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return Export{}, fmt.Errorf("export dir: %w", err)
	}
	for i := range work {
		w := &work[i]
		path := filepath.Join(dir, w.ch.ID+".csv")
		if err := writeCSV(path, w.ch, w.rows); err != nil {
			return Export{}, err
		}
		n := int32(len(w.rows)) //nolint:gosec // US-00-009: bounded by the product count.
		if err := tq.InsertExportFile(ctx, store.InsertExportFileParams{ExportID: e.ID, Channel: w.ch.ID, FilePath: path, RowCount: n}); err != nil {
			return Export{}, fmt.Errorf("record export file: %w", err)
		}
		out.Files = append(out.Files, File{Channel: w.ch.ID, RowCount: n})
	}
	if err := tx.Commit(ctx); err != nil {
		return Export{}, fmt.Errorf("commit export: %w", err)
	}
	return out, nil
}

// Get reads an export and its files.
func (s *Service) Get(ctx context.Context, id int64) (Export, error) {
	q := store.New(s.pool)
	e, err := q.GetExport(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Export{}, ErrNotFound
	}
	if err != nil {
		return Export{}, fmt.Errorf("load export: %w", err)
	}
	files, err := q.ListExportFiles(ctx, id)
	if err != nil {
		return Export{}, fmt.Errorf("export files: %w", err)
	}
	out := Export{ID: e.ID, CreatedAt: e.CreatedAt.Time}
	for _, f := range files {
		out.Files = append(out.Files, File{Channel: f.Channel, RowCount: f.RowCount})
	}
	return out, nil
}

// Open returns one channel's CSV of an export; the caller closes it.
func (s *Service) Open(ctx context.Context, id int64, channel string) (io.ReadCloser, error) {
	path, err := store.New(s.pool).GetExportFile(ctx, store.GetExportFileParams{ExportID: id, Channel: channel})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("export file: %w", err)
	}
	f, err := os.Open(path) //nolint:gosec // US-00-009: the path was written by Create, never by a client.
	if err != nil {
		return nil, fmt.Errorf("open export file: %w", err)
	}
	return f, nil
}

func writeCSV(path string, ch channels.Channel, rows []store.ApprovedForChannelRow) error {
	f, err := os.Create(path) //nolint:gosec // US-00-009: path built from the export id and a configured channel id.
	if err != nil {
		return fmt.Errorf("create %s: %w", filepath.Base(path), err)
	}
	w := csv.NewWriter(f)
	header := make([]string, len(ch.ExportColumns))
	for i, c := range ch.ExportColumns {
		header[i] = c.Header
	}
	if err := w.Write(header); err != nil {
		_ = f.Close() // the write error is the one reported
		return fmt.Errorf("write header: %w", err)
	}
	for j := range rows {
		rec := make([]string, len(ch.ExportColumns))
		for i, c := range ch.ExportColumns {
			rec[i] = EscapeCell(fieldValue(&rows[j], c.Field))
		}
		if err := w.Write(rec); err != nil {
			_ = f.Close() // the write error is the one reported
			return fmt.Errorf("write row: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		_ = f.Close() // the flush error is the one reported
		return fmt.Errorf("flush csv: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close csv: %w", err)
	}
	return nil
}

func fieldValue(r *store.ApprovedForChannelRow, field string) string {
	t := func(v pgtype.Text) string { return v.String }
	switch field {
	case "sku":
		return r.Sku
	case "title":
		return t(r.Title)
	case "bullet_1":
		return t(r.Bullet1)
	case "bullet_2":
		return t(r.Bullet2)
	case "bullet_3":
		return t(r.Bullet3)
	case "bullet_4":
		return t(r.Bullet4)
	case "bullet_5":
		return t(r.Bullet5)
	case "description":
		return t(r.Description)
	case "colour":
		return t(r.Colour)
	case "pattern":
		return t(r.Pattern)
	case "sleeve":
		return t(r.Sleeve)
	case "neckline":
		return t(r.Neckline)
	case "fit":
		return t(r.Fit)
	}
	return ""
}

// EscapeCell stops a spreadsheet reading a cell as a formula (T-25): a value
// starting with = + - @ tab or carriage return gets a leading quote.
func EscapeCell(v string) string {
	if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
		return "'" + v
	}
	return v
}

// Readiness counts approved and total listings per channel (S-06).
func (s *Service) Readiness(ctx context.Context) ([]store.ApprovalCountsRow, error) {
	rows, err := store.New(s.pool).ApprovalCounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("approval counts: %w", err)
	}
	return rows, nil
}
