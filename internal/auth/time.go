package auth

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func tsz(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
