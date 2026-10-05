package channels

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDirDisablesOnlyTheBrokenFile(t *testing.T) {
	dir := t.TempDir()
	good := "id: good\nname: Good\ntitle_max_length: 100\nexport_columns:\n  - { header: sku, field: sku }\n"
	bad := "id: bad\nname: Bad\ntitle_max_length: lots\n"
	for name, body := range map[string]string{"good.yaml": good, "bad.yaml": bad} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	set, err := LoadDir(dir)

	if err != nil {
		t.Fatal(err)
	}
	if len(set.Channels) != 1 || set.Channels[0].ID != "good" {
		t.Fatalf("channels: %+v", set.Channels)
	}
	if len(set.Errors) != 1 || set.Errors[0].ID != "bad" {
		t.Fatalf("errors: %+v", set.Errors)
	}
}

func TestShippedChannelFilesLoad(t *testing.T) {
	set, err := LoadDir("../../config/channels")

	if err != nil {
		t.Fatal(err)
	}
	if len(set.Errors) != 0 {
		t.Fatalf("a shipped channel file is broken: %+v", set.Errors)
	}
	if got := set.IDs(); len(got) != 2 || got[0] != "amazon_style" || got[1] != "own_website" {
		t.Fatalf("ids: %v", got)
	}
}

func TestHashChangesWithTheFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	body := "id: c1\nname: C\ntitle_max_length: %d\nexport_columns:\n  - { header: sku, field: sku }\n"
	write := func(n string) {
		if err := os.WriteFile(path, []byte(n), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(fmt.Sprintf(body, 100))
	a, _ := LoadDir(dir)
	write(fmt.Sprintf(body, 90))
	b, _ := LoadDir(dir)

	if a.Channels[0].Hash == b.Channels[0].Hash {
		t.Fatal("hash did not change when the limit changed")
	}
}
