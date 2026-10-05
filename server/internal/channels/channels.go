// Package channels loads the channel rule files (REQ-017) and checks a
// listing against them (REQ-008 to REQ-010). Rules are data: adding a channel
// is a new file under config/channels, never code.
package channels

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ExportColumn maps one CSV header to a listing or attribute field.
type ExportColumn struct {
	Header string `yaml:"header" json:"header"`
	Field  string `yaml:"field" json:"field"`
}

// Channel is one channel's configuration.
type Channel struct {
	ID                 string         `yaml:"id" json:"id"`
	Name               string         `yaml:"name" json:"name"`
	TitleMaxLength     int            `yaml:"title_max_length" json:"title_max_length"`
	RequiredAttributes []string       `yaml:"required_attributes" json:"required_attributes"`
	BannedWords        []string       `yaml:"banned_words" json:"banned_words"`
	ExportColumns      []ExportColumn `yaml:"export_columns" json:"export_columns"`
	// Hash identifies this exact configuration; rule results record it (D7).
	Hash string `yaml:"-" json:"-"`
}

// LoadError is a channel file that could not be used; that channel is off (D8).
type LoadError struct {
	File  string
	ID    string
	Error string
}

// Set is every channel that loaded, plus the files that did not.
type Set struct {
	Channels []Channel
	Errors   []LoadError
}

// Get returns the enabled channel with this id.
func (s Set) Get(id string) (Channel, bool) {
	for i := range s.Channels {
		if s.Channels[i].ID == id {
			return s.Channels[i], true
		}
	}
	return Channel{}, false
}

// IDs lists the enabled channel ids, sorted.
func (s Set) IDs() []string {
	ids := make([]string, 0, len(s.Channels))
	for i := range s.Channels {
		ids = append(ids, s.Channels[i].ID)
	}
	sort.Strings(ids)
	return ids
}

var (
	idRe     = regexp.MustCompile(`^[a-z][a-z0-9_]{1,39}$`)
	fieldSet = map[string]bool{
		"sku": true, "title": true, "description": true,
		"bullet_1": true, "bullet_2": true, "bullet_3": true, "bullet_4": true, "bullet_5": true,
		"colour": true, "pattern": true, "sleeve": true, "neckline": true, "fit": true,
	}
	attributeSet = map[string]bool{"colour": true, "pattern": true, "sleeve": true, "neckline": true, "fit": true}
)

// LoadDir reads every *.yaml in dir. A broken file disables only its own
// channel and is reported (D8); it never stops the others from loading.
func LoadDir(dir string) (Set, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return Set{}, fmt.Errorf("list channel files in %s: %w", dir, err)
	}
	sort.Strings(files)
	var set Set
	seen := map[string]bool{}
	for _, f := range files {
		c, err := loadFile(f)
		switch {
		case err != nil:
			set.Errors = append(set.Errors, LoadError{File: filepath.Base(f), ID: strings.TrimSuffix(filepath.Base(f), ".yaml"), Error: err.Error()})
		case seen[c.ID]:
			set.Errors = append(set.Errors, LoadError{File: filepath.Base(f), ID: c.ID, Error: "id " + c.ID + " is used by another file"})
		default:
			seen[c.ID] = true
			set.Channels = append(set.Channels, c)
		}
	}
	return set, nil
}

func loadFile(path string) (Channel, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // US-00-005: paths come from the config directory glob, not from a request.
	if err != nil {
		return Channel{}, fmt.Errorf("read: %w", err)
	}
	var c Channel
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return Channel{}, fmt.Errorf("parse: %w", err)
	}
	if err := c.validate(); err != nil {
		return Channel{}, err
	}
	sum := sha256.Sum256(raw)
	c.Hash = hex.EncodeToString(sum[:8])
	return c, nil
}

func (c Channel) validate() error {
	var problems []string
	if !idRe.MatchString(c.ID) {
		problems = append(problems, "id must be lowercase letters, digits and _, 2 to 40 long")
	}
	if strings.TrimSpace(c.Name) == "" {
		problems = append(problems, "name is required")
	}
	if c.TitleMaxLength < 1 {
		problems = append(problems, "title_max_length must be a whole number above 0")
	}
	for _, a := range c.RequiredAttributes {
		if !attributeSet[a] {
			problems = append(problems, "required_attributes has unknown attribute "+a)
		}
	}
	if len(c.ExportColumns) == 0 {
		problems = append(problems, "export_columns is required")
	}
	for _, col := range c.ExportColumns {
		if !fieldSet[col.Field] || strings.TrimSpace(col.Header) == "" {
			problems = append(problems, fmt.Sprintf("export column %q has unknown field %q", col.Header, col.Field))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	return nil
}
