package catalogue

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Categories is the set of product categories the upload accepts.
type Categories struct {
	names []string
	known map[string]bool
}

// NewCategories builds the set; names are compared without case or spaces.
func NewCategories(names ...string) Categories {
	c := Categories{known: map[string]bool{}}
	for _, n := range names {
		k := normCategory(n)
		if k != "" && !c.known[k] {
			c.known[k] = true
			c.names = append(c.names, k)
		}
	}
	return c
}

// LoadCategories reads the categories file; an empty list is an error, since
// it would reject every row.
func LoadCategories(path string) (Categories, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // US-00-001: path comes from CATEGORIES_FILE, set by the operator.
	if err != nil {
		return Categories{}, fmt.Errorf("read categories: %w", err)
	}
	var doc struct {
		Categories []string `yaml:"categories"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return Categories{}, fmt.Errorf("parse categories: %w", err)
	}
	c := NewCategories(doc.Categories...)
	if len(c.names) == 0 {
		return Categories{}, errors.New("categories file lists no categories")
	}
	return c, nil
}

// Known reports whether a category is accepted.
func (c Categories) Known(name string) bool { return c.known[normCategory(name)] }

// Names lists the accepted categories in file order.
func (c Categories) Names() []string { return c.names }

// Filter splits rows into accepted ones and row errors for unknown categories.
func (c Categories) Filter(rows []Row) ([]Row, []RowError) {
	ok := rows[:0:0]
	var bad []RowError
	for _, r := range rows {
		if c.Known(r.Category) {
			ok = append(ok, r)
			continue
		}
		bad = append(bad, RowError{Row: r.Line, SKU: r.SKU, Reason: c.unknownReason(r.Category),
			Category: r.Category, Brand: r.Brand, Price: fmt.Sprintf("%.2f", float64(r.PriceMinor)/100)})
	}
	return ok, bad
}

func normCategory(s string) string { return strings.Join(strings.Fields(strings.ToLower(s)), " ") }

func (c Categories) unknownReason(category string) string {
	return fmt.Sprintf("category %q is not one Catalift handles; use one of: %s", category, strings.Join(c.names, ", "))
}
