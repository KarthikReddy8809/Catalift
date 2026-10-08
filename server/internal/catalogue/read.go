package catalogue

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/KarthikReddy8809/catalift/server/internal/store"
)

// ErrNoUpload means nothing has been uploaded yet.
var ErrNoUpload = errors.New("no upload yet")

// LatestUpload is the newest upload's id; every screen shows only its records.
func (s *Service) LatestUpload(ctx context.Context) (int64, error) {
	id, err := store.New(s.pool).LatestUpload(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNoUpload
	}
	if err != nil {
		return 0, fmt.Errorf("latest upload: %w", err)
	}
	return id, nil
}

// ErrBrandNotFound is an unknown brand id.
var ErrBrandNotFound = errors.New("no such brand")

// ErrProductNotFound is an unknown product id.
var ErrProductNotFound = errors.New("no such product")

// Brands lists brands by name after the given name.
func (s *Service) Brands(ctx context.Context, afterName string, limit int32, uploadID int64) ([]store.Brand, error) {
	rows, err := store.New(s.pool).ListBrands(ctx, store.ListBrandsParams{
		AfterName: afterName, PageSize: limit, UploadID: pgtype.Int8{Int64: uploadID, Valid: uploadID != 0},
	})
	if err != nil {
		return nil, fmt.Errorf("list brands: %w", err)
	}
	return rows, nil
}

// Brand voice limits: enough for a style guide, small enough for a prompt.
const (
	MaxAvoidWords   = 50
	MaxAvoidWordLen = 60
)

// CleanAvoidWords trims the words, drops blanks and repeats (ignoring case)
// and returns a reason when the list breaks a limit.
func CleanAvoidWords(words []string) (clean []string, reason string) {
	clean = []string{}
	seen := map[string]bool{}
	for _, w := range words {
		w = strings.Join(strings.Fields(w), " ")
		if w == "" || seen[strings.ToLower(w)] {
			continue
		}
		if len([]rune(w)) > MaxAvoidWordLen {
			return nil, fmt.Sprintf("each word or phrase must be at most %d characters", MaxAvoidWordLen)
		}
		seen[strings.ToLower(w)] = true
		clean = append(clean, w)
	}
	if len(clean) > MaxAvoidWords {
		return nil, fmt.Sprintf("at most %d words or phrases", MaxAvoidWords)
	}
	return clean, ""
}

// SetBrandVoice sets the brand's tone (its voice note; nil clears it) and,
// when words is not nil, its words to avoid (brand voice settings).
func (s *Service) SetBrandVoice(ctx context.Context, id int64, tone *string, words []string) (store.Brand, error) {
	v := pgtype.Text{}
	if tone != nil {
		v = pgtype.Text{String: *tone, Valid: true}
	}
	b, err := store.New(s.pool).UpdateBrandVoice(ctx, store.UpdateBrandVoiceParams{VoiceNote: v, WordsToAvoid: words, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Brand{}, ErrBrandNotFound
	}
	if err != nil {
		return store.Brand{}, fmt.Errorf("update brand voice: %w", err)
	}
	return b, nil
}

// BrandsWithoutVoice names the brands in scope with no voice note; uploadID 0 means all.
func (s *Service) BrandsWithoutVoice(ctx context.Context, uploadID int64) ([]string, error) {
	names, err := store.New(s.pool).BrandsWithoutVoice(ctx, pgtype.Int8{Int64: uploadID, Valid: uploadID != 0})
	if err != nil {
		return nil, fmt.Errorf("brands without voice: %w", err)
	}
	return names, nil
}

// Upload reads an upload and its row errors.
func (s *Service) Upload(ctx context.Context, id int64) (store.GetUploadRow, []store.ListRowErrorsRow, error) {
	q := store.New(s.pool)
	u, err := q.GetUpload(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.GetUploadRow{}, nil, ErrUploadNotFound
	}
	if err != nil {
		return store.GetUploadRow{}, nil, fmt.Errorf("load upload: %w", err)
	}
	errs, err := q.ListRowErrors(ctx, id)
	if err != nil {
		return store.GetUploadRow{}, nil, fmt.Errorf("row errors: %w", err)
	}
	return u, errs, nil
}

// Products lists products with their attributes and AI cost (US-00-011).
func (s *Service) Products(ctx context.Context, p store.ListProductsParams) ([]store.ListProductsRow, error) {
	rows, err := store.New(s.pool).ListProducts(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	return rows, nil
}

// Product reads one product.
func (s *Service) Product(ctx context.Context, id int64) (store.ListProductsRow, error) {
	rows, err := s.Products(ctx, store.ListProductsParams{ProductID: pgtype.Int8{Int64: id, Valid: true}, PageSize: 1})
	if err != nil {
		return store.ListProductsRow{}, err
	}
	if len(rows) == 0 {
		return store.ListProductsRow{}, ErrProductNotFound
	}
	return rows[0], nil
}

// ErrNoImage means the product has no photo yet.
var ErrNoImage = errors.New("the product has no photo")

// Thumbnail opens a product's first photo as its 1024 px JPEG detection copy,
// small enough for the review grid; the caller closes it.
func (s *Service) Thumbnail(ctx context.Context, productID int64) (io.ReadCloser, error) {
	img, err := store.New(s.pool).FirstImage(ctx, productID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoImage
	}
	if err != nil {
		return nil, fmt.Errorf("first image: %w", err)
	}
	f, err := os.Open(img.DetectionPath)
	if err != nil {
		return nil, fmt.Errorf("open thumbnail: %w", err)
	}
	return f, nil
}
