package catalogue

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/KarthikReddy8809/catalift/server/internal/store"
)

// ErrBrandNotFound is an unknown brand id.
var ErrBrandNotFound = errors.New("no such brand")

// ErrProductNotFound is an unknown product id.
var ErrProductNotFound = errors.New("no such product")

// Brands lists brands by name after the given name.
func (s *Service) Brands(ctx context.Context, afterName string, limit int32) ([]store.Brand, error) {
	rows, err := store.New(s.pool).ListBrands(ctx, store.ListBrandsParams{AfterName: afterName, PageSize: limit})
	if err != nil {
		return nil, fmt.Errorf("list brands: %w", err)
	}
	return rows, nil
}

// SetVoiceNote sets or clears a brand's voice note (US-00-002).
func (s *Service) SetVoiceNote(ctx context.Context, id int64, note *string) (store.Brand, error) {
	v := pgtype.Text{}
	if note != nil {
		v = pgtype.Text{String: *note, Valid: true}
	}
	b, err := store.New(s.pool).UpdateBrandVoice(ctx, store.UpdateBrandVoiceParams{VoiceNote: v, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Brand{}, ErrBrandNotFound
	}
	if err != nil {
		return store.Brand{}, fmt.Errorf("update voice note: %w", err)
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
