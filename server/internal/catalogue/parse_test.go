package catalogue

import (
	"strconv"
	"strings"
	"testing"
)

func TestMatchSKU(t *testing.T) {
	skus := []string{"TS-1", "TS-10", "KU-104"}
	tests := []struct {
		file string
		want string
	}{
		{"TS-10_front.jpg", "TS-10"},
		{"TS-10-back.jpg", "TS-10"},
		{"TS-10.jpg", "TS-10"},
		{"TS-1_front.jpg", "TS-1"},
		{"ts-10_front.JPG", "TS-10"},
		{"KU-104", "KU-104"},
		{"TS-100_front.jpg", ""},
		{"TS10-front.jpg", ""},
		{"holiday-banner.png", ""},
	}
	for _, tc := range tests {
		t.Run(tc.file, func(t *testing.T) {
			got := MatchSKU(tc.file, skus)

			if got != tc.want {
				t.Fatalf("MatchSKU(%q) = %q, want %q", tc.file, got, tc.want)
			}
		})
	}
}

func TestParseCSV(t *testing.T) {
	in := "sku,category,brand,price\n" +
		"KU-101,kurta,Indigo Loom,1299\n" +
		"KU-102,kurta,Indigo Loom,1499.50\n" +
		",kurta,Indigo Loom,999\n" +
		"KU-101,kurta,Indigo Loom,1299\n" +
		"KU-103,kurta,,999\n" +
		"KU-104,kurta,Indigo Loom,-5\n" +
		"KU 105,kurta,Indigo Loom,999\n"

	rows, errs, err := ParseCSV(strings.NewReader(in))

	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].SKU != "KU-101" || rows[1].PriceMinor != 149950 {
		t.Fatalf("rows: %+v", rows)
	}
	want := []string{
		"4:sku is missing",
		"5:SKU KU-101 is repeated in this file (first on row 2)",
		"6:brand is missing",
		"7:price must be a positive number of rupees",
		"8:sku must have no spaces and at most 64 characters",
	}
	got := make([]string, 0, len(errs))
	for _, e := range errs {
		got = append(got, itoa(e.Row)+":"+e.Reason)
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("errors:\n got %v\nwant %v", got, want)
	}
}

func TestParseCSVRejectsWrongHeader(t *testing.T) {
	_, _, err := ParseCSV(strings.NewReader("name,price\nx,1\n"))

	if err == nil || !strings.Contains(err.Error(), "sku, category, brand, price") {
		t.Fatalf("err = %v", err)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }
