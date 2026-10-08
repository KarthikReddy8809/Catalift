package catalogue

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // decoder for uploaded photos
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/image/draw"

	_ "golang.org/x/image/webp" // decoder for uploaded photos

	"github.com/KarthikReddy8809/catalift/server/internal/store"
)

// Limits on uploads (HLD section 9, T-11, T-16).
const (
	MaxCSVBytes   = 5 << 20
	MaxImageBytes = 10 << 20
	MaxPixels     = 50_000_000
	DetectionEdge = 1024
)

// ErrUploadNotFound is an unknown upload id.
var ErrUploadNotFound = errors.New("no such upload")

// Service stores uploads, products and photos.
type Service struct {
	pool       *pgxpool.Pool
	dataDir    string
	categories Categories
}

// NewService builds the service; photos go under dataDir/images and only
// rows in a known category are loaded.
func NewService(pool *pgxpool.Pool, dataDir string, categories Categories) *Service {
	return &Service{pool: pool, dataDir: dataDir, categories: categories}
}

// UploadCSV reads the product list and creates one product per valid row.
// A SKU already in Catalift is a row error (D21); the rest still load. The
// upload's counts are written in the same transaction.
func (s *Service) UploadCSV(ctx context.Context, userID int64, fileName string, r io.Reader) (int64, error) {
	rows, rowErrs, err := ParseCSV(r)
	if err != nil {
		return 0, err
	}
	rows, catErrs := s.categories.Filter(rows)
	rowErrs = append(rowErrs, catErrs...)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin upload: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // a no-op after Commit
	q := store.New(tx)
	uploadID, err := q.CreateUpload(ctx, store.CreateUploadParams{UploadedBy: userID, FileName: clipName(fileName)})
	if err != nil {
		return 0, fmt.Errorf("create upload: %w", err)
	}
	accepted := 0
	for _, row := range rows {
		brandID, err := q.UpsertBrand(ctx, row.Brand)
		if err != nil {
			return 0, fmt.Errorf("brand on row %d: %w", row.Line, err)
		}
		_, err = q.InsertProduct(ctx, store.InsertProductParams{
			Sku: row.SKU, BrandID: brandID, UploadID: uploadID, Category: row.Category, PriceMinor: row.PriceMinor,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			rowErrs = append(rowErrs, RowError{Row: row.Line, SKU: row.SKU, Reason: "SKU already exists"})
			continue
		}
		if err != nil {
			return 0, fmt.Errorf("product on row %d: %w", row.Line, err)
		}
		accepted++
	}
	sort.Slice(rowErrs, func(i, j int) bool { return rowErrs[i].Row < rowErrs[j].Row })
	for _, e := range rowErrs {
		if err := q.InsertRowError(ctx, store.InsertRowErrorParams{
			UploadID: uploadID, RowNumber: int32(e.Row), //nolint:gosec // US-00-001: row numbers are bounded by the 5 MB file.
			Sku: optText(e.SKU), Reason: e.Reason,
			RawCategory: optText(e.Category), RawBrand: optText(e.Brand), RawPrice: optText(e.Price),
		}); err != nil {
			return 0, fmt.Errorf("row error: %w", err)
		}
	}
	total := accepted + len(rowErrs)
	if err := q.FinishUpload(ctx, store.FinishUploadParams{
		ID: uploadID, RowsTotal: int32(total), RowsAccepted: int32(accepted), RowsRejected: int32(len(rowErrs)), //nolint:gosec // US-00-001: bounded by the 5 MB file.
	}); err != nil {
		return 0, fmt.Errorf("finish upload: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit upload: %w", err)
	}
	return uploadID, nil
}

// ImageFile is one uploaded photo.
type ImageFile struct {
	Name string
	Data []byte
}

// ImagesResult is what the seller sees after uploading photos.
type ImagesResult struct {
	Attached             []AttachedImage
	UnmatchedFiles       []string
	Rejected             []RejectedImage
	ProductsMissingImage []string
}

// AttachedImage is one photo that found its SKU.
type AttachedImage struct {
	FileName string
	SKU      string
	Position int16
}

// RejectedImage is a photo refused for its type or size.
type RejectedImage struct {
	FileName string
	Reason   string
}

var allowedTypes = map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}

// AttachImages matches each photo to a SKU of the upload (D20), checks its
// real type and size (T-15, T-16), stores the original under a generated
// name and a copy no larger than 1024 px for detection (D23).
func (s *Service) AttachImages(ctx context.Context, uploadID int64, files []ImageFile) (ImagesResult, error) {
	q := store.New(s.pool)
	if _, err := q.GetUpload(ctx, uploadID); errors.Is(err, pgx.ErrNoRows) {
		return ImagesResult{}, ErrUploadNotFound
	} else if err != nil {
		return ImagesResult{}, fmt.Errorf("load upload: %w", err)
	}
	products, err := q.ListUploadSkus(ctx, uploadID)
	if err != nil {
		return ImagesResult{}, fmt.Errorf("upload skus: %w", err)
	}
	skus := make([]string, 0, len(products))
	idBySKU := map[string]int64{}
	for _, p := range products {
		skus = append(skus, p.Sku)
		idBySKU[p.Sku] = p.ID
	}
	var res ImagesResult
	for _, f := range files {
		sku := MatchSKU(f.Name, skus)
		if sku == "" {
			res.UnmatchedFiles = append(res.UnmatchedFiles, f.Name)
			continue
		}
		pos, reason, err := s.storeImage(ctx, q, idBySKU[sku], f)
		if err != nil {
			return ImagesResult{}, err
		}
		if reason != "" {
			res.Rejected = append(res.Rejected, RejectedImage{FileName: f.Name, Reason: reason})
			continue
		}
		res.Attached = append(res.Attached, AttachedImage{FileName: f.Name, SKU: sku, Position: pos})
	}
	missing, err := q.ProductsMissingImage(ctx, uploadID)
	if err != nil {
		return ImagesResult{}, fmt.Errorf("products missing image: %w", err)
	}
	res.ProductsMissingImage = missing
	return res, nil
}

// detectionCopy refuses an image over 50 megapixels before decoding it in
// full (T-16), then scales it so its long side is at most 1024 px and
// re-encodes it as JPEG, which also drops its metadata (T-17).
// A photo it refuses comes back as a reason the seller sees, not an error.
func detectionCopy(data []byte) (copyJPEG []byte, reason string) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, "the image could not be read"
	}
	if cfg.Width*cfg.Height > MaxPixels {
		return nil, "larger than 50 megapixels"
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "the image could not be read"
	}
	w, h := cfg.Width, cfg.Height
	if w > DetectionEdge || h > DetectionEdge {
		if w >= h {
			h, w = h*DetectionEdge/w, DetectionEdge
		} else {
			w, h = w*DetectionEdge/h, DetectionEdge
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, max(w, 1), max(h, 1)))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 85}); err != nil {
		return nil, "the image could not be converted"
	}
	return out.Bytes(), ""
}

func clipName(n string) string {
	n = filepath.Base(n)
	r := []rune(n)
	if len(r) > 255 {
		return string(r[:255])
	}
	if n == "" || n == "." {
		return "upload"
	}
	return n
}

func optText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

// storeImage checks one photo's real type, size and pixel count (T-15, T-16)
// and stores it with its 1024 px detection copy (D23) for productID. A photo
// refused for its content comes back as a reason, not an error.
func (s *Service) storeImage(ctx context.Context, q *store.Queries, pid int64, f ImageFile) (pos int16, reason string, err error) {
	ctype := http.DetectContentType(f.Data)
	ext, ok := allowedTypes[ctype]
	if !ok {
		return 0, "not a JPEG, PNG or WebP image", nil
	}
	if len(f.Data) > MaxImageBytes {
		return 0, "larger than 10 MB", nil
	}
	copyJPEG, problem := detectionCopy(f.Data)
	if problem != "" {
		return 0, problem, nil
	}
	pos, err = q.NextImagePosition(ctx, pid)
	if err != nil {
		return 0, "", fmt.Errorf("next position: %w", err)
	}
	dir := filepath.Join(s.dataDir, "images", fmt.Sprint(pid))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return 0, "", fmt.Errorf("image dir: %w", err)
	}
	orig := filepath.Join(dir, fmt.Sprintf("%d-original%s", pos, ext))
	det := filepath.Join(dir, fmt.Sprintf("%d-detection.jpg", pos))
	if err := os.WriteFile(orig, f.Data, 0o600); err != nil {
		return 0, "", fmt.Errorf("write original: %w", err)
	}
	if err := os.WriteFile(det, copyJPEG, 0o600); err != nil {
		return 0, "", fmt.Errorf("write detection copy: %w", err)
	}
	if err := q.InsertImage(ctx, store.InsertImageParams{
		ProductID: pid, Position: pos, OriginalFileName: clipName(f.Name), OriginalPath: orig,
		DetectionPath: det, ContentType: ctype, ByteSize: int32(len(f.Data)), //nolint:gosec // US-00-001: capped at 10 MB above.
	}); err != nil {
		return 0, "", fmt.Errorf("record image: %w", err)
	}
	return pos, "", nil
}

// AttachToProduct adds photos to one product the seller chose, whatever the
// files are called: the way to fix a product left without a photo.
func (s *Service) AttachToProduct(ctx context.Context, productID int64, files []ImageFile) (ImagesResult, error) {
	q := store.New(s.pool)
	p, err := s.Product(ctx, productID)
	if err != nil {
		return ImagesResult{}, err
	}
	var res ImagesResult
	for _, f := range files {
		pos, reason, err := s.storeImage(ctx, q, productID, f)
		if err != nil {
			return ImagesResult{}, err
		}
		if reason != "" {
			res.Rejected = append(res.Rejected, RejectedImage{FileName: f.Name, Reason: reason})
			continue
		}
		res.Attached = append(res.Attached, AttachedImage{FileName: f.Name, SKU: p.Sku, Position: pos})
	}
	return res, nil
}

// ErrRowNotFound is a row number the upload has no open error for.
var ErrRowNotFound = errors.New("no such rejected row")

// RowFix is a seller's corrected values for a rejected CSV row.
type RowFix struct {
	SKU, Category, Brand, Price string
}

// FixRow re-checks a corrected row with the upload's rules (ValidateRow, the
// categories, SKUs already in Catalift). A valid row becomes a product of the
// upload and its error is removed; an invalid one keeps its error, now with
// the new reason and values. It returns the product id, or the reason.
func (s *Service) FixRow(ctx context.Context, uploadID int64, rowNumber int32, fix RowFix) (productID int64, reason string, err error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, "", fmt.Errorf("begin fix: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // a no-op after Commit
	q := store.New(tx)
	errID, err := q.GetRowErrorForUpdate(ctx, store.GetRowErrorForUpdateParams{UploadID: uploadID, RowNumber: rowNumber})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", ErrRowNotFound
	}
	if err != nil {
		return 0, "", fmt.Errorf("load rejected row: %w", err)
	}
	row, reason := ValidateRow(int(rowNumber), fix.SKU, fix.Category, fix.Brand, fix.Price)
	if reason == "" && !s.categories.Known(row.Category) {
		reason = s.categories.unknownReason(row.Category)
	}
	if reason == "" {
		brandID, err := q.UpsertBrand(ctx, row.Brand)
		if err != nil {
			return 0, "", fmt.Errorf("brand: %w", err)
		}
		productID, err = q.InsertProduct(ctx, store.InsertProductParams{
			Sku: row.SKU, BrandID: brandID, UploadID: uploadID, Category: row.Category, PriceMinor: row.PriceMinor,
		})
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			reason = "SKU already exists"
		case err != nil:
			return 0, "", fmt.Errorf("product: %w", err)
		}
	}
	if reason != "" {
		if err := q.UpdateRowError(ctx, store.UpdateRowErrorParams{
			ID: errID, Sku: optText(clipRaw(strings.TrimSpace(fix.SKU))), Reason: reason,
			RawCategory: optText(clipRaw(fix.Category)), RawBrand: optText(clipRaw(fix.Brand)), RawPrice: optText(clipRaw(fix.Price)),
		}); err != nil {
			return 0, "", fmt.Errorf("update rejected row: %w", err)
		}
	} else {
		if err := q.DeleteRowError(ctx, errID); err != nil {
			return 0, "", fmt.Errorf("clear rejected row: %w", err)
		}
		if err := q.CountRowFixed(ctx, uploadID); err != nil {
			return 0, "", fmt.Errorf("upload counts: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, "", fmt.Errorf("commit fix: %w", err)
	}
	return productID, reason, nil
}

// OpenRowErrors lists the rejected rows not yet fixed; uploadID 0 means all.
func (s *Service) OpenRowErrors(ctx context.Context, uploadID int64) ([]store.ListOpenRowErrorsRow, error) {
	rows, err := store.New(s.pool).ListOpenRowErrors(ctx, pgtype.Int8{Int64: uploadID, Valid: uploadID != 0})
	if err != nil {
		return nil, fmt.Errorf("open row errors: %w", err)
	}
	return rows, nil
}

// CategoryNames lists the categories the upload accepts, for the fix form.
func (s *Service) CategoryNames() []string { return s.categories.Names() }

// ErrHasApprovals means a product has approved listings, so its source data
// is no longer the seller's to change: the reviewer has signed off on them.
var ErrHasApprovals = errors.New("the product has approved listings")

// UpdateProduct corrects a loaded product's SKU, category, brand and price
// with the upload's rules. A refused change comes back as a reason.
func (s *Service) UpdateProduct(ctx context.Context, id int64, fix RowFix) (reason string, err error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", fmt.Errorf("begin product edit: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // a no-op after Commit
	q := store.New(tx)
	if _, err := q.GetProductForUpdate(ctx, id); errors.Is(err, pgx.ErrNoRows) {
		return "", ErrProductNotFound
	} else if err != nil {
		return "", fmt.Errorf("load product: %w", err)
	}
	approved, err := q.ProductHasApprovals(ctx, id)
	if err != nil {
		return "", fmt.Errorf("check approvals: %w", err)
	}
	if approved {
		return "", ErrHasApprovals
	}
	row, reason := ValidateRow(2, fix.SKU, fix.Category, fix.Brand, fix.Price)
	if reason == "" && !s.categories.Known(row.Category) {
		reason = s.categories.unknownReason(row.Category)
	}
	if reason == "" {
		taken, err := q.SkuTakenByOther(ctx, store.SkuTakenByOtherParams{Sku: row.SKU, ID: id})
		if err != nil {
			return "", fmt.Errorf("check sku: %w", err)
		}
		if taken {
			reason = "SKU already exists"
		}
	}
	if reason != "" {
		return reason, nil
	}
	brandID, err := q.UpsertBrand(ctx, row.Brand)
	if err != nil {
		return "", fmt.Errorf("brand: %w", err)
	}
	if err := q.UpdateProductFields(ctx, store.UpdateProductFieldsParams{
		ID: id, Sku: row.SKU, BrandID: brandID, Category: row.Category, PriceMinor: row.PriceMinor,
	}); err != nil {
		return "", fmt.Errorf("update product: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit product edit: %w", err)
	}
	return "", nil
}

// DiscardRow drops a rejected row the seller does not want to load, such as a
// duplicate of a product already in Catalift.
func (s *Service) DiscardRow(ctx context.Context, uploadID int64, rowNumber int32) error {
	n, err := store.New(s.pool).DiscardRowError(ctx, store.DiscardRowErrorParams{UploadID: uploadID, RowNumber: rowNumber})
	if err != nil {
		return fmt.Errorf("discard rejected row: %w", err)
	}
	if n == 0 {
		return ErrRowNotFound
	}
	return nil
}

// DiscardRows drops every open rejected row; uploadID 0 means every upload.
func (s *Service) DiscardRows(ctx context.Context, uploadID int64) (int64, error) {
	n, err := store.New(s.pool).DiscardRowErrors(ctx, pgtype.Int8{Int64: uploadID, Valid: uploadID != 0})
	if err != nil {
		return 0, fmt.Errorf("discard rejected rows: %w", err)
	}
	return n, nil
}
