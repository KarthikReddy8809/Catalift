package listings

import (
	"context"
	"errors"
	"fmt"

	"github.com/KarthikReddy8809/catalift/server/internal/channels"
	"github.com/KarthikReddy8809/catalift/server/internal/store"
)

// InvalidRulesError is an edit the rules engine could not apply; the message
// names every problem.
type InvalidRulesError struct{ Reason string }

func (e *InvalidRulesError) Error() string { return e.Reason }

// RefreshRules loads the latest saved rule edit per channel into reg. The
// API calls it at start-up and after each save; the worker before each job,
// so a reviewer's change reaches the next prompt and the next check.
func RefreshRules(ctx context.Context, q *store.Queries, reg *channels.Registry) error {
	rows, err := q.LatestChannelRuleEdits(ctx)
	if err != nil {
		return fmt.Errorf("load rule edits: %w", err)
	}
	edits := make(map[string]channels.Edit, len(rows))
	for _, r := range rows {
		edits[r.Channel] = channels.Edit{
			Rules: channels.Rules{
				TitleMaxLength: int(r.TitleMaxLength), RequiredAttributes: r.RequiredAttributes, BannedWords: r.BannedWords,
			},
			By: r.EditedBy, At: r.CreatedAt.Time,
		}
	}
	reg.Apply(edits)
	return nil
}

// SaveRules stores a reviewer's edit to one channel's rules, made against the
// rules with configHash, then re-checks that channel's listings: an approved
// listing that now fails loses its approval (D7). A stale configHash means
// someone else changed the rules first: ErrVersionConflict.
func (s *Service) SaveRules(ctx context.Context, userID int64, channel, configHash string, rules channels.Rules) (RecheckCounts, error) {
	clean, err := rules.Clean()
	if err != nil {
		return RecheckCounts{}, &InvalidRulesError{Reason: err.Error()}
	}
	q := store.New(s.pool)
	if err := RefreshRules(ctx, q, s.rules); err != nil {
		return RecheckCounts{}, err
	}
	ch, ok := s.rules.Current().Get(channel)
	if !ok {
		return RecheckCounts{}, ErrNotFound
	}
	if ch.Hash != configHash {
		return RecheckCounts{}, ErrVersionConflict
	}
	if _, err := q.InsertChannelRuleEdit(ctx, store.InsertChannelRuleEditParams{
		Channel: channel, TitleMaxLength: int32(clean.TitleMaxLength), //nolint:gosec // US-00-005: Clean bounds it to 10..500.
		RequiredAttributes: clean.RequiredAttributes, BannedWords: clean.BannedWords, EditedBy: userID,
	}); err != nil {
		return RecheckCounts{}, fmt.Errorf("save rule edit: %w", err)
	}
	if err := RefreshRules(ctx, q, s.rules); err != nil {
		return RecheckCounts{}, err
	}
	counts, err := s.recheck(ctx)
	if err != nil {
		return RecheckCounts{}, err
	}
	return counts[channel], nil
}

// IsInvalidRules reports whether err is a refused rule edit.
func IsInvalidRules(err error) (*InvalidRulesError, bool) {
	var e *InvalidRulesError
	ok := errors.As(err, &e)
	return e, ok
}

// RefreshRules reloads the saved rule edits into the service's rule set.
func (s *Service) RefreshRules(ctx context.Context) error {
	return RefreshRules(ctx, store.New(s.pool), s.rules)
}
