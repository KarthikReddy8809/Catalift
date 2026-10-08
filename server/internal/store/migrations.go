package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/KarthikReddy8809/catalift/server/db"
)

// RequireMigrations refuses to run against a database that is behind the
// code: new queries would fail on columns that do not exist yet, one job at a
// time, as products marked failed. It names what is missing and the fix.
func (s *Store) RequireMigrations(ctx context.Context) error {
	all, err := db.Migrations()
	if err != nil {
		return err
	}
	applied, err := s.AppliedMigrations(ctx)
	if err != nil {
		return err
	}
	if missing := db.Missing(all, applied); len(missing) > 0 {
		return fmt.Errorf("the database is missing migrations %s: stop make dev, run make migrate, then start it again",
			strings.Join(missing, ", "))
	}
	return nil
}
