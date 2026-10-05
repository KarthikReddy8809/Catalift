package channels

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Listing is what the rules engine checks: the text and the product's attributes.
type Listing struct {
	Title       string
	Bullets     [5]string
	Description string
	Attributes  map[string]string
}

// Failure is one broken rule, shown to the reviewer as is (REQ-009).
type Failure struct {
	Rule    string `json:"rule"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Validate is the code-based rules engine (REQ-008): pure, deterministic, no
// AI. The order of failures is fixed: empty fields, title length, banned
// words by field, required attributes.
func Validate(c Channel, l Listing) []Failure {
	failures := []Failure{}
	fields := l.fields()

	for _, f := range fields {
		if strings.TrimSpace(f.text) == "" {
			failures = append(failures, Failure{Rule: "empty_field", Field: f.name, Message: fieldLabel(f.name) + " is empty."})
		}
	}

	if n := utf8.RuneCountInString(l.Title); n > c.TitleMaxLength {
		failures = append(failures, Failure{
			Rule: "title_max_length", Field: "title",
			Message: fmt.Sprintf("Title is %d characters; the %s limit is %d.", n, c.Name, c.TitleMaxLength),
		})
	}

	for _, f := range fields {
		for _, word := range c.BannedWords {
			if containsWord(f.text, word) {
				failures = append(failures, Failure{
					Rule: "banned_word", Field: f.name,
					Message: fmt.Sprintf("%s uses the banned phrase %q.", fieldLabel(f.name), word),
				})
			}
		}
	}

	for _, a := range c.RequiredAttributes {
		v := strings.TrimSpace(l.Attributes[a])
		if v == "" || strings.EqualFold(v, "unknown") {
			failures = append(failures, Failure{
				Rule: "required_attribute", Field: a,
				Message: fmt.Sprintf("%s needs the %s attribute; it is missing or unknown.", c.Name, a),
			})
		}
	}
	return failures
}

type field struct{ name, text string }

func (l Listing) fields() []field {
	out := []field{{"title", l.Title}}
	for i, b := range l.Bullets {
		out = append(out, field{fmt.Sprintf("bullet_%d", i+1), b})
	}
	return append(out, field{"description", l.Description})
}

func fieldLabel(name string) string {
	if strings.HasPrefix(name, "bullet_") {
		return "Bullet " + strings.TrimPrefix(name, "bullet_")
	}
	if name == "" {
		return name
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// containsWord matches a banned word or phrase on word boundaries, ignoring
// case, so "sale" does not flag "wholesale".
func containsWord(text, word string) bool {
	if strings.TrimSpace(word) == "" {
		return false
	}
	re := regexp.MustCompile(`(?i)(^|[^\pL\pN])` + regexp.QuoteMeta(word) + `($|[^\pL\pN])`)
	return re.MatchString(text)
}
