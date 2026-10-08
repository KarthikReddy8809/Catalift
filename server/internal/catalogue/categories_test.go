package catalogue

import (
	"strings"
	"testing"
)

func TestCategoriesFilterRejectsUnknownCategories(t *testing.T) {
	cats := NewCategories("kurta", "Salwar Suit")
	rows := []Row{
		{Line: 2, SKU: "A-1", Category: "Kurta"},
		{Line: 3, SKU: "A-2", Category: " salwar   suit "},
		{Line: 4, SKU: "A-3", Category: "jeans"},
	}

	ok, bad := cats.Filter(rows)

	if len(ok) != 2 || len(bad) != 1 || bad[0].Row != 4 || !strings.Contains(bad[0].Reason, "kurta, salwar suit") {
		t.Fatalf("ok=%+v bad=%+v", ok, bad)
	}
}

func TestLoadCategoriesReadsTheShippedFile(t *testing.T) {
	cats, err := LoadCategories("../../config/categories.yaml")

	if err != nil || !cats.Known("kurta") || cats.Known("jeans") {
		t.Fatalf("cats=%v err=%v", cats.Names(), err)
	}
}
