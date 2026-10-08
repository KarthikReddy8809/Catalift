package channels

import (
	"strings"
	"testing"
)

func TestRegistryAppliesAnEditAndChangesTheHash(t *testing.T) {
	base := Set{Channels: []Channel{{ID: "web", TitleMaxLength: 120, BannedWords: []string{"sale"}, Hash: "aaaa"}}}
	reg := NewRegistry(base)

	reg.Apply(map[string]Edit{"web": {Rules: Rules{TitleMaxLength: 60, BannedWords: []string{"cheap"}}, By: "r@example.com"}})

	c, _ := reg.Current().Get("web")
	if c.TitleMaxLength != 60 || c.BannedWords[0] != "cheap" || c.Hash == "aaaa" {
		t.Fatalf("channel = %+v", c)
	}
	if e, ok := reg.LastEdit("web"); !ok || e.By != "r@example.com" {
		t.Fatalf("last edit = %+v %v", e, ok)
	}
	if base.Channels[0].TitleMaxLength != 120 {
		t.Fatal("Apply changed the base set")
	}
}

func TestRulesCleanTidiesAndRefuses(t *testing.T) {
	r, err := Rules{TitleMaxLength: 80, RequiredAttributes: []string{" Fit", "colour", "fit"}, BannedWords: []string{" Best  Seller ", "", "best seller"}}.Clean()
	if err != nil || len(r.RequiredAttributes) != 2 || len(r.BannedWords) != 1 || r.BannedWords[0] != "best seller" {
		t.Fatalf("r=%+v err=%v", r, err)
	}

	_, err = Rules{TitleMaxLength: 5, RequiredAttributes: []string{"size"}}.Clean()

	if err == nil || !strings.Contains(err.Error(), "title_max_length") || !strings.Contains(err.Error(), "size") {
		t.Fatalf("err = %v", err)
	}
}
