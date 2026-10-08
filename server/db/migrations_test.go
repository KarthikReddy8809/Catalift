package db

import (
	"strings"
	"testing"
)

func TestMigrationsAreEmbeddedInOrder(t *testing.T) {
	all, err := Migrations()

	if err != nil || len(all) < 8 {
		t.Fatalf("migrations: %v, %v", all, err)
	}
	for i := 1; i < len(all); i++ {
		if all[i].Version <= all[i-1].Version {
			t.Fatalf("not in version order: %v", all)
		}
	}
	if !strings.HasPrefix(all[0].Name, "00001_") {
		t.Fatalf("first migration %q", all[0].Name)
	}
}

func TestMissingNamesTheUnappliedMigrations(t *testing.T) {
	all := []Migration{{1, "00001_init.sql"}, {7, "00007_words.sql"}, {8, "00008_export.sql"}}

	got := Missing(all, map[int64]bool{1: true, 8: true})

	if strings.Join(got, ",") != "00007_words.sql" {
		t.Fatalf("missing %v", got)
	}
}
