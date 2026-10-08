// Package db carries the migration files inside the binaries, so the API and
// the worker can refuse to start against a database that is behind them.
package db

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var files embed.FS

// Migration is one goose migration file: its version and file name.
type Migration struct {
	Version int64
	Name    string
}

// Migrations lists the embedded migrations in version order.
func Migrations() ([]Migration, error) {
	entries, err := fs.ReadDir(files, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}
	out := make([]Migration, 0, len(entries))
	for _, e := range entries {
		prefix, _, ok := strings.Cut(e.Name(), "_")
		if !ok {
			continue
		}
		v, err := strconv.ParseInt(prefix, 10, 64)
		if err != nil {
			continue
		}
		out = append(out, Migration{Version: v, Name: e.Name()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// Missing names the migrations the database has not applied, in order.
func Missing(all []Migration, applied map[int64]bool) []string {
	var out []string
	for _, m := range all {
		if !applied[m.Version] {
			out = append(out, m.Name)
		}
	}
	return out
}
