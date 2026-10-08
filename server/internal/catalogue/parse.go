// Package catalogue owns brands, uploads, products and their images
// (US-00-001): reading the CSV, matching photos to SKUs and storing files.
package catalogue

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Row is one valid CSV product row.
type Row struct {
	Line       int
	SKU        string
	Category   string
	Brand      string
	PriceMinor int64
}

// RowError is one rejected CSV row with the reason the seller sees, and the
// values as typed, so the seller can correct them in place and send the row
// again (seller flow step 2).
type RowError struct {
	Row      int
	SKU      string
	Reason   string
	Category string
	Brand    string
	Price    string
}

// ValidateRow checks one row's values; it is shared by the CSV upload and a
// seller's correction of a rejected row, so both apply the same rules. The
// reason is empty when the row is valid.
func ValidateRow(line int, sku, category, brand, price string) (row Row, reason string) {
	row = Row{Line: line, SKU: strings.TrimSpace(sku), Category: strings.TrimSpace(category), Brand: strings.TrimSpace(brand)}
	switch {
	case row.SKU == "":
		return row, "sku is missing"
	case !skuRe.MatchString(row.SKU):
		return row, "sku must have no spaces and at most 64 characters"
	case row.Category == "" || len(row.Category) > 100:
		return row, "category is missing or longer than 100 characters"
	case row.Brand == "" || len(row.Brand) > 100:
		return row, "brand is missing"
	}
	p, ok := rupeesToPaise(price)
	if !ok {
		return row, "price must be a positive number of rupees"
	}
	row.PriceMinor = p
	return row, ""
}

// ErrBadHeader means the file is not a Catalift product list at all.
var ErrBadHeader = errors.New("the first line must be the columns sku, category, brand, price")

var skuRe = regexp.MustCompile(`^\S{1,64}$`)

// ParseCSV reads the product list. Bad rows are rejected one by one with a
// reason (Q-020); the rest are returned. Row numbers count the header as 1.
func ParseCSV(r io.Reader) ([]Row, []RowError, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true
	header, err := cr.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("read header: %w", ErrBadHeader)
	}
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\ufeff")))] = i
	}
	for _, need := range []string{"sku", "category", "brand", "price"} {
		if _, ok := idx[need]; !ok {
			return nil, nil, ErrBadHeader
		}
	}

	var rows []Row
	var errs []RowError
	seen := map[string]int{}
	line := 1
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		line++
		if err != nil {
			errs = append(errs, RowError{Row: line, Reason: "the row could not be read as CSV"})
			continue
		}
		get := func(k string) string {
			if i := idx[k]; i < len(rec) {
				return strings.TrimSpace(rec[i])
			}
			return ""
		}
		row, reason := ValidateRow(line, get("sku"), get("category"), get("brand"), get("price"))
		if reason == "" && seen[strings.ToLower(row.SKU)] != 0 {
			reason = fmt.Sprintf("SKU %s is repeated in this file (first on row %d)", row.SKU, seen[strings.ToLower(row.SKU)])
		}
		if reason != "" {
			errs = append(errs, RowError{Row: line, SKU: row.SKU, Reason: reason,
				Category: clipRaw(get("category")), Brand: clipRaw(get("brand")), Price: clipRaw(get("price"))})
			continue
		}
		seen[strings.ToLower(row.SKU)] = line
		rows = append(rows, row)
	}
	return rows, errs, nil
}

func rupeesToPaise(s string) (int64, bool) {
	s = strings.ReplaceAll(strings.TrimPrefix(strings.TrimSpace(s), "₹"), ",", "")
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f <= 0 || math.IsInf(f, 0) || f > 1e9 {
		return 0, false
	}
	return int64(math.Round(f * 100)), true
}

// MatchSKU returns the SKU a photo belongs to: the file name must start with
// the SKU followed by _, -, . or the end of the name; the longest match wins,
// so TS-10_front.jpg never lands on TS-1 (D20). Case is ignored. "" means no
// SKU matched.
func MatchSKU(fileName string, skus []string) string {
	name := strings.ToLower(fileName)
	best := ""
	for _, sku := range skus {
		s := strings.ToLower(sku)
		if !strings.HasPrefix(name, s) {
			continue
		}
		rest := name[len(s):]
		if rest != "" && !strings.ContainsAny(rest[:1], "_-.") {
			continue
		}
		if len(sku) > len(best) {
			best = sku
		}
	}
	return best
}

// clipRaw keeps a typed value short enough to store for correction.
func clipRaw(v string) string {
	r := []rune(v)
	if len(r) > 100 {
		return string(r[:100])
	}
	return v
}
