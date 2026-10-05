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
	pool    *pgxpool.Pool
	dataDir string
}

// NewService builds the service; photos go under dataDir/images.
func NewService(pool *pgxpool.Pool, dataDir string) *Service {
	return &Service{pool: pool, dataDir: dataDir}
}

// UploadCSV reads the product list and creates one product per valid row.
// A SKU already in Catalift is a row error (D21); the rest still load. The
// upload's counts are written in the same transaction.
func (s *Service) UploadCSV(ctx context.Context, userID int64, fileName string, r io.Reader) (int64, error) {
	rows, rowErrs, err := ParseCSV(r)
	if err != nil {
		return 0, err
	}
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
	for _, e := range rowErrs {
		if err := q.InsertRowError(ctx, store.InsertRowErrorParams{UploadID: uploadID, RowNumber: int32(e.Row), Sku: optText(e.SKU), Reason: e.Reason}); err != nil { //nolint:gosec // US-00-001: row numbers are bounded by the 5 MB file.
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
		ctype := http.DetectContentType(f.Data)
		ext, ok := allowedTypes[ctype]
		if !ok {
			res.Rejected = append(res.Rejected, RejectedImage{FileName: f.Name, Reason: "not a JPEG, PNG or WebP image"})
			continue
		}
		if len(f.Data) > MaxImageBytes {
			res.Rejected = append(res.Rejected, RejectedImage{FileName: f.Name, Reason: "larger than 10 MB"})
			continue
		}
		copyJPEG, err := detectionCopy(f.Data)
		if err != nil {
			res.Rejected = append(res.Rejected, RejectedImage{FileName: f.Name, Reason: err.Error()})
			continue
		}
		pid := idBySKU[sku]
		pos, err := q.NextImagePosition(ctx, pid)
		if err != nil {
			return ImagesResult{}, fmt.Errorf("next position: %w", err)
		}
		dir := filepath.Join(s.dataDir, "images", fmt.Sprint(pid))
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return ImagesResult{}, fmt.Errorf("image dir: %w", err)
		}
		orig := filepath.Join(dir, fmt.Sprintf("%d-original%s", pos, ext))
		det := filepath.Join(dir, fmt.Sprintf("%d-detection.jpg", pos))
		if err := os.WriteFile(orig, f.Data, 0o600); err != nil {
			return ImagesResult{}, fmt.Errorf("write original: %w", err)
		}
		if err := os.WriteFile(det, copyJPEG, 0o600); err != nil {
			return ImagesResult{}, fmt.Errorf("write detection copy: %w", err)
		}
		if err := q.InsertImage(ctx, store.InsertImageParams{
			ProductID: pid, Position: pos, OriginalFileName: clipName(f.Name), OriginalPath: orig,
			DetectionPath: det, ContentType: ctype, ByteSize: int32(len(f.Data)), //nolint:gosec // US-00-001: capped at 10 MB above.
		}); err != nil {
			return ImagesResult{}, fmt.Errorf("record image: %w", err)
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
func detectionCopy(data []byte) ([]byte, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("the image could not be read")
	}
	if cfg.Width*cfg.Height > MaxPixels {
		return nil, fmt.Errorf("larger than 50 megapixels")
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("the image could not be read")
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
		return nil, fmt.Errorf("encode detection copy: %w", err)
	}
	return out.Bytes(), nil
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
