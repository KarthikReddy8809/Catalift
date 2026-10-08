package channels

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Rules are the parts of a channel a reviewer may change in the app; the
// id, name and export columns stay as the channel file sets them.
type Rules struct {
	TitleMaxLength     int      `json:"title_max_length"`
	RequiredAttributes []string `json:"required_attributes"`
	BannedWords        []string `json:"banned_words"`
}

// Edit is the latest saved change to one channel's rules.
type Edit struct {
	Rules
	By string
	At time.Time
}

// Limits on an edit; the database repeats them as CHECK constraints.
const (
	MinTitleLength  = 10
	MaxTitleLength  = 500
	MaxBannedWords  = 200
	MaxBannedLength = 100
)

// Clean trims and de-duplicates an edit and refuses one the rules engine
// could not apply.
func (r Rules) Clean() (Rules, error) {
	var problems []string
	if r.TitleMaxLength < MinTitleLength || r.TitleMaxLength > MaxTitleLength {
		problems = append(problems, fmt.Sprintf("title_max_length must be %d to %d", MinTitleLength, MaxTitleLength))
	}
	attrs := []string{}
	seen := map[string]bool{}
	for _, a := range r.RequiredAttributes {
		a = strings.ToLower(strings.TrimSpace(a))
		if !attributeSet[a] {
			problems = append(problems, "required_attributes has unknown attribute "+a)
			continue
		}
		if !seen[a] {
			seen[a] = true
			attrs = append(attrs, a)
		}
	}
	words := []string{}
	seen = map[string]bool{}
	for _, w := range r.BannedWords {
		w = strings.Join(strings.Fields(strings.ToLower(w)), " ")
		switch {
		case w == "" || seen[w]:
		case len([]rune(w)) > MaxBannedLength:
			problems = append(problems, fmt.Sprintf("banned word %q is longer than %d characters", w, MaxBannedLength))
		default:
			seen[w] = true
			words = append(words, w)
		}
	}
	if len(words) > MaxBannedWords {
		problems = append(problems, fmt.Sprintf("at most %d banned words", MaxBannedWords))
	}
	if len(problems) > 0 {
		return Rules{}, errors.New(strings.Join(problems, "; "))
	}
	sort.Strings(attrs)
	return Rules{TitleMaxLength: r.TitleMaxLength, RequiredAttributes: attrs, BannedWords: words}, nil
}

// Registry is the live channel set: the files' channels with the latest
// saved edit of each applied. The API and the worker share its shape; each
// refreshes it from the database (listings.RefreshRules).
type Registry struct {
	mu    sync.RWMutex
	base  Set
	cur   Set
	edits map[string]Edit
}

// NewRegistry starts from the channel files with no edits.
func NewRegistry(base Set) *Registry {
	return &Registry{base: base, cur: base, edits: map[string]Edit{}}
}

// Current is the channel set to use now; it never changes under the caller.
func (r *Registry) Current() Set {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cur
}

// LastEdit says who last changed a channel's rules in the app, if anyone.
func (r *Registry) LastEdit(id string) (Edit, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.edits[id]
	return e, ok
}

// Apply replaces the edits. A changed channel gets a new Hash, which is what
// makes the re-check find its listings (D7).
func (r *Registry) Apply(edits map[string]Edit) {
	cur := Set{Errors: r.base.Errors, Channels: make([]Channel, len(r.base.Channels))}
	copy(cur.Channels, r.base.Channels)
	for i := range cur.Channels {
		c := &cur.Channels[i]
		e, ok := edits[c.ID]
		if !ok {
			continue
		}
		c.TitleMaxLength = e.TitleMaxLength
		c.RequiredAttributes = e.RequiredAttributes
		c.BannedWords = e.BannedWords
		b, _ := json.Marshal(e.Rules) // a struct of ints and strings always marshals
		sum := sha256.Sum256(append([]byte(c.Hash+"|"), b...))
		c.Hash = hex.EncodeToString(sum[:8])
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cur, r.edits = cur, edits
}
